package playstation4

import "github.com/fmartingr/games-screenshot-manager/internal/models"

func addScreenshotToGame(userGames []*models.Game, gameName string, screenshot models.Screenshot) []*models.Game {
	var foundGame *models.Game
	for gameIndex, game := range userGames {
		if game.Name == gameName {
			foundGame = game
			userGames[gameIndex].Screenshots = append(userGames[gameIndex].Screenshots, screenshot)
		}
	}

	if foundGame == nil {
		foundGame := models.Game{Name: gameName, ID: gameName, Platform: platformName, Provider: platformName}
		foundGame.Screenshots = append(foundGame.Screenshots, screenshot)
		userGames = append(userGames, &foundGame)
	}

	return userGames
}
