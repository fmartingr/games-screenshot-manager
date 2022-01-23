package models

type Options struct {
	OutputPath        string
	DryRun            bool
	DownloadCovers    bool
	ProcessBufferSize int
}

type ProviderOptions struct {
	InputPath string
}

type Provider interface {
	FindGames(options ProviderOptions) ([]*Game, error)
}
