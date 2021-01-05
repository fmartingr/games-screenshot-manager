package steam

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
)

const providerName string = "steam"
const gameListURL string = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"
const baseGameHeaderURL string = "https://cdn.cloudflare.steamstatic.com/steam/apps/{id}/header.jpg"

type SteamApp struct {
	AppID uint64 `json:"appid"`
	Name  string `json:"name"`
}

type SteamAppList struct {
	Apps []SteamApp `json:"apps"`
}

func (appList SteamAppList) FindID(id string) (SteamApp, error) {
	GameIDNotFound := errors.New("Game ID not found")
	for _, game := range appList.Apps {
		uintGameID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			log.Panic(err)
		}
		if game.AppID == uintGameID {
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

	log.Printf("Updated Steam game list. Found %d games.", len(steamListResponse.AppList.Apps))

	c <- steamListResponse.AppList
}

func downloadGameHeaderImage(appId string) string {
	response, err := helpers.DoRequest("GET", "https://cdn.cloudflare.steamstatic.com/steam/apps/"+appId+"/header.jpg")
	if err != nil {
		panic(err)
	}

	if response.Body != nil {
		defer response.Body.Close()
	}

	tmpfile, err := ioutil.TempFile("", "games-screenshot-manager-cover")
	if err != nil {
		panic(err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}

	tmpfile.Write(body)

	return tmpfile.Name()
}

func GuessUsers() []string {
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

func GetGamesFromUser(user string) []string {
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

func GetScreenshotsForGame(user string, game *games.Game) {
	path := filepath.Join(getBasePathForOS(), "userdata", user, "/760/remote/", game.ID, "screenshots")
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

func GetGames(downloadCovers bool) []games.Game {
	var localGames []games.Game
	c := make(chan SteamAppList)
	go getSteamAppList(c)
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

			if downloadCovers {
				userGame.Cover = games.Screenshot{Path: downloadGameHeaderImage(userGameID), DestinationName: ".cover"}
			}

			log.Printf("Found Steam game for user %s: %s (%s)", userID, userGame.Name, userGame.ID)
			GetScreenshotsForGame(userID, &userGame)
			localGames = append(localGames, userGame)
		}
	}
	return localGames
}
