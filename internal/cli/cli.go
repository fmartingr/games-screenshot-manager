package cli

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/processor"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/minecraft"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/nintendo_switch"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/playstation4"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/retroarch"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/steam"
	"github.com/fmartingr/games-screenshot-manager/pkg/registry"
)

const defaultOutputPath string = "./Output"

const defaultInputPath string = ""
const defaultProvider string = "steam"
const defaultDryRun bool = false
const defaultDownloadCovers bool = false

func Start() {
	registry := registry.NewProviderRegistry()
	registry.Register(minecraft.Name, minecraft.NewMinecraftProvider())
	registry.Register(nintendo_switch.Name, nintendo_switch.NewNintendoSwitchProvider())
	registry.Register(playstation4.Name, playstation4.NewPlaystation4Provider())
	registry.Register(steam.Name, steam.NewSteamProvider())
	registry.Register(retroarch.Name, retroarch.NewRetroArchProvider())

	flagSet := flag.NewFlagSet("gsm", flag.ExitOnError)

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

	flagSet.Parse(os.Args[1:])

	provider, err := registry.Get(*providerName)
	if err != nil {
		log.Printf("Provider %s not found!", *providerName)
		return
	}
	games, err := provider.FindGames(providerOptions)
	if err != nil {
		log.Println(err)
		return
	}

	if len(games) > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		processor := processor.NewProcessor(options)
		processor.Start(ctx)

		for _, g := range games {
			processor.Process(g)
		}

		processor.Wait()
		cancel()
	}
}
