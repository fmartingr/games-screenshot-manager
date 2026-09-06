package gamesscreenshotmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
)

func TestConfigLoad(t *testing.T) {
	// Create a temporary test config file
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "test_config.toml")

	testConfig := `
output_path = "test_output"
[global]
download_covers = true

[gallery]
create = true

[providers.guild_wars_2]
path = "test_gw2_path"

[providers.minecraft]
path = "test_minecraft_path"

[providers.steam]
userdata_path = "test_steam_userdata"
recordings_path = "test_steam_recordings"
custom_games = { "1234567890" = "Test Game" }
ignored_games = ["4148250", "4597250"]
`

	err := os.WriteFile(testConfigPath, []byte(testConfig), 0644)
	require.NoError(t, err)

	// Test loading the config
	config, err := NewConfig(testConfigPath)
	require.NoError(t, err)

	// Test global settings
	require.Equal(t, "test_output", config.OutputPath)
	require.True(t, config.Global.DownloadCovers)
	require.False(t, config.DryRun)

	// Test gallery settings
	require.True(t, config.Gallery.Create)

	// Test provider settings
	require.Equal(t, "test_gw2_path", config.Providers.GuildWars2.GetPath())
	require.Equal(t, "test_minecraft_path", config.Providers.Minecraft.GetPath())
	require.Equal(t, "test_steam_userdata", config.Providers.Steam.GetPath())
	require.Equal(t, "test_steam_recordings", config.Providers.Steam.RecordingsPath)
	require.True(t, config.Providers.Steam.IsEnabled(), "Steam provider should be enabled by default")
	require.Len(t, config.Providers.Steam.CustomGames, 1)
	require.Equal(t, "Test Game", config.Providers.Steam.CustomGames["1234567890"])
	require.Equal(t, []string{"4148250", "4597250"}, config.Providers.Steam.IgnoredGames)
}

func TestConfigSave(t *testing.T) {
	// Create a test config
	config := &Config{
		OutputPath: "test_output",
		Gallery: GalleryConfig{
			Create: true,
		},
		Providers: Providers{
			GuildWars2: ProviderConfig{
				Path: &[]string{"test_gw2_path"}[0],
			},
			Steam: SteamConfig{
				UserDataPath:   "test_steam_userdata",
				RecordingsPath: "test_steam_recordings",
				CustomGames: map[string]string{
					"1234567890": "Test Game",
				},
			},
		},
	}

	// Save to temporary file
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "test_config.toml")

	err := config.Save(testConfigPath)
	require.NoError(t, err)

	// Read back the saved file
	savedData, err := os.ReadFile(testConfigPath)
	require.NoError(t, err)

	// Create a new config and load the saved data
	newConfig := &Config{}
	err = toml.Unmarshal(savedData, newConfig)
	require.NoError(t, err)

	// Compare the configs
	require.Equal(t, config.OutputPath, newConfig.OutputPath)
	require.Equal(t, config.Gallery.Create, newConfig.Gallery.Create)
	require.Equal(t, config.Providers.GuildWars2.GetPath(), newConfig.Providers.GuildWars2.GetPath())
	require.Equal(t, config.Providers.Steam.GetPath(), newConfig.Providers.Steam.GetPath())
}

func TestConfigInheritance(t *testing.T) {
	// Create a test config with inheritance
	testConfig := `
output_path = "global_output"

[global]
download_covers = true

[providers.guild_wars_2]
enabled = false
path = "gw2_path"
output_path = "gw2_output"
download_covers = false

[providers.minecraft]
path = "minecraft_path"
`

	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "test_config.toml")

	err := os.WriteFile(testConfigPath, []byte(testConfig), 0644)
	require.NoError(t, err)

	// Load the config
	config, err := NewConfig(testConfigPath)
	require.NoError(t, err)

	// Test inheritance in GuildWars2 config
	require.Equal(t, "gw2_path", config.Providers.GuildWars2.GetPath())
	require.False(t, config.Providers.GuildWars2.IsEnabled())
	require.False(t, config.DryRun)

	// Test inheritance in Minecraft config (should inherit from global)
	require.Equal(t, "minecraft_path", config.Providers.Minecraft.GetPath())
	require.True(t, config.Global.DownloadCovers)
}

func TestConfigMerge(t *testing.T) {
	config := &Config{
		Global: GlobalConfig{
			DownloadCovers: true,
		},
		Providers: Providers{
			GuildWars2: ProviderConfig{
				Enabled: Ptr(false),
				Path:    Ptr("gw2_path"),
			},
		},
	}

	config.Providers.GuildWars2.Merge(&config.Global)

	require.False(t, config.Providers.GuildWars2.IsEnabled())
	require.True(t, *config.Providers.GuildWars2.DownloadCovers)
}
