package renderer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- DefaultConfig Tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.OutputWidth != 1920 {
		t.Errorf("expected OutputWidth 1920, got %d", cfg.OutputWidth)
	}
	if cfg.OutputHeight != 1080 {
		t.Errorf("expected OutputHeight 1080, got %d", cfg.OutputHeight)
	}
	if cfg.FPS != 30 {
		t.Errorf("expected FPS 30, got %d", cfg.FPS)
	}
	if cfg.TransitionDuration != 0.5 {
		t.Errorf("expected TransitionDuration 0.5, got %f", cfg.TransitionDuration)
	}
	if cfg.CountdownDuration != 5 {
		t.Errorf("expected CountdownDuration 5, got %d", cfg.CountdownDuration)
	}
	if cfg.AnswerRevealDur != 5.0 {
		t.Errorf("expected AnswerRevealDur 5.0, got %f", cfg.AnswerRevealDur)
	}
	if cfg.OutroDuration != 5.0 {
		t.Errorf("expected OutroDuration 5.0, got %f", cfg.OutroDuration)
	}
}

// --- FFmpeg Argument Construction Tests ---

func TestBuildQuestionWithAudioArgs(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCompositor(cfg, "/tmp/work")

	args := c.buildQuestionWithAudioArgs(
		"/tmp/slides/q0_question.png",
		"/tmp/audio/q0.mp3",
		"/tmp/work/question.mp4",
	)

	assertContainsStr(t, args, "-loop")
	assertContainsStr(t, args, "1")
	assertContainsStr(t, args, "/tmp/slides/q0_question.png")
	assertContainsStr(t, args, "/tmp/audio/q0.mp3")
	assertContainsStr(t, args, "libx264")
	assertContainsStr(t, args, "aac")
	assertContainsStr(t, args, "yuv420p")
	assertContainsStr(t, args, "-shortest")
	assertContainsStr(t, args, "30")
	assertContainsStr(t, args, "/tmp/work/question.mp4")

	hasScaleFilter := false
	for _, arg := range args {
		if strings.Contains(arg, "scale=1920:1080") {
			hasScaleFilter = true
			break
		}
	}
	if !hasScaleFilter {
		t.Error("expected scale filter with 1920:1080")
	}
}

func TestBuildImageToVideoArgs(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCompositor(cfg, "/tmp/work")

	args := c.buildImageToVideoArgs(
		"/tmp/slides/answer.png", 3.0, "/tmp/work/answer.mp4",
	)

	assertContainsStr(t, args, "-loop")
	assertContainsStr(t, args, "1")
	assertContainsStr(t, args, "-t")
	assertContainsStr(t, args, "3.0")
	assertContainsStr(t, args, "/tmp/slides/answer.png")
	assertContainsStr(t, args, "libx264")
	assertContainsStr(t, args, "-an")
	assertContainsStr(t, args, "/tmp/work/answer.mp4")
}

func TestBuildCountdownArgs(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCompositor(cfg, "/tmp/work")

	slides := []string{
		"/tmp/slides/q0_countdown_1.png",
		"/tmp/slides/q0_countdown_2.png",
		"/tmp/slides/q0_countdown_3.png",
		"/tmp/slides/q0_countdown_4.png",
		"/tmp/slides/q0_countdown_5.png",
	}

	args := c.buildCountdownArgs(slides, "/tmp/work/countdown.mp4")

	for _, slide := range slides {
		assertContainsStr(t, args, slide)
	}
	assertContainsStr(t, args, "-filter_complex")
	assertContainsStr(t, args, "libx264")
	assertContainsStr(t, args, "/tmp/work/countdown.mp4")

	hasConcat := false
	for _, arg := range args {
		if strings.Contains(arg, "concat=n=5:v=1:a=0") {
			hasConcat = true
			break
		}
	}
	if !hasConcat {
		t.Error("expected concat=n=5:v=1:a=0 in filter")
	}
}

func TestBuildConcatWithTransitionsArgs(t *testing.T) {
	workDir := t.TempDir()
	cfg := DefaultConfig()
	c := NewCompositor(cfg, workDir)

	segments := []string{"/tmp/seg_0.mp4", "/tmp/seg_1.mp4", "/tmp/outro.mp4"}
	args := c.buildConcatWithTransitionsArgs(segments, "/tmp/output.mp4")

	assertContainsStr(t, args, "-f")
	assertContainsStr(t, args, "concat")
	assertContainsStr(t, args, "-safe")
	assertContainsStr(t, args, "0")
	assertContainsStr(t, args, filepath.Join(workDir, "concat_list.txt"))
	assertContainsStr(t, args, "/tmp/output.mp4")
	assertContainsStr(t, args, "libx264")
}

// --- WriteConcatList Tests ---

func TestWriteConcatList(t *testing.T) {
	tmpDir := t.TempDir()
	listPath := filepath.Join(tmpDir, "concat.txt")

	files := []string{"/tmp/segment_0.mp4", "/tmp/segment_1.mp4", "/tmp/outro.mp4"}
	err := writeConcatList(listPath, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatalf("failed to read concat list: %v", err)
	}

	expected := "file '/tmp/segment_0.mp4'\nfile '/tmp/segment_1.mp4'\nfile '/tmp/outro.mp4'\n"
	if string(content) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(content))
	}
}

// --- BuildSegments Tests ---

func TestBuildSegments_SingleQuestion(t *testing.T) {
	slideFiles := []string{
		"/tmp/slides/q0_question.png",
		"/tmp/slides/q0_answer.png",
	}
	audioFiles := []string{"/tmp/audio/q0.mp3", "/tmp/audio/q0_answer.mp3"}

	segments, outroSlide, err := BuildSegments(slideFiles, audioFiles, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}

	seg := segments[0]
	if seg.QuestionSlide != "/tmp/slides/q0_question.png" {
		t.Errorf("question slide: got %s", seg.QuestionSlide)
	}
	if seg.CountdownSlides != nil {
		t.Errorf("expected nil countdown slides, got %v", seg.CountdownSlides)
	}
	if seg.AnswerSlide != "/tmp/slides/q0_answer.png" {
		t.Errorf("answer slide: got %s", seg.AnswerSlide)
	}
	if seg.AudioFile != "/tmp/audio/q0.mp3" {
		t.Errorf("audio file: got %s", seg.AudioFile)
	}
	if seg.AnswerAudio != "/tmp/audio/q0_answer.mp3" {
		t.Errorf("answer audio: got %s", seg.AnswerAudio)
	}
	if outroSlide != "" {
		t.Errorf("expected empty outro slide, got %s", outroSlide)
	}
}

func TestBuildSegments_MultipleQuestions(t *testing.T) {
	slideFiles := []string{
		"/tmp/slides/q0_question.png",
		"/tmp/slides/q0_answer.png",
		"/tmp/slides/q1_question.png",
		"/tmp/slides/q1_answer.png",
	}
	audioFiles := []string{
		"/tmp/audio/q0.mp3", "/tmp/audio/q0_answer.mp3",
		"/tmp/audio/q1.mp3", "/tmp/audio/q1_answer.mp3",
	}

	segments, outroSlide, err := BuildSegments(slideFiles, audioFiles, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}

	seg := segments[1]
	if seg.QuestionSlide != "/tmp/slides/q1_question.png" {
		t.Errorf("q1 question: got %s", seg.QuestionSlide)
	}
	if seg.AudioFile != "/tmp/audio/q1.mp3" {
		t.Errorf("q1 audio: got %s", seg.AudioFile)
	}
	if seg.AnswerAudio != "/tmp/audio/q1_answer.mp3" {
		t.Errorf("q1 answer audio: got %s", seg.AnswerAudio)
	}
	if outroSlide != "" {
		t.Errorf("expected empty outro slide, got %s", outroSlide)
	}
}

func TestBuildSegments_MissingSlides(t *testing.T) {
	slideFiles := []string{"/tmp/slides/q0_question.png"}
	audioFiles := []string{"/tmp/audio/q0.mp3", "/tmp/audio/q0_answer.mp3"}

	_, _, err := BuildSegments(slideFiles, audioFiles, 1)
	if err == nil {
		t.Fatal("expected error for missing slides")
	}
}

func TestBuildSegments_MissingAudio(t *testing.T) {
	slideFiles := []string{
		"/tmp/slides/q0_question.png",
		"/tmp/slides/q0_answer.png",
	}
	audioFiles := []string{} // Need 2 audio files for 1 question

	_, _, err := BuildSegments(slideFiles, audioFiles, 1)
	if err == nil {
		t.Fatal("expected error for missing audio")
	}
}

// --- DownloadAssets Tests ---

func TestDownloadAssets_Success(t *testing.T) {
	store := newMockStorageClient()
	store.getData = []byte("file-data")

	workDir := t.TempDir()
	slideKeys := []string{
		"temp/proj1/slides/q0_question.png",
		"temp/proj1/slides/q0_answer.png",
	}
	audioKeys := []string{"temp/proj1/audio/q0.mp3"}

	slideFiles, audioFiles, err := DownloadAssets(
		context.Background(), store, "bucket", slideKeys, audioKeys, workDir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slideFiles) != 2 {
		t.Fatalf("expected 2 slide files, got %d", len(slideFiles))
	}
	if len(audioFiles) != 1 {
		t.Fatalf("expected 1 audio file, got %d", len(audioFiles))
	}

	for _, f := range slideFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("slide file missing: %s", f)
		}
	}
	for _, f := range audioFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("audio file missing: %s", f)
		}
	}
}

func TestDownloadAssets_SlideDownloadError(t *testing.T) {
	failStore := &failingGetStorageClient{}

	workDir := t.TempDir()
	slideKeys := []string{"temp/proj1/slides/missing.png"}

	_, _, err := DownloadAssets(
		context.Background(), failStore, "bucket", slideKeys, nil, workDir,
	)
	if err == nil {
		t.Fatal("expected error for slide download failure")
	}
	if !strings.Contains(err.Error(), "failed to download slide") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadAssets_AudioDownloadError(t *testing.T) {
	failOnAudio := &selectiveFailStorageClient{
		failPrefix: "temp/proj1/audio/",
	}

	workDir := t.TempDir()
	slideKeys := []string{"temp/proj1/slides/q0_question.png"}
	audioKeys := []string{"temp/proj1/audio/q0.mp3"}

	_, _, err := DownloadAssets(
		context.Background(), failOnAudio, "bucket", slideKeys, audioKeys, workDir,
	)
	if err == nil {
		t.Fatal("expected error for audio download failure")
	}
	if !strings.Contains(err.Error(), "failed to download audio") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- ComposeVideo Tests (mocked FFmpeg) ---

func TestComposeVideo_CallsFFmpeg(t *testing.T) {
	workDir := t.TempDir()
	cfg := DefaultConfig()
	c := NewCompositor(cfg, workDir)

	var cmdCount int
	c.RunCommand = func(name string, args []string) error {
		cmdCount++
		for _, arg := range args {
			if strings.HasSuffix(arg, ".mp4") {
				_ = os.WriteFile(arg, []byte("fake"), 0o644)
				break
			}
		}
		return nil
	}

	segments := []SegmentInfo{
		{
			QuestionSlide:   "/tmp/q0_question.png",
			CountdownSlides: nil,
			AnswerSlide:     "/tmp/q0_answer.png",
			AudioFile:       "/tmp/q0.mp3",
			AnswerAudio:     "/tmp/q0_answer.mp3",
		},
	}

	outputPath, err := c.ComposeVideo(context.Background(), segments, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(outputPath, "output.mp4") {
		t.Errorf("expected output.mp4, got %s", outputPath)
	}

	// question + answer-with-audio + concat-parts + final-concat (no outro)
	if cmdCount < 3 {
		t.Errorf("expected at least 3 FFmpeg calls, got %d", cmdCount)
	}
}

func TestComposeVideo_FFmpegFailure(t *testing.T) {
	workDir := t.TempDir()
	cfg := DefaultConfig()
	c := NewCompositor(cfg, workDir)

	c.RunCommand = func(name string, args []string) error {
		return fmt.Errorf("ffmpeg not found")
	}

	segments := []SegmentInfo{
		{
			QuestionSlide:   "/tmp/q0_question.png",
			CountdownSlides: nil,
			AnswerSlide:     "/tmp/q0_answer.png",
			AudioFile:       "/tmp/q0.mp3",
			AnswerAudio:     "/tmp/q0_answer.mp3",
		},
	}

	_, err := c.ComposeVideo(context.Background(), segments, "")
	if err == nil {
		t.Fatal("expected error when FFmpeg fails")
	}
}

func TestConcatenateSegments_EmptyList(t *testing.T) {
	workDir := t.TempDir()
	cfg := DefaultConfig()
	c := NewCompositor(cfg, workDir)

	err := c.concatenateSegments([]string{}, "/tmp/output.mp4")
	if err == nil {
		t.Fatal("expected error for empty segment list")
	}
}

func TestConcatenateSegments_SingleSegment(t *testing.T) {
	workDir := t.TempDir()
	cfg := DefaultConfig()
	c := NewCompositor(cfg, workDir)

	var capturedArgs []string
	c.RunCommand = func(name string, args []string) error {
		capturedArgs = args
		return nil
	}

	err := c.concatenateSegments([]string{"/tmp/segment_0.mp4"}, "/tmp/out.mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContainsStr(t, capturedArgs, "-c")
	assertContainsStr(t, capturedArgs, "copy")
}
