package notifier

// The Lambda runtime reports a returned error to Step Functions under the
// error's Go type name (pointer stripped). The state machines match on these
// names, so the type names below are part of the contract:
//
//	Throttled   retried by the state machine with backoff
//	SendFailed  not retried; the state machine's Catch continues the job
//
// Neither can change a job's outcome: the notify states sit after the final
// status update and are wrapped in a Catch.

// Throttled means SES refused or failed the send in a way that is safe to
// retry: nothing was sent.
type Throttled struct{ Err error }

func (e *Throttled) Error() string { return "throttled: " + e.Err.Error() }
func (e *Throttled) Unwrap() error { return e.Err }

// SendFailed means the email was not sent and retrying will not help, or the
// outcome is unknown and a retry could send a duplicate.
type SendFailed struct{ Err error }

func (e *SendFailed) Error() string { return "send failed: " + e.Err.Error() }
func (e *SendFailed) Unwrap() error { return e.Err }
