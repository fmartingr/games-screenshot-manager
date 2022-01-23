package registry

import (
	"errors"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
)

var ErrProviderAlreadyRegistered = errors.New("provider already registered")
var ErrProviderNotRegistered = errors.New("provider not registered")

type ProviderRegistry struct {
	providers map[string]*models.Provider
}

func (r *ProviderRegistry) Register(name string, provider models.Provider) error {
	_, exists := r.providers[name]
	if exists {
		return ErrProviderAlreadyRegistered
	}

	r.providers[name] = &provider

	return nil
}

func (r *ProviderRegistry) Get(providerName string) (models.Provider, error) {
	provider, exists := r.providers[providerName]
	if !exists {
		return nil, ErrProviderNotRegistered
	}
	return *provider, nil
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]*models.Provider),
	}
}
