package gamesscreenshotmanager

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type GalleryNodeKind string

const (
	GalleryNodeKindFolder GalleryNodeKind = "folder"
	GalleryNodeKindImage  GalleryNodeKind = "image"
	GalleryNodeKindVideo  GalleryNodeKind = "video"
)

// VideoMetadata represents cached metadata for video files
type VideoMetadata struct {
	FFmpegMetadata string  `json:"ffmpeg_metadata"`
	Duration       float64 `json:"duration"`
	Timestamp      string  `json:"timestamp"` // ISO8601/RFC3339
}

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

// getMetadataCachePath returns the path to the metadata cache file for a video
func (n *GalleryNode) getMetadataCachePath() string {
	return n.Path + ".metadata.json"
}

// GetVideoDuration returns the duration of a video file in seconds
// Returns 0 if the file is not a video or if there's an error getting the duration
func (n *GalleryNode) GetVideoDuration() float64 {
	if n.Kind() != GalleryNodeKindVideo {
		return 0
	}

	// Check if we have a metadata cache file
	cachePath := n.getMetadataCachePath()
	videoInfo, err := os.Stat(n.Path)
	if err != nil {
		slog.Error("failed to stat video file", slog.String("path", n.Path), slog.String("err", err.Error()))
		return 0
	}

	// Try to read from cache first
	if cacheInfo, err := os.Stat(cachePath); err == nil {
		// Check if cache is newer than the video file (cache is valid if it was created after the video was last modified)
		if !videoInfo.ModTime().After(cacheInfo.ModTime()) {
			cacheData, err := os.ReadFile(cachePath)
			if err == nil {
				var metadata VideoMetadata
				if err := json.Unmarshal(cacheData, &metadata); err == nil {
					// Cache hit - return the cached duration
					return metadata.Duration
				}
			}
		}
		// If we get here, the cache exists but is invalid or outdated
	}

	// No valid cache - run ffprobe and cache the result
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", n.Path)
	output, err := cmd.Output()
	if err != nil {
		slog.Error("failed to get video duration", slog.String("path", n.Path), slog.String("err", err.Error()))
		return 0
	}

	ffmpegOutput := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(ffmpegOutput, 64)
	if err != nil {
		slog.Error("failed to parse video duration", slog.String("path", n.Path), slog.String("err", err.Error()))
		return 0
	}

	// Cache the result
	metadata := VideoMetadata{
		FFmpegMetadata: ffmpegOutput,
		Duration:       duration,
		Timestamp:      time.Now().Format(time.RFC3339),
	}

	cacheData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		slog.Error("failed to marshal metadata", slog.String("path", n.Path), slog.String("err", err.Error()))
		// Non-fatal error, we can still return the duration
		return duration
	}

	if err := os.WriteFile(cachePath, cacheData, 0644); err != nil {
		slog.Error("failed to write metadata cache", slog.String("path", cachePath), slog.String("err", err.Error()))
		// Non-fatal error, we can still return the duration
	}

	return duration
}

// GetFormattedVideoDuration returns the duration of a video file formatted as:
// - "MM:SS" if the duration is 1 minute or longer
// - "XXs" if the duration is less than 1 minute
// Returns an empty string if the file is not a video, if there's an error, or if
// show_video_duration is disabled in the configuration
func (n *GalleryNode) GetFormattedVideoDuration(config *Config) string {
	// If showing video duration is disabled, return empty string
	if config != nil && !config.Gallery.ShowVideoDuration {
		return ""
	}

	duration := n.GetVideoDuration()
	if duration <= 0 {
		return ""
	}

	// Less than 1 minute: format as "XXs"
	if duration < 60 {
		return fmt.Sprintf("%ds", int(duration))
	}

	// 1 minute or longer: format as "MM:SS"
	totalSeconds := int(duration)
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
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

// GetLastUpdated returns the most recent modification time of original image and video files
// in this folder or its subfolders recursively. It returns this time formatted as RFC3339.
func (n *GalleryNode) GetLastUpdated() string {
	var latestTime time.Time
	
	// Check files in this folder - only consider original image and video files
	for _, file := range n.Files {
		// Only check original image and video files
		if file.Kind() == GalleryNodeKindImage || file.Kind() == GalleryNodeKindVideo {
			fileInfo, err := os.Stat(file.Path)
			if err != nil {
				slog.Error("failed to stat file", slog.String("path", file.Path), slog.String("err", err.Error()))
				continue
			}
			
			modTime := fileInfo.ModTime()
			if modTime.After(latestTime) {
				latestTime = modTime
			}
		}
	}
	
	// Recursively check subfolders
	for _, folder := range n.Folders {
		folderLastUpdated, err := time.Parse(time.RFC3339, folder.GetLastUpdated())
		if err != nil {
			slog.Error("failed to parse folder last updated time", slog.String("path", folder.Path), slog.String("err", err.Error()))
			continue
		}
		
		if folderLastUpdated.After(latestTime) {
			latestTime = folderLastUpdated
		}
	}
	
	// If no files found, return current time
	if latestTime.IsZero() {
		return time.Now().Format(time.RFC3339)
	}
	
	return latestTime.Format(time.RFC3339)
}
