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
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
	"github.com/rahul/indifferent/backend/internal/shorts/cliprender"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// audioExtensions are source file types with no video stream; they render onto
// a solid canvas rather than a center-crop. Extension-based by necessity (no
// stream probe in this worker); covered formats stay generous.
var audioExtensions = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".m4a":  true,
	".aac":  true,
	".ogg":  true,
	".opus": true,
	".flac": true,
	".wma":  true,
}

func main() {
	// ECS sends SIGTERM when it stops the task. Cancelling the context stops the
	// render, and the ffmpeg it runs is killed with it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
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
	// RERENDER_TOKEN marks a single-clip render of a finished clip (#329),
	// started from the clip's page rather than by the pipeline.
	token := os.Getenv("RERENDER_TOKEN")
	if token != "" && !cliprender.ValidToken(token) {
		return fmt.Errorf("invalid RERENDER_TOKEN")
	}

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
	if token != "" {
		// The job is finished: its progress belongs to the pipeline, not to
		// one clip rendered again.
		renderer.Table = ""
		rec, err := cliprender.Get(ctx, dbClient, table, userID, jobID, clipID)
		if err != nil {
			return fmt.Errorf("failed to read the clip render record: %w", err)
		}
		if rec == nil || rec.Token != token || rec.State != models.ClipRenderRendering {
			// A newer render took its place, or the job was deleted. Nothing to do.
			log.Printf("clip_render_superseded job=%s clip=%s", jobID, clipID)
			return nil
		}
	}

	effects, err := fetchJobEffects(ctx, dbClient, table, userID, jobID)
	if err != nil {
		return fmt.Errorf("failed to load video effects: %w", err)
	}

	in := render.RenderInput{
		JobID:         jobID,
		UserID:        userID,
		SourceKey:     sourceKey,
		TranscriptKey: transcriptKey,
		Caption:       fetchJobCaption(ctx, dbClient, table, userID, jobID),
		Edit:          fetchClipEdit(ctx, dbClient, table, userID, jobID, clipID),
		Effects:       effects,
		Hook:          fetchClipHook(ctx, dbClient, table, userID, jobID, clipID),
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

	if token != "" {
		return renderOne(ctx, renderer, dbClient, table, in, token)
	}

	// A user cancel only writes the job record. Poll it, and stop this render
	// once the job leaves "rendering".
	renderCtx, cancelRender := context.WithCancel(ctx)
	defer cancelRender()
	go render.WatchJobStatus(renderCtx, cancelPollInterval, func(ctx context.Context) (string, error) {
		return jobStatus(ctx, dbClient, table, userID, jobID)
	}, cancelRender)

	clipKey, err := renderer.Render(renderCtx, in)
	if err != nil {
		// The full error (ffmpeg's exit, the phase) stays in the task log; the
		// job record gets a short reason a person can read, instead of the
		// state machine's generic "failed on both Spot and on-demand" (#287).
		log.Printf("render_failed job=%s clip=%s effects=%t err=%q", jobID, clipID, effects.Active(), logCause(err))
		if msg := renderErrorMessage(err); msg != "" {
			recordRenderError(dbClient, table, userID, jobID, errors.New(msg))
		}
		return err
	}

	log.Printf("rendered clip %s -> s3://%s/%s", clipID, bucket, clipKey)
	return nil
}

// clipRenderer renders one clip. *render.Renderer implements it.
type clipRenderer interface {
	Render(ctx context.Context, in render.RenderInput) (string, error)
}

// renderOne renders a finished clip again (#329). The new file goes to a key of
// its own, and the clip is pointed at it only once it rendered, and only while
// this render is still the clip's latest. A failure or a newer request leaves
// the file that plays as it is. The job record is not touched.
//
// The render stops when a newer request takes the clip's record or the record
// is deleted with the job. A stop for that reason ends the task without an
// error, so the state machine does not try again. Any other failure keeps its
// reason on the record and ends with an error, so the state machine can try on
// other capacity and, when it gives up, marks the render failed.
func renderOne(ctx context.Context, renderer clipRenderer, db cliprender.DB, table string, in render.RenderInput, token string) error {
	in.OutputKey = cliprender.OutputKey(in.UserID, in.JobID, in.Clip.ClipID, token)
	renderCtx, cancelRender := context.WithCancel(ctx)
	defer cancelRender()
	var superseded atomicFlag
	go render.WatchJobStatus(renderCtx, cancelPollInterval, func(ctx context.Context) (string, error) {
		rec, err := cliprender.Get(ctx, db, table, in.UserID, in.JobID, in.Clip.ClipID)
		if err != nil {
			return "", err
		}
		if rec == nil || rec.Token != token || rec.State != models.ClipRenderRendering {
			superseded.set()
			return "superseded", nil
		}
		return "rendering", nil
	}, cancelRender)

	key, err := renderer.Render(renderCtx, in)
	if err != nil {
		if superseded.get() {
			log.Printf("clip_render_superseded job=%s clip=%s", in.JobID, in.Clip.ClipID)
			return nil
		}
		log.Printf("clip_render_failed job=%s clip=%s effects=%t err=%q", in.JobID, in.Clip.ClipID, in.Effects.Active(), logCause(err))
		if msg := renderErrorMessage(err); msg != "" {
			if rerr := cliprender.RecordError(context.Background(), db, table, in.UserID, in.JobID, in.Clip.ClipID, token, msg); rerr != nil && !errors.Is(rerr, cliprender.ErrSuperseded) {
				log.Printf("could not record the clip render error: %v", rerr)
			}
		}
		return err
	}
	switch err := cliprender.Promote(ctx, db, table, in.UserID, in.JobID, in.Clip.ClipID, token, key, time.Now()); {
	case errors.Is(err, cliprender.ErrSuperseded):
		// The file stays under the job's folder and goes when the job is deleted.
		log.Printf("clip_render_superseded job=%s clip=%s key=%s", in.JobID, in.Clip.ClipID, key)
		return nil
	case err != nil:
		return fmt.Errorf("failed to point the clip at its new file: %w", err)
	}
	log.Printf("clip_render_done job=%s clip=%s key=%s", in.JobID, in.Clip.ClipID, key)
	return nil
}

// atomicFlag is a flag set by the watcher and read after the render.
type atomicFlag struct{ v atomic.Bool }

func (f *atomicFlag) set()      { f.v.Store(true) }
func (f *atomicFlag) get() bool { return f.v.Load() }

// fetchClipHook loads one clip's hook. Any failure, or none saved, renders the
// clip without one, the way clips rendered before hooks existed.
func fetchClipHook(ctx context.Context, dbClient *dynamodb.Client, table, userID, jobID, clipID string) *models.ClipHook {
	out, err := dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil || out.Item == nil {
		return nil
	}
	return hookFromItem(out.Item, clipID)
}

// hookFromItem decodes one clip's hook from a job record.
func hookFromItem(item map[string]dbtypes.AttributeValue, clipID string) *models.ClipHook {
	raw, ok := item["clipHooks"].(*dbtypes.AttributeValueMemberS)
	if !ok || raw.Value == "" {
		return nil
	}
	var hooks map[string]models.ClipHook
	if err := json.Unmarshal([]byte(raw.Value), &hooks); err != nil {
		return nil
	}
	h, ok := hooks[clipID]
	if !ok || !h.Active() {
		return nil
	}
	h = h.Normalized()
	if h.Validate() != nil {
		return nil
	}
	return &h
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

// cancelPollInterval is how often the render task re-reads its job record to see
// whether the user cancelled the job (a variable so tests can shorten it).
var cancelPollInterval = 10 * time.Second

// jobStatus reads the job's status with a consistent read. A missing record
// reads as an empty status, so a deleted job stops its render too.
func jobStatus(ctx context.Context, dbClient *dynamodb.Client, table, userID, jobID string) (string, error) {
	out, err := dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}
	status, _ := out.Item["status"].(*dbtypes.AttributeValueMemberS)
	if status == nil {
		return "", nil
	}
	return status.Value, nil
}

// Render failure messages a person sees on the job. None names ffmpeg, an
// exit status or a file path. None reads as a problem with the user's file
// either: the status updater's classifier keeps every one of them retryable
// (none matches a VALIDATION pattern), because a render failure is ours to fix.
const (
	renderFailedMessage   = "We could not render this clip. Try again, and if it keeps failing, try a different file."
	renderTimeoutMessage  = "This clip took too long to render. Try again, or turn off some video effects first."
	renderEffectsMessage  = "These video effects could not be applied to this clip. Change them, then try again."
	maxLoggedRenderCauses = 2000
)

// renderErrorMessage is the reason to store for a failed render, or "" when
// none should be stored: a cancelled render ended because the user asked.
func renderErrorMessage(err error) string {
	switch {
	case errors.Is(err, render.ErrRenderCanceled):
		return ""
	case errors.Is(err, render.ErrRenderTimeout):
		return renderTimeoutMessage
	case errors.Is(err, render.ErrEffectsInvalid):
		return renderEffectsMessage
	default:
		return renderFailedMessage
	}
}

// logCause bounds an error for one log line. It is logged with %q, which
// escapes newlines, so ffmpeg output inside it cannot forge log lines.
func logCause(err error) string {
	r := []rune(err.Error())
	if len(r) > maxLoggedRenderCauses {
		return string(r[:maxLoggedRenderCauses]) + "..."
	}
	return string(r)
}

// maxRenderErrorChars bounds the render reason stored on the job record.
const maxRenderErrorChars = 300

// recordRenderError stores why a render failed on the job record.
// The status updater reports that reason in place of the generic cause. The
// first reason wins: the write only happens while no reason is set, so the
// failure of the first clip, which usually holds the root cause, is kept. The
// write is also conditional on the job existing, so a deleted job is never
// recreated. Failures are only logged, because the render error is the one the
// task returns.
func recordRenderError(db render.DynamoUpdater, table, userID, jobID string, renderErr error) {
	msg := []rune(renderErr.Error())
	if len(msg) > maxRenderErrorChars {
		msg = msg[:maxRenderErrorChars]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		},
		ConditionExpression:      aws.String("attribute_exists(PK) AND attribute_not_exists(renderError)"),
		UpdateExpression:         aws.String("SET #renderError = :reason"),
		ExpressionAttributeNames: map[string]string{"#renderError": "renderError"},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":reason": &dbtypes.AttributeValueMemberS{Value: string(msg)},
		},
	}); err != nil {
		var condFailed *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &condFailed) {
			log.Printf("render error not recorded for job %s: a reason is already set or the job is gone", jobID)
			return
		}
		log.Printf("could not record the render error: %v", err)
	}
}

// fetchJobEffects loads the job's saved video effects with a consistent read,
// so a retry started right after a save sees the new settings. Unlike the
// caption style, a failed read fails the render: a clip rendered without the
// effects the user chose would be a different clip.
func fetchJobEffects(ctx context.Context, dbClient *dynamodb.Client, table, userID, jobID string) (*models.VideoEffects, error) {
	out, err := dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	return effectsFromItem(out.Item)
}

// effectsFromItem decodes the stored video effects. A missing value means the
// job has no effects, and a value that cannot be decoded is an error.
func effectsFromItem(item map[string]dbtypes.AttributeValue) (*models.VideoEffects, error) {
	raw, ok := item["effects"].(*dbtypes.AttributeValueMemberS)
	if !ok || raw.Value == "" {
		return nil, nil
	}
	var effects models.VideoEffects
	if err := json.Unmarshal([]byte(raw.Value), &effects); err != nil {
		return nil, fmt.Errorf("stored effects are unreadable: %w", err)
	}
	return &effects, nil
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
