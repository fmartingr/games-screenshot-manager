package cli

import (
	"bytes"
	"flag"
	"log"
	"os"
	"path"
	"strconv"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
	"github.com/fmartingr/games-screenshot-mananger/pkg/providers/steam"
)

var AllowedProviders = [...]string{"steam"}

const OutputPath string = "~/Developer/games-screenshot-manager/Output"

func main() {
	Start()
}

func Start() {
	var provider = flag.String("provider", "steam", "steam")
	flag.Parse()
	if helpers.SliceContainsString(AllowedProviders[:], *provider, nil) {
		games := getGamesFromProvider(*provider)
		processGames(games)
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

func processGames(games []games.Game) {
	dryRun := false

	for _, game := range games {
		destinationPath := path.Join(helpers.ExpandUser(OutputPath), game.Platform)
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

		log.Printf("=> Proceesing screenshots for %s", game.Name)
		for _, screenshot := range game.Screenshots {
			fileStat, statErr := os.Stat(screenshot.Path)
			if statErr != nil {
				log.Fatal(statErr)
			}

			destinationPath := path.Join(destinationPath, fileStat.ModTime().Format("2006-01-02_15-04-05")+path.Ext(screenshot.Path))

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
				if !dryRun {
					helpers.CopyFile(screenshot.Path, destinationPath)
				}
			}
		}
	}

}
