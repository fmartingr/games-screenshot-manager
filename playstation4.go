package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
	"github.com/barasher/go-exiftool"
	"github.com/fmartingr/games-screenshot-manager/internal/exif"
)

var _ Provider = (*Playstation4Provider)(nil)

const (
	ps4ID             = "ps4"
	ps4Name           = "playstation-4"
	ps4PlatformName   = "PlayStation 4"
	ps4ExifTimeLayout = "2006:01:02 15:04:05-07:00"
	ps4ExifTimeTag    = "FileModifyDate"
	ps4VideoLayout    = "20060102150405"
)

type Playstation4Provider struct {
	config      Config
	ps4Config   ProviderConfig
	log         *slog.Logger
	gameManager *GameManager
	fileManager *FileManager
}

func NewPlaystation4Provider(config Config) (*Playstation4Provider, error) {
	ps4Provider := &Playstation4Provider{
		config:    config,
		ps4Config: config.Providers.PlayStation4,
		log:       slog.Default().With("provider", ps4Name),
	}

	ps4Provider.gameManager = NewGameManager()
	ps4Provider.fileManager = NewFileManager(config)

	return ps4Provider, nil
}

func (p *Playstation4Provider) Run() error {
	if !p.ps4Config.Enabled {
		p.log.Warn("PlayStation 4 provider is not enabled")
		return nil
	}

	if _, err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

func (p *Playstation4Provider) GetScreenshots() ([]*Game, error) {
	path, err := p.getScreenshotsPath()
	if err != nil {
		return nil, fmt.Errorf("error getting screenshots path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", path)
	}

	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, fmt.Errorf("error initializing exiftool: %w", err)
	}
	defer et.Close()

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

				var media *Media

				var destinationName string
				if extension == ".jpg" {
					exifTags, err := exif.GetTags(et, filePath)
					if err != nil {
						p.log.Error("Error getting EXIF tags", slog.String("file", filePath), slog.Any("error", err))
						return nil
					}

					fileDate, err := time.Parse(ps4ExifTimeLayout, exifTags[ps4ExifTimeTag])
					if err != nil {
						p.log.Warn("Error parsing media creation time", slog.String("file", filePath), slog.Any("error", err))
						return nil
					}
					destinationName = fileDate.Format(DatetimeFormat)
					media = NewMedia(MediaKindScreenshot, filePath)
				} else if extension == ".mp4" {
					if len(fileName) >= len(ps4VideoLayout)+len(extension) {
						videoDatetime, err := time.Parse(ps4VideoLayout, fileName[len(fileName)-len(extension)-len(ps4VideoLayout):len(fileName)-len(extension)])
						if err != nil {
							p.log.Warn("File does not follow datetime convention", slog.String("file", fileName))
							return nil
						}
						destinationName = videoDatetime.Format(DatetimeFormat)
					} else {
						p.log.Warn("File does not follow datetime convention", slog.String("file", fileName))
						return nil
					}
					media = NewMedia(MediaKindClip, filePath)
				}

				media.DestinationName = destinationName + extension

				game := NewGame(ps4ID, ps4Name, ps4PlatformName, gameName)
				game.AddScreenshot(media)
				p.gameManager.AddGame(game)
			}

			return nil
		})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return p.gameManager.GetGames(), nil
}

func (p *Playstation4Provider) GetRecordings() ([]Game, error) {
	return nil, fmt.Errorf("GetRecordings not implemented")
}

func (p *Playstation4Provider) GetClips() ([]Game, error) {
	return nil, fmt.Errorf("GetClips not implemented")
}

func (p *Playstation4Provider) FindGames(options ProviderConfig) ([]Game, error) {
	return nil, nil
}

// getScreenshotsPath returns the path where PlayStation 4 screenshots are stored
func (p *Playstation4Provider) getScreenshotsPath() (string, error) {
	if p.ps4Config.Path != "" && p.ps4Config.Path != "auto" {
		return toolkitPaths.ExpandUser(p.ps4Config.Path), nil
	}

	return "", fmt.Errorf("path to PlayStation 4 screenshots folder must be provided")
}
