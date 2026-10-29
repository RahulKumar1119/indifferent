package stockmedia

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

const (
	// maxRedirects bounds how many hops an availability check follows.
	maxRedirects = 5

	// Largest files accepted. A 1080x1920 stock clip of up to a few minutes
	// is tens of megabytes; these limits stop a mislabeled or hostile file
	// from reaching the render worker, whose scratch disk is finite.
	maxVideoBytes = 250 << 20
	maxPhotoBytes = 50 << 20
)

// WithAssetClient replaces the HTTP client CheckAvailable uses. The client is
// responsible for enforcing the URL policy on redirects; without this option
// the service builds one that does.
func WithAssetClient(c HTTPDoer) Option { return func(s *Service) { s.assetClient = c } }

// assetHTTPClient builds the default client for CheckAvailable. Every redirect
// target must pass the service's URL policy, so a provider URL cannot bounce
// the request to an internal address.
func (s *Service) assetHTTPClient() HTTPDoer {
	return &http.Client{
		Timeout:   requestTimeout,
		Transport: s.assetTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("%w: too many redirects", errBadRedirect)
			}
			// The redirect keeps the provider of the request that started it.
			provider, _ := req.Context().Value(providerCtxKey{}).(ProviderName)
			if req.URL.Scheme != "https" {
				return fmt.Errorf("%w: redirect to a non-https URL", errBadRedirect)
			}
			// A policy refusal is an *InvalidAssetError, which CheckAvailable
			// reports as such.
			return s.policy(provider, req.URL)
		},
	}
}

// errBadRedirect marks a redirect the client refused to follow.
var errBadRedirect = errors.New("unacceptable redirect")

type providerCtxKey struct{}

// CheckAvailable confirms an asset's file can be fetched before it is copied
// or rendered. It validates the asset first, then asks the provider's server
// for the file's headers without downloading it, and checks the response
// status, content type and size.
//
// The error wraps ErrInvalidAsset when the asset itself is bad and
// ErrAssetUnavailable when its file is gone or unreachable. When the cause may
// be temporary (a timeout, a 5xx) it also wraps ErrUnavailable, so callers can
// tell "pick another asset" from "try again". Errors never include the URL.
func (s *Service) CheckAvailable(ctx context.Context, a Asset) error {
	if err := validateAsset(a, s.policy, s.minLongSide); err != nil {
		return err
	}
	ctx = context.WithValue(ctx, providerCtxKey{}, a.Provider)

	resp, err := s.probe(ctx, a.MediaURL, http.MethodHead)
	if err == nil && (resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented || resp.StatusCode == http.StatusForbidden) {
		// Some CDNs refuse HEAD but serve GET. Ask for one byte, so a server
		// that ignores Range still costs only the headers: the body is never
		// read and is closed straight away.
		resp.Body.Close()
		resp, err = s.probe(ctx, a.MediaURL, http.MethodGet)
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		// Nothing below returns err itself: net/http puts the full URL in its
		// errors.
		var ia *InvalidAssetError
		switch {
		case errors.As(err, &ia):
			return ia
		case errors.Is(err, errBadRedirect):
			return fmt.Errorf("%w: %s redirected somewhere not allowed", ErrAssetUnavailable, a.describe())
		}
		return fmt.Errorf("%w: %w: %s", ErrAssetUnavailable, ErrUnavailable, a.describe())
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent:
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %w: %s answered HTTP %d", ErrAssetUnavailable, ErrUnavailable, a.describe(), resp.StatusCode)
	default:
		return fmt.Errorf("%w: %s answered HTTP %d", ErrAssetUnavailable, a.describe(), resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		mt, _, err := mime.ParseMediaType(ct)
		want := "image/"
		if a.Type == MediaVideo {
			want = "video/"
		}
		if err != nil || !(strings.HasPrefix(mt, want) || mt == "application/octet-stream" || mt == "binary/octet-stream") {
			return fmt.Errorf("%w: %s served content type %q, expected %s*", ErrAssetUnavailable, a.describe(), mt, want)
		}
	}

	limit := int64(maxPhotoBytes)
	if a.Type == MediaVideo {
		limit = maxVideoBytes
	}
	if size := totalSize(resp); size > limit {
		return fmt.Errorf("%w: %s is %d bytes, over the %d byte limit", ErrAssetUnavailable, a.describe(), size, limit)
	}
	return nil
}

// probe sends one headers-only request.
func (s *Service) probe(ctx context.Context, raw, method string) (*http.Response, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	return s.assetClient.Do(req)
}

// totalSize reads the full size of the file from a probe response: the total
// after the slash of Content-Range for a ranged reply, else Content-Length.
// It returns 0 when the size is unknown.
func totalSize(resp *http.Response) int64 {
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		if i := strings.LastIndex(cr, "/"); i >= 0 {
			var n int64
			if _, err := fmt.Sscanf(cr[i+1:], "%d", &n); err == nil {
				return n
			}
		}
	}
	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return 0
}
