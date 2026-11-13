package api

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/storymetrics"
)

// Issue #301: every "Make the video" request writes one story_video_start
// line, and one that made no job is counted by code.

var startLine = regexp.MustCompile(`story_video_start user=(\S+) plan=(\S+) outcome=(\S+) code=(\S+)\n`)

func startLines(t *testing.T, logs string) [][]string {
	t.Helper()
	return startLine.FindAllStringSubmatch(logs, -1)
}

func TestStoryVideoStart_EveryAttemptWritesOneLine(t *testing.T) {
	for _, c := range []struct {
		name          string
		setup         func(e *svEnv)
		body          string
		outcome, code string
		refusedMetric bool
	}{
		{"started", func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &p)
		}, createBody(svPlanA, 1, ""), "started", "-", false},
		{"a scene without a picture", func(e *svEnv) {
			p := planWithPictures("en")
			p.Scenes[1].Media = nil
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &p)
		}, createBody(svPlanA, 1, ""), "refused", "MISSING_PICTURES", true},
		{"a stale revision", func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 5, storyPlanReady, &p)
		}, createBody(svPlanA, 1, ""), "refused", "STALE_REVISION", true},
		{"no such plan", func(e *svEnv) {}, createBody(svPlanA, 1, ""), "refused", "NOT_FOUND", true},
		{"already running", func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &p)
			e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
		}, createBody(svPlanA, 1, ""), "refused", "ALREADY_RUNNING", true},
		{"a bad body", func(e *svEnv) {}, `{not json`, "refused", "INVALID_BODY", true},
		{"the job could not be written", func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &p)
			e.putErr = errors.New("down")
		}, createBody(svPlanA, 1, ""), "error", "DB_ERROR", true},
		{"the pipeline would not start", func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &p)
			e.startErr = errors.New("states: throttled")
		}, createBody(svPlanA, 1, ""), "error", "PIPELINE_ERROR", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			logs := captureLog(t)
			e := newSVEnv(t)
			m := withMetrics(e.h)
			c.setup(e)
			e.do("POST", "/story-videos", c.body, svGoogleUser)
			lines := startLines(t, logs.String())
			if len(lines) != 1 {
				t.Fatalf("%d story_video_start lines, want 1:\n%s", len(lines), logs.String())
			}
			l := lines[0]
			if l[1] != svGoogleUser || l[3] != c.outcome || l[4] != c.code {
				t.Errorf("line %q, want user %s outcome %s code %s", l[0], svGoogleUser, c.outcome, c.code)
			}
			wantPlan := svPlanA
			if c.body == `{not json` {
				wantPlan = "-"
			}
			if l[2] != wantPlan {
				t.Errorf("plan = %q, want %q", l[2], wantPlan)
			}
			got := m.read(t)
			if refused := got.totals[storymetrics.VideoStartsRefused]; (refused == 1) != c.refusedMetric {
				t.Errorf("VideoStartsRefused = %v, want counted=%v", refused, c.refusedMetric)
			}
			if c.refusedMetric {
				found := false
				for _, code := range got.codes {
					found = found || code == c.code
				}
				if !found {
					t.Errorf("codes %v, want %s", got.codes, c.code)
				}
			}
		})
	}
}

// The line never repeats what the client sent, except a plan id that is a
// valid one: a hostile planId cannot forge fields or log lines.
func TestStoryVideoStart_TheLineCarriesNoClientTextOrPersonalData(t *testing.T) {
	logs := captureLog(t)
	e := newSVEnv(t)
	p := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &p)
	e.do("POST", "/story-videos", `{"planId":"x outcome=started\nstory_video_start user=evil","revision":1}`, svGoogleUser)
	e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser)
	out := logs.String()
	if strings.Contains(out, "evil") || strings.Contains(out, "x outcome=") {
		t.Errorf("client text reached the log:\n%s", out)
	}
	for _, l := range startLines(t, out) {
		if strings.Contains(l[0], "@") {
			t.Errorf("an email reached the start line: %s", l[0])
		}
	}
	if got := len(startLines(t, out)); got != 2 {
		t.Errorf("%d lines, want 2", got)
	}
	for _, narration := range p.Scenes {
		if narration.Narration != "" && strings.Contains(out, narration.Narration) {
			t.Error("narration reached the log")
		}
	}
}
