package gamesscreenshotmanager

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

func Ptr[T any](v T) *T {
	return &v
}

// Config represents the root configuration structure
type Config struct {
	OutputPath string        `toml:"output_path"`
	Global     GlobalConfig  `toml:"global"`
	Gallery    GalleryConfig `toml:"gallery"`
	Providers  Providers     `toml:"providers"`

	// Dry run is provided via -dry-run CLI flag
	DryRun bool
}

func (c *Config) Defaults() {
	c.OutputPath = "Output"
	c.Global.Defaults()
	c.Gallery.Defaults()
}

// Providers represents all game provider configurations
type Providers struct {
	GuildWars2      ProviderConfig        `toml:"guild_wars_2"`
	Hytale          ProviderConfig        `toml:"hytale"`
	Minecraft       ProviderConfig        `toml:"minecraft"`
	NintendoSwitch2 NintendoSwitch2Config `toml:"nintendo_switch_2"`
	PlayStation4    ProviderConfig        `toml:"playstation4"`
	PlayStation5    ProviderConfig        `toml:"playstation5"`
	Steam           SteamConfig           `toml:"steam"`
	WorldOfWarcraft ProviderConfig        `toml:"world_of_warcraft"`
	XboxGameBar     ProviderConfig        `toml:"xbox_game_bar"`
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
	Create            bool              `toml:"create"`
	SiteTitle         string            `toml:"site_title"`
	Context           map[string]string `toml:"context"`
	ShowVideoDuration bool              `toml:"show_video_duration"`
}

func (c *GalleryConfig) Defaults() {
	c.Create = false
	c.SiteTitle = "Video Games Screenshots"
	c.ShowVideoDuration = true

	if c.Context == nil {
		c.Context = map[string]string{}
	}
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

func (c *ProviderConfig) ShouldDownloadCovers() bool {
	if c.DownloadCovers != nil {
		return *c.DownloadCovers
	}
	return false
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

// switch2DefaultIgnoredFolders holds the album folder the console makes for
// itself. It carries the captures taken outside a game, as in the Home menu and
// the shop.
//
// The console names it in the language it is set to, so this default matches
// one console only. A user on another language adds their own name.
var switch2DefaultIgnoredFolders = []string{"Otra carpeta"}

// NintendoSwitch2Config represents the Nintendo Switch 2 specific configuration
type NintendoSwitch2Config struct {
	ProviderConfig
	IgnoredFolders []string `toml:"ignored_folders"`
}

// GetIgnoredFolders returns the album folders the provider skips.
//
// A key the config file does not hold takes the default. An empty list is a
// choice, not an absence, so it copies every folder.
func (c *NintendoSwitch2Config) GetIgnoredFolders() []string {
	if c.IgnoredFolders == nil {
		return switch2DefaultIgnoredFolders
	}

	return c.IgnoredFolders
}

// SteamConfig represents the Steam-specific configuration
type SteamConfig struct {
	ProviderConfig
	UserDataPath      string            `toml:"userdata_path"`
	RecordingsPath    string            `toml:"recordings_path"`
	ProcessRecordings bool              `toml:"process_recordings"`
	ProcessClips      bool              `toml:"process_clips"`
	OnlineGallery     bool              `toml:"online_gallery"`
	UserID            string            `toml:"user_id"`
	APIKey            string            `toml:"api_key"`
	CustomGames       map[string]string `toml:"custom_games"`
	IgnoredGames      []string          `toml:"ignored_games"`
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
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if err := toml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error decoding config file: %w", err)
	}

	// Merge the global config with the provider configs
	config.Providers.Steam.Merge(&config.Global)
	config.Providers.Minecraft.Merge(&config.Global)
	config.Providers.PlayStation4.Merge(&config.Global)
	config.Providers.PlayStation5.Merge(&config.Global)
	config.Providers.WorldOfWarcraft.Merge(&config.Global)
	config.Providers.XboxGameBar.Merge(&config.Global)
	config.Providers.GuildWars2.Merge(&config.Global)
	config.Providers.Hytale.Merge(&config.Global)
	config.Providers.NintendoSwitch2.Merge(&config.Global)

	return config, nil
}

// Save saves the configuration back to a TOML file
func (c *Config) Save(path string) error {
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
