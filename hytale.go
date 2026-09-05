package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fmartingr/games-screenshot-manager/covers"
)

var _ Provider = (*HytaleProvider)(nil)

const (
	hytaleID           = "hytale"
	hytaleName         = "Hytale"
	hytalePlatformName = "PC"
)

type HytaleProvider struct {
	config       Config
	hytaleConfig ProviderConfig
	log          *slog.Logger
	gameManager  *GameManager
	fileManager  *FileManager
}

func NewHytaleProvider(config Config, gameManager *GameManager, fileManager *FileManager) (*HytaleProvider, error) {
	hytaleProvider := &HytaleProvider{
		config:       config,
		hytaleConfig: config.Providers.Hytale,
		log:          slog.Default().With("provider", hytaleName),
		gameManager:  gameManager,
		fileManager:  fileManager,
	}

	return hytaleProvider, nil
}

func (p *HytaleProvider) Run() error {
	if !p.hytaleConfig.IsEnabled() {
		p.log.Warn("Hytale provider is not enabled")
		return nil
	}

	if err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	if p.hytaleConfig.ShouldDownloadCovers() {
		if err := p.GetCovers(); err != nil {
			p.log.Error("Failed to get covers", slog.Any("error", err))
		}
	}

	return nil
}

func (p *HytaleProvider) GetScreenshots() error {
	path, err := p.getScreenshotsPath()
	if err != nil {
		return fmt.Errorf("error getting screenshots path: %w", err)
	}

	// Check if directory exists, skip if it doesn't
	path = expandUser(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		p.log.Debug("Screenshots directory does not exist", slog.String("path", path))
		return nil
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("error reading from path %s: %w", path, err)
	}

	game := NewGame(hytaleID, hytaleName, hytalePlatformName, hytaleName)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fullPath := filepath.Join(path, file.Name())
		ext := strings.ToLower(filepath.Ext(file.Name()))

		// Support both .png and .jpg screenshot formats
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			media := NewMedia(MediaKindScreenshot, fullPath)
			media.DestinationName = file.Name()
			game.AddScreenshot(media)
		}
	}

	// Only add game if it has screenshots
	if len(game.Screenshots) > 0 {
		p.gameManager.AddGame(game)
	}

	return nil
}

func (p *HytaleProvider) GetCovers() error {
	if p.config.DryRun {
		return nil
	}

	game := p.gameManager.GetGame(hytaleID)
	if game == nil {
		// Create game if it doesn't exist yet
		game = NewGame(hytaleID, hytaleName, hytalePlatformName, hytaleName)
		p.gameManager.AddGame(game)
	}

	// Get embedded cover
	tempFile, err := covers.GetCover(hytaleID)
	if err != nil {
		return fmt.Errorf("error getting embedded cover: %w", err)
	}

	media := NewMedia(MediaKindCover, tempFile.Name())
	media.DestinationName = "cover.png"
	game.SetCover(media)

	return nil
}

func (p *HytaleProvider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *HytaleProvider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *HytaleProvider) FindGames(options ProviderConfig) ([]Game, error) {
	return nil, nil
}

// getScreenshotsPath returns the path where Hytale screenshots are stored
func (p *HytaleProvider) getScreenshotsPath() (string, error) {
	if p.hytaleConfig.GetPath() != "" && p.hytaleConfig.GetPath() != "auto" {
		return expandUser(p.hytaleConfig.GetPath()), nil
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return expandUser("~/Pictures/Hytale Screenshots"), nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s (only macOS and Linux are supported)", runtime.GOOS)
	}
}
