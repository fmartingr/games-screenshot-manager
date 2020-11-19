# Games Screenshot Manager

A simple tool to collect and sort games screenshots from different platforms.

## Supported providers

Use the appropriate ID with the `-provider` flag. [See examples below](#Usage)

| Name | ID | Notes |
| --- | --- | --- |
| Steam | `steam` | Linux, macOS, Windows
| Minecraft | `minecraft` | Linux, Linux Flatpak, macOS, Windows
| Nintendo Switch | `nintend-switch` | Requires `-input-path` |

## How it works

Each provider has it's own way of finding the screenshots, but ideally the screenshots folder for games are known to us users so we only need to traverse them and find image files.

When talking about a game's store (Steam, for example) we need to retrieve a bit more info from internet to identify each game name from the internal ID used by Steam on disk.

For more details, you can check out [the source code for all providers](https://github.com/fmartingr/games-screenshot-manager/tree/master/pkg/providers)

## Installation

```
go get -u github.com/fmartingr/games-screenshot-manager
```

Or get a binary build from the [releases page](https://github.com/fmartingr/games-screenshot-manager/releases)

## Usage

```
# Help
games-screenshot-manager -h

# Fetch and sort all Steam screenshots into ./Output
games-screenshot-manager -provider steam -output-path ./Output

# Perform a dry run (see what's gonna get copied where)
games-screenshot-mananger -provider steam -dry-run

# Parse all Nintendo Switch screenshots
games-screenshot-manager -provider nintendo-switch -input-path ./Album
```
