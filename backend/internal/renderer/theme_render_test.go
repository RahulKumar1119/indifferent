package renderer

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/slidegen"
)

// End-to-end check for issue #102: a three-question quiz in every shipped
// theme goes through the real native renderer and the real compositor, and the
// result is a playable 1920x1080 H.264 video whose frames carry that theme's
// colours. It needs ffmpeg and ffprobe on PATH and skips without them. It
// encodes six 1080p videos, so it also skips under -short.
func TestEveryThemeComposesAPlayableVideo(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes six 1080p videos; skipped with -short")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}

	questions := []slidegen.QuestionSlideData{
		sampleQuestion(1, "Which AWS service runs containers without managing servers?",
			[]string{"Amazon EC2", "AWS Fargate", "Amazon Lightsail", "AWS Batch"}, 1),
		sampleQuestion(2, "Which storage class is cheapest for rarely read archives?",
			[]string{"S3 Standard", "S3 Intelligent-Tiering", "S3 Glacier Deep Archive", "EBS gp3"}, 2),
		sampleQuestion(3, "What does IAM stand for?",
			[]string{"Identity and Access Management", "Internal Account Manager", "Instance Auth Module", "Inbound Access Map"}, 0),
	}

	// One 2 s tone, reused for every clip of every theme.
	tone := filepath.Join(t.TempDir(), "tone.mp3")
	run(t, filepath.Dir(tone), "ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:a", "libmp3lame", "-y", tone)

	// Theme must not change timing, so every theme's video is the same length.
	var firstDuration float64

	for _, name := range slidegen.ThemeNames() {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			theme := slidegen.ThemeFor(name)
			r := slidegen.NewNativeRenderer()

			var slides, audios []string
			for i, q := range questions {
				for _, f := range []struct{ file, tag string }{
					{"question.html", "question"},
					{"answer-reveal.html", "answer"},
				} {
					png, err := r.RenderTemplate(name+"/"+f.file, q)
					if err != nil {
						t.Fatalf("render q%d %s: %v", i, f.tag, err)
					}
					p := filepath.Join(dir, fmt.Sprintf("q%d_%s.png", i, f.tag))
					if err := os.WriteFile(p, png, 0o644); err != nil {
						t.Fatal(err)
					}
					slides = append(slides, p)
					audios = append(audios, tone)
				}
			}

			segments, outro, err := BuildSegments(slides, audios, len(questions))
			if err != nil {
				t.Fatal(err)
			}
			work := filepath.Join(dir, "work")
			if err := os.MkdirAll(work, 0o755); err != nil {
				t.Fatal(err)
			}
			video, err := NewCompositor(DefaultConfig(), work).ComposeVideo(context.Background(), segments, outro)
			if err != nil {
				t.Fatalf("compose: %v", err)
			}

			info := probe(t, video)
			if info.width != 1920 || info.height != 1080 {
				t.Errorf("size = %dx%d, want 1920x1080", info.width, info.height)
			}
			if info.codec != "h264" {
				t.Errorf("video codec = %q, want h264", info.codec)
			}
			// 3 questions x (2 s question + 2 s answer) is 12 s of audio. The
			// compositor's looped still image runs a little past its audio
			// (about 0.4 s per clip), so allow up to 16 s.
			if info.duration < 11.5 || info.duration > 16 {
				t.Errorf("duration = %.2fs, want between 11.5s and 16s", info.duration)
			}
			if firstDuration == 0 {
				firstDuration = info.duration
			} else if math.Abs(info.duration-firstDuration) > 0.1 {
				t.Errorf("duration = %.2fs, but another theme gave %.2fs", info.duration, firstDuration)
			}

			// The theme must survive encoding: sample a corner pixel (empty
			// background) on a question frame and on an answer frame. yuv420p
			// rounds colours, so allow a small tolerance.
			for _, f := range []struct {
				at   string
				want color.RGBA
				what string
			}{
				{"0.5", theme.QuestionBG, "first question"},
				{"2.5", theme.AnswerBG, "first answer"},
				{"-0.3", theme.AnswerBG, "last answer"}, // 0.3 s before the end
			} {
				got := frameCorner(t, dir, video, f.at)
				if !near(got, f.want, 6) {
					t.Errorf("%s frame corner = %v, want about %v", f.what, got, f.want)
				}
			}
		})
	}
}

func sampleQuestion(n int, text string, options []string, correct int) slidegen.QuestionSlideData {
	labels := []string{"A", "B", "C", "D"}
	var opts []models.Option
	for i, o := range options {
		opts = append(opts, models.Option{Label: labels[i], Text: o})
	}
	return slidegen.QuestionSlideData{
		QuestionNumber: n,
		TotalQuestions: 3,
		QuestionText:   text,
		Options:        opts,
		CorrectIndex:   correct,
		CorrectIndices: []int{correct},
	}
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

type videoInfo struct {
	width, height int
	codec         string
	duration      float64
}

func probe(t *testing.T, path string) videoInfo {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-show_format",
		"-of", "json", path).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var parsed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("ffprobe json: %v", err)
	}
	var info videoInfo
	hasAudio := false
	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			info.width, info.height, info.codec = s.Width, s.Height, s.CodecName
		case "audio":
			hasAudio = true
		}
	}
	if !hasAudio {
		t.Error("video has no audio stream")
	}
	info.duration, _ = strconv.ParseFloat(parsed.Format.Duration, 64)
	return info
}

// frameCorner decodes one frame at the given time and returns its bottom-right
// pixel, which is plain background on every slide.
func frameCorner(t *testing.T, dir, video, at string) color.RGBA {
	t.Helper()
	out := filepath.Join(dir, "frame_"+strings.NewReplacer(".", "_", "-", "m").Replace(at)+".png")
	seek := "-ss"
	if strings.HasPrefix(at, "-") {
		seek = "-sseof" // negative: counted back from the end
	}
	run(t, dir, "ffmpeg", seek, at, "-i", video, "-frames:v", "1", "-y", out)
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	return toRGBA(img, b.Max.X-20, b.Max.Y-30)
}

func toRGBA(img image.Image, x, y int) color.RGBA {
	r, g, b, _ := img.At(x, y).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
}

func near(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}
