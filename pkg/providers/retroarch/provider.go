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
)

const Name = "retroarch"

const libretroCoverURLBase = "http://thumbnails.libretro.com/"
const datetimeLayout = "060102-150405"

type RetroArchProvider struct {
}

func (p *RetroArchProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var userGames []*models.Game

	playlists := readPlaylists(options.InputPath)

	for playlistName := range playlists {
		for _, item := range playlists[playlistName].Items {
			userGames = append(userGames, &models.Game{
				Platform:    cleanPlatformName(playlistName),
				Name:        cleanGameName(item.Label),
				Provider:    Name,
				Screenshots: findScreenshotsForGame(item),
				CoverURL:    formatLibretroBoxartURL(playlistName, item.Label),
			})
		}
	}

	return userGames, nil
}

func NewRetroArchProvider() *RetroArchProvider {
	return &RetroArchProvider{}
}
