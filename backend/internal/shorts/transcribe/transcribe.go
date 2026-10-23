// Package transcribe provides the Amazon Transcribe-backed transcription
// service for the AI Shorts pipeline. It submits async transcription jobs and
// polls them, normalizing the Transcribe output into models.Transcript.
package transcribe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transcribe"
	"github.com/aws/aws-sdk-go-v2/service/transcribe/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
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
	DeleteTranscriptionJob(ctx context.Context, in *transcribe.DeleteTranscriptionJobInput, optFns ...func(*transcribe.Options)) (*transcribe.DeleteTranscriptionJobOutput, error)
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

// Start submits the source media for transcription with word-level timestamps
// and returns the Transcribe job name. The Transcribe output is written to our
// own bucket (via OutputBucketName/OutputKey) so Poll can read it back with
// GetObject. (Requirements 2.1, 2.4)
//
// language is a canonical language code from shorts.NormalizeLanguage: an
// explicit code pins the spoken language (Hindi must use hi-IN, never the
// en-US default), while "auto" asks Transcribe to identify it from the
// supported languages. Job names are deterministic ("shorts-<jobID>"), so a
// retry or re-entry reports ConflictException instead of starting a duplicate:
// completed/in-flight jobs are reused for polling, while a previously failed
// job is replaced so retries actually re-run.
func (s *Service) Start(ctx context.Context, jobID, userID, sourceKey, language string) (string, error) {
	lang, err := shorts.NormalizeLanguage(language)
	if err != nil {
		log.Printf("transcribe_start_invalid_language job=%s language=%q err=%v", jobID, language, err)
		return "", err
	}

	name := jobName(jobID)
	in := s.startInput(name, userID, jobID, sourceKey, lang)
	startErr := s.start(ctx, in)
	if startErr == nil {
		log.Printf("transcribe_started job=%s language=%s", jobID, lang)
		return name, nil
	}

	var conflict *types.ConflictException
	if !errors.As(startErr, &conflict) {
		log.Printf("transcribe_start_failed job=%s language=%s err=%v", jobID, lang, startErr)
		return "", fmt.Errorf("failed to start transcription job (%s): %w", lang, startErr)
	}

	log.Printf("transcribe_start_conflict job=%s language=%s", jobID, lang)
	out, err := s.Client.GetTranscriptionJob(ctx, &transcribe.GetTranscriptionJobInput{
		TranscriptionJobName: aws.String(name),
	})
	if err != nil {
		return "", fmt.Errorf("failed to inspect existing transcription job %s: %w", name, err)
	}
	if out.TranscriptionJob == nil {
		return "", fmt.Errorf("transcription job %s not found after conflict", name)
	}

	switch out.TranscriptionJob.TranscriptionJobStatus {
	case types.TranscriptionJobStatusCompleted, types.TranscriptionJobStatusInProgress, types.TranscriptionJobStatusQueued:
		log.Printf("transcribe_reuse job=%s language=%s status=%s", jobID, lang, out.TranscriptionJob.TranscriptionJobStatus)
		return name, nil
	case types.TranscriptionJobStatusFailed:
		// A deterministic job name would otherwise pin the failure forever:
		// delete the stale job so this retry runs with the current language.
		if _, err := s.Client.DeleteTranscriptionJob(ctx, &transcribe.DeleteTranscriptionJobInput{
			TranscriptionJobName: aws.String(name),
		}); err != nil {
			log.Printf("transcribe_delete_failed job=%s language=%s err=%v", jobID, lang, err)
			return "", fmt.Errorf("failed to replace failed transcription job %s: %w", name, err)
		}
		if err := s.start(ctx, in); err != nil {
			log.Printf("transcribe_restart_failed job=%s language=%s err=%v", jobID, lang, err)
			return "", fmt.Errorf("failed to restart transcription job (%s): %w", lang, err)
		}
		log.Printf("transcribe_restarted job=%s language=%s", jobID, lang)
		return name, nil
	default:
		return name, nil
	}
}

// startInput builds the StartTranscriptionJob request. An explicit language
// uses LanguageCode; auto uses IdentifyLanguage restricted to the languages
// the product supports (IdentifyMultipleLanguages is deliberately not used:
// it produces per-language transcripts instead of one identified language).
func (s *Service) startInput(name, userID, jobID, sourceKey, language string) *transcribe.StartTranscriptionJobInput {
	in := &transcribe.StartTranscriptionJobInput{
		TranscriptionJobName: aws.String(name),
		Media: &types.Media{
			MediaFileUri: aws.String(fmt.Sprintf("s3://%s/%s", s.Bucket, sourceKey)),
		},
		OutputBucketName: aws.String(s.Bucket),
		OutputKey:        aws.String(rawOutputKey(userID, jobID)),
	}
	if language == shorts.LanguageAuto {
		in.IdentifyLanguage = aws.Bool(true)
		in.LanguageOptions = transcribeLanguageCodes(shorts.SupportedLanguageCodes)
		return in
	}
	in.LanguageCode = transcribeLanguageCode(language)
	return in
}

// start calls StartTranscriptionJob, kept separate so Start can retry after
// deleting a stale failed job.
func (s *Service) start(ctx context.Context, in *transcribe.StartTranscriptionJobInput) error {
	_, err := s.Client.StartTranscriptionJob(ctx, in)
	return err
}

// transcribeLanguageCode maps a canonical product language to the Transcribe
// enum. NormalizeLanguage has already validated the value.
func transcribeLanguageCode(language string) types.LanguageCode {
	switch language {
	case shorts.LanguageHindi:
		return types.LanguageCodeHiIn
	case shorts.LanguageEnglishIN:
		return types.LanguageCodeEnIn
	default:
		return types.LanguageCodeEnUs
	}
}

func transcribeLanguageCodes(codes []string) []types.LanguageCode {
	out := make([]types.LanguageCode, 0, len(codes))
	for _, code := range codes {
		out = append(out, transcribeLanguageCode(code))
	}
	return out
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
	log.Printf("transcribe_poll job=%s status=%s language=%s identifiedScore=%.2f",
		jobID, job.TranscriptionJobStatus, job.LanguageCode, aws.ToFloat32(job.IdentifiedLanguageScore))
	switch job.TranscriptionJobStatus {
	case types.TranscriptionJobStatusCompleted:
		key, err := s.storeTranscript(ctx, jobID, userID)
		if err != nil {
			log.Printf("transcribe_store_failed job=%s language=%s err=%v", jobID, job.LanguageCode, err)
			return PollResult{}, err
		}
		log.Printf("transcribe_completed job=%s language=%s transcript=%s", jobID, job.LanguageCode, key)
		return PollResult{State: StateCompleted, TranscriptKey: key}, nil

	case types.TranscriptionJobStatusFailed:
		reason := "transcription failed"
		if job.FailureReason != nil {
			reason = *job.FailureReason
		}
		log.Printf("transcribe_failed job=%s language=%s reason=%s", jobID, job.LanguageCode, reason)
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

	// Server-side content validation (the client-supplied duration is only
	// advisory): an empty transcript means corrupt, silent, or unintelligible
	// media, and a transcript running far past the cap means the duration
	// claim was wrong. Both fail fast with actionable reasons instead of
	// burning rank/render work on garbage.
	if len(transcript.Words) == 0 {
		return "", fmt.Errorf("no speech detected: the media has no transcribable words — try a file with clear speech, or check the file isn't corrupt")
	}
	if lastEnd := transcript.Words[len(transcript.Words)-1].End; lastEnd > shorts.MaxSourceDuration+60 {
		return "", fmt.Errorf("media runs about %.0fs, over the %.0f minute limit - please trim it under %d minutes", lastEnd, shorts.MaxSourceDuration/60, int(shorts.MaxSourceDuration/60))
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
			Type         string `json:"type"` // pronunciation | punctuation
			StartTime    string `json:"start_time"`
			EndTime      string `json:"end_time"`
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
