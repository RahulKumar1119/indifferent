package render

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
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
	// The clip is 10 s and the freeze adds 0.7 s. The canvas is the clip
	// length, and the hold graph adds the held frames.
	if !strings.Contains(joined, "color=c=black:s=1080x1920:r=30:d=10.000") {
		t.Errorf("audio-only canvas must be the clip length, the hold graph adds the held frames: %s", joined)
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

// --- Fade in / out (issue #172) ---

func TestPlanFadeSetsTimingFromOutputLength(t *testing.T) {
	e := &models.VideoEffects{Fade: &models.FadeEffect{Enabled: true, InDuration: 0.5, OutDuration: 0.8}}
	p := planEffects(e, 10)
	if p.fade == nil || p.fade.in != 0.5 || p.fade.out != 0.8 {
		t.Fatalf("fade timing: got %+v want in=0.5 out=0.8", p.fade)
	}
}

func TestPlanFadeUsesFreezeExtendedLength(t *testing.T) {
	// Freeze adds 0.7s (10.7s out). The fade-out must start from the longer length.
	e := freezeEffects()
	e.Fade = &models.FadeEffect{Enabled: true, OutDuration: 1.0}
	p := planEffects(e, 10)
	got := strings.Join(p.fade.videoFilters(p.outSec), ",")
	if !strings.Contains(got, "fade=t=out:st=9.700:d=1.000") {
		t.Fatalf("fade-out must start at outSec-dur on the extended timeline, got %q (outSec=%v)", got, p.outSec)
	}
}

func TestPlanFadeClampsToClipLength(t *testing.T) {
	// 2s + 2s of fade on a 3s clip: both shrink in proportion to fill it.
	e := &models.VideoEffects{Fade: &models.FadeEffect{Enabled: true, InDuration: 2, OutDuration: 2}}
	p := planEffects(e, 3)
	if p.fade == nil {
		t.Fatal("expected a fade")
	}
	if total := p.fade.in + p.fade.out; math.Abs(total-3) > 1e-6 {
		t.Errorf("clamped fades should fill the clip, got in=%v out=%v (sum %v)", p.fade.in, p.fade.out, total)
	}
}

func TestPlanFadeDropsSubFrameSides(t *testing.T) {
	e := &models.VideoEffects{Fade: &models.FadeEffect{Enabled: true, InDuration: 0.005, OutDuration: 0}}
	if p := planEffects(e, 10); p.fade != nil {
		t.Errorf("a fade shorter than a frame on its only side must be dropped, got %+v", p.fade)
	}
}

func TestFadeGraphAppliesVideoAndAudioLast(t *testing.T) {
	e := &models.VideoEffects{Fade: &models.FadeEffect{Enabled: true, InDuration: 0.4, OutDuration: 0.6}}
	p := planEffects(e, 10)
	graph, audioLabel, err := p.graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]", burn: "subtitles=/tmp/x.ass"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"fade=t=in:st=0:d=0.400",
		"fade=t=out:st=9.400:d=0.600",
		"afade=t=in:st=0:d=0.400",
		"afade=t=out:st=9.400:d=0.600",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("fade graph missing %q\n%s", want, graph)
		}
	}
	// The video fade must run after the caption burn, so burn is not the terminal step.
	if strings.Contains(graph, "subtitles=/tmp/x.ass[vout]") {
		t.Errorf("caption burn must feed the fade, not go straight to [vout]\n%s", graph)
	}
	if !strings.Contains(graph, "[vout]") {
		t.Errorf("graph must produce [vout]: %s", graph)
	}
	if audioLabel == "" {
		t.Error("a fade must produce an audio stream to map")
	}
}

func TestFadeGraphWithoutAudioHasNoAfade(t *testing.T) {
	e := &models.VideoEffects{Fade: &models.FadeEffect{Enabled: true, InDuration: 0.4}}
	p := planEffects(e, 10)
	graph, audioLabel, err := p.graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: ""})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(graph, "afade") || audioLabel != "" {
		t.Errorf("a clip with no audio must not get an afade or an audio label, got label %q\n%s", audioLabel, graph)
	}
	if !strings.Contains(graph, "fade=t=in:st=0:d=0.400") {
		t.Errorf("video fade-in still expected\n%s", graph)
	}
}

func TestFadeGraphAudioOnlyFadesCanvasAudio(t *testing.T) {
	e := &models.VideoEffects{Fade: &models.FadeEffect{Enabled: true, OutDuration: 0.5}}
	p := planEffects(e, 10)
	// Audio-only sources read audio from the second input.
	graph, audioLabel, err := p.graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[1:a]"})
	if err != nil {
		t.Fatal(err)
	}
	if audioLabel == "" || !strings.Contains(graph, "[1:a]afade=t=out:st=9.500:d=0.500") {
		t.Errorf("audio-only fade must read [1:a] and map the result, got label %q\n%s", audioLabel, graph)
	}
}

func TestFadeOffLeavesGraphUnchanged(t *testing.T) {
	off := &models.VideoEffects{Fade: &models.FadeEffect{Enabled: false, InDuration: 1, OutDuration: 1}, Grade: &models.GradeEffect{Enabled: true, Look: "warm", Intensity: 40}}
	graph, _, err := planEffects(off, 10).graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(graph, "fade=") || strings.Contains(graph, "afade=") {
		t.Errorf("a disabled fade must add no fade filters\n%s", graph)
	}
}

// --- Speed ramping (issue #171) ---

func speedEffects(windows ...models.SpeedWindow) *models.VideoEffects {
	return &models.VideoEffects{Speed: &models.SpeedEffect{Enabled: true, Windows: windows}}
}

func TestPlanSpeedSkipsAndClampsWindows(t *testing.T) {
	// Window at 50s is past a 10s clip (skipped); the 8s window is clamped to the clip end.
	e := speedEffects(
		models.SpeedWindow{At: 2, Duration: 2, Rate: 0.5},
		models.SpeedWindow{At: 50, Duration: 1, Rate: 2},
		models.SpeedWindow{At: 9, Duration: 8, Rate: 2},
	)
	p := planEffects(e, 10)
	if len(p.speeds) != 2 {
		t.Fatalf("expected 2 windows inside the clip, got %+v", p.speeds)
	}
	if p.speeds[1].start != 9 || p.speeds[1].dur != 1 {
		t.Errorf("window past the clip end must be clamped to it, got %+v", p.speeds[1])
	}
}

func TestSpeedOutputLength(t *testing.T) {
	// 10s clip. 2s at 0.5x -> 4s; 1s at 2x -> 0.5s. Rest 7s at 1x.
	// out = 7 + 4 + 0.5 = 11.5s.
	p := planEffects(speedEffects(
		models.SpeedWindow{At: 2, Duration: 2, Rate: 0.5},
		models.SpeedWindow{At: 6, Duration: 1, Rate: 2},
	), 10)
	if math.Abs(p.outSec-11.5) > 1e-9 {
		t.Errorf("output length: got %v want 11.5", p.outSec)
	}
	if math.Abs(p.srcSec-10) > 1e-9 {
		t.Errorf("source length: got %v want 10", p.srcSec)
	}
}

func TestSpeedMapsTimesForCaptionsAndImpact(t *testing.T) {
	// One window: 2s to 4s at 0.5x (slow-mo, so it takes 4s of output).
	p := planEffects(speedEffects(models.SpeedWindow{At: 2, Duration: 2, Rate: 0.5}), 10)
	cases := []struct{ src, out float64 }{
		{0, 0},     // before the window: 1:1
		{2, 2},     // window start: unchanged
		{3, 4},     // halfway through a 0.5x window: 2 + 1/0.5 = 4
		{4, 6},     // window end: 2 + 2/0.5 = 6
		{10, 12},   // tail: 6 + (10-4) = 12 = outSec
	}
	for _, c := range cases {
		if got := p.mapStart(c.src); math.Abs(got-c.out) > 1e-9 {
			t.Errorf("mapStart(%v) = %v, want %v", c.src, got, c.out)
		}
		if got := p.mapEnd(c.src); math.Abs(got-c.out) > 1e-9 {
			t.Errorf("mapEnd(%v) = %v, want %v", c.src, got, c.out)
		}
	}
	if math.Abs(p.outSec-12) > 1e-9 {
		t.Errorf("outSec: got %v want 12", p.outSec)
	}
}

func TestRemapWordsUnderSpeed(t *testing.T) {
	// Clip starts at absolute 100s; a 0.5x window at clip-relative 2-4s.
	p := planEffects(speedEffects(models.SpeedWindow{At: 2, Duration: 2, Rate: 0.5}), 10)
	words := []models.Word{{Text: "slow", Start: 103, End: 104}} // clip-relative 3-4s
	out := p.remapWords(words, 100)
	// src 3 -> out 4, src 4 -> out 6, absolute = clipStart + mapped.
	if math.Abs(out[0].Start-104) > 1e-9 || math.Abs(out[0].End-106) > 1e-9 {
		t.Errorf("word under slow-mo: got %+v want start 104 end 106", out[0])
	}
}

func TestSpeedGraphRetimesVideoAndAudio(t *testing.T) {
	p := planEffects(speedEffects(models.SpeedWindow{At: 2, Duration: 2, Rate: 0.5}), 10)
	graph, audioLabel, err := p.graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"split=3",            // before / window / after
		"asplit=3",
		"trim=start=0.000000:end=2.000000,setpts=PTS-STARTPTS",       // normal run keeps PTS
		"trim=start=2.000000:end=4.000000,setpts=(PTS-STARTPTS)/0.500000", // 0.5x window
		"atempo=0.500000",    // audio slowed to match
		"concat=n=3:v=1:a=0",
		"concat=n=3:v=0:a=1",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("speed graph missing %q\n%s", want, graph)
		}
	}
	if audioLabel == "" {
		t.Error("speed must map a retimed audio stream")
	}
	if !strings.Contains(graph, "[vout]") {
		t.Errorf("graph must produce [vout]: %s", graph)
	}
}

func TestSpeedGraphWithoutAudio(t *testing.T) {
	p := planEffects(speedEffects(models.SpeedWindow{At: 1, Duration: 2, Rate: 2}), 10)
	graph, audioLabel, err := p.graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: ""})
	if err != nil {
		t.Fatal(err)
	}
	if audioLabel != "" || strings.Contains(graph, "atempo") || strings.Contains(graph, "asplit") {
		t.Errorf("a clip with no audio must build no audio chain, got label %q\n%s", audioLabel, graph)
	}
	if !strings.Contains(graph, "setpts=(PTS-STARTPTS)/2.000000") {
		t.Errorf("video must still be retimed: %s", graph)
	}
}

func TestAtempoFactors(t *testing.T) {
	cases := []struct {
		rate float64
		want []float64
	}{
		{1, nil},
		{0.5, []float64{0.5}},
		{2, []float64{2}},
		{4, []float64{2, 2}},     // 4x needs two 2.0 stages
		{0.25, []float64{0.5, 0.5}}, // 0.25x needs two 0.5 stages
		{3, []float64{2, 1.5}},
	}
	for _, c := range cases {
		got := atempoFactors(c.rate)
		if len(got) != len(c.want) {
			t.Fatalf("atempoFactors(%v) = %v, want %v", c.rate, got, c.want)
		}
		product := 1.0
		for i, f := range got {
			if f < 0.5-1e-9 || f > 2.0+1e-9 {
				t.Errorf("atempoFactors(%v): factor %v outside 0.5-2.0", c.rate, f)
			}
			if math.Abs(f-c.want[i]) > 1e-9 {
				t.Errorf("atempoFactors(%v)[%d] = %v, want %v", c.rate, i, f, c.want[i])
			}
			product *= f
		}
		if math.Abs(product-c.rate) > 1e-9 {
			t.Errorf("atempoFactors(%v) product = %v, want %v", c.rate, product, c.rate)
		}
	}
}

func TestSpeedOffLeavesGraphUnchanged(t *testing.T) {
	off := &models.VideoEffects{
		Speed: &models.SpeedEffect{Enabled: false, Windows: []models.SpeedWindow{{At: 1, Duration: 2, Rate: 2}}},
		Grade: &models.GradeEffect{Enabled: true, Look: "warm", Intensity: 40},
	}
	graph, _, err := planEffects(off, 10).graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(graph, "atempo") || strings.Contains(graph, "setpts=(PTS") {
		t.Errorf("a disabled speed effect must add no retiming\n%s", graph)
	}
}

// --- Phase 3: finish look (#176), waveform (#177), camera shake (#178) ---

func TestFinishFiltersPerSubEffect(t *testing.T) {
	// Each sub-effect appears only when its strength is above 0.
	p := planEffects(&models.VideoEffects{Finish: &models.FinishEffect{Enabled: true, Grain: 40, Glow: 0, LightLeak: 0}}, 10)
	f := strings.Join(p.finishFilters(), ",")
	if !strings.Contains(f, "noise=") || strings.Contains(f, "unsharp") || strings.Contains(f, "drawbox") {
		t.Errorf("grain only should emit noise and nothing else, got %q", f)
	}
	p = planEffects(&models.VideoEffects{Finish: &models.FinishEffect{Enabled: true, Glow: 50}}, 10)
	if f = strings.Join(p.finishFilters(), ","); !strings.Contains(f, "unsharp") {
		t.Errorf("glow should emit unsharp, got %q", f)
	}
	p = planEffects(&models.VideoEffects{Finish: &models.FinishEffect{Enabled: true, LightLeak: 70}}, 10)
	if f = strings.Join(p.finishFilters(), ","); !strings.Contains(f, "drawbox") || !strings.Contains(f, "orange@") {
		t.Errorf("light leak should emit an orange drawbox, got %q", f)
	}
}

func TestFinishOffEmitsNothing(t *testing.T) {
	p := planEffects(&models.VideoEffects{Finish: &models.FinishEffect{Enabled: false, Grain: 80}}, 10)
	if p.finish != nil || len(p.finishFilters()) != 0 {
		t.Errorf("a disabled finish must contribute no filters, got %v", p.finishFilters())
	}
}

func TestFinishInGraphAfterGrade(t *testing.T) {
	e := &models.VideoEffects{
		Grade:  &models.GradeEffect{Enabled: true, Look: "warm", Intensity: 40},
		Finish: &models.FinishEffect{Enabled: true, Grain: 30, Glow: 40},
	}
	graph, _, err := planEffects(e, 10).graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	gradeAt := strings.Index(graph, "colorbalance=")
	noiseAt := strings.Index(graph, "noise=")
	if gradeAt < 0 || noiseAt < 0 || noiseAt < gradeAt {
		t.Errorf("finish must come after the grade\n%s", graph)
	}
}

func TestShakeFilterOverscansAndStaysInMargin(t *testing.T) {
	// Intensity 100 -> peak shakeMaxPx (24px), margin is 6% of 1080 = ~32px, so it fits.
	p := planEffects(&models.VideoEffects{Shake: &models.ShakeEffect{
		Enabled: true, Intensity: 100, Windows: []models.ShakeWindow{{At: 2, Duration: 1}},
	}}, 10)
	f := p.shakeFilter()
	if !strings.HasPrefix(f, "scale=") || !strings.Contains(f, "crop=1080:1920") {
		t.Errorf("shake must overscan with scale then crop back to the output size, got %q", f)
	}
	// The displacement amplitude must not exceed the margin.
	sw := math.Ceil(outputWidth * shakeOverscan)
	margin := (sw - outputWidth) / 2
	if p.shakeAmp > margin {
		t.Errorf("shake amplitude %v exceeds the overscan margin %v (black edge risk)", p.shakeAmp, margin)
	}
	if !strings.Contains(f, "between(t,2.000,3.000)") {
		t.Errorf("shake must gate motion to its window, got %q", f)
	}
}

func TestShakeSkipsWindowsPastClip(t *testing.T) {
	p := planEffects(&models.VideoEffects{Shake: &models.ShakeEffect{
		Enabled: true, Intensity: 50,
		Windows: []models.ShakeWindow{{At: 2, Duration: 1}, {At: 50, Duration: 1}, {At: 9, Duration: 5}},
	}}, 10)
	if len(p.shakes) != 2 {
		t.Fatalf("expected 2 windows inside the clip, got %+v", p.shakes)
	}
	if p.shakes[1].dur != 1 {
		t.Errorf("a window past the clip end must be clamped, got %+v", p.shakes[1])
	}
}

func TestShakeRunsBeforeRetiming(t *testing.T) {
	// Shake is source-timed: it must appear before a speed stage in the graph.
	e := &models.VideoEffects{
		Shake: &models.ShakeEffect{Enabled: true, Intensity: 60, Windows: []models.ShakeWindow{{At: 1, Duration: 1}}},
		Speed: &models.SpeedEffect{Enabled: true, Windows: []models.SpeedWindow{{At: 3, Duration: 2, Rate: 0.5}}},
	}
	graph, _, err := planEffects(e, 10).graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	shakeAt := strings.Index(graph, "crop=1080:1920:x=")
	speedAt := strings.Index(graph, "setpts=(PTS-STARTPTS)/")
	if shakeAt < 0 || speedAt < 0 || shakeAt > speedAt {
		t.Errorf("shake must run before speed retiming\n%s", graph)
	}
}

func TestShakeOffEmitsNoCrop(t *testing.T) {
	e := &models.VideoEffects{Shake: &models.ShakeEffect{Enabled: false, Intensity: 80, Windows: []models.ShakeWindow{{At: 1, Duration: 1}}}}
	graph, _, err := planEffects(e, 10).graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(graph, "scale=") && strings.Contains(graph, "crop=1080:1920:x=") {
		t.Errorf("a disabled shake must add no overscan crop\n%s", graph)
	}
}

func TestWaveformOverlaysAndSplitsAudio(t *testing.T) {
	e := &models.VideoEffects{Waveform: &models.WaveformEffect{
		Enabled: true, Style: "waves", Color: "#FF0000", Position: "bottom", Height: 25,
	}}
	graph, audioLabel, err := planEffects(e, 10).graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"asplit",                      // audio shared between output and visualizer
		"showwaves=s=1080x480",        // 25% of 1920 = 480
		"colors=0xFF0000",
		"overlay=x=0:y=H-h",           // bottom
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("waveform graph missing %q\n%s", want, graph)
		}
	}
	if audioLabel == "" {
		t.Error("waveform must still map an audio stream")
	}
	if !strings.HasSuffix(graph, "[vout]") {
		t.Errorf("overlay must produce the terminal [vout]: %s", graph)
	}
}

func TestWaveformBarsStyleUsesShowfreqs(t *testing.T) {
	e := &models.VideoEffects{Waveform: &models.WaveformEffect{
		Enabled: true, Style: "bars", Color: "#00FF00", Position: "top", Height: 40,
	}}
	graph, _, err := planEffects(e, 10).graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: "[0:a]"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(graph, "showfreqs=s=1080x768") || !strings.Contains(graph, "overlay=x=0:y=0") {
		t.Errorf("bars style at top should use showfreqs and y=0\n%s", graph)
	}
}

func TestWaveformWithoutAudioIsSkipped(t *testing.T) {
	e := &models.VideoEffects{Waveform: &models.WaveformEffect{Enabled: true, Style: "waves", Color: "#FFFFFF", Position: "bottom", Height: 20}}
	graph, audioLabel, err := planEffects(e, 10).graph(graphInput{framing: "null", videoIn: "[0:v]", audioIn: ""})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(graph, "showwaves") || strings.Contains(graph, "asplit") {
		t.Errorf("no audio means no visualizer\n%s", graph)
	}
	if audioLabel != "" {
		t.Errorf("no audio to map, got %q", audioLabel)
	}
	if !strings.Contains(graph, "[vout]") {
		t.Errorf("graph must still produce [vout]: %s", graph)
	}
}

// TestPhase3GraphsRunThroughFFmpeg builds each Phase 3 effect's real graph and
// runs it through the system ffmpeg on a synthetic clip. This catches
// filtergraph syntax errors that string assertions cannot. Skipped when ffmpeg
// is not on PATH (CI images without it, local machines without it).
func TestPhase3GraphsRunThroughFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	// 3s 1080x1920 test pattern with a tone, so there is video and audio.
	mk := exec.Command("ffmpeg", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=s=1080x1920:r=30:d=3",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", "-y", src)
	if out, err := mk.CombinedOutput(); err != nil {
		t.Fatalf("make source: %v\n%s", err, out)
	}

	cases := map[string]*models.VideoEffects{
		"finish":   {Finish: &models.FinishEffect{Enabled: true, Grain: 40, Glow: 50, LightLeak: 30}},
		"shake":    {Shake: &models.ShakeEffect{Enabled: true, Intensity: 100, Windows: []models.ShakeWindow{{At: 0.5, Duration: 1}}}},
		"wavewaves": {Waveform: &models.WaveformEffect{Enabled: true, Style: "waves", Color: "#FF3B30", Position: "bottom", Height: 25}},
		"wavebars":  {Waveform: &models.WaveformEffect{Enabled: true, Style: "bars", Color: "#00C7FF", Position: "top", Height: 40}},
		"combo":    {Shake: &models.ShakeEffect{Enabled: true, Intensity: 60, Windows: []models.ShakeWindow{{At: 0.5, Duration: 1}}}, Finish: &models.FinishEffect{Enabled: true, Grain: 20}, Waveform: &models.WaveformEffect{Enabled: true, Style: "waves", Color: "#FFFFFF", Position: "middle", Height: 20}},
	}
	for name, e := range cases {
		t.Run(name, func(t *testing.T) {
			graph, audioLabel, err := planEffects(e, 3).graph(graphInput{
				framing: "scale=1080:1920", videoIn: "[0:v]", audioIn: "[0:a]",
			})
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(dir, name+".mp4")
			args := []string{"-v", "error", "-i", src, "-filter_complex", graph, "-map", "[vout]"}
			if audioLabel != "" {
				args = append(args, "-map", audioLabel)
			}
			args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-t", "4", "-y", out)
			if o, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
				t.Fatalf("ffmpeg rejected the %s graph: %v\ngraph: %s\n%s", name, err, graph, o)
			}
			fi, err := os.Stat(out)
			if err != nil || fi.Size() < 1000 {
				t.Fatalf("%s produced no real output: %v", name, err)
			}
		})
	}
}
