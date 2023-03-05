package models

import (
	"log"
	"os"
	"path/filepath"
)

const DatetimeFormat = "2006-01-02_15-04-05"

type Game struct {
	ID          string
	Name        string
	Platform    string
	Provider    string
	Screenshots []Screenshot
	Notes       string
	CoverURL    string
}

func NewGame(id, name, platform, provider string) Game {
	return Game{
		ID:       id,
		Name:     name,
		Platform: platform,
		Provider: provider,
	}
}

type Screenshot struct {
	Path            string
	DestinationName string
}

func (screenshot Screenshot) GetDestinationName() string {
	if screenshot.DestinationName != "" {
		return screenshot.DestinationName
	}
	fileStat, statErr := os.Stat(screenshot.Path)
	if statErr != nil {
		log.Fatal(statErr)
	}
	return fileStat.ModTime().Format(DatetimeFormat) + filepath.Ext(screenshot.Path)
}

func NewScreenshot(path, destinationName string) Screenshot {
	return Screenshot{
		Path:            path,
		DestinationName: destinationName,
	}
}

func NewScreenshotWithoutDestination(path string) Screenshot {
	return Screenshot{
		Path: path,
	}
}

func AddScreenshotToGame(platform string, userGames []*Game, gameName string, screenshot Screenshot) []*Game {
	var foundGame *Game
	for gameIndex, game := range userGames {
		if game.Name == gameName {
			foundGame = game
			userGames[gameIndex].Screenshots = append(userGames[gameIndex].Screenshots, screenshot)
		}
	}

	if foundGame == nil {
		foundGame := Game{Name: gameName, ID: gameName, Platform: platform, Provider: platform}
		foundGame.Screenshots = append(foundGame.Screenshots, screenshot)
		userGames = append(userGames, &foundGame)
	}

	return userGames
}
