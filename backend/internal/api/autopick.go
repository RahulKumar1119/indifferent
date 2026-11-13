package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Auto-pick (#302): when a plan is made, each scene gets a stock picture,
// chosen from the same search the picker makes when it opens. The user can
// replace any of them. It never fails or noticeably slows plan creation: what
// is not found in time stays empty, as before.
const (
	// autoPickBudget is the most auto-pick may add to a plan request.
	autoPickBudget = 4 * time.Second
	// autoPickDeadline is how long after the request started auto-pick must
	// be done, whatever is left of its budget. The API Lambda stops at 30s and
	// the gateway at 29s.
	autoPickDeadline = 26 * time.Second
	// autoPickConcurrency is how many searches one plan runs at once.
	autoPickConcurrency = 3
	// autoPickReserve is the provider quota left for people using the picker:
	// auto-pick does not call a provider that last reported fewer requests left.
	autoPickReserve = 30
	// autoPickProvider is the only library used: Pexels has no production key,
	// and allows 200 requests an hour for the whole service.
	autoPickProvider = stockmedia.ProviderPixabay
	// The picker's opening search (story-media-picker.component.ts): page 1, 12
	// a page, portrait. Asking the same means the results are cached for the
	// picker, and the picker's first page shows the picture that was chosen.
	autoPickPerPage = 12
)

// Why a scene was left without a picture. A fixed list: these are logged and
// stored on the plan record, and become metric dimensions.
const (
	autoPickNoResults = "no_results"
	autoPickQuota     = "quota"
	autoPickError     = "error"
	autoPickBudgetOut = "budget"
	autoPickSensitive = "skipped_sensitive"
)

// Patterns for search words that may be personal data. Deliberately broad on
// what they catch and narrow on what they read: a skipped scene only means the
// person picks that picture themselves, while a search sends the words to a
// third party.
var (
	// An email address or a handle: an "@" with something after it.
	sensitiveAt = regexp.MustCompile(`@\S`)
	// A link: a scheme, "www.", or a name followed by a dot and a domain ending.
	sensitiveURL = regexp.MustCompile(`(?i)(\b[a-z][a-z0-9+.-]*://|\bwww\.|\b[a-z0-9-]+\.(com|net|org|in|io|co|me|info|biz|app|dev|ly|gl|xyz|edu|gov)\b)`)
	// A phone number: seven or more digits, which may be split by spaces,
	// dots, dashes or brackets, with an optional leading "+".
	sensitivePhone = regexp.MustCompile(`\+?\(?\d(?:[\s().-]*\d){6,}`)
)

// looksSensitive reports whether q may hold contact details or an account,
// phone or id number. Any digit counts, Devanagari ones included, so "९८७६५४३२१०"
// is caught like "9876543210". A year or a small count ("1920s", "2 dogs") is
// not.
func looksSensitive(q string) bool {
	var ascii strings.Builder
	run := 0
	for _, r := range q {
		if unicode.IsDigit(r) {
			run++
			if run >= 6 {
				return true // an id, account or card number
			}
			// The patterns below read ASCII digits; which digit does not matter.
			ascii.WriteByte('0')
			continue
		}
		run = 0
		ascii.WriteRune(r)
	}
	s := ascii.String()
	return sensitiveAt.MatchString(s) || sensitiveURL.MatchString(s) || sensitivePhone.MatchString(s)
}

// autoPickSummary is stored on the plan record for the funnel (#305). Counts
// only: no search words.
type autoPickSummary struct {
	Picked  int            `json:"picked"`
	Empty   int            `json:"empty"`
	Reasons map[string]int `json:"reasons,omitempty"`
	Ms      int64          `json:"ms"`
}

// autoPick gives scenes of plan a picture where it can, in place, and returns
// what it did. It returns nil without searching when auto-pick is off or there
// is no stock service. start is when the request began.
func (h *APIHandler) autoPick(ctx context.Context, plan *storyplan.Plan, start time.Time) *autoPickSummary {
	if !h.AutoPick || h.StockMedia == nil || plan == nil || len(plan.Scenes) == 0 {
		return nil
	}
	providers := h.StockMedia.Providers()
	if !containsProvider(providers, autoPickProvider) {
		return nil
	}
	began := time.Now()
	budget := autoPickBudget
	if left := autoPickDeadline - time.Since(start); left < budget {
		budget = left
	}
	summary := &autoPickSummary{Reasons: map[string]int{}}
	if budget <= 0 {
		summary.Empty = len(plan.Scenes)
		summary.Reasons[autoPickBudgetOut] = len(plan.Scenes)
		h.logAutoPick(summary, len(plan.Scenes))
		return summary
	}
	// Detached from the request so a client hanging up mid-search does not
	// leave half the cache unwritten; bounded by the budget either way.
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
	defer cancel()
	actx = stockmedia.WithQuotaReserve(actx, autoPickReserve)

	type found struct {
		assets []stockmedia.Asset
		reason string
	}
	results := make([]found, len(plan.Scenes))
	sem := make(chan struct{}, autoPickConcurrency)
	// A scene whose search words look like personal data (an email, a link, a
	// phone or account number) is not searched at all: nothing about it is
	// sent to the provider unless the person opens the picker themselves.
	for i := range plan.Scenes {
		if plan.Scenes[i].Media == nil && looksSensitive(autoPickParams(plan.Scenes[i]).Query) {
			results[i].reason = autoPickSensitive
		}
	}
	var wg sync.WaitGroup
	// Searches start in scene order, so when time runs out it is the last
	// scenes that go without, not a random few.
scenes:
	for i := range plan.Scenes {
		sc := plan.Scenes[i]
		if sc.Media != nil || results[i].reason != "" {
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-actx.Done():
			break scenes // the rest are reported as out of budget below
		}
		wg.Add(1)
		go func(i int, sc storyplan.Scene) {
			defer wg.Done()
			defer func() { <-sem }()
			res, err := h.StockMedia.Search(actx, autoPickProvider, autoPickParams(sc))
			switch {
			case err != nil:
				results[i].reason = autoPickFailure(actx, err)
			case len(res.Assets) == 0:
				results[i].reason = autoPickNoResults
			default:
				results[i].assets = res.Assets
			}
		}(i, sc)
	}
	wg.Wait()

	// Choose in scene order, so the same picture is not used twice and the
	// choice does not depend on which search finished first.
	used := map[string]bool{}
	for i := range plan.Scenes {
		if plan.Scenes[i].Media != nil {
			used[mediaKey(*plan.Scenes[i].Media)] = true
		}
	}
	for i := range plan.Scenes {
		sc := &plan.Scenes[i]
		if sc.Media != nil {
			continue
		}
		r := results[i]
		if r.reason == "" && r.assets == nil {
			r.reason = autoPickBudgetOut
		}
		if a, ok := chooseAsset(r.assets, sc, used); ok {
			ref := storyplan.MediaRef{Provider: string(a.Provider), Type: string(a.Type), ID: a.ID}
			if ref.Valid() {
				sc.Media = &ref
				sc.MediaSource = storyplan.MediaSourceAuto
				used[mediaKey(ref)] = true
				summary.Picked++
				continue
			}
		}
		if r.reason == "" {
			r.reason = autoPickNoResults // every result was already used, or unusable
		}
		summary.Empty++
		summary.Reasons[r.reason]++
	}
	summary.Ms = time.Since(began).Milliseconds()
	h.logAutoPick(summary, len(plan.Scenes))
	return summary
}

// autoPickAttachTimeout bounds looking up the picked assets for the response.
// They were just stored in the asset cache by the searches, so this is cache
// reads, not provider calls.
const autoPickAttachTimeout = 2 * time.Second

// attachAutoPicked fills resp.Media for a plan auto-pick just gave pictures,
// so the page can show them at once. It keeps to the deadline, and a lookup
// that would reach a provider yields to people as the searches did.
func (h *APIHandler) attachAutoPicked(ctx context.Context, resp *StoryPlanResponse, start time.Time) {
	timeout := autoPickAttachTimeout
	if left := autoPickDeadline + time.Second - time.Since(start); left < timeout {
		timeout = left
	}
	if timeout <= 0 {
		return // the page looks them up when it opens the plan
	}
	actx, cancel := context.WithTimeout(stockmedia.WithQuotaReserve(ctx, autoPickReserve), timeout)
	defer cancel()
	h.attachMedia(actx, resp)
}

// autoPickParams is the picker's opening search for a scene.
func autoPickParams(sc storyplan.Scene) stockmedia.SearchParams {
	typ := stockmedia.MediaPhoto
	if sc.MediaType == storyplan.MediaVideo {
		typ = stockmedia.MediaVideo
	}
	return stockmedia.SearchParams{
		Query:       sc.SearchQuery(stockmedia.MaxQueryRunes),
		Type:        typ,
		Orientation: stockmedia.OrientationPortrait,
		Page:        1,
		PerPage:     autoPickPerPage,
	}
}

// autoPickFailure names why a search gave nothing: the quota was kept for
// people (reserve or cooldown), the budget ran out, or anything else.
func autoPickFailure(ctx context.Context, err error) string {
	var pe *stockmedia.ProviderError
	switch {
	case errors.Is(err, stockmedia.ErrQuotaReserved), errors.Is(err, stockmedia.ErrCoolingDown),
		errors.As(err, &pe) && pe.Kind == stockmedia.KindRateLimited:
		return autoPickQuota
	case ctx.Err() != nil:
		return autoPickBudgetOut
	}
	return autoPickError
}

// chooseAsset picks the first asset no earlier scene uses. For a video scene a
// clip at least as long as the scene comes first, so it need not loop.
func chooseAsset(assets []stockmedia.Asset, sc *storyplan.Scene, used map[string]bool) (stockmedia.Asset, bool) {
	var fallback *stockmedia.Asset
	for i := range assets {
		a := &assets[i]
		ref := storyplan.MediaRef{Provider: string(a.Provider), Type: string(a.Type), ID: a.ID}
		if used[mediaKey(ref)] || !ref.Valid() {
			continue
		}
		if a.Type != stockmedia.MediaVideo || a.Duration >= sc.Duration {
			return *a, true
		}
		if fallback == nil {
			fallback = a
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return stockmedia.Asset{}, false
}

func mediaKey(m storyplan.MediaRef) string { return m.Provider + "|" + m.Type + "|" + m.ID }

func containsProvider(ps []stockmedia.ProviderName, p stockmedia.ProviderName) bool {
	for _, x := range ps {
		if x == p {
			return true
		}
	}
	return false
}

// logAutoPick writes one line per plan, and the metrics. Counts and reason
// codes only: no search words, no story.
func (h *APIHandler) logAutoPick(s *autoPickSummary, scenes int) {
	log.Printf("storyplan_autopick scenes=%d picked=%d empty=%d reasons=%s ms=%d", scenes, s.Picked, s.Empty, formatCounts(s.Reasons), s.Ms)
	h.Metrics.AutoPicked(s.Picked, s.Reasons)
}

func formatCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k, n := range m {
		if n > 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "-"
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s:%d", k, m[k])
	}
	return strings.Join(parts, ",")
}
