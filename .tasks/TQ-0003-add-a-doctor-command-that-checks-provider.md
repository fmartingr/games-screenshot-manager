---
id: TQ-0003
title: Add a doctor command that checks provider dependencies
status: done
priority: normal
labels:
  - feature
  - component/cli
created: 2026-09-06T09:03:35+02:00
updated: 2026-09-06T09:46:28+02:00
---

## Problem

The tool needs external programs, and it needs them only for some providers.
Nothing tells the user about a missing program before a run. The run starts,
it walks the disk, and it fails part way. The error names one file, not the
missing program.

The current dependencies:

| Program | Needed by | Call site |
| --- | --- | --- |
| `exiftool` | `guild_wars_2`, `playstation4`, `xbox_game_bar` | `exiftool.NewExiftool()` in each provider |
| `ffprobe` | `playstation5` | `video.go:53`, through `AdjustDatetimeByVideoDuration` |
| `ffmpeg` | the gallery, for a video thumbnail | `gallery.go:324` |
| `ffprobe` | the gallery, for a video duration | `gallery_node.go:153` |

Two more failures are silent today:

- `xbox_game_bar` is enabled on Linux or macOS. `registry.go` drops it. The
  user gets no message.
- `steam.online_gallery` is true and `steam.api_key` is empty.
  `steam_client.go:386` needs the key.

## Design

Add a `doctor` subcommand:

```
games-screenshot-manager doctor -config ./config.toml
```

The command changes no file. It exits 1 when a check fails.

**Each provider declares its own requirements.** The doctor asks the enabled
providers, joins the answers, removes the duplicates, and looks for each
program on the system. A provider that gains a dependency states it in one
place, next to the code that runs the program.

A check has three states:

1. `ok` — the program is on the system.
2. `warn` — the run works, but a feature is off or degraded.
3. `fail` — an enabled provider cannot work.

The doctor checks only what the config turns on. A missing `exiftool` is
`ok` when no enabled provider asks for it.

## Prerequisite: a constructor must not use the network

`NewSteamProvider` calls `NewSteamClient`, and `NewSteamClient` calls
`DownloadSteamAppList` at `steam_client.go:150`. A doctor that builds the
providers would download the whole Steam app list. That is wrong for a
check command, and it fails on a host with no network.

Make the app list lazy:

- `SteamClient` gets `appsOnce sync.Once` and `appsErr error`.
- A new method `ensureAppList() error` runs `DownloadSteamAppList` once.
- `GetSteamApps` (`steam_client.go:360`), `GetGameName` (`365`) and
  `GetGameID` (`370`) call `ensureAppList` first. Those are the only three
  readers of `c.steamApps`.
- `NewSteamClient` drops the download call and the `Steam app list ready`
  log line. Move that log line into `ensureAppList`.

A constructor that reads only the config is then safe for the doctor to
call. This is a fix on its own: a run with `-only-build-gallery` and a run
that fails early both stop paying for the download.

## Implementation

**provider.go — the requirement and the optional interface**

```go
// Requirement is one external program a provider needs.
type Requirement struct {
	Binary  string // the program to look for: "exiftool"
	Package string // what the user installs to get it: "ffmpeg" ships "ffprobe"
	Reason  string // why this provider needs it, one short phrase
}

// Requirer is an optional interface. A provider that needs an external
// program implements it. A provider that needs none does not.
type Requirer interface {
	Requirements() []Requirement
}
```

The method sits on the provider, so it reads that provider's own config. A
requirement can be conditional. A provider that handles video only when a
config key is set returns `ffprobe` only in that case.

Implement it on:

- `*GuildWars2Provider`, `*Playstation4Provider`, `*XboxGameBarProvider` —
  `exiftool`, package `exiftool`, reason "reads the capture date from a
  screenshot".
- `*Playstation5Provider` — `ffprobe`, package `ffmpeg`, reason "reads the
  length of a clip, to date it from its start".
- `*GalleryBuilder` — `ffmpeg` when `gallery.create` is true, reason
  "makes a thumbnail for a video". Add `ffprobe` when
  `gallery.show_video_duration` is also true, reason "shows the length of a
  video".

`SteamProvider`, `MinecraftProvider` and `HytaleProvider` need no external
program. They implement nothing.

**registry.go — the enabled set**

`registerProviders` keeps its shape. Add one method:

```go
// Requirements returns every requirement the registered providers declare.
func (r *ProviderRegistry) Requirements() map[string][]RequirementUse
```

It walks `r.Providers`, tests each one for `Requirer`, and groups the
results by `Requirement.Binary`. `RequirementUse` holds the provider name
and the `Requirement`, so the report can name every provider that wants a
program.

The map is keyed by the binary, so the deduplication is the grouping. Two
providers that both want `exiftool` produce one key and two uses.

**doctor.go — new file**

```go
type CheckStatus string

const (
	CheckOK   CheckStatus = "ok"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

type CheckResult struct {
	Scope  string // "config", "provider", "dependency", "gallery"
	Name   string // "exiftool", "api_key"
	Status CheckStatus
	Detail string // the resolved path, or why it failed
	Remedy string // what the user does about it
}

// A var, so a test can replace it.
var lookPath = exec.LookPath

func RunDoctor(config *Config) ([]CheckResult, error)
func PrintChecks(w io.Writer, results []CheckResult)
func HasFailure(results []CheckResult) bool
```

`RunDoctor` builds the list in this order:

1. The config file. It parsed, so this is `ok` with the path it read.
2. The output path. `fail` when the parent directory is missing.
3. The registry. Build it with a throwaway `GameManager` and `FileManager`.
   A constructor that returns an error is a `fail` that names the provider.
   `NewSteamProvider` returns an error on an empty `api_key`, so that case
   reports itself.
4. One `ok` line per enabled provider.
5. One line per requirement from `registry.Requirements()`, sorted by
   binary. Call `lookPath` once per binary. `Detail` holds the resolved
   path or the reason it failed. `Remedy` names the providers that want it
   and the package that ships it.
6. The gallery, when `gallery.create` is true. Build the
   `GalleryBuilder` and read its requirements the same way.
7. `xbox_game_bar` is enabled and `runtime.GOOS` is not `windows`. That is
   a `warn`, because the run drops the provider.
8. `steam.online_gallery` is true and `steam.api_key` is empty. That is a
   `warn`. The rest of the Steam provider still runs.
9. Every provider with a `path` that is not `auto`. `fail` when the path is
   missing. `playstation4` and `playstation5` have no automatic path, so an
   enabled provider without a `path` is a `fail`.

A missing program is `fail` for a provider and `warn` for the gallery. A
gallery without `ffmpeg` still builds. It has no video thumbnail.

**cli.go — the subcommand**

`RunCLI` uses the package level `flag`. Take the subcommand off the argument
list before the parse:

```go
args := os.Args[1:]
doctorMode := false
if len(args) > 0 && args[0] == "doctor" {
	doctorMode = true
	args = args[1:]
}
// ... the flag.StringVar calls stay as they are ...
flag.CommandLine.Parse(args)
```

`-config` and `-log` still work after `doctor`. `-dry-run` and
`-only-build-gallery` mean nothing there, and the doctor ignores them.

After the config loads, and before the registry is built for a run:

```go
if doctorMode {
	results, err := RunDoctor(config)
	if err != nil {
		return err
	}
	PrintChecks(os.Stdout, results)
	if HasFailure(results) {
		return fmt.Errorf("doctor found %d failed checks", countFailures(results))
	}
	return nil
}
```

**cmd/games-screenshot-manager/main.go — the exit code**

`main` logs the error from `RunCLI` and returns. The process exits 0 on
every path today. Add `os.Exit(1)` after the log.

This changes the exit code of every failure, not the doctor alone. A script
that ignores the current exit code sees a new one. That is the intent: a
failed run must not report success.

## Output

Write plain text to stdout, not through `slog`. The doctor is a report, and
its lines are the product. `slog` writes to stdout too, so keep the two
apart with a heading and a blank line.

```
Configuration: /home/user/.config/games-screenshot-manager/config.toml

Providers
  ok    steam                 enabled
  ok    guild_wars_2          enabled
  ok    playstation5          enabled, path /media/usb/PS5
  warn  xbox_game_bar         enabled, but this host is linux; the run skips it

Dependencies
  ok    exiftool              /usr/bin/exiftool
                              wanted by guild_wars_2, playstation4
  fail  ffprobe               not found in PATH
                              wanted by playstation5, to read the length of a clip
                              Install the ffmpeg package, which ships ffprobe.

Gallery
  ok    create                enabled
  warn  ffmpeg                not found in PATH; a video gets no thumbnail

2 problems found.
```

## Docs

- `README.md`: a "Requirements" section. It lists `exiftool` and `ffmpeg`,
  and it says which provider wants each. Point at `doctor` from the Usage
  section.
- `config.example.toml`: one comment line above each provider that needs a
  program.

## Tests

New `doctor_test.go`:

- Replace `lookPath` with a stub. One program is present, one is missing.
- An enabled `playstation5` and a missing `ffprobe` give a `fail`.
- A disabled `playstation5` and a missing `ffprobe` give no `ffprobe` line.
- Two providers want `exiftool`. The result holds one `exiftool` line, and
  the line names both providers.
- `gallery.create` is true and `ffmpeg` is missing. The result is a `warn`,
  not a `fail`.
- `xbox_game_bar` is enabled and `runtime.GOOS` is not `windows`. The
  result is a `warn`. Skip this test on Windows.
- `steam.online_gallery` is true and `api_key` is empty. The result is a
  `warn`.
- `HasFailure` is true for a list with one `fail`. It is false for a list of
  `ok` and `warn`.

New `registry_test.go`:

- `Requirements` groups two providers that want one binary under one key.
- `Requirements` is empty for a config that enables only `minecraft`.

`steam_client_test.go`:

- `NewSteamClient` makes no HTTP request.
- The first `GetGameName` call downloads the app list. The second call does
  not.

## Acceptance

- `gofmt -l .` is empty.
- `go vet ./...` passes.
- `go test ./...` passes.
- `games-screenshot-manager doctor` on a config with every provider enabled
  reports each missing program once, and names every provider that wants it.
- The command exits 1 when a check fails. It exits 0 when none fails.
- The command writes no file, creates no directory, and makes no HTTP
  request.

---

## Notes

- 2026-09-06T09:07:41+02:00 — Providers declare their own requirements through an optional Requirer interface. The doctor builds the registry and asks each provider, so a requirement can depend on that provider's config.

  This needs the Steam app list download out of NewSteamClient (steam_client.go:150). A doctor that builds the registry would otherwise download the app list on every check.
- 2026-09-06T09:46:28+02:00 — Implemented and verified.

  Files: doctor.go (new), doctor_test.go (new), registry_test.go (new), provider.go (Requirement, Requirer, RequirementUse), registry.go (Requirements), exif.go (exiftoolRequirement), guildwars2.go, playstation4.go, playstation5.go, xboxgamebar.go, gallery.go (Requirements each), steam_client.go (lazy app list), cli.go (the subcommand), main.go (os.Exit(1)), README.md, config.example.toml.

  Five things came out different from the plan:

  1. RunDoctor(config *Config, configPath string) []CheckResult. It takes the config path, so the report can name the file, and it returns no error: a problem is a check, not an error.
  2. A gallery program is reported in the Dependencies section rather than a Gallery section of its own. A program is looked for once and gets one line, and the line names every component that wants it. A program only the gallery wants is a warn. The Gallery section reports gallery.create alone.
  3. Steam with an empty api_key is a fail from NewSteamProvider today, not a warn, so the doctor reports the constructor message under a registry check. The two Steam warns are a userdata_path the run does not read, and online_gallery without a user_id.
  4. CountProblems and CountFailures are exported next to HasFailure.
  5. xbox_game_bar on a host that is not Windows gives one warn line, not an ok line and a warn line.

  gofmt -l is empty. go vet and go test ./... pass. 27 new tests.

  Verified by running the built binary:
  - Every provider enabled, PATH emptied: exiftool fail naming guild_wars_2 and playstation4, ffprobe fail naming playstation5 and the gallery, ffmpeg warn naming the gallery. Exit 1.
  - Steam enabled with a bogus API key and XDG_CACHE_HOME on a temp folder: the cache folder stayed empty, so the doctor made no download. Exit 0 with 2 warns.
  - The real config on this machine: no problems found, exit 0.
  - No file and no directory was created below the output path.

  The change is in the working tree. It is not committed.
