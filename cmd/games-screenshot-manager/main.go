package main

import (
	"log/slog"
	"os"

	gamesscreenshotmanager "github.com/fmartingr/games-screenshot-manager"
)

func main() {
	if err := gamesscreenshotmanager.RunCLI(); err != nil {
		slog.Error("Error", slog.Any("error", err))
		// A failed run must not report success. This covers the doctor, which
		// exits 1 when a check fails.
		os.Exit(1)
	}
}
