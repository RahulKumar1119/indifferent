package storyvideo

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func specOf(t *testing.T, scenes []Scene, narr []float64) Spec {
	t.Helper()
	tl, err := BuildTimeline(narr)
	if err != nil {
		t.Fatal(err)
	}
	return Spec{Scenes: scenes, Timeline: tl, OutputPath: "/work/out.mp4"}
}

func scene(kind MediaKind, w, h int, n string) Scene {
	return Scene{Kind: kind, MediaPath: "/work/m" + n, Width: w, Height: h, NarrationPath: "/work/n" + n + ".mp3"}
}

func graphOf(t *testing.T, args []string) string {
	t.Helper()
	for i, a := range args {
		if a == "-filter_complex" {
			return args[i+1]
		}
	}
	t.Fatal("no -filter_complex in " + strings.Join(args, " "))
	return ""
}

func TestFrameFor_ChoosesByShape(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		want Framing
	}{
		{1080, 1920, FramingCover}, // 9:16 phone video
		{1280, 1920, FramingCover}, // 2:3 portrait photo
		{1200, 1600, FramingCover}, // 3:4, the widest still cropped
		{1200, 1500, FramingFit},   // 4:5 would lose too much
		{1200, 1200, FramingFit},
		{1920, 1080, FramingFit},
		{1920, 1280, FramingFit},
		{100, 0, FramingFit}, // never divides by zero
	} {
		if got := FrameFor(tc.w, tc.h); got != tc.want {
			t.Errorf("FrameFor(%d,%d) = %s, want %s", tc.w, tc.h, got, tc.want)
		}
	}
}

func TestBuildArgs_OutputSpecMatchesAIShorts(t *testing.T) {
	args, err := BuildArgs(specOf(t, []Scene{scene(KindPhoto, 1280, 1920, "1"), scene(KindVideo, 1920, 1080, "2")}, []float64{3, 3}))
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(args, " ") + " "
	for _, want := range []string{" -c:v libx264 ", " -pix_fmt yuv420p ", " -r 30 ", " -c:a aac ", " -ar 48000 ", " -movflags +faststart ", " -y /work/out.mp4 "} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %s", want, joined)
		}
	}
}

func TestBuildArgs_EveryInputGetsItsOwnLabelsAndNothingIsLeftDangling(t *testing.T) {
	// All three shapes of picture, so every branch of the graph is in play:
	// a fitted still, a cropped still, a fitted clip, a cropped clip.
	scenes := []Scene{
		scene(KindPhoto, 1920, 1280, "1"),
		scene(KindPhoto, 1280, 1920, "2"),
		scene(KindVideo, 1920, 1080, "3"),
		scene(KindVideo, 1080, 1920, "4"),
	}
	args, err := BuildArgs(specOf(t, scenes, []float64{3, 3, 3, 3}))
	if err != nil {
		t.Fatal(err)
	}
	graph := graphOf(t, args)
	// A label used as an output ([name] after a chain) must be unique, and
	// every label used as an input must have been produced before.
	produced := map[string]bool{}
	for _, stmt := range strings.Split(graph, ";") {
		ins := regexp.MustCompile(`^(\[[^\]]+\])+`).FindString(stmt)
		for _, in := range regexp.MustCompile(`\[[^\]]+\]`).FindAllString(ins, -1) {
			if !regexp.MustCompile(`^\[\d+:[av]\]$`).MatchString(in) && !produced[in] {
				t.Errorf("statement uses %s before anything produces it:\n%s", in, stmt)
			}
		}
		outs := regexp.MustCompile(`\[[^\]]+\]$`).FindString(stmt)
		if strings.Count(stmt, "split=2") > 0 { // split makes two labels at once
			for _, l := range regexp.MustCompile(`split=2(\[[^\]]+\])(\[[^\]]+\])`).FindStringSubmatch(stmt)[1:] {
				if produced[l] {
					t.Errorf("label %s produced twice", l)
				}
				produced[l] = true
			}
		}
		if outs != "" {
			if produced[outs] && !strings.Contains(stmt, "split=2") {
				t.Errorf("label %s produced twice", outs)
			}
			produced[outs] = true
		}
	}
	for _, label := range []string{"[v0]", "[v1]", "[v2]", "[v3]", "[x1]", "[x2]", "[x3]", "[aout]"} {
		if !produced[label] {
			t.Errorf("label %s is never produced", label)
		}
	}
}

func TestBuildArgs_FadesStartWhenTheNextNarrationDoes(t *testing.T) {
	spec := specOf(t, []Scene{scene(KindPhoto, 1280, 1920, "1"), scene(KindPhoto, 1280, 1920, "2"), scene(KindPhoto, 1280, 1920, "3")}, []float64{4.5, 4.4, 4.65})
	args, _ := BuildArgs(spec)
	graph := graphOf(t, args)
	offsets := regexp.MustCompile(`xfade=transition=fade:duration=0\.400:offset=([0-9.]+)`).FindAllStringSubmatch(graph, -1)
	if len(offsets) != 2 {
		t.Fatalf("want 2 fades for 3 scenes, got %d in %s", len(offsets), graph)
	}
	for i, want := range []string{"4.850", "9.600"} {
		if offsets[i][1] != want {
			t.Errorf("fade %d at %s, want %s (the next scene's start)", i+1, offsets[i][1], want)
		}
	}
}

func TestBuildArgs_APictureIsHeldPastItsSpanSoTheFadeHasSomethingToFadeInto(t *testing.T) {
	spec := specOf(t, []Scene{scene(KindPhoto, 1280, 1920, "1"), scene(KindVideo, 1920, 1080, "2")}, []float64{4.5, 4.5})
	args, _ := BuildArgs(spec)
	joined := strings.Join(args, " ")
	// span 4.85 + fade 0.4 = 5.25 for both
	if !strings.Contains(joined, "-loop 1 -framerate 30 -t 5.250 -i /work/m1") {
		t.Errorf("the photo is not held for span + fade: %s", joined)
	}
	if !strings.Contains(joined, "-stream_loop -1 -t 5.250 -i /work/m2") {
		t.Errorf("the clip is not looped and cut to span + fade: %s", joined)
	}
}

func TestBuildArgs_OnlyStillsZoom_AndTheyZoomFromAnUpscaledCopy(t *testing.T) {
	args, _ := BuildArgs(specOf(t, []Scene{scene(KindPhoto, 1280, 1920, "1"), scene(KindVideo, 1080, 1920, "2")}, []float64{3, 3}))
	graph := graphOf(t, args)
	for _, stmt := range strings.Split(graph, ";") {
		if strings.HasSuffix(stmt, "[v0]") {
			if !strings.Contains(stmt, "scale=2160:3840") || !strings.Contains(stmt, "eval=frame") {
				t.Errorf("a still must zoom from an upscaled copy: %s", stmt)
			}
		}
		if strings.HasSuffix(stmt, "[v1]") && strings.Contains(stmt, "eval=frame") {
			t.Errorf("a clip must not be zoomed: %s", stmt)
		}
	}
}

func TestBuildArgs_ClipSoundIsNeverUsed(t *testing.T) {
	args, _ := BuildArgs(specOf(t, []Scene{scene(KindVideo, 1080, 1920, "1"), scene(KindVideo, 1080, 1920, "2")}, []float64{3, 3}))
	graph := graphOf(t, args)
	for _, stmt := range strings.Split(graph, ";") {
		if regexp.MustCompile(`^\[[01]:a\]`).MatchString(stmt) {
			t.Errorf("a clip's own audio feeds the graph: %s", stmt)
		}
	}
}

func TestBuildArgs_NarrationIsPlacedOnTheTimeline(t *testing.T) {
	spec := specOf(t, []Scene{scene(KindPhoto, 1280, 1920, "1"), scene(KindPhoto, 1280, 1920, "2")}, []float64{4, 5})
	args, _ := BuildArgs(spec)
	graph := graphOf(t, args)
	// narration inputs follow the pictures: inputs 2 and 3
	for i, want := range []string{"[2:a]aresample=48000,apad=whole_dur=4.350,atrim=0:4.350", "[3:a]aresample=48000,apad=whole_dur=5.350,atrim=0:5.350"} {
		if !strings.Contains(graph, want) {
			t.Errorf("scene %d: missing %q in %s", i+1, want, graph)
		}
	}
	if !strings.Contains(graph, "concat=n=2:v=0:a=1,apad=whole_dur=10.100") {
		t.Errorf("the sound is not padded to the video's length: %s", graph)
	}
	if !strings.Contains(strings.Join(args, " "), "-t 10.100 ") {
		t.Errorf("the output is not cut at the timeline's total")
	}
}

func TestBuildArgs_CaptionsAreBurnedInWithTheShortsStyle(t *testing.T) {
	spec := specOf(t, []Scene{scene(KindPhoto, 1280, 1920, "1"), scene(KindPhoto, 1280, 1920, "2")}, []float64{3, 3})
	spec.CaptionsPath = "/work/captions.srt"
	spec.Caption = models.CaptionStyle{Preset: "minimal", Position: "top"}
	args, err := BuildArgs(spec)
	if err != nil {
		t.Fatal(err)
	}
	graph := graphOf(t, args)
	if !strings.Contains(graph, "subtitles=/work/captions.srt:force_style='Alignment=8,FontSize=14") {
		t.Errorf("the SRT is not burned in with the minimal/top style: %s", graph)
	}
	if !strings.Contains(strings.Join(args, " "), "-map [vout] ") {
		t.Errorf("the captioned video is not what gets written")
	}
	spec.CaptionsPath = "/work/captions.ass"
	spec.Caption = models.CaptionStyle{Preset: "karaoke"}
	args, _ = BuildArgs(spec)
	if !strings.Contains(graphOf(t, args), "subtitles=/work/captions.ass[vout]") {
		t.Errorf("karaoke should burn the ASS as it is: %s", graphOf(t, args))
	}
}

func TestBuildArgs_WithoutCaptionsThereIsNoSubtitlesFilter(t *testing.T) {
	args, _ := BuildArgs(specOf(t, []Scene{scene(KindPhoto, 1280, 1920, "1"), scene(KindPhoto, 1280, 1920, "2")}, []float64{3, 3}))
	if strings.Contains(graphOf(t, args), "subtitles") {
		t.Error("a subtitles filter with no file would fail the render")
	}
}

func TestBuildArgs_ASingleSceneNeedsNoFade(t *testing.T) {
	args, err := BuildArgs(specOf(t, []Scene{scene(KindPhoto, 1280, 1920, "1")}, []float64{3}))
	if err != nil {
		t.Fatal(err)
	}
	graph := graphOf(t, args)
	if strings.Contains(graph, "xfade") {
		t.Errorf("one scene has nothing to fade to: %s", graph)
	}
	if !strings.Contains(strings.Join(args, " "), "-map [v0] ") {
		t.Errorf("the only scene's video is not mapped")
	}
}

func TestBuildArgs_RefusesInputsThatWouldMakeAnInvalidCommand(t *testing.T) {
	ok := []Scene{scene(KindPhoto, 1280, 1920, "1"), scene(KindPhoto, 1280, 1920, "2")}
	base := specOf(t, ok, []float64{3, 3})
	tests := map[string]func(*Spec){
		"no scenes":         func(s *Spec) { s.Scenes = nil },
		"timeline mismatch": func(s *Spec) { s.Timeline.Slots = s.Timeline.Slots[:1] },
		"no output":         func(s *Spec) { s.OutputPath = "" },
		"missing media":     func(s *Spec) { s.Scenes = []Scene{{NarrationPath: "n"}, ok[1]} },
		"missing narration": func(s *Spec) { s.Scenes = []Scene{{MediaPath: "m", Width: 1, Height: 1}, ok[1]} },
		"unknown size":      func(s *Spec) { s.Scenes = []Scene{{Kind: KindPhoto, MediaPath: "m", NarrationPath: "n"}, ok[1]} },
		"unknown kind": func(s *Spec) {
			s.Scenes = []Scene{{Kind: "gif", MediaPath: "m", NarrationPath: "n", Width: 1, Height: 1}, ok[1]}
		},
		"caption path with ':'": func(s *Spec) { s.CaptionsPath = "/work/a:b.srt" },
		"caption path quote":    func(s *Spec) { s.CaptionsPath = "/work/a'b.srt" },
		"caption path space":    func(s *Spec) { s.CaptionsPath = "/work/my captions.srt" },
	}
	for name, mutate := range tests {
		s := base
		s.Scenes = append([]Scene(nil), base.Scenes...)
		mutate(&s)
		if _, err := BuildArgs(s); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
