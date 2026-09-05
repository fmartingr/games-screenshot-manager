package gamesscreenshotmanager

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// requireNormalizationSensitiveFS skips the test when the filesystem resolves
// the NFC and the NFD form of a name to one file, as APFS and HFS+ do. Such a
// volume cannot hold the two names these tests need.
func requireNormalizationSensitiveFS(t *testing.T, dir string) {
	t.Helper()

	probe := filepath.Join(dir, "probe-e\u0301")
	if err := os.Mkdir(probe, 0755); err != nil {
		t.Fatalf("failed to create the probe directory: %v", err)
	}
	defer os.RemoveAll(probe)

	if _, err := os.Stat(filepath.Join(dir, "probe-\u00e9")); err == nil {
		t.Skip("the filesystem is normalization-insensitive, so it cannot hold both forms")
	}
}

func newTestGalleryBuilder(dryRun bool) *GalleryBuilder {
	return &GalleryBuilder{Config: Config{DryRun: dryRun}, log: slog.Default()}
}

// namesOf maps each returned entry to the name it has on disk.
func namesOf(entries []folderEntry) map[string]bool {
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		names[entry.name] = true
	}
	return names
}

func TestGalleryBuilder_normalizeFolderRenamesToNFC(t *testing.T) {
	nfdDir := "Poke\u0301mon Rojo Fuego"
	nfcDir := "Pok\u00e9mon Rojo Fuego"
	nfdFile := "captura e\u0301pica.jpg"
	nfcFile := "captura \u00e9pica.jpg"

	root := t.TempDir()
	requireNormalizationSensitiveFS(t, root)

	if err := os.Mkdir(filepath.Join(root, nfdDir), 0755); err != nil {
		t.Fatalf("failed to create the NFD directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, nfdFile), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create the NFD file: %v", err)
	}

	entries, err := newTestGalleryBuilder(false).normalizeFolder(root)
	if err != nil {
		t.Fatalf("normalizeFolder() returned an error: %v", err)
	}

	names := namesOf(entries)
	for _, name := range []string{nfcDir, nfcFile} {
		if !names[name] {
			t.Errorf("normalizeFolder() did not return the NFC name %q", name)
		}

		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("expected the NFC name %q on disk: %v", name, err)
		}
	}

	for _, name := range []string{nfdDir, nfdFile} {
		if names[name] {
			t.Errorf("normalizeFolder() returned the NFD name %q", name)
		}

		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("the NFD name %q is still on disk", name)
		}
	}
}

func TestGalleryBuilder_normalizeFolderKeepsBothOnCollision(t *testing.T) {
	nfdDir := "Poke\u0301mon"
	nfcDir := "Pok\u00e9mon"

	root := t.TempDir()
	requireNormalizationSensitiveFS(t, root)

	for _, name := range []string{nfdDir, nfcDir} {
		if err := os.Mkdir(filepath.Join(root, name), 0755); err != nil {
			t.Fatalf("failed to create the directory %q: %v", name, err)
		}
	}

	entries, err := newTestGalleryBuilder(false).normalizeFolder(root)
	if err != nil {
		t.Fatalf("normalizeFolder() returned an error: %v", err)
	}

	names := namesOf(entries)
	for _, name := range []string{nfdDir, nfcDir} {
		if !names[name] {
			t.Errorf("normalizeFolder() did not return the name %q", name)
		}

		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("expected the directory %q to stay on disk: %v", name, err)
		}
	}
}

// A skipped entry must not stop the entries that follow it in the listing.
func TestGalleryBuilder_normalizeFolderContinuesAfterASkip(t *testing.T) {
	nfdEarly := "Ame\u0301lie"
	nfcEarly := "Am\u00e9lie"
	nfdLate := "Zoe\u0301"
	nfcLate := "Zo\u00e9"

	root := t.TempDir()
	requireNormalizationSensitiveFS(t, root)

	for _, name := range []string{nfdEarly, nfcEarly, nfdLate} {
		if err := os.Mkdir(filepath.Join(root, name), 0755); err != nil {
			t.Fatalf("failed to create the directory %q: %v", name, err)
		}
	}

	entries, err := newTestGalleryBuilder(false).normalizeFolder(root)
	if err != nil {
		t.Fatalf("normalizeFolder() returned an error: %v", err)
	}

	if !namesOf(entries)[nfcLate] {
		t.Errorf("the entry after the skipped one was not renamed to %q", nfcLate)
	}

	if _, err := os.Stat(filepath.Join(root, nfcLate)); err != nil {
		t.Errorf("expected the NFC name %q on disk: %v", nfcLate, err)
	}
}

// A sidecar carries the name of its media file, so it must stay with it.
func TestGalleryBuilder_normalizeFolderKeepsASidecarWithItsMediaFile(t *testing.T) {
	nfdMedia := "foto e\u0301.jpg"
	nfcMedia := "foto \u00e9.jpg"
	nfdThumb := "foto e\u0301.jpg.thumb.jpg"
	nfcThumb := "foto \u00e9.jpg.thumb.jpg"

	root := t.TempDir()
	requireNormalizationSensitiveFS(t, root)

	for _, name := range []string{nfdMedia, nfcMedia, nfdThumb} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0644); err != nil {
			t.Fatalf("failed to create the file %q: %v", name, err)
		}
	}

	if _, err := newTestGalleryBuilder(false).normalizeFolder(root); err != nil {
		t.Fatalf("normalizeFolder() returned an error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, nfdThumb)); err != nil {
		t.Errorf("the sidecar left its media file behind: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, nfcThumb)); !os.IsNotExist(err) {
		t.Errorf("the sidecar was renamed to %q while its media file kept its name", nfcThumb)
	}
}

func TestGalleryBuilder_normalizeFolderDryRun(t *testing.T) {
	nfdDir := "Poke\u0301mon"

	root := t.TempDir()
	requireNormalizationSensitiveFS(t, root)

	if err := os.Mkdir(filepath.Join(root, nfdDir), 0755); err != nil {
		t.Fatalf("failed to create the NFD directory: %v", err)
	}

	entries, err := newTestGalleryBuilder(true).normalizeFolder(root)
	if err != nil {
		t.Fatalf("normalizeFolder() returned an error: %v", err)
	}

	// The name must come back unchanged, so the link the gallery writes still
	// matches the file a dry run left alone.
	if !namesOf(entries)[nfdDir] {
		t.Errorf("a dry run did not return the name %q that is on disk", nfdDir)
	}

	if _, err := os.Stat(filepath.Join(root, nfdDir)); err != nil {
		t.Errorf("a dry run renamed %q: %v", nfdDir, err)
	}
}
