// Package storymetrics records the success metrics of Story2Shorts (issue #194)
// as CloudWatch custom metrics, without a CloudWatch client.
//
// Each event is written as one line of JSON in CloudWatch's embedded metric
// format (EMF) to the process's output. CloudWatch Logs reads those lines and
// publishes the metrics they describe: automatically for Lambda, and for the
// awslogs driver on Fargate. So recording a metric needs no permission, no
// agent and no network call, and a failure to record one cannot fail a request.
//
// The five metrics, and what each one measures in the product as it is:
//
//  1. Story-to-video completion rate: VideosCompleted ÷ (PlansReady + PlansFailed).
//     Of the stories people submit, how many end as a finished video.
//  2. Average generation time: VideoSeconds, from "Make the video" to the
//     finished file, so it includes waiting for a task to start.
//  3. Stock media coverage: 1 − StockSearchesEmpty ÷ StockSearches. The product
//     does not pick a picture for a scene; the picker searches the scene's
//     keywords and the user chooses. So this counts searches that found
//     something, not scenes. A refinement the user types counts too.
//  4. Render failure rate: VideosFailed ÷ (VideosCompleted + VideosFailed). A
//     video the user cancels is not a failure and is counted apart.
//  5. Replacement rate: (PicturesReplaced + PicturesRemoved) ÷ PicturesChosen.
//     Again no picture is suggested, so this is how often a chosen picture is
//     later swapped or taken out. NarrationsEdited counts text edits.
//
// Nothing here carries a user id, a plan id or any text of a story: only counts,
// a duration, and a failure code from a fixed list. A metric's dimensions create
// one CloudWatch metric each, so the only dimension is that code.
//
// Delivery is at least once: CloudWatch says a metric value may, rarely, be
// recorded twice. The rates are a guide, not an audit.
package storymetrics

import (
	"encoding/json"
	"io"
	"math"
	"sync"
	"time"
)

// Namespace is the CloudWatch namespace of every metric here.
const Namespace = "Indifferent/Story"

// Metric names. They are the names in CloudWatch, in the dashboard and in the
// README; change one and the history under the old name is orphaned.
const (
	PlansReady         = "PlansReady"
	PlansFailed        = "PlansFailed"
	VideosStarted      = "VideosStarted"
	VideosCompleted    = "VideosCompleted"
	VideosFailed       = "VideosFailed"
	VideosCancelled    = "VideosCancelled"
	VideoSeconds       = "VideoSeconds"
	StockSearches      = "StockSearches"
	StockSearchesEmpty = "StockSearchesEmpty"
	PicturesChosen     = "PicturesChosen"
	PicturesReplaced   = "PicturesReplaced"
	PicturesRemoved    = "PicturesRemoved"
	NarrationsEdited   = "NarrationsEdited"
	// VideoStartsRefused counts "Make the video" requests that made no job
	// (#301), by Code: what the user had to fix, or a failure of ours before
	// the job was written. A start that made a job is VideosStarted instead.
	VideoStartsRefused = "VideoStartsRefused"
)

// CodeDimension is the one dimension, on VideosFailed.
const CodeDimension = "Code"

// Names lists every metric, for the dashboard and the tests.
func Names() []string {
	return []string{
		PlansReady, PlansFailed, VideosStarted, VideosCompleted, VideosFailed, VideosCancelled, VideoSeconds,
		StockSearches, StockSearchesEmpty, PicturesChosen, PicturesReplaced, PicturesRemoved, NarrationsEdited,
		VideoStartsRefused,
	}
}

// failureCodes are the codes a failed video can carry (backend/internal/storyjob
// errors.go; a test fails if one is missing here). A code from outside this list
// is recorded as OtherCode, which bounds the number of metrics: the value comes
// from a database record, and every distinct value would otherwise be a new
// CloudWatch metric.
var failureCodes = map[string]bool{
	"MEDIA_UNAVAILABLE":  true,
	"MEDIA_INVALID":      true,
	"MEDIA_TEMPORARY":    true,
	"NARRATION_FAILED":   true,
	"NARRATION_TOO_LONG": true,
	"INTERNAL":           true,
	"CANCELLED":          true,
	"START_FAILED":       true,
	"TIMED_OUT":          true,
}

// OtherCode stands for any failure code not in the list.
const OtherCode = "OTHER"

// FailureCodes returns the codes a VideosFailed metric can carry, OtherCode
// included.
func FailureCodes() []string {
	out := make([]string, 0, len(failureCodes)+1)
	for c := range failureCodes {
		out = append(out, c)
	}
	return append(out, OtherCode)
}

// SafeCode maps a failure code to one of the known values.
func SafeCode(code string) string {
	if failureCodes[code] {
		return code
	}
	return OtherCode
}

// startRefusalCodes are the API error codes a refused video start can carry
// (handleCreateStoryVideo in backend/internal/api). As with failureCodes, any
// other value is recorded as OtherCode, so the number of metrics stays bounded.
var startRefusalCodes = map[string]bool{
	"MISSING_PICTURES":     true,
	"UNSUPPORTED_LANGUAGE": true,
	"STALE_REVISION":       true,
	"NOT_READY":            true,
	"NOT_FOUND":            true,
	"ALREADY_RUNNING":      true,
	"TOO_MANY_ACTIVE":      true,
	"RATE_LIMITED":         true,
	"VALIDATION_ERROR":     true,
	"INVALID_BODY":         true,
	"UNAVAILABLE":          true,
	"DB_ERROR":             true,
	"INTERNAL_ERROR":       true,
}

// StartRefusalCodes returns the codes a VideoStartsRefused metric can carry,
// OtherCode included.
func StartRefusalCodes() []string {
	out := make([]string, 0, len(startRefusalCodes)+1)
	for c := range startRefusalCodes {
		out = append(out, c)
	}
	return append(out, OtherCode)
}

// SafeStartCode maps a start refusal code to one of the known values.
func SafeStartCode(code string) string {
	if startRefusalCodes[code] {
		return code
	}
	return OtherCode
}

// maxVideoSeconds bounds a recorded generation time. A longer one is a clock or
// record error, not a render, and would wreck the average.
const maxVideoSeconds = 24 * 60 * 60

// Seconds returns the time from createdAt to completedAt, both RFC 3339, and
// whether it is a believable generation time.
func Seconds(createdAt, completedAt string) (float64, bool) {
	c, err1 := time.Parse(time.RFC3339, createdAt)
	d, err2 := time.Parse(time.RFC3339, completedAt)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	s := d.Sub(c).Seconds()
	if s < 0 || s > maxVideoSeconds {
		return 0, false
	}
	return s, true
}

// Emitter writes metrics. The zero value and a nil *Emitter record nothing, so
// code that has none (a test, a handler built without one) needs no checks.
type Emitter struct {
	mu  sync.Mutex
	w   io.Writer
	now func() time.Time
}

// New returns an Emitter that writes one line per event to w. In a service, w
// is os.Stdout: not the log package, whose timestamp prefix would make the line
// invalid.
func New(w io.Writer) *Emitter { return &Emitter{w: w, now: time.Now} }

type unit string

const (
	count   unit = "Count"
	seconds unit = "Seconds"
)

type metric struct {
	name  string
	value float64
	unit  unit
}

// emit writes one EMF document. dimSets lists the dimension sets to publish each
// metric under (an empty set publishes it with no dimension); dims gives the
// value of every dimension key a set names.
func (e *Emitter) emit(dimSets [][]string, dims map[string]string, ms ...metric) {
	if e == nil || e.w == nil || len(ms) == 0 {
		return
	}
	defs := make([]map[string]string, 0, len(ms))
	root := make(map[string]any, len(ms)+len(dims)+1)
	for _, m := range ms {
		if math.IsNaN(m.value) || math.IsInf(m.value, 0) || m.value < 0 {
			continue
		}
		defs = append(defs, map[string]string{"Name": m.name, "Unit": string(m.unit)})
		root[m.name] = m.value
	}
	if len(defs) == 0 {
		return
	}
	for k, v := range dims {
		root[k] = v
	}
	root["_aws"] = map[string]any{
		"Timestamp": e.now().UnixMilli(),
		"CloudWatchMetrics": []any{map[string]any{
			"Namespace":  Namespace,
			"Dimensions": dimSets,
			"Metrics":    defs,
		}},
	}
	line, err := json.Marshal(root)
	if err != nil {
		return
	}
	line = append(line, '\n')
	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.w.Write(line)
}

var noDimensions = [][]string{{}}

// PlanReady records a story turned into a plan.
func (e *Emitter) PlanReady() { e.emit(noDimensions, nil, metric{PlansReady, 1, count}) }

// PlanFailed records a story the planner could not turn into a plan. A story
// refused before the planner ran (too short, over the hourly allowance) is not
// recorded: it never became an attempt.
func (e *Emitter) PlanFailed() { e.emit(noDimensions, nil, metric{PlansFailed, 1, count}) }

// VideoStarted records a video job that was created.
func (e *Emitter) VideoStarted() { e.emit(noDimensions, nil, metric{VideosStarted, 1, count}) }

// VideoCancelled records a video the user cancelled.
func (e *Emitter) VideoCancelled() { e.emit(noDimensions, nil, metric{VideosCancelled, 1, count}) }

// VideoCompleted records a finished video and, when it is believable, how long
// it took.
func (e *Emitter) VideoCompleted(secs float64, known bool) {
	ms := []metric{{VideosCompleted, 1, count}}
	if known {
		ms = append(ms, metric{VideoSeconds, secs, seconds})
	}
	e.emit(noDimensions, nil, ms...)
}

// VideoFailed records a video that failed, with its code. It is published both
// with no dimension, for the total, and with the code, for the breakdown.
func (e *Emitter) VideoFailed(code string) {
	e.emit([][]string{{}, {CodeDimension}}, map[string]string{CodeDimension: SafeCode(code)}, metric{VideosFailed, 1, count})
}

// VideoStartRefused records a "Make the video" request that made no job, with
// its code. Like a failure, it is published as a total and by code.
func (e *Emitter) VideoStartRefused(code string) {
	e.emit([][]string{{}, {CodeDimension}}, map[string]string{CodeDimension: SafeStartCode(code)}, metric{VideoStartsRefused, 1, count})
}

// StockSearch records a search of the stock libraries that every provider
// answered, and whether it found anything.
func (e *Emitter) StockSearch(found bool) {
	ms := []metric{{StockSearches, 1, count}}
	if !found {
		ms = append(ms, metric{StockSearchesEmpty, 1, count})
	}
	e.emit(noDimensions, nil, ms...)
}

// Edits counts what one saved plan changed.
type Edits struct {
	Chosen    int // scenes that had no picture and now have one
	Replaced  int // scenes whose picture was swapped for another
	Removed   int // scenes whose picture was taken out
	Narration int // scenes whose narration text changed
}

// PlanEdited records the changes one save made. A save that changed nothing
// records nothing.
func (e *Emitter) PlanEdited(c Edits) {
	var ms []metric
	add := func(name string, n int) {
		if n > 0 {
			ms = append(ms, metric{name, float64(n), count})
		}
	}
	add(PicturesChosen, c.Chosen)
	add(PicturesReplaced, c.Replaced)
	add(PicturesRemoved, c.Removed)
	add(NarrationsEdited, c.Narration)
	e.emit(noDimensions, nil, ms...)
}
