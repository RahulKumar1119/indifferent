package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// Test videos are expensive to make, so each kind is built once and shared.
var (
	videoMu    sync.Mutex
	videoCache = map[string]string{}
	videoDir   string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "thumbnail-test-*")
	if err != nil {
		panic(err)
	}
	videoDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// makeVideo returns a clip made of visibly different scenes, optionally
// ending with a few seconds of black. Needs ffmpeg.
func makeVideo(t *testing.T, size string, withBlack bool) string {
	t.Helper()
	needTool(t, "ffmpeg", "ffprobe")
	key := fmt.Sprintf("%s/%v", size, withBlack)
	videoMu.Lock()
	defer videoMu.Unlock()
	if p, ok := videoCache[key]; ok {
		return p
	}
	path := filepath.Join(videoDir, fmt.Sprintf("src-%d.mp4", len(videoCache)))
	sources := []string{"testsrc2", "mandelbrot", "rgbtestsrc", "smptebars"}
	args := []string{"-v", "error"}
	for _, s := range sources {
		args = append(args, "-f", "lavfi", "-t", "8", "-i", s+"=s="+size+":r=25")
	}
	n := len(sources)
	if withBlack {
		args = append(args, "-f", "lavfi", "-t", "4", "-i", "color=c=black:s="+size+":r=25")
		n++
	}
	var in strings.Builder
	for i := 0; i < n; i++ {
		in.WriteString("[" + string(rune('0'+i)) + ":v]")
	}
	args = append(args, "-filter_complex", in.String()+"concat=n="+string(rune('0'+n))+":v=1:a=0,format=yuv420p", "-y", path)
	if out, err := (ExecRunner{}).Run(context.Background(), "ffmpeg", args...); err != nil {
		t.Fatalf("make video: %v %s", err, out)
	}
	videoCache[key] = path
	return path
}

func blackVideo(t *testing.T) string {
	t.Helper()
	needTool(t, "ffmpeg", "ffprobe")
	path := filepath.Join(t.TempDir(), "black.mp4")
	if out, err := (ExecRunner{}).Run(context.Background(), "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=640x360:r=25:d=20",
		"-pix_fmt", "yuv420p", "-y", path); err != nil {
		t.Fatalf("make video: %v %s", err, out)
	}
	return path
}

func goodAnalysis() Analysis {
	return Analysis{
		Topic: "test pattern", Headline: "TEST PATTERNS", SecondaryText: "EXPLAINED",
		SuggestedStyle: StyleTechnology, BestFrameTime: -1,
	}
}

func faceEverywhere() fakeDetector {
	return fakeDetector{fn: func([]byte) (Detection, error) { return faceAt(0.4, 0.4, 0.14, 0.26, "SURPRISED"), nil }}
}

func TestServiceMakesThreeThumbnails(t *testing.T) {
	video := makeVideo(t, "640x360", false)
	an := &fakeAnalyzer{out: goodAnalysis()}
	var stages []Stage
	svc := &Service{Detector: faceEverywhere(), Analyzer: an, Progress: func(s Stage) { stages = append(stages, s) }}

	out, err := svc.Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir(), Transcript: "hello world", Brand: "My Channel"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Images) != 3 {
		t.Fatalf("got %d images, want 3", len(out.Images))
	}
	wantStages := []Stage{StageExtracting, StageSelecting, StageAnalyzing, StageConcepts, StageRendering}
	if len(stages) != len(wantStages) {
		t.Fatalf("stages = %v", stages)
	}
	for i := range wantStages {
		if stages[i] != wantStages[i] {
			t.Errorf("stage %d = %q, want %q", i, stages[i], wantStages[i])
		}
	}
	for i, im := range out.Images {
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(im.JPEG))
		if err != nil {
			t.Fatalf("image %d is not a JPEG: %v", i, err)
		}
		if cfg.Width != 1280 || cfg.Height != 720 || im.Width != 1280 || im.Height != 720 {
			t.Errorf("image %d is %dx%d (reported %dx%d), want 1280x720", i, cfg.Width, cfg.Height, im.Width, im.Height)
		}
		if im.Headline != "TEST PATTERNS" {
			t.Errorf("image %d headline = %q", i, im.Headline)
		}
	}
	if out.Images[0].Concept != ConceptFace {
		t.Errorf("first concept = %q; every frame has a face so it should be a close-up", out.Images[0].Concept)
	}
	if out.Style != StyleTechnology {
		t.Errorf("style = %q; with none requested the model's suggestion applies", out.Style)
	}
	if out.Format != FormatLandscape || !out.DetectionUsed || !out.TranscriptUsed {
		t.Errorf("format=%q detection=%v transcript=%v", out.Format, out.DetectionUsed, out.TranscriptUsed)
	}
	f := out.Funnel
	if !(f.Extracted >= f.Usable && f.Usable >= f.Unique && f.Unique >= f.Candidates && f.Candidates >= 1) {
		t.Errorf("funnel should narrow at each step: %+v", f)
	}
	// The AI saw a handful of frames, in time order, plus the transcript.
	if n := len(an.got.Frames); n < 1 || n > 4 {
		t.Errorf("analyzer saw %d frames, want 1..4", n)
	}
	for i := 1; i < len(an.got.Frames); i++ {
		if an.got.Frames[i].Time < an.got.Frames[i-1].Time {
			t.Error("frames must be shown in time order")
		}
	}
	if an.got.Transcript != "hello world" {
		t.Errorf("transcript = %q", an.got.Transcript)
	}
}

func TestServiceChoosesFormatFromTheVideo(t *testing.T) {
	video := makeVideo(t, "360x640", false)
	svc := &Service{Detector: faceEverywhere(), Analyzer: &fakeAnalyzer{out: goodAnalysis()}}
	out, err := svc.Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if out.Format != FormatPortrait || out.Images[0].Width != 720 || out.Images[0].Height != 1280 {
		t.Errorf("a vertical video should get 9:16 thumbnails, got %q %dx%d", out.Format, out.Images[0].Width, out.Images[0].Height)
	}
}

func TestServiceHonoursRequestedFormatAndStyle(t *testing.T) {
	video := makeVideo(t, "640x360", false)
	svc := &Service{Analyzer: &fakeAnalyzer{out: goodAnalysis()}}
	out, err := svc.Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir(), Format: FormatPortrait, Style: StyleNews})
	if err != nil {
		t.Fatal(err)
	}
	if out.Format != FormatPortrait || out.Style != StyleNews || out.Images[0].Height != 1280 {
		t.Errorf("format=%q style=%q height=%d", out.Format, out.Style, out.Images[0].Height)
	}
}

func TestServiceWorksWithoutTranscriptOrDetector(t *testing.T) {
	video := makeVideo(t, "640x360", false)
	an := &fakeAnalyzer{out: goodAnalysis()}
	out, err := (&Service{Analyzer: an}).Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if out.TranscriptUsed || an.got.Transcript != "" {
		t.Error("no transcript was supplied, so none should be reported as used")
	}
	if out.DetectionUsed {
		t.Error("detection was not available")
	}
	if len(out.Images) != 3 {
		t.Errorf("images = %d", len(out.Images))
	}
}

func TestServiceDegradesWhenDetectionFails(t *testing.T) {
	video := makeVideo(t, "640x360", false)
	var calls atomic.Int32
	det := fakeDetector{fn: func([]byte) (Detection, error) { calls.Add(1); return Detection{}, errors.New("AccessDenied") }}
	out, err := (&Service{Detector: det, Analyzer: &fakeAnalyzer{out: goodAnalysis()}}).Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir()})
	if err != nil {
		t.Fatalf("a Rekognition failure must not fail the run: %v", err)
	}
	if calls.Load() == 0 || out.DetectionUsed {
		t.Errorf("calls=%d detectionUsed=%v", calls.Load(), out.DetectionUsed)
	}
	if len(out.Images) != 3 {
		t.Errorf("images = %d", len(out.Images))
	}
}

func TestServiceRejectsAudioOnlyFiles(t *testing.T) {
	needTool(t, "ffmpeg", "ffprobe")
	audio := filepath.Join(t.TempDir(), "a.mp3")
	if out, err := (ExecRunner{}).Run(context.Background(), "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=d=3", "-y", audio); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	_, err := (&Service{Analyzer: &fakeAnalyzer{out: goodAnalysis()}}).Run(context.Background(), Input{VideoPath: audio, WorkDir: t.TempDir()})
	if CodeOf(err) != CodeNoVideoTrack {
		t.Errorf("code = %q, want %q", CodeOf(err), CodeNoVideoTrack)
	}
}

func TestServiceReportsFramesThatAreAllUnusable(t *testing.T) {
	an := &fakeAnalyzer{out: goodAnalysis()}
	_, err := (&Service{Analyzer: an}).Run(context.Background(), Input{VideoPath: blackVideo(t), WorkDir: t.TempDir()})
	if CodeOf(err) != CodeNoSuitableFrames {
		t.Errorf("code = %q, want %q (err %v)", CodeOf(err), CodeNoSuitableFrames, err)
	}
	if an.got.Frames != nil {
		t.Error("the AI must not be called when there is nothing worth showing it")
	}
}

func TestServiceSkipsBlackSections(t *testing.T) {
	video := makeVideo(t, "640x360", true)
	out, err := (&Service{Analyzer: &fakeAnalyzer{out: goodAnalysis()}}).Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if out.Funnel.Usable >= out.Funnel.Extracted {
		t.Errorf("the 4s of black should have cost some frames: %+v", out.Funnel)
	}
}

func TestServicePassesAnalyzerErrorsThrough(t *testing.T) {
	video := makeVideo(t, "640x360", false)
	an := &fakeAnalyzer{err: Errorf(CodeAITimeout, "slow", nil)}
	_, err := (&Service{Analyzer: an}).Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir()})
	if CodeOf(err) != CodeAITimeout {
		t.Errorf("code = %q", CodeOf(err))
	}
}

func TestServiceStopsWhenCancelled(t *testing.T) {
	video := makeVideo(t, "640x360", false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&Service{Analyzer: &fakeAnalyzer{out: goodAnalysis()}}).Run(ctx, Input{VideoPath: video, WorkDir: t.TempDir()}); err == nil {
		t.Error("a cancelled context must stop the run")
	}
}

func TestServiceNeedsADuration(t *testing.T) {
	r := &fakeRunner{run: func(name string, _ []string) ([]byte, error) {
		return []byte(`{"streams":[{"codec_type":"video","width":640,"height":360,"avg_frame_rate":"25/1"}],"format":{}}`), nil
	}}
	_, err := (&Service{Runner: r, Analyzer: &fakeAnalyzer{out: goodAnalysis()}}).Run(context.Background(), Input{VideoPath: "x.mp4", WorkDir: t.TempDir()})
	if CodeOf(err) != CodeSourceUnreadable {
		t.Errorf("code = %q", CodeOf(err))
	}
}

func TestServiceFallsBackToTheJobDuration(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{run: func(name string, args []string) ([]byte, error) {
		if name == "ffprobe" {
			return []byte(`{"streams":[{"codec_type":"video","width":640,"height":360,"avg_frame_rate":"25/1"}],"format":{}}`), nil
		}
		// Pretend ffmpeg wrote the frames: copy a real image six times.
		for i := 1; i <= 6; i++ {
			writeJPEG(t, filepath.Join(dir, "frames", "frame_000"+string(rune('0'+i))+".jpg"), scene(640, 360, i*7))
		}
		return nil, nil
	}}
	_ = os.MkdirAll(filepath.Join(dir, "frames"), 0o755)
	out, err := (&Service{Runner: r, Analyzer: &fakeAnalyzer{out: goodAnalysis()}}).Run(context.Background(), Input{VideoPath: "x.mp4", WorkDir: dir, Duration: 60})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Images) != 3 {
		t.Errorf("images = %d", len(out.Images))
	}
}
