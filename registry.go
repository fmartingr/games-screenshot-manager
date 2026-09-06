package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
	"runtime"
)

type ProviderRegistry struct {
	Providers   map[string]Provider
	config      *Config
	log         *slog.Logger
	gameManager *GameManager
	fileManager *FileManager
}

func NewProviderRegistry(config *Config, gameManager *GameManager, fileManager *FileManager) (*ProviderRegistry, error) {
	registry := &ProviderRegistry{
		Providers:   make(map[string]Provider),
		config:      config,
		log:         slog.Default().With("module", "provider_registry"),
		gameManager: gameManager,
		fileManager: fileManager,
	}

	if err := registry.registerProviders(); err != nil {
		return nil, fmt.Errorf("failed to register providers: %w", err)
	}

	return registry, nil
}

func (r *ProviderRegistry) registerProvider(name string, provider Provider) {
	r.Providers[name] = provider
}

func (r *ProviderRegistry) registerProviders() error {
	if r.config.Providers.Steam.IsEnabled() {
		steamProvider, err := NewSteamProvider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create steam provider: %w", err)
		}

		r.registerProvider("steam", steamProvider)
	}

	if r.config.Providers.GuildWars2.IsEnabled() {
		guildWars2Provider, err := NewGuildWars2Provider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create guild wars 2 provider: %w", err)
		}

		r.registerProvider("guild_wars_2", guildWars2Provider)
	}

	if r.config.Providers.WorldOfWarcraft.IsEnabled() {
		worldOfWarcraftProvider, err := NewWorldOfWarcraftProvider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create world of warcraft provider: %w", err)
		}

		r.registerProvider("world_of_warcraft", worldOfWarcraftProvider)
	}

	if r.config.Providers.Minecraft.IsEnabled() {
		minecraftProvider, err := NewMinecraftProvider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create minecraft provider: %w", err)
		}

		r.registerProvider("minecraft", minecraftProvider)
	}

	if r.config.Providers.Hytale.IsEnabled() {
		hytaleProvider, err := NewHytaleProvider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create hytale provider: %w", err)
		}

		r.registerProvider("hytale", hytaleProvider)
	}

	if r.config.Providers.NintendoSwitch2.IsEnabled() {
		nintendoSwitch2Provider, err := NewNintendoSwitch2Provider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create nintendo switch 2 provider: %w", err)
		}

		r.registerProvider("nintendo_switch_2", nintendoSwitch2Provider)
	}

	if r.config.Providers.PlayStation4.IsEnabled() {
		playstation4Provider, err := NewPlaystation4Provider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create playstation 4 provider: %w", err)
		}

		r.registerProvider("playstation4", playstation4Provider)
	}

	if r.config.Providers.PlayStation5.IsEnabled() {
		playstation5Provider, err := NewPlaystation5Provider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create playstation 5 provider: %w", err)
		}

		r.registerProvider("playstation5", playstation5Provider)
	}

	// Xbox Game Bar is only available on Windows
	if r.config.Providers.XboxGameBar.IsEnabled() && runtime.GOOS == "windows" {
		xboxGameBarProvider, err := NewXboxGameBarProvider(*r.config, r.gameManager, r.fileManager)
		if err != nil {
			return fmt.Errorf("failed to create xbox game bar provider: %w", err)
		}

		r.registerProvider("xbox_game_bar", xboxGameBarProvider)
	}

	return nil
}

// Requirements returns every requirement the registered providers declare,
// grouped by the program to look for. Two providers that want one program
// therefore produce one key that holds two uses.
func (r *ProviderRegistry) Requirements() map[string][]RequirementUse {
	requirements := make(map[string][]RequirementUse)

	for name, provider := range r.Providers {
		requirer, ok := provider.(Requirer)
		if !ok {
			continue
		}

		for _, requirement := range requirer.Requirements() {
			requirements[requirement.Binary] = append(requirements[requirement.Binary], RequirementUse{
				Provider:    name,
				Requirement: requirement,
			})
		}
	}

	return requirements
}
