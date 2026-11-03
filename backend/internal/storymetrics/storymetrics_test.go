package storymetrics

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/storyjob"
)

var fixedNow = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func newTestEmitter() (*Emitter, *bytes.Buffer) {
	var buf bytes.Buffer
	e := New(&buf)
	e.now = func() time.Time { return fixedNow }
	return e, &buf
}

// event is one decoded line.
type event struct {
	raw        string
	values     map[string]float64
	dimensions [][]string
	units      map[string]string
	strings    map[string]string
}

func lines(t *testing.T, buf *bytes.Buffer) []event {
	t.Helper()
	var out []event
	for _, l := range strings.SplitAfter(buf.String(), "\n") {
		if l == "" {
			continue
		}
		if p := emfProblems([]byte(l), fixedNow); len(p) != 0 {
			t.Fatalf("an event breaks the embedded metric format: %v\n%s", p, l)
		}
		var root map[string]any
		if err := json.Unmarshal([]byte(l), &root); err != nil {
			t.Fatal(err)
		}
		ev := event{raw: l, values: map[string]float64{}, units: map[string]string{}, strings: map[string]string{}}
		dir := root["_aws"].(map[string]any)["CloudWatchMetrics"].([]any)[0].(map[string]any)
		for _, ds := range dir["Dimensions"].([]any) {
			set := []string{} // an empty set is empty, not nil: that is what the document says
			for _, k := range ds.([]any) {
				set = append(set, k.(string))
			}
			ev.dimensions = append(ev.dimensions, set)
		}
		for _, m := range dir["Metrics"].([]any) {
			def := m.(map[string]any)
			name := def["Name"].(string)
			ev.values[name] = root[name].(float64)
			ev.units[name] = def["Unit"].(string)
		}
		for k, v := range root {
			if s, ok := v.(string); ok {
				ev.strings[k] = s
			}
		}
		out = append(out, ev)
	}
	return out
}

func only(t *testing.T, buf *bytes.Buffer) event {
	t.Helper()
	evs := lines(t, buf)
	if len(evs) != 1 {
		t.Fatalf("%d events, want 1:\n%s", len(evs), buf.String())
	}
	return evs[0]
}

func TestEveryEventIsAValidEmbeddedMetricDocumentWithTheRightMetrics(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record func(e *Emitter)
		values map[string]float64
		units  map[string]string
		dims   [][]string
	}{
		{"plan ready", func(e *Emitter) { e.PlanReady() }, map[string]float64{PlansReady: 1}, map[string]string{PlansReady: "Count"}, [][]string{{}}},
		{"plan failed", func(e *Emitter) { e.PlanFailed() }, map[string]float64{PlansFailed: 1}, map[string]string{PlansFailed: "Count"}, [][]string{{}}},
		{"video started", func(e *Emitter) { e.VideoStarted() }, map[string]float64{VideosStarted: 1}, map[string]string{VideosStarted: "Count"}, [][]string{{}}},
		{"video cancelled", func(e *Emitter) { e.VideoCancelled() }, map[string]float64{VideosCancelled: 1}, map[string]string{VideosCancelled: "Count"}, [][]string{{}}},
		{"video completed with a time", func(e *Emitter) { e.VideoCompleted(187.5, true) },
			map[string]float64{VideosCompleted: 1, VideoSeconds: 187.5}, map[string]string{VideosCompleted: "Count", VideoSeconds: "Seconds"}, [][]string{{}}},
		{"video completed without a time", func(e *Emitter) { e.VideoCompleted(0, false) },
			map[string]float64{VideosCompleted: 1}, map[string]string{VideosCompleted: "Count"}, [][]string{{}}},
		{"video failed", func(e *Emitter) { e.VideoFailed("MEDIA_UNAVAILABLE") },
			map[string]float64{VideosFailed: 1}, map[string]string{VideosFailed: "Count"}, [][]string{{}, {"Code"}}},
		{"search that found something", func(e *Emitter) { e.StockSearch(true) },
			map[string]float64{StockSearches: 1}, map[string]string{StockSearches: "Count"}, [][]string{{}}},
		{"search that found nothing", func(e *Emitter) { e.StockSearch(false) },
			map[string]float64{StockSearches: 1, StockSearchesEmpty: 1}, map[string]string{StockSearches: "Count", StockSearchesEmpty: "Count"}, [][]string{{}}},
		{"edits of every kind", func(e *Emitter) { e.PlanEdited(Edits{Chosen: 2, Replaced: 1, Removed: 3, Narration: 4}) },
			map[string]float64{PicturesChosen: 2, PicturesReplaced: 1, PicturesRemoved: 3, NarrationsEdited: 4},
			map[string]string{PicturesChosen: "Count", PicturesReplaced: "Count", PicturesRemoved: "Count", NarrationsEdited: "Count"}, [][]string{{}}},
		{"edits of one kind", func(e *Emitter) { e.PlanEdited(Edits{Replaced: 2}) },
			map[string]float64{PicturesReplaced: 2}, map[string]string{PicturesReplaced: "Count"}, [][]string{{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, buf := newTestEmitter()
			tc.record(e)
			ev := only(t, buf)
			if !reflect.DeepEqual(ev.values, tc.values) {
				t.Errorf("values = %v, want %v", ev.values, tc.values)
			}
			if !reflect.DeepEqual(ev.units, tc.units) {
				t.Errorf("units = %v, want %v", ev.units, tc.units)
			}
			if !reflect.DeepEqual(ev.dimensions, tc.dims) {
				t.Errorf("dimension sets = %v, want %v", ev.dimensions, tc.dims)
			}
		})
	}
}

func TestEveryMetricNameIsEmittedBySomeEventAndNoOtherIs(t *testing.T) {
	e, buf := newTestEmitter()
	e.PlanReady()
	e.PlanFailed()
	e.VideoStarted()
	e.VideoCancelled()
	e.VideoCompleted(1, true)
	e.VideoFailed("INTERNAL")
	e.StockSearch(false)
	e.PlanEdited(Edits{Chosen: 1, Replaced: 1, Removed: 1, Narration: 1})
	seen := map[string]bool{}
	for _, ev := range lines(t, buf) {
		for n := range ev.values {
			seen[n] = true
		}
	}
	var got []string
	for n := range seen {
		got = append(got, n)
	}
	want := Names()
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("emitted %v, but Names() lists %v: the dashboard and the README are built from Names()", got, want)
	}
}

func TestAFailureIsPublishedBothAsATotalAndByCode(t *testing.T) {
	e, buf := newTestEmitter()
	e.VideoFailed("TIMED_OUT")
	ev := only(t, buf)
	if ev.strings[CodeDimension] != "TIMED_OUT" {
		t.Errorf("Code = %q", ev.strings[CodeDimension])
	}
	if !reflect.DeepEqual(ev.dimensions, [][]string{{}, {"Code"}}) {
		t.Errorf("a failure must be one metric with no dimension (the rate needs the total) and one per code: %v", ev.dimensions)
	}
}

// The code of a failed video comes from a database record. Every distinct value
// would be a new CloudWatch metric (and a new charge), so only known codes pass.
func TestAnUnknownOrHostileFailureCodeCannotCreateMetrics(t *testing.T) {
	for _, code := range []string{"", " ", "made up", "media_unavailable", "MEDIA_UNAVAILABLE ", "é", "\x00", "A\nB", strings.Repeat("X", 5000), `"}{"`} {
		e, buf := newTestEmitter()
		e.VideoFailed(code)
		ev := only(t, buf)
		if ev.strings[CodeDimension] != OtherCode {
			t.Errorf("code %q was recorded as %q, want %q", code, ev.strings[CodeDimension], OtherCode)
		}
	}
	e, buf := newTestEmitter()
	for _, c := range FailureCodes() {
		e.VideoFailed(c)
	}
	for _, ev := range lines(t, buf) {
		if c := ev.strings[CodeDimension]; c != OtherCode && !failureCodes[c] {
			t.Errorf("a code outside the list got through: %q", c)
		}
	}
}

// storyjob owns the failure codes. A new one that is not in this package's list
// would be recorded as OTHER and hidden from the breakdown.
func TestEveryStoryJobFailureCodeIsKnownHere(t *testing.T) {
	for _, c := range []string{
		storyjob.CodeMediaUnavailable, storyjob.CodeMediaInvalid, storyjob.CodeMediaTemporary,
		storyjob.CodeNarrationFailed, storyjob.CodeNarrationTooLong, storyjob.CodeInternal,
		storyjob.CodeCancelled, storyjob.CodeStartFailed, storyjob.CodeTimedOut,
	} {
		if SafeCode(c) != c {
			t.Errorf("storyjob code %q is not in the list of known failure codes", c)
		}
	}
	// And the reverse: nothing is listed here that storyjob no longer has.
	known := map[string]bool{}
	for _, c := range []string{
		storyjob.CodeMediaUnavailable, storyjob.CodeMediaInvalid, storyjob.CodeMediaTemporary,
		storyjob.CodeNarrationFailed, storyjob.CodeNarrationTooLong, storyjob.CodeInternal,
		storyjob.CodeCancelled, storyjob.CodeStartFailed, storyjob.CodeTimedOut,
	} {
		known[c] = true
	}
	for c := range failureCodes {
		if !known[c] {
			t.Errorf("%q is listed here but storyjob has no such code", c)
		}
	}
}

func TestOnlyBelievableValuesAreRecorded(t *testing.T) {
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1} {
		e, buf := newTestEmitter()
		e.VideoCompleted(bad, true)
		ev := only(t, buf)
		if _, has := ev.values[VideoSeconds]; has {
			t.Errorf("a generation time of %v was recorded", bad)
		}
		if ev.values[VideosCompleted] != 1 {
			t.Errorf("the completed count was lost along with the bad time %v", bad)
		}
	}
	e, buf := newTestEmitter()
	e.VideoCompleted(0, true) // instant is odd but not wrong
	if _, has := only(t, buf).values[VideoSeconds]; !has {
		t.Error("a time of zero seconds was dropped")
	}
}

func TestAnEditThatChangedNothingWritesNothing(t *testing.T) {
	e, buf := newTestEmitter()
	e.PlanEdited(Edits{})
	e.PlanEdited(Edits{Chosen: -1, Replaced: -5})
	if buf.Len() != 0 {
		t.Errorf("wrote %q", buf.String())
	}
}

func TestSecondsFromTheJobRecord(t *testing.T) {
	for _, tc := range []struct {
		created, completed string
		want               float64
		ok                 bool
	}{
		{"2026-10-31T12:00:00Z", "2026-10-31T12:03:07Z", 187, true},
		{"2026-10-31T12:00:00Z", "2026-10-31T12:00:00Z", 0, true},
		{"2026-10-31T12:00:00Z", "2026-11-01T12:00:00Z", 86400, true}, // exactly the limit
		{"2026-10-31T12:00:00Z", "2026-11-01T12:00:01Z", 0, false},    // over it
		{"2026-10-31T12:00:05Z", "2026-10-31T12:00:00Z", 0, false},    // completed before created
		{"2026-10-31T12:00:00+05:30", "2026-10-31T06:31:00Z", 60, true},
		{"", "2026-10-31T12:00:00Z", 0, false},
		{"2026-10-31T12:00:00Z", "", 0, false},
		{"yesterday", "today", 0, false},
	} {
		got, ok := Seconds(tc.created, tc.completed)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("Seconds(%q, %q) = %v, %v; want %v, %v", tc.created, tc.completed, got, ok, tc.want, tc.ok)
		}
	}
}

func TestANilOrZeroEmitterRecordsNothingAndDoesNotPanic(t *testing.T) {
	for _, e := range []*Emitter{nil, {}, New(nil)} {
		e.PlanReady()
		e.PlanFailed()
		e.VideoStarted()
		e.VideoCancelled()
		e.VideoCompleted(1, true)
		e.VideoFailed("INTERNAL")
		e.StockSearch(false)
		e.PlanEdited(Edits{Chosen: 1})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, bytes.ErrTooLarge }

func TestAFailureToWriteCannotFailTheCaller(t *testing.T) {
	e := New(failingWriter{})
	e.PlanReady()
	e.VideoFailed("INTERNAL")
}

func TestConcurrentEventsNeverShareALine(t *testing.T) {
	e, buf := newTestEmitter()
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				e.VideoFailed("INTERNAL")
				e.StockSearch(i%2 == 0)
				e.PlanEdited(Edits{Chosen: 1, Narration: 1})
			}
		}()
	}
	wg.Wait()
	if n := len(lines(t, buf)); n != 20*50*3 {
		t.Errorf("%d valid events, want %d", n, 20*50*3)
	}
}

func TestTheTimestampIsInMillisecondsOfTheClock(t *testing.T) {
	e, buf := newTestEmitter()
	e.PlanReady()
	var root map[string]any
	if err := json.Unmarshal(buf.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if got := int64(root["_aws"].(map[string]any)["Timestamp"].(float64)); got != fixedNow.UnixMilli() {
		t.Errorf("Timestamp = %d, want %d", got, fixedNow.UnixMilli())
	}
}
