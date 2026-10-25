package render

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// mockStorage implements storage.StorageClient, recording calls for assertions.
type mockStorage struct {
	// exists is returned by HeadObject and HeadObjectWithMetadata.
	exists bool
	// meta is the object metadata HeadObjectWithMetadata returns.
	meta        map[string]string
	headErr     error
	getErr      error
	putErr      error
	objects     map[string][]byte // keyed by S3 key, seeds GetObject
	getCalls    int
	putCalls    int
	deleteCalls int
	headKey     string
	putKey      string
	putData     []byte
	putMeta     map[string]string
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
	m.deleteCalls++
	return nil
}

func (m *mockStorage) HeadObject(ctx context.Context, bucket, key string) (bool, error) {
	m.headKey = key
	return m.exists, m.headErr
}

// HeadObjectWithMetadata answers from exists, meta and headErr.
func (m *mockStorage) HeadObjectWithMetadata(ctx context.Context, bucket, key string) (map[string]string, bool, error) {
	m.headKey = key
	return m.meta, m.exists, m.headErr
}

// PutObjectWithMetadata uploads like PutObject and records the metadata.
func (m *mockStorage) PutObjectWithMetadata(ctx context.Context, bucket, key string, data []byte, contentType string, metadata map[string]string) error {
	m.putMeta = metadata
	return m.PutObject(ctx, bucket, key, data, contentType)
}

func TestBuildCropCaptionArgs_VideoFilterAndFlags(t *testing.T) {
	r := &Renderer{Bucket: "b", WorkDir: "/tmp"}
	args := r.buildCropCaptionArgs("source.mp4", "cap.srt", "out.mp4", 12.4, 39.9, false, "", false, models.CaptionStyle{})
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
	args := r.buildCropCaptionArgs("audio.mp3", "cap.srt", "out.mp4", 5.0, 25.0, true, "", false, models.CaptionStyle{})
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

// legacyArgsCase is one entry of testdata/legacy_args.json: the argument list
// origin/main produced for a render without effects.
type legacyArgsCase struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

// TestBuildCropCaptionArgs_MatchesOriginMain pins the no-effects argument lists
// to the lists origin/main produced. A job without effect settings must keep
// the same ffmpeg command.
func TestBuildCropCaptionArgs_MatchesOriginMain(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "legacy_args.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden []legacyArgsCase
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		subs      string
		audioOnly bool
		cropX     string
		fit       bool
		style     models.CaptionStyle
	}{
		{"video_no_captions", "", false, "", false, models.CaptionStyle{}},
		{"video_bold_default", "cap.srt", false, "", false, models.CaptionStyle{}},
		{"video_karaoke", "cap.srt", false, "", false, models.CaptionStyle{Preset: "karaoke"}},
		{"audio_only_captions", "cap.srt", true, "", false, models.CaptionStyle{}},
		{"video_subject_crop_minimal", "cap.srt", false, "420", false, models.CaptionStyle{Preset: "minimal", Position: "top"}},
		{"video_blur_fill_clean", "cap.srt", false, "", true, models.CaptionStyle{Preset: "clean", FontSize: 20, Color: "#00FF00"}},
	}
	if len(golden) != len(cases) {
		t.Fatalf("golden has %d cases, test has %d", len(golden), len(cases))
	}
	r := &Renderer{Bucket: "b", WorkDir: "/tmp"}
	for i, c := range cases {
		if golden[i].Name != c.name {
			t.Fatalf("case %d: golden %q, test %q", i, golden[i].Name, c.name)
		}
		got := r.buildCropCaptionArgs("source.mp4", c.subs, "out.mp4", 12.4, 39.9, c.audioOnly, c.cropX, c.fit, c.style)
		if !reflect.DeepEqual(got, golden[i].Args) {
			t.Errorf("%s changed\n got: %q\nwant: %q", c.name, got, golden[i].Args)
		}
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
			"uploads/u1/j1/source.mp4":     []byte("SOURCE-BYTES"),
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
		RunCommandProgress: func(_ context.Context, name string, args []string, duration float64, onProgress func(int)) error {
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
	in := RenderInput{JobID: "j1", UserID: "u1", Clip: models.Clip{ClipID: "c1"}}
	fp, err := renderFingerprint(in)
	if err != nil {
		t.Fatal(err)
	}
	store := &mockStorage{exists: true, meta: map[string]string{fingerprintMetaKey: fp}}
	r := &Renderer{
		Storage: store,
		Bucket:  "assets",
		WorkDir: t.TempDir(),
		RunCommand: func(name string, args []string) error {
			t.Fatal("RunCommand should not run when clip exists")
			return nil
		},
	}
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

// A clip that exists with another fingerprint renders again. The new clip
// carries this render's fingerprint, and no object is deleted.
func TestRender_RerendersWhenFingerprintDiffers(t *testing.T) {
	encodes := 0
	run := func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
		encodes++
		return writeOutput(args)
	}
	r, store, in := effectsRenderer(t, nil, run)
	store.exists = true
	store.meta = map[string]string{fingerprintMetaKey: "fingerprint-of-an-earlier-render"}
	key, err := r.Render(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if encodes != 1 {
		t.Errorf("a clip with another fingerprint must render once, got %d encodes", encodes)
	}
	want, err := renderFingerprint(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.putMeta[fingerprintMetaKey]; got != want {
		t.Errorf("the new clip must carry this render's fingerprint: got %q, want %q", got, want)
	}
	if key != ClipKey("u1", "j1", "c1") || store.putKey != key {
		t.Errorf("the clip keeps its key: returned %q, uploaded to %q", key, store.putKey)
	}
	if store.deleteCalls != 0 {
		t.Errorf("a re-render must not delete objects, got %d deletes", store.deleteCalls)
	}
}

// A clip stored before fingerprints existed has no metadata, so it renders once more.
func TestRender_RerendersWithoutFingerprint(t *testing.T) {
	encodes := 0
	run := func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
		encodes++
		return writeOutput(args)
	}
	r, store, in := effectsRenderer(t, nil, run)
	store.exists = true
	if _, err := r.Render(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if encodes != 1 || store.putCalls != 1 {
		t.Errorf("a clip without a fingerprint must render once: encodes=%d uploads=%d", encodes, store.putCalls)
	}
}

// Effects do not change the clip key. The settings are kept in the object's metadata.
func TestRender_EffectsKeepTheLegacyClipKey(t *testing.T) {
	effects := &models.VideoEffects{Grade: &models.GradeEffect{Enabled: true, Look: "warm", Intensity: 50}}
	run := func(_ context.Context, _ string, args []string, _ float64, _ func(int)) error {
		return writeOutput(args)
	}
	r, store, in := effectsRenderer(t, effects, run)
	key, err := r.Render(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if want := ClipKey("u1", "j1", "c1"); key != want || store.putKey != want {
		t.Errorf("the clip key must not depend on effects: returned %q, uploaded to %q, want %q", key, store.putKey, want)
	}
}

// renderFingerprint changes when the output changes, and stays the same when it does not.
func TestRenderFingerprintFollowsOutputSettings(t *testing.T) {
	on := &models.VideoEffects{Grade: &models.GradeEffect{Enabled: true, Look: "warm", Intensity: 50}}
	other := &models.VideoEffects{Grade: &models.GradeEffect{Enabled: true, Look: "cool", Intensity: 50}}
	off := &models.VideoEffects{Grade: &models.GradeEffect{Enabled: false, Look: "warm", Intensity: 50}}
	base := RenderInput{ClipStart: 10, ClipEnd: 28, Caption: models.CaptionStyle{Preset: "bold"}}
	withOn := base
	withOn.Effects = on
	withOther := base
	withOther.Effects = other
	withOff := base
	withOff.Effects = off
	styled := base
	styled.Caption = models.CaptionStyle{Preset: "karaoke"}
	shifted := base
	shifted.ClipEnd = 30
	edit := &models.ClipEdit{Start: 10, End: 28, Words: []models.Word{{Text: "hi", Start: 10, End: 10.5}}}
	editedAgain := *edit
	editedAgain.UpdatedAt = "2026-10-25T10:00:00Z"
	withEdit := base
	withEdit.Edit = edit
	withEditAgain := base
	withEditAgain.Edit = &editedAgain
	withNewWords := base
	withNewWords.Edit = &models.ClipEdit{Start: 10, End: 28, Words: []models.Word{{Text: "bye", Start: 10, End: 10.5}}}

	cases := []struct {
		name string
		a, b RenderInput
		same bool
	}{
		{"no effects and all disabled match", base, withOff, true},
		{"enabled effects differ from none", base, withOn, false},
		{"different effects differ", withOn, withOther, false},
		{"same effects match", withOn, withOn, true},
		{"caption style differs", base, styled, false},
		{"clip window differs", base, shifted, false},
		{"an edit differs from none", base, withEdit, false},
		{"an edit saved again matches", withEdit, withEditAgain, true},
		{"edited words differ", withEdit, withNewWords, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := renderFingerprint(tc.a)
			if err != nil {
				t.Fatal(err)
			}
			b, err := renderFingerprint(tc.b)
			if err != nil {
				t.Fatal(err)
			}
			if (a == b) != tc.same {
				t.Errorf("same=%v expected, got %s and %s", tc.same, a, b)
			}
		})
	}
}

// A store without metadata support cannot tell which settings a clip has, so it
// is refused before any work.
func TestRender_StoreWithoutMetadataIsRefused(t *testing.T) {
	store := struct{ storage.StorageClient }{&mockStorage{}}
	r := &Renderer{Storage: store, Bucket: "assets", WorkDir: t.TempDir()}
	in := RenderInput{JobID: "j1", UserID: "u1", Clip: models.Clip{ClipID: "c1"}}
	if _, err := r.Render(context.Background(), in); err == nil || !strings.Contains(err.Error(), "fingerprints") {
		t.Fatalf("expected a refusal for a store without metadata, got %v", err)
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
