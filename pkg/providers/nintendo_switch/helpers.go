package nintendo_switch

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"strings"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
)

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

	log.Printf("Updated Nintendo Switch game list. Found %d games.", len(switchGameList))

	return switchGameList
}

func addScreenshotToGame(userGames []*models.Game, switchGame SwitchGame, screenshot models.Screenshot) []*models.Game {
	var foundGame *models.Game
	for gameIndex, game := range userGames {
		if game.ID == switchGame.EncryptedGameID {
			foundGame = game
			userGames[gameIndex].Screenshots = append(userGames[gameIndex].Screenshots, screenshot)
		}
	}

	if foundGame == nil {
		foundGame := models.Game{Name: switchGame.Name, ID: switchGame.EncryptedGameID, Platform: platformName, Provider: platformName}
		foundGame.Screenshots = append(foundGame.Screenshots, screenshot)
		userGames = append(userGames, &foundGame)
	}

	return userGames
}
