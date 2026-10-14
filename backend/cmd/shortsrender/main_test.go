package main

import (
	"testing"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

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
