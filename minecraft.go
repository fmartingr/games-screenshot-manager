package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
)

var _ Provider = (*MinecraftProvider)(nil)

const (
	mcID           = "minecraft"
	mcName         = "Minecraft"
	mcPlatformName = "PC"
	mcExtension    = ".png"
)

type MinecraftProvider struct {
	config      Config
	mcConfig    ProviderConfig
	log         *slog.Logger
	gameManager *GameManager
	fileManager *FileManager
}

func NewMinecraftProvider(config Config) (*MinecraftProvider, error) {
	mcProvider := &MinecraftProvider{
		config:   config,
		mcConfig: config.Providers.Minecraft,
		log:      slog.Default().With("provider", mcName),
	}

	mcProvider.gameManager = NewGameManager()
	mcProvider.fileManager = NewFileManager(config)

	return mcProvider, nil
}

func (p *MinecraftProvider) Run() error {
	if !p.mcConfig.IsEnabled() {
		p.log.Warn("Minecraft provider is not enabled")
		return nil
	}

	if _, err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

func (p *MinecraftProvider) GetScreenshots() ([]*Game, error) {
	paths, err := p.getScreenshotsPaths()
	if err != nil {
		return nil, fmt.Errorf("error getting screenshots paths: %w", err)
	}

	// Create standalone Minecraft game
	standaloneGame := NewGame(mcID, mcName, mcPlatformName, mcName)

	// Create Flatpak Minecraft game for Linux
	var flatpakGame *Game
	if runtime.GOOS == "linux" {
		flatpakGame = NewGame(mcID, mcName, mcPlatformName, mcName+"-flatpak")
	}

	// Process each path
	for _, path := range paths {
		if err := p.processScreenshotsPath(path, standaloneGame, flatpakGame); err != nil {
			p.log.Error("Error processing screenshots path", slog.String("path", path), slog.Any("error", err))
		}
	}

	// Add games to manager
	p.gameManager.AddGame(standaloneGame)
	if flatpakGame != nil {
		p.gameManager.AddGame(flatpakGame)
	}

	games := p.gameManager.GetGames()
	for _, game := range games {
		if err := p.fileManager.ProcessGame(game); err != nil {
			p.log.Error("Error processing game", slog.Any("error", err))
			continue
		}
	}

	return games, nil
}

func (p *MinecraftProvider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *MinecraftProvider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *MinecraftProvider) FindGames(options ProviderConfig) ([]Game, error) {
	return nil, nil
}

// getScreenshotsPaths returns the paths where Minecraft screenshots are stored
func (p *MinecraftProvider) getScreenshotsPaths() ([]string, error) {
	if p.mcConfig.GetPath() != "" && p.mcConfig.GetPath() != "auto" {
		return []string{toolkitPaths.ExpandUser(p.mcConfig.GetPath())}, nil
	}

	var paths []string
	switch runtime.GOOS {
	case "linux":
		paths = []string{
			"~/.minecraft/screenshots",
			"~/.var/app/com.mojang.Minecraft/.minecraft/screenshots",
			"~/.var/app/com.mojang.Minecraft/data/minecraft/screenshots",
		}
	case "windows":
		paths = []string{
			filepath.Join(os.Getenv("APPDATA"), ".minecraft/screenshots"),
		}
	case "darwin":
		paths = []string{
			"~/Library/Application Support/minecraft/screenshots",
		}
	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return paths, nil
}

// processScreenshotsPath processes screenshots from a given path and adds them to the appropriate game
func (p *MinecraftProvider) processScreenshotsPath(path string, standaloneGame, flatpakGame *Game) error {
	path = toolkitPaths.ExpandUser(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Skip if directory doesn't exist
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("error reading from %s: %w", path, err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), mcExtension) {
			fullPath := filepath.Join(path, file.Name())
			media := NewMedia(MediaKindScreenshot, fullPath)
			media.DestinationName = file.Name()

			// Determine which game to add the screenshot to based on the path
			if strings.Contains(path, "com.mojang.Minecraft") {
				if flatpakGame != nil {
					flatpakGame.AddScreenshot(media)
				}
			} else {
				standaloneGame.AddScreenshot(media)
			}
		}
	}

	return nil
}
