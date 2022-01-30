package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/sirupsen/logrus"
)

type FileCache struct {
	logger *logrus.Entry
	path   string
}

func (c *FileCache) Get(key string) (result string, err error) {
	path := filepath.Join(c.path, key)

	contents, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return result, models.ErrCacheKeyDontExist
		}
		return
	}

	return string(contents), nil
}

func (c *FileCache) GetExpiry(key string, expiration time.Duration) (result string, err error) {
	path := filepath.Join(c.path, key)
	info, err := os.Stat(path)
	if err != nil {
		return result, models.ErrCacheKeyDontExist
	}

	if info.ModTime().Add(expiration).Before(time.Now()) {
		c.Delete(key)
		return result, models.ErrCacheKeyDontExist
	}

	return c.Get(key)
}

func (c *FileCache) Put(key, value string) error {
	path := filepath.Join(c.path, key)

	if err := os.WriteFile(path, []byte(value), 0766); err != nil {
		return fmt.Errorf("error writting cache file: %s", err)
	}

	return nil
}

func (c *FileCache) Delete(key string) error {
	path := filepath.Join(c.path, key)
	return os.Remove(path)
}

func NewFileCache(logger *logrus.Logger) *FileCache {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		logger.Fatalf("error getting cache directory: %s", err)
	}
	path := filepath.Join(userCacheDir, "games-screenshot-manager")

	if err := os.MkdirAll(path, 0755); err != nil {
		logger.Error(err)
	}

	return &FileCache{
		logger: logger.WithField("from", "cache.file"),
		path:   path,
	}
}
