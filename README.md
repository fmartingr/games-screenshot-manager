# Games Screenshot Manager

A simple tool to collect and sort games screenshots from different platforms.

## Supported platforms

| Name | Supported OS |
| --- | --- |
| Steam | Linux
| Minecraft | Linux, Linux Flatpak, macOS, Windows

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
```
