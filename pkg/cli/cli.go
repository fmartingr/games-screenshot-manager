package cli

import (
	"bytes"
	"flag"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
	"github.com/fmartingr/games-screenshot-mananger/pkg/providers/steam"
)

var allowedProviders = [...]string{"steam"}

const defaultOutputPath string = "./Output"
const defaultProvider string = "steam"
const defaultDryRun bool = false

func Start() {
	var provider = flag.String("provider", defaultProvider, "steam")
	var outputPath = flag.String("output-path", defaultOutputPath, "The destination path of the screenshots")
	var dryRun = flag.Bool("dry-run", defaultDryRun, "Use to disable write actions on filesystem")

	flag.Parse()
	if helpers.SliceContainsString(allowedProviders[:], *provider, nil) {
		games := getGamesFromProvider(*provider)
		processGames(games, *outputPath, *dryRun)
	} else {
		log.Printf("Provider %s not found!", *provider)
	}
}

func getGamesFromProvider(provider string) []games.Game {
	var games []games.Game
	if provider == "steam" {
		games = append(games, steam.GetGames()...)
	}
	return games
}

func processGames(games []games.Game, outputPath string, dryRun bool) {
	for _, game := range games {
		destinationPath := path.Join(helpers.ExpandUser(outputPath), game.Platform)
		if len(game.Name) > 0 {
			destinationPath = path.Join(destinationPath, game.Name)
		} else {
			log.Printf("[IMPORTANT] Game ID %d has no name!", game.ID)
			destinationPath = path.Join(destinationPath, strconv.FormatUint(game.ID, 10))
		}

		// Check if folder exists
		if _, err := os.Stat(destinationPath); os.IsNotExist(err) {
			os.MkdirAll(destinationPath, 0711)
		}

		log.Printf("=> Proceesing screenshots for %s %s", game.Name, game.Notes)
		for _, screenshot := range game.Screenshots {
			destinationPath := path.Join(destinationPath, screenshot.GetDestinationName())

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
				if dryRun {
					log.Println(path.Base(screenshot.Path), " -> ", strings.Replace(destinationPath, helpers.ExpandUser(outputPath), "", 1))
				} else {
					helpers.CopyFile(screenshot.Path, destinationPath)
				}
			}
		}
	}

}
