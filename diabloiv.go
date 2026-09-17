package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var _ Provider = (*DiabloIVProvider)(nil)

const (
	d4ID           = "diablo4"
	d4Name         = "Diablo IV"
	d4PlatformName = "PC"
)

// d4Extensions are the file types the provider collects.
var d4Extensions = []string{".jpg", ".jpeg", ".png"}

// d4DefaultPaths are the folders the game writes to on Windows. Both are
// scanned, and a folder that is not there is skipped.
var d4DefaultPaths = []string{
	"~/Pictures/Diablo IV",
	"~/Documents/Diablo IV/Screenshots",
}

type DiabloIVProvider struct {
	config      Config
	d4Config    ProviderConfig
	log         *slog.Logger
	gameManager *GameManager
	fileManager *FileManager
}

func NewDiabloIVProvider(config Config, gameManager *GameManager, fileManager *FileManager) (*DiabloIVProvider, error) {
	d4Provider := &DiabloIVProvider{
		config:      config,
		d4Config:    config.Providers.DiabloIV,
		log:         slog.Default().With("provider", d4Name),
		gameManager: gameManager,
		fileManager: fileManager,
	}

	return d4Provider, nil
}

func (p *DiabloIVProvider) Run() error {
	if !p.d4Config.IsEnabled() {
		p.log.Warn("Diablo IV provider is not enabled")
		return nil
	}

	if err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

func (p *DiabloIVProvider) GetScreenshots() error {
	paths := p.getScreenshotsPaths()
	if len(paths) == 0 {
		p.log.Warn(
			"the game writes no folder of its own on this platform, so set path in the config",
			slog.String("os", runtime.GOOS),
		)

		return nil
	}

	game := NewGame(d4ID, d4Name, d4PlatformName, d4Name)

	for _, path := range paths {
		if err := p.processScreenshotsPath(path, game); err != nil {
			p.log.Error("Error processing screenshots path", slog.String("path", path), slog.Any("error", err))
		}
	}

	// Only add game if it has screenshots
	if len(game.Screenshots) > 0 {
		p.gameManager.AddGame(game)
	}

	return nil
}

// processScreenshotsPath collects the screenshots of one folder into the game.
// A folder that is not there is not an error: the provider scans two default
// folders, and a computer holds one of them.
func (p *DiabloIVProvider) processScreenshotsPath(path string, game *Game) error {
	path = expandUser(path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		p.log.Debug("Screenshots directory does not exist", slog.String("path", path))
		return nil
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("error reading from path %s: %w", path, err)
	}

	for _, file := range files {
		if file.IsDir() || !d4IsScreenshot(file.Name()) {
			continue
		}

		fullPath := filepath.Join(path, file.Name())

		info, err := file.Info()
		if err != nil {
			p.log.Warn("error reading file info", slog.String("file", fullPath), slog.Any("err", err))
			continue
		}

		media := NewMedia(MediaKindScreenshot, fullPath)
		media.DestinationName = d4DestinationName(file.Name(), info.ModTime().Format(DatetimeFormat))
		game.AddScreenshot(media)
	}

	return nil
}

// d4IsScreenshot reports a file the provider collects.
func d4IsScreenshot(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))

	for _, candidate := range d4Extensions {
		if extension == candidate {
			return true
		}
	}

	return false
}

// d4DestinationName builds the name a screenshot is copied under.
//
// The game names every screenshot Screenshot001.jpg, and it counts up from
// there. The counter starts again on each computer, so the name says nothing
// about which screenshot it is. The date the file was written does, so the name
// is built from that.
//
// Two screenshots of one second still meet under one name. The file manager
// answers that: it compares the two hashes and puts the hash of the second file
// in its name.
func d4DestinationName(name, datetime string) string {
	return datetime + strings.ToLower(filepath.Ext(name))
}

func (p *DiabloIVProvider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *DiabloIVProvider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *DiabloIVProvider) FindGames(options ProviderConfig) ([]Game, error) {
	return nil, nil
}

// getScreenshotsPaths returns the folders that hold the Diablo IV screenshots.
//
// It returns nothing on a platform where the game writes no folder of its own.
// That is not a failure: a path in the config is the answer, and the caller
// says so. A run of a config that never named this game therefore stays quiet.
func (p *DiabloIVProvider) getScreenshotsPaths() []string {
	if p.d4Config.GetPath() != "" && p.d4Config.GetPath() != "auto" {
		return []string{expandUser(p.d4Config.GetPath())}
	}

	if runtime.GOOS == "windows" {
		return d4DefaultPaths
	}

	return nil
}
