// Package main is the entry point for the shorts-transcribe Lambda function.
// A single function serves both Step Functions states by branching on the
// event "mode" field: "start" submits an Amazon Transcribe job and "poll"
// checks its status (downloading and normalizing the transcript on completion).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	transcribesdk "github.com/aws/aws-sdk-go-v2/service/transcribe"
	transcribesvc "github.com/rahul/indifferent/backend/internal/shorts/transcribe"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// TranscribeEvent is the Step Functions input for the transcription stage.
type TranscribeEvent struct {
	Mode      string `json:"mode"` // "start" | "poll"
	JobID     string `json:"jobId"`
	UserID    string `json:"userId"`
	SourceKey string `json:"sourceKey"`
	JobName   string `json:"jobName"`
}

// TranscribeOutput is returned to Step Functions. For "start" only JobName is
// set; for "poll" the embedded PollResult fields are set.
type TranscribeOutput struct {
	JobName       string `json:"jobName,omitempty"`
	State         string `json:"state,omitempty"`
	TranscriptKey string `json:"transcriptKey,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
}

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, event TranscribeEvent) (TranscribeOutput, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return TranscribeOutput{}, fmt.Errorf("S3_BUCKET environment variable not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return TranscribeOutput{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return TranscribeOutput{}, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	client := transcribesdk.NewFromConfig(cfg)
	svc := transcribesvc.NewService(client, s3Client, bucket, cfg.Region)

	switch event.Mode {
	case "start":
		name, err := svc.Start(ctx, event.JobID, event.UserID, event.SourceKey)
		if err != nil {
			return TranscribeOutput{}, err
		}
		return TranscribeOutput{JobName: name}, nil

	case "poll":
		res, err := svc.Poll(ctx, event.JobID, event.UserID, event.JobName)
		if err != nil {
			return TranscribeOutput{}, err
		}
		return TranscribeOutput{
			State:         res.State,
			TranscriptKey: res.TranscriptKey,
			FailureReason: res.FailureReason,
		}, nil

	default:
		return TranscribeOutput{}, fmt.Errorf("unknown mode %q (expected \"start\" or \"poll\")", event.Mode)
	}
}
