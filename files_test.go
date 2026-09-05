package gamesscreenshotmanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileManager_GetPathForGame(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		gameName string
		gameID   string
		expected string
	}{
		{
			name:     "ASCII name",
			platform: "PC",
			gameName: "Terraria",
			expected: filepath.Join("Output", "PC", "Terraria"),
		},
		{
			name:     "NFD game name becomes NFC",
			platform: "Game Boy Advance",
			gameName: "Poke\u0301mon Rojo Fuego",
			expected: filepath.Join("Output", "Game Boy Advance", "Pok\u00e9mon Rojo Fuego"),
		},
		{
			name:     "NFD platform name becomes NFC",
			platform: "Nintendo DSi\u0301",
			gameName: "Terraria",
			expected: filepath.Join("Output", "Nintendo DS\u00ed", "Terraria"),
		},
		{
			name:     "NFD game ID becomes NFC when the name is empty",
			platform: "PC",
			gameID:   "Poke\u0301mon",
			expected: filepath.Join("Output", "PC", "Pok\u00e9mon"),
		},
	}

	manager := NewFileManager(Config{OutputPath: "Output"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := &Game{ID: tt.gameID, Name: tt.gameName, Platform: tt.platform}
			if result := manager.GetPathForGame(game); result != tt.expected {
				t.Errorf("GetPathForGame() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFileManager_adoptExistingDir(t *testing.T) {
	nfdPlatform := "Nintendo DSi\u0301"
	nfcPlatform := "Nintendo DS\u00ed"
	nfdGame := "Poke\u0301mon Rojo Fuego"
	nfcGame := "Pok\u00e9mon Rojo Fuego"

	base := t.TempDir()
	requireNormalizationSensitiveFS(t, base)

	existing := filepath.Join(base, nfdPlatform, nfdGame)
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatalf("failed to create the NFD directories: %v", err)
	}

	if err := os.WriteFile(filepath.Join(existing, "shot.jpg"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create the screenshot: %v", err)
	}

	manager := NewFileManager(Config{OutputPath: base})
	wanted := filepath.Join(base, nfcPlatform, nfcGame)

	if result := manager.adoptExistingDir(base, wanted); result != wanted {
		t.Errorf("adoptExistingDir() = %q, want %q", result, wanted)
	}

	// The screenshot must still be there, so nothing was copied a second time.
	if _, err := os.Stat(filepath.Join(wanted, "shot.jpg")); err != nil {
		t.Errorf("the existing directory was not adopted: %v", err)
	}

	for _, name := range []string{nfdPlatform, filepath.Join(nfcPlatform, nfdGame)} {
		if _, err := os.Stat(filepath.Join(base, name)); !os.IsNotExist(err) {
			t.Errorf("the NFD directory %q is still on disk", name)
		}
	}
}

func TestFileManager_adoptExistingDirDryRun(t *testing.T) {
	nfdGame := "Poke\u0301mon"
	nfcGame := "Pok\u00e9mon"

	base := t.TempDir()
	requireNormalizationSensitiveFS(t, base)

	existing := filepath.Join(base, nfdGame)
	if err := os.Mkdir(existing, 0755); err != nil {
		t.Fatalf("failed to create the NFD directory: %v", err)
	}

	manager := NewFileManager(Config{OutputPath: base, DryRun: true})

	// A dry run must point at the directory that is there, not at a name it
	// declined to create.
	if result := manager.adoptExistingDir(base, filepath.Join(base, nfcGame)); result != existing {
		t.Errorf("adoptExistingDir() = %q, want %q", result, existing)
	}

	if _, err := os.Stat(existing); err != nil {
		t.Errorf("a dry run renamed the directory: %v", err)
	}
}

func TestFileManager_adoptExistingDirLeavesANewPathAlone(t *testing.T) {
	base := t.TempDir()

	manager := NewFileManager(Config{OutputPath: base})
	wanted := filepath.Join(base, "PC", "Terraria")

	if result := manager.adoptExistingDir(base, wanted); result != wanted {
		t.Errorf("adoptExistingDir() = %q, want %q", result, wanted)
	}
}

func TestFileManager_ProcessMediaReusesAnNFDDirectory(t *testing.T) {
	nfdGame := "Poke\u0301mon Rojo Fuego"
	nfcGame := "Pok\u00e9mon Rojo Fuego"

	base := t.TempDir()
	requireNormalizationSensitiveFS(t, base)

	existing := filepath.Join(base, "Game Boy Advance", nfdGame)
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatalf("failed to create the NFD directory: %v", err)
	}

	source := filepath.Join(t.TempDir(), "shot.jpg")
	if err := os.WriteFile(source, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create the source file: %v", err)
	}

	manager := NewFileManager(Config{OutputPath: base})
	game := NewGame("gba-1", nfdGame, "Game Boy Advance", "manual")
	media := NewMedia(MediaKindScreenshot, source)
	media.DestinationName = "shot.jpg"

	if err := manager.ProcessMedia(game, media); err != nil {
		t.Fatalf("ProcessMedia() returned an error: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(base, "Game Boy Advance"))
	if err != nil {
		t.Fatalf("failed to read the platform directory: %v", err)
	}

	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("expected one game directory, found %d: %q", len(entries), names)
	}

	if entries[0].Name() != nfcGame {
		t.Errorf("game directory = %q, want the NFC name %q", entries[0].Name(), nfcGame)
	}

	if _, err := os.Stat(filepath.Join(base, "Game Boy Advance", nfcGame, "shot.jpg")); err != nil {
		t.Errorf("the screenshot did not land in the adopted directory: %v", err)
	}
}

// expandUser leaves a path as it found it while filepath.Join cleans, so an
// output_path written with "./" or a trailing slash used to skip the adoption
// and duplicate the game directory.
func TestFileManager_adoptExistingDirWithAnUncleanOutputPath(t *testing.T) {
	nfdGame := "Poke\u0301mon Rojo Fuego"
	nfcGame := "Pok\u00e9mon Rojo Fuego"

	for _, shape := range []string{"Output", "./Output", "Output/", "./Output/"} {
		t.Run(shape, func(t *testing.T) {
			root := t.TempDir()
			requireNormalizationSensitiveFS(t, root)

			// Work relative to the temporary directory, so the unclean shapes
			// are the ones the config would carry.
			t.Chdir(root)

			existing := filepath.Join("Output", "PC", nfdGame)
			if err := os.MkdirAll(existing, 0755); err != nil {
				t.Fatalf("failed to create the NFD directory: %v", err)
			}

			manager := NewFileManager(Config{OutputPath: shape})
			game := &Game{Name: nfdGame, Platform: "PC"}

			result := manager.adoptExistingDir(expandUser(shape), manager.GetPathForGame(game))

			if _, err := os.Stat(result); err != nil {
				t.Fatalf("adoptExistingDir() = %q, which is not on disk: %v", result, err)
			}

			entries, err := os.ReadDir(filepath.Join("Output", "PC"))
			if err != nil {
				t.Fatalf("failed to read the platform directory: %v", err)
			}

			if len(entries) != 1 {
				t.Fatalf("expected one game directory, found %d", len(entries))
			}

			if entries[0].Name() != nfcGame {
				t.Errorf("game directory = %q, want the NFC name %q", entries[0].Name(), nfcGame)
			}
		})
	}
}
