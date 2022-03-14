# Games Screenshot Manager

[![Go Report Card](https://goreportcard.com/badge/github.com/fmartingr/games-screenshot-manager)](https://goreportcard.com/report/github.com/fmartingr/games-screenshot-manager)

A simple tool to collect and sort games screenshots from different platforms.

## Supported providers

Use the appropriate ID with the `-provider` flag. [See examples below](#Usage)

| Name            | ID                | Linux | Windows | macOS | Covers | Notes                                               |
| --------------- | ----------------- | ----- | ------- | ----- | ------ | --------------------------------------------------- |
| Minecraft       | `minecraft`       | Yes   | Yes     | Yes   | No     |
| PlayStation 4   | `playstation-4`   | -     | -       | -     | No     | Requires `-input-path` pointing to PS4 folder       |
| RetroArch       | `retroarch`       | -     | -       | -     | Yes    | Requires `-input-path` pointing to Playlists folder |
| Steam           | `steam`           | Yes   | Yes     | Yes   | Yes    |
| Xbox Game Bar | `xbox-game-bar` | - | - | - | No | Requires `-input-path` pointing to the folder holding the captures |

## Requirements

- [exiftool](https://exiftool.org/) to parse EXIF data from files.

## How it works

Each provider has it's own way of finding the screenshots, but ideally the screenshots folder for games are known to us users so we only need to traverse them and find image files except for installations that may vary (like Retroarch) or systems outside of the PC ecosystem (Playstation, Nintendo Switch).

In some cases to have all the information for a particular provider we need to retrieve more data from the internet, for example the Steam game list to associate names to the IDs or in Nintendo Switch's case a community provided list to associate the internal ID with the Game's name.

For more details, you can check out [the source code for all providers](https://github.com/fmartingr/games-screenshot-manager/tree/master/pkg/providers)

Optionally a cover image for a game can be downloaded and placed under a `.cover` file in the game path. For this to work use the `-download-cover` flag. Check above for provider support for this feature.

## Installation

```
go get -u github.com/fmartingr/games-screenshot-manager
```

Or get the latest binary build from the [releases page](https://github.com/fmartingr/games-screenshot-manager/releases)

## Usage

```
# Help
games-screenshot-manager -h

# Fetch and sort all Steam screenshots into ./Output
games-screenshot-manager -provider steam -output-path ./Output

# Like the one above but it'll download all header images for the games
games-screenshot-manager -provider steam -output-path ./Output -download-covers

# Perform a dry run (see what's gonna get copied where)
games-screenshot-manager -provider steam -dry-run

# Parse all Nintendo Switch screenshots
games-screenshot-manager -provider nintendo-switch -input-path ./Album
```
