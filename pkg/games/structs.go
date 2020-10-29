package games

type Game struct {
	ID          uint64
	Name        string
	Platform    string
	Provider    string
	Screenshots []Screenshot
}

type Screenshot struct {
	Path string
}
