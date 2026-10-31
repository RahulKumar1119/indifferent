//go:build live

package pipeline

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
)

// Run with: go test -tags live ./internal/storyjob/pipeline/
//
// These call Step Functions' own engine, with AWS credentials, but only two
// read-only APIs that create and change nothing:
//
//   - ValidateStateMachineDefinition checks the definition against the Amazon
//     States Language rules;
//   - TestState runs one state against the real engine with a mocked response
//     for the service it would call, and returns what the engine decided.
//
// No state machine, task, table or function is created, and the DynamoDB table
// named in the definition is never touched: its responses are mocked.

func client(t *testing.T) *sfn.Client {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("ap-south-1"))
	if err != nil {
		t.Skipf("no AWS configuration: %v", err)
	}
	return sfn.NewFromConfig(cfg)
}

func TestLive_StepFunctionsAcceptsTheDefinition(t *testing.T) {
	c := client(t)
	def, err := Render(params())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := c.ValidateStateMachineDefinition(ctx, &sfn.ValidateStateMachineDefinitionInput{
		Definition: aws.String(def), Type: sfntypes.StateMachineTypeStandard, Severity: sfntypes.ValidateStateMachineDefinitionSeverityWarning,
	})
	if err != nil {
		t.Skipf("the validate call failed (permissions or network?): %v", err)
	}
	for _, d := range out.Diagnostics {
		t.Logf("%s %s at %s: %s", d.Severity, aws.ToString(d.Code), aws.ToString(d.Location), aws.ToString(d.Message))
	}
	if out.Result != sfntypes.ValidateStateMachineDefinitionResultCodeOk {
		t.Errorf("result = %s", out.Result)
	}
}

// stateDef returns one state's definition as the TestState API wants it.
func stateDef(t *testing.T, name string) string {
	t.Helper()
	def, _ := Render(params())
	var m struct {
		States map[string]json.RawMessage `json:"States"`
	}
	if err := json.Unmarshal([]byte(def), &m); err != nil {
		t.Fatal(err)
	}
	return string(m.States[name])
}

func testState(t *testing.T, name, input string, mock *sfntypes.MockInput) *sfn.TestStateOutput {
	t.Helper()
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	in := &sfn.TestStateInput{
		Definition: aws.String(stateDef(t, name)), Input: aws.String(input),
		InspectionLevel: sfntypes.InspectionLevelDebug, Mock: mock,
	}
	// No role: Choice and Pass states need none, and a Task state with a mock
	// never calls the service it names. A made-up role is refused by the service.
	out, err := c.TestState(ctx, in)
	if err != nil {
		if strings.Contains(err.Error(), "is not authorized to perform: states:TestState") || strings.Contains(err.Error(), "UnrecognizedClient") {
			t.Skipf("TestState is not permitted here: %v", err)
		}
		t.Fatalf("TestState %s: %v", name, err)
	}
	return out
}

const ids = `{"userId":"0b5e4a1c-7d2f-4a8e-9c31-5f6a7b8c9d0e","jobId":"01939f6c-8a40-7e21-b2c4-aa11bb22cc33"}`

// withRead is the execution's data after a getItem stored its response.
func withRead(read string) string {
	return `{"userId":"u","jobId":"j","read":` + read + `}`
}

func item(status string) string {
	return `{"Item":{"status":{"S":"` + status + `"},"planId":{"S":"p"}}}`
}

func next(o *sfn.TestStateOutput) string { return aws.ToString(o.NextState) }

// resolved is the Task's parameters after the engine filled in every path.
func resolved(t *testing.T, o *sfn.TestStateOutput) string {
	t.Helper()
	if o.InspectionData == nil || o.InspectionData.AfterParameters == nil {
		t.Fatal("the engine did not return the resolved parameters")
	}
	return aws.ToString(o.InspectionData.AfterParameters)
}

func TestLive_TheEngineRoutesEachEndStateWhereItShould(t *testing.T) {
	for status, want := range map[string]string{"completed": "NotifyCompleted", "failed": "NotifyFailed", "rendering": "StillRunning", "queued": "StillRunning", "": "StillRunning"} {
		t.Run("HowDidItEnd status="+status, func(t *testing.T) {
			out := testState(t, "HowDidItEnd?", withRead(item(status)), nil)
			if next(out) != want {
				t.Errorf("the engine sent %q to %q, want %q (error %s: %s)", status, next(out), want, aws.ToString(out.Error), aws.ToString(out.Cause))
			}
		})
	}
	for status, want := range map[string]string{"completed": "NotifyCompleted", "failed": "NotifyFailed", "rendering": "Gone"} {
		t.Run("EndedHow status="+status, func(t *testing.T) {
			out := testState(t, "EndedHow?", withRead(item(status)), nil)
			if next(out) != want {
				t.Errorf("the engine sent %q to %q, want %q (error %s: %s)", status, next(out), want, aws.ToString(out.Error), aws.ToString(out.Cause))
			}
		})
	}
}

// getItem on a key that is not there answers {} with no Item. A path read from
// that raises States.Runtime, which a Catch cannot handle, so these are the
// cases that matter: each must reach Gone, not fail the execution.
func TestLive_AMissingOrOddRecordGoesToGoneNotToAFailure(t *testing.T) {
	for _, guard := range []struct{ state, onward string }{{"IsJobThere?", "HowDidItEnd?"}, {"IsJobStillThere?", "EndedHow?"}} {
		for name, c := range map[string]struct{ read, want string }{
			"no Item":                    {`{}`, "Gone"},
			"an Item with no status":     {`{"Item":{"planId":{"S":"p"}}}`, "Gone"},
			"a status of the wrong type": {`{"Item":{"status":{"N":"1"}}}`, "Gone"},
			"an Item with a status":      {item("completed"), guard.onward},
		} {
			t.Run(guard.state+" "+name, func(t *testing.T) {
				out := testState(t, guard.state, withRead(c.read), nil)
				if next(out) != c.want {
					t.Errorf("went to %q, want %q (error %s: %s)", next(out), c.want, aws.ToString(out.Error), aws.ToString(out.Cause))
				}
			})
		}
	}
}

func TestLive_ReadJobBuildsTheRightKeyAndKeepsTheResponse(t *testing.T) {
	out := testState(t, "ReadJob", ids, &sfntypes.MockInput{Result: aws.String(item("completed"))})
	if next(out) != "IsJobThere?" {
		t.Errorf("next = %q (error %s: %s)", next(out), aws.ToString(out.Error), aws.ToString(out.Cause))
	}
	var got struct {
		Read struct {
			Item struct {
				Status struct {
					S string
				}
			}
		}
	}
	if err := json.Unmarshal([]byte(aws.ToString(out.Output)), &got); err != nil || got.Read.Item.Status.S != "completed" {
		t.Errorf("output = %s (%v)", aws.ToString(out.Output), err)
	}
	// A missing record is an answer, not an error: ReadJob itself must carry it on.
	gone := testState(t, "ReadJob", ids, &sfntypes.MockInput{Result: aws.String(`{}`)})
	if next(gone) != "IsJobThere?" {
		t.Errorf("a missing record went to %q (error %s: %s)", next(gone), aws.ToString(gone.Error), aws.ToString(gone.Cause))
	}
	// What the engine resolved to send to DynamoDB: the key built from the ids.
	body := resolved(t, out)
	for _, want := range []string{`"PK":{"S":"USER#0b5e4a1c-7d2f-4a8e-9c31-5f6a7b8c9d0e"}`, `"SK":{"S":"STORYVIDEO#01939f6c-8a40-7e21-b2c4-aa11bb22cc33"}`, `"ConsistentRead":true`, `"TableName":"indifferent-fun-projects"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the request lacks %q:\n%s", want, body)
		}
	}
	// A read that fails outright is not a reason to fail the execution.
	failed := testState(t, "ReadJob", ids, &sfntypes.MockInput{ErrorOutput: &sfntypes.MockErrorOutput{Error: aws.String("DynamoDB.AmazonDynamoDBException"), Cause: aws.String("boom")}})
	if next(failed) != "Gone" {
		t.Errorf("a failed read went to %q", next(failed))
	}
}

func TestLive_MarkFailedBuildsTheExpressionAndHandlesARefusal(t *testing.T) {
	in := `{"userId":"u1","jobId":"j1","error":{"Error":"x","Cause":"y"}}`
	out := testState(t, "MarkFailed", in, &sfntypes.MockInput{Result: aws.String(`{}`)})
	if next(out) != "NotifyFailed" {
		t.Errorf("a successful mark went to %q (error %s: %s)", next(out), aws.ToString(out.Error), aws.ToString(out.Cause))
	}
	body := resolved(t, out)
	for _, want := range []string{`"PK":{"S":"USER#u1"}`, `"SK":{"S":"STORYVIDEO#j1"}`, "#error = if_not_exists(#error, :message)", "attribute_exists(PK) AND #status <> :completed AND #status <> :failed", `":failed":{"S":"failed"}`, `":completed":{"S":"completed"}`} {
		if !strings.Contains(body, want) {
			t.Errorf("the request lacks %q:\n%s", want, body)
		}
	}
	// $$.State.EnteredTime must come out as a timestamp, not be left as a path.
	if m := regexp.MustCompile(`":now":\{"S":"(\d{4}-\d\d-\d\dT[\d:.]+Z)"\}`).FindStringSubmatch(body); m == nil {
		t.Errorf(":now is not a timestamp:\n%s", body)
	} else if _, err := time.Parse(time.RFC3339, m[1]); err != nil {
		t.Errorf("the worker's code could not read the timestamp %q: %v", m[1], err)
	}

	// The exact error name Step Functions gives a DynamoDB conditional failure.
	refused := testState(t, "MarkFailed", in, &sfntypes.MockInput{ErrorOutput: &sfntypes.MockErrorOutput{
		Error: aws.String("DynamoDB.ConditionalCheckFailedException"), Cause: aws.String("The conditional request failed"),
	}})
	if next(refused) != "ReadJobAgain" {
		t.Errorf("a refused update went to %q (error %s)", next(refused), aws.ToString(refused.Error))
	}
	// Any other error is not swallowed.
	other := testState(t, "MarkFailed", in, &sfntypes.MockInput{ErrorOutput: &sfntypes.MockErrorOutput{
		Error: aws.String("DynamoDB.ProvisionedThroughputExceededException"), Cause: aws.String("slow down"),
	}})
	if next(other) == "ReadJobAgain" {
		t.Error("a throttled update was treated as 'already ended'")
	}
}

func TestLive_ARenderThatFailsOnSpotFallsBackToOnDemand(t *testing.T) {
	mock := func(e string) *sfntypes.MockInput {
		return &sfntypes.MockInput{ErrorOutput: &sfntypes.MockErrorOutput{Error: aws.String(e), Cause: aws.String("c")}}
	}
	// stateAt runs RunOnSpot with n retries already used, as the engine would on
	// its n+1th attempt.
	stateAt := func(e string, used int32) *sfn.TestStateOutput {
		c := client(t)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := c.TestState(ctx, &sfn.TestStateInput{
			Definition: aws.String(stateDef(t, "RunOnSpot")), Input: aws.String(ids), InspectionLevel: sfntypes.InspectionLevelDebug,
			Mock: mock(e), StateConfiguration: &sfntypes.TestStateConfiguration{RetrierRetryCount: aws.Int32(used)},
		})
		if err != nil {
			t.Fatalf("TestState: %v", err)
		}
		return out
	}

	// A failing task is retried while attempts remain, then falls to on-demand.
	first := stateAt("States.TaskFailed", 0)
	t.Logf("TaskFailed, no retries used: status=%s next=%q", first.Status, next(first))
	if first.Status != sfntypes.TestExecutionStatusRetriable {
		t.Errorf("a first failure is %s, want RETRIABLE", first.Status)
	}
	spent := stateAt("States.TaskFailed", 2)
	t.Logf("TaskFailed, both retries used: status=%s next=%q", spent.Status, next(spent))
	if next(spent) != "RunOnDemand" {
		t.Errorf("with the retries spent a failure went to %q, want RunOnDemand", next(spent))
	}

	// A timeout is not retried on Spot, even with every retry still unused.
	timedOut := stateAt("States.Timeout", 0)
	t.Logf("Timeout, no retries used: status=%s next=%q", timedOut.Status, next(timedOut))
	if next(timedOut) != "RunOnDemand" || timedOut.Status == sfntypes.TestExecutionStatusRetriable {
		t.Errorf("a timeout is %s and went to %q, want a move to RunOnDemand with no retry", timedOut.Status, next(timedOut))
	}

	last := testState(t, "RunOnDemand", ids, mock("States.TaskFailed"))
	if next(last) != "MarkFailed" {
		t.Errorf("the last attempt failing went to %q", next(last))
	}
}
