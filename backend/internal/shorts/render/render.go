package render

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
)

const (
	// outputWidth and outputHeight are the canonical 9:16 vertical dimensions
	// every rendered clip is normalized to (Requirement 4.2).
	outputWidth  = 1080
	outputHeight = 1920
)

// cropCaptionFilter is the 9:16 center-crop + scale video filter applied to a
// video source. It takes the widest full-height 9:16 column, centers it
// horizontally (center-crop, Requirement 4.3), scales to the canonical
// 1080x1920 output (Requirement 4.2), and normalizes the frame rate to 30fps
// so the clip always uses a YouTube Shorts-supported frame rate regardless of
// the source (phone VFR, 120fps slow-mo, etc.).
const cropCaptionFilter = "crop=w=ih*9/16:h=ih:x=(iw-ih*9/16)/2:y=0,scale=1080:1920,setsar=1,fps=30"

// fitFillFilter scales the whole source frame to fit inside 1080x1920 over a
// blurred copy of itself as background. Nothing is ever cut: full-width
// burned-in text, cartoons, and screen recordings stay fully visible. Used
// for segments where no subject face is found (where a center-crop would
// blindly slice content). gblur needs no external library.
//
// The background is blurred at a quarter of the output size (270x480, sigma 10)
// and then scaled up to 1080x1920. A Gaussian blur this strong keeps no fine
// detail, so blurring a full-size copy (sigma 40) spent most of the filter's
// time on pixels that were about to be smoothed away. Sigma 10 at quarter size
// is the same blur radius as sigma 40 at full size. Measured on the same input
// this encodes about 40 to 50% faster with SSIM 0.9975 against the old output
// (issue #137).
const fitFillFilter = "split=2[blur][main];[blur]scale=270:480:force_original_aspect_ratio=increase,crop=270:480,gblur=sigma=10,scale=1080:1920[bg];[main]scale=1080:1920:force_original_aspect_ratio=decrease[fg];[bg][fg]overlay=(W-w)/2:(H-h)/2,setsar=1,fps=30"

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
	// Caption is the job's reusable caption style (zero value = legacy bold).
	Caption models.CaptionStyle
	// Edit, when non-nil, overrides the trim window and caption words with
	// the user's saved correction (timings preserved as stored).
	Edit *models.ClipEdit
}

// Renderer renders a single ranked segment into a 9:16 captioned clip and
// uploads it to S3. It mirrors the injectable RunCommand seam from
// internal/renderer/compositor.go for testability.
type Renderer struct {
	Storage    storage.StorageClient
	Bucket     string
	WorkDir    string
	RunCommand func(name string, args []string) error // injectable, defaults to exec
	// ProbeDimensions reports the source video dimensions. Injectable;
	// defaults to ffprobe. Only used for landscape reframing.
	ProbeDimensions func(path string) (w, h int, err error)
	// ExtractFrames samples JPEG frames across [start, end] for subject
	// detection. Injectable; defaults to ffmpeg. Only used for reframing.
	ExtractFrames func(src string, start, end float64) ([][]byte, error)
	// Detector finds faces for subject-aware reframing. When nil (the
	// default), the legacy centered crop is used.
	Detector FaceDetector
	// FetchLogo returns the project's brand logo bytes for the clip being
	// rendered, or (nil, nil) when unbranded. Injectable; defaults to nil
	// (no overlay). Failures must fall back to unbranded, never fail render.
	FetchLogo func(ctx context.Context) ([]byte, error)
	// Table is the DynamoDB table holding SHORTS# job records for progress
	// writes. Empty disables progress reporting.
	Table string
	// Dynamo performs progress writes. Injectable for tests.
	Dynamo DynamoUpdater
	// ProgressReporter receives throttled 0-99 progress reports during the
	// render. Defaults to a DynamoDB update when Table+Dynamo are set.
	ProgressReporter func(ctx context.Context, in RenderInput, percent int) error
	// RunCommandProgress runs ffmpeg while streaming -progress output.
	// Injectable; defaults to a real exec parsing out_time_ms.
	RunCommandProgress func(name string, args []string, duration float64, onProgress func(int)) error
}

// DynamoUpdater is the subset of the DynamoDB API used for progress writes.
type DynamoUpdater interface {
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// logoOverlayWidth is the on-screen width of the brand logo on a 1080x1920
// clip (top-right, 24px margin). Height scales to preserve aspect ratio.
const logoOverlayWidth = 200

// EnsureCaptionFonts verifies the runtime can shape burned-in captions.
// libass silently renders nothing when fontconfig has no usable fonts, so
// this preflight turns that silent wrong output into a loud, retryable
// render error.
func EnsureCaptionFonts() error {
	out, err := exec.Command("fc-list", "--format=%{family}\n").Output()
	if err != nil {
		return fmt.Errorf("caption fonts unavailable (fc-list failed: %v): install fontconfig fonts in the render image", err)
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return fmt.Errorf("caption fonts unavailable: fontconfig lists no font families")
	}
	return nil
}

// buildLogoOverlayArgs burns a brand logo onto a finished clip (top-right
// corner) without re-encoding audio.
func buildLogoOverlayArgs(videoPath, logoPath, outputPath string) []string {
	filter := fmt.Sprintf("[1]scale=%d:-1[logo];[0][logo]overlay=W-w-24:24:format=auto,format=yuv420p", logoOverlayWidth)
	return []string{
		"-i", videoPath,
		"-i", logoPath,
		"-filter_complex", filter,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-c:a", "copy",
		"-movflags", "+faststart",
		"-r", "30",
		"-y", outputPath,
	}
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

// ClipKey returns the deterministic S3 key for a rendered clip. Because the
// same input always maps to the same key, re-running is naturally idempotent
// (Requirement 5.5). Exported so the status updater can record the same key
// the renderer writes without duplicating the pattern.
func ClipKey(userID, jobID, clipID string) string {
	return fmt.Sprintf("shorts/%s/%s/clips/%s.mp4", userID, jobID, clipID)
}

// buildCropCaptionArgs builds the FFmpeg args for trimming to [start, end],
// applying the 9:16 center-crop + scale, and burning in the SRT captions.
// For audio-only sources a solid 1080x1920 canvas is generated instead of a
// crop. (Requirements 4.1, 4.2, 4.3, 4.4)
//
// cropX selects the horizontal position of the 9:16 window: "" keeps the
// legacy centered expression, otherwise it must be a non-negative even pixel
// offset (as produced by ComputeCropX for subject tracking). When fit is
// true the whole frame is scaled to fit over a blurred background instead of
// cropping (cropX is ignored).
func (r *Renderer) buildCropCaptionArgs(src, subs, out string, start, end float64, audioOnly bool, cropX string, fit bool, style models.CaptionStyle) []string {
	// An empty subs path means "no captions": the subtitles filter rejects
	// empty files, so it is omitted entirely (identical picture, no failure).
	// Karaoke styles burn ASS (word sweep) with embedded styling; all others
	// use SRT with a force_style override.
	subsFilter := ""
	if subs != "" {
		if isKaraokeStyle(style) {
			subsFilter = fmt.Sprintf(",subtitles=%s", subs)
		} else {
			subsFilter = fmt.Sprintf(",subtitles=%s:force_style='%s'", subs, ResolveForceStyle(style))
		}
	}

	if audioOnly {
		// No video stream: synthesize a black 1080x1920 canvas for the clip
		// duration, mix in the trimmed audio, and burn captions onto it.
		// fps=30 keeps the canvas at a Shorts-supported frame rate.
		duration := end - start
		args := []string{
			"-f", "lavfi",
			"-i", fmt.Sprintf("color=c=black:s=%dx%d:r=30", outputWidth, outputHeight),
			"-ss", formatSeconds(start),
			"-to", formatSeconds(end),
			"-i", src,
			"-t", formatSeconds(duration),
		}
		if subsFilter != "" {
			args = append(args, "-vf", subsFilter[1:])
		}
		return append(args,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-pix_fmt", "yuv420p",
			"-r", "30",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-shortest",
			"-movflags", "+faststart",
			"-y", out,
		)
	}

	// Video source: -ss/-to before -i seek quickly; the caption file is
	// generated relative to the clip start so it aligns after the trim.
	videoFilter := cropCaptionFilter
	switch {
	case fit:
		videoFilter = fitFillFilter
	case cropX != "":
		videoFilter = fmt.Sprintf("crop=w=ih*9/16:h=ih:x=%s:y=0,scale=1080:1920,setsar=1,fps=30", cropX)
	}
	return []string{
		"-ss", formatSeconds(start),
		"-to", formatSeconds(end),
		"-i", src,
		"-vf", fmt.Sprintf("%s%s", videoFilter, subsFilter),
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		"-r", "30",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "48000",
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

// smartReframe decides how the 9:16 window is placed for a landscape source.
// It returns either a tracked crop offset, fit=true to scale the whole frame
// over a blurred background (no subject found: cartoons, text cards, screen
// recordings where any crop would slice content), or the legacy centered crop
// ("", false) when reframing is unavailable or fails. Framing must never fail
// a render, so every error path falls back to center.
func (r *Renderer) smartReframe(ctx context.Context, srcPath string, start, end float64) (cropX string, fit bool) {
	if r.Detector == nil {
		return "", false
	}
	var w, h int
	var err error
	if r.ProbeDimensions != nil {
		w, h, err = r.ProbeDimensions(srcPath)
	} else {
		w, h, err = defaultProbeDimensions(srcPath)
	}
	if err != nil {
		log.Printf("reframe: probe failed (%v), using centered crop", err)
		return "", false
	}
	if cropWindowWidth(h) >= w {
		// Window covers the full width (vertical/square-narrow source):
		// pin to the left edge rather than evaluating a negative center.
		return "0", false
	}
	extract := r.ExtractFrames
	if extract == nil {
		extract = r.defaultExtractFrames
	}
	frames, err := extract(srcPath, start, end)
	if err != nil || len(frames) == 0 {
		log.Printf("reframe: frame sampling failed (%v), using centered crop", err)
		return "", false
	}
	dets := make([][]FaceBox, 0, len(frames))
	for _, img := range frames {
		boxes, err := r.Detector.DetectFaces(ctx, img)
		if err != nil {
			log.Printf("reframe: face detection failed (%v), using centered crop", err)
			return "", false
		}
		dets = append(dets, boxes)
	}
	x, ok := ComputeCropX(dets, w, h)
	if !ok {
		log.Printf("reframe: no subject found, using blur-fill fit")
		return "", true
	}
	return strconv.Itoa(x), false
}

// defaultProbeDimensions reports the first video stream's width and height
// via ffprobe.
func defaultProbeDimensions(path string) (int, int, error) {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "json", path).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe failed: %w", err)
	}
	var parsed struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, 0, fmt.Errorf("ffprobe output unparsable: %w", err)
	}
	if len(parsed.Streams) == 0 || parsed.Streams[0].Width <= 0 || parsed.Streams[0].Height <= 0 {
		return 0, 0, fmt.Errorf("ffprobe found no video dimensions")
	}
	return parsed.Streams[0].Width, parsed.Streams[0].Height, nil
}

// utcNow renders the current UTC time for DynamoDB timestamp attributes.
func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// progressReportStep is the minimum percent delta between DynamoDB progress
// writes: a 30s clip reports ~6 times instead of hundreds.
const progressReportStep = 5

// progressPercent converts an ffmpeg out_time_ms value to 0-99 percent of the
// clip duration. It returns -1 for lines carrying no timestamp.
func progressPercent(line string, duration float64) int {
	const prefix = "out_time_ms="
	if !strings.HasPrefix(line, prefix) || duration <= 0 {
		return -1
	}
	ms, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, prefix)), 10, 64)
	if err != nil || ms < 0 {
		return -1
	}
	pct := int(float64(ms) / 1e6 / duration * 100)
	if pct < 0 {
		return 0
	}
	if pct > 99 {
		return 99
	}
	return pct
}

// defaultRunCommandProgress runs ffmpeg with -progress pipe:1 and streams
// out_time_ms timestamps to onProgress (throttled to 5-point steps).
// FFmpeg diagnostics go to the task log; only progress lines are parsed.
func defaultRunCommandProgress(name string, args []string, duration float64, onProgress func(int)) error {
	full := append([]string{"-progress", "pipe:1", "-nostats"}, args...)
	cmd := exec.Command(name, full...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe failed: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}
	last := -1
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if pct := progressPercent(scanner.Text(), duration); pct >= 0 && pct-last >= progressReportStep {
			last = pct
			onProgress(pct)
		}
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

// reportProgress persists a render percent for the job. Without a reporter
// and table configured it is a no-op; failures only log (progress must never
// fail a render). ffmpeg reports 0-99 within the render stage, which starts
// at baseline 78: values are offset into 78-99 so progress never regresses
// (100 is reserved for the completed transition).
func (r *Renderer) reportProgress(ctx context.Context, in RenderInput, percent int) {
	percent = 78 + percent*21/100
	if percent < 78 {
		percent = 78
	}
	if percent > 99 {
		percent = 99
	}
	if r.ProgressReporter != nil {
		if err := r.ProgressReporter(ctx, in, percent); err != nil {
			log.Printf("progress report failed: %v", err)
		}
		return
	}
	if r.Table == "" || r.Dynamo == nil {
		return
	}
	_, err := r.Dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.Table),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + in.UserID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + in.JobID},
		},
		UpdateExpression: aws.String("SET #progress = :p, #updatedAt = :u"),
		ExpressionAttributeNames: map[string]string{
			"#progress":  "progress",
			"#updatedAt": "updatedAt",
		},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":p": &dbtypes.AttributeValueMemberN{Value: strconv.Itoa(percent)},
			":u": &dbtypes.AttributeValueMemberS{Value: utcNow()},
		},
	})
	if err != nil {
		log.Printf("progress report failed: %v", err)
	}
}

// 640 wide to bound Rekognition payload size) evenly across [start, end].
func (r *Renderer) defaultExtractFrames(src string, start, end float64) ([][]byte, error) {
	times := sampleTimes(start, end)
	fps := 1.0
	if span := end - start; span > 0 {
		fps = float64(len(times)) / span
	}
	dir, err := os.MkdirTemp("", "reframe-frames-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	cmd := exec.Command("ffmpeg",
		"-v", "error",
		"-ss", formatSeconds(start),
		"-to", formatSeconds(end),
		"-i", src,
		"-vf", fmt.Sprintf("fps=%.4f,scale=640:-1", fps),
		filepath.Join(dir, "frame_%03d.jpg"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("frame extraction failed: %w\noutput: %s", err, string(out))
	}
	matches, err := filepath.Glob(filepath.Join(dir, "frame_*.jpg"))
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("no frames extracted")
	}
	frames := make([][]byte, 0, len(matches))
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		frames = append(frames, b)
	}
	return frames, nil
}

// Render performs the full render task: idempotency check → download source +
// transcript → generate captions → FFmpeg crop+burn → upload the clip. It
// returns the S3 key of the rendered clip. When the clip already exists it is
// a no-op that returns the existing key. (Requirements 4.1, 4.5, 4.6, 5.5)
//
// Every call writes one "render_timing {json}" log line with the time spent in
// each phase and the reframe mode (see timing.go).
func (r *Renderer) Render(ctx context.Context, in RenderInput) (string, error) {
	tm := newRenderTimer(in)
	key, err := r.render(ctx, in, tm)
	tm.emit(err)
	return key, err
}

func (r *Renderer) render(ctx context.Context, in RenderInput, tm *renderTimer) (string, error) {
	key := ClipKey(in.UserID, in.JobID, in.Clip.ClipID)

	// Idempotent resume: skip everything if the clip is already present.
	done := tm.start(phaseCheckExists)
	exists, err := r.alreadyRendered(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing clip %s: %w", key, err)
	}
	done()
	if exists {
		tm.skipped = true
		return key, nil
	}

	if err := os.MkdirAll(r.WorkDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	// Fail fast when the runtime cannot shape captions: libass silently
	// produces captionless video when fontconfig has no fonts (observed in
	// production). A loud retryable error beats silent wrong output.
	if err := EnsureCaptionFonts(); err != nil {
		return "", err
	}

	// Download the source media.
	done = tm.start(phaseDownloadSource)
	srcData, err := r.Storage.GetObject(ctx, r.Bucket, in.SourceKey)
	if err != nil {
		return "", fmt.Errorf("failed to download source %s: %w", in.SourceKey, err)
	}
	srcPath := filepath.Join(r.WorkDir, "source"+safeExt(in.SourceKey))
	if err := os.WriteFile(srcPath, srcData, 0o644); err != nil {
		return "", fmt.Errorf("failed to write source: %w", err)
	}
	tm.sourceBytes = len(srcData)
	done()

	// Subject-aware reframing for landscape sources: track the dominant
	// face and pan the 9:16 window onto it; scale faceless content to fit
	// over a blurred background instead of blindly center cropping.
	// Falls back to center when reframing is unavailable. Samples the
	// effective (possibly user-edited) window below.
	cropX, fit := "", false

	// Download and parse the transcript.
	done = tm.start(phaseDownloadTranscript)
	transcriptData, err := r.Storage.GetObject(ctx, r.Bucket, in.TranscriptKey)
	if err != nil {
		return "", fmt.Errorf("failed to download transcript %s: %w", in.TranscriptKey, err)
	}
	transcript, err := parseTranscript(transcriptData)
	if err != nil {
		return "", fmt.Errorf("failed to parse transcript: %w", err)
	}
	done()

	// Filter words to the segment and build captions relative to the clip
	// start. Segments with no words (silence, out-of-window ranks) skip
	// captions entirely: the subtitles filter rejects empty files, so no
	// caption file means no filter (identical picture, no failure).
	// Karaoke styles build ASS (word sweep); all others build SRT.
	// A saved user edit overrides the trim window; edited words replace the
	// transcript slice (timings preserved as stored).
	done = tm.start(phaseCaptions)
	clipStart, clipEnd := in.ClipStart, in.ClipEnd
	words := filterWords(transcript.Words, clipStart, clipEnd)
	if in.Edit != nil && in.Edit.End > in.Edit.Start && in.Edit.Start >= 0 {
		clipStart, clipEnd = in.Edit.Start, in.Edit.End
		if len(in.Edit.Words) > 0 {
			words = filterWords(in.Edit.Words, clipStart, clipEnd)
		} else {
			words = filterWords(transcript.Words, clipStart, clipEnd)
		}
	}
	subsPath := ""
	if len(words) > 0 {
		var doc string
		var ext string
		if isKaraokeStyle(in.Caption) {
			doc = BuildASS(words, clipStart, in.Caption)
			ext = ".ass"
		} else {
			doc = BuildSRT(words, clipStart)
			ext = ".srt"
		}
		subsPath = filepath.Join(r.WorkDir, in.Clip.ClipID+ext)
		if err := os.WriteFile(subsPath, []byte(doc), 0o644); err != nil {
			return "", fmt.Errorf("failed to write captions: %w", err)
		}
	}
	done()

	done = tm.start(phaseReframe)
	if !in.AudioOnly {
		cropX, fit = r.smartReframe(ctx, srcPath, clipStart, clipEnd)
	}
	tm.mode = reframeMode(in.AudioOnly, cropX, fit)
	done()

	// Render with FFmpeg, streaming -progress so the job record reflects the
	// live encode percent (clamped at 99; completion is marked downstream).
	outPath := filepath.Join(r.WorkDir, in.Clip.ClipID+".mp4")
	args := r.buildCropCaptionArgs(srcPath, subsPath, outPath, clipStart, clipEnd, in.AudioOnly, cropX, fit, in.Caption)
	runProgress := r.RunCommandProgress
	if runProgress == nil {
		runProgress = defaultRunCommandProgress
	}
	duration := clipEnd - clipStart
	tm.clipSeconds = duration
	done = tm.start(phaseEncode)
	var runErr error
	if duration > 0 {
		runErr = runProgress("ffmpeg", args, duration, func(pct int) {
			r.reportProgress(ctx, in, pct)
		})
	} else {
		runErr = r.RunCommand("ffmpeg", args)
	}
	if runErr != nil {
		return "", fmt.Errorf("ffmpeg render failed: %w", runErr)
	}
	done()

	// Optional brand logo overlay (top-right). Fail-soft: branding must
	// never block an otherwise finished clip.
	done = tm.start(phaseBranding)
	if r.FetchLogo != nil {
		if logoData, err := r.FetchLogo(ctx); err != nil {
			log.Printf("branding skipped: %v", err)
		} else if len(logoData) > 0 {
			logoPath := filepath.Join(r.WorkDir, "logo.png")
			if err := os.WriteFile(logoPath, logoData, 0o644); err != nil {
				log.Printf("branding skipped: %v", err)
			} else {
				brandedPath := filepath.Join(r.WorkDir, in.Clip.ClipID+"-branded.mp4")
				if err := r.RunCommand("ffmpeg", buildLogoOverlayArgs(outPath, logoPath, brandedPath)); err != nil {
					log.Printf("branding skipped: %v", err)
				} else {
					outPath = brandedPath
				}
			}
		}
	}

	done()

	// Upload the result.
	done = tm.start(phaseUpload)
	clipData, err := os.ReadFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to read rendered clip: %w", err)
	}
	tm.outputBytes = len(clipData)
	if err := r.Storage.PutObject(ctx, r.Bucket, key, clipData, "video/mp4"); err != nil {
		return "", fmt.Errorf("failed to upload clip %s: %w", key, err)
	}
	done()

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
