package gamesscreenshotmanager

import (
	"os"

	"git.nakama.town/fmartingr/gotoolkit/encoding"
)

var encoder = encoding.NewTOMLEncoding()

func Ptr[T any](v T) *T {
	return &v
}

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

// GlobalConfig represents the global configuration settings
type GlobalConfig struct {
	Enabled        bool `toml:"enabled"`
	DownloadCovers bool `toml:"download_covers"`
}

func (c *GlobalConfig) Defaults() {
	c.DownloadCovers = false
	c.Enabled = true
}

// GalleryConfig represents the web gallery configuration
type GalleryConfig struct {
	CreateWebGallery bool `toml:"create_web_gallery"`
}

func (c *GalleryConfig) Defaults() {
	c.CreateWebGallery = false
}

// ProviderConfig represents the configuration for a specific provider
type ProviderConfig struct {
	Enabled        *bool   `toml:"enabled"`
	DownloadCovers *bool   `toml:"download_covers"`
	Path           *string `toml:"path"`
}

func (c *ProviderConfig) IsEnabled() bool {
	return c.Enabled != nil && *c.Enabled
}

func (c *ProviderConfig) GetPath() string {
	if c.Path != nil {
		return *c.Path
	}
	return "auto"
}

// Merge merges the provider config with the global config and defaults
func (pc *ProviderConfig) Merge(global *GlobalConfig) {
	// Merge Enabled field
	if pc.Enabled == nil {
		pc.Enabled = &global.Enabled
	}

	// Merge DownloadCovers field
	if pc.DownloadCovers == nil {
		pc.DownloadCovers = &global.DownloadCovers
	}
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

func (c *SteamConfig) GetPath() string {
	return c.GetUserDataPath()
}

func (c *SteamConfig) GetUserDataPath() string {
	return c.UserDataPath
}

func (c *SteamConfig) ShouldProcessClips() bool {
	return c.ProcessClips
}

func (c *SteamConfig) ShouldProcessRecordings() bool {
	return c.ProcessRecordings
}

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

	// Merge the global config with the provider configs
	config.Providers.Steam.Merge(&config.Global)
	config.Providers.Minecraft.Merge(&config.Global)
	config.Providers.PlayStation4.Merge(&config.Global)
	config.Providers.PlayStation5.Merge(&config.Global)
	config.Providers.WorldOfWarcraft.Merge(&config.Global)
	config.Providers.XboxGameBar.Merge(&config.Global)
	config.Providers.GuildWars2.Merge(&config.Global)

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
