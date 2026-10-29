package stockmedia

import (
	"errors"
	"testing"
)

// realAsset passes ValidateAsset with the production policy.
func realAsset() Asset {
	return Asset{
		Provider: ProviderPexels, ID: "2499611", Type: MediaVideo,
		PageURL:  "https://www.pexels.com/video/2499611/",
		MediaURL: "https://player.vimeo.com/external/342571552.hd.mp4?s=abc",
		Format:   "video/mp4", Width: 1080, Height: 1920, Duration: 22,
		Orientation: OrientationPortrait,
	}
}

func reasonOf(t *testing.T, err error) string {
	t.Helper()
	var ia *InvalidAssetError
	if !errors.As(err, &ia) {
		t.Fatalf("err = %v, want *InvalidAssetError", err)
	}
	if !errors.Is(err, ErrInvalidAsset) {
		t.Errorf("errors.Is(err, ErrInvalidAsset) = false")
	}
	return ia.Reason
}

func TestValidateAsset_Accepts(t *testing.T) {
	photo := realAsset()
	photo.Type, photo.Format, photo.Duration = MediaPhoto, "image/jpeg", 0
	photo.MediaURL = "https://images.pexels.com/photos/1/a.jpeg"
	pixabay := Asset{
		Provider: ProviderPixabay, ID: "125", Type: MediaVideo,
		PageURL:  "https://pixabay.com/videos/id-125/",
		MediaURL: "https://cdn.pixabay.com/video/2015/08/08/125-135736646_medium.mp4",
		Format:   "video/mp4", Width: 1920, Height: 1080, Duration: 12,
	}
	for name, a := range map[string]Asset{"pexels video": realAsset(), "pexels photo": photo, "pixabay video": pixabay} {
		if err := ValidateAsset(a); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestValidateAsset_Rejects(t *testing.T) {
	mutate := func(f func(a *Asset)) Asset { a := realAsset(); f(&a); return a }
	tests := []struct {
		name   string
		asset  Asset
		reason string
	}{
		{"unknown provider", mutate(func(a *Asset) { a.Provider = "unsplash" }), "provider"},
		{"missing id", mutate(func(a *Asset) { a.ID = "" }), "id"},
		{"unknown type", mutate(func(a *Asset) { a.Type = "audio" }), "type"},
		{"video with image format", mutate(func(a *Asset) { a.Format = "image/jpeg" }), "format"},
		{"photo with video format", mutate(func(a *Asset) { a.Type = MediaPhoto }), "format"},
		{"format the renderer cannot take", mutate(func(a *Asset) { a.Format = "video/x-matroska" }), "format"},
		{"gif", mutate(func(a *Asset) { a.Type, a.Format = MediaPhoto, "image/gif" }), "format"},
		{"video without duration", mutate(func(a *Asset) { a.Duration = 0 }), "duration"},
		{"zero width", mutate(func(a *Asset) { a.Width = 0 }), "dimensions"},
		{"negative height", mutate(func(a *Asset) { a.Height = -5 }), "dimensions"},
		{"too small", mutate(func(a *Asset) { a.Width, a.Height = 540, 960 }), "resolution"},
		{"empty media url", mutate(func(a *Asset) { a.MediaURL = "" }), "url"},
		{"unparseable media url", mutate(func(a *Asset) { a.MediaURL = "https://%zz" }), "url"},
		{"http media url", mutate(func(a *Asset) { a.MediaURL = "http://player.vimeo.com/x.mp4" }), "url"},
		{"file url", mutate(func(a *Asset) { a.MediaURL = "file:///etc/passwd" }), "url"},
		{"scheme-relative url", mutate(func(a *Asset) { a.MediaURL = "//player.vimeo.com/x.mp4" }), "url"},
		{"no host", mutate(func(a *Asset) { a.MediaURL = "https:///x.mp4" }), "url"},
		{"credentials in url", mutate(func(a *Asset) { a.MediaURL = "https://user:pw@player.vimeo.com/x.mp4" }), "url"},
		{"ipv4 literal", mutate(func(a *Asset) { a.MediaURL = "https://169.254.169.254/latest/meta-data/" }), "host"},
		{"ipv6 literal", mutate(func(a *Asset) { a.MediaURL = "https://[::1]/x.mp4" }), "host"},
		{"localhost", mutate(func(a *Asset) { a.MediaURL = "https://localhost/x.mp4" }), "host"},
		{"unexpected port", mutate(func(a *Asset) { a.MediaURL = "https://player.vimeo.com:8443/x.mp4" }), "host"},
		{"other domain", mutate(func(a *Asset) { a.MediaURL = "https://evil.example.com/x.mp4" }), "host"},
		{"lookalike suffix", mutate(func(a *Asset) { a.MediaURL = "https://notvimeo.com/x.mp4" }), "host"},
		{"allowed domain as a prefix", mutate(func(a *Asset) { a.MediaURL = "https://vimeo.com.evil.example/x.mp4" }), "host"},
		{"pexels asset on a pixabay host", mutate(func(a *Asset) { a.MediaURL = "https://cdn.pixabay.com/x.mp4" }), "host"},
		{"bad page url", mutate(func(a *Asset) { a.PageURL = "http://www.pexels.com/video/1/" }), "url"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := reasonOf(t, ValidateAsset(tc.asset)); got != tc.reason {
				t.Errorf("reason = %q, want %q", got, tc.reason)
			}
		})
	}
}

func TestValidateAsset_HostMatchingIsCaseInsensitiveAndAllowsSubdomains(t *testing.T) {
	for _, host := range []string{"PLAYER.VIMEO.COM", "vimeo.com", "a.b.c.vimeocdn.com", "Images.Pexels.Com"} {
		a := realAsset()
		a.MediaURL = "https://" + host + "/x.mp4"
		if err := ValidateAsset(a); err != nil {
			t.Errorf("%s should be allowed: %v", host, err)
		}
	}
}

func TestValidateAsset_ErrorDoesNotContainTheURL(t *testing.T) {
	a := realAsset()
	a.MediaURL = "https://internal.example.com/secret/path?token=abc123"
	err := ValidateAsset(a)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, leak := range []string{"abc123", "/secret/path", "internal.example.com/secret"} {
		if contains(err.Error(), leak) {
			t.Errorf("error leaks URL content %q: %v", leak, err)
		}
	}
}

func contains(s, sub string) bool { return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestExtensionFormat(t *testing.T) {
	for url, want := range map[string]string{
		"https://x.test/a.jpg":              "image/jpeg",
		"https://x.test/a.JPEG":             "image/jpeg",
		"https://x.test/a.png?w=1":          "image/png",
		"https://x.test/a.webp":             "image/webp",
		"https://x.test/a.gif":              "",
		"https://x.test/noext":              "",
		"https://x.test/a.jpg/":             "",
		"https://x.test/get/ed6a99_1280.jp": "",
		"::bad::":                           "",
	} {
		if got := extensionFormat(url); got != want {
			t.Errorf("extensionFormat(%q) = %q, want %q", url, got, want)
		}
	}
}
