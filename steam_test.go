package gamesscreenshotmanager

import "testing"

func TestSteamProvider_IsIgnored(t *testing.T) {
	tests := []struct {
		name    string
		config  []string
		gameID  string
		ignored bool
	}{
		{name: "a listed ID", config: []string{"4148250", "4597250"}, gameID: "4148250", ignored: true},
		{name: "an ID that is not listed", config: []string{"4148250"}, gameID: "1222700", ignored: false},
		{name: "an entry with spaces around it", config: []string{"  4148250 "}, gameID: "4148250", ignored: true},
		{name: "an empty entry", config: []string{"", "   "}, gameID: "", ignored: false},
		{name: "an empty list", config: []string{}, gameID: "4148250", ignored: false},
		{name: "no list at all", config: nil, gameID: "4148250", ignored: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &SteamProvider{ignoredGames: newIgnoredGamesSet(test.config)}

			if got := provider.isIgnored(test.gameID); got != test.ignored {
				t.Errorf("isIgnored(%q) = %v, want %v", test.gameID, got, test.ignored)
			}
		})
	}
}
