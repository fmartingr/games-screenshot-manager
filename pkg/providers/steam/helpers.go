package steam

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
)

func getBasePathForOS() string {
	var path string
	switch runtime.GOOS {
	case "darwin":
		path = helpers.ExpandUser("~/Library/Application Support/Steam")
	case "linux":
		path = helpers.ExpandUser("~/.local/share/Steam")
	case "windows":
		path = "C:\\Program Files (x86)\\Steam"
	default:
		log.Panic("Unsupported OS: ", runtime.GOOS)
	}
	return path
}

func getSteamAppList(c chan SteamAppList) {
	log.Println("Updating steam game list...")
	response, err := helpers.DoRequest("GET", gameListURL)
	if err != nil {
		panic(err)
	}

	if response.Body != nil {
		defer response.Body.Close()
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}

	steamListResponse := SteamAppListResponse{}
	jsonErr := json.Unmarshal(body, &steamListResponse)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	log.Printf("Updated Steam game list. Found %d apps.", len(steamListResponse.AppList.Apps))

	c <- steamListResponse.AppList
}

func guessUsers() []string {
	var users []string
	var path string = filepath.Join(getBasePathForOS(), "userdata")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			if _, err := strconv.ParseInt(file.Name(), 10, 64); err == nil {
				log.Printf("Found local install Steam user: %s", file.Name())
				users = append(users, file.Name())
			}
		}
	}
	return users
}

func getGamesFromUser(user string) []string {
	log.Println("Getting Steam games for user: " + user)
	var userGames []string
	var path string = filepath.Join(getBasePathForOS(), "userdata", user, "760", "remote")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			// len(file.Name()) == 20 -> Custom added Game to steam
			userGames = append(userGames, file.Name())
		}
	}

	return userGames
}

func getScreenshotsForGame(user string, game *models.Game) {
	path := filepath.Join(getBasePathForOS(), "userdata", user, "/760/remote/", game.ID, "screenshots")
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), ".jpg") {
			game.Screenshots = append(game.Screenshots, models.NewScreenshotWithoutDestination(path+"/"+file.Name()))
		}
	}
}
