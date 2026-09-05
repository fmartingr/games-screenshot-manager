package gamesscreenshotmanager

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// VideoMetadata represents cached metadata for video files
type VideoMetadata struct {
	FFmpegMetadata string  `json:"ffmpeg_metadata"`
	Duration       float64 `json:"duration"`
	Timestamp      string  `json:"timestamp"` // ISO8601/RFC3339
}

// GetVideoMetadataCachePath returns the path to the metadata cache file for a video file
func GetVideoMetadataCachePath(videoPath string) string {
	return videoPath + ".metadata.json"
}

// GetVideoDuration extracts the duration of a video file in seconds
// Returns 0 if there's an error getting the duration
func GetVideoDuration(videoPath string) float64 {
	// Check if we have a metadata cache file
	cachePath := GetVideoMetadataCachePath(videoPath)
	videoInfo, err := os.Stat(videoPath)
	if err != nil {
		slog.Error("failed to stat video file", slog.String("path", videoPath), slog.String("err", err.Error()))
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
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	output, err := cmd.Output()
	if err != nil {
		slog.Error("failed to get video duration", slog.String("path", videoPath), slog.String("err", err.Error()))
		return 0
	}

	ffmpegOutput := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(ffmpegOutput, 64)
	if err != nil {
		slog.Error("failed to parse video duration", slog.String("path", videoPath), slog.String("err", err.Error()))
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
		slog.Error("failed to marshal metadata", slog.String("path", videoPath), slog.String("err", err.Error()))
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
// Returns an empty string if there's an error
func GetFormattedVideoDuration(duration float64) string {
	if duration <= 0 {
		return ""
	}

	// Less than 1 minute: format as "XXs"
	if duration < 60 {
		return strings.TrimSpace(strconv.Itoa(int(duration)) + "s")
	}

	// 1 minute or longer: format as "MM:SS"
	totalSeconds := int(duration)
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	return strings.TrimSpace(strconv.Itoa(minutes) + ":" + strconv.Itoa(seconds))
}

// AdjustDatetimeByVideoDuration subtracts the duration of a video from a given datetime
// This is useful for providers like PlayStation 5 where the video timestamp is the end time
func AdjustDatetimeByVideoDuration(datetime time.Time, videoPath string) time.Time {
	duration := GetVideoDuration(videoPath)
	if duration <= 0 {
		return datetime
	}

	// Subtract the video duration to get the start time
	return datetime.Add(-time.Duration(duration) * time.Second)
}
