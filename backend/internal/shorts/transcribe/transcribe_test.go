package transcribe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transcribe"
	"github.com/aws/aws-sdk-go-v2/service/transcribe/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
)

// mockTranscribeClient returns a scripted sequence of job statuses on
// successive GetTranscriptionJob calls.
type mockTranscribeClient struct {
	started       bool
	startInput    *transcribe.StartTranscriptionJobInput
	startErr      error
	startErrs     []error // scripted per StartTranscriptionJob call
	startCalls    int
	deleted       []string
	statuses      []types.TranscriptionJobStatus
	failureReason string
	calls         int
}

func (m *mockTranscribeClient) StartTranscriptionJob(ctx context.Context, in *transcribe.StartTranscriptionJobInput, optFns ...func(*transcribe.Options)) (*transcribe.StartTranscriptionJobOutput, error) {
	m.started = true
	m.startInput = in
	err := m.startErr
	if m.startCalls < len(m.startErrs) {
		err = m.startErrs[m.startCalls]
	}
	m.startCalls++
	if err != nil {
		return nil, err
	}
	return &transcribe.StartTranscriptionJobOutput{}, nil
}

func (m *mockTranscribeClient) DeleteTranscriptionJob(ctx context.Context, in *transcribe.DeleteTranscriptionJobInput, optFns ...func(*transcribe.Options)) (*transcribe.DeleteTranscriptionJobOutput, error) {
	m.deleted = append(m.deleted, aws.ToString(in.TranscriptionJobName))
	return &transcribe.DeleteTranscriptionJobOutput{}, nil
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

func TestStart_SubmitsExplicitEnglishJob(t *testing.T) {
	client := &mockTranscribeClient{}
	svc := NewService(client, newMockStorage(), "my-bucket", "us-east-1")

	name, err := svc.Start(context.Background(), "job1", "user1", "uploads/user1/job1/source.mp4", shorts.LanguageEnglishUS)
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
	if client.startInput.IdentifyLanguage != nil {
		t.Error("explicit language must not enable IdentifyLanguage")
	}
	if got := aws.ToString(client.startInput.Media.MediaFileUri); got != "s3://my-bucket/uploads/user1/job1/source.mp4" {
		t.Errorf("unexpected media URI: %s", got)
	}
}

func TestStart_HindiUsesHiIN(t *testing.T) {
	client := &mockTranscribeClient{}
	svc := NewService(client, newMockStorage(), "my-bucket", "us-east-1")

	_, err := svc.Start(context.Background(), "job1", "user1", "uploads/user1/job1/source.mp4", "hi")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if client.startInput.LanguageCode != types.LanguageCodeHiIn {
		t.Errorf("expected hi-IN language code, got %s", client.startInput.LanguageCode)
	}
	if client.startInput.IdentifyLanguage != nil {
		t.Error("explicit Hindi must not enable IdentifyLanguage")
	}
	if len(client.startInput.LanguageOptions) != 0 {
		t.Errorf("explicit language must not set LanguageOptions, got %v", client.startInput.LanguageOptions)
	}
}

func TestStart_AutoDetectsSupportedLanguages(t *testing.T) {
	client := &mockTranscribeClient{}
	svc := NewService(client, newMockStorage(), "my-bucket", "us-east-1")

	_, err := svc.Start(context.Background(), "job1", "user1", "uploads/user1/job1/source.mp4", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if aws.ToBool(client.startInput.IdentifyLanguage) != true {
		t.Error("auto language selection must enable IdentifyLanguage")
	}
	if client.startInput.LanguageCode != "" {
		t.Errorf("auto language selection must not pin LanguageCode, got %s", client.startInput.LanguageCode)
	}
	want := []types.LanguageCode{types.LanguageCodeEnUs, types.LanguageCodeEnIn, types.LanguageCodeHiIn}
	if len(client.startInput.LanguageOptions) != len(want) {
		t.Fatalf("LanguageOptions = %v, want %v", client.startInput.LanguageOptions, want)
	}
	for i := range want {
		if client.startInput.LanguageOptions[i] != want[i] {
			t.Errorf("LanguageOptions[%d] = %s, want %s", i, client.startInput.LanguageOptions[i], want[i])
		}
	}
}

func TestStart_InvalidLanguageRejected(t *testing.T) {
	client := &mockTranscribeClient{}
	svc := NewService(client, newMockStorage(), "my-bucket", "us-east-1")

	_, err := svc.Start(context.Background(), "job1", "user1", "uploads/user1/job1/source.mp4", "klingon")
	if err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("expected unsupported-language error, got: %v", err)
	}
	if client.started {
		t.Fatal("invalid language must not start a Transcribe job")
	}
}

func TestNormalize_PreservesHindiTranscript(t *testing.T) {
	// Devanagari text and word timings must survive parsing byte-for-byte:
	// the caption pipeline renders transcript words directly, so normalizing
	// must never transliterate or drop non-Latin text.
	raw := []byte(`{
  "results": {
    "transcripts": [{"transcript": "नमस्ते दुनिया"}],
    "items": [
      {"type": "pronunciation", "start_time": "0.0", "end_time": "0.6", "alternatives": [{"content": "नमस्ते"}]},
      {"type": "pronunciation", "start_time": "0.7", "end_time": "1.2", "alternatives": [{"content": "दुनिया"}]},
      {"type": "punctuation", "alternatives": [{"content": "."}]}
    ]
  }
}`)

	tr, err := normalize(raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if tr.Text != "नमस्ते दुनिया" {
		t.Errorf("text = %q, want Hindi transcript unchanged", tr.Text)
	}
	if len(tr.Words) != 2 {
		t.Fatalf("expected 2 Hindi words, got %d: %+v", len(tr.Words), tr.Words)
	}
	if tr.Words[0].Text != "नमस्ते" || tr.Words[1].Text != "दुनिया" {
		t.Errorf("words changed during normalization: %+v", tr.Words)
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

func TestStart_ConflictResumesExistingJob(t *testing.T) {
	// A re-entry for the same job ID reports ConflictException: Start must
	// hand back the still-valid existing job for polling instead of failing.
	client := &mockTranscribeClient{
		startErr: &types.ConflictException{Message: aws.String("job exists")},
		statuses: []types.TranscriptionJobStatus{types.TranscriptionJobStatusCompleted},
	}
	svc := NewService(client, newMockStorage(), "assets", "ap-south-1")

	name, err := svc.Start(context.Background(), "job1", "user1", "uploads/user1/job1/source.mp4", shorts.LanguageEnglishUS)
	if err != nil {
		t.Fatalf("conflict must resume, got error: %v", err)
	}
	if name != "shorts-job1" {
		t.Errorf("expected existing job name shorts-job1, got %q", name)
	}
	if len(client.deleted) != 0 {
		t.Errorf("completed job must not be deleted, deleted %v", client.deleted)
	}
}

func TestStart_ReplacesStaleFailedJob(t *testing.T) {
	// A failed transcription under the deterministic name must be deleted so
	// a retry actually re-runs (and picks up the current language) instead of
	// replaying the stale failure forever.
	client := &mockTranscribeClient{
		startErrs: []error{
			&types.ConflictException{Message: aws.String("job exists")},
			nil,
		},
		statuses: []types.TranscriptionJobStatus{types.TranscriptionJobStatusFailed},
	}
	svc := NewService(client, newMockStorage(), "assets", "ap-south-1")

	name, err := svc.Start(context.Background(), "job1", "user1", "uploads/user1/job1/source.mp4", shorts.LanguageHindi)
	if err != nil {
		t.Fatalf("failed job must be replaced, got error: %v", err)
	}
	if name != "shorts-job1" {
		t.Errorf("expected job name shorts-job1, got %q", name)
	}
	if len(client.deleted) != 1 || client.deleted[0] != "shorts-job1" {
		t.Errorf("expected stale job deletion, got %v", client.deleted)
	}
	if client.startCalls != 2 {
		t.Errorf("expected a fresh start after deletion, got %d calls", client.startCalls)
	}
	if client.startInput.LanguageCode != types.LanguageCodeHiIn {
		t.Errorf("restarted job language = %s, want hi-IN", client.startInput.LanguageCode)
	}
}

func TestPoll_EmptyTranscriptFailsActionably(t *testing.T) {
	// Silence/music with no transcribable words must fail fast with guidance,
	// not burn rank/render work downstream.
	client := &mockTranscribeClient{statuses: []types.TranscriptionJobStatus{types.TranscriptionJobStatusCompleted}}
	store := newMockStorage()
	store.objects[rawOutputKey("user1", "job1")] = []byte(`{"results":{"transcripts":[{"transcript":""}],"items":[]}}`)
	svc := NewService(client, store, "my-bucket", "us-east-1")

	_, err := svc.Poll(context.Background(), "job1", "user1", "shorts-job1")
	if err == nil || !strings.Contains(err.Error(), "no speech detected") {
		t.Fatalf("expected no-speech error, got: %v", err)
	}
}

func TestPoll_OverlongTranscriptFailsActionably(t *testing.T) {
	// A transcript running far past the duration cap means the duration claim
	// was wrong: fail with the limit instead of rendering blindly.
	client := &mockTranscribeClient{statuses: []types.TranscriptionJobStatus{types.TranscriptionJobStatusCompleted}}
	store := newMockStorage()
	store.objects[rawOutputKey("user1", "job1")] = []byte(`{"results":{"transcripts":[{"transcript":"hi"}],"items":[{"type":"pronunciation","start_time":"0.0","end_time":"4000.0","alternatives":[{"content":"hi"}]}]}}`)
	svc := NewService(client, store, "my-bucket", "us-east-1")

	_, err := svc.Poll(context.Background(), "job1", "user1", "shorts-job1")
	if err == nil || !strings.Contains(err.Error(), "over the") {
		t.Fatalf("expected duration-limit error, got: %v", err)
	}
}
