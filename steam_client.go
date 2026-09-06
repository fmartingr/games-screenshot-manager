package gamesscreenshotmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	steamAppListURL    = "https://api.steampowered.com/IStoreService/GetAppList/v1/"
	steamGameHeaderURL = "https://cdn.cloudflare.steamstatic.com/steam/apps/%s/header.jpg"

	// The app list holds no app of type "demo", and no request flag adds one.
	// This endpoint answers for a single app ID of any type, and it needs no
	// API key.
	steamAppDetailsURL = "https://store.steampowered.com/api/appdetails"

	// How long a name from steamAppDetailsURL is kept. A name almost never
	// changes, and the lookup is one request per app ID.
	steamAppNameTTL = 30 * 24 * time.Hour

	// How long an empty answer is kept. An ID with no store app is either a
	// non-Steam shortcut or a delisted app, so the answer is stable, but it is
	// cheap to ask again.
	steamAppNameMissTTL = 24 * time.Hour

	// API endpoints for published screenshots
	steamGetPublishedFilesURL = "https://api.steampowered.com/IPublishedFileService/GetUserFiles/v1/"

	// Steam content types
	steamContentTypeScreenshot = "4"
)

// PublishedScreenshot represents a screenshot published to Steam
type PublishedFileDetail struct {
	AppID           int    `json:"consumer_appid"`
	FileURL         string `json:"file_url"`
	PreviewURL      string `json:"preview_url"`
	PublishedFileID string `json:"publishedfileid"`
	TimeCreated     int    `json:"time_created"`
	TimeUpdated     int    `json:"time_updated"`
	ShortcutName    string `json:"shortcutname"`
}

func (f *PublishedFileDetail) GetFileSHA1() string {
	parts := strings.Split(f.FileURL, "/")
	return parts[len(parts)-1]
}

func (f *PublishedFileDetail) GetPreviewSHA1() string {
	parts := strings.Split(f.PreviewURL, "/")
	return parts[len(parts)-1]
}

// SteamPublishedFilesResponse represents the response from GetPublishedFilesByUser
type SteamPublishedFilesResponse struct {
	Response struct {
		PublishedFileDetails []PublishedFileDetail `json:"publishedfiledetails"`
		Total                int                   `json:"total"`
		StartIndex           int                   `json:"startindex"`
	} `json:"response"`
}

// SteamFileDetailsRequest represents the request to GetPublishedFileDetails
type SteamFileDetailsRequest struct {
	ItemCount        int      `json:"itemcount"`
	PublishedFileIDs []string `json:"publishedfileids"`
}

// SteamFileDetailsResponse represents the response from GetPublishedFileDetails
type SteamFileDetailsResponse struct {
	Response struct {
		PublishedFileDetails []struct {
			PublishedFileID string `json:"publishedfileid"`
			AppID           int    `json:"consumer_app_id"`
			Title           string `json:"title"`
			Description     string `json:"description"`
			FileURL         string `json:"file_url"`
			PreviewURL      string `json:"preview_url"`
			FileType        int    `json:"file_type"`
		} `json:"publishedfiledetails"`
		Result int `json:"result"`
	} `json:"response"`
}

// steamAppIDPattern is what a Steam app ID looks like. A folder name on disk
// supplies the ID, and the ID becomes part of a cache key, so anything else is
// refused before a request is made.
var steamAppIDPattern = regexp.MustCompile(`^[0-9]+$`)

// steamAppDetails is the part of the appdetails response this tool reads. The
// response is a map keyed by the app ID that was asked for. A Success of false
// is an answer: the ID is not a store app.
type steamAppDetails struct {
	Success bool `json:"success"`
	Data    struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"data"`
}

// SteamApp struct for game details
type SteamApp struct {
	AppID uint64 `json:"appid"`
	Name  string `json:"name"`
}

// SteamAppList is a collection of SteamApp
type SteamAppList struct {
	Apps []SteamApp `json:"apps"`
}

func (s *SteamAppList) GetGameName(gameID string) string {
	for _, app := range s.Apps {
		if fmt.Sprintf("%d", app.AppID) == gameID {
			return app.Name
		}
	}
	return ""
}

func (s *SteamAppList) GetGameID(gameName string) string {
	for _, app := range s.Apps {
		if app.Name == gameName {
			return fmt.Sprintf("%d", app.AppID)
		}
	}
	return ""
}

type SteamAppListResponse struct {
	Response struct {
		Apps      []SteamApp `json:"apps"`
		LastAppID uint64     `json:"last_appid,omitempty"`
	} `json:"response"`
}

// gameCache is the part of the cache the Steam client uses. It is an interface
// so a test can supply a cache that does not touch the user's cache directory.
type gameCache interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
}

// SteamClient handles API interactions with Steam
type SteamClient struct {
	log         *slog.Logger
	cache       gameCache
	steamApps   SteamAppList
	apiKey      string
	fileManager *FileManager

	// appDetailsURL is the endpoint a single app ID is looked up at. It is a
	// field so a test can point it at a local server.
	appDetailsURL string

	// The app list is downloaded the first time a name is asked for. A command
	// that builds a provider without reading a name pays nothing for it.
	appsOnce sync.Once
	appsErr  error
}

// NewSteamClient creates a new Steam client
// apiKey is optional but required for downloading the app list (the endpoint requires authentication)
func NewSteamClient(fileManager *FileManager, apiKey string) (*SteamClient, error) {
	steamCache, err := newFileCache("games-screenshot-manager")
	if err != nil {
		return nil, fmt.Errorf("error creating file cache: %s", err)
	}

	client := &SteamClient{
		log:           slog.Default().With("component", "steam-client"),
		cache:         steamCache,
		fileManager:   fileManager,
		apiKey:        apiKey,
		appDetailsURL: steamAppDetailsURL,
	}

	return client, nil
}

// ensureAppList downloads the app list once. A failure is reported once as
// well, and it is not fatal: a name is then asked of the store, one app ID at
// a time.
func (c *SteamClient) ensureAppList() error {
	c.appsOnce.Do(func() {
		if c.appsErr = c.DownloadSteamAppList(); c.appsErr != nil {
			c.log.Error("could not download the Steam app list, so every name comes from the store",
				slog.String("err", c.appsErr.Error()))

			return
		}

		c.log.Info("Steam app list ready", slog.Int("num_apps", len(c.steamApps.Apps)))
	})

	return c.appsErr
}

// SetAPIKey sets the Steam Web API key for authenticated requests
func (c *SteamClient) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// DownloadSteamAppList downloads the Steam APP List and caches it
// It's used to get the JSON response with all the games so we can match the game ID (folder name)
// with the game name (from the JSON response)
func (c *SteamClient) DownloadSteamAppList() error {
	cacheKey := "steam-applist"
	download := true
	var payload []byte

	result, err := c.cache.Get(cacheKey)
	if err != nil && !errors.Is(err, errCacheKeyNotFound) {
		return fmt.Errorf("error retrieving cache: %s", err)
	}

	// An empty payload is not a hit. It cannot be parsed, and treating it as
	// one would keep the download from ever repairing the cache.
	if err == nil && len(result) > 0 {
		c.log.Debug("Using cached Steam app list", slog.String("cache_key", cacheKey))
		download = false
		payload = result
	} else {
		c.log.Debug("Cache miss, will download Steam app list", slog.String("cache_key", cacheKey))
	}

	if download {
		if c.apiKey == "" {
			return fmt.Errorf("Steam API key is required to download app list. The ISteamApps/GetAppList endpoint has been deprecated and replaced with IStoreService/GetAppList which requires authentication")
		}

		c.log.Info("Downloading Steam APP List, used to get all game IDs and Names")

		// Collect all apps from all pages
		var allApps []SteamApp
		lastAppID := uint64(0)
		page := 1
		const maxResults = 50000 // Maximum allowed by Steam API

		for {
			c.log.Debug("Fetching Steam app list page",
				slog.Int("page", page),
				slog.Uint64("last_app_id", lastAppID),
				slog.Int("apps_collected", len(allApps)))

			// Build URL with API key and pagination parameters
			parsedURL, urlErr := url.Parse(steamAppListURL)
			if urlErr != nil {
				return fmt.Errorf("error parsing Steam APP List URL: %s", urlErr)
			}

			queryParams := parsedURL.Query()
			queryParams.Set("key", c.apiKey)
			queryParams.Set("max_results", strconv.Itoa(maxResults))
			if lastAppID > 0 {
				queryParams.Set("last_appid", strconv.FormatUint(lastAppID, 10))
			}
			parsedURL.RawQuery = queryParams.Encode()

			fullURL := parsedURL.String()
			c.log.Debug("Making request to Steam API", slog.String("url", fullURL))

			request := http.Request{
				Method: "GET",
				URL:    parsedURL,
				Header: map[string][]string{
					"User-Agent": {"github.com/fmartingr/games-screenshot-manager"},
				},
				ProtoMajor: 2,
				ProtoMinor: 1,
			}
			response, err := http.DefaultClient.Do(&request)
			if err != nil {
				return fmt.Errorf("error making request for Steam APP List (page %d): %s", page, err)
			}

			c.log.Debug("Steam API response received",
				slog.Int("page", page),
				slog.Int("status_code", response.StatusCode),
				slog.String("status", response.Status),
				slog.String("content_type", response.Header.Get("Content-Type")))

			// The body is read and closed inside the iteration. A deferred
			// close would hold every page's connection until the whole
			// download ends.
			pagePayload, err := io.ReadAll(response.Body)
			response.Body.Close()

			if response.StatusCode < 200 || response.StatusCode >= 300 {
				bodyPreview := string(pagePayload)
				if len(bodyPreview) > 500 {
					bodyPreview = bodyPreview[:500]
				}
				return fmt.Errorf("HTTP error on page %d: status code %d, status: %s. Response body: %s", page, response.StatusCode, response.Status, bodyPreview)
			}

			if err != nil {
				return fmt.Errorf("error reading steam response (page %d): %s", page, err)
			}

			// Parse this page's response
			var pageResponse SteamAppListResponse
			if err := json.Unmarshal(pagePayload, &pageResponse); err != nil {
				preview := string(pagePayload)
				if len(preview) > 500 {
					preview = preview[:500]
				}
				return fmt.Errorf("error unmarshalling steam's response (page %d): %s. Response preview: %s", page, err, preview)
			}

			// Check if we got any apps
			if len(pageResponse.Response.Apps) == 0 {
				c.log.Debug("No apps in response, stopping pagination", slog.Int("page", page))
				break
			}

			// Add apps from this page to our collection
			allApps = append(allApps, pageResponse.Response.Apps...)
			c.log.Debug("Page fetched successfully",
				slog.Int("page", page),
				slog.Int("apps_in_page", len(pageResponse.Response.Apps)),
				slog.Int("total_apps", len(allApps)),
				slog.Uint64("last_app_id", pageResponse.Response.LastAppID))

			// Check if there are more pages
			if pageResponse.Response.LastAppID == 0 {
				c.log.Debug("Last app ID is 0, no more pages", slog.Int("page", page))
				break
			}

			lastAppID = pageResponse.Response.LastAppID
			page++

			// Safety limit to prevent infinite loops
			if page > 100 {
				c.log.Warn("Reached maximum page limit (100), stopping pagination", slog.Int("total_apps", len(allApps)))
				break
			}
		}

		// Create combined response
		combinedResponse := SteamAppListResponse{
			Response: struct {
				Apps      []SteamApp `json:"apps"`
				LastAppID uint64     `json:"last_appid,omitempty"`
			}{
				Apps:      allApps,
				LastAppID: 0, // Set to 0 to indicate complete list
			},
		}

		// Marshal combined response to JSON for caching
		payload, err = json.Marshal(combinedResponse)
		if err != nil {
			return fmt.Errorf("error marshalling combined Steam app list: %s", err)
		}

		c.log.Info("Successfully downloaded all Steam app list pages",
			slog.Int("total_pages", page),
			slog.Int("total_apps", len(allApps)))

		// Cache the complete combined response for 1 week
		// The cache only saves the next download. Losing it must not throw away
		// the list that is already in memory.
		if err := c.cache.Set(cacheKey, payload, 7*24*time.Hour); err != nil {
			c.log.Warn("could not cache the Steam app list", slog.String("err", err.Error()))
		}
	}

	c.log.Debug("Attempting to parse JSON response", slog.Int("payload_length", len(payload)))

	steamListResponse := SteamAppListResponse{}
	jsonErr := json.Unmarshal(payload, &steamListResponse)
	if jsonErr != nil {
		// Try to provide more helpful error message
		preview := string(payload)
		if len(preview) > 500 {
			preview = preview[:500]
		}
		c.log.Error("Failed to unmarshal Steam API response",
			slog.Any("error", jsonErr),
			slog.String("preview", preview))
		return fmt.Errorf("error unmarshalling steam's response: %s. Response preview: %s", jsonErr, preview)
	}

	c.log.Debug("Successfully parsed Steam API response",
		slog.Int("apps_count", len(steamListResponse.Response.Apps)),
		slog.Uint64("last_app_id", steamListResponse.Response.LastAppID))

	if len(steamListResponse.Response.Apps) == 0 {
		return fmt.Errorf("coulnd't get steam app list")
	}

	c.steamApps = SteamAppList{
		Apps: steamListResponse.Response.Apps,
	}

	return nil
}

// GetSteamApps returns the current list of Steam apps
func (c *SteamClient) GetSteamApps() SteamAppList {
	_ = c.ensureAppList()

	return c.steamApps
}

// GetGameName returns the name of a game by its ID. The app list is the first
// source. It holds no demo, so a miss asks the store for that one app ID.
func (c *SteamClient) GetGameName(gameID string) string {
	_ = c.ensureAppList()

	if name := c.steamApps.GetGameName(gameID); name != "" {
		return name
	}

	return c.lookupAppName(gameID)
}

// lookupAppName returns the store name for one app ID and remembers the
// answer. An empty name is cached as well, so an ID with no store app makes
// one request rather than one per run.
//
// A failure returns an empty string. The caller falls back to the folder name,
// which is the behaviour without this lookup.
func (c *SteamClient) lookupAppName(gameID string) string {
	if !steamAppIDPattern.MatchString(gameID) {
		return ""
	}

	cacheKey := "steam-appname-" + gameID

	cached, err := c.cache.Get(cacheKey)
	if err == nil {
		c.log.Debug("Using the cached app name", slog.String("game_id", gameID))
		return string(cached)
	}

	if !errors.Is(err, errCacheKeyNotFound) {
		c.log.Warn("could not read the cached app name", slog.String("game_id", gameID), slog.String("err", err.Error()))
	}

	name, ok := c.downloadAppName(gameID)
	if !ok {
		return ""
	}

	ttl := steamAppNameTTL
	if name == "" {
		ttl = steamAppNameMissTTL
	}

	// The cache only saves the next request. Losing it must not lose the name.
	if err := c.cache.Set(cacheKey, []byte(name), ttl); err != nil {
		c.log.Warn("could not cache the app name", slog.String("game_id", gameID), slog.String("err", err.Error()))
	}

	return name
}

// downloadAppName asks the store for one app ID. The second return value
// reports an answer that is worth caching: a request that failed says nothing
// about the app ID, so it must not be stored as a miss.
func (c *SteamClient) downloadAppName(gameID string) (string, bool) {
	endpoint := c.appDetailsURL
	if endpoint == "" {
		endpoint = steamAppDetailsURL
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		c.log.Warn("could not parse the app details URL", slog.String("err", err.Error()))
		return "", false
	}

	queryParams := parsedURL.Query()
	queryParams.Set("appids", gameID)
	queryParams.Set("filters", "basic")
	parsedURL.RawQuery = queryParams.Encode()

	request, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		c.log.Warn("could not build the app details request", slog.String("err", err.Error()))
		return "", false
	}
	request.Header.Set("User-Agent", "github.com/fmartingr/games-screenshot-manager")

	c.log.Debug("Looking up an app name in the Steam store", slog.String("game_id", gameID))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		c.log.Warn("could not request the app details", slog.String("game_id", gameID), slog.String("err", err.Error()))
		return "", false
	}
	defer response.Body.Close()

	// The endpoint is rate limited. A refusal is about the request, not about
	// the app ID, so it is not cached.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		c.log.Warn("the app details request failed", slog.String("game_id", gameID), slog.Int("status_code", response.StatusCode))
		return "", false
	}

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		c.log.Warn("could not read the app details response", slog.String("game_id", gameID), slog.String("err", err.Error()))
		return "", false
	}

	details := map[string]steamAppDetails{}
	if err := json.Unmarshal(payload, &details); err != nil {
		c.log.Warn("could not parse the app details response", slog.String("game_id", gameID), slog.String("err", err.Error()))
		return "", false
	}

	entry, found := details[gameID]
	if !found || !entry.Success {
		c.log.Debug("The Steam store holds no app for this ID", slog.String("game_id", gameID))
		return "", true
	}

	c.log.Info("Resolved an app name from the Steam store",
		slog.String("game_id", gameID),
		slog.String("name", entry.Data.Name),
		slog.String("type", entry.Data.Type))

	return entry.Data.Name, true
}

// GetGameID returns the ID of a game by its name
func (c *SteamClient) GetGameID(gameName string) string {
	_ = c.ensureAppList()

	return c.steamApps.GetGameID(gameName)
}

// DownloadGameCover downloads a game's cover image
func (c *SteamClient) DownloadGameCover(gameID string) (*os.File, error) {
	coverURL := fmt.Sprintf(steamGameHeaderURL, gameID)
	coverFile, err := c.fileManager.DownloadURL(coverURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading cover: %s", err)
	}

	return coverFile, nil
}

// GetPublishedScreenshots retrieves all screenshots published by a Steam user
func (c *SteamClient) GetPublishedScreenshots(steamID string) ([]PublishedFileDetail, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Steam API key not set. Use SetAPIKey method first")
	}

	// First, get the list of published files
	publishedFiles, err := c.getPublishedFilesByUser(steamID)
	if err != nil {
		return nil, fmt.Errorf("error getting published files: %s", err)
	}

	if len(publishedFiles) == 0 {
		c.log.Info("No published files found for user", slog.String("steamID", steamID))
		return []PublishedFileDetail{}, nil
	}

	return publishedFiles, nil
}

// getPublishedFilesByUser gets all published files for a specific user
func (c *SteamClient) getPublishedFilesByUser(steamID string) ([]PublishedFileDetail, error) {
	c.log.Info("Getting published files for user", slog.String("steamID", steamID))

	// Set up query parameters
	params := url.Values{}
	params.Add("key", c.apiKey)
	params.Add("steamid", steamID)
	params.Add("filetype", steamContentTypeScreenshot)
	params.Add("numperpage", "100") // Maximum allowed by Steam API

	var allPublishedFiles []PublishedFileDetail
	page := 1

	for {
		// Add page parameter
		params.Set("page", strconv.Itoa(page))

		// Build the request URL
		reqURL := fmt.Sprintf("%s?%s", steamGetPublishedFilesURL, params.Encode())

		// Make the request
		response, err := http.Get(reqURL)
		if err != nil {
			return nil, fmt.Errorf("error making request: %s", err)
		}

		if response.Body != nil {
			defer response.Body.Close()
		}

		// Read the response
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response: %s", err)
		}

		// Store the body for debugging
		// _ = os.WriteFile(fmt.Sprintf("steam-published-files-%d.json", page), body, 0644)

		// Parse the response
		var result SteamPublishedFilesResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("error unmarshalling response: %s", err)
		}

		// Check if we got any results
		if result.Response.Total == 0 {
			break
		}

		// Add the results to our collection
		allPublishedFiles = append(allPublishedFiles, result.Response.PublishedFileDetails...)

		// Check if we've got all the results
		if len(allPublishedFiles) >= result.Response.Total {
			break
		}

		// Move to the next page
		page++
	}

	return allPublishedFiles, nil
}
