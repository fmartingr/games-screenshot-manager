package gamesscreenshotmanager

import (
	"os"

	"git.nakama.town/fmartingr/gotoolkit/encoding"
)

// Config represents the root configuration structure
type Config struct {
	OutputPath string        `toml:"output_path"`
	DryRun     bool          `toml:"dry_run"`
	Global     GlobalConfig  `toml:"global"`
	Gallery    GalleryConfig `toml:"gallery"`
	Providers  Providers     `toml:"providers"`
}

func (c *Config) Defaults() {
	c.OutputPath = "Output"

	c.Global.Defaults()
	c.Gallery.Defaults()
	c.Providers.Defaults()
}

// Providers represents all game provider configurations
type Providers struct {
	GuildWars2      ProviderConfig `toml:"guild_wars_2"`
	Minecraft       ProviderConfig `toml:"minecraft"`
	PlayStation4    ProviderConfig `toml:"playstation4"`
	PlayStation5    ProviderConfig `toml:"playstation5"`
	Steam           SteamConfig    `toml:"steam"`
	WorldOfWarcraft ProviderConfig `toml:"world_of_warcraft"`
	XboxGameBar     ProviderConfig `toml:"xbox_game_bar"`
}

func (c *Providers) Defaults() {
	c.GuildWars2.Defaults()
	c.Minecraft.Defaults()
	c.PlayStation4.Defaults()
	c.PlayStation5.Defaults()
	c.Steam.Defaults()
	c.WorldOfWarcraft.Defaults()
	c.XboxGameBar.Defaults()
}

// GlobalConfig represents the global configuration settings
type GlobalConfig struct {
	DownloadCovers bool `toml:"download_covers"`
}

func (c *GlobalConfig) Defaults() {
	c.DownloadCovers = false
}

// GalleryConfig represents the web gallery configuration
type GalleryConfig struct {
	CreateWebGallery bool `toml:"create_web_gallery"`
}

func (c *GalleryConfig) Defaults() {
	c.CreateWebGallery = false
}

// ProviderConfig represents the base configuration for most games
type ProviderConfig struct {
	GlobalConfig
	Enabled bool   `toml:"enabled,default=true"`
	Path    string `toml:"path"`
}

func (c *ProviderConfig) Defaults() {
	c.Path = "auto"
	c.Enabled = true
}

// SteamConfig represents the Steam-specific configuration
type SteamConfig struct {
	ProviderConfig
	UserDataPath      string            `toml:"userdata_path"`
	RecordingsPath    string            `toml:"recordings_path"`
	ProcessRecordings bool              `toml:"process_recordings"`
	ProcessClips      bool              `toml:"process_clips"`
	CustomGames       map[string]string `toml:"custom_games"`
}

func (c *SteamConfig) Defaults() {
	c.Enabled = true
	c.UserDataPath = "auto"
	c.RecordingsPath = "auto"
}

var encoder = encoding.NewTOMLEncoding()

// NewConfig creates a new Config instance from a TOML file
func NewConfig(path string) (*Config, error) {
	config := &Config{}
	config.Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := encoder.Decode(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// Save saves the configuration back to a TOML file
func (c *Config) Save(path string) error {
	data, err := encoder.Encode(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
