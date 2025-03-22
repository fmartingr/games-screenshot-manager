package gamesscreenshotmanager

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// RunCLI starts the command line interface
func RunCLI() error {
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	))

	// Define flags
	configPath := flag.String("config", "config.toml", "Path to the configuration file")
	flag.Parse()

	// Load configuration
	config, err := NewConfig(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	registry, err := NewProviderRegistry(config)
	if err != nil {
		return fmt.Errorf("failed to create provider registry: %w", err)
	}

	for _, provider := range registry.Providers {
		if err := provider.Run(); err != nil {
			return fmt.Errorf("failed to run provider: %w", err)
		}
	}

	return nil
}

// main is the entry point of the application
func main() {
	if err := RunCLI(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
