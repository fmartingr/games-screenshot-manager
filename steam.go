package gamesscreenshotmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	toolkitCache "git.nakama.town/fmartingr/gotoolkit/cache"
	toolkitModel "git.nakama.town/fmartingr/gotoolkit/model"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
)

var _ Provider = (*SteamProvider)(nil)

const (
	steamAppListURL    = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"
	steamGameHeaderURL = "https://cdn.cloudflare.steamstatic.com/steam/apps/%d/header.jpg"
)

type SteamApp struct {
	AppID uint64 `json:"appid"`
	Name  string `json:"name"`
}

type SteamAppList struct {
	Apps []SteamApp `json:"apps"`
}

func (s *SteamAppList) GetGameName(gameID string) string {
	for _, app := range s.Apps {
		if fmt.Sprintf("%d", app.AppID) == gameID {
			return app.Name
		}
	}
	return ""
}

type SteamAppListResponse struct {
	AppList SteamAppList `json:"applist"`
}

type SteamProvider struct {
	config      Config
	steamConfig SteamConfig
	log         *slog.Logger
	cache       toolkitModel.Cache
	steamApps   SteamAppList
	gameManager *GameManager
	fileManager *FileManager
}

func NewSteamProvider(config Config) (*SteamProvider, error) {
	steamProvider := &SteamProvider{
		config:      config,
		steamConfig: config.Providers.Steam,
		log:         slog.Default().With("provider", "steam"),
		cache:       toolkitCache.NewMemoryCache(), // TODO: Use a file based cache
	}

	if err := steamProvider.downloadSteamAppList(); err != nil {
		return nil, fmt.Errorf("error downloading steam app list: %s", err)
	}

	steamProvider.gameManager = NewGameManager()
	steamProvider.fileManager = NewFileManager(config)

	return steamProvider, nil
}

func (p *SteamProvider) Run() error {
	if !p.steamConfig.Enabled {
		p.log.Warn("Steam provider is not enabled")
		return nil
	}

	if _, err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	if p.steamConfig.ProcessClips {
		if _, err := p.GetClips(); err != nil {
			p.log.Error("Failed to get clips", slog.Any("error", err))
		}
	}

	if p.steamConfig.ProcessRecordings {
		if _, err := p.GetRecordings(); err != nil {
			p.log.Error("Failed to get recordings", slog.Any("error", err))
		}
	}

	return nil
}

func (p *SteamProvider) GetScreenshots() ([]*Game, error) {
	steamBasePath, err := p.getSteamBasePath()
	if err != nil {
		return nil, fmt.Errorf("error getting steam base path: %s", err)
	}

	users, err := p.getUsers(steamBasePath)
	if err != nil {
		return nil, fmt.Errorf("error getting users: %s", err)
	}

	for _, userID := range users {
		p.log.Info("Processing user", slog.String("user_id", userID))

		userPath := filepath.Join(steamBasePath, "userdata", userID, "760", "remote")

		files, err := os.ReadDir(userPath)
		if err != nil {
			return nil, fmt.Errorf("error reading userdata path: %s", err)
		}

		for _, file := range files {
			gameName := p.steamApps.GetGameName(file.Name())
			if gameName == "" {
				gameName = file.Name()
			}

			game := NewGame(file.Name(), gameName, "PC", "steam")
			p.gameManager.AddGame(game)

			// Get all files from folder
			gamePath := filepath.Join(userPath, file.Name(), "screenshots")
			screenshots, err := os.ReadDir(gamePath)
			if err != nil {
				return nil, fmt.Errorf("error reading game path: %s", err)
			}

			for _, screenshot := range screenshots {
				if !screenshot.IsDir() && strings.HasSuffix(screenshot.Name(), ".jpg") {
					media := NewMedia(MediaKindScreenshot, filepath.Join(gamePath, screenshot.Name()))
					game.AddScreenshot(media)
				}
			}

			p.log.Info("Processing game", slog.String("game_id", file.Name()))
		}

	}

	games := p.gameManager.GetGames()
	for _, game := range games {
		for _, media := range game.Screenshots {
			p.log.Info("Processing screenshot", slog.String("screenshot_path", media.Path))
		}
	}

	return games, nil
}

func (p *SteamProvider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *SteamProvider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *SteamProvider) FindGames(options SteamConfig) ([]Game, error) {
	return nil, nil
}

// downloadSteamAppList downloads the Steam APP List and caches it
// It's used to get the JSON response with all the games so we can match the game ID (folder name)
// with the game name (from the JSON response)
func (p *SteamProvider) downloadSteamAppList() error {
	p.log.Info("Downloading Steam APP List, used to get all game IDs and Names")

	cacheKey := "steam-applist"
	download := true
	var payload []byte

	result, err := p.cache.GetWithExpiry(cacheKey, 24*time.Hour)
	if err != nil && !errors.Is(err, toolkitModel.ErrCacheKeyDontExist) {
		return fmt.Errorf("error retrieving cache: %s", err)
	}

	if result != nil {
		download = false
		payload = result.([]byte)
	}

	if download {
		response, err := helpers.DoRequest("GET", steamAppListURL)
		if err != nil {
			return fmt.Errorf("error making request for Steam APP List: %s", err)
		}

		if response.Body != nil {
			defer response.Body.Close()
		}

		payload, err = io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("error reading steam response: %s", err)
		}

		if err := p.cache.Set(cacheKey, payload); err != nil {
			return fmt.Errorf("error caching steam app list: %s", err)
		}
	}

	steamListResponse := SteamAppListResponse{}
	jsonErr := json.Unmarshal(payload, &steamListResponse)
	if jsonErr != nil {
		return fmt.Errorf("error unmarshalling steam's response: %s", jsonErr)
	}

	if len(steamListResponse.AppList.Apps) == 0 {
		return fmt.Errorf("coulnd't get steam app list")
	}

	p.steamApps = steamListResponse.AppList

	return nil
}

// getSteamBasePath returns the base path for the Steam installation
// It supports Darwin, Linux and Windows
func (p *SteamProvider) getSteamBasePath() (string, error) {
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

// getUsers returns a list of user IDs from the Steam userdata directory
// It only returns user IDs that have a 760/remote subpath, which is where screenshots are stored
func (p *SteamProvider) getUsers(steamBasePath string) ([]string, error) {
	var userIDs []string

	userPath := filepath.Join(steamBasePath, "userdata")

	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("userdata path does not exist: %s", userPath)
	}

	files, err := os.ReadDir(userPath)
	if err != nil {
		return nil, fmt.Errorf("error reading userdata path: %s", err)
	}

	for _, file := range files {
		// Check if subpath 760/remote exists, if not, skip
		// 760/remote is where screenshots are stored
		gamePath := filepath.Join(userPath, file.Name(), "760", "remote")
		if _, err := os.Stat(gamePath); os.IsNotExist(err) {
			continue
		}

		userIDs = append(userIDs, file.Name())
	}

	return userIDs, nil
}
