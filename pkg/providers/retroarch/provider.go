// RetroArch screenshot provider

// Notes:
// This provider only works if the following retroarch configuration is set:
// screenshots_in_content_dir = "true"
// auto_screenshot_filename = "true"
// This way the screenshots will be stored in the same folders as the games
// We will read the playlists from retroarch to determine the Platforms and games
// from there, and screenshots will be extracted from the content folders, so you can
// sort your games the way you like most, but screenshots need to be renamed
// by retroarch for us to parse them properly.

package retroarch

import (
	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/sirupsen/logrus"
)

const Name = "retroarch"

const libretroCoverURLBase = "http://thumbnails.libretro.com/"
const datetimeLayout = "060102-150405"

type RetroArchProvider struct {
	logger *logrus.Entry
}

func (p *RetroArchProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var userGames []*models.Game

	playlists, err := readPlaylists(p.logger, options.InputPath)
	if err != nil {
		return nil, err
	}

	for playlistName := range playlists {
		for _, item := range playlists[playlistName].Items {
			screenshots, err := findScreenshotsForGame(p.logger, item)
			if err != nil {
				p.logger.Errorf("Error retrieving game screenshots: %s", err)
				continue
			}

			userGames = append(userGames, &models.Game{
				Platform:    cleanPlatformName(playlistName),
				Name:        cleanGameName(item.Label),
				Provider:    Name,
				Screenshots: screenshots,
				CoverURL:    formatLibretroBoxartURL(playlistName, item.Label),
			})
		}
	}

	return userGames, nil
}

func NewRetroArchProvider(logger *logrus.Logger) models.Provider {
	return &RetroArchProvider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
