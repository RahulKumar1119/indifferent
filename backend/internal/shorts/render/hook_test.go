package render

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// Hook texts at the 80 character limit, in the two supported languages.
const (
	hookEnglish80 = "Nobody tells you this about starting a business until you have lost everything"
	hookHindi80   = "यह बात कोई नहीं बताता कि पहले साल में सबसे बड़ी गलती क्या होती है और उससे कैसे बचें आप सब लोग अभी जानिए ध्यान से"
	hookShort     = "Watch this"
)

func TestHookTestTexts_AreAtTheLimit(t *testing.T) {
	for _, s := range []string{hookEnglish80, hookHindi80} {
		if n := models.HookTextLength(s); n > models.MaxHookChars || n < models.MaxHookChars-6 {
			t.Fatalf("%q is %d characters; the tests want close to %d", s, n, models.MaxHookChars)
		}
	}
}

var hookStyles = []string{models.HookStyleBox, models.HookStyleOutline, models.HookStyleBand}
var hookPositions = []string{models.HookTop, models.HookMiddle}

// captionCases are every caption preset, position, font and size choice a job can
// have, and animated captions across their sizes, outlines and backgrounds.
func captionCases() map[string]HookCaptions {
	out := map[string]HookCaptions{"no captions": {None: true}}
	for _, preset := range []string{"", CaptionMinimal, CaptionBold, CaptionKaraoke, CaptionClean} {
		for _, pos := range []string{"top", "middle", "bottom"} {
			for _, size := range []int{0, 8, 12, 14, 16, 18, 24, 32, 40, 48, 56, 64, 72} {
				for _, above := range []bool{false, true} {
					for _, font := range []string{"", FontSerif, FontCondensed} {
						s := models.CaptionStyle{Preset: preset, Position: pos, FontSize: size, AboveControls: above, Font: font}
						out[fmt.Sprintf("caption %+v", s)] = HookCaptions{Style: s}
					}
				}
			}
		}
	}
	for _, pos := range []string{"top", "middle", "bottom"} {
		for _, size := range []int{8, 12, 18, 24, 32, 48, 72} {
			for _, outline := range []int{0, 6, 12} {
				for _, bg := range []string{"none", "dark", "light"} {
					a := models.AnimatedCaptions{Enabled: true, Font: "sans", FontSize: size, Position: pos, Outline: outline, Background: bg, Color: "#FFFFFF", Highlight: "#FFD60A"}
					out[fmt.Sprintf("animated %+v", a)] = HookCaptions{Animated: &a}
				}
			}
		}
	}
	return out
}

// The hook is never drawn over the captions, never in the top bar or the bottom
// 15%, and every caption choice leaves it room (the hook shrinks if it has to),
// for every style, position and language.
func TestPlanHook_NeverOverlapsCaptions(t *testing.T) {
	noRoom := map[string]int{}
	checked := 0
	for name, caps := range captionCases() {
		capBand, hasCaps := captionBand(caps)
		for _, style := range hookStyles {
			for _, pos := range hookPositions {
				for _, text := range []string{hookShort, hookEnglish80, hookHindi80} {
					h := models.ClipHook{Enabled: true, Text: text, Style: style, Position: pos, Seconds: 2.5}
					p, err := planHook(h, caps)
					if errors.Is(err, ErrHookNoRoom) {
						noRoom[name]++
						t.Errorf("no room for a %s %s hook with %s (size %d)", style, pos, name, captionSize(caps))
						continue
					}
					if err != nil {
						t.Fatalf("%s %s %q with %s: %v", style, pos, text, name, err)
					}
					checked++
					top := p.top - float64(p.look.pad)
					block := band{top, top + hookBlockHeight(p.sizePx, p.lines, p.look)}
					if block.y0 < hookTopSafePx || block.y1 > float64(assPlayResY-hookBottomSafePx) {
						t.Errorf("%s %s hook with %s outside the safe area: %+v", style, pos, name, block)
					}
					if hasCaps && block.overlaps(band{capBand.y0 - hookCaptionGapPx, capBand.y1 + hookCaptionGapPx}) {
						t.Errorf("%s %s hook with %s overlaps the captions: hook %+v captions %+v", style, pos, name, block, capBand)
					}
					if p.lines > 2 || p.sizePx < hookFloorPx {
						t.Errorf("layout %+v", p)
					}
				}
			}
		}
	}
	t.Logf("checked %d hook placements; %d caption choices left no room", checked, len(noRoom))
}

// captionSize is the size a caption choice asks for, in the 8 to 72 scale.
func captionSize(c HookCaptions) int {
	switch {
	case c.None:
		return 0
	case c.Animated != nil:
		return c.Animated.FontSize
	default:
		return resolvedCaption(c.Style).FontSize
	}
}

func TestPlanHook_KeepsItsSlotWhenCaptionsAreElsewhere(t *testing.T) {
	bottom := HookCaptions{Style: models.CaptionStyle{}}
	p, err := planHook(models.ClipHook{Enabled: true, Text: hookShort, Style: models.HookStyleBox, Position: models.HookTop, Seconds: 2.5}, bottom)
	if err != nil || p.top-float64(p.look.pad) != hookTopSafePx {
		t.Fatalf("top hook with bottom captions: %+v %v", p, err)
	}
	p, err = planHook(models.ClipHook{Enabled: true, Text: hookShort, Style: models.HookStyleBox, Position: models.HookMiddle, Seconds: 2.5}, bottom)
	h := hookBlockHeight(p.sizePx, p.lines, p.look)
	if err != nil || int(p.top-float64(p.look.pad)+h/2) != assPlayResY/2 {
		t.Fatalf("middle hook with bottom captions: %+v %v", p, err)
	}
	if p.sizePx != hookBasePx || p.lines != 1 {
		t.Fatalf("a short hook keeps the base size on one line: %+v", p)
	}
}

func TestPlanHook_MovesBesideCaptionsInItsSlot(t *testing.T) {
	top := HookCaptions{Style: models.CaptionStyle{Position: "top"}}
	p, err := planHook(models.ClipHook{Enabled: true, Text: hookShort, Style: models.HookStyleBox, Position: models.HookTop, Seconds: 2.5}, top)
	if err != nil {
		t.Fatal(err)
	}
	cb, _ := captionBand(top)
	if got := p.top - float64(p.look.pad); got < cb.y1+hookCaptionGapPx {
		t.Fatalf("top hook with top captions should sit below them: hook at %.0f, captions end %.0f", got, cb.y1)
	}
}

func TestPlanHook_LongTextWrapsToTwoLinesWithoutSplittingWords(t *testing.T) {
	for _, text := range []string{hookEnglish80, hookHindi80} {
		for _, style := range hookStyles {
			p, err := planHook(models.ClipHook{Enabled: true, Text: text, Style: style, Position: models.HookTop, Seconds: 2.5}, HookCaptions{None: true})
			if err != nil {
				t.Fatalf("%s %q: %v", style, text, err)
			}
			if p.lines != 2 || p.breakAt <= 0 {
				t.Fatalf("%s %q: want two lines, got %+v", style, text, p)
			}
			doc, err := BuildHookASS(models.ClipHook{Enabled: true, Text: text, Style: style}, HookCaptions{None: true}, 30)
			if err != nil {
				t.Fatal(err)
			}
			// The line break falls between words, so every word is intact.
			body := doc[strings.LastIndex(doc, "}")+1:]
			body = strings.TrimSpace(strings.ReplaceAll(body, `\N`, " "))
			if body != text {
				t.Fatalf("drawn text %q, want %q", body, text)
			}
		}
	}
}

func TestPlanHook_RefusesTextThatCannotFitTwoLines(t *testing.T) {
	word := strings.Repeat("W", 60) // one word wider than the frame even at the floor
	_, err := planHook(models.ClipHook{Enabled: true, Text: word, Style: models.HookStyleBox, Position: models.HookTop, Seconds: 2}, HookCaptions{None: true})
	if !errors.Is(err, ErrHookTooLong) {
		t.Fatalf("err = %v, want ErrHookTooLong", err)
	}
}

var dialogue = regexp.MustCompile(`Dialogue: 1,([0-9:.]+),([0-9:.]+),Hook,,0,0,0,,\{\\an8\\pos\(540,(\d+)\)\\fad\((\d+),(\d+)\)\}`)

func TestBuildHookASS_TimingAndFades(t *testing.T) {
	cases := []struct {
		seconds, outSec float64
		wantEnd         string
		wantFade        int
	}{
		{2.5, 30, "0:00:02.50", 200},
		{5, 30, "0:00:05.00", 200},
		{1, 30, "0:00:01.00", 200},
		{2.5, 0.3, "0:00:00.30", 150}, // a clip shorter than the hook: fades halve, never overlap
		{2.5, 1.2, "0:00:01.20", 200},
	}
	for _, c := range cases {
		doc, err := BuildHookASS(models.ClipHook{Enabled: true, Text: hookShort, Style: models.HookStyleOutline, Position: models.HookTop, Seconds: c.seconds}, HookCaptions{}, c.outSec)
		if err != nil {
			t.Fatal(err)
		}
		m := dialogue.FindStringSubmatch(doc)
		if m == nil {
			t.Fatalf("no hook dialogue in %q", doc)
		}
		in, _ := strconv.Atoi(m[4])
		out, _ := strconv.Atoi(m[5])
		if m[1] != "0:00:00.00" || m[2] != c.wantEnd || in != c.wantFade || out != c.wantFade {
			t.Errorf("seconds=%v outSec=%v: start %s end %s fades %d/%d", c.seconds, c.outSec, m[1], m[2], in, out)
		}
	}
}

func TestBuildHookASS_StylesAndEscaping(t *testing.T) {
	want := map[string]string{
		models.HookStyleBox:     ",3,18,0,8,",
		models.HookStyleOutline: ",1,6,3,8,",
		models.HookStyleBand:    ",3,22,0,8,",
	}
	for style, frag := range want {
		doc, err := BuildHookASS(models.ClipHook{Enabled: true, Text: `a {\b1} b`, Style: style}, HookCaptions{None: true}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(doc, "Style: Hook,Arial,") || !strings.Contains(doc, frag) {
			t.Errorf("%s style line missing %q:\n%s", style, frag, doc)
		}
		if !strings.Contains(doc, `a \{\\b1\} b`) {
			t.Errorf("%s: user text not escaped:\n%s", style, doc)
		}
		if strings.Count(doc, "\nDialogue:") != 1 {
			t.Errorf("%s: want one event", style)
		}
	}
}

func TestCheckHook(t *testing.T) {
	if err := CheckHook(models.ClipHook{}, models.CaptionStyle{}, nil); err != nil {
		t.Fatalf("an inactive hook is always fine: %v", err)
	}
	if err := CheckHook(models.ClipHook{Enabled: true, Text: hookHindi80}, models.CaptionStyle{}, nil); err != nil {
		t.Fatalf("80 Hindi characters with default captions: %v", err)
	}
	if err := CheckHook(models.ClipHook{Enabled: true, Text: strings.Repeat("W", 60)}, models.CaptionStyle{}, nil); !errors.Is(err, ErrHookTooLong) {
		t.Fatalf("err = %v, want ErrHookTooLong", err)
	}
	// Animated captions replace the caption style when the effect is on: a
	// middle hook moves off large middle animated captions.
	fx := &models.VideoEffects{Captions: &models.AnimatedCaptions{Enabled: true, FontSize: 72, Position: "middle", Font: "sans", Outline: 4, Background: "dark"}}
	caps := HookCaptionsFor(models.CaptionStyle{}, fx)
	if caps.Animated == nil {
		t.Fatal("animated captions not picked")
	}
	p, err := planHook(models.ClipHook{Enabled: true, Text: hookEnglish80, Style: models.HookStyleBox, Position: models.HookMiddle, Seconds: 2}, caps)
	if err != nil {
		t.Fatal(err)
	}
	cb, _ := captionBand(caps)
	if top := p.top - float64(p.look.pad); top+hookBlockHeight(p.sizePx, p.lines, p.look) > cb.y0 {
		t.Fatalf("middle hook not above large middle animated captions: %+v band %+v", p, cb)
	}
}

func TestPlaceHook_NoRoomWhenCaptionsFillTheSafeArea(t *testing.T) {
	look := hookLooks[models.HookStyleBox]
	if _, ok := placeHook(models.HookTop, 120, look, band{100, 1700}, true); ok {
		t.Fatal("captions over the whole safe area leave no room")
	}
	if _, ok := placeHook(models.HookTop, 120, look, band{100, 1700}, false); !ok {
		t.Fatal("without captions there is always room")
	}
}

func TestHookFade_NeverOverlaps(t *testing.T) {
	for ms := 0; ms <= 6000; ms += 10 {
		if f := hookFade(ms); 2*f > ms || f > hookFadeMs || f < 0 {
			t.Fatalf("hookFade(%d) = %d", ms, f)
		}
	}
}

func TestRenderFingerprint_Hook(t *testing.T) {
	base := RenderInput{ClipStart: 1, ClipEnd: 10}
	plain, _ := renderFingerprint(base)
	off := base
	off.Hook = &models.ClipHook{Enabled: false, Text: "x", Style: "box", Position: "top", Seconds: 2}
	offFP, _ := renderFingerprint(off)
	if offFP != plain {
		t.Fatal("a hook that is off must keep the fingerprint clips had before hooks")
	}
	on := base
	on.Hook = &models.ClipHook{Enabled: true, Text: "x", Style: "box", Position: "top", Seconds: 2, UpdatedAt: "a"}
	onFP, _ := renderFingerprint(on)
	if onFP == plain {
		t.Fatal("an active hook must change the fingerprint")
	}
	again := on
	h := *on.Hook
	h.UpdatedAt = "b"
	again.Hook = &h
	if fp, _ := renderFingerprint(again); fp != onFP {
		t.Fatal("saving the same hook again must keep the fingerprint")
	}
	h.Seconds = 3
	if fp, _ := renderFingerprint(again); fp == onFP {
		t.Fatal("changing the hook must change the fingerprint")
	}
}

func TestWithHookBurn(t *testing.T) {
	got, err := withHookBurn("subtitles=/tmp/w/c.srt:force_style='x'", "/tmp/w/c-hook.ass")
	if err != nil || got != "subtitles=/tmp/w/c.srt:force_style='x',ass=/tmp/w/c-hook.ass:shaping=complex" {
		t.Fatalf("got %q %v", got, err)
	}
	if got, _ := withHookBurn("", "/tmp/w/c-hook.ass"); got != "ass=/tmp/w/c-hook.ass:shaping=complex" {
		t.Fatalf("hook without captions: %q", got)
	}
	if got, _ := withHookBurn("subtitles=a", ""); got != "subtitles=a" {
		t.Fatalf("no hook: %q", got)
	}
	if _, err := withHookBurn("", "/tmp/w/c'hook.ass"); err == nil {
		t.Fatal("a path that is not filter safe must be refused")
	}
}

func TestRender_HookIsBurnedAfterTheCaptions_LegacyPath(t *testing.T) {
	var got []string
	run := func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
		got = args
		return writeOutput(args)
	}
	r, store, in := effectsRenderer(t, nil, run)
	in.Hook = &models.ClipHook{Enabled: true, Text: hookHindi80, Style: models.HookStyleBand, Position: models.HookMiddle, Seconds: 3}
	if _, err := r.Render(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(r.WorkDir, "c1-hook.ass")
	want := r.buildCropCaptionHookArgs(filepath.Join(r.WorkDir, "source.mp4"), filepath.Join(r.WorkDir, "c1.srt"), hookPath,
		filepath.Join(r.WorkDir, "c1.mp4"), 10.0, 28.0, false, "", false, models.CaptionStyle{})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args\n got: %q\nwant: %q", got, want)
	}
	vf := argValue(got, "-vf")
	if !strings.HasSuffix(vf, ","+HookFilter(hookPath)) || strings.Index(vf, "c1.srt") > strings.Index(vf, "c1-hook.ass") {
		t.Fatalf("the hook must be burned after the captions: %q", vf)
	}
	doc, err := os.ReadFile(hookPath)
	if err != nil || !strings.Contains(string(doc), "0:00:03.00,Hook") {
		t.Fatalf("hook file: %v\n%s", err, doc)
	}
	if store.putKey != ClipKey("u1", "j1", "c1") {
		t.Fatalf("uploaded to %q", store.putKey)
	}
}

func TestRender_HookIsBurnedAfterTheCaptions_EffectsPath(t *testing.T) {
	var got []string
	run := func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
		got = args
		return writeOutput(args)
	}
	fx := &models.VideoEffects{
		Grade:    &models.GradeEffect{Enabled: true, Look: "warm", Intensity: 50},
		Captions: &models.AnimatedCaptions{Enabled: true, Intensity: 40, Font: "sans", FontSize: 18, Color: "#FFFFFF", Highlight: "#FFD60A", Position: "middle", Outline: 4, Background: "dark"},
	}
	r, _, in := effectsRenderer(t, fx, run)
	in.Hook = &models.ClipHook{Enabled: true, Text: hookEnglish80, Style: models.HookStyleBox, Position: models.HookMiddle, Seconds: 2.5}
	if _, err := r.Render(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	graph := argValue(got, "-filter_complex")
	caps := "subtitles=" + filepath.Join(r.WorkDir, "c1.ass")
	hook := HookFilter(filepath.Join(r.WorkDir, "c1-hook.ass"))
	if !strings.Contains(graph, caps+","+hook) {
		t.Fatalf("the hook must follow the animated captions in one burn: %q", graph)
	}
	// The middle hook moved off the middle animated captions.
	doc, _ := os.ReadFile(filepath.Join(r.WorkDir, "c1-hook.ass"))
	m := dialogue.FindStringSubmatch(string(doc))
	if m == nil {
		t.Fatalf("no hook event: %s", doc)
	}
	y, _ := strconv.Atoi(m[3])
	cb, _ := captionBand(HookCaptions{Animated: fx.Captions})
	if float64(y) > cb.y0 && float64(y) < cb.y1 {
		t.Fatalf("hook at y=%d is inside the caption band %+v", y, cb)
	}
}

func TestRender_InactiveHookKeepsTheArgsClipsHadBefore(t *testing.T) {
	for name, hook := range map[string]*models.ClipHook{
		"nil": nil,
		"off": {Enabled: false, Text: "x", Style: "box", Position: "top", Seconds: 2},
	} {
		t.Run(name, func(t *testing.T) {
			var got []string
			run := func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
				got = args
				return writeOutput(args)
			}
			r, _, in := effectsRenderer(t, nil, run)
			in.Hook = hook
			if _, err := r.Render(context.Background(), in); err != nil {
				t.Fatal(err)
			}
			want := r.buildCropCaptionArgs(filepath.Join(r.WorkDir, "source.mp4"), filepath.Join(r.WorkDir, "c1.srt"),
				filepath.Join(r.WorkDir, "c1.mp4"), 10.0, 28.0, false, "", false, models.CaptionStyle{})
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("args changed\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

func TestRender_HookThatCannotBeDrawnIsLeftOut(t *testing.T) {
	var got []string
	run := func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
		got = args
		return writeOutput(args)
	}
	r, _, in := effectsRenderer(t, nil, run)
	in.Hook = &models.ClipHook{Enabled: true, Text: strings.Repeat("W", 60), Style: "box", Position: "top", Seconds: 2}
	if _, err := r.Render(context.Background(), in); err != nil {
		t.Fatalf("a hook that cannot be drawn must not fail the clip: %v", err)
	}
	if strings.Contains(argValue(got, "-vf"), "hook") {
		t.Fatalf("hook burned anyway: %q", got)
	}
}

func TestRender_OutputKeyIsWhereASingleClipRenderWrites(t *testing.T) {
	r, store, in := effectsRenderer(t, nil, func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
		return writeOutput(args)
	})
	in.OutputKey = "shorts/u1/j1/clips/c1.r-abc.mp4"
	key, err := r.Render(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if key != in.OutputKey || store.putKey != in.OutputKey || store.headKey != in.OutputKey {
		t.Fatalf("key %q put %q head %q", key, store.putKey, store.headKey)
	}
}
