package gamesscreenshotmanager

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// cliArgs is what one command line asks for.
type cliArgs struct {
	doctor           bool
	configPath       string
	onlyBuildGallery bool
	dryRun           bool
	logLevel         string
}

// parseArgs reads one command line.
//
// The doctor subcommand is accepted before the flags and after them, so
// "doctor -config <path>" and "-config <path> doctor" ask for the same thing.
// Any other leftover argument is refused. A typo that was ignored would run
// the tool and copy files, when the user asked it to report and change
// nothing.
//
// A leading "doctor" is taken off the list before the flags are parsed,
// because flag.Parse stops at the first argument that is not a flag. A
// "doctor" that follows the flags is a leftover, and it is read there. A flag
// value of "doctor", as in "-config doctor", is a value and is left alone.
func parseArgs(args []string) (cliArgs, error) {
	parsed := cliArgs{}

	if len(args) > 0 && args[0] == "doctor" {
		parsed.doctor = true
		args = args[1:]
	}

	flags := flag.NewFlagSet("games-screenshot-manager", flag.ContinueOnError)
	flags.StringVar(&parsed.configPath, "config", "", "Path to the configuration file")
	flags.BoolVar(&parsed.onlyBuildGallery, "only-build-gallery", false, "Only build the gallery and avoid processing games")
	flags.BoolVar(&parsed.dryRun, "dry-run", false, "Dry run mode")
	flags.StringVar(&parsed.logLevel, "log", "info", "Log level")

	if err := flags.Parse(args); err != nil {
		return parsed, fmt.Errorf("failed to parse the arguments: %w", err)
	}

	leftover := make([]string, 0, flags.NArg())
	for _, arg := range flags.Args() {
		if arg == "doctor" {
			parsed.doctor = true
			continue
		}

		leftover = append(leftover, arg)
	}

	if len(leftover) > 0 {
		return parsed, fmt.Errorf("unknown argument: %s", strings.Join(leftover, " "))
	}

	if _, ok := levelMap[parsed.logLevel]; !ok {
		return parsed, fmt.Errorf("invalid log level: %s", parsed.logLevel)
	}

	return parsed, nil
}

// RunCLI starts the command line interface
func RunCLI() error {
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		return err
	}

	configPath := args.configPath
	if configPath == "" {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get user config directory: %w", err)
		}

		configPath = filepath.Join(userConfigDir, "games-screenshot-manager", "config.toml")
	}

	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      levelMap[args.logLevel],
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

	config.DryRun = args.dryRun

	// The doctor reports on the configuration and the system. It changes
	// nothing, so it returns before any provider runs.
	if args.doctor {
		results := RunDoctor(config, configPath)
		PrintChecks(os.Stdout, results)

		if HasFailure(results) {
			return fmt.Errorf("the doctor found %d failed checks", CountFailures(results))
		}

		return nil
	}

	gameManager := NewGameManager()
	fileManager := NewFileManager(*config)

	if !args.onlyBuildGallery {
		registry, err := NewProviderRegistry(config, gameManager, fileManager)
		if err != nil {
			return fmt.Errorf("failed to create provider registry: %w", err)
		}

		if args.dryRun {
			slog.Info("dry run is enabled, no changes will be made to the files in the system")
		}

		for _, provider := range registry.Providers {
			if err := provider.Run(); err != nil {
				return fmt.Errorf("failed to run provider: %w", err)
			}
		}

		if !args.dryRun {
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
