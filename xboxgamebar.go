package gamesscreenshotmanager

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
	"github.com/barasher/go-exiftool"
	"github.com/fmartingr/games-screenshot-manager/internal/exif"
	"github.com/gosimple/slug"
)

var _ Provider = (*XboxGameBarProvider)(nil)

const (
	xboxID           = "xboxgamebar"
	xboxName         = "xbox-game-bar"
	xboxPlatformName = "PC"
	xboxTimeLayout   = "2006:01:02 15:04:05"
)

type dvrMetadata struct {
	StartTime time.Time `json:"startTime"`
}

type XboxGameBarProvider struct {
	config      Config
	xboxConfig  ProviderConfig
	log         *slog.Logger
	gameManager *GameManager
	fileManager *FileManager
}

func NewXboxGameBarProvider(config Config) (*XboxGameBarProvider, error) {
	xboxProvider := &XboxGameBarProvider{
		config:     config,
		xboxConfig: config.Providers.XboxGameBar,
		log:        slog.Default().With("provider", xboxName),
	}

	xboxProvider.gameManager = NewGameManager()
	xboxProvider.fileManager = NewFileManager(config)

	return xboxProvider, nil
}

func (p *XboxGameBarProvider) Run() error {
	if !p.xboxConfig.IsEnabled() {
		p.log.Warn("Xbox Game Bar provider is not enabled")
		return nil
	}

	if _, err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

func (p *XboxGameBarProvider) GetScreenshots() ([]*Game, error) {
	path, err := p.getScreenshotsPath()
	if err != nil {
		return nil, fmt.Errorf("error getting screenshots path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", path)
	}

	// Initialize exiftool
	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, fmt.Errorf("error initializing exiftool: %w", err)
	}
	defer et.Close()

	// Read directory
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %w", path, err)
	}

	// Process files
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		fullPath := filepath.Join(path, fileName)

		// Process only PNG and MP4 files
		if !strings.HasSuffix(fileName, ".png") && !strings.HasSuffix(fileName, ".mp4") {
			continue
		}

		// Ignore hidden files
		if strings.HasPrefix(fileName, ".") {
			continue
		}

		// Get EXIF tags
		tags, err := exif.GetTags(et, fullPath)
		if err != nil {
			p.log.Error("Error getting EXIF tags", slog.String("file", fullPath), slog.Any("error", err))
			continue
		}

		// Get game name from tags
		titleTag := "MicrosoftGameDVRTitle"
		if strings.HasSuffix(fileName, ".mp4") {
			titleTag = "Title"
		}

		var gameName string
		for tagKey, tagValue := range tags {
			if strings.HasPrefix(tagKey, titleTag) {
				gameName = tagValue
			}
		}

		if gameName == "" {
			p.log.Warn("No game name found", slog.String("file", fullPath))
			continue
		}

		var destinationName string
		var mediaKind MediaKind

		if strings.HasSuffix(fileName, ".png") {
			mediaKind = MediaKindScreenshot

			// Get metadata for PNG
			var metadataString string
			for tagKey, tagValue := range tags {
				if strings.HasPrefix(tagKey, "MicrosoftGameDVRExtended") {
					metadataString = tagValue
					break
				}
			}

			if metadataString == "" {
				p.log.Warn("No metadata found", slog.String("file", fullPath))
				continue
			}

			var metadata dvrMetadata
			if err := json.Unmarshal([]byte(metadataString), &metadata); err != nil {
				p.log.Error("Error parsing metadata", slog.String("file", fullPath), slog.Any("error", err))
				continue
			}

			destinationName = metadata.StartTime.Format(DatetimeFormat) + ".png"
		} else {
			mediaKind = MediaKindClip

			// Get media creation time for MP4
			mediaCreateTag := "MediaCreateDate"
			mediaCreateString, exists := tags[mediaCreateTag]
			if !exists {
				p.log.Warn("No media creation time found", slog.String("file", fullPath))
				continue
			}

			mediaCreationTime, err := time.Parse(xboxTimeLayout, mediaCreateString)
			if err != nil {
				p.log.Warn("Error parsing media creation time", slog.String("file", fullPath), slog.Any("error", err))
				continue
			}

			destinationName = mediaCreationTime.Format(DatetimeFormat) + ".mp4"
		}

		// Add to game manager
		gameID := slug.Make(gameName)
		game := p.gameManager.AddGame(NewGame(gameID, gameName, xboxPlatformName, xboxID))

		media := NewMedia(mediaKind, fullPath)
		media.DestinationName = destinationName

		game.AddScreenshot(media)
	}

	// Process all games
	for _, game := range p.gameManager.GetGames() {
		if err := p.fileManager.ProcessGame(game); err != nil {
			p.log.Error("Error processing game", slog.Any("error", err))
			continue
		}
	}

	return p.gameManager.GetGames(), nil
}

func (p *XboxGameBarProvider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *XboxGameBarProvider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *XboxGameBarProvider) FindGames(options ProviderConfig) ([]Game, error) {
	return nil, nil
}

// getScreenshotsPath returns the path where Xbox Game Bar screenshots are stored
func (p *XboxGameBarProvider) getScreenshotsPath() (string, error) {
	if p.xboxConfig.GetPath() != "" && p.xboxConfig.GetPath() != "auto" {
		return toolkitPaths.ExpandUser(p.xboxConfig.GetPath()), nil
	}

	// Default path for Xbox Game Bar captures
	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultPath := filepath.Join(homeDir, "Videos", "Captures")
		if _, err := os.Stat(defaultPath); err == nil {
			return defaultPath, nil
		}
	}

	return "", fmt.Errorf("path to Xbox Game Bar screenshots folder must be provided")
}
