package gamesscreenshotmanager

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path/filepath"
)

const DatetimeFormat = "2006-01-02_15-04-05"

type Provider interface {
	Run() error
}

type MediaKind string

const (
	MediaKindScreenshot MediaKind = "screenshot"
	MediaKindClip       MediaKind = "clip"
	MediaKindRecording  MediaKind = "recording"
	MediaKindCover      MediaKind = "cover"
)

// LocalMediaComparison compares the sha1 shases of the source and destination files
func LocalMediaComparison(m MediaFile, destinationPath string) bool {
	// Generate SHA1 hash for source file
	sourceFile, err := os.Open(m.GetSourcePath())
	if err != nil {
		return false
	}
	defer sourceFile.Close()

	sourceHash := sha1.New()
	if _, err := io.Copy(sourceHash, sourceFile); err != nil {
		return false
	}

	// Generate SHA1 hash for destination file
	destFile, err := os.Open(destinationPath)
	if err != nil {
		return false
	}
	defer destFile.Close()

	destHash := sha1.New()
	if _, err := io.Copy(destHash, destFile); err != nil {
		return false
	}

	// Compare the hashes
	return bytes.Equal(sourceHash.Sum(nil), destHash.Sum(nil))
}

type Media struct {
	Kind            MediaKind
	SourceURL       string
	SourcePath      string
	DestinationName string
	ComparisionFunc func(m MediaFile, destinationPath string) bool
}

func NewMedia(kind MediaKind, path string) *Media {
	return &Media{
		Kind:            kind,
		SourcePath:      path,
		ComparisionFunc: LocalMediaComparison,
	}
}

func (m *Media) GetSourcePath() string {
	return m.SourcePath
}

func (m *Media) GetSourceURL() string {
	return m.SourceURL
}

func (m *Media) GetDestinationName() string {
	if m.DestinationName != "" {
		return m.DestinationName
	}
	fileStat, statErr := os.Stat(m.SourcePath)
	if statErr != nil {
		log.Fatal(statErr)
	}
	return fileStat.ModTime().Format(DatetimeFormat) + filepath.Ext(m.SourcePath)
}

func (m *Media) SetDestinationName(destinationName string) {
	m.DestinationName = destinationName
}

func (m *Media) GetKind() MediaKind {
	return m.Kind
}

func (m *Media) IsLocal() bool {
	return m.SourceURL == "" && m.SourcePath != ""
}

func (m *Media) Compare(destinationPath string) bool {
	return m.ComparisionFunc(m, destinationPath)
}

func (m *Media) GetSourceHash() string {
	sourceFile, err := os.Open(m.SourcePath)
	if err != nil {
		return ""
	}
	defer sourceFile.Close()

	sourceHash := sha1.New()
	if _, err := io.Copy(sourceHash, sourceFile); err != nil {
		return ""
	}
	return hex.EncodeToString(sourceHash.Sum(nil))
}

type MediaFile interface {
	GetSourcePath() string
	GetSourceURL() string
	GetSourceHash() string
	GetDestinationName() string
	SetDestinationName(destinationName string)
	GetKind() MediaKind
	IsLocal() bool
	Compare(destinationPath string) bool
}
