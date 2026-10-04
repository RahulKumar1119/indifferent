package render

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

func TestProgressPercentParsing(t *testing.T) {
	for _, tc := range []struct {
		line     string
		duration float64
		want     int
	}{
		{"out_time_ms=15000000", 30, 50},
		{"out_time_ms=0", 30, 0},
		{"out_time_ms=30000000", 30, 99}, // clamped, never 100
		{"out_time_us=15000000", 30, -1}, // wrong key ignored
		{"progress=continue", 30, -1},
		{"out_time_ms=abc", 30, -1},
		{"out_time_ms=-5", 30, -1},
		{"out_time_ms=15000000", 0, -1}, // no duration, no progress
	} {
		if got := progressPercent(tc.line, tc.duration); got != tc.want {
			t.Errorf("progressPercent(%q, %v) = %d, want %d", tc.line, tc.duration, got, tc.want)
		}
	}
}

func TestRender_ReportsProgress(t *testing.T) {
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
	var reported []int
	r := &Renderer{
		Storage: store,
		Bucket:  "assets",
		WorkDir: workDir,
		RunCommandProgress: func(name string, args []string, duration float64, onProgress func(int)) error {
			if duration != 18.0 {
				t.Errorf("expected duration 18, got %v", duration)
			}
			onProgress(10)
			onProgress(30)
			return os.WriteFile(args[len(args)-1], []byte("RENDERED"), 0o644)
		},
		RunCommand: func(name string, args []string) error {
			return os.WriteFile(args[len(args)-1], []byte("RENDERED"), 0o644)
		},
		ProgressReporter: func(ctx context.Context, in RenderInput, percent int) error {
			reported = append(reported, percent)
			return nil
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
	if len(reported) != 2 || reported[0] != 10 || reported[1] != 30 {
		t.Errorf("expected reporter to receive [10 30], got %v", reported)
	}
}

func TestReportProgress_WritesPercent(t *testing.T) {
	db := &memDynamo{items: map[string]map[string]dbtypes.AttributeValue{}}
	r := &Renderer{Table: "jobs", Dynamo: db}
	in := RenderInput{JobID: "j1", UserID: "u1", Clip: models.Clip{ClipID: "c1"}}
	r.reportProgress(context.Background(), in, 42)

	item, ok := db.items["USER#u1|SHORTS#j1"]
	if !ok {
		t.Fatal("expected job progress item written")
	}
	n, ok := item["progress"].(*dbtypes.AttributeValueMemberN)
	if !ok || n.Value != "42" {
		t.Errorf("expected progress 42, got %+v", item["progress"])
	}
}
