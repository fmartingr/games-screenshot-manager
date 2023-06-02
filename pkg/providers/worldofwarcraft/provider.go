package worldofwarcraft

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/sirupsen/logrus"
)

const (
	Name           = "worldofwarcraft"
	platformName   = "PC"
	dateTimeLayout = "010206_150405"
	filenamePrefix = "WoWScrnShot_"
	extension      = ".jpg"
)

type WorldOfWarcraftProvider struct {
	logger *logrus.Entry
}

func (p *WorldOfWarcraftProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	game := models.NewGame("wow", "World of Warcraft", platformName, Name)
	var path string

	if options.InputPath != "" {
		path = helpers.ExpandUser(options.InputPath)
	} else if runtime.GOOS == "windows" {
		path = "C:/Program Files (x86)/World of Warcraft/_retail_/Screenshots"
	} else if runtime.GOOS == "darwin" {
		path = "/Applications/World of Warcraft/_retail_/Screenshots"
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error reading from path %s: %s", options.InputPath, err)
	}

	for _, file := range files {
		fullPath := filepath.Join(path, file.Name())

		if strings.Contains(file.Name(), extension) {
			var destinationName string

			parts := strings.SplitN(file.Name(), "_", 2)
			dateStr := strings.Replace(parts[1], extension, "", 1)

			filenameDate, err := time.Parse(dateTimeLayout, dateStr)
			if err != nil {
				p.logger.Warnf("error parsing media creation time for %s: %s", file.Name(), err)
				continue
			}

			destinationName = filenameDate.Format(models.DatetimeFormat) + extension

			game.Screenshots = append(game.Screenshots, models.NewScreenshot(fullPath, destinationName))
		}
	}

	return []*models.Game{&game}, nil
}

func NewWorldOfWarcraftProvider(logger *logrus.Logger, cache models.Cache) models.Provider {
	return &WorldOfWarcraftProvider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
