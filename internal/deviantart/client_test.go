package deviantart

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aether/internal/platform"
	"aether/internal/wallpapersrc"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// respond builds a synthetic HTTP response for the injected transport.
func respond(status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func TestSearchRequiresCredentialsAndTag(t *testing.T) {
	c := NewClient()
	c.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("request issued without credentials")
		return nil, errors.New("unexpected network request")
	})

	if c.HasCredentials() {
		t.Fatal("fresh client must not report credentials")
	}
	if _, err := c.Search(wallpapersrc.SearchQuery{Query: "cats"}); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected missing-credentials error, got %v", err)
	}

	c.SetCredentials("id", "secret")
	if !c.HasCredentials() {
		t.Fatal("client must report configured credentials")
	}
	if _, err := c.Search(wallpapersrc.SearchQuery{Query: "   "}); err == nil || !strings.Contains(err.Error(), "tag is required") {
		t.Fatalf("expected empty-tag error, got %v", err)
	}
}

func TestSearchTokenAndBrowseRequest(t *testing.T) {
	c := NewClient()
	c.SetCredentials("my-id", "my-secret")

	var tokenReqs, browseReqs int
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Hostname() == "www.deviantart.com" && r.URL.Path == "/oauth2/token":
			tokenReqs++
			if r.Method != http.MethodPost {
				t.Errorf("token request method = %s, want POST", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("token request must not carry Authorization, got %q", got)
			}
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read token body: %v", err)
			}
			form, err := url.ParseQuery(string(raw))
			if err != nil {
				t.Fatalf("parse token body: %v", err)
			}
			if form.Get("grant_type") != "client_credentials" {
				t.Errorf("grant_type = %q", form.Get("grant_type"))
			}
			if form.Get("client_id") != "my-id" || form.Get("client_secret") != "my-secret" {
				t.Errorf("credentials form = %q", string(raw))
			}
			return respond(http.StatusOK, `{"access_token":"tok-1","token_type":"Bearer","expires_in":3600,"status":"success"}`), nil

		case r.URL.Hostname() == "www.deviantart.com" && r.URL.Path == "/api/v1/oauth2/browse/tags":
			browseReqs++
			if got := r.Header.Get("Authorization"); got != "Bearer tok-1" {
				t.Errorf("browse Authorization = %q, want Bearer tok-1", got)
			}
			q := r.URL.Query()
			if q.Get("tag") != "cyberpunk cityscape" {
				t.Errorf("tag = %q", q.Get("tag"))
			}
			if q.Get("limit") != "24" || q.Get("offset") != "0" {
				t.Errorf("limit/offset = %q/%q", q.Get("limit"), q.Get("offset"))
			}
			return respond(http.StatusOK, `{"estimated_total":1,"results":[{"deviationid":"d1","url":"https://www.deviantart.com/a/art/X","title":"X","author":{"username":"artist"},"content":{"src":"https://images-wixmp-ed30a86b8c4ca887773594c2.wixmp.com/f/a/b.jpg","width":1920,"height":1080,"filesize":1234},"preview":{"src":"https://images-wixmp-ed30a86b8c4ca887773594c2.wixmp.com/f/a/p.jpg","width":400,"height":225},"is_mature":false}]}`), nil
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		return nil, errors.New("unexpected network request")
	})

	res, err := c.Search(wallpapersrc.SearchQuery{Query: "  cyberpunk cityscape  "})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if tokenReqs != 1 || browseReqs != 1 {
		t.Fatalf("token requests = %d, browse requests = %d", tokenReqs, browseReqs)
	}
	if len(res.Data) != 1 {
		t.Fatalf("hits = %d", len(res.Data))
	}
	h := res.Data[0]
	if h.ID != "d1" || h.Source != wallpapersrc.SourceDeviantArt || h.Site != "deviantart" {
		t.Errorf("identity = %+v", h)
	}
	if h.Author != "artist" || h.Title != "X" || h.PageURL != "https://www.deviantart.com/a/art/X" {
		t.Errorf("metadata = %+v", h)
	}
	if h.ImageURL != "https://images-wixmp-ed30a86b8c4ca887773594c2.wixmp.com/f/a/b.jpg" {
		t.Errorf("ImageURL = %q", h.ImageURL)
	}
	if h.Width != 1920 || h.Height != 1080 || h.Resolution != "1920x1080" || h.FileSize != 1234 {
		t.Errorf("dimensions = %+v", h)
	}
	if h.Rating != "" || h.Tags != nil {
		t.Errorf("rating/tags = %q/%v", h.Rating, h.Tags)
	}
	if res.Meta.CurrentPage != 1 || res.Meta.LastPage != 1 || res.Meta.Total != 1 || res.Meta.PerPage != 1 {
		t.Errorf("meta = %+v", res.Meta)
	}
}

func TestSearchReusesToken(t *testing.T) {
	c := NewClient()
	c.SetCredentials("id", "secret")
	tokenReqs := 0
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/oauth2/token" {
			tokenReqs++
			return respond(http.StatusOK, `{"access_token":"tok","expires_in":3600}`), nil
		}
		return respond(http.StatusOK, `{"estimated_total":0,"results":[]}`), nil
	})

	for i := 0; i < 2; i++ {
		if _, err := c.Search(wallpapersrc.SearchQuery{Query: "cats"}); err != nil {
			t.Fatalf("Search %d: %v", i, err)
		}
	}
	if tokenReqs != 1 {
		t.Fatalf("token requests = %d, want 1", tokenReqs)
	}
}

func TestSearchRefreshesTokenAfter401(t *testing.T) {
	c := NewClient()
	c.SetCredentials("id", "secret")
	tokenReqs := 0
	var browseAuth []string
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/oauth2/token" {
			tokenReqs++
			return respond(http.StatusOK, fmt.Sprintf(`{"access_token":"tok-%d","expires_in":3600}`, tokenReqs)), nil
		}
		browseAuth = append(browseAuth, r.Header.Get("Authorization"))
		if len(browseAuth) == 1 {
			return respond(http.StatusUnauthorized, `{"error":"invalid_token"}`), nil
		}
		return respond(http.StatusOK, `{"estimated_total":0,"results":[]}`), nil
	})

	_, err := c.Search(wallpapersrc.SearchQuery{Query: "cats"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("expected distinct 401 error, got %v", err)
	}
	if _, err := c.Search(wallpapersrc.SearchQuery{Query: "cats"}); err != nil {
		t.Fatalf("second Search: %v", err)
	}
	if tokenReqs != 2 {
		t.Fatalf("token requests = %d, want 2 (refetch after 401)", tokenReqs)
	}
	if len(browseAuth) != 2 || browseAuth[1] != "Bearer tok-2" {
		t.Fatalf("browse auth headers = %v", browseAuth)
	}
}

func TestSearchRejectsNonZeroErrorCode(t *testing.T) {
	c := NewClient()
	c.SetCredentials("id", "secret")
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/oauth2/token" {
			return respond(http.StatusOK, `{"access_token":"tok","expires_in":3600}`), nil
		}
		return respond(http.StatusOK, `{"error_code":4,"error_description":"bad tag","results":[]}`), nil
	})
	if _, err := c.Search(wallpapersrc.SearchQuery{Query: "cats"}); err == nil || !strings.Contains(err.Error(), "bad tag") {
		t.Fatalf("expected error_code failure, got %v", err)
	}
}

func TestSearchClampsLimitAndOffset(t *testing.T) {
	c := NewClient()
	c.SetCredentials("id", "secret")

	// captureLimit returns the limit/offset seen by the browse endpoint.
	captureLimit := func() (*string, *string) {
		var seenLimit, seenOffset string
		c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/oauth2/token" {
				return respond(http.StatusOK, `{"access_token":"tok","expires_in":3600}`), nil
			}
			seenLimit = r.URL.Query().Get("limit")
			seenOffset = r.URL.Query().Get("offset")
			return respond(http.StatusOK, `{"estimated_total":0,"results":[]}`), nil
		})
		return &seenLimit, &seenOffset
	}

	for _, tc := range []struct {
		page, limit       int
		wantLimit, wantOff string
	}{
		{0, 0, "24", "0"},          // defaults
		{-3, -9, "24", "0"},        // clamped up
		{0, 999, "50", "0"},        // clamped down
		{3, 10, "10", "20"},        // page math
		{100000, 50, "50", "50000"}, // offset ceiling
	} {
		l, o := captureLimit()
		if _, err := c.Search(wallpapersrc.SearchQuery{Query: "x", Page: tc.page, Limit: tc.limit}); err != nil {
			t.Fatalf("Search(%+v): %v", tc, err)
		}
		if *l != tc.wantLimit || *o != tc.wantOff {
			t.Errorf("page=%d limit=%d: limit=%s offset=%s, want %s/%s", tc.page, tc.limit, *l, *o, tc.wantLimit, tc.wantOff)
		}
	}
}

func TestSearchHitMappingFallbacksAndSkips(t *testing.T) {
	c := NewClient()
	c.SetCredentials("id", "secret")
	body := `{"estimated_total":100,"results":[
		{"deviationid":"ok","url":"https://www.deviantart.com/a","title":"T","author":{"username":"u"},
		 "content":{"src":"https://images.example.com/full.jpg","width":2560,"height":1440,"filesize":900},
		 "preview":{"src":"https://images.example.com/preview.jpg","width":400,"height":225},"is_mature":true},
		{"deviationid":"preview-fallback","url":"https://www.deviantart.com/b","title":"B","author":{"username":"u2"},
		 "content":{"src":"","width":0,"height":0},
		 "preview":{"src":"https://images.example.com/p2.jpg","width":800,"height":600}},
		{"deviationid":"thumb-fallback","url":"https://www.deviantart.com/c",
		 "content":{"src":"https://images.example.com/c.jpg","width":100,"height":50},
		 "preview":{"src":""},"thumbs":[{"src":"https://images.example.com/t.jpg","width":100,"height":50}]},
		{"deviationid":"no-url","url":"","content":{"src":"https://images.example.com/x.jpg"},"preview":{}},
		{"deviationid":"no-image","url":"https://www.deviantart.com/d","content":{"src":""},"preview":{"src":""}},
		{"deviationid":"bad-image","url":"https://www.deviantart.com/e","content":{"src":"http://insecure.example.com/y.jpg"}}
	]}`
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/oauth2/token" {
			return respond(http.StatusOK, `{"access_token":"tok","expires_in":3600}`), nil
		}
		return respond(http.StatusOK, body), nil
	})

	res, err := c.Search(wallpapersrc.SearchQuery{Query: "x"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Data) != 3 {
		t.Fatalf("hits = %d, want 3 (invalid results skipped): %+v", len(res.Data), res.Data)
	}

	first := res.Data[0]
	if first.ImageURL != "https://images.example.com/full.jpg" || first.PreviewURL != "https://images.example.com/preview.jpg" {
		t.Errorf("first hit URLs = %q / %q", first.ImageURL, first.PreviewURL)
	}
	if first.Rating != "mature" {
		t.Errorf("first hit rating = %q, want mature", first.Rating)
	}
	if first.Resolution != "2560x1440" || first.FileSize != 900 {
		t.Errorf("first hit geometry = %+v", first)
	}

	second := res.Data[1]
	if second.ImageURL != "https://images.example.com/p2.jpg" {
		t.Errorf("content->preview fallback failed: %q", second.ImageURL)
	}
	if second.Width != 800 || second.Height != 600 || second.Resolution != "800x600" {
		t.Errorf("dimension fallback failed: %+v", second)
	}

	third := res.Data[2]
	if third.PreviewURL != "https://images.example.com/t.jpg" {
		t.Errorf("thumb fallback failed: %q", third.PreviewURL)
	}

	if res.Meta.Total != 100 || res.Meta.LastPage != 5 || res.Meta.PerPage != 3 || res.Meta.CurrentPage != 1 {
		t.Errorf("meta = %+v", res.Meta)
	}
}

func TestRejectUnsafeDownloadURLs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	c := NewClient()
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("unsafe input reached HTTP transport")
		return nil, errors.New("unexpected network request")
	})

	for _, rawURL := range []string{
		"", "file:///image.jpg", "http://example.com/image.jpg", "https://user@example.com/image.jpg",
		"https://127.0.0.1/image.jpg", "https://192.168.1.1/image.jpg", "https://[::1]/image.jpg",
		"https://169.254.169.254/image.jpg", "https://localhost/image.jpg",
		"https://example.com/", "https://example.com/..", "https://example.com/%5Cname.jpg",
		"https://example.com/%00name.jpg",
	} {
		for _, call := range []func(string) (string, error){c.Download, c.DownloadThumb} {
			if _, err := call(rawURL); err == nil {
				t.Errorf("accepted unsafe URL %q", rawURL)
			}
		}
	}
}

func TestDownloadFilenameDerivation(t *testing.T) {
	dataHome := t.TempDir()
	cacheHome := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	if err := os.MkdirAll(platform.DownloadDir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(platform.CacheDir(), "deviantart-thumbs"), 0755); err != nil {
		t.Fatal(err)
	}

	c := NewClient()
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("img")), ContentLength: 3}, nil
	})

	own, err := c.Download("https://images-wixmp-ed30a86b8c4ca887773594c2.wixmp.com/f/a/photo.jpg")
	if err != nil {
		t.Fatalf("own-domain Download: %v", err)
	}
	if filepath.Base(own) != "photo.jpg" {
		t.Errorf("provider host filename = %q, want photo.jpg", filepath.Base(own))
	}

	foreign, err := c.Download("https://cdn.example.com/photo.jpg")
	if err != nil {
		t.Fatalf("foreign Download: %v", err)
	}
	if base := filepath.Base(foreign); base == "photo.jpg" || !strings.HasSuffix(base, ".jpg") {
		t.Errorf("foreign host filename = %q, want hashed .jpg name", base)
	}
}
