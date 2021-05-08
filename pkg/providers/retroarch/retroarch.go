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
	"encoding/json"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers"
)

const providerName = "retroarch"

const libretroCoverURLBase = "http://thumbnails.libretro.com/"
const datetimeLayout = "060102-150405"

type RetroArchPlaylistItem struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	CorePath string `json:"core_path"`
	CoreName string `json:"core_name"`
	CRC32    string `json:"crc32"`
	DBName   string `json:"db_name"`
}

type RetroArchPlaylist struct {
	Version            string                  `json:"version"`
	DefaultCorePath    string                  `json:"default_core_path"`
	DefaultCoreName    string                  `json:"default_core_name"`
	LabelDisplayMode   int                     `json:"label_display_mode"`
	RightThumbnailMode int                     `json:"right_thumbnail_mode"`
	LeftThumbnailMode  int                     `json:"left_thumbnail_mode"`
	SortMode           int                     `json:"sort_mode"`
	Items              []RetroArchPlaylistItem `json:"items"`
}

func formatLibretroBoxartURL(platform string, game string) string {
	return libretroCoverURLBase + url.PathEscape(path.Join(platform, "Named_Boxarts", game)) + ".png"
}

func cleanPlatformName(platformName string) string {
	// Removes the "Nintendo - " portion of nintendo systems
	// Could probably be extended to others (Sony, Microsoft) by removing all until the first hyphen
	if strings.Contains(platformName, "Nintendo") {
		return strings.Replace(platformName, "Nintendo - ", "", 1)
	}
	return platformName
}

func cleanGameName(gameName string) string {
	splits := strings.Split(gameName, "(")
	return splits[0]
}

func readPlaylists(playlistsPath string) map[string]RetroArchPlaylist {
	var result = make(map[string]RetroArchPlaylist)
	playlistsPath = helpers.ExpandUser(playlistsPath)
	if _, err := os.Stat(playlistsPath); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(playlistsPath)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			if strings.Contains(file.Name(), ".lpl") {
				var item RetroArchPlaylist
				source, errOpen := os.Open(filepath.Join(playlistsPath, file.Name()))
				if errOpen != nil {
					log.Printf("[ERROR] Error reading playlist %s: %s", file.Name(), errOpen)
					continue
				}
				fileContents, errReadContent := ioutil.ReadAll(source)
				if errReadContent != nil {
					log.Printf("[ERROR] Reading contents of %s: %s", file.Name(), err)
					continue
				}

				errUnmarshal := json.Unmarshal(fileContents, &item)
				if errUnmarshal != nil {
					log.Printf("[ERROR] Formatting %s: %s", file.Name(), errUnmarshal)
					continue
				}
				result[strings.Replace(file.Name(), ".lpl", "", 1)] = item
				source.Close()
			}
		}
	}
	return result
}

func findScreenshotsForGame(item RetroArchPlaylistItem) []providers.Screenshot {
	var result []providers.Screenshot
	filePath := filepath.Dir(item.Path)
	fileName := strings.Replace(filepath.Base(item.Path), filepath.Ext(item.Path), "", 1)
	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		// Get all screenshots for the game, excluding state screenshots
		if !file.IsDir() && strings.Contains(file.Name(), fileName) && strings.Contains(file.Name(), ".png") {
			if strings.Contains(file.Name(), ".state.") {
				// Ignore state screenshots
				continue
			}

			extension := filepath.Ext(file.Name())
			var screenshotDestinationName string

			// Handle autoamtic achievement screenshots: get datetime from modtime
			if strings.Contains(file.Name(), "-cheevo-") {
				filenameParts := strings.Split(file.Name(), "-")
				achievementID := strings.Replace(filenameParts[len(filenameParts)-1], extension, "", 1)
				screenshotDestinationName = file.ModTime().Format(providers.DatetimeFormat) + "_retroachievement-" + achievementID + extension
			} else {
				screenshotDate, err := time.Parse(datetimeLayout, file.Name()[len(file.Name())-len(extension)-len(datetimeLayout):len(file.Name())-len(extension)])
				if err == nil {
					screenshotDestinationName = screenshotDate.Format(providers.DatetimeFormat) + extension
				} else {
					log.Printf("[error] Formatting screenshot %s: %s", file.Name(), err)
					continue
				}
			}

			result = append(result, providers.Screenshot{Path: filepath.Join(filePath, file.Name()), DestinationName: screenshotDestinationName})
		}
	}
	return result
}

func GetGames(cliOptions providers.ProviderOptions) []providers.Game {
	var userGames []providers.Game

	playlists := readPlaylists(*cliOptions.InputPath)

	for playlistName := range playlists {
		for _, item := range playlists[playlistName].Items {
			var cover providers.Screenshot

			if *cliOptions.DownloadCovers {
				coverURL := formatLibretroBoxartURL(playlistName, item.Label)
				boxartPath, err := helpers.DownloadURLIntoTempFile(coverURL)
				if err == nil {
					cover = providers.Screenshot{Path: boxartPath, DestinationName: ".cover"}
				} else {
					log.Printf("[error] Error downloading cover for %s: %s", item.Label, err)
				}
			}

			userGames = append(userGames, providers.Game{
				Platform:    cleanPlatformName(playlistName),
				Name:        cleanGameName(item.Label),
				Provider:    providerName,
				Screenshots: findScreenshotsForGame(item),
				Cover:       cover,
			})
		}
	}

	return userGames
}
