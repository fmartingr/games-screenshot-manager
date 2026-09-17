---
id: TQ-0011
title: Add a Diablo IV provider
status: done
priority: normal
labels:
  - feature
  - component/backend
created: 2026-09-17T08:41:59+02:00
updated: 2026-09-17T09:00:48+02:00
---

## Goal

Collect Diablo IV screenshots, in the same way as the World of Warcraft
provider: a default path per platform, and a custom path in the config.

## The name problem

The game names every screenshot `Screenshot001.jpg`, and it counts up from
there. The counter restarts on each computer, so two computers produce two
different screenshots under one name. The name alone cannot identify a file.

The provider therefore renames each file to the modification time of the file,
in `DatetimeFormat`. Two files taken in one second still collide, and the file
manager already answers that: it compares the hashes and puts the hash in the
name of the second file.

## Scope

- `diabloiv.go` — the provider.
- `config.go` — a `ProviderConfig` under `providers.diablo_4`, and its merge.
- `registry.go` — register it when it is enabled.
- `doctor.go` — one line in the provider report.
- `config.example.toml` and `README.md` — document it.
- `diabloiv_test.go` — the tests.

---

## Notes

- 2026-09-17T08:43:56+02:00 — Implemented. The provider is diabloiv.go, and it follows the World of Warcraft one: a default path per platform, and a custom path in the config.

  Naming: the game writes Screenshot001.jpg and counts up, so the name carries no
  date and it repeats across computers. Each file is renamed to the modification
  time of the file, in DatetimeFormat, with the extension lowercased. EXIF was not
  used, so the provider needs no external program.

  Paths: 'auto' reads two folders on Windows, ~/Pictures/Diablo IV and
  ~/Documents/Diablo IV/Screenshots. Which of the two the game writes to is not
  confirmed here, so both are read and a folder that is missing is skipped. Every
  other platform needs a path, as the World of Warcraft and Guild Wars 2 providers
  do.

  Config key is providers.diablo_4, registry name is diablo_4, game ID is diablo4.

  Verified against a fixture of two folders that stand for two computers:
  - Run 1 copied 2 files under their dates, and left readme.txt alone.
  - Run 2, against the second folder, copied a file of the same name and a later
    date under its own name, and put the hash in the name of the file that shared
    a second with an existing one.
  - Run 3 copied nothing.
  - The doctor reports the provider and its path.

  go vet, gofmt and go test are clean. Six tests added in diabloiv_test.go.
- 2026-09-17T09:00:48+02:00 — Code review findings acted on.

  1. The doctor said 'enabled, and it finds its own path' on a host where the
     game writes no default folder. doctorProvider gains autoPlatforms, and
     diablo_4 sets it to windows. The report now warns and names the remedy. The
     field is empty for every other provider, so nothing else changed.

  2. A run of a config that never named this game logged an ERROR on Linux and on
     macOS, because a provider inherits global.enabled. getScreenshotsPaths now
     returns nothing rather than an error on a platform without a default folder,
     and GetScreenshots warns once and collects nothing.

  3. TestDiabloIVSkipsAMissingPath asserted a nil error only, which GetScreenshots
     always returns. It now asserts that no game was added. Two tests added:
     TestDiabloIVReadsOnPastAnUnreadableFolder, which is what covers the error
     path of processScreenshotsPath, and
     TestDiabloIVAutoOnAPlatformWithoutADefaultFolder.

  4. The README states the timestamp caveat: the date is the time the file was
     written, so a copy has to keep the timestamps.

  The review also asked for EXIF DateTimeOriginal rather than the modification
  time. It was not taken. The game renders the JPEG itself and writes no capture
  date into it, so exiftool would return nothing and would add a dependency.

  Not taken: the review reads the modification time as lost on a copy. It is kept
  by rsync -t, cp -p and robocopy. A creation time is worse, because a copy always
  resets it.

  Verified again: the doctor warns on Linux and exits 0, a run warns once and
  collects nothing, and the fixture of two computers still produces 4 files.
  go vet, gofmt and go test are clean. Eight tests now.
