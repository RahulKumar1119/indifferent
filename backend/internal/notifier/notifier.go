// Package notifier sends one email when a quiz-video, AI Shorts or Story2Shorts
// job finishes or fails. It runs as the last state of each Step Functions
// pipeline, after the job's final status has been written, and is wrapped in a
// Catch there: whatever happens in this package, a job's outcome never changes.
package notifier

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/notifyprefs"
	"github.com/rahul/indifferent/backend/internal/projectfail"
	"github.com/rahul/indifferent/backend/internal/storymetrics"
)

// Attributes the notifier writes on the job item (projects table).
const (
	attrNotifiedExecution = "notifiedExecution"
	attrNotifiedAt        = "notifiedAt"
	attrNotifyMessageID   = "notifyMessageId"
	// attrMetricsExecution holds the execution whose outcome has been counted in
	// the metrics (see recordStoryOutcome).
	attrMetricsExecution = "metricsExecution"

	// The two failure codes the API records itself (storyjob.CodeCancelled and
	// storyjob.CodeStartFailed). They are spelled out here because this Lambda
	// should not import storyjob, which pulls in the renderer; a test in
	// story_metrics_test.go fails if they ever differ.
	codeCancelled   = "CANCELLED"
	codeStartFailed = "START_FAILED"
)

// Result statuses returned to Step Functions and written to the log.
const (
	StatusSent    = "sent"
	StatusSkipped = "skipped"
)

// DynamoDBAPI is the part of the DynamoDB client the notifier uses.
type DynamoDBAPI interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// Input is the event the state machines pass to the notifier. Exactly one of
// ProjectID (kind "quiz") or JobID (kinds "shorts" and "story") is used. ExecutionID is the
// Step Functions execution ARN ($$.Execution.Id).
type Input struct {
	Kind        string `json:"kind"`
	Outcome     string `json:"outcome"`
	UserID      string `json:"userId"`
	ProjectID   string `json:"projectId,omitempty"`
	JobID       string `json:"jobId,omitempty"`
	ExecutionID string `json:"executionId"`
}

// Result describes what happened. Step Functions discards it (ResultPath is
// null); it exists for the logs and for tests.
type Result struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Notifier decides whether to email and sends.
type Notifier struct {
	DB            DynamoDBAPI
	Mailer        Mailer
	ProjectsTable string // jobs: PK=USER#<id>, SK=PROJECT#<id> | SHORTS#<id> | STORYVIDEO#<id>
	UsersTable    string // profiles: PK=USER#<id>, SK=PROFILE
	BaseURL       string // site origin used for links, for example https://indifferent.fun
	Now           func() time.Time
	// Metrics records how Story2Shorts videos end (issue #194). Nil records
	// nothing. See recordStoryOutcome for why it is here and not in the worker.
	Metrics *storymetrics.Emitter
}

// Handle processes one notification request.
//
// Order matters: preferences and address checks first (nothing is written for
// a user who will not get mail), then a conditional claim keyed on the
// execution id, then the send. The claim makes Step Functions retries inside
// one execution safe, while a later execution for the same project (the user
// retried a failed job) can notify again.
func (n *Notifier) Handle(ctx context.Context, in Input) (Result, error) {
	sortKey, err := validate(in)
	if err != nil {
		// A malformed request is a bug in the state machine, not a user
		// problem. Fail loudly; the Catch keeps the job unaffected.
		return Result{}, &SendFailed{Err: err}
	}
	id := jobID(in)

	// Before the checks that skip the email: whether a video finished does not
	// depend on whether its owner wants mail about it.
	if in.Kind == KindStory {
		n.recordStoryOutcome(ctx, in, sortKey)
	}

	profile, err := n.getItem(ctx, n.UsersTable, "USER#"+in.UserID, "PROFILE")
	if err != nil {
		return Result{}, &SendFailed{Err: fmt.Errorf("load profile: %w", err)}
	}
	if profile == nil {
		return n.skip(in, "no_profile"), nil
	}
	if notifyprefs.Email(profile) == "" {
		return n.skip(in, "no_email"), nil
	}
	if !notifyprefs.EmailVerified(profile) {
		return n.skip(in, "email_not_verified"), nil
	}
	prefs := notifyprefs.FromItem(profile)
	if (in.Outcome == OutcomeCompleted && !prefs.Completed) || (in.Outcome == OutcomeFailed && !prefs.Failed) {
		return n.skip(in, "preference_off"), nil
	}

	job, err := n.getItem(ctx, n.ProjectsTable, "USER#"+in.UserID, sortKey)
	if err != nil {
		return Result{}, &SendFailed{Err: fmt.Errorf("load job: %w", err)}
	}
	if job == nil {
		return n.skip(in, "job_missing"), nil
	}
	if stringAttr(job, "status") != in.Outcome {
		// The record moved on (for example the job was reset for a retry).
		return n.skip(in, "status_mismatch"), nil
	}
	if (in.Kind == KindShorts || in.Kind == KindStory) && in.Outcome == OutcomeFailed && stringAttr(job, "errorCode") == "CANCELLED" {
		return n.skip(in, "cancelled_by_user"), nil
	}

	// The email's link is made from this id. For a story video it is the plan's,
	// not the job's: the page that shows the video is the plan's page.
	linkID := id
	if in.Kind == KindStory {
		if linkID = stringAttr(job, "planId"); linkID == "" {
			return n.skip(in, "no_plan"), nil
		}
	}

	reason := ""
	if in.Kind == KindQuiz && in.Outcome == OutcomeFailed {
		// Only a fixed sentence for a file problem; the stored error stays private.
		reason = projectfail.Classify(stringAttr(job, "error")).Explanation()
	}
	content, err := RenderWithReason(in.Kind, in.Outcome, stringAttr(job, "name"), linkID, n.BaseURL, reason)
	if err != nil {
		return Result{}, &SendFailed{Err: err}
	}

	claimed, err := n.claim(ctx, in.UserID, sortKey, in.ExecutionID)
	if err != nil {
		return Result{}, &SendFailed{Err: fmt.Errorf("claim: %w", err)}
	}
	if !claimed {
		return n.skip(in, "already_notified"), nil
	}

	messageID, err := n.Mailer.Send(ctx, Message{
		To:      notifyprefs.Email(profile),
		Subject: content.Subject,
		Text:    content.Text,
		HTML:    content.HTML,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrThrottled):
			// Nothing was sent: release the claim so the retry can send.
			n.release(ctx, in.UserID, sortKey, in.ExecutionID)
			log.Printf("notify result=throttled kind=%s outcome=%s user=%s id=%s err=%v", in.Kind, in.Outcome, in.UserID, id, err)
			return Result{}, &Throttled{Err: err}
		case errors.Is(err, ErrRejected):
			n.release(ctx, in.UserID, sortKey, in.ExecutionID)
			log.Printf("notify result=rejected kind=%s outcome=%s user=%s id=%s err=%v", in.Kind, in.Outcome, in.UserID, id, err)
			return Result{}, &SendFailed{Err: err}
		default:
			// Outcome unknown (timeout, dropped connection): keep the claim so
			// nothing retries and risks a duplicate.
			log.Printf("notify result=unknown kind=%s outcome=%s user=%s id=%s err=%v", in.Kind, in.Outcome, in.UserID, id, err)
			return Result{}, &SendFailed{Err: err}
		}
	}

	n.recordMessageID(ctx, in.UserID, sortKey, in.ExecutionID, messageID)
	log.Printf("notify result=sent kind=%s outcome=%s user=%s id=%s messageId=%s", in.Kind, in.Outcome, in.UserID, id, messageID)
	return Result{Status: StatusSent}, nil
}

// recordStoryOutcome counts a Story2Shorts video that has finished or failed, for
// the completion rate, the failure rate and the generation time.
//
// It is done here because every way a video can end passes through this Lambda
// once: the worker finishing or failing it, and also the state machine marking
// it failed when the worker never got to say (a crash, a stopped Spot task,
// retries that ran out). The worker cannot report those. This path is the only
// one that sees them all.
//
// It does not count what the API already has: a cancel stops the execution, and
// a start that failed never began one, so neither may reach here, and if one
// does it must not be counted twice.
//
// Step Functions runs this Lambda again when sending the email is throttled, so
// an execution is counted once, with the same kind of conditional write the
// email uses to avoid sending twice. Nothing in here may stop the email: a
// problem is logged and the metric is skipped.
func (n *Notifier) recordStoryOutcome(ctx context.Context, in Input, sortKey string) {
	if n.Metrics == nil {
		return
	}
	job, err := n.getItem(ctx, n.ProjectsTable, "USER#"+in.UserID, sortKey)
	if err != nil {
		log.Printf("notify metrics_skipped reason=load_failed kind=%s id=%s err=%v", in.Kind, in.JobID, err)
		return
	}
	if job == nil || stringAttr(job, "status") != in.Outcome {
		return // gone, or the record has moved on since this was sent
	}
	code := stringAttr(job, "errorCode")
	if in.Outcome == OutcomeFailed && (code == codeCancelled || code == codeStartFailed) {
		return // the API counted these when they happened
	}
	claimed, err := n.claimMetrics(ctx, in.UserID, sortKey, in.ExecutionID)
	if err != nil {
		log.Printf("notify metrics_skipped reason=claim_failed kind=%s id=%s err=%v", in.Kind, in.JobID, err)
		return
	}
	if !claimed {
		return // an earlier run of this execution counted it
	}
	if in.Outcome == OutcomeFailed {
		n.Metrics.VideoFailed(code)
		return
	}
	secs, ok := storymetrics.Seconds(stringAttr(job, "createdAt"), stringAttr(job, "completedAt"))
	n.Metrics.VideoCompleted(secs, ok)
}

// claimMetrics records that this execution's outcome has been counted. It
// succeeds only if the item exists and this execution has not claimed it.
func (n *Notifier) claimMetrics(ctx context.Context, userID, sortKey, executionID string) (bool, error) {
	_, err := n.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(n.ProjectsTable),
		Key:              key("USER#"+userID, sortKey),
		UpdateExpression: aws.String("SET " + attrMetricsExecution + " = :exec"),
		ConditionExpression: aws.String("attribute_exists(PK) AND (attribute_not_exists(" + attrMetricsExecution +
			") OR " + attrMetricsExecution + " <> :exec)"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":exec": &dbtypes.AttributeValueMemberS{Value: executionID},
		},
	})
	if err != nil {
		var cond *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &cond) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (n *Notifier) skip(in Input, reason string) Result {
	log.Printf("notify result=skipped reason=%s kind=%s outcome=%s user=%s id=%s", reason, in.Kind, in.Outcome, in.UserID, jobID(in))
	return Result{Status: StatusSkipped, Reason: reason}
}

// validate checks the request and returns the job's sort key.
func validate(in Input) (string, error) {
	if in.UserID == "" {
		return "", errors.New("userId is required")
	}
	if in.ExecutionID == "" {
		return "", errors.New("executionId is required")
	}
	if in.Outcome != OutcomeCompleted && in.Outcome != OutcomeFailed {
		return "", fmt.Errorf("unknown outcome %q", in.Outcome)
	}
	switch in.Kind {
	case KindQuiz:
		if in.ProjectID == "" {
			return "", errors.New("projectId is required for kind quiz")
		}
		return "PROJECT#" + in.ProjectID, nil
	case KindShorts:
		if in.JobID == "" {
			return "", errors.New("jobId is required for kind shorts")
		}
		return "SHORTS#" + in.JobID, nil
	case KindStory:
		if in.JobID == "" {
			return "", errors.New("jobId is required for kind story")
		}
		return "STORYVIDEO#" + in.JobID, nil
	}
	return "", fmt.Errorf("unknown kind %q", in.Kind)
}

func jobID(in Input) string {
	if in.Kind == KindShorts || in.Kind == KindStory {
		return in.JobID
	}
	return in.ProjectID
}

func (n *Notifier) getItem(ctx context.Context, table, pk, sk string) (map[string]dbtypes.AttributeValue, error) {
	out, err := n.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(table),
		Key:            key(pk, sk),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	return out.Item, nil
}

// claim records this execution as the one that notified. It succeeds only if
// the item exists and no other execution has claimed it already.
func (n *Notifier) claim(ctx context.Context, userID, sortKey, executionID string) (bool, error) {
	_, err := n.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(n.ProjectsTable),
		Key:              key("USER#"+userID, sortKey),
		UpdateExpression: aws.String("SET " + attrNotifiedExecution + " = :exec, " + attrNotifiedAt + " = :at"),
		ConditionExpression: aws.String("attribute_exists(PK) AND (attribute_not_exists(" + attrNotifiedExecution +
			") OR " + attrNotifiedExecution + " <> :exec)"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":exec": &dbtypes.AttributeValueMemberS{Value: executionID},
			":at":   &dbtypes.AttributeValueMemberS{Value: n.now().UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		var cond *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &cond) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// release undoes claim, only if this execution still holds it.
func (n *Notifier) release(ctx context.Context, userID, sortKey, executionID string) {
	_, err := n.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(n.ProjectsTable),
		Key:                 key("USER#"+userID, sortKey),
		UpdateExpression:    aws.String("REMOVE " + attrNotifiedExecution + ", " + attrNotifiedAt),
		ConditionExpression: aws.String(attrNotifiedExecution + " = :exec"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":exec": &dbtypes.AttributeValueMemberS{Value: executionID},
		},
	})
	if err != nil {
		log.Printf("notify release_failed user=%s sk=%s err=%v", userID, sortKey, err)
	}
}

// recordMessageID stores the SES message id for support lookups. Best effort:
// the email is already sent, so a failure here is only logged.
func (n *Notifier) recordMessageID(ctx context.Context, userID, sortKey, executionID, messageID string) {
	if messageID == "" {
		return
	}
	_, err := n.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(n.ProjectsTable),
		Key:                 key("USER#"+userID, sortKey),
		UpdateExpression:    aws.String("SET " + attrNotifyMessageID + " = :mid"),
		ConditionExpression: aws.String(attrNotifiedExecution + " = :exec"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":mid":  &dbtypes.AttributeValueMemberS{Value: messageID},
			":exec": &dbtypes.AttributeValueMemberS{Value: executionID},
		},
	})
	if err != nil {
		log.Printf("notify record_message_id_failed user=%s sk=%s err=%v", userID, sortKey, err)
	}
}

func (n *Notifier) now() time.Time {
	if n.Now != nil {
		return n.Now()
	}
	return time.Now()
}

func key(pk, sk string) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: pk},
		"SK": &dbtypes.AttributeValueMemberS{Value: sk},
	}
}

func stringAttr(item map[string]dbtypes.AttributeValue, name string) string {
	if v, ok := item[name].(*dbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}
