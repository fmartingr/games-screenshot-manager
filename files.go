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
	destinationPath := filepath.Join(f.config.OutputPath, normalizeName(game.Platform))
	if game.Name != "" {
		destinationPath = filepath.Join(destinationPath, normalizeName(game.Name))
	} else {
		destinationPath = filepath.Join(destinationPath, normalizeName(game.ID))
	}

	return expandUser(destinationPath)
}

// adoptExistingDir returns the directory to use for path. Where a segment below
// base is missing, but a sibling holds the same name in another Unicode form,
// that sibling is renamed to its NFC name. An earlier run's directory is
// therefore reused, not duplicated beside it.
func (f *FileManager) adoptExistingDir(base, path string) string {
	// Both sides are cleaned first. expandUser leaves a path as it found it,
	// while filepath.Join cleans, so an output_path of "./Output" or "Output/"
	// would otherwise fail this test and skip the adoption entirely.
	base = filepath.Clean(base)
	path = filepath.Clean(path)

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
		if item.Name() == name || normalizeName(item.Name()) != name {
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

	destinationPath := f.adoptExistingDir(expandUser(f.config.OutputPath), f.GetPathForGame(game))
	if media.GetKind() == MediaKindRecording {
		destinationPath = filepath.Join(destinationPath, "recordings")
	}

	if subfolder := media.GetSubfolder(); subfolder != "" {
		destinationPath = filepath.Join(destinationPath, normalizeName(subfolder))
	}

	// Check if folder exists (create otherwise)
	if _, err := os.Stat(destinationPath); os.IsNotExist(err) && !f.config.DryRun {
		mkdirErr := os.MkdirAll(destinationPath, 0711)
		if mkdirErr != nil {
			return fmt.Errorf("Couldn't create directory with name %s, falling back to %s", game.Name, slug.Make(game.Name))
		}
	}

	mediaName := normalizeName(media.GetDestinationName())
	destMediaPath := filepath.Join(destinationPath, mediaName)

	if f.FileExists(destMediaPath) {
		if media.Compare(destMediaPath) {
			return nil
		}

		if media.GetKind() != MediaKindCover {
			destinationName := mediaName
			extestion := filepath.Ext(destinationName)
			destinationName = strings.TrimSuffix(destinationName, extestion)

			// The hash reads the source, so the source has to be on disk
			// before the new name can be built. Remote media is downloaded
			// here rather than further down: an empty hash would put every
			// run's file at the same name, and the copy would then fail on a
			// destination that already exists.
			//
			// A dry run downloads nothing, so its hash stays empty and the
			// name it reports is not the final one.
			if !f.config.DryRun {
				if err := f.ensureLocalSource(media); err != nil {
					return err
				}
			}

			sourceHash := media.GetSourceHash()
			destinationName = fmt.Sprintf("%s_%s%s", destinationName, sourceHash, extestion)
			oldDestMediaPath := destMediaPath
			media.SetDestinationName(destinationName)
			mediaName = destinationName
			destMediaPath = filepath.Join(destinationPath, mediaName)

			// The hash of the source is in the name, so a file at the new path
			// holds this exact content. A later run finds it and stops here,
			// instead of a failure on a destination that exists. The message is
			// a debug one, because every run after the first repeats it. An
			// empty hash names no content, so it cannot stand for the file.
			if sourceHash != "" && f.FileExists(destMediaPath) {
				f.log.Debug(
					"media already exists under its hash name, skipping",
					slog.String("source_path", media.GetSourcePath()),
					slog.String("source_url", media.GetSourceURL()),
					slog.String("destination_path", destMediaPath),
				)

				return nil
			}

			slog.Info(
				"media already exists but hash mismatch, renaming file",
				slog.String("source_path", media.GetSourcePath()),
				slog.String("source_url", media.GetSourceURL()),
				slog.String("old_destination_path", oldDestMediaPath),
				slog.String("new_destination_path", destinationName),
			)
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
		if err := f.ensureLocalSource(media); err != nil {
			return err
		}

		_, err := f.copyFile(media.GetSourcePath(), filepath.Join(destinationPath, mediaName))
		if err != nil {
			return fmt.Errorf("error copying media %s -> %s: %s", media.GetSourcePath(), destMediaPath, err)
		}
	}

	return nil
}

// ensureLocalSource puts a remote media file on disk, so its content can be
// read and copied. It does nothing for a source that is already there, so it
// is safe to call more than once.
//
// A source path is the test, not IsLocal. IsLocal stays false after a
// download, because the URL the file came from is kept.
func (f *FileManager) ensureLocalSource(media MediaFile) error {
	if media.GetSourcePath() != "" {
		return nil
	}

	tempFile, err := f.DownloadURL(media.GetSourceURL())
	if err != nil {
		return fmt.Errorf("error downloading URL: %w", err)
	}

	media.SetSourcePath(tempFile.Name())

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

// WriteFile puts the data at path in one step. It writes a temporary file in
// the same directory and renames it, so a failure part way cannot leave a
// truncated file where a complete one was.
func (f *FileManager) WriteFile(path string, data io.Reader) error {
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %w", err)
	}

	defer os.Remove(temp.Name())

	if _, err := io.Copy(temp, data); err != nil {
		temp.Close()
		return fmt.Errorf("error writing temporary file: %w", err)
	}

	if err := temp.Close(); err != nil {
		return fmt.Errorf("error closing temporary file: %w", err)
	}

	if err := os.Chmod(temp.Name(), 0644); err != nil {
		return fmt.Errorf("error setting the file mode: %w", err)
	}

	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("error moving the file into place: %w", err)
	}

	return nil
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
