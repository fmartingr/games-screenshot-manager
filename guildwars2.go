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
	"github.com/barasher/go-exiftool"
)

var _ Provider = (*GuildWars2Provider)(nil)

const (
	gw2ID             = "gw2"
	gw2Name           = "Guild Wars 2"
	gw2PlatformName   = "PC"
	gw2Extension      = ".jpg"
	gw2ExifTimeLayout = "2006:01:02 15:04:05-07:00"
	gw2ExifTimeTag    = "FileModifyDate"
)

type GuildWars2Provider struct {
	config      Config
	gw2Config   ProviderConfig
	log         *slog.Logger
	gameManager *GameManager
	fileManager *FileManager
}

func NewGuildWars2Provider(config Config, gameManager *GameManager, fileManager *FileManager) (*GuildWars2Provider, error) {
	gw2Provider := &GuildWars2Provider{
		config:      config,
		gw2Config:   config.Providers.GuildWars2,
		log:         slog.Default().With("provider", gw2Name),
		gameManager: gameManager,
		fileManager: fileManager,
	}

	return gw2Provider, nil
}

func (p *GuildWars2Provider) Run() error {
	if !p.gw2Config.IsEnabled() {
		p.log.Warn("Guild Wars 2 provider is not enabled")
		return nil
	}

	if err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

func (p *GuildWars2Provider) GetScreenshots() error {
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

	et, err := exiftool.NewExiftool()
	if err != nil {
		return fmt.Errorf("error initializing exiftool: %w", err)
	}
	defer et.Close()

	game := NewGame(gw2ID, gw2Name, gw2PlatformName, gw2Name)

	for _, file := range files {
		fullPath := filepath.Join(path, file.Name())

		if strings.Contains(file.Name(), gw2Extension) {
			exifTags, err := GetExifTagsWithTool(et, fullPath)
			if err != nil {
				p.log.Error("error retrieving exif tags", slog.Any("err", err), slog.String("file", fullPath))
				continue
			}

			fileDate, err := time.Parse(gw2ExifTimeLayout, exifTags[gw2ExifTimeTag])
			if err != nil {
				p.log.Warn("error parsing media creation time", slog.String("file", fullPath), slog.Any("err", err))
				continue
			}

			destinationName := fileDate.Format(DatetimeFormat) + gw2Extension
			media := NewMedia(MediaKindScreenshot, fullPath)
			media.DestinationName = destinationName
			game.AddScreenshot(media)
		}
	}

	return nil
}

func (p *GuildWars2Provider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *GuildWars2Provider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *GuildWars2Provider) FindGames(options ProviderConfig) ([]Game, error) {
	return nil, nil
}

// getScreenshotsPath returns the path where Guild Wars 2 screenshots are stored
func (p *GuildWars2Provider) getScreenshotsPath() (string, error) {
	if p.gw2Config.GetPath() != "" && p.gw2Config.GetPath() != "auto" {
		return toolkitPaths.ExpandUser(p.gw2Config.GetPath()), nil
	}

	if runtime.GOOS == "windows" {
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("error getting user home directory: %w", err)
		}
		return filepath.Join(userHomeDir, "Documents", "Guild Wars 2", "Screens"), nil
	}

	return "", fmt.Errorf("for non-windows systems the path to the screenshots folder must be provided")
}
