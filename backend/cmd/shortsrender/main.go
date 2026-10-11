// Package main is the entry point for the shorts-render Fargate task. Unlike
// the other pipeline stages, this runs as a plain ECS task (not lambda.Start)
// because clipping and caption-burning a segment of a long source can exceed
// Lambda's timeout and ephemeral-storage limits. It reads its input from
// environment variables provided by the Step Functions ecs:runTask override,
// renders one 9:16 captioned clip, and exits 0 on success or non-zero on error.
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/rahul/indifferent/backend/internal/models"
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
	if end-start > 600 {
		return fmt.Errorf("invalid clip window: %.0fs exceeds the 600s source cap", end-start)
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

// isAudioOnly reports whether the source key names an audio-only file.
func isAudioOnly(sourceKey string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(sourceKey))]
}
