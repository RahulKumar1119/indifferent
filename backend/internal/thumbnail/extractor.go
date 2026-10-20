package thumbnail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Runner runs an external program and returns its standard output. It is an
// interface so the probe and extract logic can be tested without FFmpeg.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner runs real processes.
type ExecRunner struct{}

// Run executes name with args. On failure the error includes stderr.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 800 {
			msg = msg[len(msg)-800:]
		}
		return nil, fmt.Errorf("%s failed: %w: %s", name, err, msg)
	}
	return stdout.Bytes(), nil
}

// VideoInfo is the metadata the issue asks for: duration, resolution, aspect
// ratio and frame rate. Width and Height are the displayed size, so a phone
// video stored sideways with a rotation flag reports its upright shape.
type VideoInfo struct {
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
	HasVideo bool    `json:"hasVideo"`
}

// Aspect is the displayed width divided by height, or 0 without video.
func (v VideoInfo) Aspect() float64 {
	if v.Height == 0 {
		return 0
	}
	return float64(v.Width) / float64(v.Height)
}

type probeOutput struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		Disposition  struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
		Tags struct {
			Rotate string `json:"rotate"`
		} `json:"tags"`
		SideDataList []struct {
			Rotation float64 `json:"rotation"`
		} `json:"side_data_list"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// Probe reads a video's metadata with ffprobe. Cover art inside an audio file
// (an attached picture) is not treated as a video track.
func Probe(ctx context.Context, run Runner, path string) (VideoInfo, error) {
	out, err := run.Run(ctx, "ffprobe", "-v", "error", "-show_streams", "-show_format", "-of", "json", path)
	if err != nil {
		return VideoInfo{}, Errorf(CodeSourceUnreadable, "We couldn't read this video file.", err)
	}
	return parseProbe(out)
}

func parseProbe(out []byte) (VideoInfo, error) {
	var p probeOutput
	if err := json.Unmarshal(out, &p); err != nil {
		return VideoInfo{}, Errorf(CodeSourceUnreadable, "We couldn't read this video file.", err)
	}
	info := VideoInfo{}
	info.Duration, _ = strconv.ParseFloat(p.Format.Duration, 64)
	for _, s := range p.Streams {
		if s.CodecType != "video" || s.Disposition.AttachedPic == 1 || s.Width <= 0 || s.Height <= 0 {
			continue
		}
		w, h := s.Width, s.Height
		rot := 0.0
		if len(s.SideDataList) > 0 {
			rot = s.SideDataList[0].Rotation
		} else if s.Tags.Rotate != "" {
			rot, _ = strconv.ParseFloat(s.Tags.Rotate, 64)
		}
		if r := int(math.Abs(math.Round(rot))) % 180; r == 90 {
			w, h = h, w
		}
		info.HasVideo, info.Width, info.Height = true, w, h
		info.FPS = parseRate(s.AvgFrameRate)
		break
	}
	return info, nil
}

func parseRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	n, _ := strconv.ParseFloat(num, 64)
	d, _ := strconv.ParseFloat(den, 64)
	if d == 0 {
		return 0
	}
	return n / d
}

// Frame limits. A short clip needs few frames; a ten-minute video would need
// hundreds at one per second, which costs time for no gain, so the count is
// bounded and the spacing stretches with the duration.
const (
	minFrames   = 6
	maxFrames   = 48
	targetEvery = 5.0 // aim for one frame per this many seconds
	minInterval = 0.5
)

// FramePlan says how to sample a video.
type FramePlan struct {
	Interval float64 // seconds between samples
	Count    int
}

// PlanFrames picks how many frames to pull for a video of the given length:
// about one per five seconds, never fewer than six or more than 48, and never
// closer together than half a second.
func PlanFrames(duration float64) FramePlan {
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return FramePlan{Interval: 1, Count: minFrames}
	}
	n := int(math.Round(duration / targetEvery))
	if n < minFrames {
		n = minFrames
	}
	if n > maxFrames {
		n = maxFrames
	}
	interval := duration / float64(n)
	if interval < minInterval {
		interval = minInterval
		n = int(duration / interval)
		if n < 1 {
			n = 1
		}
	}
	return FramePlan{Interval: interval, Count: n}
}

// Time returns the timestamp of the ith sample. Samples sit in the middle of
// each interval, which avoids the black frame many videos open with.
func (p FramePlan) Time(i int) float64 {
	return p.Interval/2 + float64(i)*p.Interval
}

// RawFrame is an extracted frame on disk.
type RawFrame struct {
	Index int
	Time  float64
	Path  string
}

// maxFrameWidth caps extracted frame width. 1920 keeps a 9:16 crop of a 1080p
// source sharp without storing 4K frames.
const maxFrameWidth = 1920

// ExtractArgs builds the ffmpeg arguments that pull plan.Count JPEG frames
// into dir, one every plan.Interval seconds starting at half an interval.
func ExtractArgs(src, dir string, plan FramePlan) []string {
	return []string{
		"-v", "error",
		"-ss", strconv.FormatFloat(plan.Interval/2, 'f', 3, 64),
		"-i", src,
		"-an", "-sn",
		"-vf", fmt.Sprintf("fps=1/%s,scale='min(%d,iw)':-2", strconv.FormatFloat(plan.Interval, 'f', 4, 64), maxFrameWidth),
		"-frames:v", strconv.Itoa(plan.Count),
		"-q:v", "3",
		"-y", filepath.Join(dir, "frame_%04d.jpg"),
	}
}

// Extract pulls candidate frames from src into dir. It returns the frames
// that were written, in time order. The count can fall short of the plan when
// the container's reported duration is optimistic; that is not an error.
func Extract(ctx context.Context, run Runner, src, dir string, plan FramePlan) ([]RawFrame, error) {
	if _, err := run.Run(ctx, "ffmpeg", ExtractArgs(src, dir, plan)...); err != nil {
		return nil, Errorf(CodeSourceUnreadable, "We couldn't read frames from this video.", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "frame_*.jpg"))
	if err != nil {
		return nil, Errorf(CodeSourceUnreadable, "We couldn't read frames from this video.", err)
	}
	sort.Strings(matches)
	frames := make([]RawFrame, 0, len(matches))
	for i, m := range matches {
		frames = append(frames, RawFrame{Index: i, Time: plan.Time(i), Path: m})
	}
	if len(frames) == 0 {
		return nil, Errorf(CodeNoSuitableFrames, "We couldn't find any frames in this video.", nil)
	}
	return frames, nil
}
