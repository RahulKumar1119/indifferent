package render

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/models"
)

// testCaptions is a complete, valid animated caption setting.
func testCaptions() *models.AnimatedCaptions {
	return &models.AnimatedCaptions{
		Enabled: true, Intensity: 50, Font: "sans", FontSize: 18,
		Color: "#FFFFFF", Highlight: "#FFD60A", Position: "bottom",
		Outline: 4, Background: "dark",
	}
}

func TestPlanEffectsSkipsEventsPastClipEnd(t *testing.T) {
	e := &models.VideoEffects{
		Zoom: &models.ZoomEffect{Enabled: true, Intensity: 40, PunchIns: []models.PunchIn{
			{At: 2, Duration: 1}, {At: 50, Duration: 1}, {At: 9.5, Duration: 2},
		}},
		Freeze: &models.FreezeEffect{Enabled: true, Intensity: 50, Freezes: []models.FreezeHold{{At: 25, Duration: 1}}},
	}
	p := planEffects(e, 10)
	if len(p.punches) != 2 {
		t.Fatalf("expected 2 punch-ins inside the clip, got %+v", p.punches)
	}
	if p.punches[1].dur != 0.5 {
		t.Errorf("a punch-in past the clip end must be shortened to the clip end, got %+v", p.punches[1])
	}
	if len(p.holds) != 0 || p.outSec != 10 {
		t.Errorf("freeze past the clip end must be skipped, got holds=%+v outSec=%v", p.holds, p.outSec)
	}
}

func TestZoomExpressionPulsesPerPunchIn(t *testing.T) {
	e := &models.VideoEffects{Zoom: &models.ZoomEffect{Enabled: true, Intensity: 40, PunchIns: []models.PunchIn{
		{At: 2, Duration: 1.5}, {At: 8, Duration: 0.5},
	}}}
	got := planEffects(e, 10).zoomExpr()
	want := "1+0.2000*(between(time,2.000,3.500)*sin(PI*(time-2.000)/1.500)+between(time,8.000,8.500)*sin(PI*(time-8.000)/0.500))"
	if got != want {
		t.Fatalf("zoom expression\n got: %s\nwant: %s", got, want)
	}
}

func TestGradeLookAtZeroIntensityIsIdentity(t *testing.T) {
	for _, look := range []string{"cinematic", "warm", "cool", "high-contrast"} {
		g := gradeLookFor(look, 0)
		if g.contrast != 1 || g.saturation != 1 {
			t.Errorf("%s at intensity 0: contrast=%v saturation=%v, want 1 and 1", look, g.contrast, g.saturation)
		}
		for i, v := range g.balance {
			if v != 0 {
				t.Errorf("%s at intensity 0: balance[%d]=%v, want 0", look, i, v)
			}
		}
		filters := strings.Join(g.filters(), ";")
		if !strings.Contains(filters, "eq=contrast=1.000:saturation=1.000") || !strings.Contains(filters, "colorbalance=") {
			t.Errorf("%s: expected eq and colorbalance in %q", look, filters)
		}
	}
}

func TestGradeFiltersUseEqCurvesAndColorbalance(t *testing.T) {
	for _, look := range []string{"cinematic", "warm", "cool", "high-contrast"} {
		filters := strings.Join(gradeLookFor(look, 1).filters(), ";")
		if !strings.Contains(filters, "eq=") || !strings.Contains(filters, "curves=") || !strings.Contains(filters, "colorbalance=") {
			t.Errorf("%s: expected eq, curves and colorbalance, got %q", look, filters)
		}
	}
}

// freezeEffects holds frame 30 (1.0 s) for 0.5 s and frame 60 (2.0 s) for 0.2 s.
func freezeEffects() *models.VideoEffects {
	return &models.VideoEffects{Freeze: &models.FreezeEffect{
		Enabled: true, Intensity: 50,
		Freezes: []models.FreezeHold{{At: 1, Duration: 0.5}, {At: 2, Duration: 0.2}},
	}}
}

func TestFreezeSegmentsAndOutputLength(t *testing.T) {
	p := planEffects(freezeEffects(), 10)
	wantHolds := []frameHold{{frame: 30, extra: 15}, {frame: 60, extra: 6}}
	if !reflect.DeepEqual(p.holds, wantHolds) {
		t.Fatalf("holds: got %+v want %+v", p.holds, wantHolds)
	}
	if math.Abs(p.outSec-10.7) > 1e-9 {
		t.Errorf("output length: got %v want 10.7 (clip plus 21 held frames)", p.outSec)
	}
	wantSegs := []segment{
		{from: 0, to: 30},
		{hold: true, from: 30, to: 31, extra: 15},
		{from: 31, to: 60},
		{hold: true, from: 60, to: 61, extra: 6},
		{from: 61, to: -1},
	}
	if got := p.segments(); !reflect.DeepEqual(got, wantSegs) {
		t.Fatalf("segments: got %+v want %+v", got, wantSegs)
	}
}

func TestFreezeGraphHoldsFramesAndSilencesAudio(t *testing.T) {
	p := planEffects(freezeEffects(), 10)
	graph, audioLabel, err := p.graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"trim=start_frame=30:end_frame=31,setpts=PTS-STARTPTS,tpad=stop=15:stop_mode=clone",
		"trim=start_frame=60:end_frame=61,setpts=PTS-STARTPTS,tpad=stop=6:stop_mode=clone",
		"trim=start_frame=61,",
		"anullsrc=r=48000:cl=stereo:d=0.533333",
		"anullsrc=r=48000:cl=stereo:d=0.233333",
		"split=5",
		"asplit=3",
		"concat=n=5:v=1:a=0",
		"concat=n=5:v=0:a=1",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("freeze graph is missing %q\n%s", want, graph)
		}
	}
	if audioLabel == "" {
		t.Error("audio with freeze holds must be cut and mapped")
	}
	if !strings.HasSuffix(graph, "[vout]") {
		t.Errorf("graph must end at [vout]: %s", graph)
	}
}

func TestFreezeWithoutAudioBuildsNoAudioChain(t *testing.T) {
	p := planEffects(freezeEffects(), 10)
	graph, audioLabel, err := p.graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: ""})
	if err != nil {
		t.Fatal(err)
	}
	if audioLabel != "" || strings.Contains(graph, "asplit") || strings.Contains(graph, "anullsrc") {
		t.Errorf("video without audio must build no audio chain, got label %q in %s", audioLabel, graph)
	}
}

func TestRemapWordsMovesWordsAfterAHold(t *testing.T) {
	p := planEffects(freezeEffects(), 10)
	words := []models.Word{
		{Text: "before", Start: 10.2, End: 10.6},
		{Text: "after", Start: 11.5, End: 11.9},
	}
	got := p.remapWords(words, 10)
	// The hold at 1.0 s adds 0.5 s, so a word at 1.5 s moves to 2.0 s.
	if math.Abs(got[0].Start-10.2) > 1e-9 || math.Abs(got[0].End-10.6) > 1e-9 {
		t.Errorf("word before the hold must not move, got %+v", got[0])
	}
	if math.Abs(got[1].Start-12.0) > 1e-9 || math.Abs(got[1].End-12.4) > 1e-9 {
		t.Errorf("word after the hold must move by 0.5 s, got %+v", got[1])
	}
}

func TestImpactTextKeepsUserTextOutOfTheGraph(t *testing.T) {
	const text = `DROP; [x]: 100% it's, "a" \b {c}`
	e := &models.VideoEffects{Freeze: &models.FreezeEffect{Enabled: true, Intensity: 50,
		ImpactTexts: []models.ImpactText{{At: 1, Duration: 1, Text: text}}}}
	p := planEffects(e, 10)
	dir := t.TempDir()
	if err := writeImpactFiles(dir, "clip-1", p.texts); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(p.texts[0].file)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != text {
		t.Fatalf("impact text file: got %q want %q", body, text)
	}
	graph, _, err := p.graph(graphInput{framing: "null", videoIn: "[0:v]"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(graph, "DROP") || strings.Contains(graph, "100%") {
		t.Errorf("user text must not reach the filtergraph: %s", graph)
	}
	for _, want := range []string{"textfile=", "expansion=none", "fontsize=66", "enable='between(t,1.000,2.000)'"} {
		if !strings.Contains(graph, want) {
			t.Errorf("impact graph is missing %q: %s", want, graph)
		}
	}
}

func TestFilterPathAcceptsOnlySafeCharacters(t *testing.T) {
	for _, bad := range []string{"a b.srt", "a:b.srt", "x;y.srt", "it's.srt", "%x.srt", "a,b.srt", "a[b].srt", "a\\b.srt"} {
		if _, err := filterPath(bad); err == nil {
			t.Errorf("filterPath(%q) must reject the path", bad)
		}
	}
	if _, err := filterPath("/tmp/shorts-render-42/clip-1.ass"); err != nil {
		t.Errorf("safe path rejected: %v", err)
	}
}

func TestBuildAnimatedASSEscapesUserText(t *testing.T) {
	words := []models.Word{
		{Text: "it's", Start: 0, End: 0.1},
		{Text: "a:b", Start: 0.1, End: 0.2},
		{Text: `C:\path`, Start: 0.2, End: 0.3},
		{Text: "100%", Start: 0.3, End: 0.4},
		{Text: "x,y;z", Start: 0.4, End: 0.5},
		{Text: "[tag]", Start: 0.5, End: 0.6},
		{Text: "{brace}", Start: 0.6, End: 0.7},
		{Text: "line\nbreak", Start: 0.7, End: 0.8},
	}
	doc := BuildAnimatedASS(words, 0, *testCaptions())
	for _, want := range []string{"it's", "a:b", "C:/path", "100%", "x,y;z", "[tag]", "(brace)", "line break"} {
		if !strings.Contains(doc, want) {
			t.Errorf("ASS document is missing %q\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "{brace}") || strings.Contains(doc, "line\nbreak") {
		t.Errorf("braces and line breaks must not reach the ASS text\n%s", doc)
	}
	if got, tags := strings.Count(doc, `\`), strings.Count(doc, `{\kf`); got != tags {
		t.Errorf("only karaoke tags may contain a backslash: %d backslashes, %d tags", got, tags)
	}
	if got := strings.Count(doc, `{\kf`); got != len(words) {
		t.Errorf("expected one karaoke tag per word: got %d for %d words", got, len(words))
	}
}

func TestBuildAnimatedASSKeepsHindiAndStyles(t *testing.T) {
	words := []models.Word{
		{Text: "नमस्ते", Start: 0, End: 0.5},
		{Text: "दुनिया", Start: 0.5, End: 1.0},
	}
	doc := BuildAnimatedASS(words, 0, *testCaptions())
	for _, want := range []string{"नमस्ते", "दुनिया", "Style: Default,DejaVu Sans,120,&H000AD6FF,&H80FFFFFF,&H99000000,&H99000000,0,0,0,0,100,100,0,0,3,4,0,2,40,40,90,1"} {
		if !strings.Contains(doc, want) {
			t.Errorf("ASS document is missing %q\n%s", want, doc)
		}
	}
}

func TestBuildAnimatedASSSweepsFollowSpeech(t *testing.T) {
	words := []models.Word{
		{Text: "A", Start: 10.0, End: 10.5},
		{Text: "B", Start: 11.0, End: 11.5},
	}
	doc := BuildAnimatedASS(words, 10, *testCaptions())
	// A's sweep is its own length. B's sweep starts where A ended, so the
	// silence between them is inside B's sweep and the highlight stays on speech.
	if !strings.Contains(doc, `{\kf50}A {\kf100}B`) {
		t.Errorf("unexpected word sweeps\n%s", doc)
	}
}

func TestCaptionBurnChoosesStyleByKind(t *testing.T) {
	got, err := captionBurn("/tmp/shorts-render-1/c1.ass", models.CaptionStyle{}, true)
	if err != nil || got != "subtitles=/tmp/shorts-render-1/c1.ass" {
		t.Errorf("animated captions: got %q, %v", got, err)
	}
	got, err = captionBurn("/tmp/shorts-render-1/c1.srt", models.CaptionStyle{Preset: "bold"}, false)
	if err != nil || !strings.HasPrefix(got, "subtitles=/tmp/shorts-render-1/c1.srt:force_style='") {
		t.Errorf("srt captions: got %q, %v", got, err)
	}
	if got, err := captionBurn("", models.CaptionStyle{}, true); err != nil || got != "" {
		t.Errorf("no caption file must mean no filter, got %q, %v", got, err)
	}
}

func TestEffectArgsAudioOnlyCanvasCoversHeldFrames(t *testing.T) {
	p := planEffects(freezeEffects(), 10)
	args, err := effectArgs(effectJob{
		src: "audio.mp3", out: "out.mp4", start: 5, end: 15, audioOnly: true,
		plan: p,
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "color=c=black:s=1080x1920:r=30:d=10.700") {
		t.Errorf("audio-only canvas must cover the held frames: %s", joined)
	}
	if !strings.Contains(joined, "-shortest") {
		t.Errorf("audio-only renders must end with the shortest input: %s", joined)
	}
}

func TestEffectArgsWithoutAudioMapsNoAudio(t *testing.T) {
	p := planEffects(freezeEffects(), 10)
	args, err := effectArgs(effectJob{src: "silent.mp4", out: "out.mp4", start: 0, end: 10, framing: cropCaptionFilter, plan: p})
	if err != nil {
		t.Fatal(err)
	}
	if got := argValue(args, "-map"); got != "[vout]" {
		t.Errorf("first -map must be the video output, got %q", got)
	}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "asplit") || !strings.Contains(joined, "-map 0:a?") {
		t.Errorf("a silent source must map no audio: %s", joined)
	}
}

func TestEffectEncodeTimeoutBounds(t *testing.T) {
	cases := map[float64]time.Duration{
		1:   2 * time.Minute,
		30:  5 * time.Minute,
		90:  15 * time.Minute,
		120: 15 * time.Minute,
	}
	for sec, want := range cases {
		if got := effectEncodeTimeout(sec); got != want {
			t.Errorf("timeout for %vs: got %v want %v", sec, got, want)
		}
	}
}

// effectsRenderer returns a renderer over an in-memory store and the matching
// render input: one 18 s clip with two spoken words.
func effectsRenderer(t *testing.T, effects *models.VideoEffects, run func(context.Context, string, []string, float64, func(int)) error) (*Renderer, *mockStorage, RenderInput) {
	t.Helper()
	tData, _ := json.Marshal(models.Transcript{Text: "hello world", Words: []models.Word{
		{Text: "hello", Start: 10.0, End: 10.5},
		{Text: "world", Start: 10.5, End: 11.0},
	}})
	store := &mockStorage{objects: map[string][]byte{
		"uploads/u1/j1/source.mp4":     []byte("SOURCE-BYTES"),
		"shorts/u1/j1/transcript.json": tData,
	}}
	r := &Renderer{Storage: store, Bucket: "assets", WorkDir: t.TempDir(), RunCommandProgress: run}
	in := RenderInput{
		JobID: "j1", UserID: "u1",
		SourceKey:     "uploads/u1/j1/source.mp4",
		TranscriptKey: "shorts/u1/j1/transcript.json",
		Clip:          models.Clip{ClipID: "c1"},
		ClipStart:     10.0,
		ClipEnd:       28.0,
		Effects:       effects,
	}
	return r, store, in
}

// writeOutput is a fake ffmpeg that writes the output file it is asked for.
func writeOutput(args []string) error {
	return os.WriteFile(args[len(args)-1], []byte("RENDERED-CLIP"), 0o644)
}

func TestRenderWithoutEffectsKeepsLegacyArgs(t *testing.T) {
	disabled := &models.VideoEffects{
		Zoom:  &models.ZoomEffect{Enabled: false, Intensity: 40},
		Grade: &models.GradeEffect{Enabled: false, Look: "warm", Intensity: 50},
	}
	for name, effects := range map[string]*models.VideoEffects{"nil": nil, "all disabled": disabled} {
		t.Run(name, func(t *testing.T) {
			var got []string
			run := func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
				got = args
				return writeOutput(args)
			}
			r, _, in := effectsRenderer(t, effects, run)
			if _, err := r.Render(context.Background(), in); err != nil {
				t.Fatal(err)
			}
			want := r.buildCropCaptionArgs(filepath.Join(r.WorkDir, "source.mp4"), filepath.Join(r.WorkDir, "c1.srt"),
				filepath.Join(r.WorkDir, "c1.mp4"), 10.0, 28.0, false, "", false, models.CaptionStyle{})
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("render args changed\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

func TestRenderWithEffectsUsesFilterGraphUnderDeadline(t *testing.T) {
	effects := &models.VideoEffects{Grade: &models.GradeEffect{Enabled: true, Look: "warm", Intensity: 50}}
	var got []string
	run := func(ctx context.Context, _ string, args []string, _ float64, _ func(int)) error {
		got = args
		if _, ok := ctx.Deadline(); !ok {
			t.Error("a render with effects must run under a deadline")
		}
		return writeOutput(args)
	}
	r, store, in := effectsRenderer(t, effects, run)
	if _, err := r.Render(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if argValue(got, "-filter_complex") == "" {
		t.Fatalf("render with effects must use -filter_complex: %q", got)
	}
	if store.putCalls != 1 {
		t.Errorf("expected one upload, got %d", store.putCalls)
	}
}

func TestRenderRejectsInvalidEffectsBeforeFFmpeg(t *testing.T) {
	effects := &models.VideoEffects{Zoom: &models.ZoomEffect{Enabled: true, Intensity: 200, PunchIns: []models.PunchIn{{At: 1, Duration: 1}}}}
	called := false
	run := func(context.Context, string, []string, float64, func(int)) error {
		called = true
		return nil
	}
	r, store, in := effectsRenderer(t, effects, run)
	_, err := r.Render(context.Background(), in)
	if !errors.Is(err, ErrEffectsInvalid) {
		t.Fatalf("expected ErrEffectsInvalid, got %v", err)
	}
	if called || store.putCalls != 0 || store.getCalls != 0 {
		t.Errorf("invalid settings must stop before any work: ffmpeg=%v upload=%d reads=%d", called, store.putCalls, store.getCalls)
	}
}

func TestRenderEffectTimeoutReturnsTimeoutStatus(t *testing.T) {
	effects := &models.VideoEffects{Grade: &models.GradeEffect{Enabled: true, Look: "cool", Intensity: 50}}
	run := func(ctx context.Context, _ string, _ []string, _ float64, _ func(int)) error {
		<-ctx.Done()
		return ctx.Err()
	}
	r, store, in := effectsRenderer(t, effects, run)
	r.EffectTimeout = time.Millisecond
	_, err := r.Render(context.Background(), in)
	if !errors.Is(err, ErrRenderTimeout) {
		t.Fatalf("expected ErrRenderTimeout, got %v", err)
	}
	if errors.Is(err, ErrRenderCanceled) || store.putCalls != 0 {
		t.Errorf("a timeout must not look like a cancel, and must not upload: %v, uploads=%d", err, store.putCalls)
	}
}

func TestRenderCancelledReturnsCanceledStatus(t *testing.T) {
	effects := &models.VideoEffects{Grade: &models.GradeEffect{Enabled: true, Look: "cool", Intensity: 50}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := func(runCtx context.Context, _ string, _ []string, _ float64, _ func(int)) error {
		cancel()
		<-runCtx.Done()
		return runCtx.Err()
	}
	r, store, in := effectsRenderer(t, effects, run)
	_, err := r.Render(ctx, in)
	if !errors.Is(err, ErrRenderCanceled) {
		t.Fatalf("expected ErrRenderCanceled, got %v", err)
	}
	if errors.Is(err, ErrRenderTimeout) || store.putCalls != 0 {
		t.Errorf("a cancel must not look like a timeout, and must not upload: %v, uploads=%d", err, store.putCalls)
	}
}

func TestRenderFFmpegFailureKeepsMessage(t *testing.T) {
	effects := &models.VideoEffects{Grade: &models.GradeEffect{Enabled: true, Look: "cool", Intensity: 50}}
	run := func(context.Context, string, []string, float64, func(int)) error {
		return errors.New("exit status 1")
	}
	r, store, in := effectsRenderer(t, effects, run)
	_, err := r.Render(context.Background(), in)
	if err == nil || !strings.Contains(err.Error(), "ffmpeg render failed: exit status 1") {
		t.Fatalf("expected the ffmpeg failure, got %v", err)
	}
	if errors.Is(err, ErrRenderTimeout) || errors.Is(err, ErrRenderCanceled) || store.putCalls != 0 {
		t.Errorf("an ffmpeg failure is neither a timeout nor a cancel, and must not upload: %v", err)
	}
}
