// Package main is the entry point for the Narrator Lambda function.
// It reads parsed question JSON from S3, generates MP3 audio for each
// question using Amazon Polly, and stores the results in S3.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/narrator"
	"github.com/rahul/indifferent/backend/internal/storage"
)

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, input models.NarratorInput) (models.NarratorOutput, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return models.NarratorOutput{}, fmt.Errorf("S3_BUCKET environment variable not set")
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return models.NarratorOutput{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Initialize S3 client
	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return models.NarratorOutput{}, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Initialize Polly client
	pollyClient := polly.NewFromConfig(cfg)

	// Create narration service
	narrationService := narrator.NewPollyNarrationService(pollyClient)

	// Create handler and process request
	handler := narrator.NewHandler(narrationService, s3Client, bucket)
	return handler.HandleRequest(ctx, input)
}
