package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
)

var _ Provider = (*Playstation5Provider)(nil)

const (
	ps5ID             = "ps5"
	ps5Name           = "playstation-5"
	ps5PlatformName   = "PlayStation 5"
	ps5FilenameLayout = "20060102150405"
)

type Playstation5Provider struct {
	config      Config
	ps5Config   ProviderConfig
	log         *slog.Logger
	gameManager *GameManager
	fileManager *FileManager
}

func NewPlaystation5Provider(config Config) (*Playstation5Provider, error) {
	ps5Provider := &Playstation5Provider{
		config:    config,
		ps5Config: config.Providers.PlayStation5,
		log:       slog.Default().With("provider", ps5Name),
	}

	ps5Provider.gameManager = NewGameManager()
	ps5Provider.fileManager = NewFileManager(config)

	return ps5Provider, nil
}

func (p *Playstation5Provider) Run() error {
	if !p.ps5Config.IsEnabled() {
		p.log.Warn("PlayStation 5 provider is not enabled")
		return nil
	}

	if _, err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

func (p *Playstation5Provider) GetScreenshots() ([]*Game, error) {
	path, err := p.getScreenshotsPath()
	if err != nil {
		return nil, fmt.Errorf("error getting screenshots path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", path)
	}

	err = filepath.Walk(path,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				p.log.Error("Error walking path", slog.String("path", filePath), slog.Any("error", err))
				return err
			}

			if !info.IsDir() {
				gameName := filepath.Base(filepath.Dir(filePath))
				fileName := filepath.Base(filePath)
				extension := filepath.Ext(fileName)

				// Ignore hidden files
				if strings.HasPrefix(fileName, ".") {
					return nil
				}

				var destinationName string
				var mediaKind MediaKind

				if extension == ".jpg" || extension == ".webm" {
					parts := strings.Split(strings.TrimSuffix(fileName, extension), "_")
					if len(parts) < 2 {
						p.log.Warn("File does not follow naming convention", slog.String("file", fileName))
						return nil
					}

					datetime, err := time.Parse(ps5FilenameLayout, parts[1])
					if err != nil {
						p.log.Warn("Error parsing datetime from filename", slog.String("file", fileName), slog.Any("error", err))
						return nil
					}

					destinationName = datetime.Format(DatetimeFormat)
					if extension == ".jpg" {
						mediaKind = MediaKindScreenshot
					} else {
						mediaKind = MediaKindClip
					}

					media := NewMedia(mediaKind, filePath)
					media.DestinationName = destinationName + extension

					game := p.gameManager.AddGame(NewGame(gameName, gameName, ps5PlatformName, ps5ID))
					game.AddScreenshot(media)
				}
			}

			return nil
		})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	for _, game := range p.gameManager.GetGames() {
		if err := p.fileManager.ProcessGame(game); err != nil {
			p.log.Error("Error processing game", slog.Any("error", err))
			continue
		}
	}

	return p.gameManager.GetGames(), nil
}

func (p *Playstation5Provider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *Playstation5Provider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *Playstation5Provider) FindGames(options ProviderConfig) ([]Game, error) {
	return nil, nil
}

// getScreenshotsPath returns the path where PlayStation 5 screenshots are stored
func (p *Playstation5Provider) getScreenshotsPath() (string, error) {
	if p.ps5Config.GetPath() != "" && p.ps5Config.GetPath() != "auto" {
		return toolkitPaths.ExpandUser(p.ps5Config.GetPath()), nil
	}

	return "", fmt.Errorf("path to PlayStation 5 screenshots folder must be provided")
}
