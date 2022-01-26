package models

type ProviderOptions struct {
	InputPath string
}

type Provider interface {
	FindGames(options ProviderOptions) ([]*Game, error)
}
