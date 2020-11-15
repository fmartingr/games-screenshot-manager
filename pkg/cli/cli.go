package cli

import (
	"bytes"
	"flag"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
	"github.com/fmartingr/games-screenshot-mananger/pkg/providers/minecraft"
	"github.com/fmartingr/games-screenshot-mananger/pkg/providers/steam"
)

var allowedProviders = [...]string{"steam", "minecraft"}

const defaultOutputPath string = "./Output"

// const defaultInputPath string = "./Input"
const defaultProvider string = "steam"
const defaultDryRun bool = false

func Start() {
	var provider = flag.String("provider", defaultProvider, "steam")
	var outputPath = flag.String("output-path", defaultOutputPath, "The destination path of the screenshots")
	// var inputPath = flag.String("input-path", defaultInputPath, "Input path for the provider that requires it")
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
	switch provider {
	case "steam":
		games = append(games, steam.GetGames()...)
	case "minecraft":
		games = append(games, minecraft.GetGames()...)
	}
	return games
}

func processGames(games []games.Game, outputPath string, dryRun bool) {
	for _, game := range games {
		destinationPath := filepath.Join(helpers.ExpandUser(outputPath), game.Platform)
		if len(game.Name) > 0 {
			destinationPath = filepath.Join(destinationPath, game.Name)
		} else {
			log.Printf("[IMPORTANT] Game ID %d has no name!", game.ID)
			destinationPath = filepath.Join(destinationPath, strconv.FormatUint(game.ID, 10))
		}

		// Check if folder exists
		if _, err := os.Stat(destinationPath); os.IsNotExist(err) && !dryRun {
			os.MkdirAll(destinationPath, 0711)
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
				if dryRun {
					log.Println(path.Base(screenshot.Path), " -> ", strings.Replace(destinationPath, helpers.ExpandUser(outputPath), "", 1))
				} else {
					helpers.CopyFile(screenshot.Path, destinationPath)
				}
			}
		}
	}

}
