package main

import (
	"log/slog"

	gamesscreenshotmanager "github.com/fmartingr/games-screenshot-manager"
)

func main() {
	if err := gamesscreenshotmanager.RunCLI(); err != nil {
		slog.Error("Error", slog.Any("error", err))
	}
}
