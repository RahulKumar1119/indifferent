package storyplan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Issue #299: an unusable plan says which check failed, and why its scenes
// were dropped, so failures on real stories can be diagnosed from the log.

func TestPlanFailure_NamesTheCheckThatFailed(t *testing.T) {
	tooLong := englishPlan(3)
	for i := range tooLong.Scenes {
		tooLong.Scenes[i].Narration = strings.Repeat("word ", 60) // 24s of speech, over the 18s a scene may hold
	}
	noKeywords := englishPlan(3)
	for i := range noKeywords.Scenes {
		noKeywords.Scenes[i].Keywords = []string{"कीवर्ड"}
	}
	mixed := englishPlan(4)
	mixed.Scenes[0].Narration = strings.Repeat("word ", 60)
	mixed.Scenes[1].Narration = strings.Repeat("word ", 60)
	mixed.Scenes[2].Keywords = nil
	short := planJSON{Language: "en", Hook: "h", Scenes: []sceneJSON{englishScene(1), englishScene(2)}}
	for i := range short.Scenes {
		short.Scenes[i].Narration, short.Scenes[i].Duration = "One two three.", 2
	}

	for _, c := range []struct {
		name, answer, script string
		reason               string
		drops                map[string]int
	}{
		{"prose", "I'm sorry, I can't help with that.", "latin", ReasonNoJSON, nil},
		{"truncated", mustJSON(t, englishPlan(6))[:80], "latin", ReasonNoJSON, nil},
		{"not a plan", `{"hook":"x","scenes":"three"}`, "latin", ReasonBadShape, nil},
		{"empty scenes", `{"hook":"x","scenes":[]}`, "latin", ReasonNoScenes, nil},
		{"narration too long", mustJSON(t, tooLong), "latin", ReasonTooFew, map[string]int{DropTooLong: 3}},
		{"wrong language", mustJSON(t, hindiPlan(3)), "latin", ReasonTooFew, map[string]int{DropWrongScript: 3}},
		{"no usable keywords", mustJSON(t, noKeywords), "latin", ReasonTooFew, map[string]int{DropNoKeywords: 3}},
		{"mixed drops", mustJSON(t, mixed), "latin", ReasonTooFew, map[string]int{DropTooLong: 2, DropNoKeywords: 1}},
		{"too short", mustJSON(t, short), "latin", ReasonTooShort, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := quietService(&stubModel{answer: c.answer}).Generate(context.Background(), englishStory)
			if !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("err = %v, want ErrInvalidPlan", err)
			}
			reason, drops := PlanFailure(err)
			if reason != c.reason {
				t.Errorf("reason = %q, want %q (%v)", reason, c.reason, err)
			}
			for k, want := range c.drops {
				if drops[k] != want {
					t.Errorf("drops[%s] = %d, want %d (all: %v)", k, drops[k], want, drops)
				}
			}
		})
	}
	if r, _ := PlanFailure(errors.New("x")); r != ReasonOther {
		t.Errorf("a non-plan error has reason %q", r)
	}
}

func TestPlanFailure_IsLoggedAsCodesNotText(t *testing.T) {
	bad := englishPlan(3)
	for i := range bad.Scenes {
		bad.Scenes[i].Narration = "SECRET-NARRATION " + strings.Repeat("word ", 60)
	}
	var logs []string
	svc := NewService(&stubModel{answer: mustJSON(t, bad)}, "m", WithLogger(func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }))
	svc.Generate(context.Background(), englishStory)
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "outcome=invalid_plan reason=too_few drops=narration_too_long:3 answerRunes=") {
		t.Errorf("log: %s", joined)
	}
	if strings.Contains(joined, "SECRET") || strings.Contains(joined, "fisherman") {
		t.Errorf("text reached the log: %s", joined)
	}
}

func TestFormatDrops(t *testing.T) {
	if got := formatDrops(nil); got != "-" {
		t.Errorf("nil = %q", got)
	}
	got := formatDrops(map[string]int{DropNotObject: 1, DropTooLong: 2, DropWrongScript: 0})
	if got != "narration_too_long:2,not_object:1" {
		t.Errorf("got %q", got)
	}
}
