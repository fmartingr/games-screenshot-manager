package gamesscreenshotmanager

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// usbSysfsRoot is where Linux lists the devices on the USB bus. It is a
// variable so a test can point it at a tree it built.
var usbSysfsRoot = "/sys/bus/usb/devices"

// usbDevice is one device on the USB bus.
type usbDevice struct {
	VendorID  string
	ProductID string
	Product   string
	Serial    string
}

// findUSBDevice returns the first device that reports vendorID and productID.
// The second value reports whether one was found.
//
// Linux is the only host this works on. It reads the sysfs tree, which no other
// platform has, so elsewhere it finds nothing and the caller falls back to the
// path in the config.
func findUSBDevice(vendorID, productID string) (usbDevice, bool) {
	entries, err := os.ReadDir(usbSysfsRoot)
	if err != nil {
		return usbDevice{}, false
	}

	for _, entry := range entries {
		path := filepath.Join(usbSysfsRoot, entry.Name())

		// An interface is listed beside its device, as "10-1:1.0" beside
		// "10-1". Only a device carries idVendor, so a missing file is the
		// test for one.
		vendor, err := readSysfsAttribute(path, "idVendor")
		if err != nil {
			continue
		}

		product, err := readSysfsAttribute(path, "idProduct")
		if err != nil {
			continue
		}

		if !strings.EqualFold(vendor, vendorID) || !strings.EqualFold(product, productID) {
			continue
		}

		name, _ := readSysfsAttribute(path, "product")
		serial, _ := readSysfsAttribute(path, "serial")

		return usbDevice{VendorID: vendor, ProductID: product, Product: name, Serial: serial}, true
	}

	return usbDevice{}, false
}

// readSysfsAttribute reads one attribute file of a sysfs entry.
func readSysfsAttribute(path, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(path, name))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// The libmtp tools report a failure in their output and still exit 0, so the
// output is what the client reads to tell one failure from another.
const (
	mtpBusyMarker      = "libusb_claim_interface() reports device is busy"
	mtpStaleMarker     = "Unable to read device information"
	mtpNoRawMarker     = "No raw devices found"
	mtpNoDevicesMarker = "No devices."
)

var (
	// ErrMTPBusy reports a device another program holds. Only one program at a
	// time can hold an MTP device, and a desktop file manager takes it as soon
	// as it appears.
	ErrMTPBusy = errors.New("another program holds the device, so it cannot be read")

	// ErrMTPStaleSession reports a device that kept the session of a program
	// that died. The device answers the next OpenSession with
	// SessionAlreadyOpen, and then answers GetDeviceInfo with
	// InvalidTransactionID. Only a replug clears it.
	ErrMTPStaleSession = errors.New("the device kept a session from a program that died")

	// ErrMTPNoDevice reports that no MTP device answered.
	ErrMTPNoDevice = errors.New("no MTP device answered")
)

// mtpFolder is one folder of an MTP storage.
type mtpFolder struct {
	ID   string
	Name string
}

// mtpFile is one file of an MTP storage.
type mtpFile struct {
	ID       string
	Name     string
	Size     int64
	ParentID string
}

// mtpPull asks for one file, to be written at Path.
type mtpPull struct {
	File mtpFile
	Path string
}

// mtpPullBatchSize is how many files one mtp-connect call asks for. A
// connection costs about four seconds, and the transfer itself runs at about
// 32 MB/s, so a batch is what keeps a whole album under a few minutes. The
// size also keeps the argument list well under the limit of the host.
const mtpPullBatchSize = 500

// mtpClient reads an MTP device through the libmtp command line tools.
//
// The tools take no device argument. Each call finds the one device on the bus
// and opens its own session, so a client is only correct while exactly one MTP
// device is connected.
type mtpClient struct {
	log *slog.Logger
	// run executes one libmtp tool and returns everything it printed, on both
	// streams. It is a field so a test can answer without a device.
	run func(name string, args ...string) ([]byte, error)
}

// newMTPClient returns a client that runs the libmtp tools.
func newMTPClient(log *slog.Logger) *mtpClient {
	return &mtpClient{log: log, run: runMTPTool}
}

// runMTPTool runs one libmtp tool and returns everything it printed.
//
// Both streams are read together on purpose. A tool writes its warnings and
// some of its errors to standard error, and it reports a failure in its output
// rather than in its exit code, so the two have to be read as one text.
func runMTPTool(name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)

	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output

	if err := command.Run(); err != nil {
		return output.Bytes(), fmt.Errorf("%s failed: %w", name, err)
	}

	return output.Bytes(), nil
}

// checkMTPOutput turns a failure the tool reported in its text into an error.
// The tools exit 0 for every failure below, so the text is the only signal.
func checkMTPOutput(output []byte) error {
	text := string(output)

	switch {
	case strings.Contains(text, mtpBusyMarker):
		return ErrMTPBusy
	case strings.Contains(text, mtpStaleMarker):
		return ErrMTPStaleSession
	case strings.Contains(text, mtpNoRawMarker), strings.Contains(text, mtpNoDevicesMarker):
		return ErrMTPNoDevice
	}

	return nil
}

// Folders returns every folder of the connected device.
func (c *mtpClient) Folders() ([]mtpFolder, error) {
	output, err := c.run("mtp-folders")
	if checkErr := checkMTPOutput(output); checkErr != nil {
		return nil, checkErr
	}

	if err != nil {
		return nil, err
	}

	return parseMTPFolders(output), nil
}

// Files returns every file of the connected device.
func (c *mtpClient) Files() ([]mtpFile, error) {
	output, err := c.run("mtp-files")
	if checkErr := checkMTPOutput(output); checkErr != nil {
		return nil, checkErr
	}

	if err != nil {
		return nil, err
	}

	return parseMTPFiles(output), nil
}

// Pull copies each file off the device to its path. It creates the folder each
// path needs.
//
// The pulls are sent in batches, because one call opens one session and a
// session costs about four seconds. One call per file would cost hours for a
// full album.
func (c *mtpClient) Pull(pulls []mtpPull) error {
	for start := 0; start < len(pulls); start += mtpPullBatchSize {
		end := min(start+mtpPullBatchSize, len(pulls))

		batch := pulls[start:end]

		args := make([]string, 0, len(batch)*3)
		for _, pull := range batch {
			if err := os.MkdirAll(filepath.Dir(pull.Path), 0o755); err != nil {
				return fmt.Errorf("error creating the staging folder: %w", err)
			}

			args = append(args, "--getfile", pull.File.ID, pull.Path)
		}

		c.log.Debug("pulling a batch of files", slog.Int("files", len(batch)), slog.Int("done", start))

		output, err := c.run("mtp-connect", args...)
		if checkErr := checkMTPOutput(output); checkErr != nil {
			return checkErr
		}

		if err != nil {
			return err
		}

		if err := verifyPulls(batch); err != nil {
			return err
		}
	}

	return nil
}

// verifyPulls checks that every file of a batch arrived whole.
//
// mtp-connect exits 0 for a file it did not copy, and it writes nothing, so
// the exit code says nothing about the result. The size the listing reported is
// what the file on disk has to match.
func verifyPulls(pulls []mtpPull) error {
	for _, pull := range pulls {
		info, err := os.Stat(pull.Path)
		if err != nil {
			return fmt.Errorf("file %s (id %s) was not copied off the device: %w", pull.File.Name, pull.File.ID, err)
		}

		if info.Size() != pull.File.Size {
			return fmt.Errorf(
				"file %s (id %s) arrived with %d bytes, and the device reported %d",
				pull.File.Name, pull.File.ID, info.Size(), pull.File.Size,
			)
		}
	}

	return nil
}

// parseMTPFolders reads the output of mtp-folders.
//
// The tool prints a header, one "Storage:" line per storage, and then one line
// per folder that holds the folder ID, a tab and the folder name. A line it
// does not recognise is skipped, which is what keeps the libmtp warnings out of
// the result.
func parseMTPFolders(output []byte) []mtpFolder {
	var folders []mtpFolder

	for _, line := range strings.Split(string(output), "\n") {
		id, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || !isDigits(id) {
			continue
		}

		folders = append(folders, mtpFolder{ID: id, Name: strings.TrimSpace(name)})
	}

	return folders
}

// parseMTPFiles reads the output of mtp-files.
//
// The tool prints one block per file. The block opens with a "File ID:" line,
// and the lines below it are indented:
//
//	File ID: 5906
//	   Filename: 2026032619431800_s.jpg
//	   File size 502016 (0x000000000007A900) bytes
//	   Parent ID: 5905
//	   Storage ID: 0x00040001
//	   Filetype: JPEG file
//
// A block with no file ID and a line the parser does not recognise are both
// skipped.
func parseMTPFiles(output []byte) []mtpFile {
	var (
		files   []mtpFile
		current *mtpFile
	)

	closeBlock := func() {
		if current != nil && current.Name != "" {
			files = append(files, *current)
		}

		current = nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "File ID: "):
			closeBlock()

			id := strings.TrimSpace(strings.TrimPrefix(trimmed, "File ID: "))
			if isDigits(id) {
				current = &mtpFile{ID: id}
			}
		case current == nil:
			continue
		case strings.HasPrefix(trimmed, "Filename: "):
			current.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "Filename: "))
		case strings.HasPrefix(trimmed, "File size "):
			// The line reads "File size 502016 (0x...) bytes", so the decimal
			// count is the field between the prefix and the hexadecimal one.
			fields := strings.Fields(strings.TrimPrefix(trimmed, "File size "))
			if len(fields) == 0 {
				continue
			}

			if size, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
				current.Size = size
			}
		case strings.HasPrefix(trimmed, "Parent ID: "):
			current.ParentID = strings.TrimSpace(strings.TrimPrefix(trimmed, "Parent ID: "))
		}
	}

	closeBlock()

	return files
}
