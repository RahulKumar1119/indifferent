package stockmedia

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

var orientations = []Orientation{OrientationPortrait, OrientationLandscape, OrientationSquare}

func genAsset(t *rapid.T, label string) Asset {
	w := rapid.IntRange(1, 4000).Draw(t, label+"-w")
	h := rapid.IntRange(1, 4000).Draw(t, label+"-h")
	return Asset{ID: rapid.StringMatching(`[a-z0-9]{1,6}`).Draw(t, label+"-id"), Width: w, Height: h, Orientation: orientationOf(w, h)}
}

// sortByOrientation must be a stable permutation that puts closer matches
// first, for any list and any preference.
func TestProperty_SortByOrientation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 30).Draw(t, "n")
		assets := make([]Asset, n)
		for i := range assets {
			assets[i] = genAsset(t, fmt.Sprintf("a%d", i))
			assets[i].ID = fmt.Sprintf("%d-%s", i, assets[i].ID) // unique, and records the input position
		}
		pref := rapid.SampledFrom(append([]Orientation{OrientationAny, ""}, orientations...)).Draw(t, "pref")

		in := append([]Asset(nil), assets...)
		sortByOrientation(assets, pref)

		// Permutation: same multiset of ids.
		seen := map[string]int{}
		for _, a := range in {
			seen[a.ID]++
		}
		for _, a := range assets {
			seen[a.ID]--
		}
		for id, c := range seen {
			if c != 0 {
				t.Fatalf("not a permutation: %s off by %d", id, c)
			}
		}
		// Non-decreasing rank, and stable within a rank.
		pos := map[string]int{}
		for i, a := range in {
			pos[a.ID] = i
		}
		for i := 1; i < len(assets); i++ {
			ra, rb := orientationRank(pref, assets[i-1].Orientation), orientationRank(pref, assets[i].Orientation)
			if ra > rb {
				t.Fatalf("rank went from %d to %d at %d", ra, rb, i)
			}
			if ra == rb && pos[assets[i-1].ID] > pos[assets[i].ID] {
				t.Fatalf("unstable: %s came after %s within one rank", assets[i-1].ID, assets[i].ID)
			}
		}
	})
}

// pickRendition must return an element of the input; when one fits the limit
// it returns the largest that fits, otherwise the smallest.
func TestProperty_PickRendition(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 8).Draw(t, "n")
		limit := rapid.IntRange(100, 4000).Draw(t, "limit")
		rs := make([]rendition, n)
		for i := range rs {
			rs[i] = rendition{
				URL:    rapid.SampledFrom([]string{"", fmt.Sprintf("https://x.test/%d.mp4", i)}).Draw(t, "url"),
				Format: rapid.SampledFrom([]string{"video/mp4", "video/webm", "video/x-flv", ""}).Draw(t, "fmt"),
				Width:  rapid.IntRange(-5, 4000).Draw(t, "w"),
				Height: rapid.IntRange(-5, 4000).Draw(t, "h"),
			}
		}
		usable := func(r rendition) bool {
			return r.URL != "" && r.Width > 0 && r.Height > 0 && allowedFormats[r.Format]
		}
		got, ok := pickRendition(rs, limit)

		var cands, fits []rendition
		for _, r := range rs {
			if usable(r) {
				cands = append(cands, r)
				if r.longSide() <= limit {
					fits = append(fits, r)
				}
			}
		}
		if ok != (len(cands) > 0) {
			t.Fatalf("ok = %v with %d usable renditions", ok, len(cands))
		}
		if !ok {
			return
		}
		if !usable(got) {
			t.Fatalf("picked an unusable rendition %+v", got)
		}
		if len(fits) > 0 {
			if got.longSide() > limit {
				t.Fatalf("picked %+v over the limit %d though %d fit", got, limit, len(fits))
			}
			for _, r := range fits {
				if r.pixels() > got.pixels() {
					t.Fatalf("picked %+v but %+v fits and is larger", got, r)
				}
			}
		} else {
			for _, r := range cands {
				if r.pixels() < got.pixels() {
					t.Fatalf("nothing fits, picked %+v but %+v is smaller", got, r)
				}
			}
		}
	})
}

// A normalized query is stable: normalizing twice changes nothing, the result
// has no control characters or edge/double spaces, and it is within the limit.
func TestProperty_NormalizeParamsIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		q := rapid.String().Draw(t, "q")
		p, err := normalizeParams(SearchParams{Query: q, Type: MediaPhoto})
		if err != nil {
			return
		}
		again, err := normalizeParams(p)
		if err != nil || again != p {
			t.Fatalf("not idempotent: %+v -> %+v (%v)", p, again, err)
		}
		if p.Query != strings.TrimSpace(p.Query) || strings.Contains(p.Query, "  ") || len([]rune(p.Query)) > MaxQueryRunes {
			t.Fatalf("bad normalized query %q", p.Query)
		}
		for _, r := range p.Query {
			if r < 0x20 || r == 0x7f {
				t.Fatalf("control character %U in %q", r, p.Query)
			}
		}
	})
}

// scrub must remove the secret from any text, whatever characters it holds.
func TestProperty_ScrubRemovesSecret(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		secret := rapid.StringMatching(`[A-Za-z0-9+/=_.-]{6,30}`).Draw(t, "secret")
		pre := rapid.StringMatching(`[ -~]{0,20}`).Draw(t, "pre")
		post := rapid.StringMatching(`[ -~]{0,20}`).Draw(t, "post")
		for _, form := range []string{secret, url.QueryEscape(secret)} {
			got := scrub(pre+form+post, Secret(secret))
			if strings.Contains(got, secret) || strings.Contains(got, url.QueryEscape(secret)) {
				t.Fatalf("scrub left the secret in %q", got)
			}
		}
	})
}

// The URL policy must reject every URL whose host is not on the allowlist,
// including hosts built to look like an allowed one.
func TestProperty_URLPolicyRejectsForeignHosts(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		label := rapid.StringMatching(`[a-z][a-z0-9]{0,10}`).Draw(t, "label")
		tld := rapid.SampledFrom([]string{"com", "net", "io", "test", "example"}).Draw(t, "tld")
		host := label + "." + tld
		for _, allowed := range providerHosts[ProviderPexels] {
			if host == allowed || strings.HasSuffix(host, "."+allowed) {
				return // this draw happens to be a legitimate host
			}
		}
		if err := checkURL(defaultURLPolicy, ProviderPexels, "https://"+host+"/x.mp4"); err == nil {
			t.Fatalf("host %q was accepted", host)
		}
		// Embedding an allowed domain does not make a host allowed.
		for _, allowed := range providerHosts[ProviderPexels] {
			for _, sneaky := range []string{allowed + "." + host, label + allowed, host + "/" + allowed} {
				if err := checkURL(defaultURLPolicy, ProviderPexels, "https://"+sneaky+"/x.mp4"); err == nil && !strings.HasSuffix(strings.SplitN(sneaky, "/", 2)[0], allowed) {
					t.Fatalf("sneaky host %q was accepted", sneaky)
				}
			}
		}
	})
}
