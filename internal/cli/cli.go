package cli

import (
	"context"
	"flag"
	"os"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/cache"
	"github.com/fmartingr/games-screenshot-manager/pkg/processor"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/minecraft"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/playstation4"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/playstation5"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/retroarch"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/steam"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/worldofwarcraft"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/xbox_game_bar"
	"github.com/fmartingr/games-screenshot-manager/pkg/registry"
	"github.com/sirupsen/logrus"
)

const defaultOutputPath string = "./Output"

const defaultInputPath string = ""
const defaultProvider string = "steam"
const defaultDryRun bool = false
const defaultDownloadCovers bool = false

func Start() {
	logger := logrus.New()
	flagSet := flag.NewFlagSet("gsm", flag.ExitOnError)

	cache := cache.NewFileCache(logger)

	registry := registry.NewProviderRegistry(logger, cache)
	registry.Register(minecraft.Name, minecraft.NewMinecraftProvider)
	registry.Register(playstation4.Name, playstation4.NewPlaystation4Provider)
	registry.Register(playstation5.Name, playstation5.NewPlaystation5Provider)
	registry.Register(xbox_game_bar.Name, xbox_game_bar.NewXboxGameGarProvider)
	registry.Register(steam.Name, steam.NewSteamProvider)
	registry.Register(retroarch.Name, retroarch.NewRetroArchProvider)
	registry.Register(worldofwarcraft.Name, worldofwarcraft.NewWorldOfWarcraftProvider)

	options := models.Options{
		ProcessBufferSize: 32,
	}

	flagSet.StringVar(&options.OutputPath, "output-path", defaultOutputPath, "The destination path of the screenshots")
	flagSet.BoolVar(&options.DownloadCovers, "download-covers", defaultDownloadCovers, "use to enable the download of covers (if the provider supports it)")
	flagSet.BoolVar(&options.DryRun, "dry-run", defaultDryRun, "Use to disable write actions on filesystem")
	flagSet.IntVar(&options.WorkersNum, "workers-num", 2, "Number of workers to use to process games")

	var providerName = flagSet.String("provider", defaultProvider, "steam")
	providerOptions := models.ProviderOptions{}
	flagSet.StringVar(&providerOptions.InputPath, "input-path", defaultInputPath, "Input path for the provider that requires it")

	loglevelFlag := flagSet.String("log-level", logrus.InfoLevel.String(), "Log level")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		logger.Errorf("error parsing args: %s", err)
	}

	loglevel, err := logrus.ParseLevel(*loglevelFlag)
	if err != nil {
		logger.Warnf("Invalid loglevel %s, using %s instead.", *loglevelFlag, logrus.InfoLevel.String())
		loglevel = logrus.InfoLevel
	}
	logger.SetLevel(loglevel)

	provider, err := registry.Get(*providerName)
	if err != nil {
		logger.Errorf("Provider %s not found!", *providerName)
		return
	}
	games, err := provider.FindGames(providerOptions)
	if err != nil {
		logger.Errorf("Error obtaining game list: %s", err)
		return
	}

	if len(games) > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		processor := processor.NewProcessor(logger, options)
		processor.Start(ctx)

		for _, g := range games {
			processor.Process(g)
		}

		processor.Wait()
		cancel()
	} else {
		logger.Info("No games found.")
	}
}
