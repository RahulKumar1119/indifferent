package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
)

// fakeRenderUpdater records the last UpdateItem call.
type fakeRenderUpdater struct {
	in *dynamodb.UpdateItemInput
}

func (f *fakeRenderUpdater) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.in = in
	return &dynamodb.UpdateItemOutput{}, errors.New("throttled")
}

func TestRecordRenderError(t *testing.T) {
	db := &fakeRenderUpdater{}
	recordRenderError(db, "table", "u1", "j1", errors.New(strings.Repeat("x", 400)))
	if db.in == nil {
		t.Fatal("expected the render error to be written")
	}
	if got := db.in.Key["PK"].(*dbtypes.AttributeValueMemberS).Value; got != "USER#u1" {
		t.Errorf("wrong PK %q", got)
	}
	if db.in.ConditionExpression == nil || *db.in.ConditionExpression != "attribute_exists(PK) AND attribute_not_exists(renderError)" {
		t.Errorf("the write must not recreate a deleted job or overwrite a reason, got %v", db.in.ConditionExpression)
	}
	if got := db.in.ExpressionAttributeValues[":reason"].(*dbtypes.AttributeValueMemberS).Value; len([]rune(got)) != maxRenderErrorChars {
		t.Errorf("reason must be capped at %d characters, got %d", maxRenderErrorChars, len([]rune(got)))
	}
}

// fakeRenderStore applies the render error's write condition the way DynamoDB
// does: a write is refused when the job already has a reason.
type fakeRenderStore struct {
	reason string
	writes int
}

func (f *fakeRenderStore) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if in.ConditionExpression != nil && strings.Contains(*in.ConditionExpression, "attribute_not_exists(renderError)") && f.reason != "" {
		return nil, &dbtypes.ConditionalCheckFailedException{Message: aws.String("The conditional request failed")}
	}
	f.reason = in.ExpressionAttributeValues[":reason"].(*dbtypes.AttributeValueMemberS).Value
	f.writes++
	return &dynamodb.UpdateItemOutput{}, nil
}

func TestRecordRenderErrorKeepsTheFirstReason(t *testing.T) {
	store := &fakeRenderStore{}
	recordRenderError(store, "table", "u1", "j1", errors.New("clip-1: ffmpeg render failed: exit status 1"))
	recordRenderError(store, "table", "u1", "j1", errors.New("clip-2: render timed out"))
	if store.reason != "clip-1: ffmpeg render failed: exit status 1" {
		t.Errorf("the first reason must stay, got %q", store.reason)
	}
	if store.writes != 1 {
		t.Errorf("only the first failure may be written, got %d writes", store.writes)
	}
}

// Issue #287: every failed render stores a short reason a person can read.
// The technical cause goes to the task log only.
func TestRenderErrorMessage(t *testing.T) {
	ffmpegErr := errors.New(`ffmpeg failed: exit status 234
[Parsed_crop_0 @ 0x7fe0] Invalid too big or non positive size for width '1424' or height '2532'`)
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"an ffmpeg failure", fmt.Errorf("ffmpeg render failed: %w", ffmpegErr), renderFailedMessage},
		{"a download failure", errors.New("download source: s3 GetObject: AccessDenied"), renderFailedMessage},
		{"a timeout", fmt.Errorf("%w after 2m0s: signal: killed", render.ErrRenderTimeout), renderTimeoutMessage},
		{"invalid effects", fmt.Errorf("%w: freeze at 40s is outside the clip", render.ErrEffectsInvalid), renderEffectsMessage},
		{"a cancel stores nothing", fmt.Errorf("%w: context canceled", render.ErrRenderCanceled), ""},
	}
	for _, c := range cases {
		got := renderErrorMessage(c.err)
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	for _, m := range []string{renderFailedMessage, renderTimeoutMessage, renderEffectsMessage} {
		if m == "" || len([]rune(m)) > maxRenderErrorChars {
			t.Errorf("message %q must be short and present", m)
		}
		if strings.ContainsAny(m, "{}[]@\n") || regexp.MustCompile(`(?i)ffmpeg|exit status|/tmp|crop|s3`).MatchString(m) {
			t.Errorf("message %q leaks technical detail", m)
		}
	}
}

func TestLogCauseIsBounded(t *testing.T) {
	long := errors.New(strings.Repeat("é", maxLoggedRenderCauses+10))
	if got := []rune(logCause(long)); len(got) != maxLoggedRenderCauses+3 {
		t.Errorf("logCause kept %d runes", len(got))
	}
	if logCause(errors.New("short")) != "short" {
		t.Error("a short cause is logged whole")
	}
}

func TestCaptionFromItem(t *testing.T) {
	if got := captionFromItem(nil); got != (models.CaptionStyle{}) {
		t.Errorf("nil item must resolve to zero style, got %+v", got)
	}
	if got := captionFromItem(map[string]dbtypes.AttributeValue{}); got != (models.CaptionStyle{}) {
		t.Errorf("missing attr must resolve to zero style, got %+v", got)
	}
	if got := captionFromItem(map[string]dbtypes.AttributeValue{
		"caption": &dbtypes.AttributeValueMemberS{Value: "not-json"},
	}); got != (models.CaptionStyle{}) {
		t.Errorf("corrupt value must resolve to zero style, got %+v", got)
	}
	got := captionFromItem(map[string]dbtypes.AttributeValue{
		"caption": &dbtypes.AttributeValueMemberS{Value: `{"preset":"karaoke","fontSize":24,"position":"top"}`},
	})
	if got.Preset != "karaoke" || got.FontSize != 24 || got.Position != "top" {
		t.Errorf("valid value must decode, got %+v", got)
	}
}

func TestEffectsFromItem(t *testing.T) {
	if got, err := effectsFromItem(nil); got != nil || err != nil {
		t.Errorf("nil item must mean no effects, got %+v, %v", got, err)
	}
	if got, err := effectsFromItem(map[string]dbtypes.AttributeValue{}); got != nil || err != nil {
		t.Errorf("missing attr must mean no effects, got %+v, %v", got, err)
	}
	if _, err := effectsFromItem(map[string]dbtypes.AttributeValue{
		"effects": &dbtypes.AttributeValueMemberS{Value: "not-json"},
	}); err == nil {
		t.Error("corrupt value must be an error, not a silent render without effects")
	}
	got, err := effectsFromItem(map[string]dbtypes.AttributeValue{
		"effects": &dbtypes.AttributeValueMemberS{Value: `{"grade":{"enabled":true,"look":"warm","intensity":60}}`},
	})
	if err != nil || got == nil || got.Grade == nil || !got.Grade.Enabled || got.Grade.Look != "warm" || got.Grade.Intensity != 60 {
		t.Errorf("valid value must decode, got %+v, %v", got, err)
	}
}
