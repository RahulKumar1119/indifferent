package render

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// Issue #287: the 9:16 crop window must fit every source shape. A fixed
// ih*9/16 width was wider than the frame for any source narrower than 9:16,
// which is most phone portrait video (1170x2532 is 19.5:9), and ffmpeg
// refused the render.

func TestFramingFilters(t *testing.T) {
	if want := "crop=w='min(iw,ih*9/16)':h='min(ih,iw*16/9)':x='(iw-ow)/2':y='(ih-oh)/2',scale=1080:1920,setsar=1,fps=30"; cropCaptionFilter != want {
		t.Errorf("centred crop\n got %s\nwant %s", cropCaptionFilter, want)
	}
	if got := framingFilter("464", false); got != "crop=w='min(iw,ih*9/16)':h='min(ih,iw*16/9)':x='min(464,iw-ow)':y='(ih-oh)/2',scale=1080:1920,setsar=1,fps=30" {
		t.Errorf("tracked crop: %s", got)
	}
	if got := trackedCropFilter(-8); !strings.Contains(got, "x='min(0,iw-ow)'") {
		t.Errorf("a negative offset must clamp to 0: %s", got)
	}
	// Only an integer offset may reach ffmpeg; anything else frames centred.
	for _, bad := range []string{"iw", "1;drawtext", "12.5", " 3"} {
		if got := framingFilter(bad, false); got != cropCaptionFilter {
			t.Errorf("framingFilter(%q) = %s, want the centred crop", bad, got)
		}
	}
	if framingFilter("464", true) != fitFillFilter {
		t.Error("fit wins over a tracked offset")
	}
}

func TestDisplayDimensionsAppliesRotation(t *testing.T) {
	cases := []struct {
		name, json string
		w, h       int
	}{
		{"no rotation", `{"streams":[{"width":1920,"height":1080}]}`, 1920, 1080},
		{"display matrix -90", `{"streams":[{"width":1920,"height":1080,"side_data_list":[{"side_data_type":"Display Matrix","rotation":-90}]}]}`, 1080, 1920},
		{"display matrix 90", `{"streams":[{"width":2532,"height":1170,"side_data_list":[{"rotation":90}]}]}`, 1170, 2532},
		{"display matrix 270", `{"streams":[{"width":1920,"height":1080,"side_data_list":[{"rotation":270}]}]}`, 1080, 1920},
		{"upside down keeps the shape", `{"streams":[{"width":1920,"height":1080,"side_data_list":[{"rotation":180}]}]}`, 1920, 1080},
		{"older rotate tag", `{"streams":[{"width":1920,"height":1080,"tags":{"rotate":"90"}}]}`, 1080, 1920},
		{"side data wins over the tag", `{"streams":[{"width":1920,"height":1080,"side_data_list":[{"rotation":0}],"tags":{"rotate":"90"}}]}`, 1920, 1080},
		{"side data without rotation", `{"streams":[{"width":1080,"height":1920,"side_data_list":[{"side_data_type":"other"}]}]}`, 1080, 1920},
		{"a whole turn and a quarter", `{"streams":[{"width":1920,"height":1080,"side_data_list":[{"rotation":-450}]}]}`, 1080, 1920},
	}
	for _, c := range cases {
		w, h, err := displayDimensions([]byte(c.json))
		if err != nil || w != c.w || h != c.h {
			t.Errorf("%s: got %dx%d (%v), want %dx%d", c.name, w, h, err, c.w, c.h)
		}
	}
	for _, bad := range []string{`{"streams":[]}`, `{"streams":[{"width":0,"height":1080}]}`, `nope`} {
		if _, _, err := displayDimensions([]byte(bad)); err == nil {
			t.Errorf("%s: expected an error", bad)
		}
	}
}

// shape is one source the framing must handle.
type shape struct {
	name     string
	w, h     int    // coded size
	codec    string // libx264 or libx265
	rotate   int    // display rotation written into the file, degrees
	dispW    int    // displayed size, after rotation
	dispH    int
	windowW  int // the 9:16 window ffmpeg must cut from the displayed frame
	windowH  int
	trackedX string
}

var shapes = []shape{
	{name: "iphone_portrait_hevc", w: 1170, h: 2532, codec: "libx265", dispW: 1170, dispH: 2532, windowW: 1170, windowH: 2080, trackedX: "0"},
	{name: "android_20x9", w: 1080, h: 2400, codec: "libx264", dispW: 1080, dispH: 2400, windowW: 1080, windowH: 1920, trackedX: "0"},
	{name: "exact_9x16", w: 1080, h: 1920, codec: "libx264", dispW: 1080, dispH: 1920, windowW: 1080, windowH: 1920, trackedX: "0"},
	{name: "small_9x16", w: 720, h: 1280, codec: "libx264", dispW: 720, dispH: 1280, windowW: 720, windowH: 1280, trackedX: "0"},
	{name: "landscape", w: 1920, h: 1080, codec: "libx264", dispW: 1920, dispH: 1080, windowW: 608, windowH: 1080, trackedX: "1312"},
	{name: "square", w: 1080, h: 1080, codec: "libx264", dispW: 1080, dispH: 1080, windowW: 608, windowH: 1080, trackedX: "472"},
	{name: "rotated_landscape_stored_portrait", w: 1920, h: 1080, codec: "libx264", rotate: 90, dispW: 1080, dispH: 1920, windowW: 1080, windowH: 1920, trackedX: "0"},
}

// TestFramingRunsThroughFFmpegForEveryShape makes a short real video of each
// shape (with audio) and runs the production argument builders through the
// system ffmpeg: the centred crop, the tracked crop, an offset far outside
// the frame, and a render with effects including the waveform. Every output
// must be a 1080x1920 clip with audio. It also cuts the bare crop window to
// check its size: for 1170x2532 that is the full width and 2080 rows, 226 off
// the top and the bottom. Skipped without ffmpeg; run it in the render image
// (see the PR) to test the ffmpeg production uses.
func TestFramingRunsThroughFFmpegForEveryShape(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	dir := t.TempDir()
	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			src := makeShapeSource(t, dir, s)

			// The probe sees the displayed size, so reframing works on the
			// frames ffmpeg filters.
			w, h, err := defaultProbeDimensions(src)
			if err != nil || w != s.dispW || h != s.dispH {
				t.Fatalf("probe: %dx%d (%v), want %dx%d as displayed", w, h, err, s.dispW, s.dispH)
			}
			if got := cropWindowWidth(h) >= w; got != (s.trackedX == "0") {
				t.Errorf("full-width window = %v for %dx%d", got, w, h)
			}

			// The window itself: never larger than the frame, exactly 9:16
			// (to the pixel ffmpeg rounds to), centred.
			cropOnly := strings.SplitN(cropCaptionFilter, ",scale=", 2)[0]
			png := filepath.Join(dir, s.name+"-window.png")
			runShapeCmd(t, "ffmpeg", "-v", "error", "-i", src, "-vf", cropOnly, "-frames:v", "1", "-y", png)
			if cw, ch := probeShapeSize(t, png); cw != s.windowW || ch != s.windowH {
				t.Errorf("crop window %dx%d, want %dx%d", cw, ch, s.windowW, s.windowH)
			} else if cw > s.dispW || ch > s.dispH {
				t.Errorf("crop window %dx%d exceeds the %dx%d frame", cw, ch, s.dispW, s.dispH)
			}

			r := &Renderer{Bucket: "b", WorkDir: dir}
			for _, v := range []struct {
				name  string
				cropX string
			}{{"centred", ""}, {"tracked", s.trackedX}, {"offset_past_the_edge", "100000"}} {
				out := filepath.Join(dir, s.name+"-"+v.name+".mp4")
				args := r.buildCropCaptionArgs(src, "", out, 0, 2, false, v.cropX, false, models.CaptionStyle{})
				runShapeCmd(t, "ffmpeg", append([]string{"-v", "error"}, args...)...)
				checkShapeClip(t, out)
			}

			// With effects, the same framing feeds the effects graph; the
			// waveform maps its audio label through the production builder.
			out := filepath.Join(dir, s.name+"-effects.mp4")
			e := &models.VideoEffects{
				Fade:     &models.FadeEffect{Enabled: true, InDuration: 0.3, OutDuration: 0.3},
				Waveform: &models.WaveformEffect{Enabled: true, Style: "bars", Color: "#FFFFFF", Position: "bottom", Height: 20},
			}
			args, err := effectArgs(effectJob{src: src, out: out, start: 0, end: 2, hasAudio: true, framing: framingFilter(s.trackedX, false), plan: planEffects(e, 2)})
			if err != nil {
				t.Fatal(err)
			}
			runShapeCmd(t, "ffmpeg", append([]string{"-v", "error"}, args...)...)
			checkShapeClip(t, out)
		})
	}
}

// makeShapeSource writes a 2 second test pattern with a tone in the given shape,
// and the display rotation when the shape has one.
func makeShapeSource(t *testing.T, dir string, s shape) string {
	t.Helper()
	coded := filepath.Join(dir, s.name+"-coded.mp4")
	runShapeCmd(t, "ffmpeg", "-v", "error",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=s=%dx%d:r=30:d=2", s.w, s.h),
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:v", s.codec, "-pix_fmt", "yuv420p", "-preset", "ultrafast", "-c:a", "aac", "-shortest", "-y", coded)
	if s.rotate == 0 {
		return coded
	}
	rotated := filepath.Join(dir, s.name+".mp4")
	// ffmpeg 6.x sets a display matrix with -display_rotation (counter-
	// clockwise degrees). Older ffmpeg (4.x) writes the "rotate" tag instead.
	if err := exec.Command("ffmpeg", "-v", "error", "-display_rotation", fmt.Sprint(s.rotate), "-i", coded, "-c", "copy", "-y", rotated).Run(); err != nil {
		runShapeCmd(t, "ffmpeg", "-v", "error", "-i", coded, "-c", "copy", "-metadata:s:v:0", fmt.Sprintf("rotate=%d", s.rotate), "-y", rotated)
	}
	return rotated
}

// checkShapeClip asserts a rendered clip is 1080x1920 with an audio stream.
func checkShapeClip(t *testing.T, path string) {
	t.Helper()
	if w, h := probeShapeSize(t, path); w != outputWidth || h != outputHeight {
		t.Errorf("%s is %dx%d, want %dx%d", filepath.Base(path), w, h, outputWidth, outputHeight)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "a", "-show_entries", "stream=codec_name", "-of", "csv=p=0", path).Output()
	if err != nil || strings.TrimSpace(string(out)) != "aac" {
		t.Errorf("%s has no AAC audio stream (%q, %v)", filepath.Base(path), out, err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() < 1000 {
		t.Errorf("%s is empty", filepath.Base(path))
	}
}

func probeShapeSize(t *testing.T, path string) (int, int) {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "json", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	var p struct {
		Streams []struct{ Width, Height int } `json:"streams"`
	}
	if err := json.Unmarshal(out, &p); err != nil || len(p.Streams) == 0 {
		t.Fatalf("ffprobe %s: %s", path, out)
	}
	return p.Streams[0].Width, p.Streams[0].Height
}

func runShapeCmd(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v\nargs: %q\n%s", name, err, args, out)
	}
}
