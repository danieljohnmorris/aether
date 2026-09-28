// Package booru implements the Danbooru-family wallpaper catalog provider. One
// client speaks to four boorus: safebooru and danbooru (Danbooru engine),
// konachan (Moebooru engine) and gelbooru (gelbooru dapi).
package booru

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"aether/internal/platform"
	"aether/internal/wallpaper"
	"aether/internal/wallpapersrc"
)

const (
	// maxResponseBytes bounds the JSON body read back from a booru listing.
	maxResponseBytes int64 = 4 << 20
	// maxErrorBytes bounds the error body echoed back to the caller.
	maxErrorBytes int64 = 4 << 10
	// maxThumbBytes bounds a thumbnail download.
	maxThumbBytes int64 = 5 << 20
	// imageTimeout bounds a full-size image download.
	imageTimeout = 5 * time.Minute
)

// Client is an HTTP client for the supported booru catalogs.
type Client struct {
	http   *http.Client
	apiKey string
	userID string
}

// NewClient creates a new booru client with the shared public-network policy.
func NewClient() *Client {
	client := wallpaper.NewPublicHTTPClient()
	client.Timeout = 30 * time.Second
	return &Client{http: client}
}

// SetCredentials sets the default gelbooru/danbooru API credentials. An empty
// value leaves that field unset.
func (c *Client) SetCredentials(apiKey, userID string) {
	c.apiKey = apiKey
	c.userID = userID
}

// Sites returns the sorted ids of the supported booru catalogs.
func Sites() []string {
	ids := make([]string, 0, len(sites))
	for id := range sites {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// IsSite reports whether site is a supported booru catalog id.
func IsSite(site string) bool {
	_, ok := sites[site]
	return ok
}

// Search queries the selected booru and normalizes the posts into Hits. An
// empty Site selects safebooru.
//
// Boorus do not report a total count, so PageMeta.Total is always zero and
// PageMeta.LastPage is an estimate: it is CurrentPage+1 when a full page came
// back, and CurrentPage otherwise.
func (c *Client) Search(q wallpapersrc.SearchQuery) (*wallpapersrc.SearchResult, error) {
	siteID := q.Site
	if siteID == "" {
		siteID = wallpapersrc.SiteSafebooru
	}
	s, ok := sites[siteID]
	if !ok {
		return nil, fmt.Errorf("unsupported booru site %q", siteID)
	}

	apiKey := q.APIKey
	if apiKey == "" {
		apiKey = c.apiKey
	}
	userID := q.UserID
	if userID == "" {
		userID = c.userID
	}
	hasCredentials := apiKey != "" || userID != ""

	if s.requireCredentials && (apiKey == "" || userID == "") {
		return nil, fmt.Errorf("%s requires api_key and user_id credentials", s.id)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	page := q.Page
	if page < 1 {
		page = 1
	}

	tags := strings.Fields(q.Query)
	if q.MinWidth > 0 {
		tags = append(tags, fmt.Sprintf("width:>=%d", q.MinWidth))
	}
	if q.MinHeight > 0 {
		tags = append(tags, fmt.Sprintf("height:>=%d", q.MinHeight))
	}
	if q.SafeOnly {
		tags = append(tags, s.ratingTag)
	}

	if s.anonymousBudget > 0 && !hasCredentials && len(tags) > s.anonymousBudget {
		return nil, fmt.Errorf("%s allows %d tags without an API key (got %d): add credentials or drop a filter",
			s.id, s.anonymousBudget, len(tags))
	}

	reqURL := buildListingURL(s, tags, limit, page, apiKey, userID)

	body, err := c.get(reqURL)
	if err != nil {
		return nil, err
	}

	hits, err := decodeHits(s, body)
	if err != nil {
		return nil, err
	}

	lastPage := page
	if len(hits) == limit {
		lastPage = page + 1
	}
	return &wallpapersrc.SearchResult{
		Data: hits,
		Meta: wallpapersrc.PageMeta{
			CurrentPage: page,
			LastPage:    lastPage,
			Total:       0,
			PerPage:     len(hits),
		},
	}, nil
}

// buildListingURL assembles the site-specific listing request URL.
func buildListingURL(s site, tags []string, limit, page int, apiKey, userID string) string {
	tagString := strings.Join(tags, " ")

	switch s.engine {
	case engineGelbooru:
		q := url.Values{}
		q.Set("page", "dapi")
		q.Set("s", "post")
		q.Set("q", "index")
		q.Set("json", "1")
		q.Set("tags", tagString)
		q.Set("limit", strconv.Itoa(limit))
		q.Set("pid", strconv.Itoa(page-1))
		q.Set("api_key", apiKey)
		q.Set("user_id", userID)
		return s.listingURL + "?" + q.Encode()
	default:
		q := url.Values{}
		q.Set("tags", tagString)
		q.Set("limit", strconv.Itoa(limit))
		q.Set("page", strconv.Itoa(page))
		if s.engine == engineDanbooru && apiKey != "" {
			q.Set("login", userID)
			q.Set("api_key", apiKey)
		}
		return s.listingURL + "?" + q.Encode()
	}
}

// decodeHits normalizes the engine-specific payload into Hits.
func decodeHits(s site, body []byte) ([]wallpapersrc.Hit, error) {
	switch s.engine {
	case engineDanbooru:
		var posts []danbooruPost
		if err := json.Unmarshal(body, &posts); err != nil {
			return nil, fmt.Errorf("failed to decode %s response: %w", s.id, err)
		}
		hits := make([]wallpapersrc.Hit, 0, len(posts))
		for _, p := range posts {
			id := string(p.ID)
			imageURL := p.FileURL
			if imageURL == "" {
				continue
			}
			hits = append(hits, wallpapersrc.Hit{
				ID:         id,
				Source:     wallpapersrc.SourceBooru,
				Site:       s.id,
				PageURL:    fmt.Sprintf(s.pageURLFormat, id),
				ImageURL:   imageURL,
				// Prefer the ~850px sample over the 180px preview: the card
				// downscales this image, so a tiny source renders blurry.
				PreviewURL: firstNonEmpty(p.LargeFileURL, p.PreviewFileURL),
				Width:      int(p.ImageWidth),
				Height:     int(p.ImageHeight),
				Resolution: wallpapersrc.Resolution(int(p.ImageWidth), int(p.ImageHeight)),
				FileSize:   int64(p.FileSize),
				Rating:     p.Rating,
				Tags:       strings.Fields(p.TagString),
			})
		}
		return hits, nil
	case engineMoebooru:
		var posts []moebooruPost
		if err := json.Unmarshal(body, &posts); err != nil {
			return nil, fmt.Errorf("failed to decode %s response: %w", s.id, err)
		}
		hits := make([]wallpapersrc.Hit, 0, len(posts))
		for _, p := range posts {
			id := string(p.ID)
			imageURL := p.FileURL
			if imageURL == "" {
				continue
			}
			hits = append(hits, wallpapersrc.Hit{
				ID:         id,
				Source:     wallpapersrc.SourceBooru,
				Site:       s.id,
				PageURL:    fmt.Sprintf(s.pageURLFormat, id),
				ImageURL:   imageURL,
				PreviewURL: firstNonEmpty(p.SampleURL, p.PreviewURL, p.JPEGURL),
				Width:      int(p.Width),
				Height:     int(p.Height),
				Resolution: wallpapersrc.Resolution(int(p.Width), int(p.Height)),
				FileSize:   int64(p.FileSize),
				Rating:     p.Rating,
				Tags:       strings.Fields(p.Tags),
			})
		}
		return hits, nil
	case engineGelbooru:
		var resp gelbooruResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to decode %s response: %w", s.id, err)
		}
		hits := make([]wallpapersrc.Hit, 0, len(resp.Post))
		for _, p := range resp.Post {
			id := string(p.ID)
			imageURL := p.FileURL
			if imageURL == "" {
				continue
			}
			hits = append(hits, wallpapersrc.Hit{
				ID:         id,
				Source:     wallpapersrc.SourceBooru,
				Site:       s.id,
				PageURL:    fmt.Sprintf(s.pageURLFormat, id),
				ImageURL:   imageURL,
				PreviewURL: firstNonEmpty(p.SampleURL, p.PreviewURL),
				Width:      int(p.Width),
				Height:     int(p.Height),
				Resolution: wallpapersrc.Resolution(int(p.Width), int(p.Height)),
				FileSize:   int64(p.FileSize),
				Rating:     p.Rating,
				Tags:       strings.Fields(p.Tags),
			})
		}
		return hits, nil
	default:
		return nil, fmt.Errorf("unsupported booru engine for site %q", s.id)
	}
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Download downloads a booru image into the shared downloads directory.
func (c *Client) Download(imageURL string) (string, error) {
	return c.DownloadContext(context.Background(), imageURL)
}

// DownloadContext downloads a booru image with cancellation.
func (c *Client) DownloadContext(ctx context.Context, imageURL string) (string, error) {
	return c.downloadContext(ctx, imageURL, platform.DownloadDir(), wallpaper.MaxImageBytes)
}

// DownloadThumb downloads a booru thumbnail into the provider's cache directory,
// reusing an existing file when one is already cached.
func (c *Client) DownloadThumb(thumbURL string) (string, error) {
	destDir := filepath.Join(platform.CacheDir(), "booru-thumbs")
	return c.downloadContext(context.Background(), thumbURL, destDir, maxThumbBytes)
}

// downloadContext validates, names and fetches one remote file.
func (c *Client) downloadContext(ctx context.Context, rawURL, destDir string, maxBytes int64) (string, error) {
	if err := wallpaper.ValidateRemoteURL(rawURL); err != nil {
		return "", err
	}
	u, _ := url.Parse(rawURL)
	filename := path.Base(u.Path)
	if filename == "." || filename == ".." || filename == "/" ||
		strings.ContainsAny(filename, "\\\x00") {
		return "", fmt.Errorf("invalid download filename")
	}
	destPath := filepath.Join(destDir, filename)
	host := strings.ToLower(u.Hostname())
	if !hostWithin(host, booruDomains) {
		sum := sha256.Sum256([]byte(rawURL))
		ext := path.Ext(u.Path)
		if len(ext) > 8 {
			ext = ""
		}
		destPath = filepath.Join(destDir, fmt.Sprintf("%x%s", sum[:8], ext))
	}
	client := *c.http
	if maxBytes == wallpaper.MaxImageBytes {
		client.Timeout = imageTimeout
	}
	if err := wallpaper.DownloadFileContext(ctx, &client, rawURL, destPath, maxBytes); err != nil {
		return "", err
	}
	return destPath, nil
}

// hostWithin reports whether host equals or is a subdomain of any of domains.
func hostWithin(host string, domains []string) bool {
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// get performs one listing request and returns the bounded JSON body.
func (c *Client) get(reqURL string) ([]byte, error) {
	resp, err := c.http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("booru API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		return nil, fmt.Errorf("booru API returned %d: %s", resp.StatusCode, body)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read booru response: %w", err)
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, fmt.Errorf("booru response exceeds %d-byte limit", maxResponseBytes)
	}
	return body, nil
}
