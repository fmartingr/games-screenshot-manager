package gamesscreenshotmanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecordingsIntegration runs the full recordings processing pipeline
// using the testdata directory, mimicking what GetRecordings does
// but without requiring the Steam API client.
func TestRecordingsIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	basePath := filepath.Join("testdata", "GameRecordings")
	outputDir := t.TempDir()

	// Parse the protobuf index
	pbPath := filepath.Join(basePath, "gamerecording.pb")
	recordings, err := ParseGameRecordingPB(pbPath)
	require.NoError(t, err)
	require.NotEmpty(t, recordings)

	config := Config{OutputPath: outputDir}
	gameManager := NewGameManager()
	fileManager := NewFileManager(config)
	defer fileManager.Cleanup()

	// Process only recordings that have video data in testdata
	// (bg_105600_20250701_095836 is the only folder with actual m4s files)
	var targetRecording *SteamRecording
	for i := range recordings {
		if recordings[i].TimelineName == "timeline_10560020250701_095820" {
			targetRecording = &recordings[i]
			break
		}
	}
	require.NotNil(t, targetRecording)

	t.Logf("Processing recording: app=%d timeline=%s segments=%d",
		targetRecording.AppID, targetRecording.TimelineName, len(targetRecording.VideoSegments))

	// Filter to only segments with actual video data
	var validSegments []SteamVideoSegment
	for _, seg := range targetRecording.VideoSegments {
		segPath := filepath.Join(basePath, "video", seg.FolderName)
		mpdPath := filepath.Join(segPath, "session.mpd")
		if _, err := os.Stat(mpdPath); err == nil {
			validSegments = append(validSegments, seg)
		}
	}
	require.NotEmpty(t, validSegments, "need at least one video segment with data")

	t.Logf("Valid video segments: %d", len(validSegments))

	// Process video segment
	segPath := filepath.Join(basePath, "video", validSegments[0].FolderName)
	videoPath, err := processVideoSegment(segPath)
	require.NoError(t, err)
	defer os.Remove(videoPath)

	t.Logf("Created video: %s", videoPath)

	// Verify video duration matches MPD
	duration := GetVideoDuration(videoPath)
	defer os.Remove(GetVideoMetadataCachePath(videoPath))
	assert.InDelta(t, 14.96, duration, 0.5, "video duration should match MPD")
	t.Logf("Video duration: %.2f seconds", duration)

	// Extract thumbnail
	thumbPath, err := extractFrame(videoPath, 0)
	require.NoError(t, err)
	defer os.Remove(thumbPath)

	thumbInfo, err := os.Stat(thumbPath)
	require.NoError(t, err)
	assert.Greater(t, thumbInfo.Size(), int64(1000), "thumbnail should be a real JPEG")
	t.Logf("Thumbnail: %s (%d bytes)", thumbPath, thumbInfo.Size())

	// Parse the timeline (this one has no entries, so test with the Factorio one)
	timelinePath := filepath.Join(basePath, "timelines", "timeline_42752020250315_173406.json")
	timeline, err := ParseTimeline(timelinePath)
	require.NoError(t, err)
	require.NotEmpty(t, timeline.Entries)
	t.Logf("Timeline entries: %d", len(timeline.Entries))

	for _, entry := range timeline.Entries {
		t.Logf("  [%s] %s at %s ms (achievement=%s)",
			entry.ID, entry.Type, entry.Time, entry.AchievementName)
	}

	// Now simulate the full file manager flow: create game, add media, process
	game := NewGame("105600", "Terraria", "PC", "steam")
	gameManager.AddGame(game)

	videoDestName := "2025-07-01_09-58-20.mp4"

	videoMedia := NewMedia(MediaKindRecording, videoPath)
	videoMedia.DestinationName = videoDestName
	game.AddRecording(videoMedia)

	thumbMedia := NewMedia(MediaKindRecording, thumbPath)
	thumbMedia.DestinationName = videoDestName + ".thumb.jpg"
	game.AddRecording(thumbMedia)

	// Extract a keyframe at 5 seconds as a demo
	keyframePath, err := extractFrame(videoPath, 5.0)
	require.NoError(t, err)
	defer os.Remove(keyframePath)

	keyframeMedia := NewMedia(MediaKindRecording, keyframePath)
	keyframeMedia.DestinationName = videoDestName + ".keyframe.screenshot.001.jpg"
	game.AddRecording(keyframeMedia)

	// Process the game through the file manager
	err = fileManager.ProcessGame(game)
	require.NoError(t, err)

	// Verify output structure
	gamePath := filepath.Join(outputDir, "PC", "Terraria", "Recordings")
	assert.DirExists(t, gamePath)

	expectedFiles := []string{
		"2025-07-01_09-58-20.mp4",
		"2025-07-01_09-58-20.mp4.thumb.jpg",
		"2025-07-01_09-58-20.mp4.keyframe.screenshot.001.jpg",
	}

	for _, f := range expectedFiles {
		fullPath := filepath.Join(gamePath, f)
		assert.FileExists(t, fullPath, "expected output file: %s", f)

		info, err := os.Stat(fullPath)
		if err == nil {
			t.Logf("Output: %s (%d bytes)", f, info.Size())
		}
	}

	// Clean up metadata cache for copied video
	os.Remove(GetVideoMetadataCachePath(filepath.Join(gamePath, "2025-07-01_09-58-20.mp4")))

	t.Log("Integration test completed successfully!")
}
