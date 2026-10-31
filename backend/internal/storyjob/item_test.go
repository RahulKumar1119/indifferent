package storyjob

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

func fullJob() Job {
	return Job{
		UserID: "u1", JobID: "j1", PlanID: "plan-9", PlanRevision: 4,
		Status: StatusRendering, Progress: 57, Language: "hi", Voice: "Aditi",
		Caption: models.CaptionStyle{Preset: "karaoke", FontSize: 24, Color: "#FFFFFF", Highlight: "#FFD60A", Position: "middle"},
		Scenes: []Scene{
			{
				Narration: "लाइटहाउस चट्टान पर अकेला खड़ा था।",
				Pick:      storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: "101"},
				Media:     &Media{Key: "storyvideo-work/u1/j1/scene-01-media.jpg", Kind: "photo", Format: "image/jpeg", Width: 1280, Height: 1920, Credit: "Photo 101 by Jo on Pixabay", PageURL: "https://pixabay.com/p/101"},
				Voice:     &Voice{AudioKey: "storyvideo-work/u1/j1/scene-01-voice.mp3", MarksKey: "storyvideo-work/u1/j1/scene-01-marks.jsonl", Seconds: 2.6645},
			},
			{Narration: "Second scene.", Pick: storyplan.MediaRef{Provider: "pexels", Type: "video", ID: "202"}},
		},
		TotalSeconds: 31.45, OutputKey: "storyvideo/u1/j1/video.mp4",
		Error: "Scene 2: the picture is no longer available from Pexels.", ErrorCode: CodeMediaUnavailable, FailedStage: StatusPreparing,
		CreatedAt: "2026-10-31T12:00:00Z", UpdatedAt: "2026-10-31T12:05:00Z", CompletedAt: "2026-10-31T12:06:00Z",
	}
}

func TestItem_RoundTripLosesNothing(t *testing.T) {
	in := fullJob()
	item, err := ToItem(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := FromItem(item)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip changed the job:\n in  %+v\n out %+v", in, out)
	}
}

func TestItem_AMinimalJobRoundTripsToo(t *testing.T) {
	in := Job{UserID: "u", JobID: "j", Status: StatusQueued, CreatedAt: "t", UpdatedAt: "t"}
	item, _ := ToItem(in)
	out, err := FromItem(item)
	if err != nil {
		t.Fatal(err)
	}
	in.Scenes = nil
	if out.UserID != "u" || out.JobID != "j" || out.Status != StatusQueued || len(out.Scenes) != 0 {
		t.Errorf("out = %+v", out)
	}
}

func TestItem_IsStoredBesideTheUsersOtherRecordsUnderItsOwnSortKey(t *testing.T) {
	item, _ := ToItem(fullJob())
	if pk := item["PK"].(*types.AttributeValueMemberS).Value; pk != "USER#u1" {
		t.Errorf("PK = %s", pk)
	}
	if sk := item["SK"].(*types.AttributeValueMemberS).Value; sk != "STORYVIDEO#j1" {
		t.Errorf("SK = %s", sk)
	}
	lo, hi := SortKeyRange()
	if !(lo <= "STORYVIDEO#j1" && "STORYVIDEO#j1" <= hi) {
		t.Errorf("SK is outside the query range %q..%q", lo, hi)
	}
	// It must not fall in the ranges of the neighbours.
	for _, other := range []string{"SHORTS#x", "STORYPLAN#x", "PROJECT#x"} {
		if lo <= other && other <= hi {
			t.Errorf("the range %q..%q contains %q", lo, hi, other)
		}
	}
	if !reflect.DeepEqual(Key("u1", "j1"), map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: "USER#u1"}, "SK": &types.AttributeValueMemberS{Value: "STORYVIDEO#j1"}}) {
		t.Error("Key does not match the item's key")
	}
}

func TestItem_NeverHoldsANarrationOrPictureAsAnAttributeOfItsOwn(t *testing.T) {
	// Scenes live in one JSON attribute: DynamoDB items are limited to 400 KB
	// and a long story must not grow the number of attributes.
	item, _ := ToItem(fullJob())
	if _, ok := item["scenes"].(*types.AttributeValueMemberS); !ok {
		t.Error("scenes should be a single JSON string")
	}
	if !strings.Contains(item["scenes"].(*types.AttributeValueMemberS).Value, "लाइटहाउस") {
		t.Error("non-Latin text did not survive")
	}
}

func TestFromItem_RefusesWhatItCannotReadInsteadOfGuessing(t *testing.T) {
	good, _ := ToItem(fullJob())
	with := func(name string, v types.AttributeValue) map[string]types.AttributeValue {
		m := map[string]types.AttributeValue{}
		for k, val := range good {
			m[k] = val
		}
		m[name] = v
		return m
	}
	for name, item := range map[string]map[string]types.AttributeValue{
		"another kind of record": with("SK", &types.AttributeValueMemberS{Value: "SHORTS#j1"}),
		"scenes not JSON":        with("scenes", &types.AttributeValueMemberS{Value: "{nope"}),
		"caption not JSON":       with("caption", &types.AttributeValueMemberS{Value: "[1,"}),
		"progress not a number":  with("progress", &types.AttributeValueMemberN{Value: "x"}),
		"length not a number":    with("totalSeconds", &types.AttributeValueMemberN{Value: "1.2.3"}),
		"empty":                  {},
	} {
		if _, err := FromItem(item); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
