package models

import "github.com/sirupsen/logrus"

type ProviderOptions struct {
	InputPath string
}

type Provider interface {
	FindGames(options ProviderOptions) ([]*Game, error)
}

type ProviderFactory func(logger *logrus.Logger) Provider
