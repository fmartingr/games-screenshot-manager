package gamesscreenshotmanager

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	destinationPath := filepath.Join(f.config.OutputPath, NormalizeName(game.Platform))
	if game.Name != "" {
		destinationPath = filepath.Join(destinationPath, NormalizeName(game.Name))
	} else {
		destinationPath = filepath.Join(destinationPath, NormalizeName(game.ID))
	}

	return toolkitPaths.ExpandUser(destinationPath)
}

// adoptExistingDir returns the directory to use for path. Where a segment below
// base is missing, but a sibling holds the same name in another Unicode form,
// that sibling is renamed to its NFC name. An earlier run's directory is
// therefore reused, not duplicated beside it.
func (f *FileManager) adoptExistingDir(base, path string) string {
	if !strings.HasPrefix(path, base+string(filepath.Separator)) {
		return path
	}

	parent := f.adoptExistingDir(base, filepath.Dir(path))
	name := filepath.Base(path)
	path = filepath.Join(parent, name)

	if f.FileExists(path) {
		return path
	}

	items, err := os.ReadDir(parent)
	if err != nil {
		return path
	}

	for _, item := range items {
		if item.Name() == name || NormalizeName(item.Name()) != name {
			continue
		}

		oldPath := filepath.Join(parent, item.Name())

		if f.config.DryRun {
			f.log.Info("rename to NFC form", slog.String("old_path", oldPath), slog.String("new_path", path))
			return oldPath
		}

		if err := renameToNFC(parent, item.Name(), name); err != nil {
			f.log.Error("failed to rename to NFC form", slog.String("path", oldPath), slog.String("err", err.Error()))
			return oldPath
		}

		f.log.Info("renamed to NFC form", slog.String("old_path", oldPath), slog.String("new_path", path))

		return path
	}

	return path
}

func (f *FileManager) ProcessMedia(game *Game, media MediaFile) error {
	f.log.Debug(
		"Processing media",
		slog.Bool("is_local", media.IsLocal()),
		slog.String("source_path", media.GetSourcePath()),
		slog.String("source_url", media.GetSourceURL()),
		slog.String("destination_name", media.GetDestinationName()),
	)

	destinationPath := f.adoptExistingDir(toolkitPaths.ExpandUser(f.config.OutputPath), f.GetPathForGame(game))
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

	mediaName := NormalizeName(media.GetDestinationName())
	destMediaPath := filepath.Join(destinationPath, mediaName)

	if f.FileExists(destMediaPath) {
		if media.Compare(destMediaPath) {
			return nil
		}

		if media.GetKind() != MediaKindCover {
			destinationName := mediaName
			extestion := filepath.Ext(destinationName)
			destinationName = strings.TrimSuffix(destinationName, extestion)
			destinationName = fmt.Sprintf("%s_%s%s", destinationName, media.GetSourceHash(), extestion)
			slog.Warn(
				"media already exists but hash mismatch, renaming file",
				slog.String("source_path", media.GetSourcePath()),
				slog.String("source_url", media.GetSourceURL()),
				slog.String("old_destination_path", destMediaPath),
				slog.String("new_destination_path", destinationName),
			)
			media.SetDestinationName(destinationName)
			mediaName = destinationName
			destMediaPath = filepath.Join(destinationPath, mediaName)
		} else {
			slog.Warn(
				"cover already exists but hash mismatch, skipping",
				slog.String("source_path", media.GetSourcePath()),
				slog.String("source_url", media.GetSourceURL()),
				slog.String("destination_path", destMediaPath),
			)

			return nil
		}
	}

	if f.config.DryRun {
		slog.Info("copy media", slog.String("src", media.GetSourcePath()), slog.String("dst", destMediaPath))
	} else {
		if !media.IsLocal() {
			tempFile, err := f.DownloadURL(media.GetSourceURL())
			if err != nil {
				return fmt.Errorf("error downloading URL: %w", err)
			}

			media.SetSourcePath(tempFile.Name())
		}

		_, err := f.copyFile(media.GetSourcePath(), filepath.Join(destinationPath, mediaName))
		if err != nil {
			return fmt.Errorf("error copying media %s -> %s: %s", media.GetSourcePath(), destMediaPath, err)
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
