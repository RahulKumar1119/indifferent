// Package main is the entry point for the shorts-rank Lambda function.
// It reads the transcript JSON from S3, prompts a Bedrock model for candidate
// viral moments, validates the response, and returns the validated clips to
// the Step Functions state machine.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/rank"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// RankEvent is the Step Functions input for the ranking stage.
type RankEvent struct {
	JobID          string  `json:"jobId"`
	UserID         string  `json:"userId"`
	SourceKey      string  `json:"sourceKey"`
	TranscriptKey  string  `json:"transcriptKey"`
	SourceDuration float64 `json:"sourceDuration"`
}

// RankOutput is returned to Step Functions.
type RankOutput struct {
	Clips []models.RankedSegment `json:"clips"`
}

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, event RankEvent) (RankOutput, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return RankOutput{}, fmt.Errorf("S3_BUCKET environment variable not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return RankOutput{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return RankOutput{}, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Read and parse the transcript.
	data, err := s3Client.GetObject(ctx, bucket, event.TranscriptKey)
	if err != nil {
		return RankOutput{}, fmt.Errorf("failed to read transcript %s: %w", event.TranscriptKey, err)
	}
	var transcript models.Transcript
	if err := json.Unmarshal(data, &transcript); err != nil {
		return RankOutput{}, fmt.Errorf("failed to parse transcript: %w", err)
	}

	// Build the ranking service from configured model.
	model := rank.ModelFromString(os.Getenv("RANKING_MODEL"))
	bedrockClient := bedrockruntime.NewFromConfig(cfg)
	svc := rank.NewService(bedrockClient, model)

	segments, err := svc.Rank(ctx, transcript, event.SourceDuration)
	if err != nil {
		// ErrNoValidSegments and any other error fail the state; the state
		// machine catches this and routes to MarkFailed.
		return RankOutput{}, fmt.Errorf("ranking failed: %w", err)
	}

	return RankOutput{Clips: segments}, nil
}
