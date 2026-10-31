package storyvideo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// progressStep is the smallest change in percent that is reported. A 90 second
// video would otherwise report hundreds of times, and each report is a
// database write.
const progressStep = 2

// FFmpeg runs FFmpeg for BuildArgs's output.
type FFmpeg struct {
	// Binary is the ffmpeg to run; empty means "ffmpeg" on the PATH.
	Binary string
}

// Run runs ffmpeg in directory dir (empty means the current one) with args, and
// reports progress as a percent from 0 to 99, in
// steps of progressStep. total is the length in seconds of the video being
// made, which progress is measured against. It returns when ffmpeg exits. On
// failure the error carries the tail of ffmpeg's diagnostics; it never carries
// the arguments, which hold file paths.
//
// If ctx ends, ffmpeg is killed and ctx's error is returned.
func (f FFmpeg) Run(ctx context.Context, dir string, args []string, total float64, onPercent func(int)) error {
	bin := f.Binary
	if bin == "" {
		bin = "ffmpeg"
	}
	full := append([]string{"-nostdin", "-hide_banner", "-loglevel", "error", "-progress", "pipe:1", "-nostats"}, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg could not start: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg could not start: %w", err)
	}
	last := -progressStep
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		pct := ProgressPercent(scanner.Text(), total)
		if pct >= 0 && pct-last >= progressStep && onPercent != nil {
			last = pct
			onPercent(pct)
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("ffmpeg failed: %w: %s", err, lastChars(stderr.String(), 600))
	}
	return nil
}

// ProgressPercent reads one line of ffmpeg's -progress output and returns the
// percent of total it shows, from 0 to 99, or -1 for a line that reports no
// time. (out_time_ms is in microseconds, despite its name.) 100 is left to the
// caller: it means the file is finished and checked, not that ffmpeg has
// reached the end of its input.
func ProgressPercent(line string, total float64) int {
	const prefix = "out_time_ms="
	if !strings.HasPrefix(line, prefix) || total <= 0 {
		return -1
	}
	us, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, prefix)), 10, 64)
	if err != nil || us < 0 {
		return -1
	}
	pct := int(float64(us) / 1e6 / total * 100)
	if pct > 99 {
		return 99
	}
	return pct
}

func lastChars(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
