package storyvideo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProgressPercent(t *testing.T) {
	for _, tc := range []struct {
		line  string
		total float64
		want  int
	}{
		{"out_time_ms=0", 40, 0},
		{"out_time_ms=10000000", 40, 25}, // microseconds: 10 s of 40 s
		{"out_time_ms=39990000", 40, 99},
		{"out_time_ms=40000000", 40, 99}, // 100 is for the caller, once the file is checked
		{"out_time_ms=99999999999", 40, 99},
		{"out_time_ms=-5", 40, -1},
		{"out_time_ms=N/A", 40, -1},
		{"out_time_ms=", 40, -1},
		{"frame=120", 40, -1},
		{"progress=end", 40, -1},
		{"out_time_ms=10000000", 0, -1},
		{"out_time_ms=10000000", -3, -1},
	} {
		if got := ProgressPercent(tc.line, tc.total); got != tc.want {
			t.Errorf("ProgressPercent(%q, %v) = %d, want %d", tc.line, tc.total, got, tc.want)
		}
	}
}

func needBinaries(t *testing.T) {
	t.Helper()
	for _, b := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("%s is not installed", b)
		}
	}
}

func TestFFmpegRun_ReportsProgressUpToTheEndAndNoMoreOftenThanTheStep(t *testing.T) {
	needBinaries(t)
	dir := t.TempDir()
	var seen []int
	err := FFmpeg{}.Run(context.Background(), dir,
		// Slow enough to run for several of ffmpeg's half-second progress intervals.
		[]string{"-f", "lavfi", "-i", "testsrc2=s=1280x720:r=30:d=20", "-c:v", "libx264", "-preset", "slow", "-pix_fmt", "yuv420p", "-y", "out.mp4"},
		20, func(p int) { seen = append(seen, p) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.mp4")); err != nil {
		t.Errorf("output is not in the directory ffmpeg was run in: %v", err)
	}
	if len(seen) < 3 {
		t.Fatalf("progress = %v, want several reports", seen)
	}
	for i, p := range seen {
		if p < 0 || p > 99 {
			t.Errorf("report %d = %d, outside 0 to 99", i, p)
		}
		if i > 0 && p-seen[i-1] < progressStep {
			t.Errorf("reports %d then %d are closer than %d", seen[i-1], p, progressStep)
		}
		if i > 0 && p <= seen[i-1] {
			t.Errorf("progress went backwards: %v", seen)
		}
	}
}

func TestFFmpegRun_AFailureSaysWhatFFmpegSaidAndNeverEchoesThePaths(t *testing.T) {
	needBinaries(t)
	secret := "/home/someone/very-private-directory/missing-input.mp4"
	err := FFmpeg{}.Run(context.Background(), t.TempDir(), []string{"-i", secret, "-y", "out.mp4"}, 5, nil)
	if err == nil {
		t.Fatal("no error for a missing input")
	}
	if !strings.Contains(err.Error(), "ffmpeg failed") {
		t.Errorf("err = %v", err)
	}
	// ffmpeg's own words are kept, they are the diagnosis; our argument list is not.
	if strings.Contains(err.Error(), "-y out.mp4") {
		t.Errorf("the error repeats the arguments: %v", err)
	}
}

func TestFFmpegRun_ACancelKillsFFmpegAndReturnsTheContextError(t *testing.T) {
	needBinaries(t)
	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	go func() { time.Sleep(400 * time.Millisecond); cancel() }()
	// Would take far longer than the test if it were not killed.
	err := FFmpeg{}.Run(ctx, t.TempDir(),
		[]string{"-f", "lavfi", "-i", "testsrc=s=1920x1080:r=30:d=600", "-c:v", "libx264", "-preset", "veryslow", "-y", "out.mp4"}, 600, nil)
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("took %v to stop", time.Since(start))
	}
}

func TestFFmpegRun_ABinaryThatDoesNotExistIsAnError(t *testing.T) {
	err := FFmpeg{Binary: "/no/such/ffmpeg"}.Run(context.Background(), "", nil, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "could not start") {
		t.Errorf("err = %v", err)
	}
}
