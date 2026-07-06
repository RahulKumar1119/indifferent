// Package main is the entry point for the Slide Generator Lambda function.
// It reads parsed question JSON from S3, renders each question as PNG slides
// using Go's native image rendering (no browser dependency), and stores the
// results in S3.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/slidegen"
	"github.com/rahul/indifferent/backend/internal/storage"
)

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, input models.SlideGenInput) (models.SlideGenOutput, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return models.SlideGenOutput{}, fmt.Errorf("S3_BUCKET environment variable not set")
	}

	// Initialize S3 client
	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return models.SlideGenOutput{}, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Use the Go-native renderer — no Playwright/Chromium dependency needed.
	// This works in standard Lambda zip deployments without a container image.
	renderer := slidegen.NewNativeRenderer()

	handler := slidegen.NewHandler(renderer, s3Client, bucket)
	return handler.HandleRequest(ctx, input)
}
