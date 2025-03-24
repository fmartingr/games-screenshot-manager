package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
)

var _ Provider = (*WorldOfWarcraftProvider)(nil)

const (
	wowID             = "wow"
	wowName           = "World of Warcraft"
	wowPlatformName   = "PC"
	wowDateTimeLayout = "010206_150405"
	wowFilenamePrefix = "WoWScrnShot_"
	wowExtension      = ".jpg"
)

type WorldOfWarcraftProvider struct {
	config      Config
	wowConfig   ProviderConfig
	log         *slog.Logger
	gameManager *GameManager
	fileManager *FileManager
}

func NewWorldOfWarcraftProvider(config Config, gameManager *GameManager, fileManager *FileManager) (*WorldOfWarcraftProvider, error) {
	wowProvider := &WorldOfWarcraftProvider{
		config:      config,
		wowConfig:   config.Providers.WorldOfWarcraft,
		log:         slog.Default().With("provider", wowName),
		gameManager: gameManager,
		fileManager: fileManager,
	}

	return wowProvider, nil
}

func (p *WorldOfWarcraftProvider) Run() error {
	if !p.wowConfig.IsEnabled() {
		p.log.Warn("World of Warcraft provider is not enabled")
		return nil
	}

	if err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

func (p *WorldOfWarcraftProvider) GetScreenshots() error {
	path, err := p.getScreenshotsPath()
	if err != nil {
		return fmt.Errorf("error getting screenshots path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", path)
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("error reading from path %s: %w", path, err)
	}

	game := NewGame(wowID, wowName, wowPlatformName, wowName)

	for _, file := range files {
		fullPath := filepath.Join(path, file.Name())

		if strings.Contains(file.Name(), wowExtension) {
			parts := strings.SplitN(file.Name(), "_", 2)
			if len(parts) != 2 {
				p.log.Warn("invalid filename format", slog.String("file", file.Name()))
				continue
			}

			dateStr := strings.Replace(parts[1], wowExtension, "", 1)
			fileDate, err := time.Parse(wowDateTimeLayout, dateStr)
			if err != nil {
				p.log.Warn("error parsing media creation time", slog.String("file", file.Name()), slog.Any("err", err))
				continue
			}

			destinationName := fileDate.Format(DatetimeFormat) + wowExtension
			media := NewMedia(MediaKindScreenshot, fullPath)
			media.DestinationName = destinationName
			game.AddScreenshot(media)
		}
	}

	return nil
}

func (p *WorldOfWarcraftProvider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *WorldOfWarcraftProvider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *WorldOfWarcraftProvider) FindGames(options ProviderConfig) ([]Game, error) {
	return nil, nil
}

// getScreenshotsPath returns the path where World of Warcraft screenshots are stored
func (p *WorldOfWarcraftProvider) getScreenshotsPath() (string, error) {
	if p.wowConfig.GetPath() != "" && p.wowConfig.GetPath() != "auto" {
		return toolkitPaths.ExpandUser(p.wowConfig.GetPath()), nil
	}

	switch runtime.GOOS {
	case "windows":
		return "C:/Program Files (x86)/World of Warcraft/_retail_/Screenshots", nil
	case "darwin":
		return "/Applications/World of Warcraft/_retail_/Screenshots", nil
	default:
		return "", fmt.Errorf("for non-windows/darwin systems the path to the screenshots folder must be provided")
	}
}
