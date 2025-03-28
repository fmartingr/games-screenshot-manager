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
	"time"

	"git.nakama.town/fmartingr/gotoolkit/cache"
	toolkitModel "git.nakama.town/fmartingr/gotoolkit/model"
)

const (
	steamAppListURL    = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"
	steamGameHeaderURL = "https://cdn.cloudflare.steamstatic.com/steam/apps/%s/header.jpg"

	// API endpoints for published screenshots
	steamGetPublishedFilesURL = "https://api.steampowered.com/IPublishedFileService/GetUserFiles/v1/"
	steamGetFileDetailsURL    = "https://api.steampowered.com/IPublishedFileService/GetUserFiles/v1/"

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
	AppList SteamAppList `json:"applist"`
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
func NewSteamClient(fileManager *FileManager) (*SteamClient, error) {
	steamCache, err := cache.NewFileCache("games-screenshot-manager")
	if err != nil {
		return nil, fmt.Errorf("error creating file cache: %s", err)
	}

	client := &SteamClient{
		log:         slog.Default().With("component", "steam-client"),
		cache:       steamCache,
		fileManager: fileManager,
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
		download = false
		payload = result.([]byte)
	}

	if download {
		c.log.Info("Downloading Steam APP List, used to get all game IDs and Names")
		parsedURL, _ := url.Parse(steamAppListURL)
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
			return fmt.Errorf("error making request for Steam APP List: %s", err)
		}

		if response.Body != nil {
			defer response.Body.Close()
		}

		payload, err = io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("error reading steam response: %s", err)
		}

		if err := c.cache.Set(cacheKey, payload, cache.WithTTL(24*time.Hour)); err != nil {
			return fmt.Errorf("error caching steam app list: %s", err)
		}
	}

	steamListResponse := SteamAppListResponse{}
	jsonErr := json.Unmarshal(payload, &steamListResponse)
	if jsonErr != nil {
		return fmt.Errorf("error unmarshalling steam's response: %s", jsonErr)
	}

	if len(steamListResponse.AppList.Apps) == 0 {
		return fmt.Errorf("coulnd't get steam app list")
	}

	c.steamApps = steamListResponse.AppList

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
