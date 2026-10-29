package stockmedia

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"
)

// MinLongSide is the smallest longer side, in pixels, an asset may have. A
// 1080x1920 frame built from a smaller source is visibly soft, and providers
// list plenty of larger files, so small ones are dropped (and counted in
// SearchResult.Skipped) instead of offered.
const MinLongSide = 1280

// allowedFormats are the MIME types the render pipeline can take.
var allowedFormats = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"video/mp4":  true,
	"video/webm": true,
}

// providerHosts lists, per provider, the domains its URLs may point at. A
// host matches when it equals an entry or is a subdomain of one.
//
// This is the guard for the step that downloads MediaURL on our servers: a
// provider response can only send us to the provider's own CDN, never to an
// internal address. Pexels serves video files through Vimeo's CDN, which is
// why those domains appear. If a provider moves to a new CDN, assets are
// dropped with reason "host" and counted, which is visible in tests and logs
// before it becomes a user-facing gap.
var providerHosts = map[ProviderName][]string{
	ProviderPexels:  {"pexels.com", "vimeo.com", "vimeocdn.com"},
	ProviderPixabay: {"pixabay.com"},
}

// URLPolicy decides whether a provider URL may be used. It runs after the
// scheme has been checked as https.
type URLPolicy func(provider ProviderName, u *url.URL) error

// defaultURLPolicy requires a named host on the provider's allowlist, the
// default port, and no embedded credentials. IP literals are refused outright.
func defaultURLPolicy(provider ProviderName, u *url.URL) error {
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "":
		return invalidAsset("url", "URL has no host")
	case u.User != nil:
		return invalidAsset("url", "URL contains credentials")
	case u.Port() != "" && u.Port() != "443":
		return invalidAsset("host", "unexpected port %s", u.Port())
	case net.ParseIP(host) != nil || host == "localhost":
		return invalidAsset("host", "IP address or localhost not allowed")
	}
	for _, allowed := range providerHosts[provider] {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	// The rejected host is not in the message: it came from a provider
	// response or a redirect, so it is untrusted text. The service logs it
	// separately (see Service.finish).
	return invalidAsset("host", "host is not a %s domain", provider)
}

// checkURL requires https and then applies the policy.
func checkURL(policy URLPolicy, provider ProviderName, raw string) error {
	if raw == "" {
		return invalidAsset("url", "empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return invalidAsset("url", "unparseable URL")
	}
	if u.Scheme != "https" {
		return invalidAsset("url", "URL must use https")
	}
	return policy(provider, u)
}

// ValidateAsset checks an asset against the default policy: known provider,
// id, type and format; https URLs on the provider's own domains; real
// dimensions at least MinLongSide on the longer side; and a duration for video.
// The error is an *InvalidAssetError whose Reason names the failed check.
func ValidateAsset(a Asset) error {
	return validateAsset(a, defaultURLPolicy, MinLongSide)
}

func validateAsset(a Asset, policy URLPolicy, minLongSide int) error {
	if _, ok := providerHosts[a.Provider]; !ok {
		return invalidAsset("provider", "unknown provider %q", a.Provider)
	}
	if a.ID == "" {
		return invalidAsset("id", "missing id")
	}
	switch a.Type {
	case MediaPhoto:
		if !strings.HasPrefix(a.Format, "image/") {
			return invalidAsset("format", "photo has format %q", a.Format)
		}
	case MediaVideo:
		if !strings.HasPrefix(a.Format, "video/") {
			return invalidAsset("format", "video has format %q", a.Format)
		}
		if a.Duration <= 0 {
			return invalidAsset("duration", "video has no duration")
		}
	default:
		return invalidAsset("type", "unknown media type %q", a.Type)
	}
	if !allowedFormats[a.Format] {
		return invalidAsset("format", "format %q is not supported", a.Format)
	}
	if a.Width <= 0 || a.Height <= 0 {
		return invalidAsset("dimensions", "size %dx%d", a.Width, a.Height)
	}
	if long := max(a.Width, a.Height); long < minLongSide {
		return invalidAsset("resolution", "%dx%d is under %dpx on the longer side", a.Width, a.Height, minLongSide)
	}
	if err := checkURL(policy, a.Provider, a.MediaURL); err != nil {
		return err
	}
	return checkURL(policy, a.Provider, a.PageURL)
}

// extensionFormat maps a URL's file extension to an image MIME type. It
// returns "" for anything it does not know, so callers drop the asset
// rather than guess.
func extensionFormat(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	}
	return ""
}

// describe is a short, URL-free label for logs and error text.
func (a Asset) describe() string {
	return fmt.Sprintf("%s/%s/%s", a.Provider, a.Type, a.ID)
}
