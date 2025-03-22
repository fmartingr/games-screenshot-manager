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
	steamProvider, err := NewSteamProvider(*r.config)
	if err != nil {
		return fmt.Errorf("failed to create steam provider: %w", err)
	}

	r.registerProvider("steam", steamProvider)

	return nil
}
