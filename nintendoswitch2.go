package gamesscreenshotmanager

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var _ Provider = (*NintendoSwitch2Provider)(nil)

const (
	switch2ID           = "nintendo-switch-2"
	switch2Name         = "nintendo-switch-2"
	switch2PlatformName = "Nintendo Switch 2"

	// nintendoVendorID is the USB vendor ID of Nintendo.
	nintendoVendorID = "057e"

	// switch2AlbumProductID is what the console reports while it shares its
	// album with a computer. In its normal mode it reports 2060 and offers a
	// HID interface only, so the product ID is what says the album is there.
	switch2AlbumProductID = "2061"

	// switch2CaptureSuffix marks the capture the provider copies. The console
	// also writes a "_c" file, which is a copy of a capture that already exists
	// somewhere else, so only "_s" is taken.
	switch2CaptureSuffix = "_s"
)

// switch2Extensions are the file types the console writes. A screenshot is a
// JPEG, and a clip is an MP4.
var switch2Extensions = map[string]MediaKind{
	".jpg": MediaKindScreenshot,
	".mp4": MediaKindClip,
}

// NintendoSwitch2Provider collects the album of a Nintendo Switch 2.
//
// The console shares its album over MTP, and the provider reads it through the
// libmtp command line tools. A path in the config reads a folder instead, which
// is how an album that was already copied off the console is collected.
type NintendoSwitch2Provider struct {
	config        Config
	switch2Config NintendoSwitch2Config
	log           *slog.Logger
	gameManager   *GameManager
	fileManager   *FileManager
	client        *mtpClient
}

func NewNintendoSwitch2Provider(config Config, gameManager *GameManager, fileManager *FileManager) (*NintendoSwitch2Provider, error) {
	log := slog.Default().With("provider", switch2Name)

	return &NintendoSwitch2Provider{
		config:        config,
		switch2Config: config.Providers.NintendoSwitch2,
		log:           log,
		gameManager:   gameManager,
		fileManager:   fileManager,
		client:        newMTPClient(log),
	}, nil
}

func (p *NintendoSwitch2Provider) Run() error {
	if !p.switch2Config.IsEnabled() {
		p.log.Warn("Nintendo Switch 2 provider is not enabled")
		return nil
	}

	if path := p.switch2Config.GetPath(); path != "" && path != "auto" {
		if err := p.collectFromFolder(expandUser(path)); err != nil {
			p.log.Error("Failed to get screenshots", slog.Any("error", err))
		}

		return nil
	}

	if err := p.collectFromConsole(); err != nil {
		p.log.Error("Failed to get screenshots", slog.Any("error", err))
	}

	return nil
}

// collectFromConsole reads the album of a connected console.
//
// A console that is away is the normal case, so it is reported once at info
// level and it fails nothing. A console in its normal mode reads the same way:
// it offers no album, so there is nothing to collect.
func (p *NintendoSwitch2Provider) collectFromConsole() error {
	device, found := findUSBDevice(nintendoVendorID, switch2AlbumProductID)
	if !found {
		p.log.Info("no Nintendo Switch 2 shares its album, skipping")
		return nil
	}

	p.log.Info("found a Nintendo Switch 2", slog.String("serial", device.Serial))

	folders, err := p.client.Folders()
	if err != nil {
		return describeMTPError(err)
	}

	files, err := p.client.Files()
	if err != nil {
		return describeMTPError(err)
	}

	stage, err := os.MkdirTemp("", "nintendo_switch_2_*")
	if err != nil {
		return fmt.Errorf("error creating the staging folder: %w", err)
	}

	p.fileManager.AddCleanup(func() {
		if err := os.RemoveAll(stage); err != nil {
			p.log.Error("failed to remove the staging folder", slog.String("path", stage), slog.Any("error", err))
		}
	})

	pulls := p.plan(folders, files, stage)

	p.log.Info(
		"planned the copy",
		slog.Int("captures", len(files)),
		slog.Int("to_copy", len(pulls)),
	)

	if len(pulls) == 0 {
		return nil
	}

	if p.config.DryRun {
		for _, pull := range pulls {
			p.log.Info("pull capture", slog.String("file", pull.File.Name), slog.String("id", pull.File.ID))
		}

		return nil
	}

	if err := p.client.Pull(pulls); err != nil {
		return describeMTPError(err)
	}

	return nil
}

// plan decides which capture is copied off the console, and where it is staged.
//
// A capture whose destination already holds a file is left on the console. The
// name of a capture is a timestamp the console never repeats, so a file that is
// already there is that capture. This is what keeps every run after the first
// one cheap.
func (p *NintendoSwitch2Provider) plan(folders []mtpFolder, files []mtpFile, stage string) []mtpPull {
	ignored := p.ignoredFolders()

	names := make(map[string]string, len(folders))
	for _, folder := range folders {
		if _, skip := ignored[normalizeName(folder.Name)]; skip {
			p.log.Debug("skipping an ignored album folder", slog.String("folder", folder.Name))
			continue
		}

		names[folder.ID] = folder.Name
	}

	var pulls []mtpPull

	for _, file := range files {
		gameName, ok := names[file.ParentID]
		if !ok {
			continue
		}

		kind, ok := switch2CaptureKind(file.Name)
		if !ok {
			continue
		}

		game := p.gameManager.AddGame(NewGame(gameName, gameName, switch2PlatformName, switch2ID))

		if p.fileManager.FileExists(p.destinationPath(game, file.Name)) {
			continue
		}

		// The staging path holds the folder ID, because two games can hold a
		// capture of the same name.
		path := filepath.Join(stage, file.ParentID, file.Name)

		media := NewMedia(kind, path)
		media.DestinationName = file.Name

		p.addMedia(game, media)

		pulls = append(pulls, mtpPull{File: file, Path: path})
	}

	return pulls
}

// collectFromFolder reads an album that was already copied off a console. The
// layout is the one the console shares: one folder per game, and the captures
// inside it.
func (p *NintendoSwitch2Provider) collectFromFolder(root string) error {
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("directory %s cannot be read: %w", root, err)
	}

	ignored := p.ignoredFolders()

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %w", root, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if _, skip := ignored[normalizeName(entry.Name())]; skip {
			p.log.Debug("skipping an ignored album folder", slog.String("folder", entry.Name()))
			continue
		}

		if err := p.collectGameFolder(filepath.Join(root, entry.Name()), entry.Name()); err != nil {
			p.log.Error("failed to read an album folder", slog.String("folder", entry.Name()), slog.Any("error", err))
		}
	}

	return nil
}

// collectGameFolder reads the captures of one game folder.
func (p *NintendoSwitch2Provider) collectGameFolder(path, gameName string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %w", path, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		kind, ok := switch2CaptureKind(entry.Name())
		if !ok {
			continue
		}

		media := NewMedia(kind, filepath.Join(path, entry.Name()))
		media.DestinationName = entry.Name()

		game := p.gameManager.AddGame(NewGame(gameName, gameName, switch2PlatformName, switch2ID))

		p.addMedia(game, media)
	}

	return nil
}

// addMedia files one capture under the game, by its kind.
func (p *NintendoSwitch2Provider) addMedia(game *Game, media *Media) {
	if media.Kind == MediaKindClip {
		game.AddClip(media)
		return
	}

	game.AddScreenshot(media)
}

// destinationPath returns where the file manager copies a capture of this game.
// The provider reads it to tell what is already copied, so the two have to
// agree: the folder comes from the file manager itself, and the name is
// normalized the same way.
func (p *NintendoSwitch2Provider) destinationPath(game *Game, name string) string {
	folder := p.fileManager.adoptExistingDir(expandUser(p.config.OutputPath), p.fileManager.GetPathForGame(game))

	return filepath.Join(folder, normalizeName(name))
}

// ignoredFolders returns the album folders to skip, keyed by their normalized
// name.
func (p *NintendoSwitch2Provider) ignoredFolders() map[string]struct{} {
	names := p.switch2Config.GetIgnoredFolders()

	ignored := make(map[string]struct{}, len(names))
	for _, name := range names {
		ignored[normalizeName(name)] = struct{}{}
	}

	return ignored
}

// switch2CaptureKind reports the kind of a capture the provider copies, and
// whether the name is one of them at all.
//
// Two things have to hold. The extension has to be one the console writes. The
// name has to end with "_s", because a "_c" file is a copy of a capture that
// already exists somewhere else.
func switch2CaptureKind(name string) (MediaKind, bool) {
	extension := strings.ToLower(filepath.Ext(name))

	kind, ok := switch2Extensions[extension]
	if !ok {
		return "", false
	}

	if !strings.HasSuffix(strings.TrimSuffix(name, filepath.Ext(name)), switch2CaptureSuffix) {
		return "", false
	}

	return kind, true
}

// describeMTPError adds the remedy to a failure of the device, so the message
// says what to do rather than only what happened.
func describeMTPError(err error) error {
	switch {
	case errors.Is(err, ErrMTPBusy):
		return fmt.Errorf("%w: close the file manager, and eject the console from it", err)
	case errors.Is(err, ErrMTPStaleSession):
		return fmt.Errorf("%w: unplug the cable, plug it back, and share the album again", err)
	case errors.Is(err, ErrMTPNoDevice):
		return fmt.Errorf("%w: share the album again on the console", err)
	}

	return err
}

// Requirements reports the external programs this provider needs.
//
// They all ship in libmtp. A path in the config reads a folder rather than the
// console, so none of them is needed then.
func (p *NintendoSwitch2Provider) Requirements() []Requirement {
	if path := p.switch2Config.GetPath(); path != "" && path != "auto" {
		return nil
	}

	return []Requirement{
		{
			Binary:  "mtp-folders",
			Package: "libmtp",
			Reason:  "lists the album folders on the console",
		},
		{
			Binary:  "mtp-files",
			Package: "libmtp",
			Reason:  "lists the captures on the console",
		},
		{
			Binary:  "mtp-connect",
			Package: "libmtp",
			Reason:  "copies the captures off the console",
		},
	}
}
