package gamesscreenshotmanager

import (
	"os"
	"path/filepath"
	"testing"
)

// newWorldOfWarcraftProvider builds a provider that reads one folder the test
// owns.
func newWorldOfWarcraftProvider(t *testing.T, path string) *WorldOfWarcraftProvider {
	t.Helper()

	config := Config{}
	config.Defaults()
	config.OutputPath = t.TempDir()
	config.Providers.WorldOfWarcraft = ProviderConfig{Enabled: Ptr(true), Path: Ptr(path)}

	provider, err := NewWorldOfWarcraftProvider(config, NewGameManager(), NewFileManager(config))
	if err != nil {
		t.Fatalf("NewWorldOfWarcraftProvider() returned an error: %v", err)
	}

	return provider
}

// The screenshots reach the GameManager. Without the AddGame call the provider
// read the folder, built the game, and dropped it, so an enabled provider
// copied no file and reported no error.
func TestWorldOfWarcraftAddsTheGame(t *testing.T) {
	dir := t.TempDir()

	// The name carries the date, as MMDDYY_HHMMSS.
	for _, name := range []string{"WoWScrnShot_031426_090501.jpg", "WoWScrnShot_040126_112233.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0644); err != nil {
			t.Fatalf("could not write %s: %v", name, err)
		}
	}

	provider := newWorldOfWarcraftProvider(t, dir)

	if err := provider.GetScreenshots(); err != nil {
		t.Fatalf("GetScreenshots() returned an error: %v", err)
	}

	game := provider.gameManager.GetGame(wowID)
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

	for _, want := range []string{"2026-03-14_09-05-01.jpg", "2026-04-01_11-22-33.jpg"} {
		if !names[want] {
			t.Errorf("the screenshots are named %v, want %q among them", names, want)
		}
	}
}

// A folder that holds no screenshot adds no game, so the output holds no empty
// World of Warcraft folder.
func TestWorldOfWarcraftAddsNoGameWithoutScreenshots(t *testing.T) {
	provider := newWorldOfWarcraftProvider(t, t.TempDir())

	if err := provider.GetScreenshots(); err != nil {
		t.Fatalf("GetScreenshots() returned an error: %v", err)
	}

	if game := provider.gameManager.GetGame(wowID); game != nil {
		t.Errorf("GetScreenshots() added a game with %d screenshots, want no game", len(game.Screenshots))
	}
}

// A name the game did not write is reported and skipped, and it stops neither
// the folder nor the run.
func TestWorldOfWarcraftSkipsANameItCannotRead(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		"WoWScrnShot_031426_090501.jpg",
		"WoWScrnShot_notadate.jpg",
		"nounderscore.jpg",
		"notes.txt",
	}

	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0644); err != nil {
			t.Fatalf("could not write %s: %v", name, err)
		}
	}

	provider := newWorldOfWarcraftProvider(t, dir)

	if err := provider.GetScreenshots(); err != nil {
		t.Fatalf("GetScreenshots() returned an error: %v", err)
	}

	game := provider.gameManager.GetGame(wowID)
	if game == nil {
		t.Fatalf("GetScreenshots() added no game")
	}

	if len(game.Screenshots) != 1 {
		t.Errorf("the game holds %d screenshots, want 1", len(game.Screenshots))
	}
}
