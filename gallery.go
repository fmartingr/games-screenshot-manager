package gamesscreenshotmanager

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
	toolkitTemplate "git.nakama.town/fmartingr/gotoolkit/template"
	"github.com/fmartingr/games-screenshot-manager/templates"
	"golang.org/x/image/draw"
)

type GalleryBuilder struct {
	Config         Config
	Root           *GalleryNode
	IgnoreNames    []string
	templateEngine *toolkitTemplate.Engine
	log            *slog.Logger
	fileManager    *FileManager
}

func NewGalleryBuilder(config Config) (*GalleryBuilder, error) {
	var templateFS fs.FS

	// If there's a `templates` folder in the path where the config files is, use that
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user config directory: %w", err)
	}
	configPath := filepath.Join(userConfigDir, "games-screenshot-manager", "templates")

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		root, err := os.OpenRoot(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open templates directory: %w", err)
		}

		templateFS = root.FS()
	} else {
		templateFS = templates.Templates
	}

	engine, err := toolkitTemplate.NewEngine(templateFS)
	if err != nil {
		return nil, fmt.Errorf("failed to create template engine: %w", err)
	}

	return &GalleryBuilder{
		Config:         config,
		templateEngine: engine,
		IgnoreNames: []string{
			"cover.*",
			".DS_Store",
			"index.html",
			"*.thumb.jpg",
			"*.metadata.json",
		},
		fileManager: NewFileManager(config),
		log:         slog.Default().With("component", "gallery"),
	}, nil
}

func (b *GalleryBuilder) Build() (*GalleryNode, error) {
	b.log.Info("building gallery", slog.String("output_path", b.Config.OutputPath))

	outputPath := toolkitPaths.ExpandUser(filepath.Join("./", b.Config.OutputPath))

	b.Root = &GalleryNode{
		Title: b.Config.Gallery.SiteTitle,
		Path:  ".",
	}

	if err := os.Chdir(outputPath); err != nil {
		return nil, fmt.Errorf("failed to change directory to output path: %w", err)
	}

	b.walkFolder(b.Root)

	b.buildSite(b.Root)

	return b.Root, nil
}

// generateThumbnail creates a thumbnail for the given image path
func (b *GalleryBuilder) generateThumbnail(sourcePath string) error {
	// Open the source file
	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}
	defer file.Close()

	// Decode the image based on its format
	var img image.Image
	ext := strings.ToLower(filepath.Ext(sourcePath))

	switch ext {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
	case ".png":
		img, err = png.Decode(file)
	default:
		return fmt.Errorf("unsupported image format: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Create a thumbnail (resize the image)
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Calculate new dimensions (max 360x360)
	maxSize := 360
	var newWidth, newHeight int

	if width > height {
		if width > maxSize {
			newWidth = maxSize
			newHeight = height * maxSize / width
		} else {
			newWidth, newHeight = width, height
		}
	} else {
		if height > maxSize {
			newHeight = maxSize
			newWidth = width * maxSize / height
		} else {
			newWidth, newHeight = width, height
		}
	}

	// Create the resized image
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(dst, dst.Rect, img, bounds, draw.Over, nil)

	// Create the output file with .thumb.jpg extension
	// For now, we'll use JPEG thumbnails since Go's standard library supports this
	thumbPath := sourcePath + ".thumb.jpg"
	outFile, err := os.Create(thumbPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer outFile.Close()

	// Encode as JPEG with high quality
	if err := jpeg.Encode(outFile, dst, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("failed to encode jpeg: %w", err)
	}

	return nil
}

// folderEntry pairs a directory entry with the name it has on disk after the
// rename to NFC. A rename that is skipped or fails leaves the original name, so
// the node, the link and the file always agree.
type folderEntry struct {
	dirEntry os.DirEntry
	name     string
}

// renameToNFC renames an entry to its NFC form. It goes through a temporary
// name, because a normalization-insensitive volume such as APFS resolves both
// forms to one file, and POSIX rename is a no-op in that case.
func renameToNFC(path, name, normalized string) error {
	oldPath := filepath.Join(path, name)
	newPath := filepath.Join(path, normalized)
	tempPath := filepath.Join(path, fmt.Sprintf(".nfc-%d-%s", os.Getpid(), normalized))

	if err := os.Rename(oldPath, tempPath); err != nil {
		return fmt.Errorf("failed to rename %s to a temporary name: %w", oldPath, err)
	}

	if err := os.Rename(tempPath, newPath); err != nil {
		if restoreErr := os.Rename(tempPath, oldPath); restoreErr != nil {
			return fmt.Errorf("failed to rename %s to %s, and %s is left behind: %w", oldPath, newPath, tempPath, err)
		}
		return fmt.Errorf("failed to rename %s to %s: %w", oldPath, newPath, err)
	}

	return nil
}

// normalizeFolder reads a folder and renames every entry that is not in NFC
// form. A web server matches the raw bytes of a URL, so the names on disk and
// the links in the gallery must use one form. It returns the name each entry
// has on disk afterwards.
func (b *GalleryBuilder) normalizeFolder(path string) ([]folderEntry, error) {
	items, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// A byte-exact set of the names in the folder. os.Stat cannot serve here,
	// because a normalization-insensitive volume reports the NFC name as
	// present while the name it stores is still NFD.
	existing := make(map[string]struct{}, len(items))
	for _, item := range items {
		existing[item.Name()] = struct{}{}
	}

	// Names whose rename did not happen. A sidecar such as <name>.thumb.jpg
	// keeps the name of its media file, so it must not move on its own.
	kept := make(map[string]struct{})

	entries := make([]folderEntry, 0, len(items))
	for _, item := range items {
		entries = append(entries, folderEntry{
			dirEntry: item,
			name:     b.normalizeEntry(path, item.Name(), existing, kept),
		})
	}

	return entries, nil
}

// normalizeEntry renames one entry to NFC and returns the name it has on disk
// afterwards. Every path that does not rename returns the original name.
func (b *GalleryBuilder) normalizeEntry(path, name string, existing, kept map[string]struct{}) string {
	normalized := NormalizeName(name)
	if normalized == name {
		return name
	}

	oldPath := filepath.Join(path, name)

	// A sidecar is its media file's name plus a suffix, as in <name>.thumb.jpg.
	for base := range kept {
		if strings.HasPrefix(name, base+".") {
			b.log.Warn(
				"skipped rename to NFC form, the media file it belongs to kept its name",
				slog.String("path", oldPath),
				slog.String("media_file", filepath.Join(path, base)),
			)
			kept[name] = struct{}{}
			return name
		}
	}

	if _, collides := existing[normalized]; collides {
		b.log.Warn(
			"skipped rename to NFC form, destination exists",
			slog.String("old_path", oldPath),
			slog.String("new_path", filepath.Join(path, normalized)),
		)
		kept[name] = struct{}{}
		return name
	}

	if b.Config.DryRun {
		b.log.Info("rename to NFC form", slog.String("old_path", oldPath), slog.String("new_path", filepath.Join(path, normalized)))
		kept[name] = struct{}{}
		return name
	}

	if err := renameToNFC(path, name, normalized); err != nil {
		b.log.Error("failed to rename to NFC form", slog.String("path", oldPath), slog.String("err", err.Error()))
		kept[name] = struct{}{}
		return name
	}

	delete(existing, name)
	existing[normalized] = struct{}{}

	b.log.Info("renamed to NFC form", slog.String("old_path", oldPath), slog.String("new_path", filepath.Join(path, normalized)))

	return normalized
}

func (b *GalleryBuilder) walkFolder(parent *GalleryNode) {
	b.log.Debug("walking folder", slog.String("path", parent.Path))

	items, err := b.normalizeFolder(parent.Path)
	if err != nil {
		b.log.Error("failed to read directory", slog.String("path", parent.Path), slog.String("err", err.Error()))
		return
	}

	for _, item := range items {
		node := GalleryNode{
			Title:  item.name,
			Parent: parent,
			Path:   filepath.Join(parent.Path, item.name),
		}

		if matches, _ := filepath.Match("cover.*", item.name); matches {
			parent.Cover = filepath.Base(node.Path)
			continue
		}

		if item.dirEntry.IsDir() {
			b.walkFolder(&node)
			parent.AddFolder(&node)
			continue
		}

		for _, ignoreName := range b.IgnoreNames {
			matches, err := filepath.Match(ignoreName, item.name)
			if err != nil {
				// Only invalid patterns, but lets leave it there just in case I mess up with the config
				panic(err)
			}
			if matches {
				goto skipnode
			}
		}

		if !node.HasThumb() {
			switch node.Kind() {
			case GalleryNodeKindImage:
				if err := b.generateThumbnail(node.Path); err != nil {
					b.log.Error("failed to generate thumbnail", slog.String("path", node.Path), slog.String("err", err.Error()))
				}
			case GalleryNodeKindVideo:
				cmd := exec.Command("ffmpeg", "-i", node.Path, "-vf", `select=eq(n\,0)`, "-q:v", "3", "-update", "1", "-frames:v", "1", node.Path+".thumb.jpg")
				output, err := cmd.CombinedOutput()
				if err != nil {
					b.log.Error("failed to generate thumb", slog.String("path", node.Path), slog.String("err", err.Error()))
					b.log.Error(string(output))
				}
			}

		}

		parent.AddFile(&node)

	skipnode:
	}
}

func (b *GalleryBuilder) buildSite(node *GalleryNode) {
	switch node.Kind() {
	case GalleryNodeKindFolder:
		// Get the last updated time for this specific node
		lastUpdated := node.GetLastUpdated()

		result, err := b.templateEngine.Render("album.html", map[string]any{
			"Node":        &node,
			"LastUpdated": lastUpdated,
			"Title":       node.Title,
			"Context":     b.Config.Gallery.Context,
			"Config":      &b.Config,
		})
		if err != nil {
			slog.Error("failed to render template", slog.String("path", node.Path), slog.String("err", err.Error()))
		}

		if err := b.fileManager.WriteFileIfModified(filepath.Join(node.Path, "index.html"), result); err != nil {
			slog.Error("failed to write template", slog.String("path", node.Path), slog.String("err", err.Error()))
		}

		for _, folder := range node.Folders {
			b.buildSite(folder)
		}
	}
}
