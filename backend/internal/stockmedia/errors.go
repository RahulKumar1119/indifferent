package stockmedia

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors. Use errors.Is; provider failures wrap the matching one.
var (
	// ErrInvalidParams means the search parameters were rejected before any
	// request was made.
	ErrInvalidParams = errors.New("invalid search parameters")
	// ErrUnsupportedLanguage means the search text is written only in a script
	// neither provider can search (Hindi and the other Indian scripts). It
	// comes wrapped in ErrInvalidParams. The providers answer such queries with
	// zero results and no error, so without this check the failure would look
	// like "no matching media". Callers should search with English keywords.
	// A query that mixes English and Hindi is not an error: the Hindi words
	// are ignored by the providers and the English ones still match.
	ErrUnsupportedLanguage = errors.New("search text has no English words: the stock providers return no results for Hindi or other Indian-script text alone")
	// ErrNotConfigured means the provider has no API key or is not registered.
	ErrNotConfigured = errors.New("stock media provider not configured")
	// ErrRateLimited means the provider refused the request for volume, or the
	// service is holding off calling it after such a refusal.
	ErrRateLimited = errors.New("stock media provider rate limited")
	// ErrCoolingDown is joined with ErrRateLimited when no request was made
	// because the provider is in a cooldown.
	ErrCoolingDown = errors.New("provider in rate-limit cooldown")
	// ErrAuth means the provider rejected the API key.
	ErrAuth = errors.New("stock media provider rejected the API key")
	// ErrBadRequest means the provider rejected the request itself.
	ErrBadRequest = errors.New("stock media provider rejected the request")
	// ErrUnavailable means the provider failed or could not be reached. It is
	// the one kind a retry policy tries again whatever the provider said (see
	// RetryPolicy).
	ErrUnavailable = errors.New("stock media provider unavailable")
	// ErrNotFound means the provider has no asset with the requested id, for
	// example because it was removed after the user chose it.
	ErrNotFound = errors.New("stock media asset not found")
	// ErrBadResponse means the provider answered with something unusable.
	ErrBadResponse = errors.New("stock media provider returned an unusable response")
	// ErrAllProvidersFailed means every queried provider returned an error.
	ErrAllProvidersFailed = errors.New("all stock media providers failed")
	// ErrInvalidAsset means an asset failed validation.
	ErrInvalidAsset = errors.New("invalid stock media asset")
	// ErrAssetUnavailable means an asset's file could not be fetched.
	ErrAssetUnavailable = errors.New("stock media asset unavailable")
)

// ErrorKind classifies a provider failure.
type ErrorKind string

// Provider failure kinds.
const (
	KindBadRequest  ErrorKind = "bad_request"
	KindNotFound    ErrorKind = "not_found"
	KindAuth        ErrorKind = "auth"
	KindRateLimited ErrorKind = "rate_limited"
	KindUnavailable ErrorKind = "unavailable"
	KindBadResponse ErrorKind = "bad_response"
)

// ProviderError is a failure talking to one provider. Its message never
// contains a request URL or an API key.
type ProviderError struct {
	Provider ProviderName
	Kind     ErrorKind
	// Status is the HTTP status, or 0 when there was no response.
	Status int
	// RetryAfter is how long the provider asked callers to wait, if it said.
	RetryAfter time.Duration
	Err        error
}

func (e *ProviderError) Error() string {
	msg := fmt.Sprintf("%s: %s", e.Provider, e.Kind)
	if e.Status != 0 {
		msg += fmt.Sprintf(" (HTTP %d)", e.Status)
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *ProviderError) Unwrap() error { return e.Err }

// Is lets errors.Is match a ProviderError against the sentinel for its kind.
func (e *ProviderError) Is(target error) bool {
	switch e.Kind {
	case KindBadRequest:
		return target == ErrBadRequest
	case KindNotFound:
		return target == ErrNotFound
	case KindAuth:
		return target == ErrAuth
	case KindRateLimited:
		return target == ErrRateLimited
	case KindUnavailable:
		return target == ErrUnavailable
	case KindBadResponse:
		return target == ErrBadResponse
	}
	return false
}

// Retryable reports whether trying the same request again could work whatever
// the provider said. Rate limiting is not retryable in this sense: how long to
// wait is the provider's to say, so it is left to RetryPolicy, which retries a
// 429 only when the provider named a short wait, and to the service's cooldown.
// Hammering a limited API is how keys get revoked.
func (e *ProviderError) Retryable() bool { return e.Kind == KindUnavailable }

// ProviderFailure records which provider failed in a multi-provider search.
type ProviderFailure struct {
	Provider ProviderName
	Err      error
}

// InvalidAssetError says why an asset was rejected. Reason is a short stable
// code suitable for counting; Detail is for humans and never holds a full URL.
type InvalidAssetError struct {
	Reason string
	Detail string
}

func (e *InvalidAssetError) Error() string {
	return fmt.Sprintf("invalid stock media asset (%s): %s", e.Reason, e.Detail)
}

func (e *InvalidAssetError) Is(target error) bool { return target == ErrInvalidAsset }

func invalidAsset(reason, format string, args ...any) error {
	return &InvalidAssetError{Reason: reason, Detail: fmt.Sprintf(format, args...)}
}
