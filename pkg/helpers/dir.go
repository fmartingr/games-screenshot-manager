package helpers

import (
	"os/user"
	"path/filepath"
	"strings"
)

func ExpandUser(providedPath string) string {
	var path string
	usr, _ := user.Current()
	dir := usr.HomeDir

	if providedPath == "~" {
		path = dir
	} else if strings.HasPrefix(providedPath, "~/") {
		path = filepath.Join(dir, providedPath[2:])
	}
	return path
}
