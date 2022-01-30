package registry

import (
	"errors"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/sirupsen/logrus"
)

var ErrProviderAlreadyRegistered = errors.New("provider already registered")
var ErrProviderNotRegistered = errors.New("provider not registered")

type ProviderRegistry struct {
	logger *logrus.Entry
	cache  models.Cache

	providers map[string]*models.Provider
}

func (r *ProviderRegistry) Register(name string, providerFactory models.ProviderFactory) error {
	_, exists := r.providers[name]
	if exists {
		return ErrProviderAlreadyRegistered
	}

	provider := providerFactory(r.logger.Logger, r.cache)
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

func NewProviderRegistry(logger *logrus.Logger, cache models.Cache) *ProviderRegistry {
	return &ProviderRegistry{
		logger:    logger.WithField("from", "registry"),
		cache:     cache,
		providers: make(map[string]*models.Provider),
	}
}
