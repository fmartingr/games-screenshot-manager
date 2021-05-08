package nintendo_switch

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers"
)

const providerName = "nintendo-switch"
const platformName = "Nintendo Switch"
const gameListURL = "https://fmartingr.github.io/switch-games-json/switch_id_names.json"

type SwitchGame struct {
	Name            string `json:"title_normalized"`
	EncryptedGameID string `json:"encrypted_game_id"`
}

func findGameByEncryptedID(gameList []SwitchGame, encryptedGameID string) SwitchGame {
	var gameFound SwitchGame = SwitchGame{EncryptedGameID: encryptedGameID}
	for _, game := range gameList {
		if strings.EqualFold(game.EncryptedGameID, encryptedGameID) {
			gameFound = game
		}
	}

	return gameFound
}

func getSwitchGameList() []SwitchGame {
	response, err := helpers.DoRequest("GET", gameListURL)
	if err != nil {
		log.Panic(err)
	}

	if response.Body != nil {
		defer response.Body.Close()
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Panic(err)
	}

	switchGameList := []SwitchGame{}
	jsonErr := json.Unmarshal(body, &switchGameList)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	log.Printf("Updated Nintendo Switch game list. Found %d providers.", len(switchGameList))

	return switchGameList
}

func addScreenshotToGame(userGames []providers.Game, switchGame SwitchGame, screenshot providers.Screenshot) []providers.Game {
	var foundGame providers.Game
	for gameIndex, game := range userGames {
		if game.ID == switchGame.EncryptedGameID {
			foundGame = game
			userGames[gameIndex].Screenshots = append(userGames[gameIndex].Screenshots, screenshot)
		}
	}

	if foundGame.ID == "" {
		foundGame := providers.Game{Name: switchGame.Name, ID: switchGame.EncryptedGameID, Platform: platformName, Provider: providerName}
		foundGame.Screenshots = append(foundGame.Screenshots, screenshot)
		userGames = append(userGames, foundGame)
	}

	return userGames
}

func GetGames(cliOptions providers.ProviderOptions) []providers.Game {
	switchGames := getSwitchGameList()
	var userGames []providers.Game

	err := filepath.Walk(*cliOptions.InputPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				filename := filepath.Base(path)
				extension := filepath.Ext(filepath.Base(path))

				filenameParsed := strings.Split(filename[:len(filename)-len(extension)], "-")
				switchGame := findGameByEncryptedID(switchGames, filenameParsed[1])

				layout := "20060102150405"
				destinationName, err := time.Parse(layout, filenameParsed[0][0:14])

				if err != nil {
					log.Panic("Could not parse filename: ", err)
				}

				screenshot := providers.Screenshot{Path: path, DestinationName: destinationName.Format(providers.DatetimeFormat) + extension}
				userGames = addScreenshotToGame(userGames, switchGame, screenshot)
			}
			return nil
		})
	if err != nil {
		log.Panic(err)
	}
	return userGames
}
