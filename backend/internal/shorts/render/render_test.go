package render

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// mockStorage implements storage.StorageClient, recording calls for assertions.
type mockStorage struct {
	// exists is returned by HeadObject.
	exists   bool
	headErr  error
	getErr   error
	putErr   error
	objects  map[string][]byte // keyed by S3 key, seeds GetObject
	getCalls int
	putCalls int
	headKey  string
	putKey   string
	putData  []byte
}

func (m *mockStorage) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	m.getCalls++
	if m.getErr != nil {
		return nil, m.getErr
	}
	if data, ok := m.objects[key]; ok {
		return data, nil
	}
	return []byte("data:" + key), nil
}

func (m *mockStorage) PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	m.putCalls++
	m.putKey = key
	m.putData = data
	return m.putErr
}

func (m *mockStorage) DeleteObject(ctx context.Context, bucket, key string) error {
	return nil
}

func (m *mockStorage) HeadObject(ctx context.Context, bucket, key string) (bool, error) {
	m.headKey = key
	return m.exists, m.headErr
}

func TestBuildCropCaptionArgs_VideoFilterAndFlags(t *testing.T) {
	r := &Renderer{Bucket: "b", WorkDir: "/tmp"}
	args := r.buildCropCaptionArgs("source.mp4", "cap.srt", "out.mp4", 12.4, 39.9, false, "", false)
	joined := strings.Join(args, " ")

	// Trim before input.
	if !argPairInOrder(args, "-ss", "-i") {
		t.Error("expected -ss to appear before -i for input seeking")
	}
	if !hasArgValue(args, "-ss", "12.400") {
		t.Errorf("expected -ss 12.400, got args: %s", joined)
	}
	if !hasArgValue(args, "-to", "39.900") {
		t.Errorf("expected -to 39.900, got args: %s", joined)
	}

	// Video filter: center-crop, scale to 9:16, subtitles burn.
	vf := argValue(args, "-vf")
	if !strings.Contains(vf, "crop=w=ih*9/16:h=ih:x=(iw-ih*9/16)/2:y=0") {
		t.Errorf("expected center-crop expression in -vf, got: %s", vf)
	}
	if !strings.Contains(vf, "scale=1080:1920,setsar=1") {
		t.Errorf("expected 9:16 scale in -vf, got: %s", vf)
	}
	if !strings.Contains(vf, "fps=30") {
		t.Errorf("expected fps=30 normalization in -vf (Shorts frame-rate spec), got: %s", vf)
	}
	if !strings.Contains(vf, "subtitles=cap.srt:force_style='Alignment=2,FontSize=18,PrimaryColour=&H00FFFFFF,BorderStyle=3,Outline=2'") {
		t.Errorf("expected caption burn in -vf, got: %s", vf)
	}

	// Encoder flags.
	for _, want := range [][2]string{
		{"-c:v", "libx264"},
		{"-preset", "veryfast"},
		{"-pix_fmt", "yuv420p"},
		{"-r", "30"},
		{"-c:a", "aac"},
		{"-b:a", "128k"},
		{"-ar", "48000"},
		{"-movflags", "+faststart"},
	} {
		if !hasArgValue(args, want[0], want[1]) {
			t.Errorf("expected %s %s, got args: %s", want[0], want[1], joined)
		}
	}
	if args[len(args)-2] != "-y" || args[len(args)-1] != "out.mp4" {
		t.Errorf("expected trailing -y out.mp4, got: %v", args[len(args)-2:])
	}
}

func TestBuildCropCaptionArgs_AudioOnlyCanvas(t *testing.T) {
	r := &Renderer{Bucket: "b", WorkDir: "/tmp"}
	args := r.buildCropCaptionArgs("audio.mp3", "cap.srt", "out.mp4", 5.0, 25.0, true, "", false)
	joined := strings.Join(args, " ")

	// Audio-only synthesizes a black 1080x1920 canvas via lavfi.
	if !hasArgValue(args, "-f", "lavfi") {
		t.Errorf("expected -f lavfi for audio-only, got: %s", joined)
	}
	if !strings.Contains(joined, "color=c=black:s=1080x1920") {
		t.Errorf("expected black 1080x1920 canvas, got: %s", joined)
	}
	// No center-crop for audio-only (there is no video stream to crop).
	if strings.Contains(joined, "crop=w=ih*9/16") {
		t.Errorf("did not expect a crop filter for audio-only, got: %s", joined)
	}
	// Duration guards the canvas length.
	if !hasArgValue(args, "-t", "20.000") {
		t.Errorf("expected -t 20.000 (end-start), got: %s", joined)
	}
	vf := argValue(args, "-vf")
	if !strings.Contains(vf, "subtitles=cap.srt") {
		t.Errorf("expected subtitles burn in audio-only -vf, got: %s", vf)
	}
	// Audio-only canvas still pins output frame rate and sample rate.
	if !hasArgValue(args, "-r", "30") {
		t.Errorf("expected -r 30 for audio-only, got: %s", joined)
	}
	if !hasArgValue(args, "-ar", "48000") {
		t.Errorf("expected -ar 48000 for audio-only, got: %s", joined)
	}
}

func TestRender_HappyPath(t *testing.T) {
	workDir := t.TempDir()

	transcript := models.Transcript{
		Text: "hello world this is a clip",
		Words: []models.Word{
			{Text: "hello", Start: 10.0, End: 10.5},
			{Text: "world", Start: 10.5, End: 11.0},
			{Text: "later", Start: 100.0, End: 100.5}, // outside the segment
		},
	}
	tData, _ := json.Marshal(transcript)

	store := &mockStorage{
		exists: false,
		objects: map[string][]byte{
			"uploads/u1/j1/source.mp4":   []byte("SOURCE-BYTES"),
			"shorts/u1/j1/transcript.json": tData,
		},
	}

	// The mocked RunCommand writes the expected output file so the upload step
	// can read it — no real ffmpeg is invoked.
	var ranArgs []string
	r := &Renderer{
		Storage: store,
		Bucket:  "assets",
		WorkDir: workDir,
		RunCommand: func(name string, args []string) error {
			ranArgs = args
			if name != "ffmpeg" {
				t.Fatalf("expected ffmpeg, got %q", name)
			}
			out := args[len(args)-1]
			return os.WriteFile(out, []byte("RENDERED-CLIP"), 0o644)
		},
		RunCommandProgress: func(name string, args []string, duration float64, onProgress func(int)) error {
			ranArgs = args
			return os.WriteFile(args[len(args)-1], []byte("RENDERED-CLIP"), 0o644)
		},
	}

	in := RenderInput{
		JobID:         "j1",
		UserID:        "u1",
		SourceKey:     "uploads/u1/j1/source.mp4",
		TranscriptKey: "shorts/u1/j1/transcript.json",
		Clip:          models.Clip{ClipID: "c1"},
		ClipStart:     10.0,
		ClipEnd:       28.0,
	}

	key, err := r.Render(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantKey := "shorts/u1/j1/clips/c1.mp4"
	if key != wantKey {
		t.Errorf("expected clip key %q, got %q", wantKey, key)
	}
	if store.putCalls != 1 {
		t.Errorf("expected exactly 1 PutObject, got %d", store.putCalls)
	}
	if store.putKey != wantKey {
		t.Errorf("expected upload to %q, got %q", wantKey, store.putKey)
	}
	if string(store.putData) != "RENDERED-CLIP" {
		t.Errorf("expected uploaded rendered bytes, got %q", string(store.putData))
	}

	// A caption file was written relative to the clip start.
	srtPath := filepath.Join(workDir, "c1.srt")
	srtBytes, err := os.ReadFile(srtPath)
	if err != nil {
		t.Fatalf("expected caption file written: %v", err)
	}
	srt := string(srtBytes)
	if !strings.Contains(srt, "hello world") {
		t.Errorf("expected in-segment words in captions, got: %s", srt)
	}
	if strings.Contains(srt, "later") {
		t.Errorf("did not expect out-of-segment word in captions, got: %s", srt)
	}
	if ranArgs == nil {
		t.Error("expected ffmpeg to be invoked")
	}
}

func TestRender_IdempotentSkipsWork(t *testing.T) {
	store := &mockStorage{exists: true}
	r := &Renderer{
		Storage: store,
		Bucket:  "assets",
		WorkDir: t.TempDir(),
		RunCommand: func(name string, args []string) error {
			t.Fatal("RunCommand should not run when clip exists")
			return nil
		},
	}
	in := RenderInput{JobID: "j1", UserID: "u1", Clip: models.Clip{ClipID: "c1"}}
	key, err := r.Render(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "shorts/u1/j1/clips/c1.mp4" {
		t.Errorf("unexpected key: %q", key)
	}
	if store.putCalls != 0 {
		t.Errorf("expected no PutObject, got %d", store.putCalls)
	}
}

func TestRender_HeadObjectError(t *testing.T) {
	store := &mockStorage{headErr: errors.New("s3 down")}
	r := NewRenderer(store, "assets", t.TempDir())
	in := RenderInput{JobID: "j1", UserID: "u1", Clip: models.Clip{ClipID: "c1"}}
	if _, err := r.Render(context.Background(), in); err == nil {
		t.Fatal("expected error when HeadObject fails")
	}
}

func TestNewRenderer_SetsDefaultRunCommand(t *testing.T) {
	r := NewRenderer(&mockStorage{}, "assets", "/tmp")
	if r.RunCommand == nil {
		t.Error("expected NewRenderer to set a default RunCommand")
	}
}

// --- small arg helpers ---

func argValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func hasArgValue(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func argPairInOrder(args []string, first, second string) bool {
	fi, si := -1, -1
	for i, a := range args {
		if a == first && fi == -1 {
			fi = i
		}
		if a == second && si == -1 {
			si = i
		}
	}
	return fi != -1 && si != -1 && fi < si
}
