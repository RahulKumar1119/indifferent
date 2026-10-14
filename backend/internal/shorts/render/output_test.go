package render

// Output-level verification for the AI Shorts renderer: real FFmpeg renders
// of generated fixtures, asserted with ffprobe and frame sampling.
//
// Covered acceptance: 1080x1920 portrait output, frame rate, audio presence,
// landscape/portrait/square/low-res/very-short/silent sources, empty and
// out-of-window transcripts, and caption sync/legibility/safe-margins proven
// at the pixel level (no OCR required).
//
// Tests needing the ffmpeg/ffprobe binaries skip when absent so pure unit
// runs stay green everywhere.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed, skipping output-level test", bin)
		}
	}
}

func runCmd(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

// makeSource generates a tiny deterministic fixture: test-pattern video with
// a sine audio track (audio=false drops the audio stream entirely).
func makeSource(t *testing.T, w, h int, dur float64, audio bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), fmt.Sprintf("src-%dx%d.mp4", w, h))
	// Explicit -map: codec options placed before a second -i are parsed as
	// *decoder* selections (ffmpeg quirk), so all output options go last.
	args := []string{
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=30:duration=%.1f", w, h, dur),
	}
	if audio {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%.1f", dur))
	}
	args = append(args, "-map", "0:v:0")
	if audio {
		args = append(args, "-map", "1:a:0")
	}
	args = append(args, "-pix_fmt", "yuv420p", "-c:v", "libx264", "-preset", "ultrafast")
	if audio {
		args = append(args, "-c:a", "aac", "-shortest")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-y", path)
	runCmd(t, "ffmpeg", args...)
	return path
}

func makeTranscript(t *testing.T, words []models.Word) []byte {
	t.Helper()
	data, err := json.Marshal(models.Transcript{Text: "fixture", Words: words})
	if err != nil {
		t.Fatalf("marshal transcript: %v", err)
	}
	return data
}

func word(text string, start, end float64) models.Word {
	return models.Word{Text: text, Start: start, End: end}
}

// renderFixture runs a full Render with the default (real) FFmpeg commands
// against mock storage and returns the local output path.
func renderFixture(t *testing.T, srcPath string, transcript []byte, start, end float64, audioOnly bool, det *stubDetector) string {
	t.Helper()
	workDir := t.TempDir()
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	store := &mockStorage{objects: map[string][]byte{
		"src":        srcBytes,
		"transcript": transcript,
	}}
	r := NewRenderer(store, "assets", workDir)
	if det != nil {
		r.Detector = det
	}
	in := RenderInput{
		JobID:         "j1",
		UserID:        "u1",
		SourceKey:     "src",
		TranscriptKey: "transcript",
		Clip:          models.Clip{ClipID: "clip1", Rank: 1},
		ClipStart:     start,
		ClipEnd:       end,
		AudioOnly:     audioOnly,
	}
	key, err := r.Render(context.Background(), in)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty clip key")
	}
	return filepath.Join(workDir, "clip1.mp4")
}

type probeInfo struct {
	width, height int
	fps           float64
	vcodec        string
	pixFmt        string
	hasAudio      bool
	duration      float64
}

func probe(t *testing.T, path string) probeInfo {
	t.Helper()
	cmd := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,avg_frame_rate,codec_name,pix_fmt,duration",
		"-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffprobe video: %v", err)
	}
	var parsed struct {
		Streams []struct {
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			AvgFrameRate string `json:"avg_frame_rate"`
			CodecName    string `json:"codec_name"`
			PixFmt       string `json:"pix_fmt"`
			Duration     string `json:"duration"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil || len(parsed.Streams) == 0 {
		t.Fatalf("parse ffprobe video: %v", err)
	}
	s := parsed.Streams[0]
	var num, den float64 = 30, 1
	if _, err := fmt.Sscanf(s.AvgFrameRate, "%f/%f", &num, &den); err != nil || den == 0 {
		num, den = 30, 1
	}
	info := probeInfo{
		width: s.Width, height: s.Height,
		fps: num / den, vcodec: s.CodecName, pixFmt: s.PixFmt,
	}
	if d, err := strconv.ParseFloat(s.Duration, 64); err == nil {
		info.duration = d
	} else {
		// Fallback: container duration.
		cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "json", path)
		var f struct {
			Format struct {
				Duration string `json:"duration"`
			} `json:"format"`
		}
		if out, err := cmd.Output(); err == nil {
			if err := json.Unmarshal(out, &f); err == nil {
				info.duration, _ = strconv.ParseFloat(f.Format.Duration, 64)
			}
		}
	}
	acmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "a",
		"-show_entries", "stream=codec_name", "-of", "csv=p=0", path)
	aout, _ := acmd.Output()
	info.hasAudio = len(bytes.TrimSpace(aout)) > 0
	return info
}

func assertPortraitPreset(t *testing.T, info probeInfo, clipDur float64) {
	t.Helper()
	if info.width != 1080 || info.height != 1920 {
		t.Errorf("dimensions = %dx%d, want 1080x1920", info.width, info.height)
	}
	if info.fps < 29 || info.fps > 31 {
		t.Errorf("fps = %.2f, want ~30", info.fps)
	}
	if info.vcodec != "h264" {
		t.Errorf("vcodec = %q, want h264", info.vcodec)
	}
	if info.pixFmt != "yuv420p" {
		t.Errorf("pix_fmt = %q, want yuv420p", info.pixFmt)
	}
	if d := info.duration - clipDur; d < -0.5 || d > 0.5 {
		t.Errorf("duration = %.2f, want ~%.2f", info.duration, clipDur)
	}
}

func TestRenderOutput_Geometries(t *testing.T) {
	requireFFmpeg(t)
	cases := []struct {
		name      string
		w, h      int
		dur       float64
		audio     bool
		clip      [2]float64
		wantAudio bool
	}{
		{"landscape", 640, 360, 4, true, [2]float64{0, 3}, true},
		{"portrait", 360, 640, 4, true, [2]float64{0, 3}, true},
		{"square", 480, 480, 4, true, [2]float64{0, 3}, true},
		{"lowres", 160, 120, 4, true, [2]float64{0, 3}, true},
		{"veryshort", 640, 360, 1, true, [2]float64{0, 0.5}, true},
		{"silent", 640, 360, 4, false, [2]float64{0, 3}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := makeSource(t, tc.w, tc.h, tc.dur, tc.audio)
			tr := makeTranscript(t, []models.Word{
				word("hello", 0.2, 0.7),
				word("world", 1.2, 1.7),
			})
			out := renderFixture(t, src, tr, tc.clip[0], tc.clip[1], false, nil)
			info := probe(t, out)
			assertPortraitPreset(t, info, tc.clip[1]-tc.clip[0])
			if info.hasAudio != tc.wantAudio {
				t.Errorf("hasAudio = %v, want %v", info.hasAudio, tc.wantAudio)
			}
		})
	}
}

func TestRenderOutput_AudioOnly(t *testing.T) {
	requireFFmpeg(t)
	// Sine-only wav: black-canvas path with burned captions + audio.
	wav := filepath.Join(t.TempDir(), "src.wav")
	runCmd(t, "ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=4",
		"-c:a", "pcm_s16le", "-y", wav)
	tr := makeTranscript(t, []models.Word{word("hello", 0.2, 0.7)})
	out := renderFixture(t, wav, tr, 0, 3, true, nil)
	info := probe(t, out)
	assertPortraitPreset(t, info, 3)
	if !info.hasAudio {
		t.Error("audio-only render must keep an audio stream")
	}
}

func TestRenderOutput_EmptyTranscript(t *testing.T) {
	requireFFmpeg(t)
	// No transcribable words (silence/music): render must still succeed and
	// produce a clean, captioned-nothing clip rather than failing on the
	// empty subtitle file.
	src := makeSource(t, 640, 360, 3, true)
	tr := makeTranscript(t, nil)
	out := renderFixture(t, src, tr, 0, 2, false, nil)
	info := probe(t, out)
	assertPortraitPreset(t, info, 2)
}

func TestRenderOutput_OutOfWindowWords(t *testing.T) {
	requireFFmpeg(t)
	// Words entirely outside [start, end] are filtered: same clean output as
	// an empty transcript (covers missing-segment ranks).
	src := makeSource(t, 640, 360, 6, true)
	tr := makeTranscript(t, []models.Word{word("late", 5.0, 5.5)})
	out := renderFixture(t, src, tr, 0, 2, false, nil)
	info := probe(t, out)
	assertPortraitPreset(t, info, 2)
}

// frameBand extracts the bottom fraction of one frame as raw bytes.
func frameBand(t *testing.T, clipPath string, at, frac float64) []byte {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-v", "error",
		"-ss", fmt.Sprintf("%.2f", at), "-i", clipPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("crop=iw:ih*%.2f:0:ih*%.2f", frac, 1-frac),
		"-f", "rawvideo", "-pix_fmt", "rgb24", "-")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("extract frame: %v", err)
	}
	return out
}

func TestCaptions_SyncLegibilityMargins(t *testing.T) {
	requireFFmpeg(t)
	// alpha shows during [0,1], beta during [2,3] of a 4s clip window [0,4].
	// Different words on screen at different times proves sync; difference
	// from a captionless render proves legibility; unchanged 5% edge columns
	// prove safe margins.
	src := makeSource(t, 640, 360, 5, true)
	tr := makeTranscript(t, []models.Word{
		word("alpha", 0.2, 0.9),
		word("beta", 2.2, 2.9),
	})
	captioned := renderFixture(t, src, tr, 0, 4, false, nil)
	plain := renderFixture(t, src, makeTranscript(t, nil), 0, 4, false, nil)

	fAlpha := frameBand(t, captioned, 0.5, 0.3)
	fBeta := frameBand(t, captioned, 2.5, 0.3)
	fPlainA := frameBand(t, plain, 0.5, 0.3)
	fPlainB := frameBand(t, plain, 2.5, 0.3)
	if len(fAlpha) == 0 || len(fBeta) == 0 {
		t.Fatal("empty caption band frames")
	}
	if bytes.Equal(fAlpha, fBeta) {
		t.Error("caption band identical at t=0.5s and t=2.5s: captions not synchronized with words")
	}
	if bytes.Equal(fAlpha, fPlainA) || bytes.Equal(fBeta, fPlainB) {
		t.Error("caption band identical to captionless render: captions not legible")
	}

	// Safe margins: outer 5% columns must be untouched by captions.
	leftCap := frameEdge(t, captioned, 0.5, true)
	leftPlain := frameEdge(t, plain, 0.5, true)
	rightCap := frameEdge(t, captioned, 0.5, false)
	rightPlain := frameEdge(t, plain, 0.5, false)
	if !bytes.Equal(leftCap, leftPlain) || !bytes.Equal(rightCap, rightPlain) {
		t.Error("caption pixels bleed into the outer 5% columns: unsafe margins")
	}
}
func frameEdge(t *testing.T, clipPath string, at float64, left bool) []byte {
	t.Helper()
	x := "iw*0.95"
	if left {
		x = "0"
	}
	cmd := exec.Command("ffmpeg", "-v", "error",
		"-ss", fmt.Sprintf("%.2f", at), "-i", clipPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("crop=iw*0.05:ih*0.3:%s:ih*0.7", x),
		"-f", "rawvideo", "-pix_fmt", "rgb24", "-")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("extract edge: %v", err)
	}
	return out
}

func TestRenderOutput_Karaoke(t *testing.T) {
	requireFFmpeg(t)
	// A karaoke-styled job must burn ASS captions and still hit the preset.
	src := makeSource(t, 640, 360, 4, true)
	tr := makeTranscript(t, []models.Word{
		word("alpha", 0.2, 0.9),
		word("beta", 1.2, 1.9),
	})
	out := renderStyledFixture(t, src, tr, 0, 3, models.CaptionStyle{Preset: "karaoke"})
	info := probe(t, out)
	assertPortraitPreset(t, info, 3)
	if !info.hasAudio {
		t.Error("karaoke render must keep the audio stream")
	}
	// Captions visible: bottom band differs from a captionless render.
	plain := renderFixture(t, src, makeTranscript(t, nil), 0, 3, false, nil)
	if bytes.Equal(frameBand(t, out, 0.5, 0.3), frameBand(t, plain, 0.5, 0.3)) {
		t.Error("karaoke captions not visible in output")
	}
}

// renderStyledFixture is renderFixture with an explicit caption style.
func renderStyledFixture(t *testing.T, srcPath string, transcript []byte, start, end float64, style models.CaptionStyle) string {
	t.Helper()
	workDir := t.TempDir()
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	store := &mockStorage{objects: map[string][]byte{
		"src":        srcBytes,
		"transcript": transcript,
	}}
	r := NewRenderer(store, "assets", workDir)
	in := RenderInput{
		JobID:         "j1",
		UserID:        "u1",
		SourceKey:     "src",
		TranscriptKey: "transcript",
		Clip:          models.Clip{ClipID: "clip1", Rank: 1},
		ClipStart:     start,
		ClipEnd:       end,
		Caption:       style,
	}
	key, err := r.Render(context.Background(), in)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty clip key")
	}
	return filepath.Join(workDir, "clip1.mp4")
}

func TestComputeCropX_MovingSpeaker(t *testing.T) {
	// A speaker crossing 0.2 -> 0.8 resolves near the middle (median), so one
	// stray frame cannot yank the 9:16 window.
	frames := [][]FaceBox{
		{{Left: 0.15, Width: 0.1, Height: 0.2}},
		{{Left: 0.35, Width: 0.1, Height: 0.2}},
		{{Left: 0.55, Width: 0.1, Height: 0.2}},
		{{Left: 0.75, Width: 0.1, Height: 0.2}},
	}
	x, ok := ComputeCropX(frames, 640, 360)
	if !ok {
		t.Fatal("expected a tracked window")
	}
	// Median of [0.2,0.4,0.6,0.8] is 0.6 -> x = 0.6*640-101 = 283 -> 282 even.
	if x < 280 || x > 284 {
		t.Errorf("moving speaker window x=%d, want ~282", x)
	}
}

func TestComputeCropX_MultiFaceLargestAndZeroArea(t *testing.T) {
	// Largest face wins per frame; zero-area boxes are ignored, not median
	// poison.
	frames := [][]FaceBox{
		{
			{Left: 0.05, Width: 0.0, Height: 0.2},
			{Left: 0.1, Width: 0.08, Height: 0.15},
			{Left: 0.7, Width: 0.2, Height: 0.25},
		},
	}
	x, ok := ComputeCropX(frames, 640, 360)
	if !ok {
		t.Fatal("expected a tracked window")
	}
	// Largest center 0.8 -> x = 411, clamped to max 438 -> 410 (even).
	if x != 410 {
		t.Errorf("multi-face window x=%d, want 410", x)
	}
}

func TestFilterWords_Boundaries(t *testing.T) {
	words := []models.Word{
		word("edge-start", 0.0, 0.4),
		word("inside", 1.0, 1.4),
		word("edge-end", 1.6, 2.0),
		word("before", -1.0, -0.2),
		word("after", 2.5, 3.0),
	}
	got := filterWords(words, 0.0, 2.0)
	if len(got) != 3 {
		t.Fatalf("expected 3 boundary-inclusive words, got %d", len(got))
	}
	if got[0].Text != "edge-start" || got[2].Text != "edge-end" {
		t.Errorf("unexpected window contents: %+v", got)
	}
}
