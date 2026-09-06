# Games Screenshot Manager

[![Go Report Card](https://goreportcard.com/badge/github.com/fmartingr/games-screenshot-manager)](https://goreportcard.com/report/github.com/fmartingr/games-screenshot-manager)

A simple tool to collect and sort games screenshots from different platforms.

## Features

- Collect screenshots taken from different platforms/games. See [Supported providers](#supported-providers) for the list of supported providers.
  - **Steam**
  - **Guild Wars 2**
  - **Hytale**
  - **Minecraft**
  - **Nintendo Switch 2**
  - **PlayStation 4**
  - **PlayStation 5**
  - **World of Warcraft**
  - **Xbox Game Bar**
- Automatically organize screenshots by platform and game in chronological order.
- Download covers for the games in some providers automatically.
- Generate a gallery of the screenshots. ([check mine](https://screenshots.fmartingr.com))

## Quickstart

### Installation

```
go install github.com/fmartingr/games-screenshot-manager/cmd/games-screenshot-manager@latest
```

Or get the latest binary build from the [releases page](https://github.com/fmartingr/games-screenshot-manager/releases)

### Configuration

Create a configuration file following the [example config](./config.example.toml).

The file contains a step by step guide to set up each section, go through the file and set up the features you want to use.

### Usage

Run the `games-screenshot-manager` command:

```
games-screenshot-manager -config ./config.toml
```

Check the setup before a run with the `doctor` subcommand:

```
games-screenshot-manager doctor -config ./config.toml
```

It reports the enabled providers, the paths they read, and the external programs they need. It changes no file, and it exits 1 when a check fails.

If no `-config` flag is provided, the binary will look for a `config.toml` file in the user configuration directory, which varies by system:

- Linux: `$XDG_CONFIG_HOME/games-screenshot-manager/config.toml`
- Windows: `%APPDATA%\games-screenshot-manager\config.toml`
- macOS: `~/Library/Application Support/games-screenshot-manager/config.toml`

## Supported providers

### Guild Wars 2

- **Supported platforms**: Windows (other platforms require a custom path)
- **Screenshots**: Yes
- **Clips**: Not supported by the game
- **Recordings**: Not supported by the game

### Hytale

- **Supported platforms**: macOS, Linux
- **Screenshots**: Yes
- **Clips**: Not supported by the game
- **Recordings**: Not supported by the game
- **Covers**: Yes, provided by us.

### Minecraft

- **Supported platforms**: Windows, macOS, Linux (launcher) and Linux (flatpak).
- **Screenshots**: Yes
- **Clips**: Not supported by the game
- **Recordings**: Not supported by the game

### Nintendo Switch 2

- **Supported platforms**: Linux, to find the console over USB. Any platform with a custom path.
- **Screenshots**: Yes
- **Clips**: Yes
- **Recordings**: Not supported by the console
- **Covers**: No

The console shares its album over MTP. Open the album on the console, copy it to a computer, and pick this computer. The console then appears on the USB bus as `057e:2061`, and the run finds it on its own.

Every capture keeps the name the console gave it, as in `2025060720031600_s.jpg`. A `_c` file is a copy of a capture that already exists somewhere else, so it is never collected. A capture the output folder already holds stays on the console, so every run after the first one is quick.

The console makes one album folder of its own for the captures taken outside a game. It names that folder in the language the console is set to, so the name lives in `ignored_folders` in [the config file](./config.example.toml) rather than in the code. The same list drops a game you do not want.

Only one program at a time can hold an MTP device, and a desktop file manager takes the console as soon as it appears. Close the file manager, and eject the console from it, before the run.

A program that dies while it holds the console leaves the session open, and nothing can read the album after that. Unplug the cable, plug it back, and share the album again.

Set `path` to read an album folder that was already copied off a console, on any platform.

### PlayStation 4

- **Supported platforms**: None, requires a custom path.
- **Screenshots**: Yes
- **Clips**: Yes (30s videos)
- **Recordings**: Not supported by the game

### PlayStation 5

- **Supported platforms**: None, requires a custom path.
- **Screenshots**: Yes
- **Clips**: Yes (30s videos)
- **Recordings**: Not supported by the game

### Steam

- **Supported platforms**: Windows, macOS and Linux.
- **Screenshots**: Yes
- **Clips**: No
- **Recordings**: No
- **Covers**: Yes, auto-downloads the header from Steam

Games can be skipped with `ignored_games` in [the config file](./config.example.toml), which takes a list of Steam app IDs. A skipped game gets no folder, no screenshots and no cover.

Game names come from the Steam app list. That list holds no demo, so the name of a demo is asked of the Steam store one app ID at a time, and the answer is cached.

Screenshots you published to the Steam online gallery are downloaded as well. Set `online_gallery = true`, `user_id` and `api_key` under `[providers.steam]` in [the config file](./config.example.toml).

### World of Warcraft

- **Supported platforms**: Windows, macOS. (Other platforms require a custom path)
- **Screenshots**: Yes
- **Clips**: Not supported by the game
- **Recordings**: Not supported by the game

### Xbox Game Bar

- **Supported platforms**: Windows only.
- **Screenshots**: Yes
- **Clips**: No
- **Recordings**: No

## Requirements

Each program is needed only by the features that call it. Run `games-screenshot-manager doctor` to see which ones your configuration needs.

| Program | Package | Needed by |
| --- | --- | --- |
| `exiftool` | [exiftool](https://exiftool.org/) | **PlayStation 4**, **Guild Wars 2** and **Xbox Game Bar**, to read the capture date from a screenshot. (Using the [barasher/go-exiftool library](https://github.com/barasher/go-exiftool)) |
| `ffprobe` | ffmpeg | **PlayStation 5**, to read the length of a clip. The gallery also uses it to show that length. |
| `ffmpeg` | ffmpeg | The gallery, to make the thumbnail for a video. |
| `mtp-folders`, `mtp-files`, `mtp-connect` | [libmtp](https://libmtp.sourceforge.net/) | **Nintendo Switch 2**, to list the album on the console and copy the captures off it. |

## What about the first Nintendo Switch?

This project initially started as a Nintendo Switch helper to import and properly organize Switch screenshots since their system was a mess, but Nintendo improved this over the years and now we can use Android File Transfer to easily get the screenshots from a Nintendo Switch with the proper game name as folder name and the screenshots sorted by date.

For more information about this, [read this issue](https://github.com/RenanGreca/Switch-Screenshots/issues/46)

The Nintendo Switch 2 has [a provider of its own](#nintendo-switch-2), which reads the console over USB.

## License

See [LICENSE](LICENSE).
