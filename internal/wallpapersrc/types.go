// Package wallpapersrc holds the result types shared by the external wallpaper
// providers (booru catalogs, DeviantArt). Providers normalize their own API
// payloads into Hit so the frontend and CLI can render every source with the
// same card and the same paging logic.
package wallpapersrc

import "fmt"

// Source identifiers. These are the values the frontend passes back when it
// downloads or favorites a result, and the values DownloadProviderWallpaper
// dispatches on.
const (
	SourceBooru      = "booru"
	SourceDeviantArt = "deviantart"
)

// Booru catalog identifiers.
const (
	SiteSafebooru = "safebooru"
	SiteDanbooru  = "danbooru"
	SiteKonachan  = "konachan"
	SiteGelbooru  = "gelbooru"
)

// SearchQuery is the provider-independent search request. Providers ignore
// fields they cannot express server-side.
type SearchQuery struct {
	// Query is free-form: keywords for DeviantArt, a tag list for boorus
	// (e.g. "cyberpunk cityscape").
	Query string `json:"query"`
	// Site selects a booru catalog; empty means the provider default.
	Site string `json:"site"`
	// Page is 1-based.
	Page  int `json:"page"`
	Limit int `json:"limit"`
	// MinWidth/MinHeight are appended as a resolution filter. Boorus enforce
	// them as tags, so they consume part of the anonymous tag budget.
	MinWidth  int  `json:"minWidth"`
	MinHeight int  `json:"minHeight"`
	SafeOnly  bool `json:"safeOnly"`
	// APIKey and UserID are optional per-request credential overrides. An empty
	// value falls back to the client's configured credentials.
	APIKey string `json:"apiKey,omitempty"`
	UserID string `json:"userID,omitempty"`
}

// Hit is one normalized provider result.
type Hit struct {
	ID         string   `json:"id"`
	Source     string   `json:"source"`
	Site       string   `json:"site"`
	Title      string   `json:"title,omitempty"`
	Author     string   `json:"author,omitempty"`
	PageURL    string   `json:"pageUrl"`
	ImageURL   string   `json:"imageUrl"`
	PreviewURL string   `json:"previewUrl"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Resolution string   `json:"resolution"`
	FileSize   int64    `json:"fileSize"`
	Rating     string   `json:"rating,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// PageMeta mirrors the pagination fields the frontend store already consumes
// for wallhaven results.
type PageMeta struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	Total       int `json:"total"`
	PerPage     int `json:"per_page"`
}

// SearchResult is the normalized search response.
type SearchResult struct {
	Data []Hit    `json:"data"`
	Meta PageMeta `json:"meta"`
}

// Resolution renders a "WIDTHxHEIGHT" label, or "" when either side is unknown.
func Resolution(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", width, height)
}
