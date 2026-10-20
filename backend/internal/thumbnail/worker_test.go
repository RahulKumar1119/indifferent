package thumbnail

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	db "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type memObjects struct {
	mu        sync.Mutex
	objects   map[string][]byte
	downloads []string
	putErr    error
	dlErr     error
	deleted   []string
}

func newMemObjects() *memObjects { return &memObjects{objects: map[string][]byte{}} }

func (m *memObjects) DownloadToFile(_ context.Context, _, key, path string) error {
	m.downloads = append(m.downloads, key)
	if m.dlErr != nil {
		return m.dlErr
	}
	return os.WriteFile(path, []byte("video"), 0o644)
}

func (m *memObjects) GetObject(_ context.Context, _, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, errors.New("NoSuchKey")
	}
	return b, nil
}

func (m *memObjects) PutObject(_ context.Context, _, key string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.putErr != nil {
		return m.putErr
	}
	m.objects[key] = data
	return nil
}

func (m *memObjects) DeleteObjects(_ context.Context, _ string, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.objects, k)
		m.deleted = append(m.deleted, k)
	}
	return nil
}

func (m *memObjects) ListKeys(_ context.Context, _, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out, nil
}

type fakePipeline struct {
	out    *Output
	err    error
	stages []Stage
	in     Input
	hook   func(progress func(Stage))
}

func (f *fakePipeline) Run(_ context.Context, in Input, progress func(Stage)) (*Output, error) {
	f.in = in
	for _, s := range []Stage{StageExtracting, StageSelecting, StageAnalyzing, StageConcepts, StageRendering} {
		progress(s)
		f.stages = append(f.stages, s)
	}
	if f.hook != nil {
		f.hook(progress)
	}
	return f.out, f.err
}

func okOutput() *Output {
	return &Output{
		Images: []Image{
			{Concept: ConceptFace, JPEG: []byte("j1"), Width: 1280, Height: 720, FrameTime: 5, Headline: "HI", SecondaryText: "THERE"},
			{Concept: ConceptSubject, JPEG: []byte("j2"), Width: 1280, Height: 720, FrameTime: 15, Headline: "HI"},
			{Concept: ConceptCreative, JPEG: []byte("j3"), Width: 1280, Height: 720, FrameTime: 25, Headline: "HI"},
		},
		Analysis: Analysis{Headline: "HI"}, Style: StyleViral, Format: FormatLandscape,
		TranscriptUsed: true, DetectionUsed: true,
	}
}

func newWorker(f *fakeDB, o *memObjects, p *fakePipeline) *Worker {
	return &Worker{
		Store: newStore(f), Objects: o, Pipeline: p, Bucket: "bkt", WorkRoot: os.TempDir(),
		JobExists: func(context.Context, string, string) (bool, error) { return true, nil },
	}
}

func beginOut(gen string) *dynamodb.UpdateItemOutput {
	return &dynamodb.UpdateItemOutput{Attributes: map[string]db.AttributeValue{"generation": &db.AttributeValueMemberN{Value: gen}}}
}

// seqDB lets different calls return different results, in the order made.
type seqDB struct {
	*fakeDB
	results []error
	outs    []*dynamodb.UpdateItemOutput
	n       int
}

func (s *seqDB) UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, o ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	s.fakeDB.updates = append(s.fakeDB.updates, in)
	i := s.n
	s.n++
	var err error
	if i < len(s.results) {
		err = s.results[i]
	}
	if err != nil {
		return nil, err
	}
	if i < len(s.outs) && s.outs[i] != nil {
		return s.outs[i], nil
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func TestWorkerHappyPath(t *testing.T) {
	f := &fakeDB{updOut: beginOut("2")}
	o := newMemObjects()
	// A previous generation's image and the transcript.
	o.objects["shorts/u1/j1/thumbnails/g1/thumbnail-01.jpg"] = []byte("old")
	o.objects["shorts/u1/j1/transcript.json"] = []byte(`{"text":"hello there world","words":[]}`)
	p := &fakePipeline{out: okOutput()}
	w := newWorker(f, o, p)
	w.Brand = func(context.Context, string, string) string { return "My Channel" }

	err := w.Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "uploads/u1/j1/source.mp4", TranscriptKey: "shorts/u1/j1/transcript.json", Duration: 90})
	if err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 3; n++ {
		key := ObjectKey("u1", "j1", 2, n)
		if string(o.objects[key]) != "j"+string(rune('0'+n)) {
			t.Errorf("object %s = %q", key, o.objects[key])
		}
	}
	if _, stale := o.objects["shorts/u1/j1/thumbnails/g1/thumbnail-01.jpg"]; stale {
		t.Error("the previous generation should be removed once the new one is saved")
	}
	if _, kept := o.objects["shorts/u1/j1/transcript.json"]; !kept {
		t.Error("pruning must stay inside the thumbnails folder")
	}
	if p.in.Transcript != "hello there world" || p.in.Brand != "My Channel" || p.in.Duration != 90 {
		t.Errorf("pipeline input = %+v", p.in)
	}
	if len(p.stages) != 5 {
		t.Errorf("stages = %v", p.stages)
	}
	// begin + 5 stages + complete
	last := f.updates[len(f.updates)-1]
	if !strings.Contains(*last.UpdateExpression, ":completed") {
		t.Errorf("last update should complete the record: %s", *last.UpdateExpression)
	}
}

func TestWorkerPassesRequestedStyleAndFormat(t *testing.T) {
	f := &fakeDB{updOut: &dynamodb.UpdateItemOutput{Attributes: map[string]db.AttributeValue{
		"generation": &db.AttributeValueMemberN{Value: "1"}, "reqStyle": str("news"), "reqFormat": str("9:16"),
	}}}
	p := &fakePipeline{out: okOutput()}
	if err := newWorker(f, newMemObjects(), p).Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"}); err != nil {
		t.Fatal(err)
	}
	if p.in.Style != StyleNews || p.in.Format != FormatPortrait {
		t.Errorf("style=%q format=%q", p.in.Style, p.in.Format)
	}
}

func TestWorkerRecordsPipelineFailureAndExitsCleanly(t *testing.T) {
	f := &fakeDB{updOut: beginOut("1")}
	p := &fakePipeline{err: Errorf(CodeNoSuitableFrames, "Every frame was too dark.", nil)}
	err := newWorker(f, newMemObjects(), p).Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"})
	if err != nil {
		t.Fatalf("a recorded failure must not fail the task: %v", err)
	}
	last := f.updates[len(f.updates)-1]
	if got := last.ExpressionAttributeValues[":code"].(*db.AttributeValueMemberS).Value; got != CodeNoSuitableFrames {
		t.Errorf("recorded code = %q", got)
	}
	if got := last.ExpressionAttributeValues[":msg"].(*db.AttributeValueMemberS).Value; got != "Every frame was too dark." {
		t.Errorf("recorded message = %q (user-facing text only)", got)
	}
}

func TestWorkerRecordsDownloadFailure(t *testing.T) {
	f := &fakeDB{updOut: beginOut("1")}
	o := newMemObjects()
	o.dlErr = errors.New("NoSuchKey: secret/internal/path")
	err := newWorker(f, o, &fakePipeline{out: okOutput()}).Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	last := f.updates[len(f.updates)-1]
	if got := last.ExpressionAttributeValues[":code"].(*db.AttributeValueMemberS).Value; got != CodeSourceUnreadable {
		t.Errorf("code = %q", got)
	}
	if msg := last.ExpressionAttributeValues[":msg"].(*db.AttributeValueMemberS).Value; strings.Contains(msg, "secret") {
		t.Errorf("internal detail leaked into the user message: %q", msg)
	}
}

func TestWorkerContinuesWithoutATranscript(t *testing.T) {
	f := &fakeDB{updOut: beginOut("1")}
	p := &fakePipeline{out: okOutput()}
	err := newWorker(f, newMemObjects(), p).Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4", TranscriptKey: "missing.json"})
	if err != nil {
		t.Fatal(err)
	}
	if p.in.Transcript != "" {
		t.Errorf("transcript = %q", p.in.Transcript)
	}
}

func TestWorkerBuildsTranscriptFromWords(t *testing.T) {
	o := newMemObjects()
	o.objects["t.json"] = []byte(`{"words":[{"text":"hello","start":0,"end":1},{"text":"world","start":1,"end":2}]}`)
	w := &Worker{Objects: o, Bucket: "b"}
	if got := w.loadTranscript(context.Background(), Job{TranscriptKey: "t.json"}); got != "hello world" {
		t.Errorf("transcript = %q", got)
	}
	o.objects["bad.json"] = []byte("nope")
	if got := w.loadTranscript(context.Background(), Job{TranscriptKey: "bad.json"}); got != "" {
		t.Errorf("unreadable transcript should be empty, got %q", got)
	}
}

func TestWorkerSkipsADeletedJob(t *testing.T) {
	f := &fakeDB{}
	w := newWorker(f, newMemObjects(), &fakePipeline{out: okOutput()})
	w.JobExists = func(context.Context, string, string) (bool, error) { return false, nil }
	if err := w.Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"}); err != nil {
		t.Fatal(err)
	}
	if len(f.updates) != 0 {
		t.Error("a deleted job must not get a new thumbnail record")
	}
}

func TestWorkerExitsWhenAnotherRunIsActive(t *testing.T) {
	f := &fakeDB{updErr: condFailed()}
	o := newMemObjects()
	if err := newWorker(f, o, &fakePipeline{out: okOutput()}).Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"}); err != nil {
		t.Fatal(err)
	}
	if len(o.downloads) != 0 {
		t.Error("the duplicate task must not download the video")
	}
}

func TestWorkerSurfacesUnrecordableFailures(t *testing.T) {
	// If even Begin cannot reach DynamoDB the task must fail loudly.
	f := &fakeDB{updErr: errors.New("dynamodb unreachable")}
	err := newWorker(f, newMemObjects(), &fakePipeline{out: okOutput()}).Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"})
	if err == nil {
		t.Error("expected an error")
	}
}

func TestWorkerStopsAndCleansUpWhenTheRecordIsRemoved(t *testing.T) {
	// begin ok; first stage write finds the record gone.
	s := &seqDB{fakeDB: &fakeDB{}, results: []error{nil, condFailed()}, outs: []*dynamodb.UpdateItemOutput{beginOut("1")}}
	o := newMemObjects()
	p := &fakePipeline{out: okOutput()}
	w := newWorker(s.fakeDB, o, p)
	w.Store = &Store{DB: &seqDBAdapter{s}, Table: "t", Now: func() time.Time { return fixedNow }}
	if err := w.Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"}); err != nil {
		t.Fatal(err)
	}
	if len(o.objects) != 0 {
		t.Errorf("nothing should be uploaded for a removed job, got %d objects", len(o.objects))
	}
}

type seqDBAdapter struct{ s *seqDB }

func (a *seqDBAdapter) GetItem(ctx context.Context, in *dynamodb.GetItemInput, o ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return a.s.fakeDB.GetItem(ctx, in, o...)
}
func (a *seqDBAdapter) UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, o ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return a.s.UpdateItem(ctx, in, o...)
}
func (a *seqDBAdapter) DeleteItem(ctx context.Context, in *dynamodb.DeleteItemInput, o ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	return a.s.fakeDB.DeleteItem(ctx, in, o...)
}

func TestWorkerRemovesUploadsIfTheRecordWasDeletedAtTheEnd(t *testing.T) {
	// begin ok, five stage writes ok, complete finds the record gone.
	s := &seqDB{fakeDB: &fakeDB{}, results: []error{nil, nil, nil, nil, nil, nil, condFailed()}, outs: []*dynamodb.UpdateItemOutput{beginOut("1")}}
	o := newMemObjects()
	w := newWorker(s.fakeDB, o, &fakePipeline{out: okOutput()})
	w.Store = &Store{DB: &seqDBAdapter{s}, Table: "t", Now: func() time.Time { return fixedNow }}
	if err := w.Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"}); err != nil {
		t.Fatal(err)
	}
	if len(o.objects) != 0 {
		t.Errorf("uploaded thumbnails for a deleted job must be removed, %d remain", len(o.objects))
	}
}

func TestWorkerRecordsUploadFailureAndCleansUp(t *testing.T) {
	f := &fakeDB{updOut: beginOut("1")}
	o := newMemObjects()
	o.putErr = errors.New("AccessDenied")
	if err := newWorker(f, o, &fakePipeline{out: okOutput()}).Run(context.Background(), Job{JobID: "j1", UserID: "u1", SourceKey: "k.mp4"}); err != nil {
		t.Fatal(err)
	}
	last := f.updates[len(f.updates)-1]
	if got := last.ExpressionAttributeValues[":code"].(*db.AttributeValueMemberS).Value; got != CodeRenderFailed {
		t.Errorf("code = %q", got)
	}
}

func TestSafeExt(t *testing.T) {
	for in, want := range map[string]string{"a/b.MP4": ".mp4", "x.mov": ".mov", "x.exe": ".bin", "noext": ".bin", "a.mp3": ".mp3"} {
		if got := safeExt(in); got != want {
			t.Errorf("safeExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestServicePipelineDoesNotLeakProgress(t *testing.T) {
	var a, b []Stage
	p := ServicePipeline{Service: Service{Analyzer: &fakeAnalyzer{err: Errorf(CodeAIFailed, "x", nil)}}}
	needTool(t, "ffmpeg", "ffprobe")
	video := makeVideo(t, "320x180", false)
	_, _ = p.Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir()}, func(s Stage) { a = append(a, s) })
	_, _ = p.Run(context.Background(), Input{VideoPath: video, WorkDir: t.TempDir()}, func(s Stage) { b = append(b, s) })
	if len(a) == 0 || len(a) != len(b) {
		t.Errorf("each run should report its own stages: %v vs %v", a, b)
	}
	if p.Service.Progress != nil {
		t.Error("the shared service must not keep a callback")
	}
}
