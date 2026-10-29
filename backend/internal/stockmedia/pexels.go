package stockmedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	pexelsBaseURL = "https://api.pexels.com"
	pexelsSiteURL = "https://www.pexels.com"
)

// ProviderConfig configures one provider client. Only APIKey is required.
type ProviderConfig struct {
	APIKey Secret
	// BaseURL overrides the API origin. Tests point it at httptest.Server.
	BaseURL string
	// HTTPClient overrides the client used for API calls.
	HTTPClient HTTPDoer
}

// Provider is one stock library. Use it through Service, which validates the
// assets it returns, caches responses and respects rate limits.
type Provider interface {
	Name() ProviderName
	// Search runs one request for already-normalized parameters.
	Search(ctx context.Context, p SearchParams) (SearchResult, error)
}

// Pexels searches the Pexels API (https://www.pexels.com/api/documentation/).
//
// Pexels has an orientation filter for both photos and videos, and returns
// several renditions per video, from which the largest one that fits the
// output frame is chosen.
type Pexels struct {
	key    Secret
	base   string
	client HTTPDoer
}

// NewPexels builds a Pexels client. It returns ErrNotConfigured without a key.
func NewPexels(cfg ProviderConfig) (*Pexels, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: pexels API key is empty", ErrNotConfigured)
	}
	base := pexelsBaseURL
	if cfg.BaseURL != "" {
		b, err := requireBase(cfg.BaseURL)
		if err != nil {
			return nil, err
		}
		base = b
	}
	client := cfg.HTTPClient
	if client == nil {
		client = defaultHTTPClient()
	}
	return &Pexels{key: cfg.APIKey, base: base, client: client}, nil
}

// Name implements Provider.
func (*Pexels) Name() ProviderName { return ProviderPexels }

// Search implements Provider.
func (p *Pexels) Search(ctx context.Context, params SearchParams) (SearchResult, error) {
	params, err := normalizeParams(params)
	if err != nil {
		return SearchResult{}, err
	}

	path := "/v1/search"
	if params.Type == MediaVideo {
		path = "/v1/videos/search"
	}
	q := url.Values{}
	q.Set("query", params.Query)
	q.Set("page", strconv.Itoa(params.Page))
	q.Set("per_page", strconv.Itoa(params.PerPage))
	filtered := false
	if params.Orientation != OrientationAny {
		q.Set("orientation", string(params.Orientation))
		filtered = true
	}

	req, err := http.NewRequest(http.MethodGet, p.base+path+"?"+q.Encode(), nil)
	if err != nil {
		return SearchResult{}, &ProviderError{Provider: ProviderPexels, Kind: KindBadRequest, Err: fmt.Errorf("building request failed")}
	}
	// Pexels takes the key as the whole Authorization header value, no scheme.
	req.Header.Set("Authorization", p.key.reveal())

	var body struct {
		TotalResults int               `json:"total_results"`
		Page         int               `json:"page"`
		PerPage      int               `json:"per_page"`
		Photos       []json.RawMessage `json:"photos"`
		Videos       []json.RawMessage `json:"videos"`
	}
	hdr, err := getJSON(ctx, p.client, ProviderPexels, p.key, req, &body)
	res := SearchResult{
		Provider: ProviderPexels,
		Type:     params.Type,
		Page:     params.Page,
		PerPage:  params.PerPage,
		filtered: filtered,
		Attribution: Attribution{
			Text: pexelsAttributionText(params.Type),
			URL:  pexelsSiteURL,
		},
	}
	res.Quota = pexelsQuota(hdr)
	if err != nil {
		return res, err
	}

	res.Total = body.TotalResults
	items := body.Photos
	if params.Type == MediaVideo {
		items = body.Videos
	}
	res.Assets = make([]Asset, 0, len(items))
	for _, raw := range items {
		var a Asset
		var ok bool
		if params.Type == MediaVideo {
			a, ok = decodePexelsVideo(raw)
		} else {
			a, ok = decodePexelsPhoto(raw)
		}
		if !ok {
			res.Skipped++
			continue
		}
		res.Assets = append(res.Assets, a)
	}
	return res, nil
}

func pexelsAttributionText(t MediaType) string {
	if t == MediaVideo {
		return "Videos provided by Pexels"
	}
	return "Photos provided by Pexels"
}

// pexelsQuota reads the monthly quota headers, which Pexels sends only on
// successful responses. X-Ratelimit-Reset is a UNIX timestamp.
func pexelsQuota(h http.Header) *Quota {
	if h == nil {
		return nil
	}
	remaining, ok := headerInt(h, "X-Ratelimit-Remaining")
	if !ok {
		return nil
	}
	return &Quota{Remaining: remaining, ResetAt: headerUnix(h, "X-Ratelimit-Reset")}
}

type pexelsPhoto struct {
	ID              int64  `json:"id"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	URL             string `json:"url"`
	Photographer    string `json:"photographer"`
	PhotographerURL string `json:"photographer_url"`
	Alt             string `json:"alt"`
	Src             struct {
		Original string `json:"original"`
		Large    string `json:"large"`
		Medium   string `json:"medium"`
	} `json:"src"`
}

// decodePexelsPhoto normalizes one photo. MediaURL is src.original, the only
// size the documentation guarantees is unchanged and exactly Width x Height;
// the sized variants are crops fitted to a box, so their dimensions are not
// known. The file is downloaded once, when a scene is confirmed.
func decodePexelsPhoto(raw json.RawMessage) (Asset, bool) {
	var ph pexelsPhoto
	if err := json.Unmarshal(raw, &ph); err != nil || ph.ID == 0 {
		return Asset{}, false
	}
	format := extensionFormat(ph.Src.Original)
	if format == "" {
		return Asset{}, false
	}
	preview := ph.Src.Medium
	if preview == "" {
		preview = ph.Src.Large
	}
	author := cleanText(ph.Photographer, 100)
	return Asset{
		Provider:    ProviderPexels,
		ID:          strconv.FormatInt(ph.ID, 10),
		Type:        MediaPhoto,
		PageURL:     ph.URL,
		PreviewURL:  preview,
		MediaURL:    ph.Src.Original,
		Format:      format,
		Width:       ph.Width,
		Height:      ph.Height,
		Orientation: orientationOf(ph.Width, ph.Height),
		Alt:         cleanText(ph.Alt, 200),
		Author:      author,
		AuthorURL:   ph.PhotographerURL,
		Credit:      credit("Photo", author, "Pexels"),
	}, true
}

type pexelsVideo struct {
	ID       int64  `json:"id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	URL      string `json:"url"`
	Image    string `json:"image"`
	Duration int    `json:"duration"`
	User     struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"user"`
	VideoFiles []struct {
		FileType string `json:"file_type"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Link     string `json:"link"`
	} `json:"video_files"`
}

// decodePexelsVideo normalizes one video, picking the rendition with
// pickRendition. The video's own width and height describe its top
// resolution, so the chosen rendition's size is what the Asset reports.
func decodePexelsVideo(raw json.RawMessage) (Asset, bool) {
	var v pexelsVideo
	if err := json.Unmarshal(raw, &v); err != nil || v.ID == 0 {
		return Asset{}, false
	}
	renditions := make([]rendition, 0, len(v.VideoFiles))
	for _, f := range v.VideoFiles {
		renditions = append(renditions, rendition{URL: f.Link, Format: f.FileType, Width: f.Width, Height: f.Height})
	}
	r, ok := pickRendition(renditions, defaultMaxLongSide)
	if !ok {
		return Asset{}, false
	}
	author := cleanText(v.User.Name, 100)
	return Asset{
		Provider:    ProviderPexels,
		ID:          strconv.FormatInt(v.ID, 10),
		Type:        MediaVideo,
		PageURL:     v.URL,
		PreviewURL:  v.Image,
		MediaURL:    r.URL,
		Format:      r.Format,
		Width:       r.Width,
		Height:      r.Height,
		Duration:    float64(v.Duration),
		Orientation: orientationOf(r.Width, r.Height),
		Author:      author,
		AuthorURL:   v.User.URL,
		Credit:      credit("Video", author, "Pexels"),
	}, true
}

// credit builds "Photo by Jane Doe on Pexels", or "Photo on Pexels" when the
// author is unknown.
func credit(kind, author, provider string) string {
	if author == "" {
		return fmt.Sprintf("%s on %s", kind, provider)
	}
	return fmt.Sprintf("%s by %s on %s", kind, author, provider)
}
