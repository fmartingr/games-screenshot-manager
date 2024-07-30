package guild_wars_2

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/barasher/go-exiftool"
	"github.com/fmartingr/games-screenshot-manager/internal/exif"
	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/sirupsen/logrus"
)

const (
	ID             = "gw2"
	Name           = "Guild Wars 2"
	platformName   = "PC"
	extension      = ".jpg"
	exifTimeLayout = "2006:01:02 15:04:05-07:00"
	exifTimeTag    = "FileModifyDate"
)

type GuildWars2Provider struct {
	logger *logrus.Entry
}

func (p *GuildWars2Provider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	game := models.NewGame(ID, Name, platformName, Name)
	var path string

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error getting user home directory: %w", err)
	}

	if options.InputPath != "" {
		path = helpers.ExpandUser(options.InputPath)
	} else if runtime.GOOS == "windows" {
		path = userHomeDir + `\Documents\Guild Wars 2\Screens`
	} else {
		return nil, fmt.Errorf("For non-windows systems the -input-path flag must be provided")
	}

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", path)
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error reading from path %s: %s", options.InputPath, err)
	}

	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, fmt.Errorf("error intializing exiftool: %w", err)
	}
	defer et.Close()

	for _, file := range files {
		fullPath := filepath.Join(path, file.Name())

		if strings.Contains(file.Name(), extension) {
			var destinationName string

			exifTags, err := exif.GetTags(et, fullPath)
			if err != nil {
				p.logger.Errorf("error retrieving exif tags for %s: %w", file.Name(), err)
				continue
			}

			fileDate, err := time.Parse(exifTimeLayout, exifTags[exifTimeTag])
			if err != nil {
				p.logger.Warnf("error parsing media creation time %s for %s: %s", exifTags[exifTimeTag], file.Name(), err)
				continue
			}

			destinationName = fileDate.Format(models.DatetimeFormat) + extension

			game.Screenshots = append(game.Screenshots, models.NewScreenshot(fullPath, destinationName))
		}
	}

	return []*models.Game{&game}, nil
}

func NewGuildWars2Provider(logger *logrus.Logger, cache models.Cache) models.Provider {
	return &GuildWars2Provider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
