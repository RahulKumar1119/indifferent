// Package main provides the Lambda entry point for the pipeline status updater.
// This Lambda is invoked by Step Functions after each pipeline stage completes
// to update the project status in DynamoDB.
package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rahul/indifferent/backend/internal/pipeline"
	"github.com/rahul/indifferent/backend/internal/storage"
)

func main() {
	ctx := context.Background()

	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		log.Fatal("DYNAMODB_TABLE environment variable is required")
	}

	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		log.Fatal("S3_BUCKET environment variable is required")
	}

	// Initialize AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	// Initialize DynamoDB client
	dbClient := dynamodb.NewFromConfig(cfg)

	// Initialize S3 client for signed URL generation
	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		log.Fatalf("failed to create S3 client: %v", err)
	}

	// Create status updater
	updater := &pipeline.StatusUpdater{
		DB:        dbClient,
		S3Storage: s3Client,
		TableName: tableName,
		Bucket:    bucket,
	}

	// Register Lambda handler
	lambda.Start(func(ctx context.Context, input pipeline.StatusInput) error {
		return updater.UpdateStatus(ctx, input)
	})
}
