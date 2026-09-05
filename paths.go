package gamesscreenshotmanager

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// expandUser replaces a leading "~" with the home directory of the current
// user. A path that does not start with "~" comes back unchanged.
func expandUser(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}

	home := homeDir()
	if home == "" {
		return path
	}

	if path == "~" {
		return home
	}

	return filepath.Join(home, path[2:])
}

// homeDir returns the home directory of the current user. It reads $HOME first
// and falls back to the OS user database, because $HOME is not set under
// systemd, under cron, or inside a container.
func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}

	if current, err := user.Current(); err == nil {
		return current.HomeDir
	}

	return ""
}
