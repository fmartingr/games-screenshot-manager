package gamesscreenshotmanager

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// newDiabloIVProvider builds a provider that reads one folder the test owns.
func newDiabloIVProvider(t *testing.T, path string) *DiabloIVProvider {
	t.Helper()

	config := Config{}
	config.Defaults()
	config.OutputPath = t.TempDir()
	config.Providers.DiabloIV = ProviderConfig{Enabled: Ptr(true), Path: Ptr(path)}

	provider, err := NewDiabloIVProvider(config, NewGameManager(), NewFileManager(config))
	if err != nil {
		t.Fatalf("NewDiabloIVProvider() returned an error: %v", err)
	}

	return provider
}

// writeScreenshot puts a file in the folder and sets the time it was written.
func writeScreenshot(t *testing.T, dir, name, content string, modTime time.Time) string {
	t.Helper()

	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("could not write %s: %v", path, err)
	}

	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("could not set the time of %s: %v", path, err)
	}

	return path
}

// The game counts its screenshots up from Screenshot001.jpg, and the counter
// starts again on each computer. The name a file is copied under therefore
// comes from the time the file was written.
func TestDiabloIVScreenshotsAreNamedAfterTheirDate(t *testing.T) {
	dir := t.TempDir()

	first := time.Date(2026, 3, 14, 9, 5, 1, 0, time.Local)
	second := time.Date(2026, 3, 14, 21, 47, 30, 0, time.Local)

	writeScreenshot(t, dir, "Screenshot001.jpg", "one", first)
	writeScreenshot(t, dir, "Screenshot002.JPG", "two", second)

	provider := newDiabloIVProvider(t, dir)

	if err := provider.GetScreenshots(); err != nil {
		t.Fatalf("GetScreenshots() returned an error: %v", err)
	}

	game := provider.gameManager.GetGame(d4ID)
	if game == nil {
		t.Fatalf("GetScreenshots() added no game")
	}

	if len(game.Screenshots) != 2 {
		t.Fatalf("the game holds %d screenshots, want 2", len(game.Screenshots))
	}

	names := map[string]bool{}
	for _, screenshot := range game.Screenshots {
		names[screenshot.GetDestinationName()] = true
	}

	// The extension is lowercased, so one game writes one form.
	want := []string{"2026-03-14_09-05-01.jpg", "2026-03-14_21-47-30.jpg"}
	for _, name := range want {
		if !names[name] {
			t.Errorf("the screenshots are named %v, want %q among them", names, name)
		}
	}
}

// Only an image is collected. A folder and any other file are left alone.
func TestDiabloIVCollectsImagesOnly(t *testing.T) {
	dir := t.TempDir()
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)

	writeScreenshot(t, dir, "Screenshot001.jpg", "one", when)
	writeScreenshot(t, dir, "Screenshot002.png", "two", when)
	writeScreenshot(t, dir, "notes.txt", "three", when)

	if err := os.Mkdir(filepath.Join(dir, "Screenshot003.jpg"), 0755); err != nil {
		t.Fatalf("could not create the directory: %v", err)
	}

	provider := newDiabloIVProvider(t, dir)

	if err := provider.GetScreenshots(); err != nil {
		t.Fatalf("GetScreenshots() returned an error: %v", err)
	}

	game := provider.gameManager.GetGame(d4ID)
	if game == nil {
		t.Fatalf("GetScreenshots() added no game")
	}

	if len(game.Screenshots) != 2 {
		t.Fatalf("the game holds %d screenshots, want 2", len(game.Screenshots))
	}
}

// A folder that holds no screenshot adds no game, so the output holds no empty
// Diablo IV folder.
func TestDiabloIVAddsNoGameWithoutScreenshots(t *testing.T) {
	provider := newDiabloIVProvider(t, t.TempDir())

	if err := provider.GetScreenshots(); err != nil {
		t.Fatalf("GetScreenshots() returned an error: %v", err)
	}

	if game := provider.gameManager.GetGame(d4ID); game != nil {
		t.Errorf("GetScreenshots() added a game with %d screenshots, want no game", len(game.Screenshots))
	}
}

// A path that is not there is not an error. The provider reads two default
// folders, and a computer holds one of them.
func TestDiabloIVSkipsAMissingPath(t *testing.T) {
	provider := newDiabloIVProvider(t, filepath.Join(t.TempDir(), "absent"))

	if err := provider.GetScreenshots(); err != nil {
		t.Errorf("GetScreenshots() returned an error: %v", err)
	}

	if game := provider.gameManager.GetGame(d4ID); game != nil {
		t.Errorf("GetScreenshots() added a game with %d screenshots, want no game", len(game.Screenshots))
	}
}

// A folder the provider cannot read is reported, and it stops neither the run
// nor the other folders.
func TestDiabloIVReadsOnPastAnUnreadableFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a folder whatever its mode says")
	}

	dir := t.TempDir()
	when := time.Date(2026, 5, 6, 7, 8, 9, 0, time.Local)
	writeScreenshot(t, dir, "Screenshot001.jpg", "one", when)

	closed := filepath.Join(t.TempDir(), "closed")
	if err := os.Mkdir(closed, 0000); err != nil {
		t.Fatalf("could not create the directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(closed, 0755) })

	provider := newDiabloIVProvider(t, dir)
	// Both folders are read, and the one that refuses is only logged.
	game := NewGame(d4ID, d4Name, d4PlatformName, d4Name)

	if err := provider.processScreenshotsPath(closed, game); err == nil {
		t.Errorf("processScreenshotsPath() returned no error for a folder it cannot read")
	}

	if err := provider.processScreenshotsPath(dir, game); err != nil {
		t.Fatalf("processScreenshotsPath() returned an error: %v", err)
	}

	if len(game.Screenshots) != 1 {
		t.Errorf("the game holds %d screenshots, want 1", len(game.Screenshots))
	}
}

// The path in the config takes precedence over the default folders.
func TestDiabloIVGetScreenshotsPaths(t *testing.T) {
	provider := newDiabloIVProvider(t, "~/Pictures/D4")

	paths := provider.getScreenshotsPaths()

	want := expandUser("~/Pictures/D4")
	if len(paths) != 1 || paths[0] != want {
		t.Errorf("getScreenshotsPaths() = %v, want [%s]", paths, want)
	}
}

// A config that asks for the default folders gets the Windows ones. Every other
// platform gets nothing, because the game writes no folder of its own there.
func TestDiabloIVGetScreenshotsPathsAuto(t *testing.T) {
	provider := newDiabloIVProvider(t, "auto")

	paths := provider.getScreenshotsPaths()

	if runtime.GOOS != "windows" {
		if len(paths) != 0 {
			t.Errorf("getScreenshotsPaths() = %v, want nothing on %s", paths, runtime.GOOS)
		}

		return
	}

	if len(paths) != len(d4DefaultPaths) {
		t.Errorf("getScreenshotsPaths() = %v, want %v", paths, d4DefaultPaths)
	}
}

// A platform without a default folder collects nothing, and it is not a
// failure. A config that never named this game therefore stays quiet.
func TestDiabloIVAutoOnAPlatformWithoutADefaultFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows holds the default folders")
	}

	provider := newDiabloIVProvider(t, "auto")

	if err := provider.GetScreenshots(); err != nil {
		t.Errorf("GetScreenshots() returned an error: %v", err)
	}

	if game := provider.gameManager.GetGame(d4ID); game != nil {
		t.Errorf("GetScreenshots() added a game, want no game")
	}
}
