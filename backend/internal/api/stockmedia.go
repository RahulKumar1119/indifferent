package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Stock media endpoints for the plan editor (issue #192):
//
//	GET /stock-media/search   find photos or clips for a scene
//	GET /stock-media/assets   look up the assets a plan already chose
//
// They exist so the browser never talks to Pexels or Pixabay and never sees an
// API key. Every result carries the provider's attribution text and link, which
// the terms require wherever results are shown (stockmedia/TERMS.md).
//
// Neither route returns a MediaURL, the full-resolution file the renderer will
// copy later (#193). The browser only needs something to look at, so it gets a
// preview image and, for clips, a light preview video.

// StockMedia is the part of stockmedia.Service the API uses.
type StockMedia interface {
	Search(ctx context.Context, name stockmedia.ProviderName, p stockmedia.SearchParams) (stockmedia.SearchResult, error)
	SearchAll(ctx context.Context, p stockmedia.SearchParams) (stockmedia.MultiResult, error)
	GetMany(ctx context.Context, refs []stockmedia.AssetRef) []stockmedia.AssetResult
	Providers() []stockmedia.ProviderName
}

// StockAsset is an asset as the browser sees it: enough to show and to credit,
// nothing that could be used to download the original.
type StockAsset struct {
	Provider    string  `json:"provider"`
	Type        string  `json:"type"`
	ID          string  `json:"id"`
	PreviewURL  string  `json:"previewUrl,omitempty"`
	PlayURL     string  `json:"playUrl,omitempty"`
	PageURL     string  `json:"pageUrl"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Duration    float64 `json:"duration,omitempty"`
	Orientation string  `json:"orientation"`
	Alt         string  `json:"alt,omitempty"`
	Author      string  `json:"author"`
	AuthorURL   string  `json:"authorUrl,omitempty"`
	Credit      string  `json:"credit"`
}

// StockAttribution is the provider credit to show beside results.
type StockAttribution struct {
	Provider string `json:"provider"`
	Text     string `json:"text"`
	URL      string `json:"url"`
}

// StockProviderFailure says that one provider did not answer, so the page can
// tell the user "Pexels is unavailable" and still show the other's results.
type StockProviderFailure struct {
	Provider string `json:"provider"`
	// Reason is a stable code: rate_limited, unavailable, auth, unsupported_language, other.
	Reason string `json:"reason"`
	// RetryAfter is seconds to wait when Reason is rate_limited, else 0.
	RetryAfter int `json:"retryAfter,omitempty"`
}

// StockSearchResponse is the body of GET /stock-media/search.
type StockSearchResponse struct {
	Assets       []StockAsset           `json:"assets"`
	Attributions []StockAttribution     `json:"attributions"`
	Failures     []StockProviderFailure `json:"failures,omitempty"`
	Page         int                    `json:"page"`
	PerPage      int                    `json:"perPage"`
	// HasMore is true when another page may exist.
	HasMore bool `json:"hasMore"`
}

// StockAssetsResponse is the body of GET /stock-media/assets: one entry per
// requested reference, in the order asked.
type StockAssetsResponse struct {
	Items        []StockAssetItem   `json:"items"`
	Attributions []StockAttribution `json:"attributions"`
}

// StockAssetItem is one looked-up reference. Exactly one of Asset and Status
// is set; Status is "unavailable" (the provider no longer has it, or it cannot
// be used) or "error" (a temporary failure, worth retrying).
type StockAssetItem struct {
	Provider string      `json:"provider"`
	Type     string      `json:"type"`
	ID       string      `json:"id"`
	Asset    *StockAsset `json:"asset,omitempty"`
	Status   string      `json:"status,omitempty"`
}

const (
	stockSearchDefaultPerPage = 12
	stockSearchMaxPerPage     = 30
	// stockAssetsMaxRefs bounds one lookup call. A plan has at most
	// storyplan.MaxScenes scenes.
	stockAssetsMaxRefs = storyplan.MaxScenes
	// stockSearchPerHour is each user's allowance of searches. A search can hit
	// two providers, and Pexels allows only 200 requests an hour for the whole
	// service, so a runaway client must not be able to spend it.
	stockSearchPerHour = 120
	stockSearchWindow  = time.Hour
)

// stockSearchLimiter is a per-user fixed window for searches. It is held in
// memory, so on Lambda each warm container counts separately: it is a
// protection against a runaway client, not an exact quota. The providers'
// own limits (and the service's cooldown) are the backstop.
func (h *APIHandler) stockLimiter() *PreviewLimiter {
	h.stockLimiterOnce.Do(func() {
		if h.StockLimiter == nil {
			h.StockLimiter = &PreviewLimiter{Limit: stockSearchPerHour, Period: stockSearchWindow}
		}
	})
	return h.StockLimiter
}

func isStockMediaPath(path string) bool {
	return path == "/stock-media/search" || path == "/stock-media/assets"
}

func (h *APIHandler) handleStockMedia(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if req.HTTPMethod != "GET" {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
	}
	if h.StockMedia == nil {
		return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Stock media search is not available yet"), nil
	}
	switch req.Path {
	case "/stock-media/search":
		return h.handleStockSearch(ctx, req, claims)
	case "/stock-media/assets":
		return h.handleStockAssets(ctx, req, claims)
	}
	return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
}

// handleStockSearch answers GET /stock-media/search with query parameters:
//
//	q          search words, required, English (Hindi alone is refused)
//	type       photo or video, required
//	provider   pexels, pixabay or omitted for both
//	orientation portrait (default), landscape, square or any
//	page, perPage
func (h *APIHandler) handleStockSearch(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	q := req.QueryStringParameters
	params := stockmedia.SearchParams{
		Query:       q["q"],
		Type:        stockmedia.MediaType(strings.ToLower(strings.TrimSpace(q["type"]))),
		Orientation: stockmedia.Orientation(strings.ToLower(strings.TrimSpace(q["orientation"]))),
	}
	var err error
	if params.Page, err = optionalInt(q["page"], "page"); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}
	if params.PerPage, err = optionalInt(q["perPage"], "perPage"); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}
	if params.PerPage == 0 {
		params.PerPage = stockSearchDefaultPerPage
	}
	// An oversized page is clamped, not refused: asking for more than the page
	// can hold is harmless, and the page reports the size it really got.
	if params.PerPage > stockSearchMaxPerPage {
		params.PerPage = stockSearchMaxPerPage
	}
	// Reject a bad query before it spends any of the user's allowance.
	if msg := stockParamsProblem(params); msg != "" {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", msg), nil
	}
	provider := strings.ToLower(strings.TrimSpace(q["provider"]))
	if provider != "" && provider != "pexels" && provider != "pixabay" {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "provider must be pexels or pixabay"), nil
	}

	if allowed, retry := h.stockLimiter().allow(claims.UserID); !allowed {
		resp := errorResponse(http.StatusTooManyRequests, "RATE_LIMITED", "You are searching too quickly. Wait a moment and try again.")
		resp.Headers["Retry-After"] = strconv.Itoa(maxInt(1, int(retry.Seconds())))
		return resp, nil
	}

	var out StockSearchResponse
	out.Page, out.PerPage = maxInt(params.Page, 1), params.PerPage
	out.Assets = []StockAsset{}
	out.Attributions = []StockAttribution{}

	if provider != "" {
		res, err := h.StockMedia.Search(ctx, stockmedia.ProviderName(provider), params)
		if err != nil {
			return stockSearchFailure(err), nil
		}
		out.Assets = toStockAssets(res.Assets)
		out.Attributions = []StockAttribution{attributionOf(res.Provider, res.Attribution)}
		out.HasMore = hasMore(res, params)
		return jsonResponse(http.StatusOK, out), nil
	}

	multi, err := h.StockMedia.SearchAll(ctx, params)
	if err != nil && len(multi.Results) == 0 {
		return stockSearchFailure(err), nil
	}
	for _, a := range multi.Assets() {
		out.Assets = append(out.Assets, toStockAsset(a))
	}
	for _, r := range multi.Results {
		out.Attributions = append(out.Attributions, attributionOf(r.Provider, r.Attribution))
		out.HasMore = out.HasMore || hasMore(r, params)
	}
	for _, f := range multi.Failures {
		out.Failures = append(out.Failures, failureOf(f.Provider, f.Err))
	}
	return jsonResponse(http.StatusOK, out), nil
}

// stockParamsProblem returns a user-facing message for a search the service
// would reject, or "". Calling it first keeps the allowance for real searches.
func stockParamsProblem(p stockmedia.SearchParams) string {
	if strings.TrimSpace(p.Query) == "" {
		return "Enter something to search for"
	}
	if p.Type != stockmedia.MediaPhoto && p.Type != stockmedia.MediaVideo {
		return "type must be photo or video"
	}
	switch p.Orientation {
	case "", stockmedia.OrientationAny, stockmedia.OrientationPortrait, stockmedia.OrientationLandscape, stockmedia.OrientationSquare:
	default:
		return "orientation must be portrait, landscape, square or any"
	}
	if p.Page < 0 || p.Page > 1000 {
		return "page is out of range"
	}
	// perPage above the maximum was already clamped by the caller, so only the
	// lower bound can fail here.
	if p.PerPage != 0 && p.PerPage < stockmedia.MinPerPage {
		return "perPage is out of range"
	}
	if len([]rune(p.Query)) > stockmedia.MaxQueryRunes {
		return "That search is too long"
	}
	if storyplan.DominantScript(p.Query) == storyplan.ScriptDevanagari || storyplan.DominantScript(p.Query) == storyplan.ScriptIndic {
		if !hasLatinWord(p.Query) {
			return "Search in English: the stock libraries have no results for Hindi words. Try the English word, or a Latin-letter spelling like \"mandir\"."
		}
	}
	return ""
}

func hasLatinWord(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func optionalInt(v, name string) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, errors.New(name + " must be a whole number")
	}
	return n, nil
}

// hasMore says whether a further page may exist: this one was full, or the
// provider's own total is beyond what has been shown.
func hasMore(r stockmedia.SearchResult, p stockmedia.SearchParams) bool {
	shown := maxInt(p.Page, 1) * p.PerPage
	if r.Total > 0 {
		return r.Total > shown
	}
	return len(r.Assets) >= p.PerPage
}

func attributionOf(p stockmedia.ProviderName, a stockmedia.Attribution) StockAttribution {
	return StockAttribution{Provider: string(p), Text: a.Text, URL: a.URL}
}

func toStockAsset(a stockmedia.Asset) StockAsset {
	return StockAsset{
		Provider: string(a.Provider), Type: string(a.Type), ID: a.ID,
		PreviewURL: a.PreviewURL, PlayURL: a.PlayURL, PageURL: a.PageURL,
		Width: a.Width, Height: a.Height, Duration: a.Duration,
		Orientation: string(a.Orientation), Alt: a.Alt,
		Author: a.Author, AuthorURL: a.AuthorURL, Credit: a.Credit,
	}
}

func toStockAssets(in []stockmedia.Asset) []StockAsset {
	out := make([]StockAsset, 0, len(in))
	for _, a := range in {
		out = append(out, toStockAsset(a))
	}
	return out
}

// failureOf maps a provider error to a code the page can act on.
func failureOf(p stockmedia.ProviderName, err error) StockProviderFailure {
	f := StockProviderFailure{Provider: string(p), Reason: "other"}
	var pe *stockmedia.ProviderError
	switch {
	case errors.Is(err, stockmedia.ErrRateLimited):
		f.Reason = "rate_limited"
		if errors.As(err, &pe) && pe.RetryAfter > 0 {
			f.RetryAfter = maxInt(1, int(pe.RetryAfter.Seconds()))
		}
	case errors.Is(err, stockmedia.ErrUnavailable):
		f.Reason = "unavailable"
	case errors.Is(err, stockmedia.ErrAuth):
		// A rejected key is the site's problem, not the user's. The user sees
		// "unavailable"; the operator sees the log.
		f.Reason = "unavailable"
		log.Printf("stock_media_auth_failed provider=%s", p)
	}
	return f
}

// stockSearchFailure is the response when no provider answered at all.
func stockSearchFailure(err error) events.APIGatewayProxyResponse {
	switch {
	case errors.Is(err, stockmedia.ErrInvalidParams):
		msg := "That search cannot be run"
		if errors.Is(err, stockmedia.ErrUnsupportedLanguage) {
			msg = "Search in English: the stock libraries have no results for Hindi words."
		}
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", msg)
	case errors.Is(err, stockmedia.ErrNotConfigured):
		return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Stock media search is not available yet")
	case errors.Is(err, stockmedia.ErrRateLimited):
		resp := errorResponse(http.StatusTooManyRequests, "PROVIDER_RATE_LIMITED", "The photo libraries are busy right now. Try again in a minute.")
		var pe *stockmedia.ProviderError
		retry := 60
		if errors.As(err, &pe) && pe.RetryAfter > 0 {
			retry = maxInt(1, int(pe.RetryAfter.Seconds()))
		}
		resp.Headers["Retry-After"] = strconv.Itoa(retry)
		return resp
	case errors.Is(err, context.Canceled):
		return errorResponse(http.StatusRequestTimeout, "CANCELED", "The search was canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return errorResponse(http.StatusGatewayTimeout, "SEARCH_TIMEOUT", "The photo libraries took too long. Please try again.")
	}
	log.Printf("stock_media_search_failed err=%v", err)
	return errorResponse(http.StatusBadGateway, "SEARCH_FAILED", "The photo libraries could not be reached. Please try again.")
}

// handleStockAssets answers GET /stock-media/assets?refs=provider:type:id,...
// It returns fresh details for assets a plan already chose.
func (h *APIHandler) handleStockAssets(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	refs, msg := parseAssetRefs(req.QueryStringParameters["refs"])
	if msg != "" {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", msg), nil
	}
	items, atts := h.resolveAssets(ctx, refs)
	return jsonResponse(http.StatusOK, StockAssetsResponse{Items: items, Attributions: atts}), nil
}

// parseAssetRefs reads "pexels:photo:123,pixabay:video:45". Malformed
// references are refused as a whole: they come from the plan the page just
// loaded, so a bad one means a bug or tampering, not a stale asset.
func parseAssetRefs(raw string) ([]stockmedia.AssetRef, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "refs is required"
	}
	parts := strings.Split(raw, ",")
	if len(parts) > stockAssetsMaxRefs {
		return nil, "too many assets in one request"
	}
	refs := make([]stockmedia.AssetRef, 0, len(parts))
	for _, p := range parts {
		f := strings.Split(p, ":")
		if len(f) != 3 {
			return nil, "each ref must look like provider:type:id"
		}
		ref := stockmedia.AssetRef{
			Provider: stockmedia.ProviderName(f[0]),
			Type:     stockmedia.MediaType(f[1]),
			ID:       f[2],
		}
		if !ref.Valid() {
			return nil, "each ref must look like provider:type:id"
		}
		refs = append(refs, ref)
	}
	return refs, ""
}

// resolveAssets looks the references up and maps each to an item. A missing or
// unusable asset is "unavailable"; any other failure is "error". It returns the
// attributions for the providers that produced an asset.
func (h *APIHandler) resolveAssets(ctx context.Context, refs []stockmedia.AssetRef) ([]StockAssetItem, []StockAttribution) {
	results := h.StockMedia.GetMany(ctx, refs)
	items := make([]StockAssetItem, 0, len(results))
	seen := map[string]bool{}
	var atts []StockAttribution
	for _, r := range results {
		item := StockAssetItem{Provider: string(r.Ref.Provider), Type: string(r.Ref.Type), ID: r.Ref.ID}
		switch {
		case r.Asset != nil:
			a := toStockAsset(*r.Asset)
			item.Asset = &a
			if !seen[a.Provider] {
				seen[a.Provider] = true
				atts = append(atts, attributionForProvider(a.Provider, r.Ref.Type))
			}
		case errors.Is(r.Err, stockmedia.ErrNotFound):
			item.Status = "unavailable"
		default:
			item.Status = "error"
			if !errors.Is(r.Err, stockmedia.ErrRateLimited) && !errors.Is(r.Err, stockmedia.ErrUnavailable) && !errors.Is(r.Err, context.Canceled) {
				log.Printf("stock_media_lookup_failed provider=%s type=%s err=%v", r.Ref.Provider, r.Ref.Type, r.Err)
			}
		}
		items = append(items, item)
	}
	return items, atts
}

// attributionForProvider returns the credit text for a provider. The wording
// mirrors what stockmedia attaches to search results.
func attributionForProvider(provider string, typ stockmedia.MediaType) StockAttribution {
	noun := map[stockmedia.MediaType]string{stockmedia.MediaPhoto: "Photos", stockmedia.MediaVideo: "Videos"}[typ]
	if provider == "pixabay" {
		if typ == stockmedia.MediaPhoto {
			noun = "Images"
		}
		return StockAttribution{Provider: provider, Text: noun + " provided by Pixabay", URL: "https://pixabay.com"}
	}
	return StockAttribution{Provider: provider, Text: noun + " provided by Pexels", URL: "https://www.pexels.com"}
}
