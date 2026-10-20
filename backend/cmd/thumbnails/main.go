// Command thumbnails is the Fargate task that makes AI thumbnails for an
// uploaded video (issue #77). Like the shorts-render task it is a plain ECS
// task, not a Lambda: it downloads the source video, runs FFmpeg over it and
// calls Rekognition and Bedrock, which can run for minutes and need disk.
//
// Step Functions starts it with the job in environment variables and the task
// exits 0 when it finished or recorded a failure on the job. It exits non-zero
// only when it could not record anything.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/thumbnail"
)

// runTimeout bounds the whole task. A normal run takes a minute or two.
const runTimeout = 12 * time.Minute

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	if err := run(ctx); err != nil {
		log.Fatalf("thumbnails failed: %v", err)
	}
}

func run(ctx context.Context) error {
	bucket := os.Getenv("S3_BUCKET")
	table := os.Getenv("DYNAMODB_TABLE")
	job := thumbnail.Job{
		JobID:         os.Getenv("JOB_ID"),
		UserID:        os.Getenv("USER_ID"),
		SourceKey:     os.Getenv("SOURCE_KEY"),
		TranscriptKey: os.Getenv("TRANSCRIPT_KEY"),
	}
	if bucket == "" || table == "" || job.JobID == "" || job.UserID == "" || job.SourceKey == "" {
		return fmt.Errorf("missing required env: S3_BUCKET, DYNAMODB_TABLE, JOB_ID, USER_ID and SOURCE_KEY must all be set")
	}
	// The state machine passes a missing duration as an empty string.
	if d, err := strconv.ParseFloat(os.Getenv("SOURCE_DURATION"), 64); err == nil && d > 0 {
		job.Duration = d
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return fmt.Errorf("create S3 client: %w", err)
	}
	dbClient := dynamodb.NewFromConfig(cfg)

	w := &thumbnail.Worker{
		Store:   &thumbnail.Store{DB: dbClient, Table: table},
		Objects: s3Client,
		Bucket:  bucket,
		Pipeline: thumbnail.ServicePipeline{Service: thumbnail.Service{
			Detector: &thumbnail.RekognitionDetector{Client: rekognition.NewFromConfig(cfg)},
			Analyzer: &thumbnail.BedrockAnalyzer{Client: bedrockruntime.NewFromConfig(cfg), Model: os.Getenv("THUMBNAIL_MODEL")},
			Logf:     log.Printf,
		}},
		JobExists: func(ctx context.Context, userID, jobID string) (bool, error) {
			return itemExists(ctx, dbClient, table, userID, "SHORTS#"+jobID)
		},
		Brand: func(ctx context.Context, userID, jobID string) string {
			return channelName(ctx, dbClient, table, userID, jobID)
		},
		Logf: log.Printf,
	}
	return w.Run(ctx, job)
}

func itemExists(ctx context.Context, c *dynamodb.Client, table, userID, sk string) (bool, error) {
	out, err := c.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(table),
		Key:            map[string]dbtypes.AttributeValue{"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID}, "SK": &dbtypes.AttributeValueMemberS{Value: sk}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return false, err
	}
	return out.Item != nil, nil
}

// channelName returns the channel name from the branding of the project this
// Shorts job is linked to. Branding is a nicety, so every failure, an unlinked
// job included, gives "" and the thumbnails are made without a tag.
func channelName(ctx context.Context, c *dynamodb.Client, table, userID, jobID string) string {
	get := func(sk string) map[string]dbtypes.AttributeValue {
		out, err := c.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(table),
			Key:       map[string]dbtypes.AttributeValue{"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID}, "SK": &dbtypes.AttributeValueMemberS{Value: sk}},
		})
		if err != nil {
			log.Printf("branding lookup %s: %v", sk, err)
			return nil
		}
		return out.Item
	}
	str := func(item map[string]dbtypes.AttributeValue, name string) string {
		if v, ok := item[name].(*dbtypes.AttributeValueMemberS); ok {
			return v.Value
		}
		return ""
	}
	projectID := str(get("SHORTS#"+jobID), "projectId")
	if projectID == "" {
		return ""
	}
	raw := str(get("PROJECT#"+projectID), "branding")
	if raw == "" {
		return ""
	}
	var b models.Branding
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		return ""
	}
	return strings.TrimSpace(b.ChannelName)
}
