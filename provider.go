package gamesscreenshotmanager

import (
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

type Media struct {
	Kind            MediaKind
	Path            string
	DestinationName string
}

func NewMedia(kind MediaKind, path string) *Media {
	return &Media{
		Kind: kind,
		Path: path,
	}
}

func (m *Media) GetPath() string {
	return m.Path
}

func (m *Media) GetDestinationName() string {
	if m.DestinationName != "" {
		return m.DestinationName
	}
	fileStat, statErr := os.Stat(m.Path)
	if statErr != nil {
		log.Fatal(statErr)
	}
	return fileStat.ModTime().Format(DatetimeFormat) + filepath.Ext(m.Path)
}

func (m *Media) GetKind() MediaKind {
	return m.Kind
}

type MediaFile interface {
	GetPath() string
	GetDestinationName() string
	GetKind() MediaKind
}
