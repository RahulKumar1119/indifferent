package stockmedia

import (
	"math"
	"sort"
	"time"
)

// ProviderName identifies a stock provider.
type ProviderName string

// Supported providers.
const (
	ProviderPexels  ProviderName = "pexels"
	ProviderPixabay ProviderName = "pixabay"
)

// MediaType is the kind of asset a search returns. Every search is for one
// type: asking for both would silently double the calls against provider rate
// limits, so callers who want both search twice.
type MediaType string

// Supported media types.
const (
	MediaPhoto MediaType = "photo"
	MediaVideo MediaType = "video"
)

// Orientation is the shape of an asset, or the shape a search prefers.
type Orientation string

// Orientations. A zero SearchParams.Orientation means portrait, because the
// output is 9:16; pass OrientationAny to express no preference.
const (
	OrientationAny       Orientation = "any"
	OrientationPortrait  Orientation = "portrait"
	OrientationLandscape Orientation = "landscape"
	OrientationSquare    Orientation = "square"
)

// Search size limits. PerPage is bounded by the stricter provider: Pexels
// allows at most 80 results per page, Pixabay at least 3.
const (
	DefaultPerPage = 15
	MinPerPage     = 3
	MaxPerPage     = 80

	// MaxQueryRunes is Pixabay's limit on the search term; it is applied to
	// both providers so a query valid for one is valid for the other.
	MaxQueryRunes = 100
)

// defaultMaxLongSide caps the video rendition we pick. The longer side of a
// 1080x1920 frame is 1920, so anything bigger only costs download time
// unless a landscape source is being cropped to portrait. 4K is used only
// when nothing smaller exists.
const defaultMaxLongSide = 1920

// SearchParams describes one search. Zero values take defaults; see
// normalizeParams.
type SearchParams struct {
	Query       string
	Type        MediaType
	Orientation Orientation
	Page        int
	PerPage     int
}

// Asset is one stock photo or video clip in the shape every provider is
// normalized to.
//
// MediaURL is the file to copy into our storage. It is a provider URL and must
// not be kept: Pixabay forbids permanent hotlinking and Pexels video links are
// signed. PreviewURL is for showing search results only.
type Asset struct {
	Provider   ProviderName `json:"provider"`
	ID         string       `json:"id"`
	Type       MediaType    `json:"type"`
	PageURL    string       `json:"pageUrl"`
	PreviewURL string       `json:"previewUrl,omitempty"`
	// PlayURL is a small video file for previewing a clip in the browser: the
	// lightest rendition that is still sharp on a phone. Video only. It is a
	// provider URL like MediaURL and is not to be kept.
	PlayURL     string      `json:"playUrl,omitempty"`
	MediaURL    string      `json:"mediaUrl"`
	Format      string      `json:"format"` // MIME type, e.g. video/mp4
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Duration    float64     `json:"duration,omitempty"` // seconds; video only
	Orientation Orientation `json:"orientation"`
	Alt         string      `json:"alt,omitempty"`
	Author      string      `json:"author"`
	AuthorURL   string      `json:"authorUrl,omitempty"`
	Credit      string      `json:"credit"` // ready-to-show credit line
}

// Attribution is the provider credit to display next to results.
type Attribution struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Quota is what a provider reported about its rate limit.
type Quota struct {
	Remaining int
	ResetAt   time.Time
}

// SearchResult is the outcome of one search against one provider.
type SearchResult struct {
	Provider ProviderName `json:"provider"`
	Type     MediaType    `json:"type"`
	Assets   []Asset      `json:"assets"`
	// Total is how many results the provider says match. For Pixabay it is
	// the count reachable through the API (at most 500 per query).
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"perPage"`
	// Skipped counts assets dropped because they could not be used: the
	// provider sent something undecodable, or the asset failed validation.
	// SkipReasons breaks the validation drops down by InvalidAssetError.Reason
	// so a provider change (a new CDN host, say) shows up as a count, not as
	// silently fewer results.
	Skipped     int            `json:"skipped"`
	SkipReasons map[string]int `json:"skipReasons,omitempty"`
	// Cached reports that the result came from the cache, not the provider.
	Cached bool `json:"cached"`
	// Attribution must be shown prominently wherever these results appear.
	Attribution Attribution `json:"attribution"`
	// Quota is nil when the provider reported nothing.
	Quota *Quota `json:"-"`

	// filtered records that the request asked the provider to filter by
	// orientation. An empty filtered result is worth retrying without the
	// filter; an empty unfiltered one is a genuine empty result.
	filtered bool
}

// Secret holds an API key. It prints as [REDACTED] with every fmt verb that
// calls String or GoString and in JSON, so a key cannot leak through a log
// line or a serialized config by accident.
type Secret string

func (Secret) String() string               { return "[REDACTED]" }
func (Secret) GoString() string             { return `"[REDACTED]"` }
func (Secret) MarshalText() ([]byte, error) { return []byte("[REDACTED]"), nil }

// reveal returns the key for use in a request. It is the only way out.
func (s Secret) reveal() string { return string(s) }

// orientationOf classifies a width and height. Equal sides are square;
// unknown (non-positive) sizes are reported as OrientationAny.
func orientationOf(width, height int) Orientation {
	switch {
	case width <= 0 || height <= 0:
		return OrientationAny
	case height > width:
		return OrientationPortrait
	case width > height:
		return OrientationLandscape
	default:
		return OrientationSquare
	}
}

// orientationRank orders an asset's orientation by how close it is to the
// preferred one: 0 is a match, higher is further away. Portrait preferred
// gives portrait, square, landscape; landscape preferred gives landscape,
// square, portrait; square preferred treats the other two as equal.
func orientationRank(preferred, got Orientation) int {
	switch {
	case preferred == OrientationAny || preferred == "":
		return 0
	case got == preferred:
		return 0
	case got == OrientationSquare || preferred == OrientationSquare:
		return 1
	default:
		return 2
	}
}

// sortByOrientation orders assets best-match first. It is stable, so within
// one orientation the provider's own relevance order is kept.
func sortByOrientation(assets []Asset, preferred Orientation) {
	if preferred == OrientationAny || preferred == "" {
		return
	}
	sort.SliceStable(assets, func(i, j int) bool {
		return orientationRank(preferred, assets[i].Orientation) < orientationRank(preferred, assets[j].Orientation)
	})
}

// rendition is one downloadable size of a video.
type rendition struct {
	URL       string
	Thumbnail string
	Format    string
	Width     int
	Height    int
}

func (r rendition) longSide() int {
	if r.Width > r.Height {
		return r.Width
	}
	return r.Height
}

func (r rendition) pixels() int { return r.Width * r.Height }

// pickRendition chooses the rendition to download. It takes the largest one
// whose longer side is within limit; when every rendition is larger than that
// it takes the smallest, so a result is never lost just because it only comes
// in 4K. Renditions without a URL, size or allowed format are ignored.
func pickRendition(rs []rendition, limit int) (rendition, bool) {
	var best, smallest rendition
	var haveBest, haveAny bool
	for _, r := range rs {
		if r.URL == "" || r.Width <= 0 || r.Height <= 0 || !allowedFormats[r.Format] {
			continue
		}
		if !haveAny || r.pixels() < smallest.pixels() {
			smallest, haveAny = r, true
		}
		if r.longSide() <= limit && (!haveBest || r.pixels() > best.pixels()) {
			best, haveBest = r, true
		}
	}
	switch {
	case haveBest:
		return best, true
	case haveAny:
		return smallest, true
	default:
		return rendition{}, false
	}
}

// previewMinLongSide is the shortest longer side a preview rendition may have.
// A 640 pixel clip is sharp at phone width and roughly a tenth the size of the
// 1080p file the render uses.
const previewMinLongSide = 640

// pickPreviewRendition chooses the file to play when a user previews a clip:
// the smallest usable rendition whose longer side is at least previewMinLongSide,
// or the largest one when every rendition is smaller than that.
func pickPreviewRendition(rs []rendition) (rendition, bool) {
	var best rendition
	var have bool
	for _, r := range rs {
		if r.URL == "" || r.Width <= 0 || r.Height <= 0 || !allowedFormats[r.Format] {
			continue
		}
		switch {
		case !have:
			best, have = r, true
		case best.longSide() < previewMinLongSide && r.longSide() > best.longSide():
			best = r // still below the floor: any bigger one is better
		case r.longSide() >= previewMinLongSide && (best.longSide() < previewMinLongSide || r.pixels() < best.pixels()):
			best = r // at or above the floor: the smallest such one
		}
	}
	return best, have
}

// scaleToLongSide returns the size with the same aspect ratio whose longer
// side equals long.
func scaleToLongSide(width, height, long int) (int, int) {
	if width <= 0 || height <= 0 || long <= 0 {
		return 0, 0
	}
	if width >= height {
		return long, int(math.Round(float64(height) * float64(long) / float64(width)))
	}
	return int(math.Round(float64(width) * float64(long) / float64(height))), long
}

// capLongSide shrinks a size so its longer side is at most limit and leaves
// smaller sizes alone.
func capLongSide(width, height, limit int) (int, int) {
	if width <= limit && height <= limit {
		return width, height
	}
	return scaleToLongSide(width, height, limit)
}
