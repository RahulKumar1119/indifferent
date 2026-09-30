// Package transcribe provides the Amazon Transcribe-backed transcription
// service for the AI Shorts pipeline. It submits async transcription jobs and
// polls them, normalizing the Transcribe output into models.Transcript.
package transcribe

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transcribe"
	"github.com/aws/aws-sdk-go-v2/service/transcribe/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// Poll state values returned by the service.
const (
	StateInProgress = "IN_PROGRESS"
	StateCompleted  = "COMPLETED"
	StateFailed     = "FAILED"
)

// TranscribeClient is the subset of the Amazon Transcribe API used by the
// service. Defining it as an interface allows testing with a mock client.
type TranscribeClient interface {
	StartTranscriptionJob(ctx context.Context, in *transcribe.StartTranscriptionJobInput, optFns ...func(*transcribe.Options)) (*transcribe.StartTranscriptionJobOutput, error)
	GetTranscriptionJob(ctx context.Context, in *transcribe.GetTranscriptionJobInput, optFns ...func(*transcribe.Options)) (*transcribe.GetTranscriptionJobOutput, error)
}

// Service submits and polls Amazon Transcribe jobs and stores the normalized
// transcript in S3.
type Service struct {
	Client TranscribeClient
	S3     storage.StorageClient
	Bucket string
	Region string
}

// NewService constructs a transcription Service.
func NewService(client TranscribeClient, s3 storage.StorageClient, bucket, region string) *Service {
	return &Service{Client: client, S3: s3, Bucket: bucket, Region: region}
}

// PollResult reports the current state of a transcription job.
type PollResult struct {
	State         string `json:"state"`         // IN_PROGRESS | COMPLETED | FAILED
	TranscriptKey string `json:"transcriptKey"` // set when COMPLETED
	FailureReason string `json:"failureReason"` // set when FAILED
}

// jobName derives a unique, deterministic Transcribe job name from the job ID.
// Transcribe job names must be unique within an account and cannot contain
// spaces.
func jobName(jobID string) string {
	return "shorts-" + jobID
}

// transcriptKey is the S3 key where the normalized transcript is stored.
func transcriptKey(userID, jobID string) string {
	return fmt.Sprintf("shorts/%s/%s/transcript.json", userID, jobID)
}

// rawOutputKey is where Transcribe writes its raw output in our bucket so we
// can GetObject it (via OutputBucketName/OutputKey).
func rawOutputKey(userID, jobID string) string {
	return fmt.Sprintf("shorts/%s/%s/transcribe-raw.json", userID, jobID)
}

// Start submits the source media for English transcription with word-level
// timestamps and returns the Transcribe job name. The Transcribe output is
// written to our own bucket (via OutputBucketName/OutputKey) so Poll can read
// it back with GetObject. (Requirements 2.1, 2.4)
func (s *Service) Start(ctx context.Context, jobID, userID, sourceKey string) (string, error) {
	name := jobName(jobID)
	mediaURI := fmt.Sprintf("s3://%s/%s", s.Bucket, sourceKey)

	_, err := s.Client.StartTranscriptionJob(ctx, &transcribe.StartTranscriptionJobInput{
		TranscriptionJobName: aws.String(name),
		LanguageCode:         types.LanguageCodeEnUs,
		Media: &types.Media{
			MediaFileUri: aws.String(mediaURI),
		},
		OutputBucketName: aws.String(s.Bucket),
		OutputKey:        aws.String(rawOutputKey(userID, jobID)),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start transcription job: %w", err)
	}
	return name, nil
}

// Poll returns the current state of the transcription job. When COMPLETED it
// downloads the Transcribe output, normalizes it to models.Transcript, stores
// it at the job's transcript key, and returns that key. When FAILED it returns
// the failure reason. (Requirements 2.2, 2.3, 2.5)
func (s *Service) Poll(ctx context.Context, jobID, userID, name string) (PollResult, error) {
	if name == "" {
		name = jobName(jobID)
	}

	out, err := s.Client.GetTranscriptionJob(ctx, &transcribe.GetTranscriptionJobInput{
		TranscriptionJobName: aws.String(name),
	})
	if err != nil {
		return PollResult{}, fmt.Errorf("failed to get transcription job: %w", err)
	}
	if out.TranscriptionJob == nil {
		return PollResult{}, fmt.Errorf("transcription job %s not found", name)
	}

	job := out.TranscriptionJob
	switch job.TranscriptionJobStatus {
	case types.TranscriptionJobStatusCompleted:
		key, err := s.storeTranscript(ctx, jobID, userID)
		if err != nil {
			return PollResult{}, err
		}
		return PollResult{State: StateCompleted, TranscriptKey: key}, nil

	case types.TranscriptionJobStatusFailed:
		reason := "transcription failed"
		if job.FailureReason != nil {
			reason = *job.FailureReason
		}
		return PollResult{State: StateFailed, FailureReason: reason}, nil

	default: // QUEUED or IN_PROGRESS
		return PollResult{State: StateInProgress}, nil
	}
}

// storeTranscript reads the raw Transcribe output from our bucket, normalizes
// it, and writes the normalized transcript to its S3 key.
func (s *Service) storeTranscript(ctx context.Context, jobID, userID string) (string, error) {
	rawKey := rawOutputKey(userID, jobID)
	raw, err := s.S3.GetObject(ctx, s.Bucket, rawKey)
	if err != nil {
		return "", fmt.Errorf("failed to read Transcribe output %s: %w", rawKey, err)
	}

	transcript, err := normalize(raw)
	if err != nil {
		return "", fmt.Errorf("failed to normalize transcript: %w", err)
	}

	data, err := json.Marshal(transcript)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transcript: %w", err)
	}

	key := transcriptKey(userID, jobID)
	if err := s.S3.PutObject(ctx, s.Bucket, key, data, "application/json"); err != nil {
		return "", fmt.Errorf("failed to store transcript %s: %w", key, err)
	}
	return key, nil
}

// transcribeOutput mirrors the Amazon Transcribe output JSON schema (the parts
// we consume).
type transcribeOutput struct {
	Results struct {
		Transcripts []struct {
			Transcript string `json:"transcript"`
		} `json:"transcripts"`
		Items []struct {
			Type         string  `json:"type"` // pronunciation | punctuation
			StartTime    string  `json:"start_time"`
			EndTime      string  `json:"end_time"`
			Alternatives []struct {
				Content string `json:"content"`
			} `json:"alternatives"`
		} `json:"items"`
	} `json:"results"`
}

// normalize converts an Amazon Transcribe output document into a
// models.Transcript, extracting only pronunciation items with start/end times.
func normalize(raw []byte) (models.Transcript, error) {
	var out transcribeOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return models.Transcript{}, err
	}

	var transcript models.Transcript
	if len(out.Results.Transcripts) > 0 {
		transcript.Text = out.Results.Transcripts[0].Transcript
	}

	for _, item := range out.Results.Items {
		if item.Type != "pronunciation" {
			continue
		}
		if len(item.Alternatives) == 0 {
			continue
		}
		start, err := strconv.ParseFloat(strings.TrimSpace(item.StartTime), 64)
		if err != nil {
			continue
		}
		end, err := strconv.ParseFloat(strings.TrimSpace(item.EndTime), 64)
		if err != nil {
			continue
		}
		transcript.Words = append(transcript.Words, models.Word{
			Text:  item.Alternatives[0].Content,
			Start: start,
			End:   end,
		})
	}

	return transcript, nil
}
