---
id: TQ-0010
title: Add a Nintendo Switch 2 provider that reads the album over MTP
status: done
priority: normal
labels:
  - feature
  - component/backend
  - component/config
created: 2026-09-06T12:14:18+02:00
updated: 2026-09-06T12:46:03+02:00
---

## Goal

Add a provider for the Nintendo Switch 2. The provider finds the console on the
USB bus. It reads the album over MTP. It copies every capture into the output
tree. It keeps each original file name.

The provider copies a file only when the name ends with `_s`. A `_c` file is a
copy the console made, or the same album seen on the other storage. Such a file
is skipped.

This replaces the manual folder copy the README describes today.

## What the console exposes

Every fact below was read from a real console on 2026-09-06.

### USB identity

The product ID says which mode the console is in.

| USB ID | Mode | Interfaces |
| --- | --- | --- |
| `057e:2061` | The album is shared with a computer | still image / MTP |
| `057e:2060` | Normal | HID only |

The vendor ID is `057e`. The product string is `Nintendo Switch 2`. The USB
serial number is the console serial number.

Detection on Linux needs no library. Read `idVendor` and `idProduct` under
`/sys/bus/usb/devices/*/`.

### The protocol is PTP, not MTP

libmtp prints this warning on every connection:

```
LIBMTP WARNING: no MTP vendor extension on device 15 on bus 10
LIBMTP WARNING: VendorExtensionID: ffffffff
LIBMTP WARNING: VendorExtensionDesc: nintendo.com: 1.0;
LIBMTP WARNING: this typically means the device is PTP (i.e. a camera) but not an MTP device at all. Trying to continue anyway.
```

The warning is harmless. Every operation works. Two effects follow from it:

1. The device reports no friendly name. `mtp-files` prints
   `Listing File Information on Device with name: (NULL)`. Identify the console
   by the USB ID, or by the libmtp device name `Nintendo Switch 2`.
2. The listing carries no modification date. The file name is the only capture
   time. This is enough, because the name holds the timestamp.

### One storage, and it is the memory card

The console exposes exactly one storage. Its name is `Album`. Its storage ID is
`0x00040001`.

A microSD Express card was in the console during the whole test. Every one of
the 1760 files carried that one storage ID. The owner confirms the files come
from the card only.

So the console shares the album of its current save destination. It does not
expose the internal album beside the card album. Write the code for one storage,
but read the storage ID from the listing rather than as a constant.

### Album layout

The tree is exactly two levels deep:

```
Storage: Album
5905 MONSTER HUNTER STORIES 3: TWISTED REFLECTION Trial Version
  5906 2026032619431800_s.jpg
  5907 2026032619513300_s.jpg
5909 The Adventures of Elliot: The Millennium Tales Debut Demo
  5910 2025073123214500_s.jpg
```

A folder name is usually a game name. It is not always a game name. See the
system folder below.

The test console held 29 folders, 1760 files and 4.26 GB.

### File names

Every one of the 1760 files matched `^[0-9]{16}_s\.(jpg|mp4)$`.

- 1641 files were `_s.jpg`.
- 119 files were `_s.mp4`.

The 16 digits are `20060102150405` plus two more digits below the second.
`parseTailDatetime` in `playstation.go` already reads a name of that shape, so
the code can be reused to date a capture.

A JPEG carries EXIF. `exiftool` reports `Nintendo co., ltd` as the maker, and a
`datetime` that matches the name. The size is 1920x1080.

No `_c` file was on the test console. The origin of a `_c` file is not
confirmed. The owner reports two possible causes: a copy the console made, or
the same album seen on both the internal memory and the card. Keep the filter as
a safety net.

## How to reach the files

### Recommendation

Shell out to the libmtp command line tools. Declare them through the existing
`Requirement` and `Requirer` interfaces in `provider.go`, so the `doctor`
subcommand reports a missing tool.

| Step | Command | What it gives |
| --- | --- | --- |
| Find the console | read `/sys/bus/usb/devices/*/` | the USB ID `057e:2061` |
| List the folders | `mtp-folders` | folder ID and folder name |
| List the files | `mtp-files` | file ID, file name, size, parent ID, storage ID |
| Pull the files | `mtp-connect --getfile <id> <path> ...` | one file per pair of arguments |

The requirement is `Binary: "mtp-connect"`, `Package: "libmtp"`.

### Why this route

- It keeps `CGO_ENABLED=0`. `.goreleaser.yml` sets that flag for all nine
  release targets. A cgo binding would break every build.
- It adds no Go dependency.
- The `doctor` subcommand already reports a missing external program.

### Measurements

`mtp-connect` takes many `--getfile` pairs and runs them in one session. This
matters, because a connection costs about 4 seconds on its own.

| Test | Files | Bytes | Time |
| --- | --- | --- | --- |
| `mtp-getfile`, one file | 1 | 502 KB | 4.5 s |
| `mtp-connect`, one batch | 5 | 3.5 MB | 4.6 s |
| `mtp-connect`, one batch | 100 | 710 MB | 21 s |

The throughput was 32.3 MB/s. A full copy of 4.26 GB takes about 2.2 minutes.

Batch the pulls. Do not call `mtp-getfile` once per file. 1760 separate calls
would cost about two hours in connection overhead alone.

Chunk each batch to stay under the argument limit. About 500 files per batch is
safe.

### Alternatives that were rejected

- **cgo bindings.** `github.com/hanwen/go-mtpfs/mtp` holds a pure Go MTP layer,
  but its transport is `github.com/hanwen/usb`, which is a cgo binding to
  libusb. This breaks `CGO_ENABLED=0`.
- **Pure Go USB.** `github.com/pzl/usb` is a USB stack in Go with no cgo. It
  supports Linux only. The PTP layer would have to be written from nothing. The
  effort and the risk are both high.
- **A desktop mount.** GNOME exposes an MTP device at
  `/run/user/<uid>/gvfs/mtp:host=.../`, and KDE exposes one through `kio-fuse`.
  Neither package was installed on the test machine. Both need the user to
  mount the console by hand, which loses the automatic detection.

## Risks

### The desktop takes the device first

Only one process can hold an MTP device. KDE claims the console as soon as it
appears. `lsof` named the holder:

```
COMMAND    PID      USER FD   TYPE   DEVICE SIZE/OFF NODE NAME
kiod6   317649 fmartingr 24u   CHR 189,1164      0t0 1792 /dev/bus/usb/010/013
```

libmtp then fails with:

```
libusb_claim_interface() reports device is busy, likely in use by GVFS or KDE MTP device handling already
```

The provider must report this clearly. Tell the user to close the file manager,
and to eject the console from it.

### A dead client leaves the session open

The console keeps its PTP session after a client dies. libmtp cannot reopen it.
The debug trace shows the cause: the console answers `OpenSession` with
`0x201e` (`SessionAlreadyOpen`), and then answers `GetDeviceInfo` with `0x2004`
(`InvalidTransactionID`). The error the user sees is:

```
LIBMTP PANIC: Unable to read device information on device 13 on bus 10, trying to continue
```

`mtp-reset` does not clear it. A USB port reset does clear it, but the console
then leaves transfer mode and re-enumerates as `057e:2060`. Only a replug and a
new transfer-mode selection recover the console.

The provider must therefore close its session cleanly on every exit path,
signals included. It must also recognise this failure and tell the user to
replug the cable.

### `mtp-connect` hides a failure

`mtp-connect` exits 0 for a file ID that does not exist, and it writes no file.
Do not trust the exit code. Check that each output file exists. Check that its
size matches the size the listing reported.

## Design

### Config

Add `NintendoSwitch2Config` to `Providers` in `config.go`, under the TOML key
`nintendo_switch_2`. It embeds `ProviderConfig` and adds `ignored_folders`. Add
it to the `Merge` list in `NewConfig`. Register it in `registry.go`. Document it
in `config.example.toml`.

`path` keeps its meaning as an override. A path that is set reads a folder
instead of the console, which preserves the old copy-from-a-folder workflow.

### The system folder

The console makes one album folder of its own, for the captures taken outside a
game. These are the Home menu, the eShop and other system screens. The test
console named it `Otra carpeta`, which is Spanish. The name follows the console
language, so it is not a name the code can hold as a constant.

Skip this folder. Use the `ignored_folders` config key, which takes a list of
folder names. Ship the known names as the default:

```toml
ignored_folders = ["Otra carpeta", "Other"]
```

Document that the name follows the console language. Tell the user to add their
own name where the default does not match. A folder name that is not on the
list is copied.

The key also serves the second purpose the Steam provider's `ignored_games`
serves: it lets the user drop a game they do not want.

### The media source is not a local file

`Media` reads its source with `os.Open` and `os.Stat`, and `ensureLocalSource`
only handles an HTTP URL. An MTP file is neither.

Stage the files first. Pull each wanted file into one temporary directory. Then
give the local path to `Media`. Register the removal of that directory with
`FileManager.cleanup`.

Pull only what is missing. Build the destination path with
`FileManager.GetPathForGame` before the pull. Skip a file that is already
there. The first run then costs 4.26 GB, and a later run costs almost nothing.

### Names

Keep the console file name, as in `2025060720031600_s.jpg`. Set it as
`DestinationName` on the `Media`. The gallery does not parse a media file name,
so nothing depends on the `2006-01-02_15-04-05` form other providers use.

### Kinds

A `.jpg` file is `MediaKindScreenshot`. An `.mp4` file is `MediaKindClip`.

### Platform

The platform name is `Nintendo Switch 2`. The game name is the album folder
name. There is no game ID, so use the folder name for both.

### Covers

The provider sets no cover. The project holds no cover source for Switch 2
games. A later task can add one.

### A console that is not there

The console is away most of the time. A run with no console found logs one line
at info level and returns nil. It does not warn. It does not fail the run.

The same rule covers a console that is plugged in but not in transfer mode. Such
a console shows as `057e:2060`, and the provider treats it as absent.

## Test plan

- Unit test the file name filter. Accept `_s`. Reject `_c`. Reject a name that
  does not match the pattern.
- Unit test the `ignored_folders` match. Check the default list. Check a name
  the user added.
- Unit test the parser for `mtp-folders` output and `mtp-files` output. Use the
  captured samples as fixtures.
- Unit test the USB detection against a fake sysfs tree. Cover `057e:2061`,
  `057e:2060` and no device at all.
- Test the `doctor` report for a missing `mtp-connect`.

---

## Notes

- 2026-09-06T12:14:23+02:00 — Research ran against a real console on 2026-09-06. Raw captures are in the session scratchpad: mtp-folders.txt (29 folders), mtp-files.txt (1760 files), files.tsv (id/name/size). They are not committed. Re-capture them with 'mtp-folders' and 'mtp-files' when the fixtures are written.
- 2026-09-06T12:20:05+02:00 — Open questions answered by the owner on 2026-09-06:

  - Storage: a microSD Express card was in the console for the whole test. The console still exposed one storage only, and every file came from it. The internal album is not exposed beside the card album.
  - The system folder ('Otra carpeta') is made by Nintendo, not by the owner. It holds captures from the system UI. Skip it. The name follows the console language, so it cannot be a constant in the code.
  - Covers: none for now.
  - A console that is not connected: log one line at info level, and return nil.
- 2026-09-06T12:46:03+02:00 — Implemented.

  New files: mtp.go (USB detection through sysfs, the libmtp wrapper, the listing parsers, the batched pull), nintendoswitch2.go (the provider), mtp_test.go, nintendoswitch2_test.go, testdata/mtp-folders.txt and testdata/mtp-files.txt (real console output).
  Changed: config.go (NintendoSwitch2Config with ignored_folders), registry.go, doctor.go, doctor_test.go, files.go (FileManager.AddCleanup), config.example.toml, README.md.

  Two decisions differ from the plan:
  - The capture filter is the '_s' suffix plus a known extension, not the 16-digit pattern. It follows the rule the owner gave, and it survives a change of the digit count.
  - ignored_folders defaults to ['Otra carpeta'] only. The English name of the system folder is not known, and a guess could skip a real game folder.

  Verified against the console on 2026-09-06:
  - A console another program holds reports 'another program holds the device' with the remedy.
  - A console with a session left by a program that died reports 'the device kept a session' with the remedy.
  - A run copied 6 captures of two game folders, and a second run copied 0.
  - A 40 MB MP4 clip arrived whole and valid.
  - The staging folder is removed by the cleanup.
  - go vet, gofmt and go test -race all pass.
