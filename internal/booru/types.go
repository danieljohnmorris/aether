package booru

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"aether/internal/wallpapersrc"
)

// engine identifies the API dialect a booru speaks. The dialects differ only in
// their query parameters and their post payload field names.
type engine int

const (
	engineDanbooru engine = iota
	engineMoebooru
	engineGelbooru
)

// site describes one supported booru catalog.
type site struct {
	// id is the catalog identifier used in SearchQuery.Site.
	id string
	// engine selects the query and payload dialect.
	engine engine
	// listingURL is the posts endpoint without any query string.
	listingURL string
	// pageURLFormat renders a human-facing post page URL from the post id.
	pageURLFormat string
	// ratingTag restricts results to the site's general/safe rating.
	ratingTag string
	// requireCredentials marks catalogs (gelbooru) that reject anonymous
	// requests outright.
	requireCredentials bool
	// anonymousBudget is the maximum number of tags accepted without
	// credentials; zero means the site imposes no budget.
	anonymousBudget int
}

// sites is the catalog registry. The order of Sites() output is derived from
// this map's keys, so it is kept sorted.
var sites = map[string]site{
	wallpapersrc.SiteSafebooru: {
		id:              wallpapersrc.SiteSafebooru,
		engine:          engineDanbooru,
		listingURL:      "https://safebooru.donmai.us/posts.json",
		pageURLFormat:   "https://safebooru.donmai.us/posts/%s",
		ratingTag:       "rating:g",
		anonymousBudget: 2,
	},
	wallpapersrc.SiteDanbooru: {
		id:              wallpapersrc.SiteDanbooru,
		engine:          engineDanbooru,
		listingURL:      "https://danbooru.donmai.us/posts.json",
		pageURLFormat:   "https://danbooru.donmai.us/posts/%s",
		ratingTag:       "rating:g",
		anonymousBudget: 2,
	},
	wallpapersrc.SiteKonachan: {
		id:            wallpapersrc.SiteKonachan,
		engine:        engineMoebooru,
		listingURL:    "https://konachan.net/post.json",
		pageURLFormat: "https://konachan.net/post/show/%s",
		ratingTag:     "rating:safe",
	},
	wallpapersrc.SiteGelbooru: {
		id:                 wallpapersrc.SiteGelbooru,
		engine:             engineGelbooru,
		listingURL:         "https://gelbooru.com/index.php",
		pageURLFormat:      "https://gelbooru.com/index.php?page=post&s=view&id=%s",
		ratingTag:          "rating:general",
		requireCredentials: true,
	},
}

// defaultLimit is the page size used when a query does not specify one.
const defaultLimit = 24

// maxLimit is the largest page size any supported booru accepts.
const maxLimit = 100

// booruDomains lists the hosts whose image URLs keep their original filename.
// Files served from any other host are stored under a hash of the URL to avoid
// collisions between unrelated remote files.
var booruDomains = []string{
	"donmai.us",
	"konachan.net",
	"konachan.com",
	"gelbooru.com",
}

// flexInt decodes a JSON number or a quoted numeric string, since booru APIs
// are inconsistent about quoting numeric attributes.
type flexInt int64

// UnmarshalJSON accepts either form and treats null/"" as zero.
func (n *flexInt) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		*n = flexInt(v)
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid number %q", s)
	}
	*n = flexInt(f)
	return nil
}

// idString decodes a JSON post id that may arrive as a number or a string.
type idString string

// UnmarshalJSON accepts either form and treats null/"" as empty.
func (id *idString) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	*id = idString(s)
	return nil
}

// danbooruPost mirrors one entry of a Danbooru-engine posts.json array.
type danbooruPost struct {
	ID             idString `json:"id"`
	FileURL        string   `json:"file_url"`
	LargeFileURL   string   `json:"large_file_url"`
	PreviewFileURL string   `json:"preview_file_url"`
	ImageWidth     flexInt  `json:"image_width"`
	ImageHeight    flexInt  `json:"image_height"`
	FileSize       flexInt  `json:"file_size"`
	Rating         string   `json:"rating"`
	TagString      string   `json:"tag_string"`
	Source         string   `json:"source"`
}

// moebooruPost mirrors one entry of a Moebooru post.json array.
type moebooruPost struct {
	ID         idString `json:"id"`
	FileURL    string   `json:"file_url"`
	PreviewURL string   `json:"preview_url"`
	SampleURL  string   `json:"sample_url"`
	JPEGURL    string   `json:"jpeg_url"`
	Width      flexInt  `json:"width"`
	Height     flexInt  `json:"height"`
	FileSize   flexInt  `json:"file_size"`
	Tags       string   `json:"tags"`
	Source     string   `json:"source"`
	Rating     string   `json:"rating"`
}

// gelbooruPost mirrors one entry of a gelbooru dapi post array.
type gelbooruPost struct {
	ID         idString `json:"id"`
	FileURL    string   `json:"file_url"`
	PreviewURL string   `json:"preview_url"`
	SampleURL  string   `json:"sample_url"`
	Width      flexInt  `json:"width"`
	Height     flexInt  `json:"height"`
	FileSize   flexInt  `json:"file_size"`
	Tags       string   `json:"tags"`
	Rating     string   `json:"rating"`
}

// gelbooruPosts tolerates the empty-string "post" field gelbooru returns when a
// query matches nothing.
type gelbooruPosts []gelbooruPost

// UnmarshalJSON decodes the post array, treating an empty string as no results.
func (posts *gelbooruPosts) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" || trimmed == `""` {
		return nil
	}
	var decoded []gelbooruPost
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*posts = decoded
	return nil
}

// gelbooruResponse is the gelbooru dapi JSON envelope.
type gelbooruResponse struct {
	Post gelbooruPosts `json:"post"`
}
