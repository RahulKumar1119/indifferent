// Package main is the entry point for the story-render Fargate task. It makes
// one Story2Shorts video: it copies the chosen stock pictures into our bucket,
// narrates each scene with Amazon Polly, renders the video with FFmpeg and
// uploads it, writing its progress and its end state on the job record.
//
// Like the shorts-render task it is a plain ECS task, not a Lambda: a render can
// run for minutes and needs disk for the stock clips. It reads which job to make
// from the environment set by the Step Functions ecs:runTask override.
//
// The exit code is the contract with the state machine (see storyjob.ExitCode):
// 0 when the job reached an end state (completed, failed with its reason on the
// record, or cancelled by the user), and 1 when it should be run again.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/rahul/indifferent/backend/internal/narrator"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/storyvideo"
)

// taskTimeout is the longest one attempt may run. The state machine's own task
// timeout (1200 s, in pipeline/statemachine.json) is five minutes longer, so
// this is what ends a stuck render: the runner marks the job failed with a
// reason the user can read, and exits 0 so the task is not run again. If the
// state machine's timeout came first, the task would be killed with nothing
// saved, and retried on Spot and then on-demand.
//
// 15 minutes is a ceiling chosen before any render has run on Fargate. The
// storyjob_timing log line records the real times; change this and the state
// machine's timeout together, keeping the state machine's the longer.
const taskTimeout = 15 * time.Minute

func main() {
	os.Exit(run())
}

func run() int {
	// ECS sends SIGTERM when it stops the task (a Spot interruption, a deploy).
	// Cancelling the context stops FFmpeg with it and makes the run ask to be
	// retried, so the job is not lost.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()

	in, err := storyjob.ParseWorkerInput(os.Getenv("USER_ID"), os.Getenv("JOB_ID"))
	if err != nil {
		// A bad start is not something a retry can fix.
		log.Printf("storyrender: refusing to start: %v", err)
		return 2
	}
	bucket, table := os.Getenv("S3_BUCKET"), os.Getenv("DYNAMODB_TABLE")
	if bucket == "" || table == "" {
		log.Print("storyrender: S3_BUCKET and DYNAMODB_TABLE must be set")
		return 2
	}

	// Without fonts libass draws nothing, and the video would have no captions
	// and no error. Fail loudly instead.
	if err := render.EnsureCaptionFonts(); err != nil {
		log.Printf("storyrender: %v", err)
		return 1
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("storyrender: loading the AWS configuration: %v", err)
		return 1
	}
	s3, err := storage.NewS3Client(ctx)
	if err != nil {
		log.Printf("storyrender: creating the S3 client: %v", err)
		return 1
	}
	// Stock search keys come from the environment (see stockmedia.NewServiceFromEnv).
	// A missing key is a deployment mistake, not something the user can fix.
	stock, err := stockmedia.NewServiceFromEnv()
	if err != nil {
		log.Printf("storyrender: stock media is not configured: %v", err)
		return 1
	}

	prober := storyjob.FFprobe{}
	deps := storyjob.Deps{
		Assets:   stock,
		Files:    s3,
		Narrator: narrator.NewPollyNarrationService(polly.NewFromConfig(cfg)),
		Prober:   prober,
		Bucket:   bucket,
		WorkDir:  workDir(),
	}
	runner := &storyjob.Runner{
		Deps:  deps,
		Store: &storyjob.DynamoStore{DB: dynamodb.NewFromConfig(cfg), Table: table},
		Renderer: &storyjob.VideoRenderer{
			Fetch: s3, Bucket: bucket, FFmpeg: storyvideo.FFmpeg{}, Prober: prober, Logf: log.Printf,
		},
	}

	outcome, err := runner.Run(ctx, in.UserID, in.JobID)
	if err != nil {
		log.Printf("storyrender: job=%s outcome=%s: %v", in.JobID, outcome, err)
	} else {
		log.Printf("storyrender: job=%s outcome=%s", in.JobID, outcome)
	}
	return storyjob.ExitCode(outcome)
}

// workDir is the scratch directory. Fargate gives a task 20 GB of ephemeral
// disk; a stock clip is at most 250 MB and a job has at most ten scenes.
func workDir() string {
	if d := os.Getenv("WORK_DIR"); d != "" {
		return d
	}
	return os.TempDir()
}
