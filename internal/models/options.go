package models

type Options struct {
	OutputPath        string
	DryRun            bool
	DownloadCovers    bool
	ProcessBufferSize int
	WorkersNum        int
}
