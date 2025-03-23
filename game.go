package gamesscreenshotmanager

type Game struct {
	ID          string
	CoverURL    string
	Name        string
	Platform    string
	Provider    string
	Screenshots []*Media
	Clips       []*Media
	Recordings  []*Media
}

func NewGame(id, name, platform, provider string) *Game {
	return &Game{
		ID:          id,
		Name:        name,
		Platform:    platform,
		Provider:    provider,
		Screenshots: make([]*Media, 0),
		Clips:       make([]*Media, 0),
		Recordings:  make([]*Media, 0),
	}
}

func (g *Game) AddScreenshot(screenshot *Media) {
	g.Screenshots = append(g.Screenshots, screenshot)
}

func (g *Game) AddClip(clip *Media) {
	g.Clips = append(g.Clips, clip)
}

func (g *Game) AddRecording(recording *Media) {
	g.Recordings = append(g.Recordings, recording)
}

type GameManager struct {
	games map[string]*Game
}

func NewGameManager() *GameManager {
	return &GameManager{
		games: make(map[string]*Game),
	}
}

func (gm *GameManager) AddGame(game *Game) *Game {
	if _, ok := gm.games[game.ID]; !ok {
		gm.games[game.ID] = game
	}

	return gm.games[game.ID]
}

func (gm *GameManager) GetGame(id string) *Game {
	for _, game := range gm.games {
		if game.ID == id {
			return game
		}
	}
	return nil
}

func (gm *GameManager) GetGames() []*Game {
	games := make([]*Game, 0, len(gm.games))
	for _, game := range gm.games {
		games = append(games, game)
	}
	return games
}
