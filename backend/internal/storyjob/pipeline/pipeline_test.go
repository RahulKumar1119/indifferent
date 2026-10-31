package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func params() Params {
	return Params{
		EcsClusterArn:         "arn:aws:ecs:ap-south-1:123456789012:cluster/indifferent-fun-shorts",
		StoryRenderTaskDefArn: "arn:aws:ecs:ap-south-1:123456789012:task-definition/indifferent-fun-story-render",
		SubnetIds:             []string{"subnet-aaa", "subnet-bbb", "subnet-ccc"},
		SecurityGroupId:       "sg-0123456789abcdef0",
		JobsTable:             "indifferent-fun-projects",
		NotifierArn:           "arn:aws:lambda:ap-south-1:123456789012:function:indifferent-fun-notifier",
	}
}

type state = map[string]any

type machine struct {
	StartAt string           `json:"StartAt"`
	States  map[string]state `json:"States"`
}

func load(t *testing.T) machine {
	t.Helper()
	def, err := Render(params())
	if err != nil {
		t.Fatal(err)
	}
	var m machine
	if err := json.Unmarshal([]byte(def), &m); err != nil {
		t.Fatalf("the rendered definition is not JSON: %v", err)
	}
	return m
}

func str(s state, k string) string { v, _ := s[k].(string); return v }

// workerTimeoutSeconds reads the worker's own deadline from its source, so the
// state machine's limit is checked against the real number, not a copy of it.
func workerTimeoutSeconds(t *testing.T) float64 {
	t.Helper()
	src, err := os.ReadFile("../../../cmd/storyrender/main.go")
	if err != nil {
		t.Fatalf("reading the worker: %v", err)
	}
	m := regexp.MustCompile(`(?m)^const taskTimeout = (\d+) \* time\.(Minute|Second)$`).FindSubmatch(src)
	if m == nil {
		t.Fatal("could not find `const taskTimeout = N * time.Minute` in cmd/storyrender/main.go")
	}
	n, _ := strconv.Atoi(string(m[1]))
	if string(m[2]) == "Minute" {
		n *= 60
	}
	return float64(n)
}

func TestRender_FillsEveryPlaceholderAndProducesJSON(t *testing.T) {
	def, err := Render(params())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(def, "${") {
		t.Error("a placeholder is left")
	}
	if !json.Valid([]byte(def)) {
		t.Error("not JSON")
	}
	want := []string{"${EcsClusterArn}", "${JobsTable}", "${NotifierArn}", "${SecurityGroupId}", "${StoryRenderTaskDefArn}", "${SubnetIds}"}
	if got := Placeholders(); !reflect.DeepEqual(got, want) {
		t.Errorf("placeholders = %v, want %v", got, want)
	}
}

func TestRender_SubnetsBecomeAJSONArrayInBothTaskStates(t *testing.T) {
	m := load(t)
	for _, name := range []string{"RunOnSpot", "RunOnDemand"} {
		cfg := m.States[name]["Parameters"].(state)["NetworkConfiguration"].(state)["AwsvpcConfiguration"].(state)
		got, _ := cfg["Subnets"].([]any)
		if len(got) != 3 || got[0] != "subnet-aaa" || got[2] != "subnet-ccc" {
			t.Errorf("%s subnets = %v", name, got)
		}
		if cfg["SecurityGroups"].([]any)[0] != "sg-0123456789abcdef0" {
			t.Errorf("%s security groups = %v", name, cfg["SecurityGroups"])
		}
	}
}

func TestRender_RefusesBadInput(t *testing.T) {
	for name, mutate := range map[string]func(*Params){
		"no subnets":      func(p *Params) { p.SubnetIds = nil },
		"empty cluster":   func(p *Params) { p.EcsClusterArn = "" },
		"empty table":     func(p *Params) { p.JobsTable = "" },
		"empty notifier":  func(p *Params) { p.NotifierArn = "" },
		"a quote":         func(p *Params) { p.JobsTable = `x","Evil":"1` },
		"a backslash":     func(p *Params) { p.SecurityGroupId = `sg\x` },
		"a newline":       func(p *Params) { p.NotifierArn = "a\nb" },
		"a bad subnet":    func(p *Params) { p.SubnetIds = []string{`subnet-1","x":"y`} },
		"a subnet with $": func(p *Params) { p.SubnetIds = []string{"${Evil}"} },
	} {
		p := params()
		mutate(&p)
		if _, err := Render(p); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestMachine_EveryStateIsReachableAndEveryTransitionGoesSomewhere(t *testing.T) {
	m := load(t)
	if _, ok := m.States[m.StartAt]; !ok {
		t.Fatalf("StartAt %q is not a state", m.StartAt)
	}
	targets := func(s state) []string {
		var out []string
		if n := str(s, "Next"); n != "" {
			out = append(out, n)
		}
		if n := str(s, "Default"); n != "" {
			out = append(out, n)
		}
		for _, key := range []string{"Catch", "Choices"} {
			if arr, ok := s[key].([]any); ok {
				for _, e := range arr {
					if n := str(e.(state), "Next"); n != "" {
						out = append(out, n)
					}
				}
			}
		}
		return out
	}
	reached := map[string]bool{m.StartAt: true}
	queue := []string{m.StartAt}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, to := range targets(m.States[cur]) {
			if _, ok := m.States[to]; !ok {
				t.Errorf("%s goes to %q, which is not a state", cur, to)
				continue
			}
			if !reached[to] {
				reached[to] = true
				queue = append(queue, to)
			}
		}
	}
	for name := range m.States {
		if !reached[name] {
			t.Errorf("state %s can never be reached", name)
		}
	}
	// Every path ends.
	for name, s := range m.States {
		if s["End"] != true && str(s, "Next") == "" && s["Type"] != "Choice" {
			t.Errorf("%s has no way on and is not an End state", name)
		}
	}
}

func TestMachine_ARenderIsRetriedOnSpotThenTriedOnDemandThenMarkedFailed(t *testing.T) {
	m := load(t)
	spot, od := m.States["RunOnSpot"], m.States["RunOnDemand"]
	cap := func(s state) string {
		return s["Parameters"].(state)["CapacityProviderStrategy"].([]any)[0].(state)["CapacityProvider"].(string)
	}
	if cap(spot) != "FARGATE_SPOT" || cap(od) != "FARGATE" {
		t.Errorf("capacity: spot=%s on-demand=%s", cap(spot), cap(od))
	}
	retriers := spot["Retry"].([]any)
	if len(retriers) != 2 {
		t.Fatalf("Spot has %d retriers, want a timeout rule and the retry rule", len(retriers))
	}
	// The timeout rule comes first and allows no attempts. A stuck task must not
	// run again on Spot, and the wildcard States.TaskFailed below it must not be
	// left to decide whether it covers a timeout.
	timeout := retriers[0].(state)
	if fmt.Sprint(timeout["ErrorEquals"]) != "[States.Timeout]" || timeout["MaxAttempts"] == nil || timeout["MaxAttempts"].(float64) != 0 {
		t.Errorf("first retrier = %v: a timeout must have MaxAttempts 0", timeout)
	}
	retry := retriers[1].(state)
	errs := fmt.Sprint(retry["ErrorEquals"])
	if !strings.Contains(errs, "States.TaskFailed") || !strings.Contains(errs, "ECS.AmazonECSException") {
		t.Errorf("Spot retries %s", errs)
	}
	if strings.Contains(errs, "States.Timeout") || strings.Contains(errs, "States.ALL") {
		t.Errorf("Spot retries %s: a task that timed out is stuck and must not be run again on Spot", errs)
	}
	if retry["MaxAttempts"].(float64) != 2 {
		t.Errorf("Spot retries %v times", retry["MaxAttempts"])
	}
	if str(spot["Catch"].([]any)[0].(state), "Next") != "RunOnDemand" {
		t.Error("Spot failures do not fall back to on-demand")
	}
	if str(od["Catch"].([]any)[0].(state), "Next") != "MarkFailed" {
		t.Error("on-demand failures are not marked failed")
	}
	if _, retried := od["Retry"]; retried {
		t.Error("on-demand is the last attempt and should not retry")
	}
	// The task's own time limit is five minutes longer than the worker's
	// (cmd/storyrender taskTimeout, 15 minutes), so the worker's deadline ends a
	// stuck render first: it marks the job failed with a reason and exits 0. If
	// this limit came first the task would be killed with nothing saved. The
	// worker's value is read from its source so the two cannot drift apart.
	worker := workerTimeoutSeconds(t)
	for name, s := range map[string]state{"spot": spot, "on-demand": od} {
		got := s["TimeoutSeconds"].(float64)
		if got != 1200 {
			t.Errorf("%s TimeoutSeconds = %v", name, got)
		}
		if got < worker+120 {
			t.Errorf("%s TimeoutSeconds = %v is not clearly longer than the worker's %v s: the worker must time out first", name, got, worker)
		}
	}
}

func TestMachine_BothAttemptsAreGivenTheSameJobAndUser(t *testing.T) {
	m := load(t)
	for _, name := range []string{"RunOnSpot", "RunOnDemand"} {
		c := m.States[name]["Parameters"].(state)["Overrides"].(state)["ContainerOverrides"].([]any)[0].(state)
		if c["Name"] != "story-render" {
			t.Errorf("%s overrides container %v", name, c["Name"])
		}
		env := map[string]string{}
		for _, e := range c["Environment"].([]any) {
			kv := e.(state)
			env[str(kv, "Name")] = str(kv, "Value.$")
		}
		if env["USER_ID"] != "$.userId" || env["JOB_ID"] != "$.jobId" || len(env) != 2 {
			t.Errorf("%s environment = %v", name, env)
		}
	}
}

func TestMachine_TheOutcomeIsReadFromTheRecordNotInferredFromTheExit(t *testing.T) {
	m := load(t)
	for _, from := range []string{"RunOnSpot", "RunOnDemand"} {
		if str(m.States[from], "Next") != "ReadJob" {
			t.Errorf("%s goes to %s, not to ReadJob: exit 0 does not mean success", from, str(m.States[from], "Next"))
		}
	}
	for _, name := range []string{"ReadJob", "ReadJobAgain"} {
		if m.States[name]["Parameters"].(state)["ConsistentRead"] != true {
			t.Errorf("%s reads the job without a consistent read", name)
		}
	}
	if str(m.States["ReadJob"], "Next") != "IsJobThere?" {
		t.Errorf("ReadJob goes to %s: the record may not exist", str(m.States["ReadJob"], "Next"))
	}
	choice := m.States["HowDidItEnd?"]["Choices"].([]any)
	got := map[string]string{}
	for _, c := range choice {
		got[str(c.(state), "StringEquals")] = str(c.(state), "Next")
	}
	if got["completed"] != "NotifyCompleted" || got["failed"] != "NotifyFailed" || len(got) != 2 {
		t.Errorf("routing = %v", got)
	}
	if str(m.States["HowDidItEnd?"], "Default") != "StillRunning" {
		t.Error("a job that is not finished is not handled")
	}
	if str(m.States["StillRunning"], "Next") != "MarkFailed" {
		t.Error("an unfinished job is left unfinished")
	}
}

func TestMachine_MarkFailedKeepsTheWorkersReasonAndNeverOverwritesAnEnd(t *testing.T) {
	m := load(t)
	p := m.States["MarkFailed"]["Parameters"].(state)
	upd := p["UpdateExpression"].(string)
	for _, want := range []string{"#error = if_not_exists(#error, :message)", "#errorCode = if_not_exists(#errorCode, :code)", "#failedStage = if_not_exists(#failedStage, #status)", "#status = :failed"} {
		if !strings.Contains(upd, want) {
			t.Errorf("the update lacks %q:\n%s", want, upd)
		}
	}
	// The same condition the worker's own store uses: the record exists, and has not ended.
	if p["ConditionExpression"] != "attribute_exists(PK) AND #status <> :completed AND #status <> :failed" {
		t.Errorf("condition = %v", p["ConditionExpression"])
	}
	// DynamoDB rejects an expression name or value that is listed and not used.
	text := upd + " " + p["ConditionExpression"].(string)
	for n := range p["ExpressionAttributeNames"].(state) {
		if !strings.Contains(text, n) {
			t.Errorf("name %s is listed but unused", n)
		}
	}
	for v := range p["ExpressionAttributeValues"].(state) {
		if !strings.Contains(text, v) {
			t.Errorf("value %s is listed but unused", v)
		}
	}
	catch := m.States["MarkFailed"]["Catch"].([]any)[0].(state)
	if fmt.Sprint(catch["ErrorEquals"]) != "[DynamoDB.ConditionalCheckFailedException]" || str(catch, "Next") != "ReadJobAgain" {
		t.Errorf("a refused MarkFailed is handled by %v", catch)
	}
	// The generic reason must be one the user can read, with no internals.
	msg := p["ExpressionAttributeValues"].(state)[":message"].(state)["S"].(string)
	if !strings.Contains(msg, "our side") || strings.ContainsAny(msg, "/\\:{}") && strings.Contains(msg, "arn") {
		t.Errorf("generic reason = %q", msg)
	}
}

func TestMachine_AJobThatEndedWhileWeWereMarkingItIsReportedAsWhatItWas(t *testing.T) {
	m := load(t)
	choice := m.States["EndedHow?"]["Choices"].([]any)
	got := map[string]string{}
	for _, c := range choice {
		got[str(c.(state), "StringEquals")] = str(c.(state), "Next")
	}
	if got["completed"] != "NotifyCompleted" || got["failed"] != "NotifyFailed" {
		t.Errorf("routing = %v", got)
	}
	// Anything else (deleted, or still not final) sends nothing.
	if str(m.States["EndedHow?"], "Default") != "Gone" {
		t.Error("an unknown state is not treated as nothing to report")
	}
}

func TestMachine_EmailsNeverChangeTheOutcome(t *testing.T) {
	m := load(t)
	for _, name := range []string{"NotifyCompleted", "NotifyFailed"} {
		s := m.States[name]
		if s["Resource"] != "arn:aws:lambda:ap-south-1:123456789012:function:indifferent-fun-notifier" {
			t.Errorf("%s calls %v", name, s["Resource"])
		}
		p := s["Parameters"].(state)
		if p["kind"] != "story" || p["jobId.$"] != "$.jobId" || p["userId.$"] != "$.userId" || p["executionId.$"] != "$$.Execution.Id" {
			t.Errorf("%s parameters = %v", name, p)
		}
		if (name == "NotifyCompleted") != (p["outcome"] == "completed") {
			t.Errorf("%s sends outcome %v", name, p["outcome"])
		}
		catch := s["Catch"].([]any)[0].(state)
		if fmt.Sprint(catch["ErrorEquals"]) != "[States.ALL]" || str(catch, "Next") != "NotifyEnd" {
			t.Errorf("%s does not catch every error: %v", name, catch)
		}
		retry := s["Retry"].([]any)[0].(state)
		if fmt.Sprint(retry["ErrorEquals"]) != "[Throttled]" {
			t.Errorf("%s retries %v: only throttling should be retried", name, retry["ErrorEquals"])
		}
		if s["ResultPath"] != nil {
			t.Errorf("%s keeps the notifier's result", name)
		}
	}
}

func TestMachine_NoStateWritesAnythingButTheJobRecordsEndState(t *testing.T) {
	m := load(t)
	for name, s := range m.States {
		res := str(s, "Resource")
		if res == "arn:aws:states:::dynamodb:putItem" || res == "arn:aws:states:::dynamodb:deleteItem" {
			t.Errorf("%s uses %s: the machine must only read the job and mark it failed", name, res)
		}
		if strings.HasSuffix(res, "dynamodb:updateItem") && name != "MarkFailed" {
			t.Errorf("%s updates the database", name)
		}
	}
}

func TestMachine_InputIsJustTheTwoIds(t *testing.T) {
	// The API starts an execution with {userId, jobId}. Nothing else is read
	// from the input, so an extra field cannot change what is run. Every JSONPath
	// the definition uses is checked.
	var paths []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if k == "ResultSelector" {
					t.Error("a ResultSelector reads into a response that may lack the path, and States.Runtime cannot be caught")
				}
				if strings.HasSuffix(k, ".$") {
					if p, ok := val.(string); ok {
						paths = append(paths, p)
					}
				}
				walk(val)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	def, _ := Render(params())
	var root any
	if err := json.Unmarshal([]byte(def), &root); err != nil {
		t.Fatal(err)
	}
	walk(root)
	if len(paths) == 0 {
		t.Fatal("found no paths: the walk is broken")
	}
	for _, p := range paths {
		for _, ref := range regexp.MustCompile(`\$\$?\.[A-Za-z.]+`).FindAllString(p, -1) {
			switch ref {
			case "$.userId", "$.jobId", "$$.Execution.Id", "$$.State.EnteredTime":
			default:
				t.Errorf("the definition reads %s (in %q)", ref, p)
			}
		}
	}
}

// getItem on a key that is not there answers {} with no Item. A path read from
// it raises States.Runtime, which no Catch can handle (not even States.ALL), so
// the whole execution would fail. IsPresent is the one Choice rule that looks
// without raising.
func TestMachine_NothingReadsIntoARecordThatMayBeMissingUntilItHasBeenChecked(t *testing.T) {
	m := load(t)
	reads := func(s state) bool {
		b, _ := json.Marshal(s["Choices"])
		b2, _ := json.Marshal(s["Parameters"])
		return strings.Contains(string(b)+string(b2), "$.read.Item")
	}
	guards := func(s state) bool {
		if s["Type"] != "Choice" {
			return false
		}
		b, _ := json.Marshal(s["Choices"])
		txt := string(b)
		return strings.Contains(txt, `"IsPresent":true,"Variable":"$.read.Item"`) &&
			strings.Contains(txt, `"IsPresent":true,"Variable":"$.read.Item.status"`) &&
			strings.Contains(txt, `"IsPresent":true,"Variable":"$.read.Item.status.S"`)
	}
	next := func(s state) []string {
		var out []string
		for _, k := range []string{"Next", "Default"} {
			if n := str(s, k); n != "" {
				out = append(out, n)
			}
		}
		for _, key := range []string{"Catch", "Choices"} {
			if arr, ok := s[key].([]any); ok {
				for _, e := range arr {
					if n := str(e.(state), "Next"); n != "" {
						out = append(out, n)
					}
				}
			}
		}
		return out
	}
	// A guard's own state is allowed to read the path (that is what it checks).
	// Any other state that reads it must be entered only from a guard.
	found := 0
	for name, s := range m.States {
		if !reads(s) || guards(s) {
			continue
		}
		found++
		for from, fs := range m.States {
			for _, to := range next(fs) {
				if to == name && !guards(fs) {
					t.Errorf("%s reads $.read.Item but is entered from %s, which has not checked the record is there", name, from)
				}
			}
		}
	}
	if found < 2 {
		t.Errorf("found %d readers of the record (want HowDidItEnd? and EndedHow?): the walk is broken", found)
	}
	// Both guards send a missing record to Gone.
	for _, name := range []string{"IsJobThere?", "IsJobStillThere?"} {
		if !guards(m.States[name]) {
			t.Errorf("%s does not check every level of the path", name)
		}
		if str(m.States[name], "Default") != "Gone" {
			t.Errorf("%s: a missing record goes to %q, not Gone", name, str(m.States[name], "Default"))
		}
	}
}
