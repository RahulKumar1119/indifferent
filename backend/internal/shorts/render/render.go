package render

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
)

const (
	// outputWidth and outputHeight are the canonical 9:16 vertical dimensions
	// every rendered clip is normalized to (Requirement 4.2).
	outputWidth  = 1080
	outputHeight = 1920

	// forceStyle styles the burned-in captions: bottom-centered, white text
	// with an outlined box for readability without sound (Requirement 4.4).
	captionForceStyle = "Alignment=2,FontSize=18,PrimaryColour=&H00FFFFFF,BorderStyle=3,Outline=2"
)

// cropCaptionFilter is the 9:16 center-crop + scale video filter applied to a
// video source. It takes the widest full-height 9:16 column, centers it
// horizontally (center-crop, Requirement 4.3), and scales to the canonical
// 1080x1920 output (Requirement 4.2).
const cropCaptionFilter = "crop=w=ih*9/16:h=ih:x=(iw-ih*9/16)/2:y=0,scale=1080:1920,setsar=1"

// RenderInput is the full input for rendering one ranked clip.
type RenderInput struct {
	JobID         string
	UserID        string
	SourceKey     string
	TranscriptKey string
	Clip          models.Clip
	// ClipStart and ClipEnd are the segment's absolute timestamps (seconds)
	// within the source used to trim and to shift caption timings.
	ClipStart float64
	ClipEnd   float64
	// AudioOnly indicates the source has no video stream (MP3/WAV); when true
	// a solid 1080x1920 canvas is rendered instead of a center-crop.
	AudioOnly bool
}

// Renderer renders a single ranked segment into a 9:16 captioned clip and
// uploads it to S3. It mirrors the injectable RunCommand seam from
// internal/renderer/compositor.go for testability.
type Renderer struct {
	Storage    storage.StorageClient
	Bucket     string
	WorkDir    string
	RunCommand func(name string, args []string) error // injectable, defaults to exec
}

// NewRenderer constructs a Renderer with the default exec-backed RunCommand.
func NewRenderer(store storage.StorageClient, bucket, workDir string) *Renderer {
	r := &Renderer{
		Storage: store,
		Bucket:  bucket,
		WorkDir: workDir,
	}
	r.RunCommand = r.defaultRunCommand
	return r
}

// defaultRunCommand executes an FFmpeg command in the working directory.
func (r *Renderer) defaultRunCommand(name string, args []string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = r.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\noutput: %s", name, err, string(output))
	}
	return nil
}

// clipKey returns the deterministic S3 key for a rendered clip. Because the
// same input always maps to the same key, re-running is naturally idempotent
// (Requirement 5.5).
func clipKey(userID, jobID, clipID string) string {
	return fmt.Sprintf("shorts/%s/%s/clips/%s.mp4", userID, jobID, clipID)
}

// buildCropCaptionArgs builds the FFmpeg args for trimming to [start, end],
// applying the 9:16 center-crop + scale, and burning in the SRT captions.
// For audio-only sources a solid 1080x1920 canvas is generated instead of a
// crop. (Requirements 4.1, 4.2, 4.3, 4.4)
func (r *Renderer) buildCropCaptionArgs(src, subs, out string, start, end float64, audioOnly bool) []string {
	subtitlesFilter := fmt.Sprintf("subtitles=%s:force_style='%s'", subs, captionForceStyle)

	if audioOnly {
		// No video stream: synthesize a black 1080x1920 canvas for the clip
		// duration, mix in the trimmed audio, and burn captions onto it.
		duration := end - start
		return []string{
			"-f", "lavfi",
			"-i", fmt.Sprintf("color=c=black:s=%dx%d", outputWidth, outputHeight),
			"-ss", formatSeconds(start),
			"-to", formatSeconds(end),
			"-i", src,
			"-t", formatSeconds(duration),
			"-vf", subtitlesFilter,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-b:a", "128k",
			"-shortest",
			"-movflags", "+faststart",
			"-y", out,
		}
	}

	// Video source: -ss/-to before -i seek quickly; the caption file is
	// generated relative to the clip start so it aligns after the trim.
	return []string{
		"-ss", formatSeconds(start),
		"-to", formatSeconds(end),
		"-i", src,
		"-vf", fmt.Sprintf("%s,%s", cropCaptionFilter, subtitlesFilter),
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y", out,
	}
}

// formatSeconds renders a timestamp for FFmpeg's -ss/-to arguments.
func formatSeconds(s float64) string {
	return strconv.FormatFloat(s, 'f', 3, 64)
}

// alreadyRendered reports whether the clip already exists in S3, enabling an
// idempotent resume after a Spot interruption (Requirement 5.5).
func (r *Renderer) alreadyRendered(ctx context.Context, key string) (bool, error) {
	return r.Storage.HeadObject(ctx, r.Bucket, key)
}

// Render performs the full render task: idempotency check → download source +
// transcript → generate captions → FFmpeg crop+burn → upload the clip. It
// returns the S3 key of the rendered clip. When the clip already exists it is
// a no-op that returns the existing key. (Requirements 4.1, 4.5, 4.6, 5.5)
func (r *Renderer) Render(ctx context.Context, in RenderInput) (string, error) {
	key := clipKey(in.UserID, in.JobID, in.Clip.ClipID)

	// Idempotent resume: skip everything if the clip is already present.
	exists, err := r.alreadyRendered(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing clip %s: %w", key, err)
	}
	if exists {
		return key, nil
	}

	if err := os.MkdirAll(r.WorkDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	// Download the source media.
	srcData, err := r.Storage.GetObject(ctx, r.Bucket, in.SourceKey)
	if err != nil {
		return "", fmt.Errorf("failed to download source %s: %w", in.SourceKey, err)
	}
	srcPath := filepath.Join(r.WorkDir, "source"+safeExt(in.SourceKey))
	if err := os.WriteFile(srcPath, srcData, 0o644); err != nil {
		return "", fmt.Errorf("failed to write source: %w", err)
	}

	// Download and parse the transcript.
	transcriptData, err := r.Storage.GetObject(ctx, r.Bucket, in.TranscriptKey)
	if err != nil {
		return "", fmt.Errorf("failed to download transcript %s: %w", in.TranscriptKey, err)
	}
	transcript, err := parseTranscript(transcriptData)
	if err != nil {
		return "", fmt.Errorf("failed to parse transcript: %w", err)
	}

	// Filter words to the segment and build the SRT relative to the clip start.
	words := filterWords(transcript.Words, in.ClipStart, in.ClipEnd)
	srt := BuildSRT(words, in.ClipStart)
	srtPath := filepath.Join(r.WorkDir, in.Clip.ClipID+".srt")
	if err := os.WriteFile(srtPath, []byte(srt), 0o644); err != nil {
		return "", fmt.Errorf("failed to write captions: %w", err)
	}

	// Render with FFmpeg.
	outPath := filepath.Join(r.WorkDir, in.Clip.ClipID+".mp4")
	args := r.buildCropCaptionArgs(srcPath, srtPath, outPath, in.ClipStart, in.ClipEnd, in.AudioOnly)
	if err := r.RunCommand("ffmpeg", args); err != nil {
		return "", fmt.Errorf("ffmpeg render failed: %w", err)
	}

	// Upload the result.
	clipData, err := os.ReadFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to read rendered clip: %w", err)
	}
	if err := r.Storage.PutObject(ctx, r.Bucket, key, clipData, "video/mp4"); err != nil {
		return "", fmt.Errorf("failed to upload clip %s: %w", key, err)
	}

	return key, nil
}

// parseTranscript decodes the normalized transcript JSON stored in S3.
func parseTranscript(data []byte) (models.Transcript, error) {
	var t models.Transcript
	if err := json.Unmarshal(data, &t); err != nil {
		return models.Transcript{}, err
	}
	return t, nil
}

// filterWords returns the transcript words that overlap [start, end].
func filterWords(words []models.Word, start, end float64) []models.Word {
	var out []models.Word
	for _, w := range words {
		if w.End < start || w.Start > end {
			continue
		}
		out = append(out, w)
	}
	return out
}

// safeExt returns a sanitized file extension for the source key, defaulting to
// ".mp4" when none can be derived.
func safeExt(key string) string {
	ext := filepath.Ext(filepath.Base(key))
	for _, r := range ext {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.') {
			return ".mp4"
		}
	}
	if ext == "" || ext == "." {
		return ".mp4"
	}
	return ext
}
