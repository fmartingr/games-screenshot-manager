package steam

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
)

const Name = "steam"
const gameListURL = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"
const baseGameHeaderURL = "https://cdn.cloudflare.steamstatic.com/steam/apps/%d/header.jpg"

type SteamApp struct {
	AppID uint64 `json:"appid"`
	Name  string `json:"name"`
}

type SteamAppList struct {
	Apps []SteamApp `json:"apps"`
}

func (appList SteamAppList) FindID(id string) (SteamApp, error) {
	GameIDNotFound := errors.New("game ID not found")
	for _, game := range appList.Apps {
		uintGameID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			log.Panic(err)
		}
		if game.AppID == uintGameID {
			return game, nil
		}
	}
	return SteamApp{}, GameIDNotFound
}

type SteamAppListResponse struct {
	AppList SteamAppList `json:"applist"`
}

type SteamProvider struct{}

func (p *SteamProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var localGames []*models.Game
	c := make(chan SteamAppList)
	go getSteamAppList(c)
	users := guessUsers()
	steamApps := <-c
	for _, userID := range users {
		userGames := getGamesFromUser(userID)
		for _, userGameID := range userGames {
			steamGame, err := steamApps.FindID(userGameID)
			if err != nil {
				log.Print("[ERROR] Steam game ID not found: ", userGameID)
			}
			userGame := models.NewGame(userGameID, steamGame.Name, "PC", Name)

			userGame.CoverURL = fmt.Sprintf(baseGameHeaderURL, steamGame.AppID)

			log.Printf("Found Steam game for user %s: %s (%s)", userID, userGame.Name, userGame.ID)
			getScreenshotsForGame(userID, &userGame)
			localGames = append(localGames, &userGame)
		}
	}
	return localGames, nil
}

func NewSteamProvider() *SteamProvider {
	return &SteamProvider{}
}
