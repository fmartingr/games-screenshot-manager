package playstation4

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
)

const providerName = "playstation-4"
const platformName = "PlayStation 4"

func addScreenshotToGame(userGames []games.Game, gameName string, screenshot games.Screenshot) []games.Game {
	var foundGame games.Game
	for gameIndex, game := range userGames {
		if game.Name == gameName {
			foundGame = game
			userGames[gameIndex].Screenshots = append(userGames[gameIndex].Screenshots, screenshot)
		}
	}

	// Game not found
	if foundGame.Name == "" {
		foundGame := games.Game{Name: gameName, ID: gameName, Platform: platformName, Provider: providerName}
		foundGame.Screenshots = append(foundGame.Screenshots, screenshot)
		userGames = append(userGames, foundGame)
	}

	return userGames
}

func GetGames(inputPath string) []games.Game {
	var userGames []games.Game

	err := filepath.Walk(inputPath,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				gameName := filepath.Base(filepath.Dir(filePath))
				fileName := filepath.Base(filePath)
				extension := filepath.Ext(filepath.Base(filePath))
				layout := "20060102150405"

				if len(fileName) >= len(layout)+len(extension) {
					destinationName, err := time.Parse(layout, fileName[len(fileName)-len(extension)-len(layout):len(fileName)-len(extension)])

					if err == nil {
						screenshot := games.Screenshot{Path: filePath, DestinationName: destinationName.Format(games.DatetimeFormat) + extension}
						userGames = addScreenshotToGame(userGames, gameName, screenshot)
					} else {
						log.Printf("File doesn't follow datetime convention: %s. (%s) skipping...", filePath, err)
					}
				} else {
					log.Printf("File doesn't follow datetime convention: %s, skipping...", filePath)
				}
			}

			return nil
		})
	if err != nil {
		log.Panic(err)
	}
	return userGames
}
