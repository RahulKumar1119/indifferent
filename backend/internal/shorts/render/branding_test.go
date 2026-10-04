package render

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

type memDynamo struct {
	items map[string]map[string]dbtypes.AttributeValue
	err   error
}

func (m *memDynamo) GetItem(ctx context.Context, in *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	pk, _ := in.Key["PK"].(*dbtypes.AttributeValueMemberS)
	sk, _ := in.Key["SK"].(*dbtypes.AttributeValueMemberS)
	if item, ok := m.items[pk.Value+"|"+sk.Value]; ok {
		return &dynamodb.GetItemOutput{Item: item}, nil
	}
	return &dynamodb.GetItemOutput{}, nil
}

func sAttr(v string) *dbtypes.AttributeValueMemberS {
	return &dbtypes.AttributeValueMemberS{Value: v}
}

func jobItem(projectID string) map[string]dbtypes.AttributeValue {
	item := map[string]dbtypes.AttributeValue{}
	if projectID != "" {
		item["projectId"] = sAttr(projectID)
	}
	return item
}

func projectItem(branding string) map[string]dbtypes.AttributeValue {
	item := map[string]dbtypes.AttributeValue{}
	if branding != "" {
		item["branding"] = sAttr(branding)
	}
	return item
}

func TestLogoFetcher_UnlinkedJobSkips(t *testing.T) {
	db := &memDynamo{items: map[string]map[string]dbtypes.AttributeValue{
		"USER#u1|SHORTS#j1": jobItem(""),
	}}
	store := &mockStorage{objects: map[string][]byte{}}
	fetch := NewLogoFetcher(db, "table", "u1", "j1", store, "b")

	logo, err := fetch(context.Background())
	if err != nil || logo != nil {
		t.Fatalf("expected (nil, nil) for unlinked job, got %v, %v", logo, err)
	}
	if store.getCalls != 0 {
		t.Errorf("expected no S3 calls, got %d", store.getCalls)
	}
}

func TestLogoFetcher_LinkedLogo(t *testing.T) {
	db := &memDynamo{items: map[string]map[string]dbtypes.AttributeValue{
		"USER#u1|SHORTS#j1": jobItem("p1"),
		"USER#u1|PROJECT#p1": projectItem(`{"logoKey":"uploads/u1/p1/logo.png","channelName":"@s"}`),
	}}
	store := &mockStorage{objects: map[string][]byte{"uploads/u1/p1/logo.png": []byte("LOGO")}}
	fetch := NewLogoFetcher(db, "table", "u1", "j1", store, "b")

	logo, err := fetch(context.Background())
	if err != nil || string(logo) != "LOGO" {
		t.Fatalf("expected logo bytes, got %q, %v", logo, err)
	}
}

func TestLogoFetcher_NoBrandingSkips(t *testing.T) {
	db := &memDynamo{items: map[string]map[string]dbtypes.AttributeValue{
		"USER#u1|SHORTS#j1":  jobItem("p1"),
		"USER#u1|PROJECT#p1": projectItem(""),
	}}
	store := &mockStorage{objects: map[string][]byte{}}
	fetch := NewLogoFetcher(db, "table", "u1", "j1", store, "b")

	logo, err := fetch(context.Background())
	if err != nil || logo != nil {
		t.Fatalf("expected (nil, nil) without branding, got %v, %v", logo, err)
	}
	if store.getCalls != 0 {
		t.Errorf("expected no S3 calls, got %d", store.getCalls)
	}
}

func TestLogoFetcher_ErrorsPropagate(t *testing.T) {
	db := &memDynamo{err: errors.New("dynamo down")}
	store := &mockStorage{objects: map[string][]byte{}}
	fetch := NewLogoFetcher(db, "table", "u1", "j1", store, "b")

	if _, err := fetch(context.Background()); err == nil {
		t.Fatal("expected lookup error to propagate")
	}
}

func TestBuildLogoOverlayArgs_Shorts(t *testing.T) {	args := buildLogoOverlayArgs("clip.mp4", "logo.png", "branded.mp4")
	joined := strings.Join(args, " ")
	for _, want := range []string{"scale=200:-1", "overlay=W-w-24:24", "-c:a copy", "branded.mp4"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in args, got: %s", want, joined)
		}
	}
}

func TestRender_BrandedClipOverlaysLogo(t *testing.T) {
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
	var calls [][]string
	r := &Renderer{
		Storage:   store,
		Bucket:    "assets",
		WorkDir:   workDir,
		FetchLogo: func(ctx context.Context) ([]byte, error) { return []byte("LOGO"), nil },
		RunCommand: func(name string, args []string) error {
			calls = append(calls, args)
			return os.WriteFile(args[len(args)-1], []byte("RENDERED"), 0o644)
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
	if key != "shorts/u1/j1/clips/c1.mp4" {
		t.Errorf("unexpected key %q", key)
	}
	if len(calls) != 2 {
		t.Fatalf("expected render + overlay calls, got %d", len(calls))
	}
	if fc := argValue(calls[1], "-filter_complex"); !strings.Contains(fc, "overlay=W-w-24:24") {
		t.Errorf("expected logo overlay in second ffmpeg call, got: %s", fc)
	}
	if got := string(store.putData); got != "RENDERED" {
		t.Errorf("expected branded output uploaded, got %q", got)
	}
}

func TestRender_LogoFailureStillRenders(t *testing.T) {
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
	var calls int
	r := &Renderer{
		Storage:   store,
		Bucket:    "assets",
		WorkDir:   workDir,
		FetchLogo: func(ctx context.Context) ([]byte, error) { return nil, errors.New("dynamo down") },
		RunCommand: func(name string, args []string) error {
			calls++
			return os.WriteFile(args[len(args)-1], []byte("RENDERED"), 0o644)
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
		t.Fatalf("logo failure must not fail render: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected only the render call, got %d", calls)
	}
}
