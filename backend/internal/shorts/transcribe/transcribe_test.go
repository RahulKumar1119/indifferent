package transcribe

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transcribe"
	"github.com/aws/aws-sdk-go-v2/service/transcribe/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

// mockTranscribeClient returns a scripted sequence of job statuses on
// successive GetTranscriptionJob calls.
type mockTranscribeClient struct {
	started       bool
	startInput    *transcribe.StartTranscriptionJobInput
	statuses      []types.TranscriptionJobStatus
	failureReason string
	calls         int
}

func (m *mockTranscribeClient) StartTranscriptionJob(ctx context.Context, in *transcribe.StartTranscriptionJobInput, optFns ...func(*transcribe.Options)) (*transcribe.StartTranscriptionJobOutput, error) {
	m.started = true
	m.startInput = in
	return &transcribe.StartTranscriptionJobOutput{}, nil
}

func (m *mockTranscribeClient) GetTranscriptionJob(ctx context.Context, in *transcribe.GetTranscriptionJobInput, optFns ...func(*transcribe.Options)) (*transcribe.GetTranscriptionJobOutput, error) {
	status := m.statuses[m.calls]
	if m.calls < len(m.statuses)-1 {
		m.calls++
	}
	job := &types.TranscriptionJob{
		TranscriptionJobName:   in.TranscriptionJobName,
		TranscriptionJobStatus: status,
	}
	if status == types.TranscriptionJobStatusFailed && m.failureReason != "" {
		job.FailureReason = aws.String(m.failureReason)
	}
	return &transcribe.GetTranscriptionJobOutput{TranscriptionJob: job}, nil
}

// mockStorage records puts and serves canned gets keyed by S3 key.
type mockStorage struct {
	objects map[string][]byte
	puts    map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{objects: map[string][]byte{}, puts: map[string][]byte{}}
}

func (m *mockStorage) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	if b, ok := m.objects[key]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("object not found: %s", key)
}

func (m *mockStorage) PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	m.puts[key] = data
	return nil
}

func (m *mockStorage) DeleteObject(ctx context.Context, bucket, key string) error { return nil }

func (m *mockStorage) HeadObject(ctx context.Context, bucket, key string) (bool, error) {
	_, ok := m.objects[key]
	return ok, nil
}

// sampleTranscribeOutput is a minimal Amazon Transcribe output document.
const sampleTranscribeOutput = `{
  "results": {
    "transcripts": [{"transcript": "hello world"}],
    "items": [
      {"type": "pronunciation", "start_time": "0.0", "end_time": "0.5", "alternatives": [{"content": "hello"}]},
      {"type": "pronunciation", "start_time": "0.6", "end_time": "1.1", "alternatives": [{"content": "world"}]},
      {"type": "punctuation", "alternatives": [{"content": "."}]}
    ]
  }
}`

func TestStart_SubmitsEnUSJob(t *testing.T) {
	client := &mockTranscribeClient{}
	svc := NewService(client, newMockStorage(), "my-bucket", "us-east-1")

	name, err := svc.Start(context.Background(), "job1", "user1", "uploads/user1/job1/source.mp4")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !client.started {
		t.Fatal("StartTranscriptionJob was not called")
	}
	if name != "shorts-job1" {
		t.Errorf("unexpected job name: %s", name)
	}
	if client.startInput.LanguageCode != types.LanguageCodeEnUs {
		t.Errorf("expected en-US language code, got %s", client.startInput.LanguageCode)
	}
	if got := aws.ToString(client.startInput.Media.MediaFileUri); got != "s3://my-bucket/uploads/user1/job1/source.mp4" {
		t.Errorf("unexpected media URI: %s", got)
	}
}

func TestPoll_InProgressThenCompleted(t *testing.T) {
	client := &mockTranscribeClient{
		statuses: []types.TranscriptionJobStatus{
			types.TranscriptionJobStatusInProgress,
			types.TranscriptionJobStatusCompleted,
		},
	}
	store := newMockStorage()
	store.objects[rawOutputKey("user1", "job1")] = []byte(sampleTranscribeOutput)

	svc := NewService(client, store, "my-bucket", "us-east-1")

	// First poll: IN_PROGRESS.
	res, err := svc.Poll(context.Background(), "job1", "user1", "shorts-job1")
	if err != nil {
		t.Fatalf("first poll: %v", err)
	}
	if res.State != StateInProgress {
		t.Fatalf("expected IN_PROGRESS, got %s", res.State)
	}

	// Second poll: COMPLETED, transcript stored.
	res, err = svc.Poll(context.Background(), "job1", "user1", "shorts-job1")
	if err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if res.State != StateCompleted {
		t.Fatalf("expected COMPLETED, got %s", res.State)
	}
	wantKey := "shorts/user1/job1/transcript.json"
	if res.TranscriptKey != wantKey {
		t.Errorf("expected transcript key %s, got %s", wantKey, res.TranscriptKey)
	}
	stored, ok := store.puts[wantKey]
	if !ok {
		t.Fatalf("transcript not stored at %s", wantKey)
	}

	var tr models.Transcript
	if err := json.Unmarshal(stored, &tr); err != nil {
		t.Fatalf("unmarshal stored transcript: %v", err)
	}
	if tr.Text != "hello world" {
		t.Errorf("unexpected transcript text: %q", tr.Text)
	}
	if len(tr.Words) != 2 {
		t.Fatalf("expected 2 words (punctuation excluded), got %d: %+v", len(tr.Words), tr.Words)
	}
	if tr.Words[0].Text != "hello" || tr.Words[0].Start != 0.0 || tr.Words[0].End != 0.5 {
		t.Errorf("unexpected first word: %+v", tr.Words[0])
	}
	if tr.Words[1].Text != "world" || tr.Words[1].Start != 0.6 {
		t.Errorf("unexpected second word: %+v", tr.Words[1])
	}
}

func TestPoll_FailedSurfacesFailureReason(t *testing.T) {
	client := &mockTranscribeClient{
		statuses:      []types.TranscriptionJobStatus{types.TranscriptionJobStatusFailed},
		failureReason: "Unsupported media format",
	}
	svc := NewService(client, newMockStorage(), "my-bucket", "us-east-1")

	res, err := svc.Poll(context.Background(), "job1", "user1", "shorts-job1")
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.State != StateFailed {
		t.Fatalf("expected FAILED, got %s", res.State)
	}
	if res.FailureReason != "Unsupported media format" {
		t.Errorf("unexpected failure reason: %q", res.FailureReason)
	}
}
