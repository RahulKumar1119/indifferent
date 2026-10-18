package render

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// timingLogger writes the render_timing line with no timestamp prefix, so each
// line is a bare JSON object. CloudWatch Logs Insights then reads its fields
// directly (the log group already stamps each event). Swapped in tests.
var timingLogger = log.New(os.Stderr, "", 0)

// Reframe modes reported in the render_timing log line.
const (
	modeAudioOnly   = "audio_only"   // no video stream: solid canvas
	modeBlurFill    = "blur_fill"    // no subject found: whole frame over a blurred copy
	modeSubjectCrop = "subject_crop" // 9:16 window placed by the crop offset (tracked subject, or pinned for vertical sources)
	modeCenterCrop  = "center_crop"  // legacy centered crop (reframing off, or it fell back)
)

// reframeMode names how a clip was framed. cropX and fit are smartReframe's
// results; audioOnly takes precedence because no reframing runs for it.
func reframeMode(audioOnly bool, cropX string, fit bool) string {
	switch {
	case audioOnly:
		return modeAudioOnly
	case fit:
		return modeBlurFill
	case cropX != "":
		return modeSubjectCrop
	default:
		return modeCenterCrop
	}
}

// Render phases, in the order they run. Each shows up in the render_timing
// line as "<name>Ms".
const (
	phaseCheckExists        = "checkExists"
	phaseDownloadSource     = "downloadSource"
	phaseDownloadTranscript = "downloadTranscript"
	phaseCaptions           = "captions"
	phaseReframe            = "reframe"
	phaseEncode             = "encode"
	phaseBranding           = "branding"
	phaseUpload             = "upload"
)

// renderTimer measures how long each phase of one clip render takes and
// writes a single structured log line at the end. Example Logs Insights query
// over the render task's log group (Logs Insights discovers the JSON fields):
//
//	fields @timestamp, jobId, clipId, reframeMode, phasesMs.encode as encodeMs, encodeSpeedX, totalMs
//	| filter event = "render_timing" and outcome = "ok"
//	| stats count(*) as clips, pct(totalMs, 50) as p50TotalMs, pct(encodeMs, 50) as p50EncodeMs,
//	        pct(encodeSpeedX, 50) as p50Speed by reframeMode
//
// The task's own time is only part of a clip's wall time: Fargate startup
// (about 17 s) happens before the process starts and is not in this line. It exists so changes to
// the render task (CPU size, filters) can be judged from data: the task's
// total time was known, but not where inside it the time goes.
//
// A renderTimer is used by one goroutine (one render).
type renderTimer struct {
	jobID, clipID string
	started       time.Time
	phases        map[string]time.Duration
	order         []string
	current       string // phase in progress; non-empty at emit means it failed there
	mode          string
	clipSeconds   float64
	sourceBytes   int
	outputBytes   int
	skipped       bool
}

func newRenderTimer(in RenderInput) *renderTimer {
	return &renderTimer{
		jobID:   in.JobID,
		clipID:  in.Clip.ClipID,
		started: time.Now(),
		phases:  map[string]time.Duration{},
	}
}

// start begins a phase and returns the function that ends it.
func (t *renderTimer) start(name string) func() {
	began := time.Now()
	t.current = name
	return func() {
		if _, seen := t.phases[name]; !seen {
			t.order = append(t.order, name)
		}
		t.phases[name] += time.Since(began)
		t.current = ""
	}
}

// timingRecord is the JSON written after "render_timing ". Field names are
// stable: CloudWatch Logs Insights queries depend on them.
type timingRecord struct {
	Event        string           `json:"event"` // always "render_timing"
	JobID        string           `json:"jobId"`
	ClipID       string           `json:"clipId"`
	Outcome      string           `json:"outcome"` // ok | skipped | error
	FailedPhase  string           `json:"failedPhase,omitempty"`
	Mode         string           `json:"reframeMode,omitempty"`
	ClipSeconds  float64          `json:"clipSeconds,omitempty"`
	SourceBytes  int              `json:"sourceBytes,omitempty"`
	OutputBytes  int              `json:"outputBytes,omitempty"`
	EncodeSpeedX float64          `json:"encodeSpeedX,omitempty"` // clip seconds / encode wall seconds, the same figure as ffmpeg's speed=
	TotalMs      int64            `json:"totalMs"`
	PhasesMs     map[string]int64 `json:"phasesMs"`
}

func (t *renderTimer) record(err error) timingRecord {
	rec := timingRecord{
		Event:       "render_timing",
		JobID:       t.jobID,
		ClipID:      t.clipID,
		Outcome:     "ok",
		Mode:        t.mode,
		ClipSeconds: t.clipSeconds,
		SourceBytes: t.sourceBytes,
		OutputBytes: t.outputBytes,
		TotalMs:     time.Since(t.started).Milliseconds(),
		PhasesMs:    map[string]int64{},
	}
	switch {
	case err != nil:
		rec.Outcome = "error"
		rec.FailedPhase = t.current
	case t.skipped:
		rec.Outcome = "skipped"
	}
	for _, name := range t.order {
		rec.PhasesMs[name] = t.phases[name].Milliseconds()
	}
	if enc, ok := t.phases[phaseEncode]; ok && enc > 0 && t.clipSeconds > 0 {
		rec.EncodeSpeedX = round2(t.clipSeconds / enc.Seconds())
	}
	return rec
}

// emit writes the one-line summary. Logging must never fail a render.
func (t *renderTimer) emit(err error) {
	b, mErr := json.Marshal(t.record(err))
	if mErr != nil {
		return
	}
	timingLogger.Print(string(b))
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
