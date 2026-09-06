package gamesscreenshotmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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

const (
	demoAppName  = "Echoes of Aincrad DEMO Version"
	demoCoverURL = "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/4148250/afd5a96ebc1247efc4e3bf864bf3ea219b9a4e7e/header.jpg?t=1781625833"

	demoAppDetails = `{"4148250":{"success":true,"data":{"type":"demo","name":"` + demoAppName + `","header_image":"` + demoCoverURL + `"}}}`
)

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
	if name != demoAppName {
		t.Errorf("GetGameName() = %q, want %q", name, demoAppName)
	}

	if *requests != 1 {
		t.Errorf("the store was asked %d times, want 1", *requests)
	}

	cached := steamAppInfo{}
	if err := json.Unmarshal(cache.writes["steam-appinfo-4148250"], &cached); err != nil {
		t.Fatalf("the cached entry could not be parsed: %v", err)
	}

	if cached.Name != demoAppName {
		t.Errorf("cached name = %q, want %q", cached.Name, demoAppName)
	}

	if cached.CoverURL != demoCoverURL {
		t.Errorf("cached cover URL = %q, want %q", cached.CoverURL, demoCoverURL)
	}

	if got := cache.ttls["steam-appinfo-4148250"]; got != steamAppInfoTTL {
		t.Errorf("cached TTL = %v, want %v", got, steamAppInfoTTL)
	}
}

// A cached name answers on its own.
func TestSteamClient_GetGameNameUsesACachedName(t *testing.T) {
	client, requests := newTestStoreClient(t, &stubCache{payload: []byte(`{"name":"Order of the Sinking Star Demo","cover_url":"https://example.invalid/header.jpg"}`)}, func(w http.ResponseWriter, r *http.Request) {
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

	got, found := cache.writes["steam-appinfo-12345"]
	if !found {
		t.Fatal("the miss was not cached")
	}

	cached := steamAppInfo{}
	if err := json.Unmarshal(got, &cached); err != nil {
		t.Fatalf("the cached entry could not be parsed: %v", err)
	}

	if cached != (steamAppInfo{}) {
		t.Errorf("cached entry = %+v, want an empty record", cached)
	}

	if got := cache.ttls["steam-appinfo-12345"]; got != steamAppInfoMissTTL {
		t.Errorf("cached TTL = %v, want %v", got, steamAppInfoMissTTL)
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

	if _, found := cache.writes["steam-appinfo-4148250"]; found {
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

	if _, found := cache.writes["steam-appinfo-4148250"]; found {
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

// coverFixture is a client whose store lookups and cover downloads both reach
// local servers, with a counter for each source.
type coverFixture struct {
	client        *SteamClient
	cache         *stubCache
	imagesURL     string
	storeRequests int
	legacyProbes  int
}

const testCoverBody = "cover-bytes"

// newTestCoverClient returns a fixture. legacyStatus is what the legacy cover
// path answers, and storeHasCover decides whether the store reports a cover
// URL.
func newTestCoverClient(t *testing.T, legacyStatus int, storeHasCover bool) *coverFixture {
	t.Helper()

	fixture := &coverFixture{cache: &stubCache{err: errCacheKeyNotFound}}

	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/legacy/") {
			fixture.legacyProbes++

			if legacyStatus != http.StatusOK {
				w.WriteHeader(legacyStatus)
				return
			}
		}

		fmt.Fprint(w, testCoverBody)
	}))
	t.Cleanup(images.Close)

	store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.storeRequests++

		if !storeHasCover {
			fmt.Fprint(w, `{"4148250":{"success":false}}`)
			return
		}

		fmt.Fprintf(w, `{"4148250":{"success":true,"data":{"type":"demo","name":%q,"header_image":%q}}}`,
			demoAppName, images.URL+"/store/abc123/header.jpg")
	}))
	t.Cleanup(store.Close)

	fileManager := NewFileManager(Config{})
	t.Cleanup(func() { _ = fileManager.Cleanup() })

	client := newTestSteamClient(fixture.cache)
	client.fileManager = fileManager
	client.appDetailsURL = store.URL
	client.gameHeaderURL = images.URL + "/legacy/%s.jpg"
	client.appsOnce.Do(func() {})

	fixture.client = client
	fixture.imagesURL = images.URL

	return fixture
}

// The legacy path needs no lookup, so an app it still serves must cost no
// store request.
func TestSteamClient_DownloadGameCoverUsesTheLegacyPath(t *testing.T) {
	fixture := newTestCoverClient(t, http.StatusOK, true)

	file, err := fixture.client.DownloadGameCover("4148250")
	if err != nil {
		t.Fatalf("DownloadGameCover() returned an error: %v", err)
	}

	if file == nil {
		t.Fatal("DownloadGameCover() returned no file")
	}

	if fixture.storeRequests != 0 {
		t.Errorf("the store was asked %d times, want 0", fixture.storeRequests)
	}
}

// An app published under the per-revision asset scheme answers 404 on the
// legacy path. The store then supplies the real URL.
func TestSteamClient_DownloadGameCoverFallsBackToTheStoreURL(t *testing.T) {
	fixture := newTestCoverClient(t, http.StatusNotFound, true)

	file, err := fixture.client.DownloadGameCover("4148250")
	if err != nil {
		t.Fatalf("DownloadGameCover() returned an error: %v", err)
	}

	contents, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("the downloaded cover could not be read: %v", err)
	}

	if string(contents) != testCoverBody {
		t.Errorf("the downloaded cover holds %q, want %q", contents, testCoverBody)
	}

	if fixture.storeRequests != 1 {
		t.Errorf("the store was asked %d times, want 1", fixture.storeRequests)
	}
}

// Both sources can fail. The error must name both, so the log says which app
// has no cover anywhere.
func TestSteamClient_DownloadGameCoverReportsBothFailures(t *testing.T) {
	fixture := newTestCoverClient(t, http.StatusNotFound, false)

	_, err := fixture.client.DownloadGameCover("4148250")
	if err == nil {
		t.Fatal("DownloadGameCover() returned no error, want both failures")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("DownloadGameCover() error = %v, want it to report the legacy 404", err)
	}

	if !strings.Contains(err.Error(), "no cover URL") {
		t.Errorf("DownloadGameCover() error = %v, want it to report the empty store answer", err)
	}
}

// The name and the cover URL arrive in the same response, so reading both must
// cost one request.
func TestSteamClient_TheNameAndTheCoverURLShareOneRequest(t *testing.T) {
	cache := &stubCache{err: errCacheKeyNotFound}
	client, requests := newTestStoreClient(t, cache, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, demoAppDetails)
	})

	if url := client.GetGameCoverURL("4148250"); url != demoCoverURL {
		t.Errorf("GetGameCoverURL() = %q, want %q", url, demoCoverURL)
	}

	// The write the first lookup made is what the second lookup reads.
	cache.payload = cache.writes["steam-appinfo-4148250"]
	cache.err = nil

	if name := client.GetGameName("4148250"); name != demoAppName {
		t.Errorf("GetGameName() = %q, want %q", name, demoAppName)
	}

	if *requests != 1 {
		t.Errorf("the store was asked %d times, want 1", *requests)
	}
}

// A damaged cache entry cannot be parsed. It is a miss, not an answer, so the
// store repairs it.
func TestSteamClient_ADamagedCacheEntryIsTreatedAsAMiss(t *testing.T) {
	cache := &stubCache{payload: []byte("not json")}
	client, requests := newTestStoreClient(t, cache, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, demoAppDetails)
	})

	if url := client.GetGameCoverURL("4148250"); url != demoCoverURL {
		t.Errorf("GetGameCoverURL() = %q, want %q", url, demoCoverURL)
	}

	if *requests != 1 {
		t.Errorf("the store was asked %d times, want 1", *requests)
	}
}

// A cover URL the cache already holds costs no request, so it is tried before
// the legacy path. Without this an app whose legacy path is dead would probe
// that dead path on every run.
func TestSteamClient_DownloadGameCoverPrefersACachedURL(t *testing.T) {
	fixture := newTestCoverClient(t, http.StatusNotFound, true)

	cached, err := json.Marshal(steamAppInfo{
		Name:     demoAppName,
		CoverURL: fixture.imagesURL + "/store/abc123/header.jpg",
	})
	if err != nil {
		t.Fatal(err)
	}

	fixture.cache.payload = cached
	fixture.cache.err = nil

	file, err := fixture.client.DownloadGameCover("4148250")
	if err != nil {
		t.Fatalf("DownloadGameCover() returned an error: %v", err)
	}

	contents, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("the downloaded cover could not be read: %v", err)
	}

	if string(contents) != testCoverBody {
		t.Errorf("the downloaded cover holds %q, want %q", contents, testCoverBody)
	}

	if fixture.legacyProbes != 0 {
		t.Errorf("the legacy path was probed %d times, want 0", fixture.legacyProbes)
	}

	if fixture.storeRequests != 0 {
		t.Errorf("the store was asked %d times, want 0", fixture.storeRequests)
	}
}

// A cached record that names no cover URL is the store saying it holds no app
// for the ID. That answer stands for its TTL, so it must not be asked again on
// every run.
func TestSteamClient_DownloadGameCoverDoesNotReAskAboutAKnownMiss(t *testing.T) {
	fixture := newTestCoverClient(t, http.StatusNotFound, false)

	cached, err := json.Marshal(steamAppInfo{})
	if err != nil {
		t.Fatal(err)
	}

	fixture.cache.payload = cached
	fixture.cache.err = nil

	if _, err := fixture.client.DownloadGameCover("4148250"); err == nil {
		t.Fatal("DownloadGameCover() returned no error, want the failure")
	}

	if fixture.storeRequests != 0 {
		t.Errorf("the store was asked %d times, want 0", fixture.storeRequests)
	}
}

// A publisher can replace the art, which leaves the cached URL answering 404.
// The store is then asked again, and the fresh URL is used.
func TestSteamClient_DownloadGameCoverRefreshesAStaleCachedURL(t *testing.T) {
	fixture := newTestCoverClient(t, http.StatusNotFound, true)

	cached, err := json.Marshal(steamAppInfo{
		Name:     demoAppName,
		CoverURL: fixture.imagesURL + "/legacy/stale.jpg",
	})
	if err != nil {
		t.Fatal(err)
	}

	fixture.cache.payload = cached
	fixture.cache.err = nil

	file, err := fixture.client.DownloadGameCover("4148250")
	if err != nil {
		t.Fatalf("DownloadGameCover() returned an error: %v", err)
	}

	contents, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("the downloaded cover could not be read: %v", err)
	}

	if string(contents) != testCoverBody {
		t.Errorf("the downloaded cover holds %q, want %q", contents, testCoverBody)
	}

	if fixture.storeRequests != 1 {
		t.Errorf("the store was asked %d times, want 1", fixture.storeRequests)
	}
}
