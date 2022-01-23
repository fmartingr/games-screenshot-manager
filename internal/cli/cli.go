package cli

import (
	"context"
	"flag"
	"log"

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

	options := models.Options{
		OutputPath:        *flag.String("output-path", defaultOutputPath, "The destination path of the screenshots"),
		DownloadCovers:    *flag.Bool("download-covers", defaultDownloadCovers, "use to enable the download of covers (if the provider supports it)"),
		DryRun:            *flag.Bool("dry-run", defaultDryRun, "Use to disable write actions on filesystem"),
		ProcessBufferSize: 0, // Unbuffered for now
	}
	var providerName = flag.String("provider", defaultProvider, "steam")
	providerOptions := models.ProviderOptions{
		InputPath: *flag.String("input-path", defaultInputPath, "Input path for the provider that requires it"),
	}

	flag.Parse()

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

	ctx, cancel := context.WithCancel(context.Background())
	processor := processor.NewProcessor(options)

	if len(games) > 0 {
		processor.Start(ctx)

		for _, g := range games {
			processor.Process(g)
		}

		processor.Wait()
	}

	cancel()
}
