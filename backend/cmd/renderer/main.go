// Package main is the entry point for the Renderer Lambda container function.
// It downloads slide PNGs and audio MP3s from S3, composites them into a
// final MP4 video with FFmpeg, generates a thumbnail, and uploads results to S3.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/renderer"
	"github.com/rahul/indifferent/backend/internal/storage"
)

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, input models.RendererInput) (models.RendererOutput, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return models.RendererOutput{}, fmt.Errorf("S3_BUCKET environment variable not set")
	}

	// Initialize S3 client
	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Create real compositor adapter and thumbnail generator (FFmpeg-based)
	compositor := &renderer.SimpleCompositor{}
	thumbnail := &renderer.ThumbnailGenerator{}

	// Create handler and process request
	handler := renderer.NewHandler(compositor, thumbnail, s3Client, bucket)
	return handler.HandleRequest(ctx, input)
}
