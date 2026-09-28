package booru

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"aether/internal/wallpapersrc"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSearchRequestURLsPerSite(t *testing.T) {
	tests := []struct {
		name      string
		site      string
		query     wallpapersrc.SearchQuery
		creds     bool
		body      string
		wantHost  string
		wantPath  string
		wantQuery map[string]string
	}{
		{
			name:      "safebooru",
			site:      wallpapersrc.SiteSafebooru,
			query:     wallpapersrc.SearchQuery{Query: "city", Page: 2},
			body:      `[]`,
			wantHost:  "safebooru.donmai.us",
			wantPath:  "/posts.json",
			wantQuery: map[string]string{"tags": "city", "limit": "24", "page": "2"},
		},
		{
			name:     "danbooru with credentials",
			site:     wallpapersrc.SiteDanbooru,
			query:    wallpapersrc.SearchQuery{Query: "city night sky", Page: 3, Limit: 100},
			creds:    true,
			body:     `[]`,
			wantHost: "danbooru.donmai.us",
			wantPath: "/posts.json",
			wantQuery: map[string]string{
				"tags": "city night sky", "limit": "100", "page": "3",
				"login": "user 1", "api_key": "key&1",
			},
		},
		{
			name:     "konachan",
			site:     wallpapersrc.SiteKonachan,
			query:    wallpapersrc.SearchQuery{Query: "sky clouds", Limit: 200},
			body:     `[]`,
			wantHost: "konachan.net",
			wantPath: "/post.json",
			wantQuery: map[string]string{
				"tags": "sky clouds", "limit": "100", "page": "1",
			},
		},
		{
			name:     "gelbooru pid is zero based",
			site:     wallpapersrc.SiteGelbooru,
			query:    wallpapersrc.SearchQuery{Query: "city", Page: 2},
			creds:    true,
			body:     `{"post":[]}`,
			wantHost: "gelbooru.com",
			wantPath: "/index.php",
			wantQuery: map[string]string{
				"page": "dapi", "s": "post", "q": "index", "json": "1",
				"tags": "city", "limit": "24", "pid": "1",
				"api_key": "key&1", "user_id": "user 1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient()
			if tc.creds {
				c.SetCredentials("key&1", "user 1")
			}
			var gotURL *url.URL
			c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotURL = r.URL
				return jsonResponse(http.StatusOK, tc.body), nil
			})

			q := tc.query
			q.Site = tc.site
			if _, err := c.Search(q); err != nil {
				t.Fatalf("Search: %v", err)
			}
			if gotURL == nil {
				t.Fatal("no request issued")
			}
			if gotURL.Hostname() != tc.wantHost || gotURL.Path != tc.wantPath {
				t.Fatalf("request = %s%s, want %s%s", gotURL.Hostname(), gotURL.Path, tc.wantHost, tc.wantPath)
			}
			for key, want := range tc.wantQuery {
				if got := gotURL.Query().Get(key); got != want {
					t.Errorf("query %s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestAnonymousTagBudget(t *testing.T) {
	for _, siteID := range []string{wallpapersrc.SiteSafebooru, wallpapersrc.SiteDanbooru} {
		c := NewClient()
		called := false
		c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			return jsonResponse(http.StatusOK, "[]"), nil
		})
		_, err := c.Search(wallpapersrc.SearchQuery{Site: siteID, Query: "cat dog", MinWidth: 1920})
		if err == nil {
			t.Fatalf("%s: expected tag budget error", siteID)
		}
		want := siteID + " allows 2 tags without an API key (got 3)"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %q, want substring %q", siteID, err, want)
		}
		if called {
			t.Errorf("%s: request issued despite budget error", siteID)
		}
	}

	c := NewClient()
	c.SetCredentials("key", "user")
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, "[]"), nil
	})
	if _, err := c.Search(wallpapersrc.SearchQuery{
		Site: wallpapersrc.SiteDanbooru, Query: "cat dog", MinWidth: 1920, MinHeight: 1080, SafeOnly: true,
	}); err != nil {
		t.Fatalf("credentials should lift the budget: %v", err)
	}
}

func TestGelbooruRequiresCredentials(t *testing.T) {
	for _, creds := range [][2]string{{"", ""}, {"key", ""}, {"", "user"}} {
		c := NewClient()
		c.SetCredentials(creds[0], creds[1])
		called := false
		c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			return jsonResponse(http.StatusOK, `{"post":[]}`), nil
		})
		_, err := c.Search(wallpapersrc.SearchQuery{Site: wallpapersrc.SiteGelbooru, Query: "cat"})
		if err == nil || !strings.Contains(err.Error(), "gelbooru requires api_key and user_id") {
			t.Errorf("creds %v: error = %v, want missing-credentials error", creds, err)
		}
		if called {
			t.Errorf("creds %v: request issued despite missing credentials", creds)
		}
	}
}

func TestSearchMapsDanbooruPayload(t *testing.T) {
	c := NewClient()
	payload := `[{
		"id": 123,
		"file_url": "https://cdn.donmai.us/original/ab/abc.jpg",
		"large_file_url": "https://cdn.donmai.us/large/abc.jpg",
		"preview_file_url": "https://cdn.donmai.us/preview/abc.jpg",
		"image_width": 1920,
		"image_height": 1080,
		"file_size": 2048,
		"rating": "g",
		"tag_string": "city night",
		"source": "https://example.com/post"
	}]`
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, payload), nil
	})

	res, err := c.Search(wallpapersrc.SearchQuery{Query: "city"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("got %d hits, want 1", len(res.Data))
	}
	hit := res.Data[0]
	if hit.ID != "123" || hit.Source != wallpapersrc.SourceBooru || hit.Site != wallpapersrc.SiteSafebooru {
		t.Errorf("identity = %q/%q/%q", hit.ID, hit.Source, hit.Site)
	}
	if hit.ImageURL != "https://cdn.donmai.us/original/ab/abc.jpg" {
		t.Errorf("ImageURL = %q", hit.ImageURL)
	}
	if hit.PreviewURL != "https://cdn.donmai.us/large/abc.jpg" {
		t.Errorf("PreviewURL = %q", hit.PreviewURL)
	}
	if hit.PageURL != "https://safebooru.donmai.us/posts/123" {
		t.Errorf("PageURL = %q", hit.PageURL)
	}
	if hit.Width != 1920 || hit.Height != 1080 || hit.Resolution != "1920x1080" {
		t.Errorf("dimensions = %d/%d/%q", hit.Width, hit.Height, hit.Resolution)
	}
	if hit.FileSize != 2048 || hit.Rating != "g" {
		t.Errorf("file/rating = %d/%q", hit.FileSize, hit.Rating)
	}
	if strings.Join(hit.Tags, ",") != "city,night" {
		t.Errorf("Tags = %v", hit.Tags)
	}
	if hit.Author != "" {
		t.Errorf("Author = %q, want empty", hit.Author)
	}
	if res.Meta.CurrentPage != 1 || res.Meta.PerPage != 1 || res.Meta.Total != 0 || res.Meta.LastPage != 1 {
		t.Errorf("meta = %+v", res.Meta)
	}
}

func TestSearchMapsMoebooruPayloadAndFullPage(t *testing.T) {
	c := NewClient()
	payload := `[{
		"id": 42,
		"file_url": "https://konachan.net/image/abc.jpg",
		"preview_url": "https://konachan.net/preview/abc.jpg",
		"sample_url": "https://konachan.net/sample/abc.jpg",
		"jpeg_url": "https://konachan.net/jpeg/abc.jpg",
		"width": 2560,
		"height": 1440,
		"file_size": 4096,
		"tags": "sky clouds",
		"source": "",
		"rating": "safe"
	}]`
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, payload), nil
	})

	res, err := c.Search(wallpapersrc.SearchQuery{Site: wallpapersrc.SiteKonachan, Query: "sky", Limit: 1})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("got %d hits, want 1", len(res.Data))
	}
	hit := res.Data[0]
	if hit.Site != wallpapersrc.SiteKonachan || hit.ID != "42" {
		t.Errorf("identity = %q/%q", hit.Site, hit.ID)
	}
	if hit.PreviewURL != "https://konachan.net/sample/abc.jpg" {
		t.Errorf("PreviewURL = %q", hit.PreviewURL)
	}
	if hit.PageURL != "https://konachan.net/post/show/42" {
		t.Errorf("PageURL = %q", hit.PageURL)
	}
	if hit.Resolution != "2560x1440" || hit.FileSize != 4096 {
		t.Errorf("resolution/size = %q/%d", hit.Resolution, hit.FileSize)
	}
	if strings.Join(hit.Tags, ",") != "sky,clouds" {
		t.Errorf("Tags = %v", hit.Tags)
	}
	// A full page (len == limit) means a next page probably exists.
	if res.Meta.LastPage != 2 {
		t.Errorf("LastPage = %d, want 2 for a full page", res.Meta.LastPage)
	}
}

func TestSearchGelbooruQuotedNumbers(t *testing.T) {
	c := NewClient()
	c.SetCredentials("key", "user")
	payload := `{"@attributes":{"count":1},"post":[{
		"id":"777",
		"file_url":"https://img3.gelbooru.com/images/ab/abc.jpg",
		"preview_url":"https://img3.gelbooru.com/thumbnails/ab/thumb.jpg",
		"sample_url":"https://img3.gelbooru.com/samples/ab/sample.jpg",
		"width":"1600",
		"height":"900",
		"file_size":"512",
		"tags":"city night",
		"rating":"general"
	}]}`
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, payload), nil
	})

	res, err := c.Search(wallpapersrc.SearchQuery{Site: wallpapersrc.SiteGelbooru, Query: "city"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("got %d hits, want 1", len(res.Data))
	}
	hit := res.Data[0]
	if hit.ID != "777" || hit.Width != 1600 || hit.Height != 900 || hit.FileSize != 512 {
		t.Errorf("gelbooru numeric decoding = %q/%d/%d/%d", hit.ID, hit.Width, hit.Height, hit.FileSize)
	}
	if hit.PageURL != "https://gelbooru.com/index.php?page=post&s=view&id=777" {
		t.Errorf("PageURL = %q", hit.PageURL)
	}
	if hit.PreviewURL != "https://img3.gelbooru.com/samples/ab/sample.jpg" {
		t.Errorf("PreviewURL = %q", hit.PreviewURL)
	}
}

func TestSearchSkipsEntriesWithoutImage(t *testing.T) {
	c := NewClient()
	payload := `[
		{"id":1,"file_url":"","preview_file_url":"https://cdn.donmai.us/preview/1.jpg","image_width":100,"image_height":100},
		{"id":2,"file_url":"https://cdn.donmai.us/original/2.jpg","image_width":100,"image_height":100}
	]`
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, payload), nil
	})

	res, err := c.Search(wallpapersrc.SearchQuery{Query: "city"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Data) != 1 || res.Data[0].ID != "2" {
		t.Fatalf("got %+v, want only the hit with an image URL", res.Data)
	}
}

func TestSearchReportsNon200Status(t *testing.T) {
	c := NewClient()
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnauthorized, "unauthorized"), nil
	})
	_, err := c.Search(wallpapersrc.SearchQuery{Query: "city"})
	if err == nil || !strings.Contains(err.Error(), "booru API returned 401: unauthorized") {
		t.Fatalf("error = %v, want status error", err)
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
	calls := map[string]func(string) (string, error){
		"Download":      c.Download,
		"DownloadThumb": c.DownloadThumb,
		"DownloadContext": func(u string) (string, error) {
			return c.DownloadContext(context.Background(), u)
		},
	}
	for _, rawURL := range []string{
		"", "file:///image.jpg", "http://example.com/image.jpg",
		"https://user@example.com/image.jpg", "https://127.0.0.1/image.jpg",
		"https://localhost/image.jpg", "https://[::1]/image.jpg",
		"https://192.168.1.1/image.jpg", "https://169.254.169.254/image.jpg",
	} {
		for name, call := range calls {
			if _, err := call(rawURL); err == nil {
				t.Errorf("%s accepted unsafe URL %q", name, rawURL)
			}
		}
	}
}

func TestSitesAndIsSite(t *testing.T) {
	got := Sites()
	want := []string{"danbooru", "gelbooru", "konachan", "safebooru"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Sites() = %v, want %v", got, want)
	}
	for _, id := range want {
		if !IsSite(id) {
			t.Errorf("IsSite(%q) = false", id)
		}
	}
	if IsSite("nope") {
		t.Error("IsSite(\"nope\") = true")
	}
}

func TestPreviewFallsBackToThumbnail(t *testing.T) {
	c := NewClient()
	// Danbooru omits large_file_url when the original is already small, so the
	// 180px preview has to be the fallback.
	payload := `[{
		"id": 9,
		"file_url": "https://cdn.donmai.us/original/ab/small.jpg",
		"large_file_url": null,
		"preview_file_url": "https://cdn.donmai.us/preview/small.jpg",
		"image_width": 800,
		"image_height": 600,
		"file_size": 1024,
		"rating": "g",
		"tag_string": "city"
	}]`
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(payload))}, nil
	})

	res, err := c.Search(wallpapersrc.SearchQuery{Query: "city"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("got %d hits, want 1", len(res.Data))
	}
	if got := res.Data[0].PreviewURL; got != "https://cdn.donmai.us/preview/small.jpg" {
		t.Errorf("PreviewURL = %q", got)
	}
}
