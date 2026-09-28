package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"aether/internal/booru"
	"aether/internal/deviantart"
	"aether/internal/platform"
	"aether/internal/wallpapersrc"
)

// sourceThumbEntry is the per-result row returned by the external-source
// thumbnail commands. The JSON keys match the wallhaven rows so existing
// consumers keep working; site/source identify the originating catalog.
type sourceThumbEntry struct {
	ID            string `json:"id"`
	Site          string `json:"site"`
	Source        string `json:"source"`
	Title         string `json:"title,omitempty"`
	Author        string `json:"author,omitempty"`
	URL           string `json:"url"`
	Path          string `json:"path"`
	Resolution    string `json:"resolution"`
	ThumbURL      string `json:"thumbURL"`
	ThumbnailPath string `json:"thumbnailPath,omitempty"`
}

// sourceQueryParams parses the search flags shared by the booru and DeviantArt
// subcommands and returns the remaining positional arguments.
func sourceQueryParams(args []string) (wallpapersrc.SearchQuery, []string) {
	site, args := parseFlag(args, "--site")
	pageStr, args := parseFlag(args, "--page")
	limitStr, args := parseFlag(args, "--limit")
	minWidthStr, args := parseFlag(args, "--min-width")
	minHeightStr, args := parseFlag(args, "--min-height")
	safeOnly, args := hasFlag(args, "--safe")

	var query wallpapersrc.SearchQuery
	query.Site = site
	query.SafeOnly = safeOnly
	if v, err := strconv.Atoi(pageStr); err == nil && v > 0 {
		query.Page = v
	}
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		query.Limit = v
	}
	if v, err := strconv.Atoi(minWidthStr); err == nil && v > 0 {
		query.MinWidth = v
	}
	if v, err := strconv.Atoi(minHeightStr); err == nil && v > 0 {
		query.MinHeight = v
	}

	return query, args
}

// booruCredentials resolves booru credentials from the saved config, letting
// explicit flags win.
func booruCredentials(apiKey, userID string) (string, string) {
	config := readSourceConfig("booru.json")
	if apiKey == "" {
		apiKey, _ = config["apiKey"].(string)
	}
	if userID == "" {
		userID, _ = config["userID"].(string)
	}
	return apiKey, userID
}

// deviantArtCredentials resolves OAuth credentials from the saved config,
// letting explicit flags win.
func deviantArtCredentials(clientID, clientSecret string) (string, string) {
	config := readSourceConfig("deviantart.json")
	if clientID == "" {
		clientID, _ = config["clientId"].(string)
	}
	if clientSecret == "" {
		clientSecret, _ = config["clientSecret"].(string)
	}
	return clientID, clientSecret
}

// readSourceConfig loads a provider config file, returning nil when absent or
// unreadable — missing credentials surface as provider errors instead.
func readSourceConfig(filename string) map[string]interface{} {
	config, err := platform.ReadJSON[map[string]interface{}](filepath.Join(platform.ConfigDir(), filename))
	if err != nil {
		return nil
	}
	return config
}

// newBooruClient builds a booru client with the resolved credentials.
func newBooruClient(apiKey, userID string) *booru.Client {
	client := booru.NewClient()
	apiKey, userID = booruCredentials(apiKey, userID)
	client.SetCredentials(apiKey, userID)
	return client
}

// newDeviantArtClient builds a DeviantArt client with the resolved credentials.
func newDeviantArtClient(clientID, clientSecret string) *deviantart.Client {
	client := deviantart.NewClient()
	clientID, clientSecret = deviantArtCredentials(clientID, clientSecret)
	client.SetCredentials(clientID, clientSecret)
	return client
}

// sourceThumbRequest carries the resolved client wiring for a thumbnail run.
type sourceThumbRequest struct {
	query  wallpapersrc.SearchQuery
	search func(wallpapersrc.SearchQuery) (*wallpapersrc.SearchResult, error)
	thumb  func(string) (string, error)
}

func runSearchBooru(args []string) int {
	jsonOut, args := stripJSON(args)
	apiKey, args := parseFlag(args, "--api-key")
	userID, args := parseFlag(args, "--user-id")
	query, args := sourceQueryParams(args)

	if len(args) == 0 {
		return sourceUsageError(jsonOut,
			"Usage: aether --search-booru <tags> [--site safebooru|danbooru|konachan|gelbooru] [--min-width 3840] [--page 1] [--limit 24] [--safe]")
	}
	query.Query = args[0]
	if query.Site != "" && !booru.IsSite(query.Site) {
		return sourceUsageError(jsonOut, fmt.Sprintf("Unknown booru site %q (supported: %v)", query.Site, booru.Sites()))
	}

	result, err := newBooruClient(apiKey, userID).Search(query)
	if err != nil {
		return sourceSearchError(jsonOut, err)
	}
	return printSourceSearch(result, jsonOut)
}

func runBooruThumbs(args []string) int {
	jsonOut, args := stripJSON(args)
	apiKey, args := parseFlag(args, "--api-key")
	userID, args := parseFlag(args, "--user-id")
	query, args := sourceQueryParams(args)
	if len(args) == 0 {
		return sourceUsageError(jsonOut, "Usage: aether --booru-thumbs <tags> [--site safebooru] [--min-width 3840] [--page 1] [--limit 24]")
	}
	query.Query = args[0]
	if query.Site != "" && !booru.IsSite(query.Site) {
		return sourceUsageError(jsonOut, fmt.Sprintf("Unknown booru site %q (supported: %v)", query.Site, booru.Sites()))
	}

	client := newBooruClient(apiKey, userID)
	return runSourceThumbs(jsonOut, sourceThumbRequest{
		query:  query,
		search: client.Search,
		thumb:  client.DownloadThumb,
	})
}

func runSearchDeviantArt(args []string) int {
	jsonOut, args := stripJSON(args)
	clientID, args := parseFlag(args, "--client-id")
	clientSecret, args := parseFlag(args, "--client-secret")
	query, args := sourceQueryParams(args)

	if len(args) == 0 {
		return sourceUsageError(jsonOut,
			"Usage: aether --search-deviantart <tag> [--page 1] [--limit 24] [--client-id ID] [--client-secret SECRET]")
	}
	query.Query = args[0]

	result, err := newDeviantArtClient(clientID, clientSecret).Search(query)
	if err != nil {
		return sourceSearchError(jsonOut, err)
	}
	return printSourceSearch(result, jsonOut)
}

func runDeviantArtThumbs(args []string) int {
	jsonOut, args := stripJSON(args)
	clientID, args := parseFlag(args, "--client-id")
	clientSecret, args := parseFlag(args, "--client-secret")
	query, args := sourceQueryParams(args)
	if len(args) == 0 {
		return sourceUsageError(jsonOut, "Usage: aether --deviantart-thumbs <tag> [--page 1] [--limit 24]")
	}
	query.Query = args[0]

	client := newDeviantArtClient(clientID, clientSecret)
	return runSourceThumbs(jsonOut, sourceThumbRequest{
		query:  query,
		search: client.Search,
		thumb:  client.DownloadThumb,
	})
}

// runSourceThumbs searches, downloads the preview images concurrently, then
// prints the rows. Thumbnail failures leave the row without a cached path.
func runSourceThumbs(jsonOut bool, request sourceThumbRequest) int {
	result, err := request.search(request.query)
	if err != nil {
		return sourceSearchError(jsonOut, err)
	}

	out := make([]sourceThumbEntry, len(result.Data))
	// Mirror the wallhaven thumbnail command: cap in-flight downloads at 8 so a
	// page finishes in a few batches without hammering the provider CDN.
	const workers = 8
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i, hit := range result.Data {
		out[i] = sourceThumbEntry{
			ID:         hit.ID,
			Site:       hit.Site,
			Source:     hit.Source,
			Title:      hit.Title,
			Author:     hit.Author,
			URL:        hit.PageURL,
			Path:       hit.ImageURL,
			Resolution: hit.Resolution,
			ThumbURL:   hit.PreviewURL,
		}
		if hit.PreviewURL == "" {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, url string) {
			defer wg.Done()
			defer func() { <-sem }()
			if path, err := request.thumb(url); err == nil {
				out[idx].ThumbnailPath = path
			}
		}(i, hit.PreviewURL)
	}
	wg.Wait()

	if jsonOut {
		return printJSON(map[string]interface{}{
			"wallpapers": out,
			"meta":       result.Meta,
		})
	}

	fmt.Printf("Results: %d (page %d/%d, total %d)\n",
		len(out), result.Meta.CurrentPage, result.Meta.LastPage, result.Meta.Total)
	for _, entry := range out {
		fmt.Printf("  %s  %s  %s\n", entry.ID, entry.Resolution, entry.ThumbnailPath)
	}
	return 0
}

// printSourceSearch renders a normalized search result.
func printSourceSearch(result *wallpapersrc.SearchResult, jsonOut bool) int {
	if jsonOut {
		return printJSON(result)
	}

	fmt.Printf("Results: %d (page %d/%d, total %d)\n",
		len(result.Data), result.Meta.CurrentPage, result.Meta.LastPage, result.Meta.Total)
	for _, hit := range result.Data {
		fmt.Printf("  %s  %s  %s\n", hit.ID, hit.Resolution, hit.PageURL)
	}
	return 0
}

// sourceSearchError reports a provider search failure.
func sourceSearchError(jsonOut bool, err error) int {
	msg := fmt.Sprintf("Search failed: %v", err)
	if jsonOut {
		return printErrorJSON(msg)
	}
	fmt.Fprintln(os.Stderr, "Error:", msg)
	return 1
}

// sourceUsageError prints a usage or validation failure in the requested format.
func sourceUsageError(jsonOut bool, message string) int {
	if jsonOut {
		return printErrorJSON(message)
	}
	fmt.Fprintln(os.Stderr, message)
	return 1
}
