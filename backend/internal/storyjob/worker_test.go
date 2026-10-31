package storyjob

import (
	"errors"
	"strings"
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

// A user who signs in with Google has Google's numeric id as their user id, not
// a UUID (internal/auth/google.go uses the userinfo "id" as is). A worker that
// accepted only UUIDs refused every one of them with exit 2.
func TestParseWorkerInput_AcceptsTheIdsRealUsersHave(t *testing.T) {
	job := "01939f6c-8a40-7e21-b2c4-aa11bb22cc33"
	for name, user := range map[string]string{
		"a Google id":               "114373949285061923844",
		"a UUID":                    "0b5e4a1c-7d2f-4a8e-9c31-5f6a7b8c9d0e",
		"an upper case UUID":        "0B5E4A1C-7D2F-4A8E-9C31-5F6A7B8C9D0E",
		"letters and an underscore": "user_42-a",
		"the longest allowed":       strings.Repeat("a", 64),
	} {
		in, err := ParseWorkerInput(user, job)
		if err != nil || in.UserID != user {
			t.Errorf("%s: %q: %+v, %v", name, user, in, err)
		}
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
		"a dot":                 {"a.b", good},
		"too long":              {strings.Repeat("a", 65), good},
		"a wildcard":            {"*", good},
		"a dynamodb-ish prefix": {"USER#" + good, good},
		"a hash":                {"a#b", good},
		"a colon":               {"a:b", good},
		"non-ascii letters":     {"ünï", good},
		// A job id is made by this system and is always a UUID.
		"a job id that is not a UUID": {good, "114373949285061923844"},
		"a short job id":              {good, "0b5e4a1c-7d2f-4a8e-9c31"},
		"a job id with a suffix":      {good, good + "0"},
		"a job id that is not hex":    {good, "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"},
	} {
		if _, err := ParseWorkerInput(tc[0], tc[1]); err == nil {
			t.Errorf("%s: accepted %q / %q", name, tc[0], tc[1])
		}
	}
}

func TestValidIDs_AreTheSameRulesTheWorkerUses(t *testing.T) {
	if !ValidUserID("114373949285061923844") || ValidUserID("") || ValidUserID("a/b") {
		t.Error("ValidUserID")
	}
	if !ValidJobID("01939f6c-8a40-7e21-b2c4-aa11bb22cc33") || ValidJobID("114373949285061923844") || ValidJobID("") {
		t.Error("ValidJobID")
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
