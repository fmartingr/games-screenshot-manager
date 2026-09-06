package gamesscreenshotmanager

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// stubCache is a gameCache that answers from memory, so a test never touches
// the user's cache directory or the network. It records what was written, so a
// test can assert on the entry and on its expiry time.
type stubCache struct {
	payload []byte
	err     error
	writes  map[string][]byte
	ttls    map[string]time.Duration
	reads   int
}

func (c *stubCache) Get(string) ([]byte, error) {
	c.reads++

	return c.payload, c.err
}

func (c *stubCache) Set(key string, value []byte, ttl time.Duration) error {
	if c.writes == nil {
		c.writes = map[string][]byte{}
		c.ttls = map[string]time.Duration{}
	}

	c.writes[key] = value
	c.ttls[key] = ttl

	return nil
}

func newTestSteamClient(cache gameCache) *SteamClient {
	return &SteamClient{log: slog.Default(), cache: cache}
}

// newTestStoreClient returns a client whose store lookups reach handler rather
// than the Steam store, and a counter of the requests that reached it.
func newTestStoreClient(t *testing.T, cache gameCache, handler http.HandlerFunc) (*SteamClient, *int) {
	t.Helper()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	client := newTestSteamClient(cache)
	client.appDetailsURL = server.URL

	// The app list is not what these tests cover, so it counts as loaded.
	client.appsOnce.Do(func() {})

	return client, &requests
}

const demoAppDetails = `{"4148250":{"success":true,"data":{"type":"demo","name":"Echoes of Aincrad DEMO Version"}}}`

// The app list is the first source. A name it holds must cost no request.
func TestSteamClient_GetGameNameUsesTheAppList(t *testing.T) {
	client, requests := newTestStoreClient(t, &stubCache{err: errCacheKeyNotFound}, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the store was asked for a name the app list holds")
	})
	client.steamApps = SteamAppList{Apps: []SteamApp{{AppID: 105600, Name: "Terraria"}}}

	if name := client.GetGameName("105600"); name != "Terraria" {
		t.Errorf("GetGameName() = %q, want %q", name, "Terraria")
	}

	if *requests != 0 {
		t.Errorf("the store was asked %d times, want 0", *requests)
	}
}

// The app list holds no app of type demo, so the store answers for it.
func TestSteamClient_GetGameNameFallsBackToTheStore(t *testing.T) {
	cache := &stubCache{err: errCacheKeyNotFound}
	client, requests := newTestStoreClient(t, cache, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("appids"); got != "4148250" {
			t.Errorf("appids = %q, want %q", got, "4148250")
		}

		if got := r.URL.Query().Get("filters"); got != "basic" {
			t.Errorf("filters = %q, want %q", got, "basic")
		}

		fmt.Fprint(w, demoAppDetails)
	})

	name := client.GetGameName("4148250")
	if name != "Echoes of Aincrad DEMO Version" {
		t.Errorf("GetGameName() = %q, want %q", name, "Echoes of Aincrad DEMO Version")
	}

	if *requests != 1 {
		t.Errorf("the store was asked %d times, want 1", *requests)
	}

	if got := string(cache.writes["steam-appname-4148250"]); got != name {
		t.Errorf("cached name = %q, want %q", got, name)
	}

	if got := cache.ttls["steam-appname-4148250"]; got != steamAppNameTTL {
		t.Errorf("cached TTL = %v, want %v", got, steamAppNameTTL)
	}
}

// A cached name answers on its own.
func TestSteamClient_GetGameNameUsesACachedName(t *testing.T) {
	client, requests := newTestStoreClient(t, &stubCache{payload: []byte("Order of the Sinking Star Demo")}, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the store was asked for a name the cache holds")
	})

	if name := client.GetGameName("4597250"); name != "Order of the Sinking Star Demo" {
		t.Errorf("GetGameName() = %q, want %q", name, "Order of the Sinking Star Demo")
	}

	if *requests != 0 {
		t.Errorf("the store was asked %d times, want 0", *requests)
	}
}

// An ID with no store app is an answer. It is cached, so the next run does not
// repeat the request.
func TestSteamClient_GetGameNameCachesAnUnknownApp(t *testing.T) {
	cache := &stubCache{err: errCacheKeyNotFound}
	client, _ := newTestStoreClient(t, cache, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"12345":{"success":false}}`)
	})

	if name := client.GetGameName("12345"); name != "" {
		t.Errorf("GetGameName() = %q, want an empty string", name)
	}

	if got, found := cache.writes["steam-appname-12345"]; !found || len(got) != 0 {
		t.Errorf("cached entry = %q, found = %v, want an empty entry", got, found)
	}

	if got := cache.ttls["steam-appname-12345"]; got != steamAppNameMissTTL {
		t.Errorf("cached TTL = %v, want %v", got, steamAppNameMissTTL)
	}
}

// A refusal is about the request, not about the app ID. It must not be cached
// as a miss, or a rate limit would hide the name for a day.
func TestSteamClient_GetGameNameDoesNotCacheAFailedRequest(t *testing.T) {
	cache := &stubCache{err: errCacheKeyNotFound}
	client, _ := newTestStoreClient(t, cache, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	if name := client.GetGameName("4148250"); name != "" {
		t.Errorf("GetGameName() = %q, want an empty string", name)
	}

	if _, found := cache.writes["steam-appname-4148250"]; found {
		t.Error("a failed request was cached")
	}
}

// A body that cannot be parsed is a failure, not a miss.
func TestSteamClient_GetGameNameHandlesABrokenBody(t *testing.T) {
	cache := &stubCache{err: errCacheKeyNotFound}
	client, _ := newTestStoreClient(t, cache, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	})

	if name := client.GetGameName("4148250"); name != "" {
		t.Errorf("GetGameName() = %q, want an empty string", name)
	}

	if _, found := cache.writes["steam-appname-4148250"]; found {
		t.Error("a broken response was cached")
	}
}

// A folder name that is not an app ID becomes part of a cache key, so it is
// refused before a request is made.
func TestSteamClient_GetGameNameRefusesAnIDThatIsNotAnAppID(t *testing.T) {
	client, requests := newTestStoreClient(t, &stubCache{err: errCacheKeyNotFound}, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the store was asked about a name that is not an app ID")
	})

	for _, gameID := range []string{"../etc", "abc", "", "760/remote"} {
		if name := client.GetGameName(gameID); name != "" {
			t.Errorf("GetGameName(%q) = %q, want an empty string", gameID, name)
		}
	}

	if *requests != 0 {
		t.Errorf("the store was asked %d times, want 0", *requests)
	}
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

// The app list is downloaded the first time a name is asked for, and once
// only. A command that reads no name must not pay for the download.
func TestSteamClient_DownloadsTheAppListOnce(t *testing.T) {
	cache := &stubCache{payload: []byte(`{"response":{"apps":[{"appid":105600,"name":"Terraria"}]}}`)}
	client := newTestSteamClient(cache)

	if cache.reads != 0 {
		t.Errorf("the cache was read %d times before a name was asked for, want 0", cache.reads)
	}

	for range 2 {
		if name := client.GetGameName("105600"); name != "Terraria" {
			t.Errorf("GetGameName() = %q, want %q", name, "Terraria")
		}
	}

	if cache.reads != 1 {
		t.Errorf("the app list was read %d times, want 1", cache.reads)
	}
}

// The constructor no longer downloads, so it no longer needs the API key that
// the download needs.
func TestNewSteamClient_DoesNotDownloadTheAppList(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client, err := NewSteamClient(nil, "")
	if err != nil {
		t.Fatalf("NewSteamClient() returned an error: %v", err)
	}

	if apps := len(client.steamApps.Apps); apps != 0 {
		t.Errorf("the client holds %d apps, want 0", apps)
	}
}
