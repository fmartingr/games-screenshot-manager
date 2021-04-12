package cli

import (
	"bytes"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fmartingr/games-screenshot-manager/pkg/games"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/minecraft"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/nintendo_switch"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/playstation4"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/retroarch"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers/steam"
	"github.com/gosimple/slug"
)

var allowedProviders = [...]string{"steam", "minecraft", "nintendo-switch", "playstation-4", "retroarch"}

const defaultOutputPath string = "./Output"

const defaultInputPath string = ""
const defaultProvider string = "steam"
const defaultDryRun bool = false
const defaultDownloadCovers bool = false

func Start() {
	cliOptions := games.CLIOptions{
		OutputPath:     flag.String("output-path", defaultOutputPath, "The destination path of the screenshots"),
		InputPath:      flag.String("input-path", defaultInputPath, "Input path for the provider that requires it"),
		DownloadCovers: flag.Bool("download-covers", defaultDownloadCovers, "use to enable the download of covers (if the provider supports it)"),
		DryRun:         flag.Bool("dry-run", defaultDryRun, "Use to disable write actions on filesystem"),
	}
	var provider = flag.String("provider", defaultProvider, "steam")

	flag.Parse()

	if helpers.SliceContainsString(allowedProviders[:], *provider, nil) {
		games := getGamesFromProvider(*provider, cliOptions)
		processGames(games, cliOptions)
	} else {
		log.Printf("Provider %s not found!", *provider)
	}
}

func getGamesFromProvider(provider string, cliOptions games.CLIOptions) []games.Game {
	var games []games.Game
	switch provider {
	case "steam":
		games = append(games, steam.GetGames(cliOptions)...)
	case "minecraft":
		games = append(games, minecraft.GetGames(cliOptions)...)
	case "nintendo-switch":
		games = append(games, nintendo_switch.GetGames(cliOptions)...)
	case "playstation-4":
		games = append(games, playstation4.GetGames(cliOptions)...)
	case "retroarch":
		games = append(games, retroarch.GetGames(cliOptions)...)
	}
	return games
}

func processGames(games []games.Game, cliOptions games.CLIOptions) {
	for _, game := range games {
		destinationPath := filepath.Join(helpers.ExpandUser(*cliOptions.OutputPath), game.Platform)
		if len(game.Name) > 0 {
			destinationPath = filepath.Join(destinationPath, game.Name)
		} else {
			log.Printf("[IMPORTANT] Game ID %s has no name!", game.ID)
			destinationPath = filepath.Join(destinationPath, game.ID)
		}

		// Do not continue if there's no screenshots
		if len(game.Screenshots) == 0 {
			continue
		}

		// Check if folder exists
		if _, err := os.Stat(destinationPath); os.IsNotExist(err) && !*cliOptions.DryRun {
			mkdirErr := os.MkdirAll(destinationPath, 0711)
			if mkdirErr != nil {
				log.Printf("[ERROR] Couldn't create directory with name %s, falling back to %s", game.Name, slug.Make(game.Name))
				destinationPath = filepath.Join(helpers.ExpandUser(*cliOptions.OutputPath), game.Platform, slug.Make(game.Name))
				os.MkdirAll(destinationPath, 0711)
			}
		}

		if *cliOptions.DownloadCovers && !*cliOptions.DryRun && game.Cover.Path != "" {
			destinationCoverPath := filepath.Join(destinationPath, game.Cover.DestinationName)

			if _, err := os.Stat(destinationCoverPath); os.IsNotExist(err) {
				helpers.CopyFile(game.Cover.Path, destinationCoverPath)
			}
		}

		log.Printf("=> Proceesing screenshots for %s %s", game.Name, game.Notes)
		for _, screenshot := range game.Screenshots {
			destinationPath := filepath.Join(destinationPath, screenshot.GetDestinationName())

			if _, err := os.Stat(destinationPath); !os.IsNotExist(err) {
				sourceMd5, err := helpers.Md5File(screenshot.Path)
				if err != nil {
					log.Fatal(err)
					continue
				}
				destinationMd5, err := helpers.Md5File(destinationPath)
				if err != nil {
					log.Fatal(err)
					continue
				}

				if bytes.Compare(sourceMd5, destinationMd5) != 0 {
					// Images are not equal, we should copy it anyway, but how?
					log.Println("Found different screenshot with equal timestamp for game ", game.Name, screenshot.Path)
				}

			} else {
				if *cliOptions.DryRun {
					log.Println(filepath.Base(screenshot.Path), " -> ", strings.Replace(destinationPath, helpers.ExpandUser(*cliOptions.OutputPath), "", 1))
				} else {
					helpers.CopyFile(screenshot.Path, destinationPath)
				}
			}
		}
	}

}
