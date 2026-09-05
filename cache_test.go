package gamesscreenshotmanager

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestCache(t *testing.T) *fileCache {
	t.Helper()
	return &fileCache{path: t.TempDir()}
}

func TestFileCache_SetAndGet(t *testing.T) {
	cache := newTestCache(t)
	payload := []byte("steam-app-list")

	if err := cache.Set("apps", payload, 0); err != nil {
		t.Fatalf("Set() returned an error: %v", err)
	}

	result, err := cache.Get("apps")
	if err != nil {
		t.Fatalf("Get() returned an error: %v", err)
	}

	if string(result) != string(payload) {
		t.Errorf("Get() = %q, want %q", result, payload)
	}
}

func TestFileCache_GetReportsAMissingKey(t *testing.T) {
	cache := newTestCache(t)

	if _, err := cache.Get("absent"); !errors.Is(err, errCacheKeyNotFound) {
		t.Errorf("Get() error = %v, want errCacheKeyNotFound", err)
	}
}

func TestFileCache_GetDeletesAnExpiredKey(t *testing.T) {
	cache := newTestCache(t)

	if err := cache.Set("stale", []byte("x"), time.Hour); err != nil {
		t.Fatalf("Set() returned an error: %v", err)
	}

	expiry := time.Now().Add(-time.Minute)
	contents, err := json.Marshal(cacheMetadata{Key: "stale", TTL: &expiry})
	if err != nil {
		t.Fatalf("failed to marshal the metadata: %v", err)
	}

	if err := os.WriteFile(cache.metadataPathFor("stale"), contents, 0644); err != nil {
		t.Fatalf("failed to write the metadata: %v", err)
	}

	if _, err := cache.Get("stale"); !errors.Is(err, errCacheKeyNotFound) {
		t.Errorf("Get() error = %v, want errCacheKeyNotFound", err)
	}

	for _, path := range []string{cache.pathFor("stale"), cache.metadataPathFor("stale")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("the expired file %q is still on disk", filepath.Base(path))
		}
	}
}

// A cache written by an earlier release carries a "Value" field. It must still
// load, so an existing Steam app list is not downloaded again.
func TestFileCache_GetReadsMetadataFromAnEarlierRelease(t *testing.T) {
	cache := newTestCache(t)
	expiry := time.Now().Add(time.Hour)

	metadata := struct {
		Key   string     `json:"Key"`
		Value any        `json:"Value"`
		TTL   *time.Time `json:"TTL"`
	}{Key: "apps", Value: nil, TTL: &expiry}

	contents, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal the metadata: %v", err)
	}

	if err := os.WriteFile(cache.metadataPathFor("apps"), contents, 0644); err != nil {
		t.Fatalf("failed to write the metadata: %v", err)
	}

	if err := os.WriteFile(cache.pathFor("apps"), []byte("payload"), 0644); err != nil {
		t.Fatalf("failed to write the payload: %v", err)
	}

	result, err := cache.Get("apps")
	if err != nil {
		t.Fatalf("Get() returned an error: %v", err)
	}

	if string(result) != "payload" {
		t.Errorf("Get() = %q, want %q", result, "payload")
	}
}

func TestFileCache_GetKeepsAKeyInsideItsTTL(t *testing.T) {
	cache := newTestCache(t)

	if err := cache.Set("fresh", []byte("x"), time.Hour); err != nil {
		t.Fatalf("Set() returned an error: %v", err)
	}

	if _, err := cache.Get("fresh"); err != nil {
		t.Errorf("Get() returned an error: %v", err)
	}
}

// The payload must be in place before the metadata that dates it, and Set must
// leave no temporary file behind.
func TestFileCache_SetLeavesNoTemporaryFile(t *testing.T) {
	cache := newTestCache(t)

	if err := cache.Set("apps", []byte("payload"), time.Hour); err != nil {
		t.Fatalf("Set() returned an error: %v", err)
	}

	entries, err := os.ReadDir(cache.path)
	if err != nil {
		t.Fatalf("failed to read the cache directory: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("Set() left the temporary file %q behind", entry.Name())
		}
	}

	if len(names) != 2 {
		t.Errorf("cache directory holds %q, want the payload and its metadata", names)
	}
}

// Metadata without a payload must report a miss, not a corrupt hit.
func TestFileCache_GetReportsAMissingPayload(t *testing.T) {
	cache := newTestCache(t)

	if err := cache.Set("apps", []byte("payload"), time.Hour); err != nil {
		t.Fatalf("Set() returned an error: %v", err)
	}

	if err := os.Remove(cache.pathFor("apps")); err != nil {
		t.Fatalf("failed to remove the payload: %v", err)
	}

	if _, err := cache.Get("apps"); !errors.Is(err, errCacheKeyNotFound) {
		t.Errorf("Get() error = %v, want errCacheKeyNotFound", err)
	}

	if _, err := os.Stat(cache.metadataPathFor("apps")); !os.IsNotExist(err) {
		t.Errorf("the orphan metadata is still on disk")
	}
}

// A key names one file. It must never place that file outside the cache
// directory.
func TestFileCache_RejectsAKeyThatEscapesTheDirectory(t *testing.T) {
	cache := newTestCache(t)

	keys := []string{
		"../escaped",
		"../../.ssh/authorized_keys",
		"nested/key",
		`windows\key`,
		"..",
		".",
		"",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			if err := cache.Set(key, []byte("x"), 0); err == nil {
				t.Errorf("Set(%q) returned no error", key)
			}

			if _, err := cache.Get(key); err == nil {
				t.Errorf("Get(%q) returned no error", key)
			}
		})
	}

	// Nothing may have been written next to the cache directory.
	if _, err := os.Stat(filepath.Join(cache.path, "..", "escaped")); !os.IsNotExist(err) {
		t.Errorf("a rejected key still wrote outside the cache directory")
	}
}

// A plain key keeps working.
func TestFileCache_AcceptsAPlainKey(t *testing.T) {
	cache := newTestCache(t)

	if err := cache.Set("steam-applist", []byte("x"), 0); err != nil {
		t.Errorf("Set() returned an error: %v", err)
	}
}

// Damaged metadata must be a miss, not a fatal error. Without this the Steam
// provider stays broken until the user deletes the file by hand.
func TestFileCache_GetHealsDamagedMetadata(t *testing.T) {
	cache := newTestCache(t)

	if err := cache.Set("apps", []byte("payload"), time.Hour); err != nil {
		t.Fatalf("Set() returned an error: %v", err)
	}

	if err := os.WriteFile(cache.metadataPathFor("apps"), []byte("{tru"), 0644); err != nil {
		t.Fatalf("failed to damage the metadata: %v", err)
	}

	if _, err := cache.Get("apps"); !errors.Is(err, errCacheKeyNotFound) {
		t.Errorf("Get() error = %v, want errCacheKeyNotFound", err)
	}

	for _, path := range []string{cache.pathFor("apps"), cache.metadataPathFor("apps")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("the damaged key left %q on disk", filepath.Base(path))
		}
	}
}

// A payload with no metadata cannot be dated, so it must not sit in the cache
// directory for good.
func TestFileCache_GetClearsAnOrphanPayload(t *testing.T) {
	cache := newTestCache(t)

	if err := cache.Set("apps", []byte("payload"), time.Hour); err != nil {
		t.Fatalf("Set() returned an error: %v", err)
	}

	if err := os.Remove(cache.metadataPathFor("apps")); err != nil {
		t.Fatalf("failed to remove the metadata: %v", err)
	}

	if _, err := cache.Get("apps"); !errors.Is(err, errCacheKeyNotFound) {
		t.Errorf("Get() error = %v, want errCacheKeyNotFound", err)
	}

	if _, err := os.Stat(cache.pathFor("apps")); !os.IsNotExist(err) {
		t.Errorf("the orphan payload is still on disk")
	}
}
