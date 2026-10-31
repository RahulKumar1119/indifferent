package stockmedia

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewServiceFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	quiet := WithLogger(func(string, ...any) {})

	t.Run("both keys", func(t *testing.T) {
		s, err := newServiceFromGetenv(env(map[string]string{EnvPexelsKey: "a", EnvPixabayKey: "b"}), quiet)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(s.Providers()); got != "[pexels pixabay]" {
			t.Errorf("providers = %s", got)
		}
	})
	t.Run("one key leaves the other provider out", func(t *testing.T) {
		s, err := newServiceFromGetenv(env(map[string]string{EnvPixabayKey: "  b  "}), quiet)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(s.Providers()); got != "[pixabay]" {
			t.Errorf("providers = %s, want only pixabay", got)
		}
	})
	t.Run("blank key counts as unset", func(t *testing.T) {
		_, err := newServiceFromGetenv(env(map[string]string{EnvPexelsKey: "   ", EnvPixabayKey: ""}), quiet)
		if !errors.Is(err, ErrNotConfigured) {
			t.Errorf("err = %v, want ErrNotConfigured", err)
		}
	})
	t.Run("no keys names the variables to set", func(t *testing.T) {
		_, err := newServiceFromGetenv(env(nil), quiet)
		if !errors.Is(err, ErrNotConfigured) || !strings.Contains(err.Error(), EnvPexelsKey) || !strings.Contains(err.Error(), EnvPixabayKey) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("keys are not echoed in errors", func(t *testing.T) {
		_, err := newServiceFromGetenv(env(nil), quiet)
		if err != nil && strings.Contains(err.Error(), "sk-") {
			t.Error("unexpected")
		}
	})
}

func TestSecret_NeverPrintsItsValue(t *testing.T) {
	s := Secret("super-secret-value")
	b, _ := s.MarshalText()
	for _, out := range []string{fmt.Sprint(s), fmt.Sprintf("%v", s), fmt.Sprintf("%s", s), fmt.Sprintf("%q", s), fmt.Sprintf("%#v", s), string(b)} {
		if strings.Contains(out, "super-secret-value") {
			t.Errorf("secret leaked: %s", out)
		}
	}
	if s.reveal() != "super-secret-value" {
		t.Error("reveal must return the real value")
	}
}

func TestMemoryCache_BoundEvictionAndTTL(t *testing.T) {
	clock := newFakeClock()
	c := NewMemoryCache(10)
	c.now = clock.Now
	res := SearchResult{Assets: []Asset{{ID: "1"}}}

	for i := 0; i < 50; i++ {
		c.Set(context.Background(), fmt.Sprintf("k%d", i), res, time.Hour+time.Duration(i)*time.Second)
	}
	if n := c.Len(); n > 10 {
		t.Errorf("cache holds %d entries, bound is 10", n)
	}
	// The newest entries outlive the oldest, so the newest must still be there.
	if _, ok := c.Get(context.Background(), "k49"); !ok {
		t.Error("the newest entry was evicted")
	}

	c.Set(context.Background(), "zero", res, 0)
	c.Set(context.Background(), "neg", res, -time.Second)
	if _, ok := c.Get(context.Background(), "zero"); ok {
		t.Error("ttl 0 must store nothing")
	}
	if _, ok := c.Get(context.Background(), "neg"); ok {
		t.Error("negative ttl must store nothing")
	}

	clock.Advance(3 * time.Hour)
	if _, ok := c.Get(context.Background(), "k49"); ok {
		t.Error("expired entry returned")
	}
	if c.Len() > 10 {
		t.Errorf("Len = %d", c.Len())
	}
	if NewMemoryCache(0).max != DefaultCacheEntries {
		t.Error("a non-positive bound should use the default")
	}
}

func TestMemoryCache_OverwriteDoesNotEvict(t *testing.T) {
	c := NewMemoryCache(2)
	r := SearchResult{}
	c.Set(context.Background(), "a", r, time.Hour)
	c.Set(context.Background(), "b", r, time.Hour)
	c.Set(context.Background(), "a", r, time.Hour) // replacing must not push anything out
	if _, ok := c.Get(context.Background(), "b"); !ok {
		t.Error("overwriting an existing key evicted another entry")
	}
}

func TestNormalizeParams(t *testing.T) {
	p, err := normalizeParams(SearchParams{Query: "  a\x00b\tc\n\nd  ", Type: MediaVideo})
	if err != nil {
		t.Fatal(err)
	}
	if p.Query != "a b c d" || p.Orientation != OrientationPortrait || p.Page != 1 || p.PerPage != DefaultPerPage {
		t.Errorf("got %+v", p)
	}
	// The limit counts characters, not bytes: 100 multi-byte runes are fine.
	// Japanese is used because the providers can search it (Hindi they cannot).
	if _, err := normalizeParams(SearchParams{Query: strings.Repeat("東", MaxQueryRunes), Type: MediaPhoto}); err != nil {
		t.Errorf("100 multi-byte characters should be accepted: %v", err)
	}
	if _, err := normalizeParams(SearchParams{Query: strings.Repeat("東", MaxQueryRunes+1), Type: MediaPhoto}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("101 characters: err = %v", err)
	}
}

// Observed against the live Pixabay API on 2026-10-29: Hindi-only queries
// return 0 results (5 of 5 words tried, images and videos), while English
// returns up to 500; a query that mixes the two returns the English results.
func TestNormalizeParams_HindiOnlyQueriesAreRejected(t *testing.T) {
	for _, q := range []string{
		"पहाड़", "बारिश", "मंदिर", "किसान", "पैसा",
		"पहाड़ और नदी",                                  // several Hindi words
		"पहाड़ 2026",                                    // digits do not make it searchable
		"पहाड़, नदी!",                                   // nor does punctuation
		"বাংলা", "தமிழ்", "తెలుగు", "ગુજરાતી", "ਪੰਜਾਬੀ", // other Indian scripts
	} {
		_, err := normalizeParams(SearchParams{Query: q, Type: MediaVideo})
		if !errors.Is(err, ErrUnsupportedLanguage) || !errors.Is(err, ErrInvalidParams) {
			t.Errorf("%q: err = %v, want ErrUnsupportedLanguage wrapped in ErrInvalidParams", q, err)
		}
	}
}

func TestNormalizeParams_MixedAndOtherScriptsPass(t *testing.T) {
	for _, q := range []string{
		"mountain पहाड़", "पहाड़ mountain", "temple मंदिर", // mixed: the English half still searches
		"mandir",                           // transliterated Hindi is plain Latin
		"東京 city", "서울", "Москва", "Αθήνα", // scripts the providers list as supported
		"café", "naïve",
	} {
		if _, err := normalizeParams(SearchParams{Query: q, Type: MediaPhoto}); err != nil {
			t.Errorf("%q should be searchable: %v", q, err)
		}
	}
}

func TestSearch_HindiOnlyQueryNeverReachesAProvider(t *testing.T) {
	prov := okProvider(ProviderPixabay)
	s := testService(t, prov)
	_, err := s.Search(context.Background(), ProviderPixabay, SearchParams{Query: "पहाड़", Type: MediaVideo})
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("Search err = %v", err)
	}
	if _, err := s.SearchAll(context.Background(), SearchParams{Query: "पहाड़", Type: MediaVideo}); !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("SearchAll err = %v", err)
	}
	if prov.callCount() != 0 {
		t.Errorf("provider called %d times for a query that can only return nothing", prov.callCount())
	}
}

func TestCleanText(t *testing.T) {
	for _, tc := range []struct {
		in   string
		max  int
		want string
	}{
		{"  Jane\u0000 \t Doe\n", 100, "Jane Doe"},
		{"<script>alert(1)</script>", 100, "<script>alert(1)</script>"}, // markup is not interpreted here; the UI escapes on output
		{"abcdefghij", 5, "abcde"},
		{"ab cdefghij", 3, "ab"}, // cut mid-word then trimmed
		{"नमस्ते दुनिया", 6, "नमस्ते"},
		{"", 5, ""},
	} {
		if got := cleanText(tc.in, tc.max); got != tc.want {
			t.Errorf("cleanText(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"", 0}, {"30", 30 * time.Second}, {" 5 ", 5 * time.Second}, {"0", 0}, {"-5", 0}, {"soon", 0},
		{now.Add(90 * time.Second).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"), 90 * time.Second},
		{now.Add(-time.Hour).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"), 0},
	} {
		if got := parseRetryAfter(tc.in, now); got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestScrub(t *testing.T) {
	for _, tc := range []struct{ secret, in string }{
		{"abc123", "GET /?key=abc123&q=x failed"},
		{"a+b/c=d", "GET /?key=a%2Bb%2Fc%3Dd failed"},
		{"a+b/c=d", "raw a+b/c=d here"},
	} {
		if got := scrub(tc.in, Secret(tc.secret)); strings.Contains(got, tc.secret) || strings.Contains(got, "a%2Bb") {
			t.Errorf("scrub(%q) = %q still contains the secret", tc.in, got)
		}
	}
	if got := scrub("nothing here", ""); got != "nothing here" {
		t.Errorf("an empty secret must change nothing, got %q", got)
	}
}
