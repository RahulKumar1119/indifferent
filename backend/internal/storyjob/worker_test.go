package storyjob

import (
	"errors"
	"testing"
)

func TestParseWorkerInput_AcceptsUUIDs(t *testing.T) {
	in, err := ParseWorkerInput("0b5e4a1c-7d2f-4a8e-9c31-5f6a7b8c9d0e", "01939f6c-8a40-7e21-b2c4-aa11bb22cc33")
	if err != nil || in.UserID == "" || in.JobID == "" {
		t.Errorf("got %+v, %v", in, err)
	}
	// Upper case is still a UUID.
	if _, err := ParseWorkerInput("0B5E4A1C-7D2F-4A8E-9C31-5F6A7B8C9D0E", "01939F6C-8A40-7E21-B2C4-AA11BB22CC33"); err != nil {
		t.Error(err)
	}
}

func TestParseWorkerInput_RefusesWhatCouldBuildAnUnsafeKey(t *testing.T) {
	good := "0b5e4a1c-7d2f-4a8e-9c31-5f6a7b8c9d0e"
	for name, tc := range map[string][2]string{
		"no user":               {"", good},
		"no job":                {good, ""},
		"neither":               {"", ""},
		"a path":                {"../other-user", good},
		"a path in the job":     {good, good + "/../../x"},
		"a slash":               {good + "/x", good},
		"a newline":             {good + "\n", good},
		"spaces":                {" " + good, good},
		"too short":             {"0b5e4a1c-7d2f-4a8e-9c31", good},
		"too long":              {good + "0", good},
		"not hex":               {"zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz", good},
		"a wildcard":            {"*", good},
		"a dynamodb-ish prefix": {"USER#" + good, good},
	} {
		if _, err := ParseWorkerInput(tc[0], tc[1]); err == nil {
			t.Errorf("%s: accepted %q / %q", name, tc[0], tc[1])
		}
	}
}

func TestParseWorkerInput_NamesWhatIsMissingButNeverEchoesTheValue(t *testing.T) {
	_, err := ParseWorkerInput("", "x")
	if err == nil || err.Error() != "missing environment: [USER_ID]" {
		t.Errorf("err = %v", err)
	}
	_, err = ParseWorkerInput("0b5e4a1c-7d2f-4a8e-9c31-5f6a7b8c9d0e", "secret-looking-value")
	if err == nil || contains(err.Error(), "secret-looking-value") {
		t.Errorf("err = %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// The mapping is the contract with the state machine, so it is pinned.
func TestExitCode_OnlyARetryFailsTheTask(t *testing.T) {
	want := map[Outcome]int{OutcomeCompleted: 0, OutcomeFailed: 0, OutcomeCancelled: 0, OutcomeRetry: 1}
	for o, code := range want {
		if got := ExitCode(o); got != code {
			t.Errorf("ExitCode(%v) = %d, want %d", o, got, code)
		}
	}
}

func TestIsUserFailure(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want bool
	}{
		"the user must change something": {&StepError{Retryable: false}, true},
		"a retry may fix it":             {&StepError{Retryable: true}, false},
		"not a step error":               {errors.New("x"), false},
		"nothing":                        {nil, false},
	} {
		if IsUserFailure(tc.err) != tc.want {
			t.Errorf("%s: %v", name, !tc.want)
		}
	}
}
