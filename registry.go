package gamesscreenshotmanager

import (
	"fmt"
	"log/slog"
)

type ProviderRegistry struct {
	Providers map[string]Provider
	config    *Config
	log       *slog.Logger
}

func NewProviderRegistry(config *Config) (*ProviderRegistry, error) {
	registry := &ProviderRegistry{
		Providers: make(map[string]Provider),
		config:    config,
		log:       slog.Default().With("module", "provider_registry"),
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
		steamProvider, err := NewSteamProvider(*r.config)
		if err != nil {
			return fmt.Errorf("failed to create steam provider: %w", err)
		}

		r.registerProvider("steam", steamProvider)
	}

	if r.config.Providers.GuildWars2.IsEnabled() {
		guildWars2Provider, err := NewGuildWars2Provider(*r.config)
		if err != nil {
			return fmt.Errorf("failed to create guild wars 2 provider: %w", err)
		}

		r.registerProvider("guild_wars_2", guildWars2Provider)
	}

	if r.config.Providers.WorldOfWarcraft.IsEnabled() {
		worldOfWarcraftProvider, err := NewWorldOfWarcraftProvider(*r.config)
		if err != nil {
			return fmt.Errorf("failed to create world of warcraft provider: %w", err)
		}

		r.registerProvider("world_of_warcraft", worldOfWarcraftProvider)
	}

	if r.config.Providers.Minecraft.IsEnabled() {
		minecraftProvider, err := NewMinecraftProvider(*r.config)
		if err != nil {
			return fmt.Errorf("failed to create minecraft provider: %w", err)
		}

		r.registerProvider("minecraft", minecraftProvider)
	}

	if r.config.Providers.PlayStation4.IsEnabled() {
		playstation4Provider, err := NewPlaystation4Provider(*r.config)
		if err != nil {
			return fmt.Errorf("failed to create playstation 4 provider: %w", err)
		}

		r.registerProvider("playstation4", playstation4Provider)
	}
	return nil
}
