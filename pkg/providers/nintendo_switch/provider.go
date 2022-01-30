package nintendo_switch

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/sirupsen/logrus"
)

const Name = "nintendo-switch"
const platformName = "Nintendo Switch"
const gameListURL = "https://fmartingr.github.io/switch-games-json/switch_id_names.json"

type NintendoSwitchProvider struct {
	logger *logrus.Entry
}

func (p *NintendoSwitchProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	switchGames, err := getSwitchGameList()
	if err != nil {
		p.logger.Error(err)
		return nil, err
	}

	var userGames []*models.Game

	err = filepath.Walk(options.InputPath,
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
					p.logger.Errorf("Could not parse filename '%s': %s", filename, err)
				}

				screenshot := models.Screenshot{Path: path, DestinationName: destinationName.Format(models.DatetimeFormat) + extension}
				userGames = addScreenshotToGame(userGames, switchGame, screenshot)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return userGames, nil
}

func NewNintendoSwitchProvider(logger *logrus.Logger, cache models.Cache) models.Provider {
	return &NintendoSwitchProvider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
