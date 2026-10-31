package storyjob

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

func planOf(lang string, pics ...bool) storyplan.Plan {
	p := storyplan.Plan{Language: lang, Hook: "h"}
	for i, has := range pics {
		s := storyplan.Scene{Narration: "Scene narration number " + string(rune('A'+i)) + ".", Description: "d", Keywords: []string{"k"}, MediaType: "photo", Duration: 5}
		if has {
			s.Media = &storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: string(rune('1' + i))}
		}
		p.Scenes = append(p.Scenes, s)
	}
	return p
}

var t0 = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func TestNewFromPlan_BuildsAQueuedJobThatKeepsTheTextAndPicks(t *testing.T) {
	plan := planOf("en", true, true, true)
	j, err := NewFromPlan("u1", "j1", "plan-9", 4, plan, "Matthew", models.CaptionStyle{Preset: "karaoke"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != StatusQueued || j.Progress != 0 || j.PlanID != "plan-9" || j.PlanRevision != 4 {
		t.Errorf("job = %+v", j)
	}
	if j.Voice != "Matthew" || j.Language != "en" || j.Caption.Preset != "karaoke" {
		t.Errorf("voice/language/caption = %q %q %+v", j.Voice, j.Language, j.Caption)
	}
	if j.CreatedAt != "2026-10-31T12:00:00Z" || j.UpdatedAt != j.CreatedAt {
		t.Errorf("timestamps = %s %s", j.CreatedAt, j.UpdatedAt)
	}
	for i, s := range j.Scenes {
		if s.Narration != plan.Scenes[i].Narration || s.Pick != *plan.Scenes[i].Media {
			t.Errorf("scene %d = %+v", i+1, s)
		}
		if s.Media != nil || s.Voice != nil {
			t.Errorf("scene %d already has files: %+v", i+1, s)
		}
	}
}

func TestNewFromPlan_IsASnapshot_EditingThePlanLaterChangesNothing(t *testing.T) {
	plan := planOf("en", true, true)
	j, _ := NewFromPlan("u", "j", "p", 1, plan, "", models.CaptionStyle{}, t0)
	plan.Scenes[0].Narration = "Rewritten after the video was started."
	plan.Scenes[0].Media.ID = "changed"
	if strings.Contains(j.Scenes[0].Narration, "Rewritten") || j.Scenes[0].Pick.ID == "changed" {
		t.Errorf("the job shares memory with the plan: %+v", j.Scenes[0])
	}
}

func TestNewFromPlan_NamesEverySceneThatHasNoPicture(t *testing.T) {
	_, err := NewFromPlan("u", "j", "p", 1, planOf("en", true, false, true, false), "", models.CaptionStyle{}, t0)
	var missing *MissingPicturesError
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Scenes, []int{2, 4}) {
		t.Fatalf("err = %v", err)
	}
	if got := missing.UserMessage(); got != "Scenes 2, 4 have no picture. Choose one for each and try again." {
		t.Errorf("message = %q", got)
	}
	_, err = NewFromPlan("u", "j", "p", 1, planOf("en", true, false), "", models.CaptionStyle{}, t0)
	if !errors.As(err, &missing) || missing.UserMessage() != "Scene 2 has no picture. Choose one for it and try again." {
		t.Errorf("single: %v", err)
	}
}

func TestNewFromPlan_RefusesAPlanThatCannotBeRendered(t *testing.T) {
	bad := planOf("en", true, true)
	bad.Scenes[1].Media = &storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: "not-a-number"}
	for name, tc := range map[string]struct {
		plan    storyplan.Plan
		voice   string
		caption models.CaptionStyle
	}{
		"no scenes":            {storyplan.Plan{Language: "en"}, "", models.CaptionStyle{}},
		"bad picture":          {bad, "", models.CaptionStyle{}},
		"unknown voice":        {planOf("en", true, true), "Nobody", models.CaptionStyle{}},
		"bad caption preset":   {planOf("en", true, true), "", models.CaptionStyle{Preset: "neon"}},
		"unsupported language": {planOf("fr", true, true), "", models.CaptionStyle{}},
	} {
		if _, err := NewFromPlan("u", "j", "p", 1, tc.plan, tc.voice, tc.caption, t0); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestVoiceFor(t *testing.T) {
	for _, tc := range []struct {
		lang, req, want string
		ok              bool
	}{
		{"en", "", "Joanna", true},
		{"en", "Matthew", "Matthew", true},
		{"en", "Aditi", "Aditi", true}, // Indian English
		{"hi", "", "Aditi", true},
		{"hi", "Aditi", "Aditi", true},
		{"hi", "Joanna", "", false}, // an English voice reads Devanagari as nothing
		{"hi", "Matthew", "", false},
		{"en", "Nobody", "", false},
		{"fr", "", "", false},
		{"mr", "Aditi", "", false}, // Marathi shares the script but not the language
		{"", "", "", false},
	} {
		got, err := VoiceFor(tc.lang, tc.req)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("VoiceFor(%q, %q) = %q, %v; want %q ok=%v", tc.lang, tc.req, got, err, tc.want, tc.ok)
		}
	}
	var unsupported *UnsupportedLanguageError
	_, err := VoiceFor("fr", "")
	if !errors.As(err, &unsupported) || !strings.Contains(unsupported.UserMessage(), "English and Hindi") {
		t.Errorf("err = %v", err)
	}
	if DefaultVoice("hi") != "Aditi" || DefaultVoice("zz") != "" {
		t.Error("DefaultVoice is wrong")
	}
	if v := Voices("en"); len(v) != 5 || v[0] != "Joanna" {
		t.Errorf("Voices(en) = %v", v)
	}
	// Changing the returned slice must not change the table.
	Voices("en")[0] = "x"
	if Voices("en")[0] != "Joanna" {
		t.Error("Voices returns the table itself")
	}
}

func TestEveryVoiceWeOfferIsOneThePollyNarratorKnows(t *testing.T) {
	for lang, voices := range voicesByLanguage {
		for _, v := range voices {
			if got, err := VoiceFor(lang, v); err != nil || got != v {
				t.Errorf("%s/%s: %q, %v", lang, v, got, err)
			}
		}
	}
}

func TestKeys_AreScopedToTheUserAndTheJobAndSortInSceneOrder(t *testing.T) {
	if got := WorkKey("u1", "j1", 3, "media.mp4"); got != "storyvideo-work/u1/j1/scene-03-media.mp4" {
		t.Errorf("WorkKey = %s", got)
	}
	if got := OutputKey("u1", "j1"); got != "storyvideo/u1/j1/video.mp4" {
		t.Errorf("OutputKey = %s", got)
	}
	// Two digits, so scene 10 sorts after scene 9 in a listing.
	if WorkKey("u", "j", 9, "x") >= WorkKey("u", "j", 10, "x") {
		t.Error("scene 10 sorts before scene 9")
	}
	// Working files expire; the finished video must not be under that prefix.
	if strings.HasPrefix(OutputKey("u", "j"), WorkPrefix) || strings.HasPrefix(WorkKey("u", "j", 1, "x"), OutputPrefix) {
		t.Error("the working and output prefixes overlap: a lifecycle rule on one would delete the other")
	}
}

func TestTerminal(t *testing.T) {
	for status, want := range map[string]bool{StatusQueued: false, StatusPreparing: false, StatusNarrating: false, StatusRendering: false, StatusCompleted: true, StatusFailed: true} {
		if Terminal(status) != want {
			t.Errorf("Terminal(%s) = %v", status, !want)
		}
	}
	// Progress only moves forward through the stages.
	prev := -1
	for _, s := range []string{StatusQueued, StatusPreparing, StatusNarrating, StatusRendering, StatusCompleted} {
		if StageProgress[s] <= prev {
			t.Errorf("progress at %s (%d) does not move forward", s, StageProgress[s])
		}
		prev = StageProgress[s]
	}
}
