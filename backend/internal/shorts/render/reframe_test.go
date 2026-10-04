package render

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

// --- ComputeCropX ---

func TestComputeCropX_TracksRightSideSubject(t *testing.T) {
	// 1920x1080 landscape source: window is 607 wide. A face centered at
	// 0.75 of the width should pan the window right instead of center-crop.
	frames := [][]FaceBox{
		{{Left: 0.65, Top: 0.2, Width: 0.2, Height: 0.3}},
		{{Left: 0.66, Top: 0.2, Width: 0.2, Height: 0.3}},
		{{Left: 0.64, Top: 0.2, Width: 0.2, Height: 0.3}},
	}
	x, ok := ComputeCropX(frames, 1920, 1080)
	if !ok {
		t.Fatal("expected ok=true when faces are present")
	}
	// Median center 0.75*1920=1440, minus 303 => ~1137, rounded even.
	if x < 1000 || x > 1200 {
		t.Errorf("expected right-panned offset ~1136, got %d", x)
	}
	if x%2 != 0 {
		t.Errorf("expected even offset for yuv420p, got %d", x)
	}
}

func TestComputeCropX_PicksLargestFace(t *testing.T) {
	frames := [][]FaceBox{
		{
			{Left: 0.0, Top: 0.0, Width: 0.05, Height: 0.05}, // passerby
			{Left: 0.4, Top: 0.2, Width: 0.2, Height: 0.3},   // speaker
		},
	}
	x, ok := ComputeCropX(frames, 1920, 1080)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Speaker center 0.5 => 960-303=657 -> 656.
	if x != 656 {
		t.Errorf("expected offset 656 on the speaker, got %d", x)
	}
}

func TestComputeCropX_NoFacesFallsBack(t *testing.T) {
	for _, frames := range [][][]FaceBox{nil, {}, {{}}, {{{Left: 0, Top: 0, Width: 0, Height: 0}}}} {
		if _, ok := ComputeCropX(frames, 1920, 1080); ok {
			t.Errorf("expected ok=false for %v", frames)
		}
	}
}

func TestComputeCropX_ClampsToBounds(t *testing.T) {
	// Face glued to the left edge: offset must not go negative.
	x, ok := ComputeCropX([][]FaceBox{{{Left: 0, Top: 0, Width: 0.05, Height: 0.1}}}, 1920, 1080)
	if !ok || x != 0 {
		t.Errorf("expected clamped 0, got %d (ok=%v)", x, ok)
	}
	// Face glued to the right edge: offset must not exceed 1920-607=1313.
	x, ok = ComputeCropX([][]FaceBox{{{Left: 0.95, Top: 0, Width: 0.05, Height: 0.1}}}, 1920, 1080)
	if !ok || x != 1312 { // 1313 rounded down to even
		t.Errorf("expected clamped 1312, got %d (ok=%v)", x, ok)
	}
}

func TestComputeCropX_VerticalSourceNeedsNoPan(t *testing.T) {
	x, ok := ComputeCropX(nil, 1080, 1920)
	if !ok || x != 0 {
		t.Errorf("expected (0, true) for vertical source, got (%d, %v)", x, ok)
	}
}

// --- sampleTimes ---

func TestSampleTimes_Bounded(t *testing.T) {
	times := sampleTimes(10, 70)
	if len(times) == 0 || len(times) > maxSampleFrames {
		t.Fatalf("expected 1..%d samples, got %d", maxSampleFrames, len(times))
	}
	for _, ts := range times {
		if ts < 10 || ts >= 70 {
			t.Errorf("sample %v outside [10, 70)", ts)
		}
	}
	if times[0] != 10 {
		t.Errorf("expected first sample at segment start, got %v", times[0])
	}
}

// --- RekognitionFaceDetector ---

type mockRekognition struct {
	out *rekognition.DetectFacesOutput
	err error
}

func (m *mockRekognition) DetectFaces(ctx context.Context, in *rekognition.DetectFacesInput, optFns ...func(*rekognition.Options)) (*rekognition.DetectFacesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.out, nil
}

func f32(v float32) *float32 { return &v }

func TestRekognitionDetector_MapsBoxesAndFiltersConfidence(t *testing.T) {
	mock := &mockRekognition{out: &rekognition.DetectFacesOutput{
		FaceDetails: []types.FaceDetail{
			{BoundingBox: &types.BoundingBox{Left: f32(0.6), Top: f32(0.2), Width: f32(0.2), Height: f32(0.3)}, Confidence: f32(99)},
			{BoundingBox: &types.BoundingBox{Left: f32(0.1), Top: f32(0.1), Width: f32(0.1), Height: f32(0.1)}, Confidence: f32(10)}, // dropped
			{Confidence: f32(99)}, // no box, dropped
		},
	}}
	d := NewRekognitionFaceDetector(mock)
	boxes, err := d.DetectFaces(context.Background(), []byte("jpeg"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(boxes) != 1 {
		t.Fatalf("expected 1 box after filtering, got %d", len(boxes))
	}
	if c := boxes[0].CenterX(); c < 0.699 || c > 0.701 {
		t.Errorf("expected center ~0.7, got %v", c)
	}
}

func TestRekognitionDetector_ErrorPropagates(t *testing.T) {
	mock := &mockRekognition{err: errors.New("throttled")}
	d := NewRekognitionFaceDetector(mock)
	if _, err := d.DetectFaces(context.Background(), []byte("jpeg")); err == nil {
		t.Fatal("expected error to propagate")
	}
}

// --- smartCropX wiring ---

type stubDetector struct {
	boxes [][]FaceBox
	err   error
	calls int
}

func (s *stubDetector) DetectFaces(ctx context.Context, image []byte) ([]FaceBox, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.calls-1 < len(s.boxes) {
		return s.boxes[s.calls-1], nil
	}
	return nil, nil
}

func TestSmartCropX_HappyPath(t *testing.T) {
	r := &Renderer{
		ProbeDimensions: func(path string) (int, int, error) { return 1920, 1080, nil },
		ExtractFrames: func(src string, start, end float64) ([][]byte, error) {
			return [][]byte{[]byte("f1"), []byte("f2")}, nil
		},
		Detector: &stubDetector{boxes: [][]FaceBox{
			{{Left: 0.65, Top: 0.2, Width: 0.2, Height: 0.3}},
			{{Left: 0.66, Top: 0.2, Width: 0.2, Height: 0.3}},
		}},
	}
	x, fit := r.smartReframe(context.Background(), "source.mp4", 10, 40)
	if fit {
		t.Fatal("expected tracked crop, got blur-fill fit")
	}
	// Median center 0.76: int(0.76*1920)=1459, minus 303 = 1156 (even).
	if x != "1156" {
		t.Errorf("expected offset 1156, got %q", x)
	}
}

func TestSmartCropX_FailSoftToCenter(t *testing.T) {
	cases := map[string]*Renderer{
		"nil detector": {},
		"probe error": {
			ProbeDimensions: func(string) (int, int, error) { return 0, 0, errors.New("no streams") },
			Detector:        &stubDetector{},
		},
		"extract error": {
			ProbeDimensions: func(string) (int, int, error) { return 1920, 1080, nil },
			ExtractFrames:   func(string, float64, float64) ([][]byte, error) { return nil, errors.New("ffmpeg down") },
			Detector:        &stubDetector{},
		},
		"detection error": {
			ProbeDimensions: func(string) (int, int, error) { return 1920, 1080, nil },
			ExtractFrames:   func(string, float64, float64) ([][]byte, error) { return [][]byte{[]byte("f")}, nil },
			Detector:        &stubDetector{err: errors.New("throttled")},
		},
	}
	for name, r := range cases {
		if x, fit := r.smartReframe(context.Background(), "source.mp4", 10, 40); x != "" || fit {
			t.Errorf("%s: expected centered fallback, got (%q, fit=%v)", name, x, fit)
		}
	}
}

func TestSmartCropX_NoFacesUsesFitFill(t *testing.T) {
	r := &Renderer{
		ProbeDimensions: func(string) (int, int, error) { return 1920, 1080, nil },
		ExtractFrames:   func(string, float64, float64) ([][]byte, error) { return [][]byte{[]byte("f")}, nil },
		Detector:        &stubDetector{boxes: [][]FaceBox{{}}},
	}
	x, fit := r.smartReframe(context.Background(), "source.mp4", 10, 40)
	if x != "" || !fit {
		t.Errorf("expected blur-fill fit for faceless content, got (%q, fit=%v)", x, fit)
	}
}

func TestSmartCropX_VerticalSourcePinsLeft(t *testing.T) {
	r := &Renderer{Detector: &stubDetector{}}
	// Probe reports vertical; no frames should even be sampled.
	r.ProbeDimensions = func(string) (int, int, error) { return 1080, 1920, nil }
	r.ExtractFrames = func(string, float64, float64) ([][]byte, error) {
		t.Fatal("must not sample frames for vertical sources")
		return nil, nil
	}
	if x, fit := r.smartReframe(context.Background(), "source.mp4", 0, 20); x != "0" || fit {
		t.Errorf("expected pinned 0, got (%q, fit=%v)", x, fit)
	}
}

// --- builder + Render plumbing ---

func TestBuildCropCaptionArgs_FitFill(t *testing.T) {
	r := &Renderer{Bucket: "b", WorkDir: "/tmp"}
	args := r.buildCropCaptionArgs("source.mp4", "cap.srt", "out.mp4", 12.4, 39.9, false, "", true)
	joined := strings.Join(args, " ")
	for _, want := range []string{"overlay=(W-w)/2:(H-h)/2", "gblur=", "force_original_aspect_ratio=decrease", "subtitles=cap.srt"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected fit-fill filter to contain %q, got: %s", want, joined)
		}
	}
	if strings.Contains(joined, "crop=w=ih*9/16") {
		t.Errorf("fit-fill must not crop, got: %s", joined)
	}
}

func TestBuildCropCaptionArgs_SmartCropOffset(t *testing.T) {
	r := &Renderer{Bucket: "b", WorkDir: "/tmp"}
	args := r.buildCropCaptionArgs("source.mp4", "cap.srt", "out.mp4", 12.4, 39.9, false, "656", false)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "crop=w=ih*9/16:h=ih:x=656:y=0") {
		t.Errorf("expected tracked crop x=656, got: %s", joined)
	}
	if strings.Contains(joined, "(iw-ih*9/16)/2") {
		t.Errorf("centered expression must be replaced when tracking, got: %s", joined)
	}
}

func TestRender_UsesTrackedCrop(t *testing.T) {
	workDir := t.TempDir()
	transcript := models.Transcript{
		Text:  "hello world",
		Words: []models.Word{{Text: "hello", Start: 10.0, End: 10.5}},
	}
	tData, _ := json.Marshal(transcript)
	store := &mockStorage{exists: false, objects: map[string][]byte{
		"uploads/u1/j1/source.mp4":     []byte("SOURCE-BYTES"),
		"shorts/u1/j1/transcript.json": tData,
	}}
	var vf string
	r := &Renderer{
		Storage:         store,
		Bucket:          "assets",
		WorkDir:         workDir,
		ProbeDimensions: func(string) (int, int, error) { return 1920, 1080, nil },
		ExtractFrames:   func(string, float64, float64) ([][]byte, error) { return [][]byte{[]byte("f")}, nil },
		Detector: &stubDetector{boxes: [][]FaceBox{
			{{Left: 0.3, Top: 0.2, Width: 0.2, Height: 0.3}},
		}},
		RunCommand: func(name string, args []string) error {
			if name != "ffmpeg" {
				t.Fatalf("expected ffmpeg, got %q", name)
			}
			vf = argValue(args, "-vf")
			return os.WriteFile(args[len(args)-1], []byte("RENDERED-CLIP"), 0o644)
		},
		RunCommandProgress: func(name string, args []string, duration float64, onProgress func(int)) error {
			vf = argValue(args, "-vf")
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
	if _, err := r.Render(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Face center 0.4*1920=768-303=465 -> 464 even.
	if !strings.Contains(vf, "x=464") {
		t.Errorf("expected tracked x=464 in ffmpeg filter, got: %s", vf)
	}
}

func TestRender_FacelessUsesFitFill(t *testing.T) {
	workDir := t.TempDir()
	transcript := models.Transcript{
		Text:  "hello world",
		Words: []models.Word{{Text: "hello", Start: 10.0, End: 10.5}},
	}
	tData, _ := json.Marshal(transcript)
	store := &mockStorage{exists: false, objects: map[string][]byte{
		"uploads/u1/j1/source.mp4":     []byte("SOURCE-BYTES"),
		"shorts/u1/j1/transcript.json": tData,
	}}
	var vf string
	r := &Renderer{
		Storage:         store,
		Bucket:          "assets",
		WorkDir:         workDir,
		ProbeDimensions: func(string) (int, int, error) { return 1920, 1080, nil },
		ExtractFrames:   func(string, float64, float64) ([][]byte, error) { return [][]byte{[]byte("f")}, nil },
		Detector:        &stubDetector{boxes: [][]FaceBox{{}}}, // cartoon/text: no faces
		RunCommand: func(name string, args []string) error {
			vf = argValue(args, "-vf")
			return os.WriteFile(args[len(args)-1], []byte("RENDERED-CLIP"), 0o644)
		},
		RunCommandProgress: func(name string, args []string, duration float64, onProgress func(int)) error {
			vf = argValue(args, "-vf")
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
	if _, err := r.Render(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(vf, "overlay=(W-w)/2:(H-h)/2") {
		t.Errorf("expected blur-fill fit for faceless content, got: %s", vf)
	}
}
