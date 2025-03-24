package gamesscreenshotmanager

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
	toolkitTemplate "git.nakama.town/fmartingr/gotoolkit/template"
)

//go:embed templates/*.html
var Templates embed.FS

type GalleryBuilder struct {
	Config         Config
	Root           *GalleryNode
	IgnoreNames    []string
	templateEngine *toolkitTemplate.Engine
	log            *slog.Logger
	fileManager    *FileManager
}

func NewGalleryBuilder(config Config) (*GalleryBuilder, error) {
	engine, err := toolkitTemplate.NewEngine(Templates)
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
		},
		fileManager: NewFileManager(config),
		log:         slog.Default().With("component", "gallery"),
	}, nil
}

func (b *GalleryBuilder) Build() (*GalleryNode, error) {
	outputPath, err := filepath.Abs(toolkitPaths.ExpandUser(b.Config.OutputPath))
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for output path: %w", err)
	}

	b.Root = &GalleryNode{
		Title: b.Config.Gallery.SiteTitle,
		Path:  outputPath,
	}

	if err := os.Chdir(outputPath); err != nil {
		return nil, err
	}

	b.walkFolder(b.Root)

	b.buildSite(b.Root, time.Now().Format(time.RFC3339))

	return b.Root, nil
}

func (b *GalleryBuilder) walkFolder(parent *GalleryNode) {
	b.log.Debug("walking folder", slog.String("path", parent.Path))

	items, err := os.ReadDir(parent.Path)
	if err != nil {
		slog.Error("failed to read directory", slog.String("path", parent.Path), slog.String("err", err.Error()))
		return
	}

	for _, item := range items {
		node := GalleryNode{
			Title:  item.Name(),
			Parent: parent,
			Path:   filepath.Join(parent.Path, item.Name()),
		}

		if matches, _ := filepath.Match("cover.*", filepath.Base(item.Name())); matches {
			parent.Cover = filepath.Base(node.Path)
			continue
		}

		if item.IsDir() {
			b.walkFolder(&node)
			parent.AddFolder(&node)
			continue
		}

		for _, ignoreName := range b.IgnoreNames {
			matches, err := filepath.Match(ignoreName, filepath.Base(item.Name()))
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
				cmd := exec.Command("convert", node.Path, "-resize", "360x360>", node.Path+".thumb.jpg")
				output, err := cmd.CombinedOutput()
				if err != nil {
					slog.Error("failed to generate thumb", slog.String("path", node.Path), slog.String("err", err.Error()))
					slog.Error(string(output))
				}
			case GalleryNodeKindVideo:
				cmd := exec.Command("ffmpeg", "-i", node.Path, "-vf", `select=eq(n\,0)`, "-q:v", "3", node.Path+".thumb.jpg")
				output, err := cmd.CombinedOutput()
				if err != nil {
					slog.Error("failed to generate thumb", slog.String("path", node.Path), slog.String("err", err.Error()))
					slog.Error(string(output))
				}
			}

		}

		parent.AddFile(&node)

	skipnode:
	}
}

func (b *GalleryBuilder) buildSite(node *GalleryNode, lastUpdated string) {
	switch node.Kind() {
	case GalleryNodeKindFolder:
		result, err := b.templateEngine.Render("album.html", map[string]any{
			"Node":        &node,
			"LastUpdated": lastUpdated,
			"Title":       node.Title,
			"Context":     b.Config.Gallery.Context,
		})
		if err != nil {
			slog.Error("failed to render template", slog.String("path", node.Path), slog.String("err", err.Error()))
		}

		if err := b.fileManager.WriteFileIfModified(filepath.Join(node.Path, "index.html"), result); err != nil {
			slog.Error("failed to write template", slog.String("path", node.Path), slog.String("err", err.Error()))
		}

		for _, folder := range node.Folders {
			b.buildSite(folder, lastUpdated)
		}
	}
}
