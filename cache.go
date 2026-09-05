package gamesscreenshotmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// errCacheKeyNotFound reports a key that is absent or past its expiry time.
var errCacheKeyNotFound = errors.New("cache key does not exist")

// validateKey rejects a key that would place the file outside the cache
// directory. A key names one file, so it holds no separator and no "..".
func validateKey(key string) error {
	if key == "" || key == "." || strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) {
		return fmt.Errorf("invalid cache key %q", key)
	}

	return nil
}

// fileCache holds one payload per key under the user's cache directory. Each
// payload has a sibling ".metadata" file that carries the expiry time.
type fileCache struct {
	path string
}

// cacheMetadata is the sibling file's content. TTL is the moment the payload
// expires. A nil TTL never expires.
type cacheMetadata struct {
	Key string     `json:"Key"`
	TTL *time.Time `json:"TTL,omitempty"`
}

// newFileCache creates the cache directory for name under the user's cache
// directory.
func newFileCache(name string) (*fileCache, error) {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve the user cache directory: %w", err)
	}

	path := filepath.Join(userCacheDir, name)
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("could not create the cache directory: %w", err)
	}

	return &fileCache{path: path}, nil
}

// Get returns the payload for key. Anything it cannot use is a miss, and it
// clears what is left behind. The cache only saves a download, so a damaged
// entry must never fail the run: the caller downloads again and repairs it.
func (c *fileCache) Get(key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	metadata, err := c.readMetadata(key)
	if err != nil {
		// The metadata is missing or damaged, so the payload cannot be dated.
		c.discard(key)
		return nil, errCacheKeyNotFound
	}

	if metadata.TTL != nil && metadata.TTL.Before(time.Now()) {
		c.discard(key)
		return nil, errCacheKeyNotFound
	}

	contents, err := os.ReadFile(c.pathFor(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			c.discard(key)
			return nil, errCacheKeyNotFound
		}
		return nil, fmt.Errorf("error reading the cache file: %w", err)
	}

	return contents, nil
}

// Set writes the payload for key. A ttl of 0 stores it without an expiry time.
//
// The payload lands before the metadata that dates it, and both go through a
// temporary file. An interrupted or concurrent write therefore cannot leave a
// fresh expiry time pointing at a damaged payload.
func (c *fileCache) Set(key string, value []byte, ttl time.Duration) error {
	if err := validateKey(key); err != nil {
		return err
	}

	metadata := cacheMetadata{Key: key}
	if ttl > 0 {
		expiry := time.Now().Add(ttl)
		metadata.TTL = &expiry
	}

	contents, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("error marshalling the cache metadata: %w", err)
	}

	if err := writeFileAtomic(c.pathFor(key), value); err != nil {
		return fmt.Errorf("error writing the cache file: %w", err)
	}

	if err := writeFileAtomic(c.metadataPathFor(key), contents); err != nil {
		return fmt.Errorf("error writing the cache metadata: %w", err)
	}

	return nil
}

// discard removes what is left of a key. It reports nothing, because a cache
// that cannot clean up must still let the run continue.
func (c *fileCache) discard(key string) {
	os.Remove(c.pathFor(key))
	os.Remove(c.metadataPathFor(key))
}

// writeFileAtomic puts content at path in one step. The temporary file carries
// a unique name and sits in the same directory, so two runs cannot write to one
// temporary file and the rename cannot cross a filesystem boundary.
func writeFileAtomic(path string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}

	defer os.Remove(temp.Name())

	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}

	if err := temp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(temp.Name(), 0644); err != nil {
		return err
	}

	return os.Rename(temp.Name(), path)
}

func (c *fileCache) pathFor(key string) string {
	return filepath.Join(c.path, key)
}

func (c *fileCache) metadataPathFor(key string) string {
	return c.pathFor(key) + ".metadata"
}

func (c *fileCache) readMetadata(key string) (cacheMetadata, error) {
	contents, err := os.ReadFile(c.metadataPathFor(key))
	if err != nil {
		return cacheMetadata{}, err
	}

	var metadata cacheMetadata
	if err := json.Unmarshal(contents, &metadata); err != nil {
		return cacheMetadata{}, fmt.Errorf("error unmarshalling the cache metadata: %w", err)
	}

	return metadata, nil
}
