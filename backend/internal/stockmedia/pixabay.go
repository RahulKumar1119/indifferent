package stockmedia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	pixabayBaseURL = "https://pixabay.com"
	pixabaySiteURL = "https://pixabay.com"

	// pixabayLargeImageLong and pixabayFullHDLong are the longer sides of the
	// two image files an API key can get: largeImageURL is scaled to 1280px
	// and fullHDURL, which needs an account approved for full API access, to
	// 1920px.
	pixabayLargeImageLong = 1280
	pixabayFullHDLong     = 1920
)

// Pixabay searches the Pixabay API (https://pixabay.com/api/docs/).
//
// Differences from Pexels that the code accounts for:
//
//   - The API key is a query parameter, so every request URL is a secret.
//     Nothing here lets a URL reach an error or a log line.
//   - Image search can filter by orientation; video search cannot, so video
//     results are ordered by orientation after the fact (see Service).
//   - Only a smaller image file is available without full API access, so
//     photos are at most 1280px on the longer side unless the key is approved
//     for fullHDURL.
//   - safesearch is always on: results end up in public videos.
type Pixabay struct {
	key    Secret
	base   string
	client HTTPDoer
	now    func() time.Time
}

// NewPixabay builds a Pixabay client. It returns ErrNotConfigured without a key.
func NewPixabay(cfg ProviderConfig) (*Pixabay, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: pixabay API key is empty", ErrNotConfigured)
	}
	base := pixabayBaseURL
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
	return &Pixabay{key: cfg.APIKey, base: base, client: client, now: time.Now}, nil
}

// Name implements Provider.
func (*Pixabay) Name() ProviderName { return ProviderPixabay }

// Search implements Provider.
func (p *Pixabay) Search(ctx context.Context, params SearchParams) (SearchResult, error) {
	params, err := normalizeParams(params)
	if err != nil {
		return SearchResult{}, err
	}

	path := "/api/"
	if params.Type == MediaVideo {
		path = "/api/videos/"
	}
	q := url.Values{}
	q.Set("key", p.key.reveal())
	q.Set("q", params.Query)
	q.Set("safesearch", "true")
	q.Set("page", strconv.Itoa(params.Page))
	q.Set("per_page", strconv.Itoa(params.PerPage))
	filtered := false
	if params.Type == MediaPhoto {
		q.Set("image_type", "photo")
		switch params.Orientation {
		case OrientationPortrait:
			q.Set("orientation", "vertical")
			filtered = true
		case OrientationLandscape:
			q.Set("orientation", "horizontal")
			filtered = true
		}
	}

	req, err := http.NewRequest(http.MethodGet, p.base+path+"?"+q.Encode(), nil)
	if err != nil {
		return SearchResult{}, &ProviderError{Provider: ProviderPixabay, Kind: KindBadRequest, Err: errors.New("building request failed")}
	}

	var body struct {
		TotalHits int               `json:"totalHits"`
		Hits      []json.RawMessage `json:"hits"`
	}
	hdr, err := getJSON(ctx, p.client, ProviderPixabay, p.key, req, &body)
	res := SearchResult{
		Provider: ProviderPixabay,
		Type:     params.Type,
		Page:     params.Page,
		PerPage:  params.PerPage,
		filtered: filtered,
		Attribution: Attribution{
			Text: pixabayAttributionText(params.Type),
			URL:  pixabaySiteURL,
		},
	}
	res.Quota = p.quota(hdr)
	if err != nil {
		var pe *ProviderError
		if errors.As(err, &pe) {
			if pe.Kind == KindRateLimited && pe.RetryAfter == 0 && res.Quota != nil {
				// Pixabay reports the window reset on the 429 itself.
				pe.RetryAfter = res.Quota.ResetAt.Sub(p.now())
			}
			if isPixabayKeyError(pe) {
				pe.Kind = KindAuth
			}
		}
		return res, err
	}

	res.Total = body.TotalHits
	res.Assets = make([]Asset, 0, len(body.Hits))
	for _, raw := range body.Hits {
		var a Asset
		var ok bool
		if params.Type == MediaVideo {
			a, ok = decodePixabayVideo(raw)
		} else {
			a, ok = decodePixabayPhoto(raw)
		}
		if !ok {
			res.Skipped++
			continue
		}
		res.Assets = append(res.Assets, a)
	}
	return res, nil
}

// isPixabayKeyError reports whether a 400 is Pixabay rejecting the API key.
// Unlike most APIs, Pixabay answers an invalid or missing key with HTTP 400 and
// a plain-text body ("[ERROR 400] Invalid or missing API key"), observed
// against the live API. Telling it apart from a malformed request matters:
// a bad key needs a human, a bad request needs a code fix.
func isPixabayKeyError(pe *ProviderError) bool {
	return pe.Kind == KindBadRequest && pe.Status == http.StatusBadRequest && pe.Err != nil &&
		strings.Contains(strings.ToLower(pe.Err.Error()), "api key")
}

func pixabayAttributionText(t MediaType) string {
	if t == MediaVideo {
		return "Videos provided by Pixabay"
	}
	return "Images provided by Pixabay"
}

// quota reads X-RateLimit-Remaining and X-RateLimit-Reset, which Pixabay
// sends as the seconds left in the current 60 second window.
func (p *Pixabay) quota(h http.Header) *Quota {
	if h == nil {
		return nil
	}
	remaining, ok := headerInt(h, "X-RateLimit-Remaining")
	if !ok {
		return nil
	}
	q := &Quota{Remaining: remaining}
	// headerInt bounds the value, so the multiplication cannot overflow.
	if reset, ok := headerInt(h, "X-RateLimit-Reset"); ok {
		q.ResetAt = p.now().Add(time.Duration(reset) * time.Second)
	}
	return q
}

// pixabayUserURL is the contributor's profile, which the documentation
// defines as https://pixabay.com/users/{USERNAME}-{ID}/.
func pixabayUserURL(user string, id int64) string {
	if user == "" || id == 0 {
		return ""
	}
	return fmt.Sprintf("%s/users/%s-%d/", pixabaySiteURL, url.PathEscape(user), id)
}

type pixabayPhoto struct {
	ID            int64  `json:"id"`
	PageURL       string `json:"pageURL"`
	Tags          string `json:"tags"`
	WebformatURL  string `json:"webformatURL"`
	LargeImageURL string `json:"largeImageURL"`
	FullHDURL     string `json:"fullHDURL"`
	ImageWidth    int    `json:"imageWidth"`
	ImageHeight   int    `json:"imageHeight"`
	WebformatW    int    `json:"webformatWidth"`
	WebformatH    int    `json:"webformatHeight"`
	User          string `json:"user"`
	UserID        int64  `json:"user_id"`
}

// decodePixabayPhoto normalizes one image. The file is fullHDURL when the key
// has full API access and largeImageURL otherwise. Neither response field
// states the size of those files, so it is derived from imageWidth and
// imageHeight (the original), shrunk to the file's documented longer side.
func decodePixabayPhoto(raw json.RawMessage) (Asset, bool) {
	var ph pixabayPhoto
	if err := json.Unmarshal(raw, &ph); err != nil || ph.ID == 0 {
		return Asset{}, false
	}
	mediaURL, long := ph.LargeImageURL, pixabayLargeImageLong
	if ph.FullHDURL != "" {
		mediaURL, long = ph.FullHDURL, pixabayFullHDLong
	}
	format := extensionFormat(mediaURL)
	if format == "" {
		return Asset{}, false
	}
	w, h := ph.ImageWidth, ph.ImageHeight
	if w <= 0 || h <= 0 {
		// Fall back to the preview's shape; its absolute size is unknown, so
		// scale it to the file's longer side.
		w, h = scaleToLongSide(ph.WebformatW, ph.WebformatH, long)
	}
	w, h = capLongSide(w, h, long)
	author := cleanText(ph.User, 100)
	return Asset{
		Provider:    ProviderPixabay,
		ID:          strconv.FormatInt(ph.ID, 10),
		Type:        MediaPhoto,
		PageURL:     ph.PageURL,
		PreviewURL:  ph.WebformatURL,
		MediaURL:    mediaURL,
		Format:      format,
		Width:       w,
		Height:      h,
		Orientation: orientationOf(w, h),
		Alt:         cleanText(ph.Tags, 200),
		Author:      author,
		AuthorURL:   pixabayUserURL(ph.User, ph.UserID),
		Credit:      credit("Image", author, "Pixabay"),
	}, true
}

type pixabayVideoFile struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Thumbnail string `json:"thumbnail"`
}

type pixabayVideo struct {
	ID       int64   `json:"id"`
	PageURL  string  `json:"pageURL"`
	Tags     string  `json:"tags"`
	Duration float64 `json:"duration"`
	Videos   struct {
		Large  pixabayVideoFile `json:"large"`
		Medium pixabayVideoFile `json:"medium"`
		Small  pixabayVideoFile `json:"small"`
		Tiny   pixabayVideoFile `json:"tiny"`
	} `json:"videos"`
	User   string `json:"user"`
	UserID int64  `json:"user_id"`
}

// decodePixabayVideo normalizes one video. Pixabay returns a fixed set of
// four renditions, any of which may be absent (large is empty when no 4K
// version exists); pickRendition ignores the empty ones. All are MP4.
func decodePixabayVideo(raw json.RawMessage) (Asset, bool) {
	var v pixabayVideo
	if err := json.Unmarshal(raw, &v); err != nil || v.ID == 0 {
		return Asset{}, false
	}
	var renditions []rendition
	for _, f := range []pixabayVideoFile{v.Videos.Large, v.Videos.Medium, v.Videos.Small, v.Videos.Tiny} {
		renditions = append(renditions, rendition{URL: f.URL, Thumbnail: f.Thumbnail, Format: "video/mp4", Width: f.Width, Height: f.Height})
	}
	r, ok := pickRendition(renditions, defaultMaxLongSide)
	if !ok {
		return Asset{}, false
	}
	author := cleanText(v.User, 100)
	return Asset{
		Provider:    ProviderPixabay,
		ID:          strconv.FormatInt(v.ID, 10),
		Type:        MediaVideo,
		PageURL:     v.PageURL,
		PreviewURL:  r.Thumbnail,
		MediaURL:    r.URL,
		Format:      r.Format,
		Width:       r.Width,
		Height:      r.Height,
		Duration:    v.Duration,
		Orientation: orientationOf(r.Width, r.Height),
		Alt:         cleanText(v.Tags, 200),
		Author:      author,
		AuthorURL:   pixabayUserURL(v.User, v.UserID),
		Credit:      credit("Video", author, "Pixabay"),
	}, true
}
