package gamesscreenshotmanager

import (
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type GalleryNodeKind string

const (
	GalleryNodeKindFolder GalleryNodeKind = "folder"
	GalleryNodeKindImage  GalleryNodeKind = "image"
	GalleryNodeKindVideo  GalleryNodeKind = "video"
)

type GalleryNode struct {
	Title   string
	Path    string
	Parent  *GalleryNode
	Folders []*GalleryNode `json:",omitempty"`
	Files   []*GalleryNode `json:",omitempty"`
	Cover   string         `json:",omitempty"`
}

func (n *GalleryNode) AddFolder(child *GalleryNode) {
	n.Folders = append(n.Folders, child)
}

func (n *GalleryNode) AddFile(child *GalleryNode) {
	n.Files = append(n.Files, child)
}

func (n *GalleryNode) Kind() GalleryNodeKind {
	extension := filepath.Ext(n.Title)
	switch extension {
	case ".jpg", ".jpeg", ".png", ".webp":
		return GalleryNodeKindImage
	case ".mp4", ".avi", ".mkv", ".webm":
		return GalleryNodeKindVideo
	default:
		return GalleryNodeKindFolder
	}
}

func (n *GalleryNode) GetWebPath() string {
	u, err := url.JoinPath("/", strings.Replace(n.Path, "?", "%3F", -1))
	if err != nil {
		slog.Error("failed to join web path", slog.String("path", n.Path), slog.String("err", err.Error()))
	}
	return u
}

func (n *GalleryNode) GetThumbPath() string {
	return n.GetWebPath() + ".thumb.jpg"
}

func (n *GalleryNode) GetWebCoverPath() string {
	if n.Cover == "" {
		return ""
	}
	u, err := url.JoinPath(n.GetWebPath(), n.Cover)
	if err != nil {
		slog.Error("failed to join cover path", slog.String("path", n.GetWebPath()), slog.String("err", err.Error()))
	}
	return u
}

func (n *GalleryNode) Empty() bool {
	return len(n.Folders) == 0 && len(n.Files) == 0
}

func (n *GalleryNode) HasThumb() bool {
	if _, err := os.Stat(n.Path + ".thumb.jpg"); err != nil {
		return false
	}
	return true
}

func (n *GalleryNode) GetImageCount() int {
	count := 0
	for _, file := range n.Files {
		if file.Kind() == GalleryNodeKindImage {
			count++
		}
	}
	return count
}

func (n *GalleryNode) GetVideoCount() int {
	count := 0
	for _, file := range n.Files {
		if file.Kind() == GalleryNodeKindVideo {
			count++
		}
	}
	return count
}

func (n *GalleryNode) GetFolderCount() int {
	return len(n.Folders)
}

func (n *GalleryNode) GetBreadcrumbs() []*GalleryNode {
	var crumbs []*GalleryNode
	for current := n; current != nil; current = current.Parent {
		crumbs = append([]*GalleryNode{current}, crumbs...)
	}
	return crumbs
}
