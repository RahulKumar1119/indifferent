package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/videocopy"
)

type fakeHookWriter struct {
	mu    sync.Mutex
	calls int
	reqs  []videocopy.Request
	fits  []func(string) bool
	res   videocopy.HookResult
	err   error
}

func (f *fakeHookWriter) SuggestHooks(_ context.Context, r videocopy.Request, fits func(string) bool) (videocopy.HookResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.reqs = append(f.reqs, r)
	f.fits = append(f.fits, fits)
	return f.res, f.err
}

func newHookEnv(t *testing.T) (*pcEnv, *fakeHookWriter) {
	e := newPCEnv(t)
	w := &fakeHookWriter{res: videocopy.HookResult{Hooks: []string{"I replaced the cracked cable", "hook one", "A safe socket lit the room"}, Rejected: map[string]int{"unsupported_claim": 2}, Model: "m"}}
	e.h.HookWriter = w
	e.h.ClipRenderStateMachineARN = "arn:aws:states:ap-south-1:123456789012:stateMachine:clip-render"
	return e, w
}

func (e *pcEnv) usedOf(q quota) int64 {
	it := e.db.get(pcQuota, "QUOTA#"+pcUser, q.name+"#20261120")
	if it == nil {
		return 0
	}
	return pcN(it["uses"])
}

func TestParseClipAction(t *testing.T) {
	good := map[string]string{
		"/shorts/j1/clips/clip-1/hook":             "hook",
		"/shorts/j1/clips/clip-1/hook-suggestions": "hook-suggestions",
		"/shorts/j1/clips/clip-1/render":           "render",
	}
	for p, want := range good {
		a, ok := parseClipAction(p)
		if !ok || a.action != want || a.jobID != "j1" || a.clipID != "clip-1" {
			t.Errorf("%s: %+v %v", p, a, ok)
		}
	}
	for _, p := range []string{"/shorts/j1/clips/clip-1", "/shorts/j1/clips/clip-1/captions", "/shorts/j#1/clips/c/hook", "/shorts/j1/clips/c#x/render", "/shorts//clips/c/hook", "/shorts/j1/clips/c/hook/x"} {
		if _, ok := parseClipAction(p); ok {
			t.Errorf("%s should not match", p)
		}
	}
}

func TestHookGet_ShowsDefaultsTheRankerLineAndAllowances(t *testing.T) {
	e, _ := newHookEnv(t)
	resp := e.do("GET", "/shorts/job1/clips/clip-1/hook", "", pcUser)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var v HookView
	if err := json.Unmarshal([]byte(resp.Body), &v); err != nil {
		t.Fatal(err)
	}
	if v.Saved || v.Hook.Enabled || v.Hook.Style != models.HookStyleBox || v.Hook.Position != models.HookTop || v.Hook.Seconds != 2.5 {
		t.Fatalf("defaults: %+v", v.Hook)
	}
	if v.Ranker != "hook one" || !v.SuggestionsEnabled || !v.RenderEnabled || v.MaxChars != 80 || v.RenderDailyLimit != 15 || v.SuggestDailyLimit != 20 {
		t.Fatalf("view: %+v", v)
	}
	if v.Render != nil {
		t.Fatalf("no render yet: %+v", v.Render)
	}
}

func TestClipRoutes_AnotherUsersClipIsNotFound(t *testing.T) {
	e, w := newHookEnv(t)
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/shorts/job1/clips/clip-1/hook", ""},
		{"PUT", "/shorts/job1/clips/clip-1/hook", `{"hook":{"enabled":true,"text":"x"}}`},
		{"POST", "/shorts/job1/clips/clip-1/hook-suggestions", `{"requestId":"` + rid(1) + `"}`},
		{"POST", "/shorts/job1/clips/clip-1/render", `{"requestId":"` + rid(1) + `"}`},
	} {
		if resp := e.do(c.method, c.path, c.body, pcOther); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s by another user: %d %s", c.method, c.path, resp.StatusCode, resp.Body)
		}
		if resp := e.do(c.method, c.path, c.body, ""); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without a token: %d", c.method, c.path, resp.StatusCode)
		}
	}
	if w.calls != 0 || e.usedOf(quotaCopyHelper) != 0 || e.usedOf(quotaShortsRetry) != 0 {
		t.Fatal("a refused request must not call the model or use an allowance")
	}
}

func TestHookSuggest_ServesCheckedLinesBesideTheRankerLine(t *testing.T) {
	e, w := newHookEnv(t)
	resp := e.do("POST", "/shorts/job1/clips/clip-1/hook-suggestions", `{"requestId":"`+rid(1)+`"}`, pcUser)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var out HookSuggestResponse
	_ = json.Unmarshal([]byte(resp.Body), &out)
	// The ranker's own line is not repeated among the suggestions.
	if strings.Join(out.Suggestions, "|") != "I replaced the cracked cable|A safe socket lit the room" || out.Ranker != "hook one" || out.Dropped != 2 {
		t.Fatalf("response %+v", out)
	}
	if e.usedOf(quotaCopyHelper) != 1 || out.RemainingToday == nil || *out.RemainingToday != 19 {
		t.Fatalf("used %d remaining %v", e.usedOf(quotaCopyHelper), out.RemainingToday)
	}
	// The model reads only this clip's words: 0 to 30 s of the transcript.
	if len(w.reqs) != 1 || len(w.reqs[0].Words) == 0 || w.reqs[0].Words[len(w.reqs[0].Words)-1].Start > 30 {
		t.Fatalf("model input: %+v", w.reqs)
	}
	// Lines are checked against the frame the way the hook is drawn.
	if w.fits[0](strings.Repeat("W", 60)) || !w.fits[0]("Short and fine") {
		t.Fatal("fits must refuse a line that cannot be drawn and accept one that can")
	}

	// The same request again: the same answer, nothing spent, no model call.
	again := e.do("POST", "/shorts/job1/clips/clip-1/hook-suggestions", `{"requestId":"`+rid(1)+`"}`, pcUser)
	var replay HookSuggestResponse
	_ = json.Unmarshal([]byte(again.Body), &replay)
	if again.StatusCode != http.StatusOK || !replay.Replayed || len(replay.Suggestions) != 2 || w.calls != 1 || e.usedOf(quotaCopyHelper) != 1 {
		t.Fatalf("replay %d %+v calls %d used %d", again.StatusCode, replay, w.calls, e.usedOf(quotaCopyHelper))
	}
	// The id cannot be reused for another clip.
	if other := e.do("POST", "/shorts/job1/clips/clip-2/hook-suggestions", `{"requestId":"`+rid(1)+`"}`, pcUser); other.StatusCode != http.StatusConflict || !strings.Contains(other.Body, "REQUEST_ID_REUSED") {
		t.Fatalf("reused id: %d %s", other.StatusCode, other.Body)
	}
}

func TestHookSuggest_ModelFailureGivesTheUseBack(t *testing.T) {
	e, w := newHookEnv(t)
	w.err = videocopy.ErrInvalidAnswer
	resp := e.do("POST", "/shorts/job1/clips/clip-1/hook-suggestions", `{"requestId":"`+rid(2)+`"}`, pcUser)
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(resp.Body, "did not use up") {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if e.usedOf(quotaCopyHelper) != 0 {
		t.Fatalf("used %d, want the use given back", e.usedOf(quotaCopyHelper))
	}
	// The ranker's line is still there to use.
	get := e.do("GET", "/shorts/job1/clips/clip-1/hook", "", pcUser)
	if !strings.Contains(get.Body, `"ranker":"hook one"`) {
		t.Fatalf("ranker line after a failed call: %s", get.Body)
	}
}

func TestHookSuggest_QuotaExhaustedIsReportedAndNothingIsCalled(t *testing.T) {
	e, w := newHookEnv(t)
	withLimit(t, &quotaCopyHelper, 1)
	if resp := e.do("POST", "/shorts/job1/clips/clip-1/hook-suggestions", `{"requestId":"`+rid(3)+`"}`, pcUser); resp.StatusCode != http.StatusOK {
		t.Fatalf("first: %d", resp.StatusCode)
	}
	resp := e.do("POST", "/shorts/job1/clips/clip-1/hook-suggestions", `{"requestId":"`+rid(4)+`"}`, pcUser)
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(resp.Body, "QUOTA_EXCEEDED") || !strings.Contains(resp.Body, "writing titles and descriptions") {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if w.calls != 1 {
		t.Fatalf("model calls %d", w.calls)
	}
}

func TestHookSuggest_OffAnswers503(t *testing.T) {
	e, _ := newHookEnv(t)
	e.h.HookWriter = nil
	if resp := e.do("POST", "/shorts/job1/clips/clip-1/hook-suggestions", `{"requestId":"`+rid(5)+`"}`, pcUser); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if get := e.do("GET", "/shorts/job1/clips/clip-1/hook", "", pcUser); !strings.Contains(get.Body, `"suggestionsEnabled":false`) {
		t.Fatalf("GET: %s", get.Body)
	}
}

// Every refusal below comes before the claim, the quota and Step Functions: it
// costs nothing and starts nothing.
func TestClipRender_RefusalsCostNothing(t *testing.T) {
	e, _ := newHookEnv(t)
	started := 0
	e.h.SFN = &mockSFN{startExecutionFunc: func(context.Context, *sfn.StartExecutionInput, ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
		started++
		return &sfn.StartExecutionOutput{}, nil
	}}
	cases := []struct {
		name, path, body string
		setup            func()
		want             int
		code             string
	}{
		{"bad id", "/shorts/job1/clips/clip-1/render", `{"requestId":"short"}`, nil, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"bad body", "/shorts/job1/clips/clip-1/render", `{`, nil, http.StatusBadRequest, "INVALID_BODY"},
		{"unknown clip", "/shorts/job1/clips/clip-9/render", `{"requestId":"` + rid(1) + `"}`, nil, http.StatusNotFound, "NOT_FOUND"},
		{"unknown job", "/shorts/job9/clips/clip-1/render", `{"requestId":"` + rid(1) + `"}`, nil, http.StatusNotFound, "NOT_FOUND"},
		{"no source key", "/shorts/job1/clips/clip-1/render", `{"requestId":"` + rid(1) + `"}`, nil, http.StatusConflict, "NOT_RENDERABLE"},
		{"not finished", "/shorts/job1/clips/clip-1/render", `{"requestId":"` + rid(1) + `"}`, func() {
			j := itemToShortsJob(e.db.get(pcTable, "USER#"+pcUser, "SHORTS#job1"))
			j.Status = "rendering"
			e.db.put(pcTable, shortsJobToItem(j))
		}, http.StatusConflict, "NOT_READY"},
		{"off", "/shorts/job1/clips/clip-1/render", `{"requestId":"` + rid(1) + `"}`, func() { e.h.ClipRenderStateMachineARN = "" }, http.StatusServiceUnavailable, "UNAVAILABLE"},
	}
	for _, c := range cases {
		if c.setup != nil {
			c.setup()
		}
		resp := e.do("POST", c.path, c.body, pcUser)
		if resp.StatusCode != c.want || !strings.Contains(resp.Body, c.code) {
			t.Errorf("%s: %d %s", c.name, resp.StatusCode, resp.Body)
		}
	}
	if e.usedOf(quotaShortsRetry) != 0 || started != 0 || e.db.count(pcTable, "USER#", "CLIPRENDER#") != 0 {
		t.Fatal("a refused render must not be charged, claimed or started")
	}
}

func TestClipRender_UnfitHookIsRefusedBeforeAnyCharge(t *testing.T) {
	e, _ := newHookEnv(t)
	j := itemToShortsJob(e.db.get(pcTable, "USER#"+pcUser, "SHORTS#job1"))
	j.SourceKey = "uploads/" + pcUser + "/job1/source.mp4"
	j.ClipHooks = map[string]models.ClipHook{"clip-1": {Enabled: true, Text: strings.Repeat("W", 60), Style: "box", Position: "top", Seconds: 2}}
	e.db.put(pcTable, shortsJobToItem(j))
	resp := e.do("POST", "/shorts/job1/clips/clip-1/render", `{"requestId":"`+rid(1)+`"}`, pcUser)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(resp.Body, "too long to fit on two lines") {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if e.usedOf(quotaShortsRetry) != 0 {
		t.Fatal("charged")
	}
}

func TestRankerHook_OnlyWhenUsableAsAHook(t *testing.T) {
	job := &models.ShortsJob{Clips: []models.Clip{{ClipID: "a", HookText: "  fine line "}, {ClipID: "b", HookText: strings.Repeat("x", 81)}, {ClipID: "c", HookText: "two\nlines"}}}
	if rankerHook(job, "a") != "fine line" || rankerHook(job, "b") != "" || rankerHook(job, "c") != "" || rankerHook(job, "zz") != "" {
		t.Fatal("ranker hook filtering")
	}
}

func TestHookCheckMessage(t *testing.T) {
	if !strings.Contains(hookCheckMessage(errors.New("x")), "cannot be drawn") {
		t.Fatal("fallback message")
	}
}
