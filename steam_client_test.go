package gamesscreenshotmanager

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// stubCache is a gameCache that answers from memory, so a test never touches
// the user's cache directory or the network.
type stubCache struct {
	payload []byte
	err     error
}

func (c *stubCache) Get(string) ([]byte, error) { return c.payload, c.err }

func (c *stubCache) Set(string, []byte, time.Duration) error { return nil }

func newTestSteamClient(cache gameCache) *SteamClient {
	return &SteamClient{log: slog.Default(), cache: cache}
}

func TestSteamClient_DownloadSteamAppListUsesACachedList(t *testing.T) {
	client := newTestSteamClient(&stubCache{
		payload: []byte(`{"response":{"apps":[{"appid":105600,"name":"Terraria"}]}}`),
	})

	if err := client.DownloadSteamAppList(); err != nil {
		t.Fatalf("DownloadSteamAppList() returned an error: %v", err)
	}

	if name := client.GetGameName("105600"); name != "Terraria" {
		t.Errorf("GetGameName() = %q, want %q", name, "Terraria")
	}
}

// An empty payload cannot be parsed. Treating it as a hit would keep the
// download from ever repairing the cache.
func TestSteamClient_DownloadSteamAppListRejectsAnEmptyPayload(t *testing.T) {
	client := newTestSteamClient(&stubCache{payload: []byte{}})

	err := client.DownloadSteamAppList()
	if err == nil {
		t.Fatal("DownloadSteamAppList() returned no error, want the download to be attempted")
	}

	// No API key is set, so the attempted download reports that instead of a
	// parse failure. That is the proof the empty payload was not used.
	if !strings.Contains(err.Error(), "API key") {
		t.Errorf("DownloadSteamAppList() error = %v, want it to report the missing API key", err)
	}
}

// A cache miss is not a failure. The download proceeds.
func TestSteamClient_DownloadSteamAppListTreatsAMissAsAMiss(t *testing.T) {
	client := newTestSteamClient(&stubCache{err: errCacheKeyNotFound})

	err := client.DownloadSteamAppList()
	if err == nil {
		t.Fatal("DownloadSteamAppList() returned no error, want the download to be attempted")
	}

	if !strings.Contains(err.Error(), "API key") {
		t.Errorf("DownloadSteamAppList() error = %v, want it to report the missing API key", err)
	}
}

// A cache that is broken, rather than empty, must be reported rather than
// swallowed as a miss.
func TestSteamClient_DownloadSteamAppListReportsACacheFailure(t *testing.T) {
	client := newTestSteamClient(&stubCache{err: errors.New("disk on fire")})

	err := client.DownloadSteamAppList()
	if err == nil {
		t.Fatal("DownloadSteamAppList() returned no error, want the cache failure")
	}

	if !strings.Contains(err.Error(), "retrieving cache") {
		t.Errorf("DownloadSteamAppList() error = %v, want it to report the cache failure", err)
	}
}
