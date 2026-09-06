package gamesscreenshotmanager

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var _ Provider = (*SteamProvider)(nil)

type SteamProvider struct {
	config       Config
	steamConfig  SteamConfig
	log          *slog.Logger
	client       *SteamClient
	gameManager  *GameManager
	fileManager  *FileManager
	ignoredGames map[string]struct{}
}

// newIgnoredGamesSet builds the lookup set for the app IDs ignored_games
// holds. An entry is trimmed, and an empty entry is dropped.
func newIgnoredGamesSet(gameIDs []string) map[string]struct{} {
	ignored := make(map[string]struct{}, len(gameIDs))

	for _, gameID := range gameIDs {
		gameID = strings.TrimSpace(gameID)
		if gameID == "" {
			continue
		}

		ignored[gameID] = struct{}{}
	}

	return ignored
}

func NewSteamProvider(config Config, gameManager *GameManager, fileManager *FileManager) (*SteamProvider, error) {
	// Validate that API key is set if Steam provider is enabled
	if config.Providers.Steam.IsEnabled() && config.Providers.Steam.APIKey == "" {
		return nil, fmt.Errorf("steam provider is enabled but API key is not set. The Steam app list endpoint requires authentication. Please set 'api_key' in the [providers.steam] section of your config file")
	}

	// Pass API key to NewSteamClient since it's now required for downloading app list
	client, err := NewSteamClient(fileManager, config.Providers.Steam.APIKey)
	if err != nil {
		return nil, fmt.Errorf("error creating steam client: %s", err)
	}

	steamProvider := &SteamProvider{
		config:       config,
		steamConfig:  config.Providers.Steam,
		log:          slog.Default().With("provider", "steam"),
		client:       client,
		gameManager:  gameManager,
		fileManager:  fileManager,
		ignoredGames: newIgnoredGamesSet(config.Providers.Steam.IgnoredGames),
	}

	return steamProvider, nil
}

func (p *SteamProvider) Run() error {
	if !p.steamConfig.IsEnabled() {
		p.log.Warn("Steam provider is not enabled")
		return nil
	}

	if err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	if p.steamConfig.ShouldProcessClips() {
		if err := p.GetClips(); err != nil {
			p.log.Error("Failed to get clips", slog.Any("error", err))
		}
	}

	if p.steamConfig.ShouldProcessRecordings() {
		if err := p.GetRecordings(); err != nil {
			p.log.Error("Failed to get recordings", slog.Any("error", err))
		}
	}

	// Process online gallery if enabled
	if p.steamConfig.OnlineGallery && p.steamConfig.UserID != "" {
		if err := p.GetPublishedScreenshots(); err != nil {
			p.log.Error("Failed to get published screenshots", slog.Any("error", err))
		}
	}

	if p.steamConfig.ShouldDownloadCovers() {
		p.log.Info("Downloading covers")
		if err := p.GetCovers(); err != nil {
			p.log.Error("Failed to get covers", slog.Any("error", err))
		}
	}

	return nil
}

// GetPublishedScreenshots retrieves screenshots published to Steam by the configured user
func (p *SteamProvider) GetPublishedScreenshots() error {
	if p.steamConfig.APIKey == "" {
		return fmt.Errorf("steam API key not configured")
	}

	if p.steamConfig.UserID == "" {
		return fmt.Errorf("steam online gallery ID not configured")
	}

	p.log.Info("Getting published screenshots from Steam",
		slog.String("steam_id", p.steamConfig.UserID))

	// Retrieve published screenshots using the client
	publishedScreenshots, err := p.client.GetPublishedScreenshots(p.steamConfig.UserID)
	if err != nil {
		return fmt.Errorf("error getting published screenshots: %s", err)
	}

	p.log.Info("Found published screenshots", slog.Int("count", len(publishedScreenshots)))

	// Create or update games with the screenshots
	for _, screenshot := range publishedScreenshots {
		if screenshot.AppID == 0 {
			gameID := p.client.GetGameID(screenshot.ShortcutName)
			if gameID == "" {
				p.log.Warn("No game ID found for shortcut name", slog.String("shortcut_name", screenshot.ShortcutName))
				continue
			} else {
				p.log.Warn(
					"Found screenshot with app ID 0 and couldn't find game ID using shortcut name",
					slog.Any("screenshot", screenshot),
					slog.String("shortcut_name", screenshot.ShortcutName),
					slog.String("game_id", gameID),
				)
			}

			screenshot.AppID, err = strconv.Atoi(gameID)
			if err != nil {
				p.log.Error("Error converting game ID to int", slog.String("game_id", gameID), slog.Any("error", err))
				continue
			}
		}

		appIDString := fmt.Sprintf("%d", screenshot.AppID)

		if p.isIgnored(appIDString) {
			p.log.Info("Skipping ignored game", slog.String("game_id", appIDString))
			continue
		}

		gameName := p.resolveGameName(appIDString)
		if gameName == "" {
			p.log.Warn("No game name found for app ID, using app ID as game name", slog.Int("app_id", screenshot.AppID))
			gameName = appIDString
		}

		// Create or get existing game
		game := p.gameManager.GetGame(appIDString)
		if game == nil {
			game = NewGame(appIDString, gameName, "PC", "steam")
			p.gameManager.AddGame(game)
		}

		media := NewMedia(MediaKindScreenshot, "")
		dateTimeCreated := time.Unix(int64(screenshot.TimeCreated), 0)
		media.SetSourceURL(screenshot.FileURL)
		media.DestinationName = fmt.Sprintf("%s.jpg", dateTimeCreated.Format("2006-01-02_15-04-05"))
		media.ComparisionFunc = steamGalleryComparisonFunc
		game.AddScreenshot(media)
	}

	return nil
}

func (p *SteamProvider) GetScreenshots() error {
	steamBasePath, err := p.getSteamBasePath()
	if err != nil {
		return fmt.Errorf("error getting steam base path: %s", err)
	}

	users, err := p.getUsers(steamBasePath)
	if err != nil {
		return fmt.Errorf("error getting users: %s", err)
	}

	for _, userID := range users {
		p.log.Info("Processing user", slog.String("user_id", userID))

		userPath := filepath.Join(steamBasePath, "userdata", userID, "760", "remote")

		files, err := os.ReadDir(userPath)
		if err != nil {
			return fmt.Errorf("error reading userdata path: %s", err)
		}

		for _, file := range files {
			if p.isIgnored(file.Name()) {
				p.log.Info("Skipping ignored game", slog.String("game_id", file.Name()))
				continue
			}

			gameName := p.resolveGameName(file.Name())

			// If game name is empty, use the folder name
			if gameName == "" {
				p.log.Warn("No game name found", slog.String("game_id", file.Name()))
				gameName = file.Name()
			}

			game := NewGame(file.Name(), gameName, "PC", "steam")
			p.gameManager.AddGame(game)

			p.log.Info("Processing game", slog.String("game_id", game.ID))

			// Get all files from folder
			gamePath := filepath.Join(userPath, file.Name(), "screenshots")
			screenshots, err := os.ReadDir(gamePath)
			if err != nil {
				return fmt.Errorf("error reading game path: %s", err)
			}

			for _, screenshot := range screenshots {
				if !screenshot.IsDir() && strings.HasSuffix(screenshot.Name(), ".jpg") {
					media := NewMedia(MediaKindScreenshot, filepath.Join(gamePath, screenshot.Name()))
					game.AddScreenshot(media)
				}
			}

		}
	}

	return nil
}

func (p *SteamProvider) GetRecordings() error {
	return nil
}

func (p *SteamProvider) GetClips() error {
	return nil
}

func (p *SteamProvider) GetCovers() error {
	if !p.config.DryRun {
		for _, game := range p.gameManager.GetGames() {
			if game.Provider == "steam" {
				// Download cover using the client
				tempFile, err := p.client.DownloadGameCover(game.ID)
				if err != nil {
					p.log.Error("error downloading cover", slog.String("game_id", game.ID), slog.String("game_name", game.Name), slog.Any("error", err))
					continue
				}

				media := NewMedia(MediaKindCover, tempFile.Name())
				media.DestinationName = "cover.jpg"
				p.gameManager.GetGame(game.ID).SetCover(media)
			}
		}
	}

	return nil
}

func (p *SteamProvider) FindGames(options SteamConfig) ([]Game, error) {
	return nil, nil
}

// resolveGameName returns the name to use for a given Steam game/folder ID.
// A name configured in [providers.steam.custom_games] always takes precedence,
// which allows overriding the default Steam store name as well as naming games
// that are not in the Steam store. It falls back to the Steam app list name and
// returns an empty string if neither is available.
func (p *SteamProvider) resolveGameName(gameID string) string {
	if name := p.steamConfig.CustomGames[gameID]; name != "" {
		return name
	}

	return p.client.GetGameName(gameID)
}

// isIgnored reports an app ID the configuration asks the provider to skip. An
// ignored game never reaches the GameManager, so it gets no directory, no
// media file and no cover.
func (p *SteamProvider) isIgnored(gameID string) bool {
	_, ignored := p.ignoredGames[gameID]
	return ignored
}

// getSteamBasePath returns the base path for the Steam installation
// It supports Darwin, Linux and Windows
func (p *SteamProvider) getSteamBasePath() (string, error) {
	var path string
	switch runtime.GOOS {
	case "darwin":
		path = expandUser("~/Library/Application Support/Steam")
	case "linux":
		path = expandUser("~/.local/share/Steam")
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

func steamGalleryComparisonFunc(m MediaFile, destinationPath string) bool {
	parts := strings.Split(m.GetSourceURL(), "/")
	if len(parts) < 2 {
		return false
	}

	sha1hash := parts[len(parts)-2]
	if sha1hash == "" {
		return false
	}

	// Generate SHA1 hash for destination file
	destFile, err := os.Open(destinationPath)
	if err != nil {
		return false
	}
	defer destFile.Close()

	destHash := sha1.New()
	if _, err := io.Copy(destHash, destFile); err != nil {
		return false
	}

	destHashString := hex.EncodeToString(destHash.Sum(nil))

	// Compare the hashes
	return destHashString == strings.ToLower(sha1hash)
}
