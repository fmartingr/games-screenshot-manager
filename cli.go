package gamesscreenshotmanager

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/lmittmann/tint"
)

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// RunCLI starts the command line interface
func RunCLI() error {
	var configPath string
	flag.StringVar(&configPath, "config", "config.toml", "Path to the configuration file")
	logLevel := flag.String("log", "info", "Log level")
	flag.Parse()

	if configPath == "" {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get user config directory: %w", err)
		}

		configPath = filepath.Join(userConfigDir, "games-screenshot-manager", "config.toml")
	}

	if _, ok := levelMap[*logLevel]; !ok {
		return fmt.Errorf("invalid log level: %s", *logLevel)
	}

	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      levelMap[*logLevel],
			TimeFormat: time.Kitchen,
		}),
	))

	// Load configuration
	config, err := NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	gameManager := NewGameManager()
	fileManager := NewFileManager(*config)

	registry, err := NewProviderRegistry(config, gameManager, fileManager)
	if err != nil {
		return fmt.Errorf("failed to create provider registry: %w", err)
	}

	for _, provider := range registry.Providers {
		if err := provider.Run(); err != nil {
			return fmt.Errorf("failed to run provider: %w", err)
		}
	}

	if !config.DryRun {
		for _, game := range gameManager.GetGames() {
			if err := fileManager.ProcessGame(game); err != nil {
				slog.Error("failed to process game", "error", err)
			}
		}

		if config.Gallery.Create {
			builder, err := NewGalleryBuilder(*config)
			if err != nil {
				return fmt.Errorf("failed to create gallery builder: %w", err)
			}

			if _, err := builder.Build(); err != nil {
				return fmt.Errorf("failed to build gallery: %w", err)
			}

			if err := fileManager.Cleanup(); err != nil {
				return fmt.Errorf("failed to cleanup: %w", err)
			}
		}
	}
	return nil
}
