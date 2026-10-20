// Package main is the entry point for the shorts-render Fargate task. Unlike
// the other pipeline stages, this runs as a plain ECS task (not lambda.Start)
// because clipping and caption-burning a segment of a long source can exceed
// Lambda's timeout and ephemeral-storage limits. It reads its input from
// environment variables provided by the Step Functions ecs:runTask override,
// renders one 9:16 captioned clip, and exits 0 on success or non-zero on error.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// audioExtensions are source file types with no video stream; they render onto
// a solid canvas rather than a center-crop. Extension-based by necessity (no
// stream probe in this worker); covered formats stay generous.
var audioExtensions = map[string]bool{
	".mp3": true,
	".wav": true,
	".m4a": true,
	".aac": true,
	".ogg": true,
	".opus": true,
	".flac": true,
	".wma": true,
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("shorts-render failed: %v", err)
	}
}

func run(ctx context.Context) error {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return fmt.Errorf("S3_BUCKET environment variable not set")
	}

	jobID := os.Getenv("JOB_ID")
	userID := os.Getenv("USER_ID")
	sourceKey := os.Getenv("SOURCE_KEY")
	transcriptKey := os.Getenv("TRANSCRIPT_KEY")
	clipID := os.Getenv("CLIP_ID")
	if jobID == "" || userID == "" || sourceKey == "" || transcriptKey == "" || clipID == "" {
		return fmt.Errorf("missing required env: JOB_ID, USER_ID, SOURCE_KEY, TRANSCRIPT_KEY, CLIP_ID must all be set")
	}

	start, err := strconv.ParseFloat(os.Getenv("CLIP_START"), 64)
	if err != nil {
		return fmt.Errorf("invalid CLIP_START: %w", err)
	}
	end, err := strconv.ParseFloat(os.Getenv("CLIP_END"), 64)
	if err != nil {
		return fmt.Errorf("invalid CLIP_END: %w", err)
	}
	// Guard the render worker against poison inputs: NaN/Inf parse cleanly
	// but would reach ffmpeg as garbage, failing and retrying forever.
	if math.IsNaN(start) || math.IsInf(start, 0) || math.IsNaN(end) || math.IsInf(end, 0) {
		return fmt.Errorf("invalid clip window: start=%v end=%v must be finite numbers", start, end)
	}
	if start < 0 || end <= start {
		return fmt.Errorf("invalid clip window: require 0 <= start < end, got start=%v end=%v", start, end)
	}
	// A clip window can never be longer than the source it comes from. This
	// is a poison-input guard, not the clip duration limit (that is
	// shorts.MaxClipDuration, 90s).
	if end-start > shorts.MaxSourceDuration {
		return fmt.Errorf("invalid clip window: %.0fs exceeds the %.0f minute source cap", end-start, shorts.MaxSourceDuration/60)
	}
	// CLIP_RANK is optional metadata; default 0 when unset or unparsable.
	rank, _ := strconv.Atoi(os.Getenv("CLIP_RANK"))

	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	workDir, err := os.MkdirTemp("", "shorts-render-*")
	if err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	renderer := render.NewRenderer(s3Client, bucket, workDir)

	cfg, cfgErr := config.LoadDefaultConfig(ctx)
	if cfgErr != nil {
		log.Printf("reframe disabled: failed to load AWS config: %v", cfgErr)
	} else {
		// Subject-aware reframing: Rekognition face detection pans the 9:16
		// crop window onto the speaker instead of blindly center-cropping.
		// Any detection failure falls back to center inside the renderer,
		// so this is best-effort by design.
		renderer.Detector = render.NewRekognitionFaceDetector(rekognition.NewFromConfig(cfg))
	}

	// Render progress + project branding share one DynamoDB client. The table
	// is required: without it renders would silently lose progress updates
	// and brand overlays, so fail fast instead of degrading quietly.
	if cfgErr != nil {
		return fmt.Errorf("reframe disabled: failed to load AWS config: %v", cfgErr)
	}
	dbClient := dynamodb.NewFromConfig(cfg)
	table := os.Getenv("DYNAMODB_TABLE")
	if table == "" {
		return fmt.Errorf("DYNAMODB_TABLE environment variable not set")
	}
	renderer.FetchLogo = render.NewLogoFetcher(dbClient, table, userID, jobID, s3Client, bucket)
	renderer.Table = table
	renderer.Dynamo = dbClient

	in := render.RenderInput{
		JobID:         jobID,
		UserID:        userID,
		SourceKey:     sourceKey,
		TranscriptKey: transcriptKey,
		Caption:       fetchJobCaption(ctx, dbClient, table, userID, jobID),
		Edit:          fetchClipEdit(ctx, dbClient, table, userID, jobID, clipID),
		Clip: models.Clip{
			// Start/End ride along for caption rebuilds (Score/HookText live
			// only in the ranker's DynamoDB record, not in worker env).
			ClipID:   clipID,
			Rank:     rank,
			Duration: end - start,
			Start:    start,
			End:      end,
		},
		ClipStart: start,
		ClipEnd:   end,
		AudioOnly: isAudioOnly(sourceKey),
	}

	clipKey, err := renderer.Render(ctx, in)
	if err != nil {
		return err
	}

	log.Printf("rendered clip %s -> s3://%s/%s", clipID, bucket, clipKey)
	return nil
}

// fetchJobCaption loads the job's reusable caption style for the render.
// Any failure (missing record, unreadable value) falls back to the zero
// style, which renders the legacy bold look: branding must never block an
// otherwise finished clip.
func fetchJobCaption(ctx context.Context, dbClient *dynamodb.Client, table, userID, jobID string) models.CaptionStyle {
	out, err := dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		},
	})
	if err != nil || out.Item == nil {
		return models.CaptionStyle{}
	}
	return captionFromItem(out.Item)
}

// captionFromItem decodes the stored caption style; anything unexpected
// resolves to the zero (legacy) style.
func captionFromItem(item map[string]dbtypes.AttributeValue) models.CaptionStyle {
	raw, ok := item["caption"].(*dbtypes.AttributeValueMemberS)
	if !ok || raw.Value == "" {
		return models.CaptionStyle{}
	}
	var style models.CaptionStyle
	if err := json.Unmarshal([]byte(raw.Value), &style); err != nil {
		return models.CaptionStyle{}
	}
	return style
}

// fetchClipEdit loads one clip's saved user override for the render. Any
// failure (or none saved) yields nil: the ranker segment renders untouched.
func fetchClipEdit(ctx context.Context, dbClient *dynamodb.Client, table, userID, jobID, clipID string) *models.ClipEdit {
	out, err := dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		},
	})
	if err != nil || out.Item == nil {
		return nil
	}
	raw, ok := out.Item["clipEdits"].(*dbtypes.AttributeValueMemberS)
	if !ok || raw.Value == "" {
		return nil
	}
	var edits map[string]models.ClipEdit
	if err := json.Unmarshal([]byte(raw.Value), &edits); err != nil {
		return nil
	}
	edit, ok := edits[clipID]
	if !ok || edit.End <= edit.Start {
		return nil
	}
	return &edit
}

// isAudioOnly reports whether the source key names an audio-only file.
func isAudioOnly(sourceKey string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(sourceKey))]
}
