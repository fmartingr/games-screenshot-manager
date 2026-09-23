package gamesscreenshotmanager

import (
	"os"
	"path/filepath"
	"testing"
)

// An absolute output_path must be used as it is. Build once joined it to
// "./", which turned "/srv/shots" into the relative "srv/shots", so the
// gallery could only be built from the root directory.
func TestGalleryBuilder_BuildAcceptsAnAbsoluteOutputPath(t *testing.T) {
	output := t.TempDir()
	if err := os.Mkdir(filepath.Join(output, "Some Game"), 0755); err != nil {
		t.Fatalf("failed to create the game folder: %v", err)
	}

	// Build changes the working directory; start from somewhere else and let
	// the test put it back.
	t.Chdir(t.TempDir())
	// Keep a templates folder in the real user config from being picked up.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	config := Config{}
	config.Defaults()
	config.OutputPath = output

	builder, err := NewGalleryBuilder(config)
	if err != nil {
		t.Fatalf("failed to create the builder: %v", err)
	}

	if _, err := builder.Build(); err != nil {
		t.Fatalf("Build failed with an absolute output path: %v", err)
	}

	if _, err := os.Stat(filepath.Join(output, "index.html")); err != nil {
		t.Fatalf("the gallery index was not written into the output path: %v", err)
	}
}
