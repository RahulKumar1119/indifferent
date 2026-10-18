// Package notifier sends one email when a quiz-video or AI Shorts job finishes
// or fails. It runs as the last state of both Step Functions pipelines, after
// the job's final status has been written, and is wrapped in a Catch there:
// whatever happens in this package, a job's outcome never changes.
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
)

// Attributes the notifier writes on the job item (projects table).
const (
	attrNotifiedExecution = "notifiedExecution"
	attrNotifiedAt        = "notifiedAt"
	attrNotifyMessageID   = "notifyMessageId"
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
// ProjectID (kind "quiz") or JobID (kind "shorts") is used. ExecutionID is the
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
	ProjectsTable string // jobs: PK=USER#<id>, SK=PROJECT#<id> | SHORTS#<id>
	UsersTable    string // profiles: PK=USER#<id>, SK=PROFILE
	BaseURL       string // site origin used for links, for example https://indifferent.fun
	Now           func() time.Time
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
	if in.Kind == KindShorts && in.Outcome == OutcomeFailed && stringAttr(job, "errorCode") == "CANCELLED" {
		return n.skip(in, "cancelled_by_user"), nil
	}

	content, err := Render(in.Kind, in.Outcome, stringAttr(job, "name"), id, n.BaseURL)
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
	}
	return "", fmt.Errorf("unknown kind %q", in.Kind)
}

func jobID(in Input) string {
	if in.Kind == KindShorts {
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
