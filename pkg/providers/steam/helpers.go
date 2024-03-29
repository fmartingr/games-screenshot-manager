package steam

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/sirupsen/logrus"
)

func getBasePathForOS() (string, error) {
	var path string
	switch runtime.GOOS {
	case "darwin":
		path = helpers.ExpandUser("~/Library/Application Support/Steam")
	case "linux":
		path = helpers.ExpandUser("~/.local/share/Steam")
	case "windows":
		path = "C:\\Program Files (x86)\\Steam"
	default:
		return "", fmt.Errorf("unsupported os: %s", runtime.GOOS)
	}
	return path, nil
}

func getSteamAppList(logger *logrus.Entry, cache models.Cache, c chan SteamAppList) {
	defer close(c)
	cacheKey := "steam-applist"
	download := true
	var payload []byte

	result, err := cache.GetExpiry(cacheKey, 24*time.Hour)
	if err != nil && !errors.Is(err, models.ErrCacheKeyDontExist) {
		logger.Errorf("error retrieving cache: %s", err)
		return
	}

	if len(result) > 0 {
		download = false
		payload = []byte(result)
	}

	if download {
		response, err := helpers.DoRequest("GET", gameListURL)
		if err != nil {
			logger.Errorf("Error making request for Steam APP List: %s", err)
		}

		if response.Body != nil {
			defer response.Body.Close()
		}

		payload, err = ioutil.ReadAll(response.Body)
		if err != nil {
			logger.Errorf("Error reading steam response: %s", err)
		}
	}

	if download {
		if err := cache.Put(cacheKey, string(payload)); err != nil {
			logger.Error(err)
		}
	}

	steamListResponse := SteamAppListResponse{}
	jsonErr := json.Unmarshal(payload, &steamListResponse)
	if jsonErr != nil {
		logger.Errorf("Error unmarshalling steam's response: %s", jsonErr)
	}

	c <- steamListResponse.AppList
}

func guessUsers(basePath string) ([]string, error) {
	var users []string
	var path string = filepath.Join(basePath, "userdata")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if _, err := strconv.ParseInt(file.Name(), 10, 64); err == nil {
				users = append(users, file.Name())
			}
		}
	}
	return users, nil
}

func getGamesFromUser(basePath, user string) ([]string, error) {
	var userGames []string
	var path string = filepath.Join(basePath, "userdata", user, "760", "remote")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			// len(file.Name()) == 20 -> Custom added Game to steam
			userGames = append(userGames, file.Name())
		}
	}

	return userGames, nil
}

func getScreenshotsForGame(basePath, user string, game *models.Game) error {
	path := filepath.Join(basePath, "userdata", user, "/760/remote/", game.ID, "screenshots")
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return fmt.Errorf("error reading game screenshot path: %s", err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), ".jpg") {
			game.Screenshots = append(game.Screenshots, models.NewScreenshotWithoutDestination(path+"/"+file.Name()))
		}
	}

	return nil
}
