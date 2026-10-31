package stockmedia

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// downloadTimeout bounds one whole file download. A 250 MB clip at 5 MB/s takes
// about a minute; five minutes leaves room for a slow CDN without letting a
// stalled one hold a render worker for long.
const downloadTimeout = 5 * time.Minute

// Download copies an asset's full-resolution file (MediaURL) into w and returns
// the number of bytes written.
//
// It applies every check CheckAvailable does (the asset is valid, its URL is on
// the provider's own domains, every redirect is too, the content type matches
// and the declared size is within the limit), then reads the body with a hard
// cap, so a server that ignores its own Content-Length cannot send more than
// the limit. Nothing is sent to the provider except a plain GET: no API key and
// no cookies.
//
// The error is shaped like CheckAvailable's: ErrInvalidAsset when the asset is
// bad, ErrAssetUnavailable when its file is gone, and also ErrUnavailable when
// the cause may be temporary and a retry could work. A download that fails part
// way leaves a partial file in w; the caller discards it. Errors never include
// the URL.
func (s *Service) Download(ctx context.Context, a Asset, w io.Writer) (int64, error) {
	if err := s.CheckAvailable(ctx, a); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.WithValue(ctx, providerCtxKey{}, a.Provider), downloadTimeout)
	defer cancel()

	u, err := url.Parse(a.MediaURL)
	if err != nil {
		return 0, invalidAsset("url", "unparseable URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, invalidAsset("url", "unusable URL")
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.downloadClient().Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, downloadCtxError(ctxErr)
		}
		if errors.Is(err, errBadRedirect) {
			return 0, fmt.Errorf("%w: %s redirected somewhere not allowed", ErrAssetUnavailable, a.describe())
		}
		return 0, fmt.Errorf("%w: %w: %s", ErrAssetUnavailable, ErrUnavailable, a.describe())
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return 0, fmt.Errorf("%w: %w: %s answered HTTP %d", ErrAssetUnavailable, ErrUnavailable, a.describe(), resp.StatusCode)
	default:
		return 0, fmt.Errorf("%w: %s answered HTTP %d", ErrAssetUnavailable, a.describe(), resp.StatusCode)
	}

	limit := int64(maxPhotoBytes)
	if a.Type == MediaVideo {
		limit = maxVideoBytes
	}
	// Read one byte past the limit: if it arrives the file is too big, whatever
	// the headers said.
	ww := &recordingWriter{w: w}
	n, err := io.Copy(ww, io.LimitReader(resp.Body, limit+1))
	if ww.err != nil {
		// The caller's writer failed (a full disk): that is not the provider's fault
		// and nothing about the asset is wrong.
		return n, fmt.Errorf("writing the download: %w", ww.err)
	}
	if n > limit {
		return n, fmt.Errorf("%w: %s is over the %d byte limit", ErrAssetUnavailable, a.describe(), limit)
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return n, downloadCtxError(ctxErr)
		}
		return n, fmt.Errorf("%w: %w: %s download was interrupted", ErrAssetUnavailable, ErrUnavailable, a.describe())
	}
	if n == 0 {
		return 0, fmt.Errorf("%w: %s was empty", ErrAssetUnavailable, a.describe())
	}
	return n, nil
}

// downloadClient is the client for CheckAvailable with its timeout lifted: the
// download's own deadline (downloadTimeout, on the context) bounds it instead,
// because requestTimeout is meant for small API calls.
func (s *Service) downloadClient() HTTPDoer {
	if c, ok := s.assetClient.(*http.Client); ok {
		clone := *c
		clone.Timeout = 0
		return &clone
	}
	return s.assetClient
}

// downloadCtxError tells a cancelled download from one that ran out of time.
func downloadCtxError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w: the download took too long", ErrAssetUnavailable, ErrUnavailable)
	}
	return err
}

// recordingWriter remembers whether a write failed, so a failure of the
// destination can be told from a failure of the source.
type recordingWriter struct {
	w   io.Writer
	err error
}

func (r *recordingWriter) Write(p []byte) (int, error) {
	n, err := r.w.Write(p)
	if err != nil && r.err == nil {
		r.err = err
	}
	return n, err
}
