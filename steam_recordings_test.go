package gamesscreenshotmanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGameRecordingPB(t *testing.T) {
	pbPath := filepath.Join("testdata", "GameRecordings", "gamerecording.pb")

	recordings, err := ParseGameRecordingPB(pbPath)
	require.NoError(t, err)
	require.NotEmpty(t, recordings)

	// Verify we parsed the expected number of recordings
	// From the protoc --decode_raw output we know there are 22 recording entries
	assert.Len(t, recordings, 22)

	// Test a specific recording: timeline_10560020250701_095820 (app 105600)
	var found105600 *SteamRecording
	for i := range recordings {
		if recordings[i].TimelineName == "timeline_10560020250701_095820" {
			found105600 = &recordings[i]
			break
		}
	}
	require.NotNil(t, found105600, "expected to find recording for timeline_10560020250701_095820")

	assert.Equal(t, uint64(105600), found105600.AppID)
	assert.Equal(t, int64(1751363900), found105600.DateRecorded)
	assert.Equal(t, uint64(129735), found105600.EndTimeMS)

	// This recording has 3 video segments
	require.Len(t, found105600.VideoSegments, 3)

	// First segment
	assert.Equal(t, "bg_105600_20250701_095836", found105600.VideoSegments[0].FolderName)
	assert.Equal(t, uint64(16079), found105600.VideoSegments[0].StartMS)
	assert.Equal(t, uint64(14960), found105600.VideoSegments[0].DurationMS) // matches MPD PT14.96S
	assert.Equal(t, uint64(5066135), found105600.VideoSegments[0].SizeBytes)

	// Second segment
	assert.Equal(t, "bg_105600_20250701_095851", found105600.VideoSegments[1].FolderName)
	assert.Equal(t, uint64(31107), found105600.VideoSegments[1].StartMS)
	assert.Equal(t, uint64(3293), found105600.VideoSegments[1].DurationMS)

	// Third segment
	assert.Equal(t, "bg_105600_20250701_095854", found105600.VideoSegments[2].FolderName)
	assert.Equal(t, uint64(34399), found105600.VideoSegments[2].StartMS)
	assert.Equal(t, uint64(93034), found105600.VideoSegments[2].DurationMS)

	// Test recording with single segment: timeline_42752020250315_173406 (app 427520)
	var found427520 *SteamRecording
	for i := range recordings {
		if recordings[i].TimelineName == "timeline_42752020250315_173406" {
			found427520 = &recordings[i]
			break
		}
	}
	require.NotNil(t, found427520, "expected to find recording for timeline_42752020250315_173406")

	assert.Equal(t, uint64(427520), found427520.AppID)
	assert.Equal(t, int64(1742060046), found427520.DateRecorded)
	assert.Equal(t, uint64(14062134), found427520.EndTimeMS)
	require.Len(t, found427520.VideoSegments, 1)
	assert.Equal(t, "bg_427520_20250315_173414", found427520.VideoSegments[0].FolderName)

	// Test recording with 2 segments: timeline_42752020250223_175337
	var found427520Multi *SteamRecording
	for i := range recordings {
		if recordings[i].TimelineName == "timeline_42752020250223_175337" {
			found427520Multi = &recordings[i]
			break
		}
	}
	require.NotNil(t, found427520Multi, "expected to find recording for timeline_42752020250223_175337")
	require.Len(t, found427520Multi.VideoSegments, 2)
}

func TestParseTimeline(t *testing.T) {
	t.Run("timeline with entries", func(t *testing.T) {
		tlPath := filepath.Join("testdata", "GameRecordings", "timelines", "timeline_42752020250315_173406.json")

		timeline, err := ParseTimeline(tlPath)
		require.NoError(t, err)

		assert.Equal(t, "1742060046", timeline.DateRecorded)
		assert.Equal(t, "0", timeline.StartTime)
		assert.Equal(t, "14062134", timeline.EndTime)

		require.Len(t, timeline.Entries, 4)

		// First entry: achievement
		assert.Equal(t, "1", timeline.Entries[0].ID)
		assert.Equal(t, "10843881", timeline.Entries[0].Time)
		assert.Equal(t, "achievement", timeline.Entries[0].Type)
		assert.Equal(t, "research-with-logistics", timeline.Entries[0].AchievementName)

		// Second entry: screenshot
		assert.Equal(t, "2", timeline.Entries[1].ID)
		assert.Equal(t, "13325956", timeline.Entries[1].Time)
		assert.Equal(t, "screenshot", timeline.Entries[1].Type)
		assert.Equal(t, "steam_screenshot", timeline.Entries[1].Icon)
		assert.Equal(t, 1000, timeline.Entries[1].Priority)

		// All entries should be key events
		for _, entry := range timeline.Entries {
			assert.True(t, entry.IsKeyEvent(), "entry type %s should be a key event", entry.Type)
		}
	})

	t.Run("timeline without entries", func(t *testing.T) {
		tlPath := filepath.Join("testdata", "GameRecordings", "timelines", "timeline_10560020250701_095820.json")

		timeline, err := ParseTimeline(tlPath)
		require.NoError(t, err)

		assert.Equal(t, "1751363900", timeline.DateRecorded)
		assert.Equal(t, "0", timeline.StartTime)
		assert.Equal(t, "129735", timeline.EndTime)
		assert.Empty(t, timeline.Entries)
	})
}

func TestParseMPD(t *testing.T) {
	mpdPath := filepath.Join("testdata", "GameRecordings", "video", "bg_105600_20250701_095836", "session.mpd")

	mpd, err := parseMPD(mpdPath)
	require.NoError(t, err)

	assert.Equal(t, "PT14.96S", mpd.Duration)
	require.Len(t, mpd.Periods, 1)

	period := mpd.Periods[0]
	require.Len(t, period.AdaptationSets, 2)

	// Video adaptation set
	videoAS := period.AdaptationSets[0]
	assert.Equal(t, "video", videoAS.ContentType)
	require.Len(t, videoAS.Representations, 1)
	assert.Equal(t, "0", videoAS.Representations[0].ID)
	assert.Equal(t, "video/mp4", videoAS.Representations[0].MimeType)

	// Audio adaptation set
	audioAS := period.AdaptationSets[1]
	assert.Equal(t, "audio", audioAS.ContentType)
	require.Len(t, audioAS.Representations, 1)
	assert.Equal(t, "1", audioAS.Representations[0].ID)
	assert.Equal(t, "audio/mp4", audioAS.Representations[0].MimeType)
}

func TestCalculateVideoOffset(t *testing.T) {
	segments := []SteamVideoSegment{
		{FolderName: "seg1", StartMS: 16079, DurationMS: 14960},
		{FolderName: "seg2", StartMS: 31107, DurationMS: 3293},
		{FolderName: "seg3", StartMS: 34399, DurationMS: 93034},
	}

	t.Run("event in first segment", func(t *testing.T) {
		// Event at 20000ms, which is 3921ms into segment 1
		offset := calculateVideoOffset(20000, segments)
		assert.InDelta(t, 3.921, offset, 0.001) // (20000-16079)/1000
	})

	t.Run("event at start of first segment", func(t *testing.T) {
		offset := calculateVideoOffset(16079, segments)
		assert.InDelta(t, 0.0, offset, 0.001)
	})

	t.Run("event in second segment", func(t *testing.T) {
		// Event at 32000ms, which is in segment 2
		// 893ms into segment 2, plus 14960ms (duration of segment 1)
		offset := calculateVideoOffset(32000, segments)
		expected := (14960.0 + 893.0) / 1000.0 // cumulative from seg1 + offset within seg2
		assert.InDelta(t, expected, offset, 0.001)
	})

	t.Run("event in third segment", func(t *testing.T) {
		// Event at 50000ms, which is 15601ms into segment 3
		// Plus cumulative: 14960 + 3293 = 18253ms from prior segments
		offset := calculateVideoOffset(50000, segments)
		expected := (14960.0 + 3293.0 + 15601.0) / 1000.0
		assert.InDelta(t, expected, offset, 0.001)
	})

	t.Run("event before any segment", func(t *testing.T) {
		offset := calculateVideoOffset(1000, segments)
		assert.Equal(t, float64(-1), offset)
	})

	t.Run("event between segments (gap)", func(t *testing.T) {
		// Between segment 1 end (16079+14960=31039) and segment 2 start (31107)
		offset := calculateVideoOffset(31050, segments)
		assert.Equal(t, float64(-1), offset)
	})

	t.Run("event after all segments", func(t *testing.T) {
		offset := calculateVideoOffset(200000, segments)
		assert.Equal(t, float64(-1), offset)
	})

	t.Run("single segment", func(t *testing.T) {
		singleSeg := []SteamVideoSegment{
			{FolderName: "seg1", StartMS: 7723, DurationMS: 14052661},
		}
		// Achievement at 10843881ms
		offset := calculateVideoOffset(10843881, singleSeg)
		expected := float64(10843881-7723) / 1000.0
		assert.InDelta(t, expected, offset, 0.001)
	})
}

func TestConcatenateDASHSegments(t *testing.T) {
	segPath := filepath.Join("testdata", "GameRecordings", "video", "bg_105600_20250701_095836")

	t.Run("video stream", func(t *testing.T) {
		tmpPath, err := concatenateDASHSegments(segPath, "0")
		require.NoError(t, err)
		defer os.Remove(tmpPath)

		// Verify the file exists and has reasonable size
		info, err := os.Stat(tmpPath)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))

		// The concatenated video should be init + 5 chunks
		// init-stream0.m4s (803) + 5 chunk files
		expectedMinSize := int64(803 + 36950 + 1210074 + 1603334 + 1287033 + 795629)
		assert.Equal(t, expectedMinSize, info.Size())
	})

	t.Run("audio stream", func(t *testing.T) {
		tmpPath, err := concatenateDASHSegments(segPath, "1")
		require.NoError(t, err)
		defer os.Remove(tmpPath)

		info, err := os.Stat(tmpPath)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))
	})
}

func TestProcessVideoSegment(t *testing.T) {
	// This test requires ffmpeg
	if _, err := lookupFFmpeg(); err != nil {
		t.Skip("ffmpeg not available")
	}

	segPath := filepath.Join("testdata", "GameRecordings", "video", "bg_105600_20250701_095836")

	outputPath, err := processVideoSegment(segPath)
	require.NoError(t, err)
	defer os.Remove(outputPath)

	// Verify the output is a valid video file
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))

	// Verify we can get the duration
	duration := GetVideoDuration(outputPath)
	// The MPD says PT14.96S
	assert.InDelta(t, 14.96, duration, 0.5)

	// Clean up metadata cache
	os.Remove(GetVideoMetadataCachePath(outputPath))
}

func TestExtractFrame(t *testing.T) {
	if _, err := lookupFFmpeg(); err != nil {
		t.Skip("ffmpeg not available")
	}

	segPath := filepath.Join("testdata", "GameRecordings", "video", "bg_105600_20250701_095836")

	// First create a video to extract from
	videoPath, err := processVideoSegment(segPath)
	require.NoError(t, err)
	defer os.Remove(videoPath)
	defer os.Remove(GetVideoMetadataCachePath(videoPath))

	t.Run("extract first frame", func(t *testing.T) {
		framePath, err := extractFrame(videoPath, 0)
		require.NoError(t, err)
		defer os.Remove(framePath)

		info, err := os.Stat(framePath)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))
	})

	t.Run("extract frame at 5 seconds", func(t *testing.T) {
		framePath, err := extractFrame(videoPath, 5.0)
		require.NoError(t, err)
		defer os.Remove(framePath)

		info, err := os.Stat(framePath)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))
	})
}

func TestTimelineEntryIsKeyEvent(t *testing.T) {
	tests := []struct {
		entryType string
		expected  bool
	}{
		{"achievement", true},
		{"screenshot", true},
		{"marker", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.entryType, func(t *testing.T) {
			entry := SteamTimelineEntry{Type: tt.entryType}
			assert.Equal(t, tt.expected, entry.IsKeyEvent())
		})
	}
}

func TestSteamConfigEnableRecordings(t *testing.T) {
	t.Run("default is false", func(t *testing.T) {
		config := SteamConfig{}
		assert.False(t, config.ShouldProcessRecordings())
	})

	t.Run("enabled", func(t *testing.T) {
		config := SteamConfig{EnableRecordings: true}
		assert.True(t, config.ShouldProcessRecordings())
	})
}

// lookupFFmpeg checks if ffmpeg is available on the system
func lookupFFmpeg() (string, error) {
	return exec.LookPath("ffmpeg")
}

// TestProtobufDecoding tests the low-level protobuf decoding functions
func TestProtobufDecoding(t *testing.T) {
	t.Run("decode varint", func(t *testing.T) {
		// Single byte varint: 1
		val, n := pbDecodeVarint([]byte{0x01})
		assert.Equal(t, uint64(1), val)
		assert.Equal(t, 1, n)

		// Multi-byte varint: 300
		val, n = pbDecodeVarint([]byte{0xAC, 0x02})
		assert.Equal(t, uint64(300), val)
		assert.Equal(t, 2, n)

		// Zero
		val, n = pbDecodeVarint([]byte{0x00})
		assert.Equal(t, uint64(0), val)
		assert.Equal(t, 1, n)

		// Empty data
		val, n = pbDecodeVarint([]byte{})
		assert.Equal(t, uint64(0), val)
		assert.Equal(t, 0, n)
	})

	t.Run("decode tag", func(t *testing.T) {
		// Field 1, wire type 2 (length-delimited) = (1 << 3) | 2 = 0x0A
		fieldNum, wireType, n := pbDecodeTag([]byte{0x0A})
		assert.Equal(t, uint64(1), fieldNum)
		assert.Equal(t, 2, wireType)
		assert.Equal(t, 1, n)

		// Field 2, wire type 0 (varint) = (2 << 3) | 0 = 0x10
		fieldNum, wireType, n = pbDecodeTag([]byte{0x10})
		assert.Equal(t, uint64(2), fieldNum)
		assert.Equal(t, 0, wireType)
		assert.Equal(t, 1, n)

		// Field 5, wire type 2 (length-delimited) = (5 << 3) | 2 = 0x2A
		fieldNum, wireType, n = pbDecodeTag([]byte{0x2A})
		assert.Equal(t, uint64(5), fieldNum)
		assert.Equal(t, 2, wireType)
		assert.Equal(t, 1, n)
	})
}
