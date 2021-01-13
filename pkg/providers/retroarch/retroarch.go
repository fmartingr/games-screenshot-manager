package retroarch

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
)

const providerName = "retroarch"

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

func readPlaylists(playlistsPath string) map[string]RetroArchPlaylist {
	//var result []RetroArchPlaylist
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
				fmt.Printf("%s\n", path.Join(playlistsPath, file.Name()))
				source, errOpen := os.Open(path.Join(playlistsPath, file.Name()))
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

func addScreenshotToGame(userGames []games.Game, platformName string, gameName string, screenshot games.Screenshot) []games.Game {
	var foundGame games.Game
	for gameIndex, game := range userGames {
		if game.Name == gameName {
			foundGame = game
			userGames[gameIndex].Screenshots = append(userGames[gameIndex].Screenshots, screenshot)
		}
	}

	// Game not found
	if foundGame.Name == "" {
		foundGame := games.Game{Name: gameName, ID: gameName, Platform: platformName, Provider: providerName}
		foundGame.Screenshots = append(foundGame.Screenshots, screenshot)
		userGames = append(userGames, foundGame)
	}

	return userGames
}

func GetGames(inputPath string) []games.Game {
	var userGames []games.Game

	playlists := readPlaylists(inputPath)

	for playlistName := range playlists {
		fmt.Println(playlistName)
	}

	err := filepath.Walk(inputPath,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Ignore state thumbnails if present
			// if !info.IsDir() && strings.Contains(info.Name(), ".png") && !strings.Contains(info.Name(), ".state.") {
			// 	platformName := filepath.Base(filepath.Dir(filePath))
			// 	fileName := filepath.Base(filePath)
			// 	extension := filepath.Ext(filepath.Base(filePath))
			// 	layout := "060102-150405"
			// 	gameName := fileName[0 : len(fileName)-(len(layout)+len(extension))-1]
			// 	screenshotDate, err := time.Parse(layout, fileName[len(fileName)-len(extension)-len(layout):len(fileName)-len(extension)])
			// 	if err == nil {
			// 		screenshot := games.Screenshot{Path: filePath, DestinationName: screenshotDate.Format(games.DatetimeFormat) + extension}
			// 		userGames = addScreenshotToGame(userGames, platformName, gameName, screenshot)
			// 	}
			// }

			return nil
		})
	if err != nil {
		log.Panic(err)
	}
	return userGames
}
