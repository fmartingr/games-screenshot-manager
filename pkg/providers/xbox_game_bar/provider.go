package xbox_game_bar

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/exif"
	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
)

const (
	Name           = "xbox-game-bar"
	platformName   = "PC"
	dateTimeLayout = "2006:01:02 15:04:05"
)

type dvrMetadata struct {
	StartTime time.Time `json:"startTime"`
}

type XboxGameBarProvider struct {
	logger *logrus.Entry
}

func (p *XboxGameBarProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var userGames []*models.Game

	path := helpers.ExpandUser(options.InputPath)

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error reading from path %s: %s", options.InputPath, err)
	}

	games := make(map[string]*models.Game)

	for _, file := range files {
		fullPath := filepath.Join(path, file.Name())

		if strings.Contains(file.Name(), ".png") || strings.Contains(file.Name(), ".mp4") {
			tags, err := exif.GetTags(fullPath)
			if err != nil {
				p.logger.Errorf("err: %s", err)
				continue
			}

			titleTag := "MicrosoftGameDVRTitle"
			if strings.Contains(file.Name(), ".mp4") {
				titleTag = "Title"
			}

			gameName := tags[titleTag]

			game, exists := games[tags[titleTag]]
			if !exists {
				game = &models.Game{
					ID:       slug.Make(gameName),
					Name:     gameName,
					Platform: platformName,
					Provider: Name,
				}
				games[tags[titleTag]] = game
			}

			var destinationName string

			if strings.Contains(file.Name(), ".png") {
				metadataTag := "MicrosoftGameDVRExtended"
				metadataString, exists := tags[metadataTag]
				if !exists {
					p.logger.Warnf("no metadata found for %s", file.Name())
				}
				var metadata dvrMetadata
				if err := json.Unmarshal([]byte(metadataString), &metadata); err != nil {
					p.logger.Errorf("error parsing metadata for %s: %s", file.Name(), err)
				}

				destinationName = metadata.StartTime.Format(models.DatetimeFormat) + ".png"
			} else {
				mediaCreateTag := "MediaCreateDate"
				mediaCreateString, exists := tags[mediaCreateTag]
				if !exists {
					p.logger.Warnf("no media creation time found for %s", file.Name())
					continue
				}

				mediaCreationTime, err := time.Parse(dateTimeLayout, mediaCreateString)
				if err != nil {
					p.logger.Warnf("error parsing media creation time for %s: %s", file.Name(), err)
					continue
				}

				destinationName = mediaCreationTime.Format(models.DatetimeFormat) + ".mp4"
			}

			game.Screenshots = append(game.Screenshots, models.NewScreenshot(path+"/"+file.Name(), destinationName))
		}
	}

	for _, g := range games {
		userGames = append(userGames, g)
	}

	return userGames, nil
}

func NewXboxGameGarProvider(logger *logrus.Logger, cache models.Cache) models.Provider {
	return &XboxGameBarProvider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
