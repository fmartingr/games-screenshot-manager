package gamesscreenshotmanager

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// newSwitch2Provider builds a provider that writes into a folder the test owns.
func newSwitch2Provider(t *testing.T, switch2Config NintendoSwitch2Config) *NintendoSwitch2Provider {
	t.Helper()

	config := Config{}
	config.Defaults()
	config.OutputPath = t.TempDir()

	switch2Config.Enabled = Ptr(true)
	config.Providers.NintendoSwitch2 = switch2Config

	provider, err := NewNintendoSwitch2Provider(config, NewGameManager(), NewFileManager(config))
	if err != nil {
		t.Fatalf("NewNintendoSwitch2Provider() returned an error: %v", err)
	}

	return provider
}

// The console writes a "_c" file for a capture that already exists somewhere
// else, so only a "_s" file is copied.
func TestSwitch2CaptureKind(t *testing.T) {
	cases := []struct {
		name     string
		wantKind MediaKind
		wantOK   bool
	}{
		{name: "2026032619431800_s.jpg", wantKind: MediaKindScreenshot, wantOK: true},
		{name: "2026032619431800_s.mp4", wantKind: MediaKindClip, wantOK: true},
		{name: "2026032619431800_s.JPG", wantKind: MediaKindScreenshot, wantOK: true},
		{name: "2026032619431800_c.jpg", wantOK: false},
		{name: "2026032619431800.jpg", wantOK: false},
		{name: "2026032619431800_s.txt", wantOK: false},
		{name: "_s.jpg", wantKind: MediaKindScreenshot, wantOK: true},
		{name: "", wantOK: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			kind, ok := switch2CaptureKind(testCase.name)

			if ok != testCase.wantOK {
				t.Fatalf("switch2CaptureKind(%q) returned %t, want %t", testCase.name, ok, testCase.wantOK)
			}

			if kind != testCase.wantKind {
				t.Errorf("switch2CaptureKind(%q) returned kind %q, want %q", testCase.name, kind, testCase.wantKind)
			}
		})
	}
}

// The console names its own folder in the language it is set to, so the name is
// config rather than a constant in the code.
func TestNintendoSwitch2ConfigGetIgnoredFolders(t *testing.T) {
	var absent NintendoSwitch2Config
	if got := absent.GetIgnoredFolders(); !slices.Equal(got, switch2DefaultIgnoredFolders) {
		t.Errorf("a config without the key returned %v, want the default %v", got, switch2DefaultIgnoredFolders)
	}

	set := NintendoSwitch2Config{IgnoredFolders: []string{"Other", "Album"}}
	if got := set.GetIgnoredFolders(); !slices.Equal(got, []string{"Other", "Album"}) {
		t.Errorf("a config with the key returned %v, want its own list", got)
	}

	// An empty list is a choice, not an absence: it copies every folder.
	empty := NintendoSwitch2Config{IgnoredFolders: []string{}}
	if got := empty.GetIgnoredFolders(); len(got) != 0 {
		t.Errorf("an empty list returned %v, want nothing", got)
	}
}

// plan reads the listing of the console and decides what to copy.
func TestNintendoSwitch2Plan(t *testing.T) {
	provider := newSwitch2Provider(t, NintendoSwitch2Config{})

	folders := []mtpFolder{
		{ID: "10", Name: "Mario Kart World"},
		{ID: "20", Name: "Otra carpeta"},
	}

	files := []mtpFile{
		{ID: "11", Name: "2025060720031600_s.jpg", Size: 10, ParentID: "10"},
		{ID: "12", Name: "2025060720031700_s.mp4", Size: 20, ParentID: "10"},
		{ID: "13", Name: "2025060720031800_c.jpg", Size: 30, ParentID: "10"},
		{ID: "14", Name: "thumbs.db", Size: 40, ParentID: "10"},
		{ID: "21", Name: "2025060720031900_s.jpg", Size: 50, ParentID: "20"},
		{ID: "31", Name: "2025060720032000_s.jpg", Size: 60, ParentID: "99"},
	}

	stage := t.TempDir()

	pulls := provider.plan(folders, files, stage)

	if len(pulls) != 2 {
		t.Fatalf("plan() asked for %d files, want 2", len(pulls))
	}

	for _, pull := range pulls {
		if pull.File.ID != "11" && pull.File.ID != "12" {
			t.Errorf("plan() asked for file %q, which is not a capture of a game folder", pull.File.ID)
		}

		want := filepath.Join(stage, "10", pull.File.Name)
		if pull.Path != want {
			t.Errorf("file %q is staged at %q, want %q", pull.File.ID, pull.Path, want)
		}
	}

	games := provider.gameManager.GetGames()
	if len(games) != 1 {
		t.Fatalf("plan() filed %d games, want 1", len(games))
	}

	game := games[0]

	if game.Name != "Mario Kart World" || game.Platform != switch2PlatformName {
		t.Errorf("the game is %q on %q, want Mario Kart World on %q", game.Name, game.Platform, switch2PlatformName)
	}

	if len(game.Screenshots) != 1 || len(game.Clips) != 1 {
		t.Fatalf("the game holds %d screenshots and %d clips, want 1 and 1", len(game.Screenshots), len(game.Clips))
	}

	// The capture keeps the name the console gave it.
	if game.Screenshots[0].GetDestinationName() != "2025060720031600_s.jpg" {
		t.Errorf("the screenshot is named %q, want the name of the console", game.Screenshots[0].GetDestinationName())
	}
}

// A capture that was copied on an earlier run stays on the console. This is
// what keeps every run after the first one cheap.
func TestNintendoSwitch2PlanSkipsACaptureThatIsCopied(t *testing.T) {
	provider := newSwitch2Provider(t, NintendoSwitch2Config{})

	folder := filepath.Join(provider.config.OutputPath, "Nintendo Switch 2", "Mario Kart World")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("could not create the destination folder: %v", err)
	}

	if err := os.WriteFile(filepath.Join(folder, "2025060720031600_s.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatalf("could not write the destination file: %v", err)
	}

	folders := []mtpFolder{{ID: "10", Name: "Mario Kart World"}}
	files := []mtpFile{
		{ID: "11", Name: "2025060720031600_s.jpg", Size: 10, ParentID: "10"},
		{ID: "12", Name: "2025060720031700_s.jpg", Size: 20, ParentID: "10"},
	}

	pulls := provider.plan(folders, files, t.TempDir())

	if len(pulls) != 1 {
		t.Fatalf("plan() asked for %d files, want 1", len(pulls))
	}

	if pulls[0].File.ID != "12" {
		t.Errorf("plan() asked for file %q, want the one that is not copied yet", pulls[0].File.ID)
	}
}

// A user on another console language names their own folder to skip.
func TestNintendoSwitch2PlanReadsIgnoredFolders(t *testing.T) {
	provider := newSwitch2Provider(t, NintendoSwitch2Config{IgnoredFolders: []string{"Mario Kart World"}})

	folders := []mtpFolder{
		{ID: "10", Name: "Mario Kart World"},
		{ID: "20", Name: "Otra carpeta"},
	}

	files := []mtpFile{
		{ID: "11", Name: "2025060720031600_s.jpg", Size: 10, ParentID: "10"},
		{ID: "21", Name: "2025060720031900_s.jpg", Size: 50, ParentID: "20"},
	}

	pulls := provider.plan(folders, files, t.TempDir())

	if len(pulls) != 1 {
		t.Fatalf("plan() asked for %d files, want 1", len(pulls))
	}

	if pulls[0].File.ID != "21" {
		t.Errorf("plan() asked for file %q, want the one the config does not skip", pulls[0].File.ID)
	}
}

// A path in the config reads an album that was already copied off a console.
func TestNintendoSwitch2CollectFromFolder(t *testing.T) {
	album := t.TempDir()

	write := func(folder, name string) {
		t.Helper()

		path := filepath.Join(album, folder)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("could not create the album folder: %v", err)
		}

		if err := os.WriteFile(filepath.Join(path, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("could not write the capture: %v", err)
		}
	}

	write("Mario Kart World", "2025060720031600_s.jpg")
	write("Mario Kart World", "2025060720031700_s.mp4")
	write("Mario Kart World", "2025060720031800_c.jpg")
	write("Otra carpeta", "2025060720031900_s.jpg")

	provider := newSwitch2Provider(t, NintendoSwitch2Config{Path: Ptr(album)})

	if err := provider.Run(); err != nil {
		t.Fatalf("Run() returned an error: %v", err)
	}

	games := provider.gameManager.GetGames()
	if len(games) != 1 {
		t.Fatalf("Run() filed %d games, want 1", len(games))
	}

	game := games[0]

	if game.Name != "Mario Kart World" {
		t.Fatalf("the game is %q, want Mario Kart World", game.Name)
	}

	if len(game.Screenshots) != 1 || len(game.Clips) != 1 {
		t.Fatalf("the game holds %d screenshots and %d clips, want 1 and 1", len(game.Screenshots), len(game.Clips))
	}
}

// A console that is away is the normal case. It is reported once, and it fails
// nothing.
func TestNintendoSwitch2RunWithoutAConsole(t *testing.T) {
	useSysfsRoot(t, filepath.Join(t.TempDir(), "missing"))

	provider := newSwitch2Provider(t, NintendoSwitch2Config{})

	if err := provider.Run(); err != nil {
		t.Fatalf("Run() returned an error: %v", err)
	}

	if games := provider.gameManager.GetGames(); len(games) != 0 {
		t.Fatalf("Run() filed %d games, want none", len(games))
	}
}

// The libmtp tools read the console. A path in the config reads a folder
// instead, so none of them is needed then.
func TestNintendoSwitch2Requirements(t *testing.T) {
	provider := newSwitch2Provider(t, NintendoSwitch2Config{})

	requirements := provider.Requirements()
	if len(requirements) != 3 {
		t.Fatalf("Requirements() returned %d programs, want 3", len(requirements))
	}

	for _, requirement := range requirements {
		if requirement.Package != "libmtp" {
			t.Errorf("%s comes from package %q, want libmtp", requirement.Binary, requirement.Package)
		}

		if requirement.Optional {
			t.Errorf("%s is optional, and the provider cannot read the console without it", requirement.Binary)
		}
	}

	fromFolder := newSwitch2Provider(t, NintendoSwitch2Config{Path: Ptr(t.TempDir())})
	if requirements := fromFolder.Requirements(); len(requirements) != 0 {
		t.Fatalf("Requirements() returned %v for a provider that reads a folder, want none", requirements)
	}
}
