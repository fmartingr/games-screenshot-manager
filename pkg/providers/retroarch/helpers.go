package retroarch

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/sirupsen/logrus"
)

type retroArchPlaylistItem struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	CorePath string `json:"core_path"`
	CoreName string `json:"core_name"`
	CRC32    string `json:"crc32"`
	DBName   string `json:"db_name"`
}

type retroArchPlaylist struct {
	Version            string                  `json:"version"`
	DefaultCorePath    string                  `json:"default_core_path"`
	DefaultCoreName    string                  `json:"default_core_name"`
	LabelDisplayMode   int                     `json:"label_display_mode"`
	RightThumbnailMode int                     `json:"right_thumbnail_mode"`
	LeftThumbnailMode  int                     `json:"left_thumbnail_mode"`
	SortMode           int                     `json:"sort_mode"`
	Items              []retroArchPlaylistItem `json:"items"`
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

func readPlaylists(logger *logrus.Entry, playlistsPath string) (map[string]retroArchPlaylist, error) {
	var result = make(map[string]retroArchPlaylist)
	playlistsPath = helpers.ExpandUser(playlistsPath)
	if _, err := os.Stat(playlistsPath); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(playlistsPath)
		if err != nil {
			return result, fmt.Errorf("error reading playlist directory: %s", err)
		}

		for _, file := range files {
			if strings.Contains(file.Name(), ".lpl") {
				var item retroArchPlaylist
				source, errOpen := os.Open(filepath.Join(playlistsPath, file.Name()))
				if errOpen != nil {
					logger.Errorf("Error reading playlist %s: %s", file.Name(), errOpen)
					continue
				}
				fileContents, errReadContent := ioutil.ReadAll(source)
				if errReadContent != nil {
					logger.Errorf("Error reading contents of %s: %s", file.Name(), err)
					continue
				}

				errUnmarshal := json.Unmarshal(fileContents, &item)
				if errUnmarshal != nil {
					logger.Errorf("Error formatting %s: %s", file.Name(), errUnmarshal)
					continue
				}
				result[strings.Replace(file.Name(), ".lpl", "", 1)] = item
				source.Close()
			}
		}
	}
	return result, nil
}

func findScreenshotsForGame(logger *logrus.Entry, item retroArchPlaylistItem) ([]models.Screenshot, error) {
	var result []models.Screenshot
	filePath := filepath.Dir(item.Path)
	fileName := strings.Replace(filepath.Base(item.Path), filepath.Ext(item.Path), "", 1)
	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		return nil, err
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
				screenshotDestinationName = file.ModTime().Format(models.DatetimeFormat) + "_retroachievement-" + achievementID + extension
			} else {
				screenshotDate, err := time.Parse(datetimeLayout, file.Name()[len(file.Name())-len(extension)-len(datetimeLayout):len(file.Name())-len(extension)])
				if err == nil {
					screenshotDestinationName = screenshotDate.Format(models.DatetimeFormat) + extension
				} else {
					logger.Errorf("Error formatting screenshot %s: %s", file.Name(), err)
					continue
				}
			}

			result = append(result, models.Screenshot{Path: filepath.Join(filePath, file.Name()), DestinationName: screenshotDestinationName})
		}
	}
	return result, nil
}
