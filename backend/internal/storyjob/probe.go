package storyjob

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
)

// FFprobe is a Prober that runs ffprobe.
type FFprobe struct {
	// Binary is the ffprobe to run; empty means "ffprobe" on the PATH.
	Binary string
}

// Probe reads a file's picture size and length. A file with no picture stream
// (narration audio) reports HasVideo false and its length.
func (p FFprobe) Probe(ctx context.Context, path string) (MediaInfo, error) {
	bin := p.Binary
	if bin == "" {
		bin = "ffprobe"
	}
	cmd := exec.CommandContext(ctx, bin, "-v", "error",
		"-show_entries", "stream=codec_type,width,height,duration,nb_frames:format=duration",
		"-of", "json", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return MediaInfo{}, ctxErr
		}
		return MediaInfo{}, fmt.Errorf("ffprobe failed: %w: %s", err, trimTail(stderr.String(), 300))
	}
	return ParseProbe(out)
}

// ParseProbe reads ffprobe's JSON output.
func ParseProbe(out []byte) (MediaInfo, error) {
	var parsed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Duration  string `json:"duration"`
			NbFrames  string `json:"nb_frames"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return MediaInfo{}, fmt.Errorf("ffprobe output is unreadable: %w", err)
	}
	var info MediaInfo
	still := false // the picture stream is a single frame: an image, not a clip
	for _, s := range parsed.Streams {
		// A stream with no size is not a picture. ffprobe reports a text file
		// named photo.jpg as a 0x0 "video" stream, so the size is checked here
		// and no caller has to remember to.
		if s.CodecType != "video" || info.HasVideo || s.Width <= 0 || s.Height <= 0 {
			continue
		}
		info.HasVideo = true
		info.Width, info.Height = s.Width, s.Height
		still = s.NbFrames == "1"
		if d, ok := seconds(s.Duration); ok && !still {
			info.Duration = d
		}
	}
	if info.Duration == 0 && !still {
		if d, ok := seconds(parsed.Format.Duration); ok {
			info.Duration = d
		}
	}
	return info, nil
}

// seconds reads a duration from ffprobe, which reports "N/A" when unknown.
func seconds(v string) (float64, bool) {
	d, err := strconv.ParseFloat(v, 64)
	if err != nil || !finite(d) || d < 0 {
		return 0, false
	}
	return d, true
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func trimTail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
