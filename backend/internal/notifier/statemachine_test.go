package notifier

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// These tests read the three places a pipeline definition lives and check
// the notify wiring: the shorts definition deployed by setup-shorts.sh, the
// quiz definition deployed by setup.sh (the live source), and the quiz
// reference copy under internal/pipeline. The job outcome must be recorded
// before any email, a notify failure must never fail the execution, and the
// retry must cover throttling only.

type machine struct {
	StartAt string                    `json:"StartAt"`
	States  map[string]map[string]any `json:"States"`
}

func loadMachine(t *testing.T, raw []byte) machine {
	t.Helper()
	var m machine
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("definition is not valid JSON: %v", err)
	}
	return m
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// quizSetupDefinition extracts the heredoc in deploy/setup.sh the way bash
// evaluates it (variables filled in, "\$" unescaped).
func quizSetupDefinition(t *testing.T) []byte {
	t.Helper()
	src := string(readFile(t, "../../../deploy/setup.sh"))
	const open = "STATE_MACHINE=$(cat << EOF\n"
	i := strings.Index(src, open)
	if i < 0 {
		t.Fatal("state machine heredoc not found in deploy/setup.sh")
	}
	rest := src[i+len(open):]
	j := strings.Index(rest, "\nEOF\n)")
	if j < 0 {
		t.Fatal("end of state machine heredoc not found in deploy/setup.sh")
	}
	body := rest[:j]
	body = strings.NewReplacer(
		"${REGION}", "ap-south-1",
		"${ACCOUNT_ID}", "438097524343",
		"${PROJECT}", "indifferent-fun",
		`\$`, "$",
	).Replace(body)
	if strings.Contains(body, "${") {
		t.Fatalf("unsubstituted shell variable in heredoc: %s", body[strings.Index(body, "${"):][:40])
	}
	return []byte(body)
}

func TestNotifyWiring(t *testing.T) {
	cases := []struct {
		name         string
		raw          func(t *testing.T) []byte
		kind         string
		idKey        string // parameter naming the job
		originalEnds []string
	}{
		{"quiz deployed by setup.sh", quizSetupDefinition, KindQuiz, "projectId", []string{"MarkCompleted", "MarkFailed"}},
		{"quiz reference file", func(t *testing.T) []byte { return readFile(t, "../pipeline/statemachine.json") }, KindQuiz, "projectId", []string{"MarkCompleted", "MarkFailed"}},
		{"shorts deployed by setup-shorts.sh", func(t *testing.T) []byte { return readFile(t, "../shorts/pipeline/statemachine.json") }, KindShorts, "jobId", []string{"MarkCompleted", "MarkFailed"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := loadMachine(t, tc.raw(t))

			for _, pair := range [][2]string{{"MarkCompleted", "NotifyCompleted"}, {"MarkFailed", "NotifyFailed"}} {
				mark, notify := m.States[pair[0]], m.States[pair[1]]
				if mark == nil || notify == nil {
					t.Fatalf("missing state %s or %s", pair[0], pair[1])
				}
				// The status update stays first, and its (empty) result must not
				// replace the execution data the notify state reads.
				if mark["Next"] != pair[1] {
					t.Errorf("%s.Next = %v, want %s", pair[0], mark["Next"], pair[1])
				}
				if _, hasEnd := mark["End"]; hasEnd {
					t.Errorf("%s must no longer end the execution", pair[0])
				}
				if v, present := mark["ResultPath"]; !present || v != nil {
					t.Errorf("%s.ResultPath must be present and null so the input flows on, got %v (present=%v)", pair[0], v, present)
				}

				if notify["Type"] != "Task" || notify["End"] != true {
					t.Errorf("%s must be a terminal Task", pair[1])
				}
				if res, _ := notify["Resource"].(string); !strings.Contains(res, "notifier") && !strings.Contains(res, "NotifierArn") {
					t.Errorf("%s resource %q does not point at the notifier", pair[1], res)
				}
				if v, present := notify["ResultPath"]; !present || v != nil {
					t.Errorf("%s.ResultPath should discard the result", pair[1])
				}

				params := notify["Parameters"].(map[string]any)
				wantOutcome := strings.TrimPrefix(pair[1], "Notify")
				if params["kind"] != tc.kind || params["outcome"] != strings.ToLower(wantOutcome) {
					t.Errorf("%s kind/outcome = %v/%v", pair[1], params["kind"], params["outcome"])
				}
				if params["userId.$"] != "$.userId" || params[tc.idKey+".$"] != "$."+tc.idKey {
					t.Errorf("%s must pass userId and %s from the execution input: %v", pair[1], tc.idKey, params)
				}
				if params["executionId.$"] != "$$.Execution.Id" {
					t.Errorf("%s executionId = %v, want $$.Execution.Id", pair[1], params["executionId.$"])
				}
				for k := range params {
					if strings.Contains(k, "error") {
						t.Errorf("%s must never pass the raw error to the notifier (%s)", pair[1], k)
					}
				}
				assertParamsMatchInput(t, pair[1], params)

				retry := notify["Retry"].([]any)
				if len(retry) != 1 {
					t.Fatalf("%s: want exactly one Retry rule, got %d", pair[1], len(retry))
				}
				r := retry[0].(map[string]any)
				if !reflect.DeepEqual(r["ErrorEquals"], []any{"Throttled"}) || r["MaxAttempts"] != float64(4) {
					t.Errorf("%s retry = %v: only Throttled, 4 attempts", pair[1], r)
				}

				catch := notify["Catch"].([]any)
				c := catch[0].(map[string]any)
				if len(catch) != 1 || !reflect.DeepEqual(c["ErrorEquals"], []any{"States.ALL"}) || c["Next"] != "NotifyEnd" {
					t.Errorf("%s must catch States.ALL and continue to NotifyEnd: %v", pair[1], catch)
				}
			}

			end := m.States["NotifyEnd"]
			if end == nil || end["Type"] != "Pass" || end["End"] != true {
				t.Errorf("NotifyEnd must be a terminal Pass state: %v", end)
			}

			// Terminal states: only the notify trio, never a Fail state, so a
			// failed job still ends the execution the way it did before (the
			// failure is recorded on the job item, not by failing the run).
			var terminals []string
			for name, s := range m.States {
				if s["End"] == true {
					terminals = append(terminals, name)
				}
				if s["Type"] == "Fail" {
					t.Errorf("top-level Fail state %s would change execution outcomes", name)
				}
			}
			got := map[string]bool{}
			for _, n := range terminals {
				got[n] = true
			}
			want := map[string]bool{"NotifyCompleted": true, "NotifyFailed": true, "NotifyEnd": true}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("terminal states = %v, want %v", got, want)
			}

			assertAllReachable(t, m)
		})
	}
}

// assertParamsMatchInput keeps the state machine and notifier.Input in step.
func assertParamsMatchInput(t *testing.T, state string, params map[string]any) {
	t.Helper()
	allowed := map[string]bool{}
	typ := reflect.TypeOf(Input{})
	for i := 0; i < typ.NumField(); i++ {
		allowed[strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	for k := range params {
		if !allowed[strings.TrimSuffix(k, ".$")] {
			t.Errorf("%s passes %q, which notifier.Input does not accept", state, k)
		}
	}
}

func assertAllReachable(t *testing.T, m machine) {
	t.Helper()
	seen := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		s, ok := m.States[name]
		if !ok {
			t.Errorf("transition to unknown state %q", name)
			return
		}
		seen[name] = true
		if n, ok := s["Next"].(string); ok {
			visit(n)
		}
		if def, ok := s["Default"].(string); ok {
			visit(def)
		}
		for _, key := range []string{"Choices", "Catch"} {
			if list, ok := s[key].([]any); ok {
				for _, item := range list {
					if n, ok := item.(map[string]any)["Next"].(string); ok {
						visit(n)
					}
				}
			}
		}
	}
	visit(m.StartAt)
	for name := range m.States {
		if !seen[name] {
			t.Errorf("state %q is unreachable", name)
		}
	}
}

// MarkFailed reads $.error. A route into it that does not set $.error makes
// MarkFailed itself fail with States.Runtime: the job is never marked failed
// and, because the notify state comes after it, never emailed. Catch rules
// set it through ResultPath; Choice branches cannot, so they must go through
// a Pass state that does.
func TestEveryRouteIntoMarkFailedCarriesAnError(t *testing.T) {
	cases := []struct {
		name string
		raw  func(t *testing.T) []byte
	}{
		{"quiz deployed by setup.sh", quizSetupDefinition},
		{"quiz reference file", func(t *testing.T) []byte { return readFile(t, "../pipeline/statemachine.json") }},
		{"shorts deployed by setup-shorts.sh", func(t *testing.T) []byte { return readFile(t, "../shorts/pipeline/statemachine.json") }},
	}
	asList := func(v any) []map[string]any {
		var out []map[string]any
		if list, ok := v.([]any); ok {
			for _, item := range list {
				out = append(out, item.(map[string]any))
			}
		}
		return out
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := loadMachine(t, tc.raw(t))
			routes := 0
			for name, s := range m.States {
				if name == "MarkFailed" {
					continue
				}
				for _, c := range asList(s["Catch"]) {
					if c["Next"] != "MarkFailed" {
						continue
					}
					routes++
					if c["ResultPath"] != "$.error" {
						t.Errorf("%s: Catch into MarkFailed must set ResultPath $.error, has %v", name, c["ResultPath"])
					}
				}
				for _, c := range asList(s["Choices"]) {
					if c["Next"] == "MarkFailed" {
						routes++
						t.Errorf("%s: a Choice branch goes straight to MarkFailed with no $.error; route it through a Pass state that sets $.error", name)
					}
				}
				if s["Default"] == "MarkFailed" {
					routes++
					t.Errorf("%s: Default goes straight to MarkFailed with no $.error; route it through a Pass state that sets $.error", name)
				}
				if s["Next"] == "MarkFailed" {
					routes++
					result, _ := s["Result"].(map[string]any)
					if s["Type"] != "Pass" || s["ResultPath"] != "$.error" || result["Error"] == nil || result["Cause"] == nil {
						t.Errorf("%s: Next MarkFailed must be a Pass state with ResultPath $.error and a Result holding Error and Cause", name)
					}
				}
			}
			if routes == 0 {
				t.Fatal("found no route into MarkFailed; the test is not looking at the right thing")
			}
		})
	}
}

// The transcription poll loop (Wait -> Check -> Choice -> Wait) checks every
// shortsPollSeconds. A shorter wait means a finished transcription is noticed
// sooner (on average half the interval) at the price of a few more cheap state
// transitions per job; see issue #137. The loop shape must not change.
const shortsPollSeconds = 5

func TestShortsTranscriptionPollLoop(t *testing.T) {
	m := loadMachine(t, readFile(t, "../shorts/pipeline/statemachine.json"))

	wait := m.States["WaitForTranscription"]
	if wait == nil || wait["Type"] != "Wait" {
		t.Fatalf("WaitForTranscription must be a Wait state, got %v", wait)
	}
	if wait["Seconds"] != float64(shortsPollSeconds) {
		t.Errorf("WaitForTranscription.Seconds = %v, want %d", wait["Seconds"], shortsPollSeconds)
	}
	if wait["Next"] != "CheckTranscription" {
		t.Errorf("WaitForTranscription.Next = %v, want CheckTranscription", wait["Next"])
	}
	if m.States["CheckTranscription"]["Next"] != "TranscriptionDone?" {
		t.Errorf("CheckTranscription must lead to the TranscriptionDone? choice")
	}

	// IN_PROGRESS must still loop back to the wait.
	looped := false
	for _, c := range m.States["TranscriptionDone?"]["Choices"].([]any) {
		choice := c.(map[string]any)
		if choice["StringEquals"] == "IN_PROGRESS" && choice["Next"] == "WaitForTranscription" {
			looped = true
		}
	}
	if !looped {
		t.Error("TranscriptionDone? must send IN_PROGRESS back to WaitForTranscription")
	}
}
