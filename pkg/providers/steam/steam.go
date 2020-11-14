package steam

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
)

const providerName string = "steam"

type SteamApp struct {
	AppID uint64 `json:"appid"`
	Name  string `json:"name"`
}

type SteamAppList struct {
	Apps []SteamApp `json:"apps"`
}

func (appList SteamAppList) FindID(id uint64) (SteamApp, error) {
	GameIDNotFound := errors.New("Game ID not found")
	for _, game := range appList.Apps {
		if game.AppID == id {
			return game, nil
		}
	}
	return SteamApp{}, GameIDNotFound
}

type SteamAppListResponse struct {
	AppList SteamAppList `json:"applist"`
}

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

func GetSteamAppsList(c chan SteamAppList) {
	log.Println("Updating steam game list...")
	steamGetAppListURL, _ := url.Parse("https://api.steampowered.com/ISteamApps/GetAppList/v2/")
	request := http.Request{
		Method: "GET",
		URL:    steamGetAppListURL,
		Header: map[string][]string{
			"User-Agent": {"games-screenshot-manager/0.0.1"},
		},
		ProtoMajor: 2,
		ProtoMinor: 1,
	}
	response, err := http.DefaultClient.Do(&request)
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

	log.Printf("Updated Steam game list. Found %d games.", len(steamListResponse.AppList.Apps))

	c <- steamListResponse.AppList
}

func GuessUsers() []string {
	var users []string
	var path string = filepath.Join(getBasePathForOS(), "userdata")
	log.Println(path)

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

func GetGamesFromUser(user string) []uint64 {
	log.Println("Getting Steam games for user: " + user)
	var userGames []uint64
	var path string = filepath.Join(getBasePathForOS(), "userdata", user, "760", "remote")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			// len(file.Name()) == 20 -> Custom added Game to steam
			gameID, err := strconv.ParseUint(file.Name(), 10, 64)
			if err == nil {
				userGames = append(userGames, gameID)
			}
		}
	}

	return userGames
}

func GetScreenshotsForGame(user string, game *games.Game) {
	path := filepath.Join(getBasePathForOS(), "userdata", user, "/760/remote/", strconv.FormatUint(game.ID, 10), "screenshots")
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), ".jpg") {
			game.Screenshots = append(game.Screenshots, games.Screenshot{Path: path + "/" + file.Name()})
			// log.Printf("Found screenshot for user %s and game %d: %s", user, game.ID, path+"/"+file.Name())
		}
	}

	if len(game.Screenshots) > 0 {
		log.Printf("Found %d screenshots", len(game.Screenshots))
	}
}

func GetGames() []games.Game {
	var localGames []games.Game
	c := make(chan SteamAppList)
	go GetSteamAppsList(c)
	users := GuessUsers()
	steamApps := <-c
	for _, userID := range users {
		userGames := GetGamesFromUser(userID)
		for _, userGameID := range userGames {
			steamGame, err := steamApps.FindID(userGameID)
			if err != nil {
				log.Print("[ERROR] Steam game ID not found: ", userGameID)
			}
			userGame := games.Game{ID: userGameID, Name: steamGame.Name, Provider: providerName, Platform: "PC"}
			log.Printf("Found Steam game for user %s: %s (%d)", userID, userGame.Name, userGame.ID)
			GetScreenshotsForGame(userID, &userGame)
			localGames = append(localGames, userGame)
		}
	}
	return localGames
}
