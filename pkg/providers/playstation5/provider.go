package playstation5

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/sirupsen/logrus"
)

const (
	Name                   = "playstation-5"
	platformName           = "PlayStation 5"
	filenameDatetimeLayout = "20060102150405"
)

type Playstation5Provider struct {
	logger *logrus.Entry
}

func (p *Playstation5Provider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var userGames []*models.Game

	err := filepath.Walk(options.InputPath,
		func(filePath string, info os.FileInfo, err error) error {
			log := p.logger.WithField("file_path", filePath)
			if err != nil {
				log.WithError(err).Error()
				return err
			}

			if !info.IsDir() {
				var destinationName string
				gameName := filepath.Base(filepath.Dir(filePath))
				filename := filepath.Base(filePath)
				extension := filepath.Ext(filepath.Base(filePath))

				// Ignore hidden files
				if strings.HasPrefix(filename, ".") {
					return nil
				}

				if extension == ".jpg" || extension == ".webm" {
					parts := strings.Split(strings.TrimSuffix(filename, extension), "_")
					datetime, err := time.Parse(filenameDatetimeLayout, parts[1])
					if err != nil {
						log.WithError(err).Warn("error parsing datetime from filename")
						return nil
					}

					destinationName = datetime.Format(models.DatetimeFormat)
				}

				screenshot := models.Screenshot{Path: filePath, DestinationName: destinationName + extension}
				userGames = models.AddScreenshotToGame(platformName, userGames, gameName, screenshot)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}
	return userGames, nil
}

func NewPlaystation5Provider(logger *logrus.Logger, cache models.Cache) models.Provider {
	return &Playstation5Provider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
