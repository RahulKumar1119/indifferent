package stockmedia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	// requestTimeout bounds one provider call.
	requestTimeout = 10 * time.Second
	// maxResponseBytes bounds a provider response. 80 video results with
	// every rendition is about 1 MB, so this leaves a wide margin without
	// letting a misbehaving server fill memory.
	maxResponseBytes = 8 << 20
	// maxErrorSnippet is how much of an error body is kept for diagnostics.
	maxErrorSnippet = 200
	userAgent       = "indifferent-stockmedia/1.0"
)

// HTTPDoer is the part of *http.Client the providers use. Tests substitute it.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: requestTimeout}
}

// getJSON performs the request and decodes a 200 response into out.
//
// Every failure is a *ProviderError. Errors never include the request URL,
// because Pixabay carries its API key in the query string and net/http puts
// the URL in its errors. The response headers are returned even on failure so
// callers can read rate-limit details. Cancellation of ctx is returned as
// ctx.Err() unchanged.
func getJSON(ctx context.Context, client HTTPDoer, provider ProviderName, secret Secret, req *http.Request, out any) (http.Header, error) {
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, &ProviderError{Provider: provider, Kind: KindUnavailable, Err: errors.New(transportReason(err, secret))}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return resp.Header, ctxErr
		}
		return resp.Header, &ProviderError{Provider: provider, Kind: KindUnavailable, Status: resp.StatusCode, Err: errors.New("reading response failed")}
	}
	if len(body) > maxResponseBytes {
		return resp.Header, &ProviderError{Provider: provider, Kind: KindBadResponse, Status: resp.StatusCode, Err: errors.New("response too large")}
	}

	if resp.StatusCode != http.StatusOK {
		return resp.Header, statusError(provider, secret, resp, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return resp.Header, &ProviderError{Provider: provider, Kind: KindBadResponse, Status: resp.StatusCode, Err: errors.New("response is not valid JSON of the expected shape")}
	}
	return resp.Header, nil
}

// statusError maps a non-200 response to a ProviderError.
func statusError(provider ProviderName, secret Secret, resp *http.Response, body []byte) error {
	pe := &ProviderError{Provider: provider, Status: resp.StatusCode}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		pe.Kind = KindRateLimited
		pe.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		pe.Kind = KindAuth
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		pe.Kind = KindBadRequest
	case resp.StatusCode >= 500:
		pe.Kind = KindUnavailable
	default:
		pe.Kind = KindBadResponse
	}
	// Only 4xx bodies are quoted: they are short messages ("page out of
	// range"), while 5xx bodies tend to be HTML error pages.
	if pe.Kind != KindUnavailable && pe.Kind != KindBadResponse {
		if snippet := errorSnippet(body, secret); snippet != "" {
			pe.Err = errors.New(snippet)
		}
	}
	return pe
}

// transportReason describes a failed round trip without its URL.
func transportReason(err error, secret Secret) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "request timed out"
	}
	return scrub(err.Error(), secret)
}

// scrub removes the secret, in raw and URL-escaped form, from s.
func scrub(s string, secret Secret) string {
	key := secret.reveal()
	if key == "" {
		return s
	}
	s = strings.ReplaceAll(s, key, "[REDACTED]")
	if esc := url.QueryEscape(key); esc != key {
		s = strings.ReplaceAll(s, esc, "[REDACTED]")
	}
	return s
}

// errorSnippet returns a short, single-line, secret-free excerpt of an error
// body, or "" when the body is empty.
func errorSnippet(body []byte, secret Secret) string {
	text := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, string(body))
	text = strings.Join(strings.Fields(text), " ")
	text = scrub(text, secret)
	if runes := []rune(text); len(runes) > maxErrorSnippet {
		text = string(runes[:maxErrorSnippet]) + "..."
	}
	return text
}

// parseRetryAfter reads a Retry-After header as delta-seconds or an HTTP
// date. It returns 0 when the header is absent or unusable.
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

// maxHeaderCount bounds the integers read from rate-limit headers. Provider
// headers are untrusted input: a malformed one could otherwise be negative or
// larger than int on a 32-bit build, or overflow when multiplied into a
// time.Duration. A million is far above any real quota (Pexels' is 20,000 a
// month) and far below every overflow point.
const maxHeaderCount = 1_000_000

// headerInt reads an integer header, reporting whether it was present and
// valid. The value is clamped to [0, maxHeaderCount], so callers can convert
// it to int or multiply it into a Duration without an overflow check. A
// negative count is treated as zero, which for a quota means "used up".
func headerInt(h http.Header, name string) (int, bool) {
	v := strings.TrimSpace(h.Get(name))
	if v == "" {
		return 0, false
	}
	// Atoi parses straight to int, so there is no 64-bit to int narrowing to
	// bound. A value too large for int on this platform comes back as ErrRange
	// and is clamped below like any other big number.
	n, err := strconv.Atoi(v)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			if strings.HasPrefix(v, "-") {
				return 0, true
			}
			return maxHeaderCount, true
		}
		return 0, false
	}
	if n < 0 {
		return 0, true
	}
	if n > maxHeaderCount {
		return maxHeaderCount, true
	}
	return n, true
}

// headerUnix reads a UNIX timestamp header such as Pexels' X-Ratelimit-Reset.
// It returns the zero time when the header is absent, invalid or not
// positive. Years beyond 9999 are rejected as not being a timestamp at all.
func headerUnix(h http.Header, name string) time.Time {
	v := strings.TrimSpace(h.Get(name))
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 || n > maxUnixTime {
		return time.Time{}
	}
	return time.Unix(n, 0)
}

// maxUnixTime is 9999-12-31T23:59:59Z.
const maxUnixTime = 253402300799

// requireBase checks a provider base URL once, at construction, so request
// building later cannot fail with an error that quotes the URL.
func requireBase(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("stockmedia: invalid base URL")
	}
	return strings.TrimRight(raw, "/"), nil
}
