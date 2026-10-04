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
// a solid canvas rather than a center-crop.
var audioExtensions = map[string]bool{
	".mp3": true,
	".wav": true,
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

	// Project branding: overlay the linked project's logo when present.
	// Missing table env, unlinked jobs, or download failures all fall back
	// to unbranded inside the renderer.
	if cfgErr == nil {
		if table := os.Getenv("DYNAMODB_TABLE"); table != "" {
			dbClient := dynamodb.NewFromConfig(cfg)
			renderer.FetchLogo = render.NewLogoFetcher(dbClient, table, userID, jobID, s3Client, bucket)
		}
	}

	in := render.RenderInput{
		JobID:         jobID,
		UserID:        userID,
		SourceKey:     sourceKey,
		TranscriptKey: transcriptKey,
		Clip: models.Clip{
			ClipID:   clipID,
			Rank:     rank,
			Duration: end - start,
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
