package storyjob

import (
	"errors"
	"fmt"
	"regexp"
)

// uuidPattern is the shape of a job id, which this system makes itself.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// userIDPattern is what a user id may look like. It is NOT only a UUID: a user
// who signs in with Google has Google's own numeric id (about 21 digits), and
// only a password signup gets a UUID. Both end up in the same places, so the
// rule is what is safe in a storage key and a database key: letters, digits, '-'
// and '_', and no '/', '.', '#', '*', space or newline.
var userIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidUserID reports whether id can be used as the user part of a storage key
// and a database key.
func ValidUserID(id string) bool { return userIDPattern.MatchString(id) }

// ValidJobID reports whether id is a job id: a UUID, as this system makes them.
func ValidJobID(id string) bool { return uuidPattern.MatchString(id) }

// WorkerInput is what the render task is told by Step Functions.
type WorkerInput struct {
	UserID string
	JobID  string
}

// ParseWorkerInput checks the ids the task was started with. They end up in
// storage keys and database keys, so anything that could build a key like
// "../another-user/" is refused first.
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
	if !ValidUserID(userID) {
		return WorkerInput{}, fmt.Errorf("USER_ID is not a valid id")
	}
	if !ValidJobID(jobID) {
		return WorkerInput{}, fmt.Errorf("JOB_ID is not a valid id")
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
