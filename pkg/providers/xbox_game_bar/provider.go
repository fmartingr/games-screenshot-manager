package xbox_game_bar

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/sirupsen/logrus"
)

const Name = "xbox-game-bar"
const platformName = "Xbox Game Bar"

type XboxGameBarProvider struct {
	logger *logrus.Entry
}

func (p *XboxGameBarProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var userGames []*models.Game

	path := helpers.ExpandUser(options.InputPath)
	log.Print(path)
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error reading from path %s: %s", options.InputPath, err)
	}

	for _, file := range files {
		fullPath := filepath.Join(path, file.Name())
		log.Println(fullPath)

		if strings.Contains(file.Name(), ".png") || strings.Contains(file.Name(), ".mp4") {
			if err := getExifTags(fullPath); err != nil {
				p.logger.Errorf("err: %s", err)
			}

			// game.Screenshots = append(game.Screenshots, models.NewScreenshotWithoutDestination(path+"/"+file.Name()))
		}
	}

	return userGames, nil
}

func NewXboxGameGarProvider(logger *logrus.Logger, cache models.Cache) models.Provider {
	return &XboxGameBarProvider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
