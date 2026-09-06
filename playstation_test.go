package gamesscreenshotmanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePlaystationDatetime(t *testing.T) {
	const layout = "20060102150405"

	tests := []struct {
		name          string
		input         string
		wantTime      string
		wantDuplicate bool
		wantError     bool
	}{
		{
			name:     "plain name",
			input:    "Ghost of Tsushima_20260831083523",
			wantTime: "2026-08-31_08-35-23",
		},
		{
			name:          "duplicate copy",
			input:         "Ghost of Tsushima_20260831083523_1",
			wantTime:      "2026-08-31_08-35-23",
			wantDuplicate: true,
		},
		{
			name:          "duplicate copy with a two digit counter",
			input:         "Ghost of Tsushima_20260831083523_12",
			wantTime:      "2026-08-31_08-35-23",
			wantDuplicate: true,
		},
		{
			name:          "game name with an underscore",
			input:         "Clair Obscur_ Expedition 33_20250603200610_1",
			wantTime:      "2025-06-03_20-06-10",
			wantDuplicate: true,
		},
		{
			// Some titles write digits below the second.
			name:     "timestamp with extra digits",
			input:    "SPACE RUN_2024081217012400",
			wantTime: "2024-08-12_17-01-24",
		},
		{
			name:          "timestamp with extra digits, duplicate copy",
			input:         "SPACE RUN_2024081217012400_1",
			wantTime:      "2024-08-12_17-01-24",
			wantDuplicate: true,
		},
		{
			name:     "game name that ends in a digit",
			input:    "Diablo 4_20260831083523",
			wantTime: "2026-08-31_08-35-23",
		},
		{
			name:     "bare timestamp",
			input:    "20260831083523",
			wantTime: "2026-08-31_08-35-23",
		},
		{
			name:          "bare timestamp, duplicate copy",
			input:         "20260831083523_1",
			wantTime:      "2026-08-31_08-35-23",
			wantDuplicate: true,
		},
		{
			name:      "no timestamp at all",
			input:     "Main Menu",
			wantError: true,
		},
		{
			name:      "too few digits",
			input:     "Game_2026083108",
			wantError: true,
		},
		{
			name:      "digits that are not a date",
			input:     "Game_99999999999999",
			wantError: true,
		},
		{
			name:      "counter with no timestamp in front of it",
			input:     "Main Menu_1",
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			datetime, duplicate, err := parsePlaystationDatetime(test.input, layout)

			if test.wantError {
				if err == nil {
					t.Fatalf("parsePlaystationDatetime(%q) returned no error, want one", test.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("parsePlaystationDatetime(%q) returned an error: %v", test.input, err)
			}

			if got := datetime.Format(DatetimeFormat); got != test.wantTime {
				t.Errorf("parsePlaystationDatetime(%q) time = %q, want %q", test.input, got, test.wantTime)
			}

			if duplicate != test.wantDuplicate {
				t.Errorf("parsePlaystationDatetime(%q) duplicate = %t, want %t", test.input, duplicate, test.wantDuplicate)
			}
		})
	}
}

// A file the provider does not read used to reach an unassigned media pointer,
// and the walk panicked on it.
func TestPlaystation4ProviderSkipsAnUnknownExtension(t *testing.T) {
	base := t.TempDir()
	gameDir := filepath.Join(base, "Bloodborne")
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatalf("failed to create the game directory: %v", err)
	}

	for _, name := range []string{"Thumbs.db", "._Bloodborne_20151215140132.mp4", ".DS_Store"} {
		if err := os.WriteFile(filepath.Join(gameDir, name), []byte("x"), 0644); err != nil {
			t.Fatalf("failed to create %q: %v", name, err)
		}
	}

	enabled := true
	config := Config{Providers: Providers{PlayStation4: ProviderConfig{Enabled: &enabled, Path: &base}}}
	gameManager := NewGameManager()

	provider, err := NewPlaystation4Provider(config, gameManager, NewFileManager(config))
	if err != nil {
		t.Fatalf("NewPlaystation4Provider() returned an error: %v", err)
	}

	if err := provider.GetScreenshots(); err != nil {
		t.Fatalf("GetScreenshots() returned an error: %v", err)
	}

	if games := gameManager.GetGames(); len(games) != 0 {
		t.Errorf("expected no game, found %d", len(games))
	}
}
