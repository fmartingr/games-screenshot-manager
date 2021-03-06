package playstation4

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fmartingr/games-screenshot-manager/pkg/providers"
	"github.com/rwcarlsen/goexif/exif"
)

const providerName = "playstation-4"
const platformName = "PlayStation 4"

func addScreenshotToGame(userGames []providers.Game, gameName string, screenshot providers.Screenshot) []providers.Game {
	var foundGame providers.Game
	for gameIndex, game := range userGames {
		if game.Name == gameName {
			foundGame = game
			userGames[gameIndex].Screenshots = append(userGames[gameIndex].Screenshots, screenshot)
		}
	}

	// Game not found
	if foundGame.Name == "" {
		foundGame := providers.Game{Name: gameName, ID: gameName, Platform: platformName, Provider: providerName}
		foundGame.Screenshots = append(foundGame.Screenshots, screenshot)
		userGames = append(userGames, foundGame)
	}

	return userGames
}

func GetGames(cliOptions providers.ProviderOptions) []providers.Game {
	var userGames []providers.Game

	err := filepath.Walk(*cliOptions.InputPath,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				var destinationName string
				gameName := filepath.Base(filepath.Dir(filePath))
				fileName := filepath.Base(filePath)
				extension := filepath.Ext(filepath.Base(filePath))
				layout := "20060102150405"

				if extension == ".jpg" {
					fileDescriptor, errFileDescriptor := os.Open(filePath)
					if errFileDescriptor != nil {
						log.Printf("[warning] Couldn't open file %s: %s", fileName, errFileDescriptor)
						return nil
					}
					exifData, errExifData := exif.Decode(fileDescriptor)
					if errExifData != nil {
						log.Printf("[Error] Decoding EXIF data from %s: %s", filePath, errExifData)
						return nil
					}
					defer fileDescriptor.Close()

					exifDateTime, _ := exifData.DateTime()
					destinationName = exifDateTime.Format(providers.DatetimeFormat)

				} else if extension == ".mp4" {
					if len(fileName) >= len(layout)+len(extension) {
						videoDatetime, err := time.Parse(layout, fileName[len(fileName)-len(extension)-len(layout):len(fileName)-len(extension)])

						if err == nil {
							destinationName = videoDatetime.Format(providers.DatetimeFormat)
						} else {
							log.Printf("[Warning] File does not follow datetime convention: %s. (%s) skipping...", fileName, err)
							return nil
						}
					} else {
						log.Printf("[Warning] File does not follow datetime convention: %s, skipping...", fileName)
						return nil
					}
				}

				screenshot := providers.Screenshot{Path: filePath, DestinationName: destinationName + extension}
				userGames = addScreenshotToGame(userGames, gameName, screenshot)

			}

			return nil
		})
	if err != nil {
		log.Panic(err)
	}
	return userGames
}
