package gamesscreenshotmanager

import (
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// A failed render must leave the page on disk alone. html/template writes as it
// goes, so the buffer holds the part it managed before the error.
func TestGalleryBuilder_buildSiteKeepsThePageOnARenderError(t *testing.T) {
	root := t.TempDir()

	existing := []byte("<html>the page that is already published</html>")
	if err := os.WriteFile(filepath.Join(root, "index.html"), existing, 0644); err != nil {
		t.Fatalf("failed to write the existing page: %v", err)
	}

	builder := &GalleryBuilder{
		Config:      Config{},
		log:         slog.Default(),
		fileManager: NewFileManager(Config{}),
		templates:   template.Must(template.New("album.html").Parse(`<html>PREFIX{{ .Node.Explode }}TAIL`)),
	}

	builder.buildSite(&GalleryNode{Title: "Album", Path: root})

	result, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil {
		t.Fatalf("failed to read the page: %v", err)
	}

	if string(result) != string(existing) {
		t.Errorf("index.html = %q, want the existing page %q", result, existing)
	}
}
