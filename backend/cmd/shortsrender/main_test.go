package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
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
	if db.in.ConditionExpression == nil || *db.in.ConditionExpression != "attribute_exists(PK)" {
		t.Errorf("the write must not recreate a deleted job, got %v", db.in.ConditionExpression)
	}
	if got := db.in.ExpressionAttributeValues[":reason"].(*dbtypes.AttributeValueMemberS).Value; len([]rune(got)) != maxRenderErrorChars {
		t.Errorf("reason must be capped at %d characters, got %d", maxRenderErrorChars, len([]rune(got)))
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
