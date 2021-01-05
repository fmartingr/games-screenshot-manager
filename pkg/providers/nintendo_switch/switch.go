package nintendo_switch

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
)

const providerName = "nintendo-switch"
const gameListURL = "https://fmartingr.github.io/switch-games-json/switch_id_names.json"

type SwitchGame struct {
	Name            string `json:"title_normalized"`
	EncryptedGameID string `json:"encrypted_game_id"`
}

func findGameByEncryptedID(gameList []SwitchGame, encryptedGameID string) SwitchGame {
	var gameFound SwitchGame = SwitchGame{EncryptedGameID: encryptedGameID}
	for _, game := range gameList {
		if strings.ToUpper(game.EncryptedGameID) == strings.ToUpper(encryptedGameID) {
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

	log.Printf("Updated Nintendo Switch game list. Found %d games.", len(switchGameList))

	return switchGameList
}

func addScreenshotToGame(userGames []games.Game, switchGame SwitchGame, screenshot games.Screenshot) []games.Game {
	var foundGame games.Game
	for gameIndex, game := range userGames {
		if game.ID == switchGame.EncryptedGameID {
			foundGame = game
			userGames[gameIndex].Screenshots = append(userGames[gameIndex].Screenshots, screenshot)
		}
	}

	if foundGame.ID == "" {
		foundGame := games.Game{Name: switchGame.Name, ID: switchGame.EncryptedGameID, Platform: "Nintendo Switch", Provider: providerName}
		foundGame.Screenshots = append(foundGame.Screenshots, screenshot)
		userGames = append(userGames, foundGame)
	}

	return userGames
}

type EShopGame struct {
	Title              string `json:"title"`
	MenuSquareCoverURL string `json:"image_url_sq_s"`
	H2X1CoverURL       string `json:"image_url_h2x1_s"`
}

func GetEshopGameList() []EShopGame {
	var result []EShopGame

	for _, region := range []string{"eu", "us", "jp"} {
		var games []EShopGame
		data, err := ioutil.ReadFile("./games_" + region + ".json")
		if err != nil {
			log.Panic(err)
		}

		errUnmarshal := json.Unmarshal(data, &games)
		if errUnmarshal != nil {
			log.Panic(err)
		}
		result = append(result, games...)
	}
	return result
}

func DownloadGameCoverForGame(eShopGames []EShopGame, gameName string) (string, error) {
	for gameIndex := range eShopGames {
		if eShopGames[gameIndex].Title == gameName {
			coverPath, err := helpers.DownloadURLIntoTempFile(eShopGames[gameIndex].MenuSquareCoverURL)
			if err != nil {
				return "", err
			} else {
				return coverPath, nil
			}
		}
	}
	return "", errors.New("Game not found")
}

func GetGames(inputPath string, downloadCovers bool) []games.Game {
	switchGames := getSwitchGameList()
	var userGames []games.Game

	if downloadCovers {
		log.Println("[info] Downloading covers for Nintendo Switch on a best-effort basis")
	}

	err := filepath.Walk(inputPath,
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

				screenshot := games.Screenshot{Path: path, DestinationName: destinationName.Format(games.DatetimeFormat) + extension}
				userGames = addScreenshotToGame(userGames, switchGame, screenshot)
			}
			return nil
		})
	if err != nil {
		log.Panic(err)
	}

	if downloadCovers {
		eshopGameList := GetEshopGameList()
		for userGameIndex := range userGames {
			coverPath, err := DownloadGameCoverForGame(eshopGameList, userGames[userGameIndex].Name)
			if err == nil {
				userGames[userGameIndex].Cover = games.Screenshot{Path: coverPath, DestinationName: ".cover"}
			} else {
				log.Printf("[info] Cover not found for game %s: %s", userGames[userGameIndex].Name, err)
			}
		}
	}

	return userGames
}
