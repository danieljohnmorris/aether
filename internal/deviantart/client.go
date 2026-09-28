// Package deviantart implements a wallpaper provider backed by DeviantArt's
// official OAuth2 API. It authenticates with the client_credentials grant
// (no browser interaction) and searches the public tag browse endpoint.
package deviantart

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
	"strconv"
	"strings"
	"sync"
	"time"

	"aether/internal/platform"
	"aether/internal/wallpaper"
	"aether/internal/wallpapersrc"
)

const (
	tokenURL      = "https://www.deviantart.com/oauth2/token"
	browseTagsURL = "https://www.deviantart.com/api/v1/oauth2/browse/tags"

	maxAPIResponseBytes int64 = 4 << 20
	maxErrorBytes       int64 = 4 << 10
	maxThumbnailBytes   int64 = 5 << 20

	defaultLimit = 24
	maxLimit     = 50
	maxOffset    = 50000

	defaultTokenTTL   = 3600
	tokenSafetyMargin = 60 * time.Second

	requestTimeout  = 30 * time.Second
	downloadTimeout = 5 * time.Minute
)

// Client is an HTTP client for the DeviantArt API.
type Client struct {
	http *http.Client

	mu           sync.Mutex
	clientID     string
	clientSecret string
	token        string
	tokenExpiry  time.Time
}

// NewClient creates a new DeviantArt API client.
func NewClient() *Client {
	client := wallpaper.NewPublicHTTPClient()
	client.Timeout = requestTimeout
	return &Client{http: client}
}

// SetCredentials sets the OAuth2 client credentials used for the
// client_credentials token grant.
func (c *Client) SetCredentials(clientID, clientSecret string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientID = strings.TrimSpace(clientID)
	c.clientSecret = strings.TrimSpace(clientSecret)
}

// HasCredentials reports whether both a client ID and secret are configured.
func (c *Client) HasCredentials() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clientID != "" && c.clientSecret != ""
}

// Search browses DeviantArt by tag and returns the normalized results.
func (c *Client) Search(q wallpapersrc.SearchQuery) (*wallpapersrc.SearchResult, error) {
	tag := strings.TrimSpace(q.Query)
	if tag == "" {
		return nil, fmt.Errorf("a DeviantArt tag is required")
	}
	if !c.HasCredentials() {
		return nil, fmt.Errorf("DeviantArt client ID and secret are required (Settings → DeviantArt)")
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	} else if limit > maxLimit {
		limit = maxLimit
	}

	page := q.Page
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * limit
	if offset > maxOffset {
		offset = maxOffset
	}

	params := url.Values{}
	params.Set("tag", tag)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(offset))

	var payload tagsResponse
	if err := c.browseGet(browseTagsURL+"?"+params.Encode(), &payload); err != nil {
		return nil, err
	}
	if payload.ErrorCode.failed() {
		msg := payload.ErrorDescription
		if msg == "" {
			msg = payload.Error
		}
		return nil, fmt.Errorf("DeviantArt API error %s: %s", payload.ErrorCode, msg)
	}

	hits := make([]wallpapersrc.Hit, 0, len(payload.Results))
	for _, d := range payload.Results {
		hit, ok := mapHit(d)
		if !ok {
			continue
		}
		hits = append(hits, hit)
	}

	total := payload.EstimatedTotal
	lastPage := 0
	if total > 0 {
		lastPage = (total + limit - 1) / limit
	}
	if lastPage < 1 {
		lastPage = 1
	}

	return &wallpapersrc.SearchResult{
		Data: hits,
		Meta: wallpapersrc.PageMeta{
			CurrentPage: page,
			LastPage:    lastPage,
			Total:       total,
			PerPage:     len(hits),
		},
	}, nil
}

// mapHit normalizes one deviation into a Hit. It reports false when the result
// lacks a usable page or image URL.
func mapHit(d deviation) (wallpapersrc.Hit, bool) {
	if d.URL == "" {
		return wallpapersrc.Hit{}, false
	}
	if err := wallpaper.ValidateRemoteURL(d.URL); err != nil {
		return wallpapersrc.Hit{}, false
	}

	imageURL := d.Content.Src
	if imageURL == "" {
		imageURL = d.Preview.Src
	}
	if imageURL == "" {
		return wallpapersrc.Hit{}, false
	}
	if err := wallpaper.ValidateRemoteURL(imageURL); err != nil {
		return wallpapersrc.Hit{}, false
	}

	previewURL := d.Preview.Src
	if previewURL == "" && len(d.Thumbs) > 0 {
		previewURL = d.Thumbs[0].Src
	}

	width, height := d.Content.Width, d.Content.Height
	if width <= 0 || height <= 0 {
		width, height = d.Preview.Width, d.Preview.Height
	}

	rating := ""
	if d.IsMature {
		rating = "mature"
	}

	return wallpapersrc.Hit{
		ID:         d.DeviationID,
		Source:     wallpapersrc.SourceDeviantArt,
		Site:       "deviantart",
		Title:      d.Title,
		Author:     d.Author.Username,
		PageURL:    d.URL,
		ImageURL:   imageURL,
		PreviewURL: previewURL,
		Width:      width,
		Height:     height,
		Resolution: wallpapersrc.Resolution(width, height),
		FileSize:   d.Content.Filesize,
		Rating:     rating,
	}, true
}

// Download downloads a wallpaper image to the local downloads directory.
// Returns the local file path.
func (c *Client) Download(imageURL string) (string, error) {
	return c.DownloadContext(context.Background(), imageURL)
}

// DownloadContext downloads a wallpaper with cancellation and the shared
// network limits. Returns the local file path.
func (c *Client) DownloadContext(ctx context.Context, imageURL string) (string, error) {
	return c.downloadContext(ctx, imageURL, platform.DownloadDir(), wallpaper.MaxImageBytes)
}

// DownloadThumb downloads a DeviantArt thumbnail URL into the dedicated cache
// dir under ~/.cache/aether/deviantart-thumbs and returns the local path.
// Cached hits skip the HTTP round-trip.
func (c *Client) DownloadThumb(thumbURL string) (string, error) {
	destDir := filepath.Join(platform.CacheDir(), "deviantart-thumbs")
	return c.download(thumbURL, destDir, maxThumbnailBytes)
}

func (c *Client) download(rawURL, destDir string, maxBytes int64) (string, error) {
	return c.downloadContext(context.Background(), rawURL, destDir, maxBytes)
}

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
	if !isDeviantArtHost(u.Hostname()) {
		sum := sha256.Sum256([]byte(rawURL))
		ext := path.Ext(u.Path)
		if len(ext) > 8 {
			ext = ""
		}
		destPath = filepath.Join(destDir, fmt.Sprintf("%x%s", sum[:8], ext))
	}
	client := *c.http
	if maxBytes == wallpaper.MaxImageBytes {
		client.Timeout = downloadTimeout
	}
	if err := wallpaper.DownloadFileContext(ctx, &client, rawURL, destPath, maxBytes); err != nil {
		return "", err
	}
	return destPath, nil
}

// isDeviantArtHost reports whether host belongs to DeviantArt or its media CDN
// (wixmp.com), which lets a download keep the origin filename instead of a
// hashed one.
func isDeviantArtHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, domain := range []string{"deviantart.com", "deviantart.net", "wixmp.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// accessToken returns a cached bearer token, fetching a fresh one when none is
// cached or the current one is about to expire.
func (c *Client) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.clientID == "" || c.clientSecret == "" {
		return "", fmt.Errorf("DeviantArt client ID and secret are required (Settings → DeviantArt)")
	}
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}
	return c.fetchTokenLocked()
}

// fetchTokenLocked performs the client_credentials token grant. Callers must
// hold c.mu.
func (c *Client) fetchTokenLocked() (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create DeviantArt token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("DeviantArt token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		return "", fmt.Errorf("DeviantArt token request returned %d: %s", resp.StatusCode, body)
	}

	body, err := readBoundedBody(resp)
	if err != nil {
		return "", err
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("failed to decode DeviantArt token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("DeviantArt token response contained no access token")
	}

	ttl := tok.ExpiresIn
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}
	c.token = tok.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(ttl)*time.Second - tokenSafetyMargin)
	return c.token, nil
}

// dropToken clears the cached token so the next request re-authenticates.
func (c *Client) dropToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
	c.tokenExpiry = time.Time{}
}

// browseGet performs an authenticated GET against the DeviantArt API. A 401 or
// 403 is reported distinctly and clears the cached token.
func (c *Client) browseGet(reqURL string, result any) error {
	token, err := c.accessToken()
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create DeviantArt request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("DeviantArt API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.dropToken()
		return fmt.Errorf("DeviantArt rejected the credentials (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		return fmt.Errorf("DeviantArt API returned %d: %s", resp.StatusCode, body)
	}

	body, err := readBoundedBody(resp)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to decode DeviantArt response: %w", err)
	}
	return nil
}

// readBoundedBody reads an API response through a 4 MiB LimitReader.
func readBoundedBody(resp *http.Response) ([]byte, error) {
	if resp.ContentLength > maxAPIResponseBytes {
		return nil, fmt.Errorf("DeviantArt response exceeds %d-byte limit", maxAPIResponseBytes)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read DeviantArt response: %w", err)
	}
	if int64(len(body)) > maxAPIResponseBytes {
		return nil, fmt.Errorf("DeviantArt response exceeds %d-byte limit", maxAPIResponseBytes)
	}
	return body, nil
}
