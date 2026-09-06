package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/barasher/go-exiftool"
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

func NewPlaystation4Provider(config Config, gameManager *GameManager, fileManager *FileManager) (*Playstation4Provider, error) {
	ps4Provider := &Playstation4Provider{
		config:      config,
		ps4Config:   config.Providers.PlayStation4,
		log:         slog.Default().With("provider", ps4Name),
		gameManager: gameManager,
		fileManager: fileManager,
	}

	return ps4Provider, nil
}

func (p *Playstation4Provider) Run() error {
	if !p.ps4Config.IsEnabled() {
		p.log.Warn("PlayStation 4 provider is not enabled")
		return nil
	}

	if err := p.GetScreenshots(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

func (p *Playstation4Provider) GetScreenshots() error {
	path, err := p.getScreenshotsPath()
	if err != nil {
		return fmt.Errorf("error getting screenshots path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", path)
	}

	et, err := exiftool.NewExiftool()
	if err != nil {
		return fmt.Errorf("error initializing exiftool: %w", err)
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

				// Ignore hidden files. A macOS AppleDouble stub beside a clip
				// carries the same extension, and it is not a clip.
				if strings.HasPrefix(fileName, ".") {
					return nil
				}

				var media *Media

				var destinationName string
				var subfolder string

				if extension == ".jpg" {
					exifTags, err := GetExifTagsWithTool(et, filePath)
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
					baseName := strings.TrimSuffix(fileName, extension)

					videoDatetime, isDuplicate, err := parsePlaystationDatetime(baseName, ps4VideoLayout)
					if err != nil {
						// The console names a capture after the scene the game
						// reports, and such a name holds no time. The name is the
						// only fact about the capture, so the file keeps it.
						p.log.Info("Filename holds no capture time, the name is kept", slog.String("file", fileName))
						destinationName = playstationUndatedPrefix + baseName
						subfolder = playstationUndatedFolder
					} else {
						if isDuplicate {
							// The file gets the destination name of the copy the
							// counter counts from. The file manager decides on the
							// content: a match is skipped, and a mismatch is written
							// beside the first copy under a hash suffix.
							p.log.Debug("Duplicate copy of a clip", slog.String("file", fileName))
						}

						destinationName = videoDatetime.Format(DatetimeFormat)
					}

					media = NewMedia(MediaKindClip, filePath)
				}

				// Neither branch above ran, so the extension is one this
				// provider does not read.
				if media == nil {
					p.log.Debug("File is not a screenshot or a clip", slog.String("file", fileName))
					return nil
				}

				media.DestinationName = destinationName + extension
				media.Subfolder = subfolder

				game := p.gameManager.AddGame(NewGame(gameName, gameName, ps4PlatformName, ps4ID))
				game.AddScreenshot(media)
			}

			return nil
		})

	if err != nil {
		return fmt.Errorf("error walking directory: %w", err)
	}

	return nil
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
	if p.ps4Config.GetPath() != "" && p.ps4Config.GetPath() != "auto" {
		return expandUser(p.ps4Config.GetPath()), nil
	}

	return "", fmt.Errorf("path to PlayStation 4 screenshots folder must be provided")
}
