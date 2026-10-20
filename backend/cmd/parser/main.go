// Package main is the entry point for the Parser Lambda function.
// It reads a TXT file from S3, detects its format, extracts questions,
// validates them, and writes structured JSON output back to S3.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/parser"
	"github.com/rahul/indifferent/backend/internal/storage"
)

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, input models.ParserInput) (models.ParserOutput, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return models.ParserOutput{}, fmt.Errorf("S3_BUCKET environment variable not set")
	}

	// Initialize S3 client
	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return models.ParserOutput{}, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Read TXT file from S3
	data, err := s3Client.GetObject(ctx, bucket, input.S3Key)
	if err != nil {
		return models.ParserOutput{}, fmt.Errorf("failed to read TXT file from S3: %w", err)
	}

	// Validate file size
	if err := parser.ValidateFileSize(data); err != nil {
		return models.ParserOutput{}, fmt.Errorf("file validation failed: %w", err)
	}

	// Sanitize content
	content := parser.SanitizeContent(string(data))

	// Detect format
	format, err := parser.DetectFormat(content)
	if err != nil {
		return models.ParserOutput{}, fmt.Errorf("format detection failed: %w", err)
	}

	// Extract questions
	questions := parser.ExtractQuestions(content, string(format))

	// Validate each question, collecting warnings and keeping valid ones
	var validQuestions []models.Question
	var warnings []models.Warning
	for _, q := range questions {
		reason := parser.ValidateQuestion(q)
		if reason != "" {
			warnings = append(warnings, models.Warning{
				Line:   q.Index + 1,
				Reason: reason,
			})
		} else {
			// A question with no marked answer still renders, but with no
			// answer reveal. Warn the user instead of failing silently.
			if missing := parser.MissingAnswerReason(q); missing != "" {
				warnings = append(warnings, models.Warning{
					Line:   q.Index + 1,
					Reason: missing,
				})
			}
			validQuestions = append(validQuestions, q)
		}
	}

	// Validate that at least one valid question exists
	if err := parser.ValidateQuestions(validQuestions, warnings); err != nil {
		return models.ParserOutput{}, fmt.Errorf("question validation failed: %w", err)
	}

	// Persist warnings on the project so the UI can show them. Best effort:
	// a failure here must not fail the render.
	if err := persistWarnings(ctx, input, warnings); err != nil {
		log.Printf("persist warnings failed: %v", err)
	}

	// Serialize valid questions as JSON
	jsonData, err := json.Marshal(validQuestions)
	if err != nil {
		return models.ParserOutput{}, fmt.Errorf("failed to serialize questions: %w", err)
	}

	// Write JSON to S3
	outputKey := fmt.Sprintf("parsed/%s/%s/questions.json", input.UserID, input.ProjectID)
	if err := s3Client.PutObject(ctx, bucket, outputKey, jsonData, "application/json"); err != nil {
		return models.ParserOutput{}, fmt.Errorf("failed to write JSON to S3: %w", err)
	}

	return models.ParserOutput{
		ProjectID: input.ProjectID,
		S3Key:     outputKey,
		Questions: len(validQuestions),
		Format:    string(format),
		Warnings:  warnings,
	}, nil
}

// persistWarnings stores the parse warnings on the project record so the
// project page can tell the user that a question has no answer reveal.
// Warnings are advisory: if the table is not configured or the write fails,
// the render continues.
func persistWarnings(ctx context.Context, input models.ParserInput, warnings []models.Warning) error {
	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" || len(warnings) == 0 {
		return nil
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}
	client := dynamodb.NewFromConfig(cfg)

	encoded, err := json.Marshal(warnings)
	if err != nil {
		return fmt.Errorf("encode warnings: %w", err)
	}

	_, err = client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", input.UserID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("PROJECT#%s", input.ProjectID)},
		},
		// Only the warnings attribute is touched, so this cannot clobber the
		// status the pipeline writes concurrently.
		UpdateExpression: aws.String("SET warnings = :warnings"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":warnings": &types.AttributeValueMemberS{Value: string(encoded)},
		},
	})
	if err != nil {
		return fmt.Errorf("update project item: %w", err)
	}
	return nil
}
