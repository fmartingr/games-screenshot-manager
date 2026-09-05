package gamesscreenshotmanager

import (
	"testing"
)

func TestGalleryNode_GetWebPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		title    string
		expected string
	}{
		{
			name:     "Root folder",
			path:     ".",
			title:    "",
			expected: "/",
		},
		{
			name:     "Simple path",
			path:     "./Steam",
			title:    "Steam",
			expected: "/Steam/",
		},
		{
			name:     "Simple path without leading dot",
			path:     "Steam",
			title:    "Steam",
			expected: "/Steam/",
		},
		{
			name:     "Nested path",
			path:     "./Steam/Game",
			title:    "Game",
			expected: "/Steam/Game/",
		},
		{
			name:     "Path with accented characters",
			path:     "./Steam/Pokémon",
			title:    "Pokémon",
			expected: "/Steam/Pok%C3%A9mon/",
		},
		{
			name:     "Path with spaces",
			path:     "./Steam/Game Name",
			title:    "Game Name",
			expected: "/Steam/Game%20Name/",
		},
		{
			name:     "Path with question mark",
			path:     "./Steam/What?",
			title:    "What?",
			expected: "/Steam/What%3F/",
		},
		{
			name:     "Path with hash symbol",
			path:     "./Steam/Game#1",
			title:    "Game#1",
			expected: "/Steam/Game%231/",
		},
		{
			name:     "Path with ampersand",
			path:     "./Steam/Dungeons & Dragons",
			title:    "Dungeons & Dragons",
			expected: "/Steam/Dungeons%20&%20Dragons/",
		},
		{
			name:     "Path with multiple special characters",
			path:     "./Steam/Game #1 & More?",
			title:    "Game #1 & More?",
			expected: "/Steam/Game%20%231%20&%20More%3F/",
		},
		{
			name:     "File with extension",
			path:     "./Steam/Pokémon/screenshot.jpg",
			title:    "screenshot.jpg",
			expected: "/Steam/Pok%C3%A9mon/screenshot.jpg",
		},
		{
			name:     "File with special characters in filename",
			path:     "./Steam/What is this?.jpg",
			title:    "What is this?.jpg",
			expected: "/Steam/What%20is%20this%3F.jpg",
		},
		{
			name:     "Path with multiple accented characters",
			path:     "./Steam/Café España",
			title:    "Café España",
			expected: "/Steam/Caf%C3%A9%20Espa%C3%B1a/",
		},
		{
			name:     "Path with plus sign",
			path:     "./Steam/Game+Plus",
			title:    "Game+Plus",
			expected: "/Steam/Game+Plus/",
		},
		{
			name:     "Path with equals sign",
			path:     "./Steam/Value=Something",
			title:    "Value=Something",
			expected: "/Steam/Value=Something/",
		},
		{
			name:     "Path with percent sign",
			path:     "./Steam/100% Complete",
			title:    "100% Complete",
			expected: "/Steam/100%25%20Complete/",
		},
		{
			name:     "Path with brackets",
			path:     "./Steam/Game [Edition]",
			title:    "Game [Edition]",
			expected: "/Steam/Game%20%5BEdition%5D/",
		},
		{
			name:     "Path with parentheses",
			path:     "./Steam/Game (2024)",
			title:    "Game (2024)",
			expected: "/Steam/Game%20%282024%29/",
		},
		{
			name:     "Path with single quote",
			path:     "./Steam/Dragon's Quest",
			title:    "Dragon's Quest",
			expected: "/Steam/Dragon%27s%20Quest/",
		},
		{
			name:     "Path with double quotes",
			path:     `./Steam/Game "Special"`,
			title:    `Game "Special"`,
			expected: "/Steam/Game%20%22Special%22/",
		},
		{
			name:     "Path with comma",
			path:     "./Steam/Game, The",
			title:    "Game, The",
			expected: "/Steam/Game%2C%20The/",
		},
		{
			name:     "Path with semicolon",
			path:     "./Steam/Game;Edition",
			title:    "Game;Edition",
			expected: "/Steam/Game%3BEdition/",
		},
		{
			name:     "Path with at symbol",
			path:     "./Steam/Game@Home",
			title:    "Game@Home",
			expected: "/Steam/Game@Home/",
		},
		{
			name:     "Deep nested path with special characters",
			path:     "./Steam/Pokémon/Generation #1/Pikachu?/screenshot.png",
			title:    "screenshot.png",
			expected: "/Steam/Pok%C3%A9mon/Generation%20%231/Pikachu%3F/screenshot.png",
		},
		{
			name:     "Path with German umlauts",
			path:     "./Steam/Über Ärger",
			title:    "Über Ärger",
			expected: "/Steam/%C3%9Cber%20%C3%84rger/",
		},
		{
			name:     "Path with Japanese characters",
			path:     "./Steam/ポケモン",
			title:    "ポケモン",
			expected: "/Steam/%E3%83%9D%E3%82%B1%E3%83%A2%E3%83%B3/",
		},
		{
			name:     "Path with emoji",
			path:     "./Steam/Game 🎮",
			title:    "Game 🎮",
			expected: "/Steam/Game%20%F0%9F%8E%AE/",
		},
		{
			// The gallery walk renames to NFC and puts the name it found on
			// disk in Path. A name that stays NFD must keep its own bytes in
			// the link, or the link stops matching the file.
			name:     "NFD path is escaped as it is stored",
			path:     "./Game Boy Advance/Poke\u0301mon Rojo Fuego",
			title:    "Poke\u0301mon Rojo Fuego",
			expected: "/Game%20Boy%20Advance/Poke%CC%81mon%20Rojo%20Fuego/",
		},
		{
			name:     "NFC path is escaped as it is stored",
			path:     "./Game Boy Advance/Pok\u00e9mon Rojo Fuego",
			title:    "Pok\u00e9mon Rojo Fuego",
			expected: "/Game%20Boy%20Advance/Pok%C3%A9mon%20Rojo%20Fuego/",
		},
		{
			name:     "NFD filename is escaped as it is stored",
			path:     "./Steam/Poke\u0301mon/captura e\u0301pica.jpg",
			title:    "captura e\u0301pica.jpg",
			expected: "/Steam/Poke%CC%81mon/captura%20e%CC%81pica.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &GalleryNode{
				Path:  tt.path,
				Title: tt.title,
			}
			result := node.GetWebPath()
			if result != tt.expected {
				t.Errorf("GetWebPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGalleryNode_GetWebCoverPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		title    string
		cover    string
		expected string
	}{
		{
			name:     "Empty cover",
			path:     "./Steam",
			title:    "Steam",
			cover:    "",
			expected: "",
		},
		{
			name:     "Simple cover filename",
			path:     "./Steam",
			title:    "Steam",
			cover:    "cover.jpg",
			expected: "/Steam/cover.jpg",
		},
		{
			name:     "Cover with special characters",
			path:     "./Steam/Pokémon",
			title:    "Pokémon",
			cover:    "screenshot 1.jpg",
			expected: "/Steam/Pok%C3%A9mon/screenshot%201.jpg",
		},
		{
			name:     "Cover with question mark",
			path:     "./Steam",
			title:    "Steam",
			cover:    "what?.jpg",
			expected: "/Steam/what%3F.jpg",
		},
		{
			name:     "Cover with hash symbol",
			path:     "./Steam",
			title:    "Steam",
			cover:    "image#1.jpg",
			expected: "/Steam/image%231.jpg",
		},
		{
			name:     "Cover with ampersand",
			path:     "./Steam",
			title:    "Steam",
			cover:    "A & B.jpg",
			expected: "/Steam/A%20&%20B.jpg",
		},
		{
			name:     "Cover with accented characters",
			path:     "./Steam",
			title:    "Steam",
			cover:    "Pokémon.jpg",
			expected: "/Steam/Pok%C3%A9mon.jpg",
		},
		{
			name:     "Path and cover both with special characters",
			path:     "./Steam/Game #1",
			title:    "Game #1",
			cover:    "Screenshot?.jpg",
			expected: "/Steam/Game%20%231/Screenshot%3F.jpg",
		},
		{
			name:     "Cover with brackets",
			path:     "./Steam",
			title:    "Steam",
			cover:    "image[1].jpg",
			expected: "/Steam/image%5B1%5D.jpg",
		},
		{
			name:     "Cover with parentheses",
			path:     "./Steam",
			title:    "Steam",
			cover:    "image(1).jpg",
			expected: "/Steam/image%281%29.jpg",
		},
		{
			name:     "NFD path and NFD cover are escaped as they are stored",
			path:     "./Steam/Poke\u0301mon",
			title:    "Poke\u0301mon",
			cover:    "portada e\u0301pica.jpg",
			expected: "/Steam/Poke%CC%81mon/portada%20e%CC%81pica.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &GalleryNode{
				Path:  tt.path,
				Title: tt.title,
				Cover: tt.cover,
			}
			result := node.GetWebCoverPath()
			if result != tt.expected {
				t.Errorf("GetWebCoverPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGalleryNode_GetThumbPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		title    string
		expected string
	}{
		{
			name:     "Simple path",
			path:     "./Steam",
			title:    "Steam",
			expected: "/Steam/.thumb.jpg",
		},
		{
			name:     "Path with special characters",
			path:     "./Steam/Pokémon",
			title:    "Pokémon",
			expected: "/Steam/Pok%C3%A9mon/.thumb.jpg",
		},
		{
			name:     "Path with spaces",
			path:     "./Steam/Game Name",
			title:    "Game Name",
			expected: "/Steam/Game%20Name/.thumb.jpg",
		},
		{
			name:     "File path",
			path:     "./Steam/Pokémon/screenshot.jpg",
			title:    "screenshot.jpg",
			expected: "/Steam/Pok%C3%A9mon/screenshot.jpg.thumb.jpg",
		},
		{
			name:     "Path with question mark",
			path:     "./Steam/What?",
			title:    "What?",
			expected: "/Steam/What%3F/.thumb.jpg",
		},
		{
			name:     "Path with multiple special characters",
			path:     "./Steam/Game #1 & More",
			title:    "Game #1 & More",
			expected: "/Steam/Game%20%231%20&%20More/.thumb.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &GalleryNode{
				Path:  tt.path,
				Title: tt.title,
			}
			result := node.GetThumbPath()
			if result != tt.expected {
				t.Errorf("GetThumbPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}
