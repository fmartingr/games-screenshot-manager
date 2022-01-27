package steam

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/sirupsen/logrus"
)

const Name = "steam"
const gameListURL = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"
const baseGameHeaderURL = "https://cdn.cloudflare.steamstatic.com/steam/apps/%d/header.jpg"

var errGameIDNotFound = errors.New("game ID not found")

type SteamApp struct {
	AppID uint64 `json:"appid"`
	Name  string `json:"name"`
}

type SteamAppList struct {
	Apps []SteamApp `json:"apps"`
}

func (appList SteamAppList) FindID(id string) (result SteamApp, err error) {
	for _, game := range appList.Apps {
		uintGameID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return result, fmt.Errorf("error parsing game ID: %s", err)
		}
		if game.AppID == uintGameID {
			return game, nil
		}
	}
	return result, errGameIDNotFound
}

type SteamAppListResponse struct {
	AppList SteamAppList `json:"applist"`
}

type SteamProvider struct {
	logger *logrus.Entry
}

func (p *SteamProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	basePath, err := getBasePathForOS()
	if err != nil {
		return nil, fmt.Errorf("error getting steam's base path: %s", err)
	}

	var localGames []*models.Game
	c := make(chan SteamAppList)
	go getSteamAppList(p.logger, c)

	users, err := guessUsers(basePath)
	if err != nil {
		return nil, fmt.Errorf("error getting users: %s", err)
	}

	p.logger.Debugf("Found %d users", len(users))

	steamApps := <-c

	if len(steamApps.Apps) == 0 {
		return nil, fmt.Errorf("coulnd't get steam app list")
	}

	for _, userID := range users {
		userGames, err := getGamesFromUser(basePath, userID)
		if err != nil {
			p.logger.Errorf("error retrieving user's %s games: %s", userID, err)
			continue
		}
		for _, userGameID := range userGames {
			steamGame, err := steamApps.FindID(userGameID)
			if err != nil {
				p.logger.Errorf("Steam game ID not found: %s", userGameID)
			}
			p.logger.WithField("userID", userID).Debugf("Found game: %s", steamGame.Name)
			userGame := models.NewGame(userGameID, steamGame.Name, "PC", Name)

			userGame.CoverURL = fmt.Sprintf(baseGameHeaderURL, steamGame.AppID)

			if err := getScreenshotsForGame(basePath, userID, &userGame); err != nil {
				p.logger.Errorf("error getting screenshots: %s", err)
			}
			localGames = append(localGames, &userGame)
		}
	}
	return localGames, nil
}

func NewSteamProvider(logger *logrus.Logger) models.Provider {
	return &SteamProvider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
