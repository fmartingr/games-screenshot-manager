package gamesscreenshotmanager

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	toolkitPaths "git.nakama.town/fmartingr/gotoolkit/paths"
)

// SteamRecording represents a single recording session from gamerecording.pb
type SteamRecording struct {
	TimelineName  string
	AppID         uint64
	DateRecorded  int64 // Unix timestamp
	EndTimeMS     uint64
	VideoSegments []SteamVideoSegment
}

// SteamVideoSegment represents a video segment (folder) within a recording
type SteamVideoSegment struct {
	FolderName string
	StartMS    uint64 // Start offset in ms from the timeline start
	DurationMS uint64 // Duration in ms
	SizeBytes  uint64
}

// SteamTimeline represents the parsed timeline JSON
type SteamTimeline struct {
	DateRecorded string               `json:"daterecorded"`
	StartTime    string               `json:"starttime"`
	EndTime      string               `json:"endtime"`
	Entries      []SteamTimelineEntry `json:"entries"`
}

// SteamTimelineEntry represents an event in the timeline
type SteamTimelineEntry struct {
	ID              string `json:"id"`
	Time            string `json:"time"` // in milliseconds from timeline start
	Type            string `json:"type"` // "achievement", "screenshot"
	AchievementName string `json:"achievement_name,omitempty"`
	Icon            string `json:"icon,omitempty"`
	Priority        int    `json:"priority,omitempty"`
	Handle          int    `json:"handle,omitempty"`
}

// IsKeyEvent returns true if the entry represents a key event worth creating a keyframe for
func (e *SteamTimelineEntry) IsKeyEvent() bool {
	return e.Type == "achievement" || e.Type == "screenshot"
}

// DASH MPD parsing structures (minimal, for what we need)
type dashMPD struct {
	XMLName  xml.Name     `xml:"MPD"`
	Duration string       `xml:"mediaPresentationDuration,attr"`
	Periods  []dashPeriod `xml:"Period"`
}

type dashPeriod struct {
	AdaptationSets []dashAdaptationSet `xml:"AdaptationSet"`
}

type dashAdaptationSet struct {
	ContentType     string               `xml:"contentType,attr"`
	Representations []dashRepresentation `xml:"Representation"`
}

type dashRepresentation struct {
	ID              string              `xml:"id,attr"`
	MimeType        string              `xml:"mimeType,attr"`
	SegmentTemplate dashSegmentTemplate `xml:"SegmentTemplate"`
}

type dashSegmentTemplate struct {
	Initialization string `xml:"initialization,attr"`
	Media          string `xml:"media,attr"`
	StartNumber    int    `xml:"startNumber,attr"`
}

// getRecordingsBasePath returns the base path for Steam game recordings
func (p *SteamProvider) getRecordingsBasePath() (string, error) {
	if p.steamConfig.RecordingsPath != "" {
		return toolkitPaths.ExpandUser(p.steamConfig.RecordingsPath), nil
	}

	// Default location inside Steam installation
	var steamPath string
	switch runtime.GOOS {
	case "darwin":
		steamPath = toolkitPaths.ExpandUser("~/Library/Application Support/Steam")
	case "linux":
		steamPath = toolkitPaths.ExpandUser("~/.local/share/Steam")
	case "windows":
		steamPath = "C:\\Program Files (x86)\\Steam"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	return filepath.Join(steamPath, "GameRecordings"), nil
}

// GetRecordings processes Steam game recordings
func (p *SteamProvider) GetRecordings() error {
	basePath, err := p.getRecordingsBasePath()
	if err != nil {
		return fmt.Errorf("error getting recordings base path: %w", err)
	}

	p.log.Info("Processing Steam game recordings", slog.String("path", basePath))

	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		p.log.Warn("Recordings path does not exist", slog.String("path", basePath))
		return nil
	}

	// Parse the gamerecording.pb file
	pbPath := filepath.Join(basePath, "gamerecording.pb")
	recordings, err := ParseGameRecordingPB(pbPath)
	if err != nil {
		return fmt.Errorf("error parsing gamerecording.pb: %w", err)
	}

	p.log.Info("Found recordings", slog.Int("count", len(recordings)))

	for _, recording := range recordings {
		if err := p.processRecording(basePath, &recording); err != nil {
			p.log.Error("Failed to process recording",
				slog.String("timeline", recording.TimelineName),
				slog.Uint64("app_id", recording.AppID),
				slog.Any("error", err))
			continue
		}
	}

	return nil
}

// processRecording processes a single recording session
func (p *SteamProvider) processRecording(basePath string, recording *SteamRecording) error {
	appIDStr := strconv.FormatUint(recording.AppID, 10)
	gameName := p.client.GetGameName(appIDStr)

	if gameName == "" && p.steamConfig.CustomGames[appIDStr] != "" {
		gameName = p.steamConfig.CustomGames[appIDStr]
	}

	if gameName == "" {
		p.log.Warn("No game name found for recording", slog.String("app_id", appIDStr))
		gameName = appIDStr
	}

	p.log.Info("Processing recording",
		slog.String("game", gameName),
		slog.String("timeline", recording.TimelineName),
		slog.Int("video_segments", len(recording.VideoSegments)))

	// Verify video segment folders exist
	var validSegments []SteamVideoSegment
	for _, seg := range recording.VideoSegments {
		segPath := filepath.Join(basePath, "video", seg.FolderName)
		if _, err := os.Stat(segPath); os.IsNotExist(err) {
			p.log.Warn("Video segment folder not found, skipping",
				slog.String("folder", seg.FolderName))
			continue
		}

		// Verify the folder has a session.mpd file
		mpdPath := filepath.Join(segPath, "session.mpd")
		if _, err := os.Stat(mpdPath); os.IsNotExist(err) {
			p.log.Warn("Video segment folder has no session.mpd, skipping",
				slog.String("folder", seg.FolderName))
			continue
		}

		validSegments = append(validSegments, seg)
	}

	if len(validSegments) == 0 {
		p.log.Warn("No valid video segments for recording", slog.String("timeline", recording.TimelineName))
		return nil
	}

	// Sort segments by start time
	sort.Slice(validSegments, func(i, j int) bool {
		return validSegments[i].StartMS < validSegments[j].StartMS
	})

	// Process each video segment folder into a temp MP4, then stitch them all
	// into a single video. Steam splits long recordings into multiple segment
	// folders (bg_*) that together form one continuous recording session.
	var segmentFiles []string
	for i, seg := range validSegments {
		p.log.Debug("Processing video segment",
			slog.Int("segment", i+1),
			slog.Int("total_segments", len(validSegments)),
			slog.String("folder", seg.FolderName),
			slog.Uint64("start_ms", seg.StartMS),
			slog.Uint64("duration_ms", seg.DurationMS))

		segPath := filepath.Join(basePath, "video", seg.FolderName)
		tempFile, err := processVideoSegment(segPath)
		if err != nil {
			p.log.Error("Failed to process video segment",
				slog.String("folder", seg.FolderName),
				slog.Any("error", err))
			// Clean up any temp files created so far
			for _, f := range segmentFiles {
				os.Remove(f)
			}
			return fmt.Errorf("error processing video segment %s: %w", seg.FolderName, err)
		}
		segmentFiles = append(segmentFiles, tempFile)
	}

	// Stitch all segment files into one final video
	var finalVideoPath string
	if len(segmentFiles) == 1 {
		finalVideoPath = segmentFiles[0]
	} else {
		p.log.Info("Stitching video segments into single recording",
			slog.Int("segments", len(segmentFiles)))

		combined, err := concatenateVideos(segmentFiles)
		if err != nil {
			for _, f := range segmentFiles {
				os.Remove(f)
			}
			return fmt.Errorf("error stitching video segments: %w", err)
		}
		// Clean up individual segment files
		for _, f := range segmentFiles {
			os.Remove(f)
		}
		finalVideoPath = combined
	}

	// Register cleanup for the final video
	p.fileManager.cleanup = append(p.fileManager.cleanup, func() {
		os.Remove(finalVideoPath)
	})

	// Create the game and recording media
	game := p.gameManager.GetGame(appIDStr)
	if game == nil {
		game = NewGame(appIDStr, gameName, "PC", "steam")
		p.gameManager.AddGame(game)
	}

	// Generate recording filename based on the date recorded
	recordingTime := time.Unix(recording.DateRecorded, 0)
	recordingBaseName := recordingTime.Format(DatetimeFormat)

	// Video destination name: <datetime>.mp4
	videoDestName := recordingBaseName + ".mp4"

	// Add the main recording video
	videoMedia := NewMedia(MediaKindRecording, finalVideoPath)
	videoMedia.DestinationName = videoDestName
	game.AddRecording(videoMedia)

	// Generate thumbnail: <video>.thumb.jpg (ignored by gallery)
	thumbnailPath, err := extractFrame(finalVideoPath, 0)
	if err != nil {
		p.log.Error("Failed to generate thumbnail", slog.Any("error", err))
	} else {
		p.fileManager.cleanup = append(p.fileManager.cleanup, func() {
			os.Remove(thumbnailPath)
		})
		thumbMedia := NewMedia(MediaKindRecording, thumbnailPath)
		thumbMedia.DestinationName = videoDestName + ".thumb.jpg"
		game.AddRecording(thumbMedia)
	}

	// Parse timeline and generate keyframes for key events
	timelinePath := filepath.Join(basePath, "timelines", recording.TimelineName+".json")
	timeline, err := ParseTimeline(timelinePath)
	if err != nil {
		p.log.Warn("Could not parse timeline, skipping keyframes",
			slog.String("timeline", recording.TimelineName),
			slog.Any("error", err))
		return nil
	}

	keyEventIndex := 0
	for _, entry := range timeline.Entries {
		if !entry.IsKeyEvent() {
			continue
		}

		entryTimeMS, err := strconv.ParseUint(entry.Time, 10, 64)
		if err != nil {
			p.log.Warn("Could not parse entry time", slog.String("time", entry.Time), slog.Any("error", err))
			continue
		}

		// Calculate the position within the combined video
		videoOffsetSeconds := calculateVideoOffset(entryTimeMS, validSegments)
		if videoOffsetSeconds < 0 {
			p.log.Warn("Event time outside video segments",
				slog.String("type", entry.Type),
				slog.Uint64("time_ms", entryTimeMS))
			continue
		}

		keyframePath, err := extractFrame(finalVideoPath, videoOffsetSeconds)
		if err != nil {
			p.log.Error("Failed to extract keyframe",
				slog.String("type", entry.Type),
				slog.Float64("offset_seconds", videoOffsetSeconds),
				slog.Any("error", err))
			continue
		}

		p.fileManager.cleanup = append(p.fileManager.cleanup, func() {
			os.Remove(keyframePath)
		})

		// Build keyframe filename: <video>.keyframe.<type>.<event>.jpg (ignored by gallery)
		keyEventIndex++
		var eventName string
		switch entry.Type {
		case "achievement":
			eventName = entry.AchievementName
			if eventName == "" {
				eventName = fmt.Sprintf("%03d", keyEventIndex)
			}
		case "screenshot":
			eventName = fmt.Sprintf("%03d", keyEventIndex)
		default:
			eventName = fmt.Sprintf("%03d", keyEventIndex)
		}

		keyframeName := fmt.Sprintf("%s.keyframe.%s.%s.jpg", videoDestName, entry.Type, eventName)

		keyframeMedia := NewMedia(MediaKindRecording, keyframePath)
		keyframeMedia.DestinationName = keyframeName
		game.AddRecording(keyframeMedia)

		p.log.Info("Created keyframe",
			slog.String("type", entry.Type),
			slog.String("name", keyframeName),
			slog.Float64("offset_seconds", videoOffsetSeconds))
	}

	return nil
}

// processVideoSegment combines the DASH m4s segments from a folder into a single MP4 file
func processVideoSegment(segmentPath string) (string, error) {
	// Parse the session.mpd to understand the stream layout
	mpdPath := filepath.Join(segmentPath, "session.mpd")
	mpd, err := parseMPD(mpdPath)
	if err != nil {
		return "", fmt.Errorf("error parsing MPD: %w", err)
	}

	// Find video and audio representations
	var videoRep, audioRep *dashRepresentation
	for _, period := range mpd.Periods {
		for _, as := range period.AdaptationSets {
			for i := range as.Representations {
				rep := &as.Representations[i]
				switch as.ContentType {
				case "video":
					videoRep = rep
				case "audio":
					audioRep = rep
				}
			}
		}
	}

	if videoRep == nil {
		return "", fmt.Errorf("no video representation found in MPD")
	}

	// Concatenate video stream segments
	videoTmp, err := concatenateDASHSegments(segmentPath, videoRep.ID)
	if err != nil {
		return "", fmt.Errorf("error concatenating video segments: %w", err)
	}
	defer os.Remove(videoTmp)

	// If no audio, just remux the video
	if audioRep == nil {
		outputFile, err := os.CreateTemp("", "steam_recording_*.mp4")
		if err != nil {
			return "", fmt.Errorf("error creating temp file: %w", err)
		}
		outputFile.Close()

		cmd := exec.Command("ffmpeg", "-y",
			"-i", videoTmp,
			"-c", "copy",
			outputFile.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			os.Remove(outputFile.Name())
			return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
		}

		return outputFile.Name(), nil
	}

	// Concatenate audio stream segments
	audioTmp, err := concatenateDASHSegments(segmentPath, audioRep.ID)
	if err != nil {
		return "", fmt.Errorf("error concatenating audio segments: %w", err)
	}
	defer os.Remove(audioTmp)

	// Mux video and audio together
	outputFile, err := os.CreateTemp("", "steam_recording_*.mp4")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}
	outputFile.Close()

	cmd := exec.Command("ffmpeg", "-y",
		"-i", videoTmp,
		"-i", audioTmp,
		"-c", "copy",
		outputFile.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		os.Remove(outputFile.Name())
		return "", fmt.Errorf("ffmpeg mux error: %w, output: %s", err, string(output))
	}

	return outputFile.Name(), nil
}

// concatenateDASHSegments binary-concatenates the init segment + all chunk segments for a stream
func concatenateDASHSegments(segmentPath string, streamID string) (string, error) {
	tmpFile, err := os.CreateTemp("", "dash_stream_*.mp4")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}

	// Write init segment
	initPath := filepath.Join(segmentPath, fmt.Sprintf("init-stream%s.m4s", streamID))
	if err := appendFileToWriter(tmpFile, initPath); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("error appending init segment: %w", err)
	}

	// Find and sort chunk segments
	pattern := filepath.Join(segmentPath, fmt.Sprintf("chunk-stream%s-*.m4s", streamID))
	chunks, err := filepath.Glob(pattern)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("error finding chunk segments: %w", err)
	}

	sort.Strings(chunks)

	for _, chunk := range chunks {
		if err := appendFileToWriter(tmpFile, chunk); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return "", fmt.Errorf("error appending chunk %s: %w", chunk, err)
		}
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("error closing temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// appendFileToWriter appends the contents of a file to a writer
func appendFileToWriter(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	return err
}

// concatenateVideos uses ffmpeg concat demuxer to join multiple video files
func concatenateVideos(videoFiles []string) (string, error) {
	// Create a concat list file
	listFile, err := os.CreateTemp("", "concat_list_*.txt")
	if err != nil {
		return "", fmt.Errorf("error creating concat list: %w", err)
	}
	defer os.Remove(listFile.Name())

	for _, vf := range videoFiles {
		// ffmpeg concat demuxer requires escaped paths
		escaped := strings.ReplaceAll(vf, "'", "'\\''")
		fmt.Fprintf(listFile, "file '%s'\n", escaped)
	}
	listFile.Close()

	outputFile, err := os.CreateTemp("", "steam_recording_combined_*.mp4")
	if err != nil {
		return "", fmt.Errorf("error creating output file: %w", err)
	}
	outputFile.Close()

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
		"-c", "copy",
		outputFile.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		os.Remove(outputFile.Name())
		return "", fmt.Errorf("ffmpeg concat error: %w, output: %s", err, string(output))
	}

	return outputFile.Name(), nil
}

// extractFrame extracts a single frame from a video at the given time offset (in seconds)
func extractFrame(videoPath string, offsetSeconds float64) (string, error) {
	outputFile, err := os.CreateTemp("", "keyframe_*.jpg")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}
	outputFile.Close()

	offsetStr := fmt.Sprintf("%.3f", offsetSeconds)

	cmd := exec.Command("ffmpeg", "-y",
		"-ss", offsetStr,
		"-i", videoPath,
		"-vframes", "1",
		"-update", "1",
		"-q:v", "2",
		outputFile.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		os.Remove(outputFile.Name())
		return "", fmt.Errorf("ffmpeg frame extraction error at %ss: %w, output: %s", offsetStr, err, string(output))
	}

	return outputFile.Name(), nil
}

// calculateVideoOffset converts a timeline event time (ms) to a position within the combined video (seconds)
// The combined video is formed by concatenating all video segments in order of their StartMS.
// Returns -1 if the event time doesn't fall within any video segment.
func calculateVideoOffset(eventTimeMS uint64, segments []SteamVideoSegment) float64 {
	// Calculate cumulative duration of prior segments to get the position in the combined video
	var cumulativeDurationMS uint64

	for _, seg := range segments {
		segEnd := seg.StartMS + seg.DurationMS
		if eventTimeMS >= seg.StartMS && eventTimeMS <= segEnd {
			// Event falls within this segment
			offsetWithinSegment := eventTimeMS - seg.StartMS
			totalOffsetMS := cumulativeDurationMS + offsetWithinSegment
			return float64(totalOffsetMS) / 1000.0
		}
		cumulativeDurationMS += seg.DurationMS
	}

	return -1
}

// parseMPD parses a DASH MPD file
func parseMPD(path string) (*dashMPD, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading MPD file: %w", err)
	}

	var mpd dashMPD
	if err := xml.Unmarshal(data, &mpd); err != nil {
		return nil, fmt.Errorf("error parsing MPD XML: %w", err)
	}

	return &mpd, nil
}

// ParseTimeline parses a Steam timeline JSON file
func ParseTimeline(path string) (*SteamTimeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading timeline file: %w", err)
	}

	var timeline SteamTimeline
	if err := json.Unmarshal(data, &timeline); err != nil {
		return nil, fmt.Errorf("error parsing timeline JSON: %w", err)
	}

	return &timeline, nil
}

// ParseGameRecordingPB parses the gamerecording.pb protobuf file.
//
// The protobuf structure (as observed via protoc --decode_raw):
//
//	Field 1 (repeated message) - Recordings:
//	  Field 1 (string): timeline name
//	  Field 2 (varint): app ID
//	  Field 3 (varint): Unix timestamp of recording
//	  Field 4 (varint): end time in ms (from timeline start)
//	  Field 5 (repeated message): video segments
//	    Field 1 (string): video folder name
//	    Field 2 (varint): start offset in ms from timeline start
//	    Field 3 (varint): duration in ms
//	    Field 4 (varint): encoding type
//	    Field 9 (varint): file size in bytes
//	    Field 10 (varint): start offset (same as field 2)
//
//	Field 2 (repeated message) - Clips/Bookmarks (skipped)
func ParseGameRecordingPB(path string) ([]SteamRecording, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading protobuf file: %w", err)
	}

	var recordings []SteamRecording

	pos := 0
	for pos < len(data) {
		fieldNum, wireType, n := pbDecodeTag(data[pos:])
		if n == 0 {
			break
		}
		pos += n

		switch wireType {
		case 0: // Varint
			_, n := pbDecodeVarint(data[pos:])
			if n == 0 {
				return nil, fmt.Errorf("error decoding varint at position %d", pos)
			}
			pos += n

		case 2: // Length-delimited
			length, n := pbDecodeVarint(data[pos:])
			if n == 0 {
				return nil, fmt.Errorf("error decoding length at position %d", pos)
			}
			pos += n

			if int(length) > len(data)-pos {
				return nil, fmt.Errorf("length %d exceeds remaining data at position %d", length, pos)
			}

			fieldData := data[pos : pos+int(length)]

			if fieldNum == 1 {
				recording, err := pbParseRecording(fieldData)
				if err != nil {
					return nil, fmt.Errorf("error parsing recording at position %d: %w", pos, err)
				}
				recordings = append(recordings, recording)
			}
			// Skip field 2 (clips/bookmarks) and any other top-level fields

			pos += int(length)

		case 5: // 32-bit fixed
			pos += 4

		case 1: // 64-bit fixed
			pos += 8

		default:
			return nil, fmt.Errorf("unexpected wire type %d at position %d", wireType, pos)
		}
	}

	return recordings, nil
}

// pbParseRecording parses a single recording message from raw protobuf bytes
func pbParseRecording(data []byte) (SteamRecording, error) {
	var rec SteamRecording
	pos := 0

	for pos < len(data) {
		fieldNum, wireType, n := pbDecodeTag(data[pos:])
		if n == 0 {
			break
		}
		pos += n

		switch wireType {
		case 0: // Varint
			val, n := pbDecodeVarint(data[pos:])
			if n == 0 {
				return rec, fmt.Errorf("error decoding varint at position %d", pos)
			}
			pos += n

			switch fieldNum {
			case 2:
				rec.AppID = val
			case 3:
				rec.DateRecorded = int64(val)
			case 4:
				rec.EndTimeMS = val
			}

		case 2: // Length-delimited
			length, n := pbDecodeVarint(data[pos:])
			if n == 0 {
				return rec, fmt.Errorf("error decoding length at position %d", pos)
			}
			pos += n

			if int(length) > len(data)-pos {
				return rec, fmt.Errorf("length %d exceeds remaining data at position %d", length, pos)
			}

			fieldData := data[pos : pos+int(length)]
			pos += int(length)

			switch fieldNum {
			case 1: // Timeline name
				rec.TimelineName = string(fieldData)
			case 5: // Video segment
				seg, err := pbParseVideoSegment(fieldData)
				if err != nil {
					return rec, fmt.Errorf("error parsing video segment: %w", err)
				}
				rec.VideoSegments = append(rec.VideoSegments, seg)
			}

		case 5: // 32-bit fixed
			pos += 4

		case 1: // 64-bit fixed
			pos += 8

		default:
			return rec, fmt.Errorf("unexpected wire type %d for field %d at position %d", wireType, fieldNum, pos)
		}
	}

	return rec, nil
}

// pbParseVideoSegment parses a video segment message from raw protobuf bytes
func pbParseVideoSegment(data []byte) (SteamVideoSegment, error) {
	var seg SteamVideoSegment
	pos := 0

	for pos < len(data) {
		fieldNum, wireType, n := pbDecodeTag(data[pos:])
		if n == 0 {
			break
		}
		pos += n

		switch wireType {
		case 0: // Varint
			val, n := pbDecodeVarint(data[pos:])
			if n == 0 {
				return seg, fmt.Errorf("error decoding varint at position %d", pos)
			}
			pos += n

			switch fieldNum {
			case 2:
				seg.StartMS = val
			case 3:
				seg.DurationMS = val
			case 9:
				seg.SizeBytes = val
			}

		case 2: // Length-delimited
			length, n := pbDecodeVarint(data[pos:])
			if n == 0 {
				return seg, fmt.Errorf("error decoding length at position %d", pos)
			}
			pos += n

			if int(length) > len(data)-pos {
				return seg, fmt.Errorf("length %d exceeds remaining data at position %d", length, pos)
			}

			fieldData := data[pos : pos+int(length)]
			pos += int(length)

			if fieldNum == 1 {
				seg.FolderName = string(fieldData)
			}

		case 5: // 32-bit fixed
			pos += 4

		case 1: // 64-bit fixed
			pos += 8

		default:
			return seg, fmt.Errorf("unexpected wire type %d for field %d at position %d", wireType, fieldNum, pos)
		}
	}

	return seg, nil
}

// pbDecodeTag decodes a protobuf field tag (field number + wire type)
// Returns (fieldNumber, wireType, bytesConsumed)
func pbDecodeTag(data []byte) (uint64, int, int) {
	val, n := pbDecodeVarint(data)
	if n == 0 {
		return 0, 0, 0
	}
	fieldNum := val >> 3
	wireType := int(val & 0x7)
	return fieldNum, wireType, n
}

// pbDecodeVarint decodes a protobuf varint
// Returns (value, bytesConsumed)
func pbDecodeVarint(data []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, b := range data {
		if i >= 10 { // Varint can't be more than 10 bytes
			return 0, 0
		}
		if b < 0x80 {
			return x | uint64(b)<<s, i + 1
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, 0
}
