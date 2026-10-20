package thumbnail

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanFramesAdaptsToDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration float64
		wantMin  int
		wantMax  int
	}{
		{"very short", 4, 6, 8},
		{"half a minute", 30, 6, 6},
		{"two minutes", 120, 24, 24},
		{"ten minutes", 600, 48, 48},
		{"an hour", 3600, 48, 48},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PlanFrames(tt.duration)
			if p.Count < tt.wantMin || p.Count > tt.wantMax {
				t.Errorf("count = %d, want %d..%d", p.Count, tt.wantMin, tt.wantMax)
			}
			if p.Interval < minInterval {
				t.Errorf("interval %.2f is under the %.1fs minimum", p.Interval, minInterval)
			}
			if last := p.Time(p.Count - 1); last > tt.duration {
				t.Errorf("last sample at %.2fs is past the end (%.2fs)", last, tt.duration)
			}
		})
	}
}

func TestPlanFramesNeverExcessive(t *testing.T) {
	// The issue says not to extract an excessive number of frames: one per
	// second for a ten minute video would be 600.
	if p := PlanFrames(600); p.Count > maxFrames {
		t.Errorf("count %d exceeds cap %d", p.Count, maxFrames)
	}
	for _, bad := range []float64{0, -3} {
		if p := PlanFrames(bad); p.Count < 1 || p.Interval <= 0 {
			t.Errorf("PlanFrames(%v) = %+v, want a usable plan", bad, p)
		}
	}
}

func TestPlanFramesSamplesMidInterval(t *testing.T) {
	p := FramePlan{Interval: 10, Count: 3}
	for i, want := range []float64{5, 15, 25} {
		if got := p.Time(i); got != want {
			t.Errorf("Time(%d) = %v, want %v", i, got, want)
		}
	}
}

func TestExtractArgs(t *testing.T) {
	args := strings.Join(ExtractArgs("/in/src.mp4", "/out", FramePlan{Interval: 5, Count: 12}), " ")
	for _, want := range []string{"-ss 2.500", "-i /in/src.mp4", "fps=1/5.0000", "min(1920,iw)", "-frames:v 12", "/out/frame_%04d.jpg", "-an"} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q:\n%s", want, args)
		}
	}
}

const probeLandscape = `{"streams":[
 {"codec_type":"audio"},
 {"codec_type":"video","width":1920,"height":1080,"avg_frame_rate":"30000/1001","disposition":{"attached_pic":0}}
],"format":{"duration":"61.5"}}`

const probeRotated = `{"streams":[
 {"codec_type":"video","width":1920,"height":1080,"avg_frame_rate":"30/1","side_data_list":[{"rotation":-90}]}
],"format":{"duration":"20"}}`

const probeTagRotated = `{"streams":[
 {"codec_type":"video","width":1280,"height":720,"avg_frame_rate":"25/1","tags":{"rotate":"90"}}
],"format":{"duration":"20"}}`

const probeAudioWithCover = `{"streams":[
 {"codec_type":"audio"},
 {"codec_type":"video","width":500,"height":500,"avg_frame_rate":"0/0","disposition":{"attached_pic":1}}
],"format":{"duration":"180"}}`

func TestParseProbe(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		hasVideo  bool
		w, h      int
		duration  float64
		fpsApprox float64
	}{
		{"landscape", probeLandscape, true, 1920, 1080, 61.5, 29.97},
		{"rotated by side data", probeRotated, true, 1080, 1920, 20, 30},
		{"rotated by tag", probeTagRotated, true, 720, 1280, 20, 25},
		{"audio with cover art", probeAudioWithCover, false, 0, 0, 180, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := parseProbe([]byte(tt.json))
			if err != nil {
				t.Fatal(err)
			}
			if v.HasVideo != tt.hasVideo || v.Width != tt.w || v.Height != tt.h || v.Duration != tt.duration {
				t.Errorf("got %+v", v)
			}
			if d := v.FPS - tt.fpsApprox; d < -0.05 || d > 0.05 {
				t.Errorf("fps = %v, want about %v", v.FPS, tt.fpsApprox)
			}
		})
	}
	if v, _ := parseProbe([]byte(probeLandscape)); v.Aspect() < 1.77 || v.Aspect() > 1.78 {
		t.Errorf("aspect = %v", v.Aspect())
	}
}

func TestProbeFailureIsClassified(t *testing.T) {
	r := &fakeRunner{run: func(string, []string) ([]byte, error) { return nil, errors.New("moov atom not found") }}
	_, err := Probe(context.Background(), r, "x.mp4")
	if CodeOf(err) != CodeSourceUnreadable {
		t.Errorf("code = %q, want %q", CodeOf(err), CodeSourceUnreadable)
	}
	r = &fakeRunner{run: func(string, []string) ([]byte, error) { return []byte("not json"), nil }}
	if _, err := Probe(context.Background(), r, "x.mp4"); CodeOf(err) != CodeSourceUnreadable {
		t.Errorf("bad probe output should be SOURCE_UNREADABLE, got %q", CodeOf(err))
	}
}

func TestExtractOrdersFramesAndTimestamps(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{run: func(name string, args []string) ([]byte, error) {
		for i := 1; i <= 3; i++ {
			_ = os.WriteFile(filepath.Join(dir, "frame_000"+string(rune('0'+i))+".jpg"), []byte("x"), 0o644)
		}
		return nil, nil
	}}
	plan := FramePlan{Interval: 4, Count: 3}
	frames, err := Extract(context.Background(), r, "src.mp4", dir, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 {
		t.Fatalf("got %d frames", len(frames))
	}
	for i, f := range frames {
		if f.Time != plan.Time(i) {
			t.Errorf("frame %d time = %v, want %v", i, f.Time, plan.Time(i))
		}
	}
}

func TestExtractWithNoFramesIsNotSuitable(t *testing.T) {
	r := &fakeRunner{run: func(string, []string) ([]byte, error) { return nil, nil }}
	_, err := Extract(context.Background(), r, "src.mp4", t.TempDir(), FramePlan{Interval: 4, Count: 3})
	if CodeOf(err) != CodeNoSuitableFrames {
		t.Errorf("code = %q", CodeOf(err))
	}
	r = &fakeRunner{run: func(string, []string) ([]byte, error) { return nil, errors.New("ffmpeg crashed") }}
	if _, err := Extract(context.Background(), r, "src.mp4", t.TempDir(), FramePlan{Interval: 4, Count: 3}); CodeOf(err) != CodeSourceUnreadable {
		t.Errorf("ffmpeg failure code = %q", CodeOf(err))
	}
}

// With real ffmpeg: probe and extract a generated clip.
func TestProbeAndExtractWithRealFFmpeg(t *testing.T) {
	needTool(t, "ffmpeg", "ffprobe")
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.mp4")
	if out, err := (ExecRunner{}).Run(context.Background(), "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=1280x720:r=25:d=30",
		"-pix_fmt", "yuv420p", "-y", src); err != nil {
		t.Fatalf("make clip: %v %s", err, out)
	}
	ctx := context.Background()
	info, err := Probe(ctx, ExecRunner{}, src)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasVideo || info.Width != 1280 || info.Height != 720 || info.Duration < 29 || info.Duration > 31 || info.FPS < 24.9 || info.FPS > 25.1 {
		t.Errorf("probe = %+v", info)
	}
	plan := PlanFrames(info.Duration)
	frames, err := Extract(ctx, ExecRunner{}, src, filepath.Join(dir), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) < plan.Count-1 || len(frames) > plan.Count {
		t.Errorf("extracted %d frames, planned %d", len(frames), plan.Count)
	}
	if img, err := LoadImage(frames[0].Path); err != nil || img.Bounds().Dx() != 1280 {
		t.Errorf("first frame not decodable at full width: %v", err)
	}
}

func TestProbeAudioOnlyHasNoVideo(t *testing.T) {
	needTool(t, "ffmpeg", "ffprobe")
	src := filepath.Join(t.TempDir(), "a.mp3")
	if out, err := (ExecRunner{}).Run(context.Background(), "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=d=2", "-y", src); err != nil {
		t.Fatalf("make audio: %v %s", err, out)
	}
	info, err := Probe(context.Background(), ExecRunner{}, src)
	if err != nil {
		t.Fatal(err)
	}
	if info.HasVideo {
		t.Error("an audio file must not report a video track")
	}
}
