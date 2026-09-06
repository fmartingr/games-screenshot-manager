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

// Requirement is one external program a provider needs.
type Requirement struct {
	// Binary is the program to look for, as in "ffprobe".
	Binary string
	// Package is what the user installs to get it. The ffmpeg package ships
	// ffprobe, so the two names differ.
	Package string
	// Reason is why the provider needs the program, in one short phrase.
	Reason string
}

// Requirer is an optional interface. A provider that needs an external program
// implements it, and a provider that needs none does not. The method sits on
// the provider, so a requirement can depend on that provider's own config.
type Requirer interface {
	Requirements() []Requirement
}

// RequirementUse is one provider's need for one external program.
type RequirementUse struct {
	// Provider is the name the registry holds, or "gallery".
	Provider    string
	Requirement Requirement
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
	Subfolder       string
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

func (m *Media) SetSourcePath(sourcePath string) {
	m.SourcePath = sourcePath
}

func (m *Media) GetSourceURL() string {
	return m.SourceURL
}

func (m *Media) SetSourceURL(sourceURL string) {
	m.SourceURL = sourceURL
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

// GetSubfolder returns the folder below the game folder that holds the file. It
// returns an empty string for a file that belongs in the game folder itself.
func (m *Media) GetSubfolder() string {
	return m.Subfolder
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
	SetSourcePath(sourcePath string)
	GetSourceURL() string
	SetSourceURL(sourceURL string)
	GetSourceHash() string
	GetDestinationName() string
	SetDestinationName(destinationName string)
	GetKind() MediaKind
	GetSubfolder() string
	IsLocal() bool
	Compare(destinationPath string) bool
}
