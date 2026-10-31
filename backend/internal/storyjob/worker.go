package storyjob

import (
	"errors"
	"fmt"
	"regexp"
)

// uuidPattern is the shape of a user id and a job id.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// WorkerInput is what the render task is told by Step Functions.
type WorkerInput struct {
	UserID string
	JobID  string
}

// ParseWorkerInput checks the ids the task was started with. They end up in
// storage keys and database keys, so anything that is not a UUID is refused
// before it can build a key like "../another-user/".
func ParseWorkerInput(userID, jobID string) (WorkerInput, error) {
	var missing []string
	if userID == "" {
		missing = append(missing, "USER_ID")
	}
	if jobID == "" {
		missing = append(missing, "JOB_ID")
	}
	if len(missing) > 0 {
		return WorkerInput{}, fmt.Errorf("missing environment: %v", missing)
	}
	for name, v := range map[string]string{"USER_ID": userID, "JOB_ID": jobID} {
		if !uuidPattern.MatchString(v) {
			return WorkerInput{}, fmt.Errorf("%s is not a valid id", name)
		}
	}
	return WorkerInput{UserID: userID, JobID: jobID}, nil
}

// ExitCode is the process exit code for an outcome. It is what Step Functions
// sees, and so what decides whether the task is run again:
//
//   - completed, failed (the reason is on the record), and cancelled all exit 0,
//     since running them again would change nothing; the state machine reads
//     the record to find out which it was;
//   - retry exits 1, so Step Functions runs the task again.
func ExitCode(o Outcome) int {
	if o == OutcomeRetry {
		return 1
	}
	return 0
}

// IsUserFailure reports whether err is a failure that was recorded for the user
// to fix, as opposed to one that a retry may clear.
func IsUserFailure(err error) bool {
	var se *StepError
	return errors.As(err, &se) && !se.Retryable
}
