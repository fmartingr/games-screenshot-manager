package playstation4

import (
	"os"
	"path/filepath"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/sirupsen/logrus"
)

const Name = "playstation-4"
const platformName = "PlayStation 4"

type Playstation4Provider struct {
	logger *logrus.Entry
}

func (p *Playstation4Provider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var userGames []*models.Game

	err := filepath.Walk(options.InputPath,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				var destinationName string
				gameName := filepath.Base(filepath.Dir(filePath))
				fileName := filepath.Base(filePath)
				extension := filepath.Ext(filepath.Base(filePath))
				layout := "20060102150405"

				if extension == ".jpg" {
					fileDescriptor, errFileDescriptor := os.Open(filePath)
					if errFileDescriptor != nil {
						p.logger.Warnf("Couldn't open file %s: %s", fileName, errFileDescriptor)
						return nil
					}
					exifData, errExifData := exif.Decode(fileDescriptor)
					if errExifData != nil {
						p.logger.Errorf("Decoding EXIF data from %s: %s", filePath, errExifData)
						return nil
					}
					defer fileDescriptor.Close()

					exifDateTime, _ := exifData.DateTime()
					destinationName = exifDateTime.Format(models.DatetimeFormat)

				} else if extension == ".mp4" {
					if len(fileName) >= len(layout)+len(extension) {
						videoDatetime, err := time.Parse(layout, fileName[len(fileName)-len(extension)-len(layout):len(fileName)-len(extension)])

						if err == nil {
							destinationName = videoDatetime.Format(models.DatetimeFormat)
						} else {
							p.logger.Warnf("File %s does not follow datetime convention, skipping.", fileName, err)
							return nil
						}
					} else {
						p.logger.Warnf("File %s does not follow datetime convention, skipping.", fileName)
						return nil
					}
				}

				screenshot := models.Screenshot{Path: filePath, DestinationName: destinationName + extension}
				addScreenshotToGame(userGames, gameName, screenshot)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}
	return userGames, nil
}

func NewPlaystation4Provider(logger *logrus.Logger) models.Provider {
	return &Playstation4Provider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
