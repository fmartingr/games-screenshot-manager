package gamesscreenshotmanager

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// useSysfsRoot points the USB search at a tree the test built, and puts the
// real one back afterwards.
func useSysfsRoot(t *testing.T, root string) {
	t.Helper()

	previous := usbSysfsRoot
	usbSysfsRoot = root

	t.Cleanup(func() { usbSysfsRoot = previous })
}

// discardLogger keeps the test output clean.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// The listing is the output of a real console, warnings and all, so the parser
// has to read past everything that is not a folder line.
func TestParseMTPFolders(t *testing.T) {
	output, err := os.ReadFile(filepath.Join("testdata", "mtp-folders.txt"))
	if err != nil {
		t.Fatalf("could not read the fixture: %v", err)
	}

	folders := parseMTPFolders(output)

	if len(folders) != 7 {
		t.Fatalf("parseMTPFolders() returned %d folders, want 7", len(folders))
	}

	if folders[0].ID != "5905" {
		t.Errorf("the first folder has ID %q, want %q", folders[0].ID, "5905")
	}

	want := "MONSTER HUNTER STORIES 3: TWISTED REFLECTION Trial Version"
	if folders[0].Name != want {
		t.Errorf("the first folder is named %q, want %q", folders[0].Name, want)
	}

	// A name with an apostrophe in it survives the parse whole.
	if folders[3].Name != "Yakuza 0 Director's Cut" {
		t.Errorf("the fourth folder is named %q, want %q", folders[3].Name, "Yakuza 0 Director's Cut")
	}
}

func TestParseMTPFiles(t *testing.T) {
	output, err := os.ReadFile(filepath.Join("testdata", "mtp-files.txt"))
	if err != nil {
		t.Fatalf("could not read the fixture: %v", err)
	}

	files := parseMTPFiles(output)

	if len(files) != 7 {
		t.Fatalf("parseMTPFiles() returned %d files, want 7", len(files))
	}

	first := files[0]

	if first.ID != "5906" {
		t.Errorf("the first file has ID %q, want %q", first.ID, "5906")
	}

	if first.Name != "2026032619431800_s.jpg" {
		t.Errorf("the first file is named %q, want %q", first.Name, "2026032619431800_s.jpg")
	}

	if first.Size != 502016 {
		t.Errorf("the first file has size %d, want 502016", first.Size)
	}

	if first.ParentID != "5905" {
		t.Errorf("the first file has parent %q, want %q", first.ParentID, "5905")
	}

	// The fixture ends part way through the last block. The block still holds a
	// name, so it counts, and this is what a listing that was cut short looks
	// like.
	last := files[len(files)-1]
	if last.ID != "5913" || last.ParentID != "5909" {
		t.Errorf("the last file is %+v, want ID 5913 under parent 5909", last)
	}
}

// The tools report a failure in their text and still exit 0, so the text is
// what has to be read.
func TestCheckMTPOutput(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   error
	}{
		{
			name:   "another program holds the device",
			output: "libusb_claim_interface() reports device is busy, likely in use by GVFS or KDE MTP device handling already",
			want:   ErrMTPBusy,
		},
		{
			name:   "the device kept an old session",
			output: "LIBMTP PANIC: Unable to read device information on device 13 on bus 10, trying to continue",
			want:   ErrMTPStaleSession,
		},
		{
			name:   "no raw device",
			output: "Listing raw device(s)\n   No raw devices found.",
			want:   ErrMTPNoDevice,
		},
		{
			name:   "no device",
			output: "Device 0 (VID=057e and PID=2061) is a Nintendo Switch 2.\nNo devices.",
			want:   ErrMTPNoDevice,
		},
		{
			name:   "a listing that worked",
			output: "Storage: Album\n5905\tMario Kart World\nOK.",
			want:   nil,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkMTPOutput([]byte(testCase.output))

			if !errors.Is(err, testCase.want) {
				t.Fatalf("checkMTPOutput() returned %v, want %v", err, testCase.want)
			}
		})
	}
}

// A connection costs about four seconds, so the pulls go out in batches rather
// than one call per file.
func TestMTPClientPullBatches(t *testing.T) {
	stage := t.TempDir()

	var calls [][]string

	client := &mtpClient{
		log: discardLogger(),
		run: func(name string, args ...string) ([]byte, error) {
			if name != "mtp-connect" {
				t.Fatalf("Pull() ran %q, want mtp-connect", name)
			}

			calls = append(calls, args)

			// Write what the tool would have written, so the check on the
			// result passes.
			for i := 0; i+2 < len(args); i += 3 {
				if err := os.WriteFile(args[i+2], []byte("ab"), 0o644); err != nil {
					t.Fatalf("could not write the pulled file: %v", err)
				}
			}

			return []byte("OK."), nil
		},
	}

	pulls := make([]mtpPull, 0, mtpPullBatchSize+1)
	for i := range mtpPullBatchSize + 1 {
		file := mtpFile{ID: strconv.Itoa(i), Name: "capture.jpg", Size: 2}
		pulls = append(pulls, mtpPull{File: file, Path: filepath.Join(stage, strconv.Itoa(i), file.Name)})
	}

	if err := client.Pull(pulls); err != nil {
		t.Fatalf("Pull() returned an error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("Pull() made %d calls, want 2", len(calls))
	}

	if len(calls[0]) != mtpPullBatchSize*3 {
		t.Errorf("the first call took %d arguments, want %d", len(calls[0]), mtpPullBatchSize*3)
	}

	if len(calls[1]) != 3 {
		t.Errorf("the second call took %d arguments, want 3", len(calls[1]))
	}

	if calls[0][0] != "--getfile" {
		t.Errorf("the first argument is %q, want --getfile", calls[0][0])
	}
}

// mtp-connect exits 0 for a file it did not copy, so the size the device
// reported is what says the file arrived whole.
func TestMTPClientPullReportsAShortFile(t *testing.T) {
	stage := t.TempDir()

	client := &mtpClient{
		log: discardLogger(),
		run: func(name string, args ...string) ([]byte, error) {
			if err := os.WriteFile(args[2], []byte("a"), 0o644); err != nil {
				t.Fatalf("could not write the pulled file: %v", err)
			}

			return []byte("OK."), nil
		},
	}

	pulls := []mtpPull{{
		File: mtpFile{ID: "1", Name: "capture.jpg", Size: 500},
		Path: filepath.Join(stage, "capture.jpg"),
	}}

	err := client.Pull(pulls)
	if err == nil {
		t.Fatal("Pull() returned no error for a file that arrived short")
	}
}

// A tool that reports a busy device exits 0, so the error has to come out of
// the text rather than out of the exit code.
func TestMTPClientFoldersReportsABusyDevice(t *testing.T) {
	client := &mtpClient{
		log: discardLogger(),
		run: func(name string, args ...string) ([]byte, error) {
			return []byte("libusb_claim_interface() reports device is busy, likely in use by GVFS"), nil
		},
	}

	_, err := client.Folders()

	if !errors.Is(err, ErrMTPBusy) {
		t.Fatalf("Folders() returned %v, want ErrMTPBusy", err)
	}
}

// writeUSBDevice builds one sysfs entry, the way Linux lists a device.
func writeUSBDevice(t *testing.T, root, name string, attributes map[string]string) {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("could not create the sysfs entry: %v", err)
	}

	for attribute, value := range attributes {
		if err := os.WriteFile(filepath.Join(path, attribute), []byte(value+"\n"), 0o644); err != nil {
			t.Fatalf("could not write the sysfs attribute: %v", err)
		}
	}
}

func TestFindUSBDevice(t *testing.T) {
	root := t.TempDir()

	// An interface is listed beside its device, and it carries no idVendor.
	writeUSBDevice(t, root, "10-1:1.0", map[string]string{"bInterfaceClass": "03"})
	writeUSBDevice(t, root, "10-4", map[string]string{"idVendor": "05e3", "idProduct": "0610"})
	writeUSBDevice(t, root, "10-1", map[string]string{
		"idVendor":  "057e",
		"idProduct": "2061",
		"product":   "Nintendo Switch 2",
		"serial":    "HAE10027265351",
	})

	useSysfsRoot(t, root)

	device, found := findUSBDevice(nintendoVendorID, switch2AlbumProductID)
	if !found {
		t.Fatal("findUSBDevice() found no console, want one")
	}

	if device.Product != "Nintendo Switch 2" {
		t.Errorf("the device is named %q, want %q", device.Product, "Nintendo Switch 2")
	}

	if device.Serial != "HAE10027265351" {
		t.Errorf("the device has serial %q, want %q", device.Serial, "HAE10027265351")
	}
}

// A console in its normal mode reports 2060 and offers a HID interface only, so
// it is not a console that shares its album.
func TestFindUSBDeviceSkipsAConsoleThatSharesNothing(t *testing.T) {
	root := t.TempDir()

	writeUSBDevice(t, root, "10-1", map[string]string{
		"idVendor":  "057e",
		"idProduct": "2060",
		"product":   "Nintendo Switch 2",
	})

	useSysfsRoot(t, root)

	if _, found := findUSBDevice(nintendoVendorID, switch2AlbumProductID); found {
		t.Fatal("findUSBDevice() found a console, want none")
	}
}

// A host with no sysfs tree finds nothing, which is what every platform other
// than Linux does.
func TestFindUSBDeviceWithoutSysfs(t *testing.T) {
	useSysfsRoot(t, filepath.Join(t.TempDir(), "missing"))

	if _, found := findUSBDevice(nintendoVendorID, switch2AlbumProductID); found {
		t.Fatal("findUSBDevice() found a console, want none")
	}
}
