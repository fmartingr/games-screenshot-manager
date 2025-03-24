package gamesscreenshotmanager

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
	"github.com/gosimple/slug"
)

var ErrCopyFileDestinationExists = errors.New("copy destination exists")

type FileManager struct {
	config  Config
	log     *slog.Logger
	cleanup []func()
}

func NewFileManager(config Config) *FileManager {
	return &FileManager{config: config, log: slog.Default().With("component", "file_manager"), cleanup: make([]func(), 0)}
}

func (f *FileManager) ProcessGame(game *Game) error {
	f.log.Debug("Processing game", slog.String("game", game.Name))

	for _, screenshot := range game.Screenshots {
		if err := f.ProcessMedia(game, screenshot); err != nil {
			return err
		}
	}

	for _, clip := range game.Clips {
		if err := f.ProcessMedia(game, clip); err != nil {
			return err
		}
	}

	for _, recording := range game.Recordings {
		if err := f.ProcessMedia(game, recording); err != nil {
			return err
		}
	}

	if game.Cover != nil {
		if err := f.ProcessMedia(game, game.Cover); err != nil {
			return err
		}
	}

	return nil
}

func (f *FileManager) GetPathForGame(game *Game) string {
	destinationPath := filepath.Join(f.config.OutputPath, game.Platform)
	if game.Name != "" {
		destinationPath = filepath.Join(destinationPath, game.Name)
	} else {
		destinationPath = filepath.Join(destinationPath, game.ID)
	}

	return toolkitPaths.ExpandUser(destinationPath)
}

func (f *FileManager) ProcessMedia(game *Game, media MediaFile) error {
	f.log.Debug("Processing media", slog.String("path", media.GetPath()), slog.String("destination_name", media.GetDestinationName()))

	destinationPath := f.GetPathForGame(game)
	if media.GetKind() == MediaKindRecording {
		destinationPath = filepath.Join(destinationPath, "recordings")
	}

	// Check if folder exists (create otherwise)
	if _, err := os.Stat(destinationPath); os.IsNotExist(err) && !f.config.DryRun {
		mkdirErr := os.MkdirAll(destinationPath, 0711)
		if mkdirErr != nil {
			return fmt.Errorf("Couldn't create directory with name %s, falling back to %s", game.Name, slug.Make(game.Name))
		}
	}

	destMediaPath := filepath.Join(destinationPath, media.GetDestinationName())

	srcMediaHash, err := f.hashFile(media.GetPath())
	if err != nil {
		return fmt.Errorf("error calculating hash for media: %s", err)
	}

	if f.FileExists(destMediaPath) {
		destMediaHash, err := f.hashFile(destMediaPath)
		if err != nil {
			return fmt.Errorf("error calculating hash for media destination: %s", err)
		}

		if bytes.Equal(srcMediaHash, destMediaHash) {
			// File already processed
			return nil
		} else {
			slog.Warn("media already exists but hash mismatch", slog.String("src", media.GetPath()), slog.String("dst", destMediaPath))
		}
	}

	if f.config.DryRun {
		slog.Info("copy media", slog.String("src", media.GetPath()), slog.String("dst", destMediaPath))
	} else {
		_, err := f.copyFile(media.GetPath(), destMediaPath)
		if err != nil {
			return fmt.Errorf("error copying media: %s", err)
		}
	}

	return nil
}

func (f *FileManager) FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (f *FileManager) copyFile(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, fmt.Errorf("error getting source file stat: %w", err)
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("error opening source file: %w", err)
	}
	defer source.Close()

	// Check if destination exists
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		return 0, ErrCopyFileDestinationExists
	}

	destination, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("error creating destination file: %w", err)
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func (f *FileManager) WriteFile(path string, data io.Reader) error {
	destination, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer destination.Close()
	_, err = io.Copy(destination, data)
	return err
}

func (f *FileManager) WriteFileIfModified(path string, data []byte) error {
	existingData, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error reading existing file: %w", err)
	}

	if bytes.Equal(existingData, data) {
		return nil
	}

	return f.WriteFile(path, bytes.NewReader(data))
}

// DownloadURL downloads a URL to a temporary file and takes care of the cleanup
func (f *FileManager) DownloadURL(url string) (tempFile *os.File, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error downloading URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error downloading URL: %s", resp.Status)
	}

	tempFile, err = os.CreateTemp("", "download_*.jpg")
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	f.cleanup = append(f.cleanup, func() {
		os.Remove(tempFile.Name())
	})

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error copying response body to temp file: %w", err)
	}

	return tempFile, nil
}

func (f *FileManager) Cleanup() error {
	for _, cleanup := range f.cleanup {
		cleanup()
	}

	return nil
}

func (f *FileManager) hashFile(src string) ([]byte, error) {
	handler, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer handler.Close()

	h := md5.New()
	if _, err := io.Copy(h, handler); err != nil {
		return nil, fmt.Errorf("error copying file to hash: %w", err)
	}

	return h.Sum(nil), nil
}
