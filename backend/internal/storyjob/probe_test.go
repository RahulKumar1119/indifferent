package storyjob

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParseProbe_ReadsAClipAPhotoAndAudio(t *testing.T) {
	for name, tc := range map[string]struct {
		json string
		want MediaInfo
	}{
		"a portrait clip": {
			`{"streams":[{"codec_type":"video","width":1080,"height":1920,"duration":"12.500000","nb_frames":"375"},{"codec_type":"audio","duration":"12.5"}],"format":{"duration":"12.520000"}}`,
			MediaInfo{Width: 1080, Height: 1920, Duration: 12.5, HasVideo: true},
		},
		"a clip with the length only on the container": {
			`{"streams":[{"codec_type":"video","width":1920,"height":1080,"duration":"N/A","nb_frames":"N/A"}],"format":{"duration":"9.000000"}}`,
			MediaInfo{Width: 1920, Height: 1080, Duration: 9, HasVideo: true},
		},
		"a still photo is a one-frame video with no length": {
			`{"streams":[{"codec_type":"video","width":1280,"height":1920,"nb_frames":"1"}],"format":{"duration":"0.040000"}}`,
			MediaInfo{Width: 1280, Height: 1920, HasVideo: true},
		},
		"a webp photo": {
			`{"streams":[{"codec_type":"video","width":1600,"height":900,"nb_frames":"1","duration":"0.040000"}],"format":{}}`,
			MediaInfo{Width: 1600, Height: 900, HasVideo: true},
		},
		"narration audio": {
			`{"streams":[{"codec_type":"audio","duration":"4.388500"}],"format":{"duration":"4.388500"}}`,
			MediaInfo{Duration: 4.3885},
		},
		"audio with the length only on the container": {
			`{"streams":[{"codec_type":"audio"}],"format":{"duration":"2.664500"}}`,
			MediaInfo{Duration: 2.6645},
		},
		// ffprobe reads a text file named photo.jpg as a 0x0 video stream.
		"a stream with no size is not a picture": {
			`{"streams":[{"codec_type":"video","width":0,"height":0,"nb_frames":"1"}],"format":{"duration":"0.04"}}`,
			MediaInfo{Duration: 0.04},
		},
		"nothing recognisable": {`{"streams":[],"format":{}}`, MediaInfo{}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseProbe([]byte(tc.json))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseProbe_IgnoresLengthsThatAreNotUsable(t *testing.T) {
	for _, d := range []string{"-1", "NaN", "Inf", "abc", ""} {
		got, err := ParseProbe([]byte(fmt.Sprintf(`{"streams":[{"codec_type":"audio","duration":%q}],"format":{"duration":%q}}`, d, d)))
		if err != nil || got.Duration != 0 {
			t.Errorf("duration %q: got %+v, %v; want 0", d, got, err)
		}
	}
}

func TestParseProbe_RefusesOutputThatIsNotJSON(t *testing.T) {
	if _, err := ParseProbe([]byte("not json")); err == nil {
		t.Error("no error")
	}
}

// The real ffprobe on real files: the parser is only as good as ffprobe's
// actual output, which differs between versions and file types.
func TestFFprobe_RealFiles(t *testing.T) {
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s is not installed", bin)
		}
	}
	dir := t.TempDir()
	mk := func(name string, args ...string) string {
		p := filepath.Join(dir, name)
		full := append([]string{"-v", "error", "-y"}, args...)
		full = append(full, p)
		if out, err := exec.Command("ffmpeg", full...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %v: %v\n%s", full, err, out)
		}
		return p
	}
	jpg := mk("p.jpg", "-f", "lavfi", "-i", "color=c=red:s=1280x1920", "-frames:v", "1")
	png := mk("p.png", "-f", "lavfi", "-i", "color=c=red:s=1920x1080", "-frames:v", "1")
	webp := mk("p.webp", "-f", "lavfi", "-i", "color=c=red:s=1600x900", "-frames:v", "1")
	mp4 := mk("c.mp4", "-f", "lavfi", "-i", "color=c=blue:s=1080x1920:r=30:d=4", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	webm := mk("c.webm", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30:d=3", "-c:v", "libvpx", "-b:v", "1M")
	mp3 := mk("n.mp3", "-f", "lavfi", "-i", "sine=frequency=440:duration=2.5", "-c:a", "libmp3lame")
	junk := filepath.Join(dir, "junk.jpg")
	if err := writeFile(junk, []byte("<html>this is not a picture</html>")); err != nil {
		t.Fatal(err)
	}

	p := FFprobe{}
	ctx := context.Background()
	near := func(a, b float64) bool { d := a - b; return d > -0.15 && d < 0.15 }

	for name, tc := range map[string]struct {
		path string
		w, h int
		secs float64
		vid  bool
	}{
		"jpeg": {jpg, 1280, 1920, 0, true},
		"png":  {png, 1920, 1080, 0, true},
		"webp": {webp, 1600, 900, 0, true},
		"mp4":  {mp4, 1080, 1920, 4, true},
		"webm": {webm, 1280, 720, 3, true},
		"mp3":  {mp3, 0, 0, 2.5, false},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := p.Probe(ctx, tc.path)
			if err != nil {
				t.Skipf("this ffmpeg cannot make or read %s: %v", name, err)
			}
			if got.Width != tc.w || got.Height != tc.h || got.HasVideo != tc.vid || !near(got.Duration, tc.secs) {
				t.Errorf("got %+v, want %dx%d %.1fs video=%v", got, tc.w, tc.h, tc.secs, tc.vid)
			}
		})
	}

	t.Run("a web page named .jpg is an error, not a picture", func(t *testing.T) {
		if got, err := p.Probe(ctx, junk); err == nil && got.HasVideo {
			t.Errorf("a web page was read as a picture: %+v", got)
		}
	})
	t.Run("a missing file is an error", func(t *testing.T) {
		if _, err := p.Probe(ctx, filepath.Join(dir, "nope.mp4")); err == nil {
			t.Error("no error")
		}
	})
	t.Run("a cancelled context stops it", func(t *testing.T) {
		c, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := p.Probe(c, mp4); err == nil {
			t.Error("no error")
		}
	})
	t.Run("a binary that does not exist is an error", func(t *testing.T) {
		if _, err := (FFprobe{Binary: "/no/such/ffprobe"}).Probe(ctx, mp4); err == nil {
			t.Error("no error")
		}
	})
	_ = time.Second
}
