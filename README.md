# Games Screenshot Manager

[![Go Report Card](https://goreportcard.com/badge/github.com/fmartingr/games-screenshot-manager)](https://goreportcard.com/report/github.com/fmartingr/games-screenshot-manager)

A simple tool to collect and sort games screenshots from different platforms.

## Installation

```
go install github.com/fmartingr/games-screenshot-manager/cmd/games-screenshot-manager@latest
```

Or get the latest binary build from the [releases page](https://github.com/fmartingr/games-screenshot-manager/releases)

## Usage

Create a configuration file following the [example config](./config.example.toml).

Run the binary or the installed command:

```
games-screenshot-manager -config ./config.toml
```

If no `-config` flag is provided, the binary will look for a `config.toml` file in the user configuration directory, which varies by system:

- Linux: `$XDG_CONFIG_HOME/games-screenshot-manager/config.toml`
- Windows: `%APPDATA%\games-screenshot-manager\config.toml`
- macOS: `~/Library/Application Support/games-screenshot-manager/config.toml`

## Supported providers

| Name              | Linux | Windows | macOS | Screenshots | Clips | Recordings | Covers | Notes                                                                                                                  |
| ----------------- | ----- | ------- | ----- | ----------- | ----- | ---------- | ------ | ---------------------------------------------------------------------------------------------------------------------- |
| Guild Wars 2      | No    | Yes     | No    | Yes         | -     | --         | No     | Works on unsupported platforms with `path` pointing to screenshots folder                                              |
| Minecraft         | Yes   | Yes     | Yes   | Yes         | -     | --         | No     |                                                                                                                        |
| PlayStation 4     | -     | -       | -     | Yes         | Yes   | --         | No     | Requires `path` pointing to PS4 folder                                                                                 |
| PlayStation 5     | -     | -       | -     | Yes         | Yes   | --         | No     | Requires `path` pointing to PS5 folder                                                                                 |
| Steam             | Yes   | Yes     | Yes   | Yes         | No    | No         | No     | Also supports downloading screenshots uploaded to Steam by specifying `steam_user_id` and `api_key` in the config file |
| World of Wardraft | No    | Yes     | Yes   | Yes         | Yes   | --         | No     | Works on unsupported platforms with `path` pointing to screenshots folder                                              |
| Xbox Game Bar     | -     | Yes     | -     | Yes         | No    | No         | No     | Requires `path` pointing to the folder holding the captures                                                            |

## Requirements

- [exiftool](https://exiftool.org/) to parse EXIF data from files. (Using [baransher/go-exiftool library](https://github.com/barasher/go-exiftool)) This is required for the **PlayStation 4**, **Guild Wars 2** and **Xbox Game Bar** providers.

## What about Nintendo Switch?

This project initially started as a Nintendo Switch helper to import and properly organize Switch screenshots since their system was a mess, but Nintendo improved this over the years and now we can use Android File Transfer to easily get the screenshots from a Nintendo Switch with the proper game name as folder name and the screenshots sorted by date.

For more information about this, [read this issue](https://github.com/RenanGreca/Switch-Screenshots/issues/46)
