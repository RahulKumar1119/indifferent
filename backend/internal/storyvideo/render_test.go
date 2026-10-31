package storyvideo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
)

// These tests render real video with the FFmpeg on the machine and look at the
// result, so they prove what the argument tests cannot: that the command runs,
// and that the pictures, the fades, the captions and the sound come out where
// the timeline says. They are skipped where FFmpeg is not installed.

func needFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s is not installed", bin)
		}
	}
}

func run(t *testing.T, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, tail(stderr.String()))
	}
	return out
}

func tail(s string) string {
	if len(s) > 1500 {
		return s[len(s)-1500:]
	}
	return s
}

// story is a four-scene video of solid colours, one per scene, so the order and
// the framing of the pictures can be read straight off the pixels.
//
// Rendering a 1080x1920 video takes about twenty seconds, so the story is built
// once and each distinct render is made once and shared by the tests that look
// at it.
type story struct {
	dir    string
	scenes []Scene
	colors [][3]int
	narr   []float64
}

var (
	sharedDir   string
	sharedOnce  sync.Once
	sharedStory story
	sharedMu    sync.Mutex
	sharedOut   = map[string]string{}
)

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedDir != "" {
		os.RemoveAll(sharedDir)
	}
	os.Exit(code)
}

func makeStory(t *testing.T) story {
	t.Helper()
	needFFmpeg(t)
	if testing.Short() {
		t.Skip("renders real video; skipped with -short")
	}
	sharedOnce.Do(func() {
		dir, err := os.MkdirTemp("", "storyvideo-test-*")
		if err != nil {
			t.Fatal(err)
		}
		sharedDir = dir
		sharedStory = buildStory(t, dir)
	})
	if sharedStory.dir == "" {
		t.Fatal("the shared story could not be built")
	}
	return sharedStory
}

func buildStory(t *testing.T, dir string) story {
	t.Helper()
	colors := [][3]int{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 255, 0}}
	hex := []string{"0xFF0000", "0x00FF00", "0x0000FF", "0xFFFF00"}
	narr := []float64{3.0, 2.5, 3.2, 2.0}

	media := []struct {
		kind MediaKind
		file string
		w, h int
		make func(path, color string)
	}{
		{KindPhoto, "1.jpg", 1280, 1920, func(p, c string) {
			run(t, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c="+c+":s=1280x1920", "-frames:v", "1", "-y", p)
		}},
		// 2 seconds of landscape clip for a scene that lasts longer: it must loop.
		{KindVideo, "2.mp4", 1920, 1080, func(p, c string) {
			run(t, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c="+c+":s=1920x1080:r=25:d=2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", p)
		}},
		{KindPhoto, "3.jpg", 1920, 1280, func(p, c string) {
			run(t, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c="+c+":s=1920x1280", "-frames:v", "1", "-y", p)
		}},
		// 10 seconds of portrait clip for a 2 second scene: it must be cut.
		{KindVideo, "4.mp4", 1080, 1920, func(p, c string) {
			run(t, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c="+c+":s=1080x1920:r=30:d=10", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", p)
		}},
	}
	var scenes []Scene
	for i, m := range media {
		path := filepath.Join(dir, m.file)
		m.make(path, hex[i])
		nar := filepath.Join(dir, fmt.Sprintf("n%d.mp3", i+1))
		run(t, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%.2f", narr[i]), "-c:a", "libmp3lame", "-y", nar)
		scenes = append(scenes, Scene{Kind: m.kind, MediaPath: path, Width: m.w, Height: m.h, NarrationPath: nar})
	}
	return story{dir: dir, scenes: scenes, colors: colors, narr: narr}
}

// render renders the story with the captions file (empty for none), once.
func (s story) render(t *testing.T, captionsPath string, style models.CaptionStyle) (string, Timeline) {
	t.Helper()
	tl, err := BuildTimeline(s.narr)
	if err != nil {
		t.Fatal(err)
	}
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if out, ok := sharedOut[captionsPath]; ok {
		return out, tl
	}
	out := filepath.Join(s.dir, fmt.Sprintf("out%d.mp4", len(sharedOut)))
	args, err := BuildArgs(Spec{Scenes: s.scenes, Timeline: tl, CaptionsPath: captionsPath, Caption: style, OutputPath: out})
	if err != nil {
		t.Fatal(err)
	}
	run(t, "ffmpeg", append([]string{"-v", "error", "-nostdin"}, args...)...)
	sharedOut[captionsPath] = out
	return out, tl
}

// pixel returns the RGB of the frame at t seconds at (x, y).
func pixel(t *testing.T, video string, at float64, x, y int) [3]int {
	t.Helper()
	raw := run(t, "ffmpeg", "-v", "error", "-ss", fmt.Sprintf("%.3f", at), "-i", video, "-frames:v", "1",
		"-vf", fmt.Sprintf("format=rgb24,crop=1:1:%d:%d", x, y), "-f", "rawvideo", "-")
	if len(raw) != 3 {
		t.Fatalf("no frame at %.2fs (got %d bytes)", at, len(raw))
	}
	return [3]int{int(raw[0]), int(raw[1]), int(raw[2])}
}

func near(a, b [3]int, tol int) bool {
	for i := range a {
		if math.Abs(float64(a[i]-b[i])) > float64(tol) {
			return false
		}
	}
	return true
}

type probed struct {
	Streams []struct {
		CodecName  string `json:"codec_name"`
		CodecType  string `json:"codec_type"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
		PixFmt     string `json:"pix_fmt"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func probe(t *testing.T, video string) probed {
	t.Helper()
	var p probed
	out := run(t, "ffprobe", "-v", "error", "-show_entries", "stream=codec_name,codec_type,width,height,r_frame_rate,pix_fmt:format=duration", "-of", "json", video)
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRender_OutputIsA1080x1920ThirtyFpsH264VideoWithSound(t *testing.T) {
	s := makeStory(t)
	out, tl := s.render(t, "", models.CaptionStyle{})
	p := probe(t, out)
	var v, a bool
	for _, st := range p.Streams {
		switch st.CodecType {
		case "video":
			v = true
			if st.CodecName != "h264" || st.Width != 1080 || st.Height != 1920 || st.RFrameRate != "30/1" || st.PixFmt != "yuv420p" {
				t.Errorf("video stream = %+v", st)
			}
		case "audio":
			a = true
			if st.CodecName != "aac" {
				t.Errorf("audio codec = %s", st.CodecName)
			}
		}
	}
	if !v || !a {
		t.Errorf("video=%v audio=%v, want both", v, a)
	}
	got, _ := strconv.ParseFloat(p.Format.Duration, 64)
	if math.Abs(got-tl.Total) > 0.15 {
		t.Errorf("duration %.2fs, want %.2fs from the timeline", got, tl.Total)
	}
}

func TestRender_ScenesAppearInPlanOrder_WhateverTheShapeOfTheirPicture(t *testing.T) {
	s := makeStory(t)
	out, tl := s.render(t, "", models.CaptionStyle{})
	// The middle of each scene is well clear of both its fade in and its fade out.
	for i, slot := range tl.Slots {
		at := slot.Start + slot.Span/2
		// Sample the centre and a point in the blurred margin (for the scenes
		// that are fitted, not cropped): both are the scene's colour.
		for _, pt := range [][2]int{{540, 960}, {540, 80}} {
			got := pixel(t, out, at, pt[0], pt[1])
			if !near(got, s.colors[i], 40) {
				t.Errorf("scene %d at %.2fs, pixel %v = %v, want about %v", i+1, at, pt, got, s.colors[i])
			}
		}
	}
}

func TestRender_ScenesCrossFadeInsteadOfCutting(t *testing.T) {
	s := makeStory(t)
	out, tl := s.render(t, "", models.CaptionStyle{})
	for i := 1; i < len(tl.Slots); i++ {
		mid := tl.Slots[i].Start + TransitionSeconds/2
		got := pixel(t, out, mid, 540, 960)
		prev, next := s.colors[i-1], s.colors[i]
		// Halfway through a fade the pixel is neither the old colour nor the new.
		if near(got, prev, 40) || near(got, next, 40) {
			t.Errorf("fade into scene %d at %.2fs is a cut: pixel %v between %v and %v", i+1, mid, got, prev, next)
		}
	}
}

func TestRender_ALoopedShortClipAndACutLongClipFillTheirScenesExactly(t *testing.T) {
	s := makeStory(t)
	out, tl := s.render(t, "", models.CaptionStyle{})
	// Scene 2's clip is 2 s but its scene is 2.85 s: the last second must still
	// be the clip, not black. Scene 4's clip is 10 s for a 2.35 s scene: the
	// video must end with the scene, not run on.
	late := tl.Slots[1].Start + tl.Slots[1].Span - 0.5
	if got := pixel(t, out, late, 540, 960); !near(got, s.colors[1], 40) {
		t.Errorf("end of the looped clip scene = %v, want about %v", got, s.colors[1])
	}
	p := probe(t, out)
	d, _ := strconv.ParseFloat(p.Format.Duration, 64)
	if d > tl.Total+0.15 {
		t.Errorf("the 10 s clip ran the video on to %.2fs, want %.2fs", d, tl.Total)
	}
}

func TestRender_NarrationIsHeardInEachSceneAndAPauseSeparatesThem(t *testing.T) {
	s := makeStory(t)
	out, tl := s.render(t, "", models.CaptionStyle{})
	level := func(from, dur float64) float64 {
		cmd := exec.Command("ffmpeg", "-v", "info", "-ss", fmt.Sprintf("%.3f", from), "-t", fmt.Sprintf("%.3f", dur), "-i", out, "-af", "volumedetect", "-vn", "-f", "null", "-")
		b, _ := cmd.CombinedOutput()
		for _, line := range strings.Split(string(b), "\n") {
			if i := strings.Index(line, "mean_volume:"); i >= 0 {
				v, _ := strconv.ParseFloat(strings.Fields(line[i+len("mean_volume:"):])[0], 64)
				return v
			}
		}
		t.Fatalf("no volume reading at %.2f\n%s", from, tail(string(b)))
		return 0
	}
	for i, slot := range tl.Slots {
		if v := level(slot.Start+0.3, s.narr[i]-0.6); v < -35 {
			t.Errorf("scene %d narration is silent in the video: mean %.1f dB", i+1, v)
		}
		// The pause: after the narration ends and before the next scene's begins.
		if v := level(slot.Start+s.narr[i]+0.08, PauseSeconds-0.16); v > -60 {
			t.Errorf("scene %d: no pause after the narration (mean %.1f dB)", i+1, v)
		}
	}
}

// captionedBottom reports how many pixels of the bottom third differ between
// two renders of the same story, and how many in the top third.
func differing(t *testing.T, a, b string, at float64) (bottom, top int) {
	t.Helper()
	cmp := func(y int) int {
		raw := func(v string) []byte {
			return run(t, "ffmpeg", "-v", "error", "-ss", fmt.Sprintf("%.3f", at), "-i", v, "-frames:v", "1",
				"-vf", fmt.Sprintf("crop=1080:640:0:%d,format=gray", y), "-f", "rawvideo", "-")
		}
		ra, rb := raw(a), raw(b)
		n := 0
		for i := range ra {
			if math.Abs(float64(int(ra[i])-int(rb[i]))) > 40 {
				n++
			}
		}
		return n
	}
	return cmp(1280), cmp(0)
}

func TestRender_CaptionsAreBurnedInAtTheirWordsAndOnlyThere(t *testing.T) {
	s := makeStory(t)
	if err := render.EnsureCaptionFonts(); err != nil {
		t.Skip(err)
	}
	tl, _ := BuildTimeline(s.narr)
	scenes := []SceneWords{
		{Words: []models.Word{{Text: "The", Start: 0.2, End: 0.7}, {Text: "lighthouse", Start: 0.7, End: 1.6}, {Text: "stood", Start: 1.6, End: 2.4}}, AudioSeconds: 3},
		{Words: []models.Word{{Text: "Every", Start: 0.2, End: 1.0}, {Text: "night", Start: 1.0, End: 2.4}}, AudioSeconds: 2.5},
		{Words: []models.Word{{Text: "Storms", Start: 0.2, End: 1.4}, {Text: "came", Start: 1.4, End: 3.0}}, AudioSeconds: 3.2},
		{Words: []models.Word{{Text: "One", Start: 0.2, End: 0.9}, {Text: "evening", Start: 0.9, End: 1.9}}, AudioSeconds: 2},
	}
	placed, err := PlaceWords(scenes, tl)
	if err != nil {
		t.Fatal(err)
	}
	style := models.CaptionStyle{Preset: "bold"}
	doc, ext := render.BuildCaptions(placed.Words, style, placed.BreakBefore)
	path := filepath.Join(s.dir, "captions"+ext)
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	plain, _ := s.render(t, "", style)
	captioned, _ := s.render(t, path, style)

	// While the second scene's first word is spoken there is text at the bottom
	// and nothing at the top.
	during := tl.Slots[1].Start + 0.6
	bottom, top := differing(t, plain, captioned, during)
	if bottom < 2000 {
		t.Errorf("no caption at %.2fs: only %d bottom pixels differ", during, bottom)
	}
	if top != 0 {
		t.Errorf("%d pixels at the top changed: the caption is in the wrong place", top)
	}
	// Between the last word of a scene and the next scene's first word there is
	// no caption: the pause is clear.
	gap := tl.Slots[0].Start + 2.4 + 0.15
	if b, _ := differing(t, plain, captioned, gap); b != 0 {
		t.Errorf("a caption is still on screen %.2fs into the pause (%d pixels)", gap, b)
	}
}

func TestRender_HindiCaptionsDrawGlyphsNotBoxes(t *testing.T) {
	s := makeStory(t)
	if err := render.EnsureCaptionFonts(); err != nil {
		t.Skip(err)
	}
	if out, _ := exec.Command("fc-list", ":lang=hi", "family").Output(); len(bytes.TrimSpace(out)) == 0 {
		t.Skip("no Devanagari font installed")
	}
	tl, _ := BuildTimeline(s.narr)
	hi := SceneWords{Words: []models.Word{{Text: "लाइटहाउस", Start: 0.2, End: 1.2}, {Text: "अकेला", Start: 1.2, End: 2.4}}, AudioSeconds: 3}
	scenes := []SceneWords{hi, hi, hi, hi}
	placed, _ := PlaceWords(scenes, tl)
	style := models.CaptionStyle{Preset: "karaoke"}
	doc, ext := render.BuildCaptions(placed.Words, style, placed.BreakBefore)
	path := filepath.Join(s.dir, "hi"+ext)
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	plain, _ := s.render(t, "", style)
	captioned, _ := s.render(t, path, style)
	if bottom, _ := differing(t, plain, captioned, 0.9); bottom < 2000 {
		t.Errorf("Hindi caption drew almost nothing (%d pixels)", bottom)
	}
}

// marked makes a picture of the given size that is green with a red band down
// its left 6%. A band that reaches the frame's left edge means the picture was
// shown whole; a frame with no red there means its sides were cut off.
func marked(t *testing.T, path string, w, h int) {
	t.Helper()
	run(t, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("color=c=0x00FF00:s=%dx%d", w, h),
		"-vf", fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=%d:color=0xFF0000:t=fill", w*6/100, h), "-frames:v", "1", "-y", path)
}

func TestRender_LandscapeIsShownWholeAndPortraitIsCropped(t *testing.T) {
	needFFmpeg(t)
	if testing.Short() {
		t.Skip("renders real video")
	}
	dir := t.TempDir()
	land, port := filepath.Join(dir, "land.png"), filepath.Join(dir, "port.png")
	marked(t, land, 1920, 1080)
	marked(t, port, 1280, 1920)
	var scenes []Scene
	for i, m := range []struct {
		path string
		w, h int
	}{{land, 1920, 1080}, {port, 1280, 1920}} {
		nar := filepath.Join(dir, fmt.Sprintf("n%d.mp3", i))
		run(t, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=1.5", "-c:a", "libmp3lame", "-y", nar)
		scenes = append(scenes, Scene{Kind: KindPhoto, MediaPath: m.path, Width: m.w, Height: m.h, NarrationPath: nar})
	}
	tl, err := BuildTimeline([]float64{1.5, 1.5})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.mp4")
	args, err := BuildArgs(Spec{Scenes: scenes, Timeline: tl, OutputPath: out})
	if err != nil {
		t.Fatal(err)
	}
	run(t, "ffmpeg", append([]string{"-v", "error", "-nostdin"}, args...)...)

	red := [3]int{255, 0, 0}
	green := [3]int{0, 255, 0}
	// The landscape photo, fitted: its width fills the frame, so its red band
	// is at the frame's left edge, in the middle of the frame vertically.
	if got := pixel(t, out, 0.9, 8, 960); !near(got, red, 60) {
		t.Errorf("landscape: the left edge = %v, want its red band (the picture was cropped)", got)
	}
	// ...and the centre of it is the picture, not the blurred background.
	if got := pixel(t, out, 0.9, 540, 960); !near(got, green, 40) {
		t.Errorf("landscape: the centre = %v, want green", got)
	}
	// The portrait photo, cropped to fill: 100 px are cut from each side of
	// its 1280, so the 77 px red band is gone entirely.
	at := tl.Slots[1].Start + 0.9
	if got := pixel(t, out, at, 8, 960); near(got, red, 60) {
		t.Errorf("portrait: the left edge = %v, the band should have been cropped away", got)
	}
	if got := pixel(t, out, at, 540, 960); !near(got, green, 40) {
		t.Errorf("portrait: the centre = %v, want green", got)
	}
}
