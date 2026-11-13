package storymetrics

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func dashboard(t *testing.T) (body []byte, widgets []map[string]any) {
	t.Helper()
	b, err := Dashboard("ap-south-1")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Start   string
		Widgets []map[string]any
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("the dashboard body is not JSON: %v", err)
	}
	if doc.Start == "" || len(doc.Widgets) == 0 {
		t.Fatalf("no start or no widgets: %s", b)
	}
	return b, doc.Widgets
}

func props(w map[string]any) map[string]any { return w["properties"].(map[string]any) }

// metricsOf returns the rows of a metric widget.
func metricsOf(w map[string]any) [][]any {
	var out [][]any
	for _, r := range props(w)["metrics"].([]any) {
		out = append(out, r.([]any))
	}
	return out
}

func TestTheDashboardNameIsOneCloudWatchAccepts(t *testing.T) {
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,255}$`).MatchString(DashboardName) {
		t.Errorf("%q has characters CloudWatch does not allow in a dashboard name (A-Z a-z 0-9 - _)", DashboardName)
	}
}

// A metric the dashboard names but nothing emits shows an empty graph and no
// error. Every metric the dashboard reads must be one the emitter writes, in the
// namespace it writes it, and the emitter's must all be on the dashboard.
func TestEveryMetricTheDashboardReadsIsOneTheCodeEmitsAndNoneIsLeftOut(t *testing.T) {
	_, widgets := dashboard(t)
	emitted := map[string]bool{}
	for _, n := range Names() {
		emitted[n] = true
	}
	read := map[string]bool{}
	for _, w := range widgets {
		if w["type"] != "metric" {
			continue
		}
		for _, row := range metricsOf(w) {
			if _, isExpr := row[0].(map[string]any); isExpr {
				continue
			}
			if row[0] != Namespace {
				t.Errorf("widget %q reads from the namespace %v, not %q", props(w)["title"], row[0], Namespace)
			}
			name, _ := row[1].(string)
			if !emitted[name] {
				t.Errorf("widget %q reads %q, which nothing emits", props(w)["title"], name)
			}
			read[name] = true
		}
	}
	var missing []string
	for n := range emitted {
		if !read[n] {
			missing = append(missing, n)
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		t.Errorf("emitted but not on the dashboard: %v", missing)
	}
}

// An expression that names an id no row defines is rejected by CloudWatch, or
// worse, evaluates to nothing.
func TestEveryExpressionUsesOnlyIdsItsWidgetDefines(t *testing.T) {
	_, widgets := dashboard(t)
	idPat := regexp.MustCompile(`\b[a-z][a-z0-9_]*\b`)
	for _, w := range widgets {
		if w["type"] != "metric" {
			continue
		}
		title := props(w)["title"]
		ids := map[string]bool{}
		var exprs []string
		for _, row := range metricsOf(w) {
			var opts map[string]any
			if m, ok := row[0].(map[string]any); ok {
				opts = m
				if e, ok := m["expression"].(string); ok {
					exprs = append(exprs, e)
				}
			} else if last, ok := row[len(row)-1].(map[string]any); ok {
				opts = last
			}
			if id, ok := opts["id"].(string); ok {
				if ids[id] {
					t.Errorf("widget %q defines the id %q twice", title, id)
				}
				if !regexp.MustCompile(`^[a-z][A-Za-z0-9_]*$`).MatchString(id) {
					t.Errorf("widget %q: id %q must start with a lowercase letter and use letters, digits and _", title, id)
				}
				ids[id] = true
			}
		}
		for _, e := range exprs {
			if strings.HasPrefix(e, "SEARCH(") {
				continue
			}
			for _, tok := range idPat.FindAllString(e, -1) {
				// A lower-case function name (fill, sum) is not an id either, so it is
				// reported here too: CloudWatch wants functions in upper case.
				if !ids[tok] {
					t.Errorf("widget %q: the expression %q uses %q, which no row of the widget defines", title, e, tok)
				}
			}
		}
	}
}

// Each rate is checked against its definition in the README, so a change to a
// formula is a change someone made on purpose.
func TestTheFiveRatesAreTheOnesTheDocumentationDefines(t *testing.T) {
	_, widgets := dashboard(t)
	byTitle := map[string]map[string]any{}
	for _, w := range widgets {
		if w["type"] == "metric" {
			byTitle[props(w)["title"].(string)] = w
		}
	}
	idsToNames := func(w map[string]any) map[string]string {
		out := map[string]string{}
		for _, row := range metricsOf(w) {
			if _, isExpr := row[0].(map[string]any); isExpr {
				continue
			}
			if opts, ok := row[len(row)-1].(map[string]any); ok {
				out[opts["id"].(string)] = row[1].(string)
			}
		}
		return out
	}
	formula := func(w map[string]any) string {
		for _, row := range metricsOf(w) {
			if m, ok := row[0].(map[string]any); ok {
				if e, ok := m["expression"].(string); ok && !strings.HasPrefix(e, "SEARCH(") {
					return e
				}
			}
		}
		return ""
	}
	for _, tc := range []struct {
		titleStart  string
		numerator   []string // metric names that make up the top of the fraction
		denominator []string
		formula     string
	}{
		{"1. Story to video", []string{VideosCompleted}, []string{PlansReady, PlansFailed}, "100 * FILL(done, 0) / (FILL(plans, 0) + FILL(failedplans, 0))"},
		{"3. Stock coverage", []string{StockSearches, StockSearchesEmpty}, []string{StockSearches}, "100 * (FILL(searches, 0) - FILL(empty, 0)) / (searches)"},
		{"4. Render failure rate", []string{VideosFailed}, []string{VideosFailed, VideosCompleted}, "100 * FILL(failed, 0) / (FILL(failed, 0) + FILL(done, 0))"},
		{"5. Replacement rate", []string{PicturesReplaced, PicturesRemoved}, []string{PicturesChosen}, "100 * (FILL(swapped, 0) + FILL(removed, 0)) / (chosen)"},
	} {
		var w map[string]any
		for title, cand := range byTitle {
			if strings.HasPrefix(title, tc.titleStart) {
				w = cand
			}
		}
		if w == nil {
			t.Errorf("no widget titled %q", tc.titleStart)
			continue
		}
		if got := formula(w); got != tc.formula {
			t.Errorf("%s:\n got  %s\n want %s", tc.titleStart, got, tc.formula)
		}
		// The ids in the formula must stand for the metrics the definition names.
		names := idsToNames(w)
		got := map[string]bool{}
		for _, id := range regexp.MustCompile(`\b[a-z][a-z0-9_]*\b`).FindAllString(tc.formula, -1) {
			if n, ok := names[id]; ok {
				got[n] = true
			}
		}
		for _, n := range append(append([]string{}, tc.numerator...), tc.denominator...) {
			if !got[n] {
				t.Errorf("%s: the formula does not use %s, which its definition does", tc.titleStart, n)
			}
		}
		// A rate is a percentage and must be drawn on a 0 to 100 axis.
		axis := props(w)["yAxis"].(map[string]any)["left"].(map[string]any)
		if axis["max"] != float64(100) || axis["min"] != float64(0) {
			t.Errorf("%s: axis = %v, want 0 to 100", tc.titleStart, axis)
		}
	}
}

// The generation time is read as an average and as percentiles, never summed:
// summing durations would be a number of seconds nobody asked for.
func TestGenerationTimeIsNeverSummed(t *testing.T) {
	_, widgets := dashboard(t)
	for _, w := range widgets {
		if w["type"] != "metric" {
			continue
		}
		for _, row := range metricsOf(w) {
			if _, isExpr := row[0].(map[string]any); isExpr || row[1] != VideoSeconds {
				continue
			}
			opts := row[len(row)-1].(map[string]any)
			stat, _ := opts["stat"].(string)
			if stat == "" || stat == "Sum" {
				t.Errorf("%q reads %s with the statistic %q", props(w)["title"], VideoSeconds, stat)
			}
		}
	}
}

// A failure is published with no dimension (the rates read that) and by Code
// (the breakdown reads that). The two widgets must ask for the right ones.
func TestTheFailureRateReadsTheTotalAndTheBreakdownReadsTheCodes(t *testing.T) {
	_, widgets := dashboard(t)
	var sawSearch bool
	for _, w := range widgets {
		for _, row := range metricsOf0(w) {
			if m, ok := row[0].(map[string]any); ok {
				if e, _ := m["expression"].(string); strings.HasPrefix(e, "SEARCH(") {
					sawSearch = true
					// Each breakdown reads one coded metric: failures, or refused starts (#301).
					metric := VideosFailed
					if strings.Contains(e, VideoStartsRefused) {
						metric = VideoStartsRefused
					}
					for _, want := range []string{Namespace, CodeDimension, `MetricName="` + metric + `"`} {
						if !strings.Contains(e, want) {
							t.Errorf("the breakdown %q lacks %q", e, want)
						}
					}
				}
				continue
			}
			// Any row that reads VideosFailed by name with dimensions would read a breakdown, not the total.
			if row[1] == VideosFailed && len(row) > 2 {
				if _, isOpts := row[2].(map[string]any); !isOpts {
					t.Errorf("%q reads %s with a dimension, but the rates need the total", props(w)["title"], VideosFailed)
				}
			}
		}
	}
	if !sawSearch {
		t.Error("no failures-by-reason breakdown")
	}
}

func metricsOf0(w map[string]any) [][]any {
	if w["type"] != "metric" {
		return nil
	}
	return metricsOf(w)
}

func TestTheRegionIsOnEveryMetricWidgetAndNowhereElse(t *testing.T) {
	b, err := Dashboard("eu-west-1")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Widgets []map[string]any }
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for _, w := range doc.Widgets {
		p := props(w)
		if w["type"] == "metric" && p["region"] != "eu-west-1" {
			t.Errorf("widget %q has the region %q", p["title"], p["region"])
		}
	}
	if strings.Contains(string(b), "ap-south-1") {
		t.Error("the region leaked from somewhere else")
	}
}

func TestWidgetsFitTheDashboardGridAndDoNotOverlap(t *testing.T) {
	_, widgets := dashboard(t)
	type box struct{ x, y, w, h int }
	var boxes []box
	for _, w := range widgets {
		b := box{int(w["x"].(float64)), int(w["y"].(float64)), int(w["width"].(float64)), int(w["height"].(float64))}
		if b.x < 0 || b.y < 0 || b.w < 1 || b.h < 1 || b.x+b.w > 24 {
			t.Errorf("widget %v is outside the 24 column grid", b)
		}
		for _, o := range boxes {
			if b.x < o.x+o.w && o.x < b.x+b.w && b.y < o.y+o.h && o.y < b.y+b.h {
				t.Errorf("widgets %v and %v overlap", b, o)
			}
		}
		boxes = append(boxes, b)
	}
}

func TestTheDefinitionsOnTheDashboardNameAllFiveMetrics(t *testing.T) {
	for _, want := range []string{"Story to video", "Generation time", "Stock coverage", "Render failure rate", "Replacement rate", "cancel", "not an audit"} {
		if !strings.Contains(definitions, want) {
			t.Errorf("the definitions do not mention %q", want)
		}
	}
	// And it says plainly what is not measured the way the issue words it.
	for _, honest := range []string{"does not pick a picture", "Nothing is suggested"} {
		if !strings.Contains(definitions, honest) {
			t.Errorf("the definitions do not say %q", honest)
		}
	}
}
