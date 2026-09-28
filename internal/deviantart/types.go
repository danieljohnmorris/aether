package deviantart

import "strings"

// tokenResponse is the OAuth2 token endpoint payload.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Status      string `json:"status"`
}

// tagsResponse is the /browse/tags payload.
type tagsResponse struct {
	HasMore          bool        `json:"has_more"`
	NextOffset       int         `json:"next_offset"`
	EstimatedTotal   int         `json:"estimated_total"`
	Results          []deviation `json:"results"`
	ErrorCode        errorCode   `json:"error_code"`
	Error            string      `json:"error"`
	ErrorDescription string      `json:"error_description"`
}

// deviation is the subset of the deviation object the provider consumes.
type deviation struct {
	DeviationID string `json:"deviationid"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Author      struct {
		Username string `json:"username"`
	} `json:"author"`
	Content struct {
		Src      string `json:"src"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Filesize int64  `json:"filesize"`
	} `json:"content"`
	Preview struct {
		Src    string `json:"src"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"preview"`
	Thumbs []struct {
		Src    string `json:"src"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"thumbs"`
	IsMature       bool `json:"is_mature"`
	IsDownloadable bool `json:"is_downloadable"`
	Stats          struct {
		Favourites int `json:"favourites"`
	} `json:"stats"`
}

// errorCode tolerates the API serializing error_code as either a JSON number
// or a JSON string.
type errorCode string

// UnmarshalJSON decodes a number or string error code.
func (e *errorCode) UnmarshalJSON(b []byte) error {
	*e = errorCode(strings.Trim(string(b), `"`))
	return nil
}

// failed reports whether the error code denotes a non-zero (failure) status.
func (e errorCode) failed() bool {
	return e != "" && e != "0"
}
