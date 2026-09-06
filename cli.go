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
	// The subcommand comes off the argument list before the flags are parsed,
	// so "doctor -config <path>" reaches the same flags a run takes.
	args := os.Args[1:]

	doctorMode := false
	if len(args) > 0 && args[0] == "doctor" {
		doctorMode = true
		args = args[1:]
	}

	var configPath string
	var onlyBuildGallery bool
	var dryRun bool
	flag.StringVar(&configPath, "config", "", "Path to the configuration file")
	flag.BoolVar(&onlyBuildGallery, "only-build-gallery", false, "Only build the gallery and avoid processing games")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run mode")
	logLevel := flag.String("log", "info", "Log level")

	if err := flag.CommandLine.Parse(args); err != nil {
		return fmt.Errorf("failed to parse the arguments: %w", err)
	}

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
			// The log follows the same rule as the report: a terminal gets
			// colour, a pipe and NO_COLOR do not.
			NoColor: !takesColor(os.Stdout),
		}),
	))

	// Load configuration
	config, err := NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	config.DryRun = dryRun

	// The doctor reports on the configuration and the system. It changes
	// nothing, so it returns before any provider runs.
	if doctorMode {
		results := RunDoctor(config, configPath)
		PrintChecks(os.Stdout, results)

		if HasFailure(results) {
			return fmt.Errorf("the doctor found %d failed checks", CountFailures(results))
		}

		return nil
	}

	gameManager := NewGameManager()
	fileManager := NewFileManager(*config)

	if !onlyBuildGallery {
		registry, err := NewProviderRegistry(config, gameManager, fileManager)
		if err != nil {
			return fmt.Errorf("failed to create provider registry: %w", err)
		}

		if dryRun {
			slog.Info("dry run is enabled, no changes will be made to the files in the system")
		}

		for _, provider := range registry.Providers {
			if err := provider.Run(); err != nil {
				return fmt.Errorf("failed to run provider: %w", err)
			}
		}

		if !dryRun {
			for _, game := range gameManager.GetGames() {
				if err := fileManager.ProcessGame(game); err != nil {
					slog.Error("failed to process game", "error", err)
				}
			}
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
	}

	if err := fileManager.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup: %w", err)
	}

	slog.Info("done")

	return nil
}
