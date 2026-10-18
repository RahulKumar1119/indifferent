package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// captureTiming runs fn with the standard logger redirected and returns the
// decoded render_timing record, failing if there is not exactly one.
func captureTiming(t *testing.T, fn func()) timingRecord {
	t.Helper()
	var buf bytes.Buffer
	prev := timingLogger
	timingLogger = log.New(&buf, "", 0)
	t.Cleanup(func() { timingLogger = prev })
	fn()

	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("want exactly one timing line, got %d in:\n%s", len(lines), buf.String())
	}
	var rec timingRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("the timing line must be a bare JSON object: %v\n%s", err, lines[0])
	}
	if rec.Event != "render_timing" {
		t.Fatalf("event = %q, want render_timing", rec.Event)
	}
	return rec
}

// timingFixture builds a Renderer whose ffmpeg is faked (writes a small file),
// so the test checks the bookkeeping, not the encode.
func timingFixture(t *testing.T, det FaceDetector) (*Renderer, RenderInput, *mockStorage) {
	t.Helper()
	tr, _ := json.Marshal(models.Transcript{
		Text:  "hello world",
		Words: []models.Word{{Text: "hello", Start: 10.0, End: 10.5}},
	})
	store := &mockStorage{objects: map[string][]byte{
		"uploads/u1/j1/source.mp4":     []byte("SOURCE-BYTES"), // 12 bytes
		"shorts/u1/j1/transcript.json": tr,
	}}
	fake := func(name string, args []string) error {
		return os.WriteFile(args[len(args)-1], []byte("RENDERED-CLIP"), 0o644) // 13 bytes
	}
	r := &Renderer{
		Storage:         store,
		Bucket:          "assets",
		WorkDir:         t.TempDir(),
		Detector:        det,
		ProbeDimensions: func(string) (int, int, error) { return 1920, 1080, nil },
		ExtractFrames:   func(string, float64, float64) ([][]byte, error) { return [][]byte{[]byte("f")}, nil },
		RunCommand:      fake,
		RunCommandProgress: func(name string, args []string, _ float64, _ func(int)) error {
			return fake(name, args)
		},
	}
	in := RenderInput{
		JobID: "j1", UserID: "u1",
		SourceKey: "uploads/u1/j1/source.mp4", TranscriptKey: "shorts/u1/j1/transcript.json",
		Clip:      models.Clip{ClipID: "c1"},
		ClipStart: 10.0, ClipEnd: 28.0, // 18 s
	}
	return r, in, store
}

func TestRenderTiming_ReportsPhasesAndFacts(t *testing.T) {
	r, in, _ := timingFixture(t, &stubDetector{boxes: [][]FaceBox{{}}}) // faceless
	rec := captureTiming(t, func() {
		if _, err := r.Render(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	})

	if rec.JobID != "j1" || rec.ClipID != "c1" || rec.Outcome != "ok" {
		t.Errorf("identity/outcome wrong: %+v", rec)
	}
	for _, phase := range []string{phaseCheckExists, phaseDownloadSource, phaseDownloadTranscript, phaseCaptions, phaseReframe, phaseEncode, phaseBranding, phaseUpload} {
		ms, ok := rec.PhasesMs[phase]
		if !ok {
			t.Errorf("phase %q missing from %v", phase, rec.PhasesMs)
		} else if ms < 0 {
			t.Errorf("phase %q has negative duration %d", phase, ms)
		}
	}
	if rec.ClipSeconds != 18 {
		t.Errorf("clipSeconds = %v, want 18", rec.ClipSeconds)
	}
	if rec.SourceBytes != len("SOURCE-BYTES") || rec.OutputBytes != len("RENDERED-CLIP") {
		t.Errorf("byte counts wrong: source %d output %d", rec.SourceBytes, rec.OutputBytes)
	}
	if rec.EncodeSpeedX <= 0 {
		t.Errorf("encodeSpeedX = %v, want a positive realtime factor", rec.EncodeSpeedX)
	}
	if rec.FailedPhase != "" {
		t.Errorf("a successful render must not report a failed phase: %q", rec.FailedPhase)
	}
}

func TestRenderTiming_ReportsTheReframeModeOnEveryClip(t *testing.T) {
	face := &stubDetector{boxes: [][]FaceBox{{{Left: 0.3, Top: 0.2, Width: 0.2, Height: 0.3}}}}
	cases := []struct {
		name string
		det  FaceDetector
		mut  func(*Renderer, *RenderInput)
		want string
	}{
		{"no faces -> blur fill", &stubDetector{boxes: [][]FaceBox{{}}}, nil, modeBlurFill},
		{"face found -> subject crop (this path logs nothing else)", face, nil, modeSubjectCrop},
		{"no detector -> centered crop", nil, nil, modeCenterCrop},
		{"detector error -> centered crop", &stubDetector{err: errors.New("throttled")}, nil, modeCenterCrop},
		{"audio only", nil, func(_ *Renderer, in *RenderInput) { in.AudioOnly = true }, modeAudioOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, in, _ := timingFixture(t, tc.det)
			if tc.det == nil {
				r.Detector = nil // avoid a typed-nil interface
			}
			if tc.mut != nil {
				tc.mut(r, &in)
			}
			rec := captureTiming(t, func() {
				if _, err := r.Render(context.Background(), in); err != nil {
					t.Fatal(err)
				}
			})
			if rec.Mode != tc.want {
				t.Errorf("reframeMode = %q, want %q", rec.Mode, tc.want)
			}
		})
	}
}

func TestRenderTiming_SkippedWhenClipAlreadyRendered(t *testing.T) {
	r, in, store := timingFixture(t, nil)
	r.Detector = nil
	store.exists = true
	rec := captureTiming(t, func() {
		if _, err := r.Render(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	})
	if rec.Outcome != "skipped" {
		t.Errorf("outcome = %q, want skipped", rec.Outcome)
	}
	if _, ran := rec.PhasesMs[phaseEncode]; ran {
		t.Error("a skipped render must not report an encode phase")
	}
}

func TestRenderTiming_NamesThePhaseThatFailed(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Renderer, *mockStorage)
		want  string
	}{
		{"source download fails", func(_ *Renderer, s *mockStorage) { s.getErr = errors.New("S3 down") }, phaseDownloadSource},
		{"encode fails", func(r *Renderer, _ *mockStorage) {
			r.RunCommandProgress = func(string, []string, float64, func(int)) error { return errors.New("ffmpeg exit 1") }
		}, phaseEncode},
		{"upload fails", func(_ *Renderer, s *mockStorage) { s.putErr = errors.New("access denied") }, phaseUpload},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, in, store := timingFixture(t, nil)
			r.Detector = nil
			tc.setup(r, store)
			rec := captureTiming(t, func() {
				if _, err := r.Render(context.Background(), in); err == nil {
					t.Fatal("want an error")
				}
			})
			if rec.Outcome != "error" || rec.FailedPhase != tc.want {
				t.Errorf("outcome=%q failedPhase=%q, want error/%q", rec.Outcome, rec.FailedPhase, tc.want)
			}
		})
	}
}

func TestRenderTiming_LineIsOneLineOfJSONWithStableKeys(t *testing.T) {
	r, in, _ := timingFixture(t, &stubDetector{boxes: [][]FaceBox{{}}})
	var buf bytes.Buffer
	prev := timingLogger
	timingLogger = log.New(&buf, "", 0)
	defer func() { timingLogger = prev }()
	if _, err := r.Render(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(buf.String())
	if strings.Contains(line, "\n") || !strings.HasPrefix(line, "{") {
		t.Fatalf("want a single bare JSON line, got %q", line)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		t.Fatal(err)
	}
	// Logs Insights queries rely on these names.
	for _, key := range []string{"event", "jobId", "clipId", "outcome", "reframeMode", "clipSeconds", "encodeSpeedX", "totalMs", "phasesMs"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in %s", key, line)
		}
	}
	for _, leaked := range []string{"userId", "uploads/", "shorts/u1"} {
		if strings.Contains(line, leaked) {
			t.Errorf("timing line should not carry %q: %s", leaked, line)
		}
	}
}

func TestReframeMode(t *testing.T) {
	tests := []struct {
		audioOnly bool
		cropX     string
		fit       bool
		want      string
	}{
		{true, "", false, modeAudioOnly},
		{true, "464", true, modeAudioOnly}, // audio-only wins
		{false, "", true, modeBlurFill},
		{false, "464", false, modeSubjectCrop},
		{false, "0", false, modeSubjectCrop}, // portrait source pinned left
		{false, "", false, modeCenterCrop},
	}
	for _, tt := range tests {
		if got := reframeMode(tt.audioOnly, tt.cropX, tt.fit); got != tt.want {
			t.Errorf("reframeMode(%v,%q,%v) = %q, want %q", tt.audioOnly, tt.cropX, tt.fit, got, tt.want)
		}
	}
}
