package renderer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CompositorConfig holds configuration parameters for video composition.
type CompositorConfig struct {
	OutputWidth        int
	OutputHeight       int
	FPS                int
	TransitionDuration float64
	CountdownDuration  int
	AnswerRevealDur    float64
	OutroDuration      float64
}

// DefaultConfig returns the default compositor configuration.
func DefaultConfig() CompositorConfig {
	return CompositorConfig{
		OutputWidth:        1920,
		OutputHeight:       1080,
		FPS:                30,
		TransitionDuration: 0.5,
		CountdownDuration:  5,
		AnswerRevealDur:    5.0,
		OutroDuration:      5.0,
	}
}

// SegmentInfo holds the file references for one question's video segment.
type SegmentInfo struct {
	QuestionSlide   string
	CountdownSlides []string
	AnswerSlide     string
	AudioFile       string
	AnswerAudio     string // audio for the answer reveal slide
}

// Compositor implements CompositorInterface using FFmpeg for video composition.
type Compositor struct {
	Config     CompositorConfig
	WorkDir    string
	RunCommand func(name string, args []string) error
}

// NewCompositor creates a new Compositor with the given config and working directory.
func NewCompositor(cfg CompositorConfig, workDir string) *Compositor {
	c := &Compositor{
		Config:  cfg,
		WorkDir: workDir,
	}
	c.RunCommand = c.defaultRunCommand
	return c
}

// defaultRunCommand executes an FFmpeg command.
func (c *Compositor) defaultRunCommand(name string, args []string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = c.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\noutput: %s", name, err, string(output))
	}
	return nil
}

// ComposeVideo composites segments and an optional outro into a final MP4 video.
// This satisfies the CompositorInterface when called from the handler via
// the simplified interface adapter.
func (c *Compositor) ComposeVideo(ctx context.Context, segments []SegmentInfo, outroSlide string) (string, error) {
	var segmentFiles []string

	for i, seg := range segments {
		segmentPath, err := c.composeSegment(ctx, i, seg)
		if err != nil {
			return "", fmt.Errorf("failed to compose segment %d: %w", i, err)
		}
		segmentFiles = append(segmentFiles, segmentPath)
	}

	// Only add outro if provided
	if outroSlide != "" {
		outroPath := filepath.Join(c.WorkDir, "outro.mp4")
		outroArgs := c.buildImageToVideoArgs(outroSlide, c.Config.OutroDuration, outroPath)
		if err := c.RunCommand("ffmpeg", outroArgs); err != nil {
			return "", fmt.Errorf("failed to create outro: %w", err)
		}
		segmentFiles = append(segmentFiles, outroPath)
	}

	// Concatenate all segments
	outputPath := filepath.Join(c.WorkDir, "output.mp4")
	if err := c.concatenateSegments(segmentFiles, outputPath); err != nil {
		return "", fmt.Errorf("failed to concatenate segments: %w", err)
	}

	return outputPath, nil
}

// composeSegment creates a single question segment (question + answer, no countdown).
func (c *Compositor) composeSegment(ctx context.Context, index int, seg SegmentInfo) (string, error) {
	// 1. Create question video with audio
	questionPath := filepath.Join(c.WorkDir, fmt.Sprintf("question_%d.mp4", index))
	questionArgs := c.buildQuestionWithAudioArgs(seg.QuestionSlide, seg.AudioFile, questionPath)
	if err := c.RunCommand("ffmpeg", questionArgs); err != nil {
		return "", fmt.Errorf("failed to create question video: %w", err)
	}

	// 2. Create answer reveal video (with audio if available, otherwise silent)
	answerPath := filepath.Join(c.WorkDir, fmt.Sprintf("answer_%d.mp4", index))
	if seg.AnswerAudio != "" {
		answerArgs := c.buildQuestionWithAudioArgs(seg.AnswerSlide, seg.AnswerAudio, answerPath)
		if err := c.RunCommand("ffmpeg", answerArgs); err != nil {
			return "", fmt.Errorf("failed to create answer video with audio: %w", err)
		}
	} else {
		answerArgs := c.buildImageToVideoArgs(seg.AnswerSlide, c.Config.AnswerRevealDur, answerPath)
		if err := c.RunCommand("ffmpeg", answerArgs); err != nil {
			return "", fmt.Errorf("failed to create answer video: %w", err)
		}
	}

	// 3. Concatenate question + answer into segment (no countdown)
	segmentPath := filepath.Join(c.WorkDir, fmt.Sprintf("segment_%d.mp4", index))
	parts := []string{questionPath, answerPath}
	if err := c.concatenateSegments(parts, segmentPath); err != nil {
		return "", fmt.Errorf("failed to concatenate segment parts: %w", err)
	}

	return segmentPath, nil
}

// buildQuestionWithAudioArgs builds FFmpeg args for a still image with audio overlay.
func (c *Compositor) buildQuestionWithAudioArgs(imagePath, audioPath, outputPath string) []string {
	return []string{
		"-loop", "1",
		"-i", imagePath,
		"-i", audioPath,
		"-vf", fmt.Sprintf("scale=%d:%d,setsar=1", c.Config.OutputWidth, c.Config.OutputHeight),
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-threads", "0",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-r", fmt.Sprintf("%d", c.Config.FPS),
		"-shortest",
		"-y", outputPath,
	}
}

// buildImageToVideoArgs builds FFmpeg args for converting a still image to a video of fixed duration.
func (c *Compositor) buildImageToVideoArgs(imagePath string, duration float64, outputPath string) []string {
	return []string{
		"-loop", "1",
		"-t", fmt.Sprintf("%.1f", duration),
		"-i", imagePath,
		"-vf", fmt.Sprintf("scale=%d:%d,setsar=1", c.Config.OutputWidth, c.Config.OutputHeight),
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-threads", "0",
		"-pix_fmt", "yuv420p",
		"-r", fmt.Sprintf("%d", c.Config.FPS),
		"-an",
		"-y", outputPath,
	}
}

// buildCountdownArgs builds FFmpeg args for concatenating countdown slides into a video.
func (c *Compositor) buildCountdownArgs(slides []string, outputPath string) []string {
	var args []string

	// Add each slide as an input with 1 second duration
	for _, slide := range slides {
		args = append(args, "-loop", "1", "-t", "1", "-i", slide)
	}

	// Build filter complex for concatenation
	var filterParts []string
	for i := range slides {
		filterParts = append(filterParts,
			fmt.Sprintf("[%d:v]scale=%d:%d,setsar=1[v%d]", i, c.Config.OutputWidth, c.Config.OutputHeight, i),
		)
	}

	var streamRefs string
	for i := range slides {
		streamRefs += fmt.Sprintf("[v%d]", i)
	}
	filterParts = append(filterParts,
		fmt.Sprintf("%sconcat=n=%d:v=1:a=0[outv]", streamRefs, len(slides)),
	)

	filterComplex := strings.Join(filterParts, ";")

	args = append(args,
		"-filter_complex", filterComplex,
		"-map", "[outv]",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-threads", "0",
		"-pix_fmt", "yuv420p",
		"-r", fmt.Sprintf("%d", c.Config.FPS),
		"-an",
		"-y", outputPath,
	)

	return args
}

// buildConcatWithTransitionsArgs builds FFmpeg args for concatenating segments using a concat file.
func (c *Compositor) buildConcatWithTransitionsArgs(segments []string, outputPath string) []string {
	concatListPath := filepath.Join(c.WorkDir, "concat_list.txt")
	_ = writeConcatList(concatListPath, segments)

	return []string{
		"-f", "concat",
		"-safe", "0",
		"-i", concatListPath,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-threads", "0",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-y", outputPath,
	}
}

// concatenateSegments concatenates multiple MP4 files into one using concat demuxer.
func (c *Compositor) concatenateSegments(segments []string, outputPath string) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments to concatenate")
	}

	if len(segments) == 1 {
		// Single segment: just copy
		args := []string{
			"-i", segments[0],
			"-c", "copy",
			"-y", outputPath,
		}
		return c.RunCommand("ffmpeg", args)
	}

	args := c.buildConcatWithTransitionsArgs(segments, outputPath)
	return c.RunCommand("ffmpeg", args)
}

// writeConcatList writes an FFmpeg concat demuxer list file.
func writeConcatList(path string, files []string) error {
	var lines []string
	for _, f := range files {
		lines = append(lines, fmt.Sprintf("file '%s'", f))
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

// composeSimple provides a fallback composition that pairs slides 1:1 with audio.
func (c *Compositor) composeSimple(ctx context.Context, workDir string, slideFiles, audioFiles []string) (string, error) {
	if len(slideFiles) == 0 {
		return "", fmt.Errorf("no slide files provided")
	}
	if len(audioFiles) == 0 {
		return "", fmt.Errorf("no audio files provided")
	}

	var segmentFiles []string
	numPairs := len(audioFiles)
	if numPairs > len(slideFiles) {
		numPairs = len(slideFiles)
	}

	for i := 0; i < numPairs; i++ {
		segPath := filepath.Join(workDir, fmt.Sprintf("simple_seg_%d.mp4", i))
		args := c.buildQuestionWithAudioArgs(slideFiles[i], audioFiles[i], segPath)
		if err := c.RunCommand("ffmpeg", args); err != nil {
			return "", fmt.Errorf("failed to create segment %d: %w", i, err)
		}
		segmentFiles = append(segmentFiles, segPath)
	}

	outputPath := filepath.Join(workDir, "output.mp4")
	if err := c.concatenateSegments(segmentFiles, outputPath); err != nil {
		return "", fmt.Errorf("failed to concatenate: %w", err)
	}

	return outputPath, nil
}

// BuildSegments organizes slide and audio files into structured segments for composition.
// Each question requires: 1 question slide + 1 answer slide (no countdown).
// Audio files are interleaved: [q0.mp3, q0_answer.mp3, q1.mp3, q1_answer.mp3, ...]
func BuildSegments(slideFiles, audioFiles []string, numQuestions int) ([]SegmentInfo, string, error) {
	slidesPerQuestion := 2 // question + answer (no countdown)
	audiosPerQuestion := 2 // question audio + answer audio
	expectedSlides := numQuestions * slidesPerQuestion
	expectedAudios := numQuestions * audiosPerQuestion

	if len(slideFiles) < expectedSlides {
		return nil, "", fmt.Errorf("not enough slides: have %d, need %d", len(slideFiles), expectedSlides)
	}
	if len(audioFiles) < expectedAudios {
		return nil, "", fmt.Errorf("not enough audio files: have %d, need %d", len(audioFiles), expectedAudios)
	}

	var segments []SegmentInfo
	for q := 0; q < numQuestions; q++ {
		slideIdx := q * slidesPerQuestion
		audioIdx := q * audiosPerQuestion

		seg := SegmentInfo{
			QuestionSlide:   slideFiles[slideIdx],
			CountdownSlides: nil,
			AnswerSlide:     slideFiles[slideIdx+1],
			AudioFile:       audioFiles[audioIdx],
			AnswerAudio:     audioFiles[audioIdx+1],
		}
		segments = append(segments, seg)
	}

	return segments, "", nil
}

// DownloadAssets downloads slide and audio files from S3 to local directories.
func DownloadAssets(ctx context.Context, storage StorageClient, bucket string, slideKeys, audioKeys []string, workDir string) ([]string, []string, error) {
	slidesDir := filepath.Join(workDir, "slides")
	audioDir := filepath.Join(workDir, "audio")

	if err := os.MkdirAll(slidesDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create slides directory: %w", err)
	}
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create audio directory: %w", err)
	}

	// Resolve absolute paths for containment checks
	absSlidesDir, err := filepath.Abs(slidesDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve slides directory path: %w", err)
	}
	absAudioDir, err := filepath.Abs(audioDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve audio directory path: %w", err)
	}

	// Download slides
	var slideFiles []string
	for _, key := range slideKeys {
		data, err := storage.GetObject(ctx, bucket, key)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to download slide (key=%s): %w", key, err)
		}
		filename := filepath.Base(key)
		// Validate filename has no path traversal
		if strings.Contains(filename, "..") || strings.ContainsAny(filename, `/\`) {
			return nil, nil, fmt.Errorf("unsafe filename in key: %s", key)
		}
		localPath := filepath.Join(slidesDir, filename)
		absPath, err := filepath.Abs(localPath)
		if err != nil || !strings.HasPrefix(absPath, absSlidesDir+string(os.PathSeparator)) && absPath != absSlidesDir {
			return nil, nil, fmt.Errorf("unsafe file path detected for slide key: %s", key)
		}
		if err := os.WriteFile(absPath, data, 0o644); err != nil {
			return nil, nil, fmt.Errorf("failed to write slide: %w", err)
		}
		slideFiles = append(slideFiles, absPath)
	}

	// Download audio
	var audioFiles []string
	for _, key := range audioKeys {
		data, err := storage.GetObject(ctx, bucket, key)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to download audio (key=%s): %w", key, err)
		}
		filename := filepath.Base(key)
		// Validate filename has no path traversal
		if strings.Contains(filename, "..") || strings.ContainsAny(filename, `/\`) {
			return nil, nil, fmt.Errorf("unsafe filename in key: %s", key)
		}
		localPath := filepath.Join(audioDir, filename)
		absPath, err := filepath.Abs(localPath)
		if err != nil || !strings.HasPrefix(absPath, absAudioDir+string(os.PathSeparator)) && absPath != absAudioDir {
			return nil, nil, fmt.Errorf("unsafe file path detected for audio key: %s", key)
		}
		if err := os.WriteFile(absPath, data, 0o644); err != nil {
			return nil, nil, fmt.Errorf("failed to write audio: %w", err)
		}
		audioFiles = append(audioFiles, absPath)
	}

	return slideFiles, audioFiles, nil
}
