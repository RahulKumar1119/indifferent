package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

// mockDynamoDBClient implements DynamoDBClient for testing.
type mockDynamoDBClient struct {
	updateItemInput *dynamodb.UpdateItemInput
	updateItemErr   error
}

func (m *mockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	m.updateItemInput = params
	return &dynamodb.UpdateItemOutput{}, m.updateItemErr
}

func TestUpdateStatus_Parsing(t *testing.T) {
	mock := &mockDynamoDBClient{}
	updater := &StatusUpdater{
		DB:        mock,
		TableName: "projects-table",
		Bucket:    "test-bucket",
	}

	input := StatusInput{
		ProjectID: "proj-123",
		UserID:    "user-456",
		Status:    "parsing",
	}

	err := updater.UpdateStatus(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key construction
	pk := mock.updateItemInput.Key["PK"].(*types.AttributeValueMemberS).Value
	sk := mock.updateItemInput.Key["SK"].(*types.AttributeValueMemberS).Value
	if pk != "USER#user-456" {
		t.Errorf("expected PK 'USER#user-456', got %q", pk)
	}
	if sk != "PROJECT#proj-123" {
		t.Errorf("expected SK 'PROJECT#proj-123', got %q", sk)
	}

	// Verify table name
	if *mock.updateItemInput.TableName != "projects-table" {
		t.Errorf("expected table 'projects-table', got %q", *mock.updateItemInput.TableName)
	}

	// Verify status value
	statusVal := mock.updateItemInput.ExpressionAttributeValues[":status"].(*types.AttributeValueMemberS).Value
	if statusVal != "parsing" {
		t.Errorf("expected status 'parsing', got %q", statusVal)
	}

	// Verify updatedAt is set
	if _, ok := mock.updateItemInput.ExpressionAttributeValues[":updatedAt"]; !ok {
		t.Error("expected :updatedAt to be set")
	}

	// Verify no extra fields for non-terminal status
	if _, ok := mock.updateItemInput.ExpressionAttributeValues[":videoKey"]; ok {
		t.Error("unexpected :videoKey for parsing status")
	}
	if _, ok := mock.updateItemInput.ExpressionAttributeValues[":error"]; ok {
		t.Error("unexpected :error for parsing status")
	}
}

func TestUpdateStatus_Completed(t *testing.T) {
	mock := &mockDynamoDBClient{}
	updater := &StatusUpdater{
		DB:        mock,
		TableName: "projects-table",
		Bucket:    "test-bucket",
	}

	input := StatusInput{
		ProjectID:    "proj-123",
		UserID:       "user-456",
		Status:       "completed",
		VideoKey:     "output/user-456/proj-123/video.mp4",
		ThumbnailKey: "output/user-456/proj-123/thumbnail.png",
	}

	err := updater.UpdateStatus(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify status
	statusVal := mock.updateItemInput.ExpressionAttributeValues[":status"].(*types.AttributeValueMemberS).Value
	if statusVal != "completed" {
		t.Errorf("expected status 'completed', got %q", statusVal)
	}

	// Verify videoKey is set
	videoKeyVal := mock.updateItemInput.ExpressionAttributeValues[":videoKey"].(*types.AttributeValueMemberS).Value
	if videoKeyVal != "output/user-456/proj-123/video.mp4" {
		t.Errorf("expected videoKey 'output/user-456/proj-123/video.mp4', got %q", videoKeyVal)
	}

	// Verify thumbnailKey is set
	thumbVal := mock.updateItemInput.ExpressionAttributeValues[":thumbnailKey"].(*types.AttributeValueMemberS).Value
	if thumbVal != "output/user-456/proj-123/thumbnail.png" {
		t.Errorf("expected thumbnailKey 'output/user-456/proj-123/thumbnail.png', got %q", thumbVal)
	}

	// Verify completedAt is set
	if _, ok := mock.updateItemInput.ExpressionAttributeValues[":completedAt"]; !ok {
		t.Error("expected :completedAt to be set")
	}

	// Verify expression attribute names include the completed fields
	if mock.updateItemInput.ExpressionAttributeNames["#videoKey"] != "videoKey" {
		t.Errorf("expected #videoKey mapped to 'videoKey', got %q", mock.updateItemInput.ExpressionAttributeNames["#videoKey"])
	}
	if mock.updateItemInput.ExpressionAttributeNames["#thumbnailKey"] != "thumbnailKey" {
		t.Errorf("expected #thumbnailKey mapped to 'thumbnailKey', got %q", mock.updateItemInput.ExpressionAttributeNames["#thumbnailKey"])
	}
	if mock.updateItemInput.ExpressionAttributeNames["#completedAt"] != "completedAt" {
		t.Errorf("expected #completedAt mapped to 'completedAt', got %q", mock.updateItemInput.ExpressionAttributeNames["#completedAt"])
	}
}

func TestUpdateStatus_Failed(t *testing.T) {
	mock := &mockDynamoDBClient{}
	updater := &StatusUpdater{
		DB:        mock,
		TableName: "projects-table",
		Bucket:    "test-bucket",
	}

	input := StatusInput{
		ProjectID: "proj-123",
		UserID:    "user-456",
		Status:    "failed",
		Error:     "Polly throttled: rate limit exceeded",
	}

	err := updater.UpdateStatus(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify status
	statusVal := mock.updateItemInput.ExpressionAttributeValues[":status"].(*types.AttributeValueMemberS).Value
	if statusVal != "failed" {
		t.Errorf("expected status 'failed', got %q", statusVal)
	}

	// Verify error is set
	errorVal := mock.updateItemInput.ExpressionAttributeValues[":error"].(*types.AttributeValueMemberS).Value
	if errorVal != "Polly throttled: rate limit exceeded" {
		t.Errorf("expected error 'Polly throttled: rate limit exceeded', got %q", errorVal)
	}

	// Verify expression attribute names include error
	if mock.updateItemInput.ExpressionAttributeNames["#error"] != "error" {
		t.Errorf("expected #error mapped to 'error', got %q", mock.updateItemInput.ExpressionAttributeNames["#error"])
	}
}

func TestUpdateStatus_FailedWithStepFunctionsError(t *testing.T) {
	mock := &mockDynamoDBClient{}
	updater := &StatusUpdater{
		DB:        mock,
		TableName: "projects-table",
		Bucket:    "test-bucket",
	}

	// Simulate Step Functions error format
	input := StatusInput{
		ProjectID: "proj-123",
		UserID:    "user-456",
		Status:    "failed",
		Error: map[string]any{
			"Error": "States.TaskFailed",
			"Cause": "Lambda function failed: out of memory",
		},
	}

	err := updater.UpdateStatus(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should extract Cause from Step Functions error
	errorVal := mock.updateItemInput.ExpressionAttributeValues[":error"].(*types.AttributeValueMemberS).Value
	if errorVal != "Lambda function failed: out of memory" {
		t.Errorf("expected error message from Cause, got %q", errorVal)
	}
}

func TestUpdateStatus_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		updateItemErr: fmt.Errorf("connection refused"),
	}
	updater := &StatusUpdater{
		DB:        mock,
		TableName: "projects-table",
		Bucket:    "test-bucket",
	}

	input := StatusInput{
		ProjectID: "proj-123",
		UserID:    "user-456",
		Status:    "rendering",
	}

	err := updater.UpdateStatus(context.Background(), input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "nil error",
			input:    nil,
			expected: "unknown error",
		},
		{
			name:     "string error",
			input:    "something went wrong",
			expected: "something went wrong",
		},
		{
			name: "step functions error with Cause",
			input: map[string]any{
				"Error": "States.TaskFailed",
				"Cause": "timeout exceeded",
			},
			expected: "timeout exceeded",
		},
		{
			name: "step functions error without Cause",
			input: map[string]any{
				"Error": "States.Timeout",
			},
			expected: "States.Timeout",
		},
		{
			name:     "numeric error",
			input:    42,
			expected: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractErrorMessage(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestUpdateShortsStatus_Stages(t *testing.T) {
	// Non-terminal shorts stages update status + updatedAt only, keyed to SHORTS#.
	stages := []string{
		ShortsStatusUploaded,
		ShortsStatusTranscribing,
		ShortsStatusRanking,
		ShortsStatusRendering,
	}

	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			mock := &mockDynamoDBClient{}
			updater := &StatusUpdater{DB: mock, TableName: "projects-table", Bucket: "test-bucket"}

			err := updater.UpdateShortsStatus(context.Background(), ShortsStatusInput{
				JobID:  "job-123",
				UserID: "user-456",
				Status: stage,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			pk := mock.updateItemInput.Key["PK"].(*types.AttributeValueMemberS).Value
			sk := mock.updateItemInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if pk != "USER#user-456" {
				t.Errorf("expected PK 'USER#user-456', got %q", pk)
			}
			if sk != "SHORTS#job-123" {
				t.Errorf("expected SK 'SHORTS#job-123', got %q", sk)
			}

			statusVal := mock.updateItemInput.ExpressionAttributeValues[":status"].(*types.AttributeValueMemberS).Value
			if statusVal != stage {
				t.Errorf("expected status %q, got %q", stage, statusVal)
			}
			if _, ok := mock.updateItemInput.ExpressionAttributeValues[":updatedAt"]; !ok {
				t.Error("expected :updatedAt to be set")
			}
			// No clips/error/completedAt for non-terminal stages.
			if _, ok := mock.updateItemInput.ExpressionAttributeValues[":clips"]; ok {
				t.Errorf("unexpected :clips for stage %q", stage)
			}
			if _, ok := mock.updateItemInput.ExpressionAttributeValues[":error"]; ok {
				t.Errorf("unexpected :error for stage %q", stage)
			}
			if _, ok := mock.updateItemInput.ExpressionAttributeValues[":completedAt"]; ok {
				t.Errorf("unexpected :completedAt for stage %q", stage)
			}
		})
	}
}

func TestUpdateShortsStatus_CompletedStoresClips(t *testing.T) {
	mock := &mockDynamoDBClient{}
	updater := &StatusUpdater{DB: mock, TableName: "projects-table", Bucket: "test-bucket"}

	clips := []models.RankedSegment{
		{ClipID: "clip-1", Start: 0, End: 27.5, Score: 0.93, Rank: 1, HookText: "a"},
		{ClipID: "clip-2", Start: 100, End: 118, Score: 0.81, Rank: 2, HookText: "b"},
	}

	err := updater.UpdateShortsStatus(context.Background(), ShortsStatusInput{
		JobID:  "job-123",
		UserID: "user-456",
		Status: ShortsStatusCompleted,
		Clips:  clips,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SHORTS# key.
	sk := mock.updateItemInput.Key["SK"].(*types.AttributeValueMemberS).Value
	if sk != "SHORTS#job-123" {
		t.Errorf("expected SK 'SHORTS#job-123', got %q", sk)
	}

	statusVal := mock.updateItemInput.ExpressionAttributeValues[":status"].(*types.AttributeValueMemberS).Value
	if statusVal != "completed" {
		t.Errorf("expected status 'completed', got %q", statusVal)
	}

	// clips attribute is written as JSON and round-trips to the input clips.
	clipsAttr, ok := mock.updateItemInput.ExpressionAttributeValues[":clips"].(*types.AttributeValueMemberS)
	if !ok {
		t.Fatal("expected :clips to be a string attribute")
	}
	var got []models.Clip
	if err := json.Unmarshal([]byte(clipsAttr.Value), &got); err != nil {
		t.Fatalf("clips attribute is not valid JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 clips, got %d", len(got))
	}
	if got[0].ClipID != "clip-1" || got[0].Rank != 1 || got[1].ClipID != "clip-2" || got[1].Rank != 2 {
		t.Errorf("clips did not round-trip correctly: %+v", got)
	}
	// S3 keys use the renderer's deterministic pattern so the gallery can
	// presign URLs; durations derive from the segment windows.
	if got[0].S3Key != "shorts/user-456/job-123/clips/clip-1.mp4" {
		t.Errorf("unexpected S3 key: %q", got[0].S3Key)
	}
	if got[0].Duration != 27.5 || got[1].Duration != 18 {
		t.Errorf("unexpected durations: %v, %v", got[0].Duration, got[1].Duration)
	}

	if mock.updateItemInput.ExpressionAttributeNames["#clips"] != "clips" {
		t.Errorf("expected #clips mapped to 'clips', got %q", mock.updateItemInput.ExpressionAttributeNames["#clips"])
	}
	if _, ok := mock.updateItemInput.ExpressionAttributeValues[":completedAt"]; !ok {
		t.Error("expected :completedAt to be set on completed")
	}
}

func TestUpdateShortsStatus_Failed(t *testing.T) {
	mock := &mockDynamoDBClient{}
	updater := &StatusUpdater{DB: mock, TableName: "projects-table", Bucket: "test-bucket"}

	err := updater.UpdateShortsStatus(context.Background(), ShortsStatusInput{
		JobID:  "job-123",
		UserID: "user-456",
		Status: ShortsStatusFailed,
		Error:  "no engaging segments identified",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errorVal := mock.updateItemInput.ExpressionAttributeValues[":error"].(*types.AttributeValueMemberS).Value
	if errorVal != "no engaging segments identified" {
		t.Errorf("expected failure reason stored, got %q", errorVal)
	}
	if _, ok := mock.updateItemInput.ExpressionAttributeValues[":clips"]; ok {
		t.Error("unexpected :clips on failed status")
	}
}

func TestUpdateShortsStatus_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{updateItemErr: fmt.Errorf("connection refused")}
	updater := &StatusUpdater{DB: mock, TableName: "projects-table", Bucket: "test-bucket"}

	err := updater.UpdateShortsStatus(context.Background(), ShortsStatusInput{
		JobID:  "job-123",
		UserID: "user-456",
		Status: ShortsStatusRendering,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
