package playstation4

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/barasher/go-exiftool"
	"github.com/fmartingr/games-screenshot-manager/internal/exif"
	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/sirupsen/logrus"
)

const (
	ID             = "ps4"
	Name           = "playstation-4"
	platformName   = "PlayStation 4"
	exifTimeLayout = "2006:01:02 15:04:05-07:00"
	exifTimeTag    = "FileModifyDate"
)

type Playstation4Provider struct {
	logger *logrus.Entry
}

func (p *Playstation4Provider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var userGames []*models.Game

	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, fmt.Errorf("error intializing exiftool: %w", err)
	}
	defer et.Close()

	err = filepath.Walk(options.InputPath,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				p.logger.WithField("path", filePath).WithError(err).Error()
				return err
			}

			if !info.IsDir() {
				var destinationName string
				gameName := filepath.Base(filepath.Dir(filePath))
				fileName := filepath.Base(filePath)
				extension := filepath.Ext(filepath.Base(filePath))
				layout := "20060102150405"

				if extension == ".jpg" {
					exifTags, err := exif.GetTags(et, filePath)
					if err != nil {
						p.logger.WithError(err).Errorf("Error getting EXIF tags from %s", filePath)
						return nil
					}

					fileDate, err := time.Parse(exifTimeLayout, exifTags[exifTimeTag])
					if err != nil {
						p.logger.Warnf("error parsing media creation time %s for %s: %s", exifTags[exifTimeTag], filePath, err)
						return nil
					}
					destinationName = fileDate.Format(models.DatetimeFormat)
				} else if extension == ".mp4" {
					if len(fileName) >= len(layout)+len(extension) {
						videoDatetime, err := time.Parse(layout, fileName[len(fileName)-len(extension)-len(layout):len(fileName)-len(extension)])

						if err == nil {
							destinationName = videoDatetime.Format(models.DatetimeFormat)
						} else {
							p.logger.WithError(err).Warnf("File %s does not follow datetime convention, skipping.", fileName)
							return nil
						}
					} else {
						p.logger.Warnf("File %s does not follow datetime convention, skipping.", fileName)
						return nil
					}
				}

				screenshot := models.Screenshot{Path: filePath, DestinationName: destinationName + extension}
				userGames = models.AddScreenshotToGame(Name, userGames, gameName, screenshot)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}
	return userGames, nil
}

func NewPlaystation4Provider(logger *logrus.Logger, cache models.Cache) models.Provider {
	return &Playstation4Provider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
