package gamesscreenshotmanager

import (
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandUser(t *testing.T) {
	// A fixed home makes the expectation independent of the machine, so the
	// test cannot pass by moving in step with the implementation.
	home := "/tmp/expanduser-home"

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "A bare tilde is the home directory",
			path:     "~",
			expected: home,
		},
		{
			name:     "A tilde prefix is replaced",
			path:     "~/Pictures/Screenshots",
			expected: filepath.Join(home, "Pictures", "Screenshots"),
		},
		{
			name:     "An absolute path is unchanged",
			path:     "/var/lib/games",
			expected: "/var/lib/games",
		},
		{
			name:     "A relative path is unchanged",
			path:     "Output",
			expected: "Output",
		},
		{
			name:     "A tilde inside the path is unchanged",
			path:     "/home/user/~backup",
			expected: "/home/user/~backup",
		},
		{
			name:     "A tilde with a user name is unchanged",
			path:     "~other/Pictures",
			expected: "~other/Pictures",
		},
		{
			name:     "An empty path is unchanged",
			path:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", home)

			if result := expandUser(tt.path); result != tt.expected {
				t.Errorf("expandUser(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// $HOME is not set under systemd, under cron, or inside a container. The path
// must still expand there, or the caller creates a directory named "~".
func TestExpandUserWithoutHOME(t *testing.T) {
	// A distroless image, or a rootless container with a synthetic uid, has no
	// entry in the user database. The fallback has nothing to return there and
	// the implementation is still correct.
	current, err := user.Current()
	if err != nil || current.HomeDir == "" {
		t.Skip("the user database holds no home directory for this user")
	}

	t.Setenv("HOME", "")

	result := expandUser("~/Pictures")

	if strings.HasPrefix(result, "~") {
		t.Errorf("expandUser() = %q, want a path expanded from the user database", result)
	}

	if !filepath.IsAbs(result) {
		t.Errorf("expandUser() = %q, want an absolute path", result)
	}

	if filepath.Base(result) != "Pictures" {
		t.Errorf("expandUser() = %q, want it to end in %q", result, "Pictures")
	}
}
