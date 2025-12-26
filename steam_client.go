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
	"strconv"
	"strings"
	"time"

	"git.nakama.town/fmartingr/gotoolkit/cache"
	toolkitModel "git.nakama.town/fmartingr/gotoolkit/model"
)

const (
	steamAppListURL    = "https://api.steampowered.com/IStoreService/GetAppList/v1/"
	steamGameHeaderURL = "https://cdn.cloudflare.steamstatic.com/steam/apps/%s/header.jpg"

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

// SteamClient handles API interactions with Steam
type SteamClient struct {
	log         *slog.Logger
	cache       toolkitModel.Cache
	steamApps   SteamAppList
	apiKey      string
	fileManager *FileManager
}

// NewSteamClient creates a new Steam client
// apiKey is optional but required for downloading the app list (the endpoint requires authentication)
func NewSteamClient(fileManager *FileManager, apiKey string) (*SteamClient, error) {
	steamCache, err := cache.NewFileCache("games-screenshot-manager")
	if err != nil {
		return nil, fmt.Errorf("error creating file cache: %s", err)
	}

	client := &SteamClient{
		log:         slog.Default().With("component", "steam-client"),
		cache:       steamCache,
		fileManager: fileManager,
		apiKey:      apiKey,
	}

	if err := client.DownloadSteamAppList(); err != nil {
		return nil, fmt.Errorf("error downloading steam app list: %s", err)
	}

	client.log.Info("Steam app list ready", slog.Int("num_apps", len(client.steamApps.Apps)))

	return client, nil
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
	if err != nil && !errors.Is(err, toolkitModel.ErrCacheKeyDontExist) {
		return fmt.Errorf("error retrieving cache: %s", err)
	}

	if result != nil {
		c.log.Debug("Using cached Steam app list", slog.String("cache_key", cacheKey))
		download = false
		payload = result.([]byte)
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

			if response.Body != nil {
				defer response.Body.Close()
			}

			c.log.Debug("Steam API response received",
				slog.Int("page", page),
				slog.Int("status_code", response.StatusCode),
				slog.String("status", response.Status),
				slog.String("content_type", response.Header.Get("Content-Type")))

			if response.StatusCode < 200 || response.StatusCode >= 300 {
				// Read body to include in error message
				bodyBytes, _ := io.ReadAll(response.Body)
				bodyPreview := string(bodyBytes)
				if len(bodyPreview) > 500 {
					bodyPreview = bodyPreview[:500]
				}
				return fmt.Errorf("HTTP error on page %d: status code %d, status: %s. Response body: %s", page, response.StatusCode, response.Status, bodyPreview)
			}

			pagePayload, err := io.ReadAll(response.Body)
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
		if err := c.cache.Set(cacheKey, payload, cache.WithTTL(7*24*time.Hour)); err != nil {
			return fmt.Errorf("error caching steam app list: %s", err)
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
	return c.steamApps
}

// GetGameName returns the name of a game by its ID
func (c *SteamClient) GetGameName(gameID string) string {
	return c.steamApps.GetGameName(gameID)
}

// GetGameID returns the ID of a game by its name
func (c *SteamClient) GetGameID(gameName string) string {
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
