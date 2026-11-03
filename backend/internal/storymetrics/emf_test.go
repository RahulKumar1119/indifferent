package storymetrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"
)

// emfProblems checks one log line against the embedded metric format
// specification (docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/
// CloudWatch_Embedded_Metric_Format_Specification.html, read 2026-10-31), and
// returns what it breaks. CloudWatch drops a document that breaks the
// specification without any error, so a mistake here would show only as a
// metric that never appears; this is the check that stands in for seeing it.
//
// It follows the specification's JSON schema and its prose rules: the namespace
// and name patterns, the unit list, the dimension limits, "the line is a JSON
// object with nothing before or after it", targets that exist at the root and
// are not nested, and the age limit of the timestamp.
func emfProblems(line []byte, now time.Time) []string {
	var out []string
	bad := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }

	if !bytes.HasSuffix(line, []byte("\n")) || bytes.Count(line, []byte("\n")) != 1 {
		bad("the event must be exactly one line ending in a newline")
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSuffix(line, []byte("\n"))))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return append(out, "not a JSON object: "+err.Error())
	}
	if dec.More() {
		bad("data after the JSON object")
	}

	meta, ok := root["_aws"].(map[string]any)
	if !ok {
		return append(out, "no _aws object")
	}
	ts, ok := meta["Timestamp"].(json.Number)
	if !ok {
		bad("_aws.Timestamp is missing or not a number")
	} else if ms, err := ts.Int64(); err != nil {
		bad("_aws.Timestamp is not an integer: %s", ts)
	} else {
		t := time.UnixMilli(ms)
		if t.Before(now.Add(-14*24*time.Hour)) || t.After(now.Add(2*time.Hour)) {
			bad("_aws.Timestamp %v is outside the accepted range (14 days back, 2 hours ahead)", t)
		}
	}
	directives, ok := meta["CloudWatchMetrics"].([]any)
	if !ok || len(directives) == 0 {
		return append(out, "_aws.CloudWatchMetrics is missing or empty")
	}

	var (
		namespacePat = regexp.MustCompile(`^[ -~]*[!-~][ -~]*$`)
		dimKeyPat    = regexp.MustCompile(`^(?:[!-9;-~]| +[!-~])[ -~]*$`)
		unitPat      = regexp.MustCompile(`^(Seconds|Microseconds|Milliseconds|Bytes|Kilobytes|Megabytes|Gigabytes|Terabytes|Bits|Kilobits|Megabits|Gigabits|Terabits|Percent|Count|Bytes/Second|Kilobytes/Second|Megabytes/Second|Gigabytes/Second|Terabytes/Second|Bits/Second|Kilobits/Second|Megabits/Second|Gigabits/Second|Terabits/Second|Count/Second|None)$`)
		printable    = regexp.MustCompile(`^[ -~]+$`)
	)
	total := 0
	for i, d := range directives {
		dir, ok := d.(map[string]any)
		if !ok {
			bad("directive %d is not an object", i)
			continue
		}
		for k := range dir {
			if k != "Namespace" && k != "Dimensions" && k != "Metrics" {
				bad("directive %d has the member %q the specification does not define", i, k)
			}
		}
		ns, _ := dir["Namespace"].(string)
		if len(ns) < 1 || len(ns) > 255 || !namespacePat.MatchString(ns) {
			bad("directive %d: namespace %q is not valid", i, ns)
		}

		dimSets, ok := dir["Dimensions"].([]any)
		if !ok || len(dimSets) < 1 {
			bad("directive %d: Dimensions must be an array with at least one set (it may be empty)", i)
		}
		for j, ds := range dimSets {
			set, ok := ds.([]any)
			if !ok {
				bad("dimension set %d is not an array", j)
				continue
			}
			if len(set) > 30 {
				bad("dimension set %d has %d keys, over 30", j, len(set))
			}
			for _, kv := range set {
				key, _ := kv.(string)
				if len(key) < 1 || len(key) > 250 || !dimKeyPat.MatchString(key) {
					bad("dimension key %q is not valid", key)
					continue
				}
				val, isString := root[key].(string)
				switch {
				case !isString:
					bad("dimension %q is not a string member of the root node", key)
				case len(val) < 1 || len(val) > 1024 || !printable.MatchString(val) || strings.TrimSpace(val) == "":
					bad("dimension %q has the value %q, which is not 1 to 1024 printable ASCII characters", key, val)
				}
			}
		}

		defs, ok := dir["Metrics"].([]any)
		if !ok {
			bad("directive %d: Metrics is missing", i)
		}
		total += len(defs)
		for _, m := range defs {
			def, ok := m.(map[string]any)
			if !ok {
				bad("a metric definition is not an object")
				continue
			}
			name, _ := def["Name"].(string)
			if len(name) < 1 || len(name) > 255 || !namespacePat.MatchString(name) {
				bad("metric name %q is not valid", name)
			}
			if u, has := def["Unit"]; has {
				if us, _ := u.(string); !unitPat.MatchString(us) {
					bad("metric %q has the unit %v, which CloudWatch does not know", name, u)
				}
			}
			for k := range def {
				if k != "Name" && k != "Unit" && k != "StorageResolution" {
					bad("metric %q has the member %q the specification does not define", name, k)
				}
			}
			switch v := root[name].(type) {
			case json.Number:
				if f, err := v.Float64(); err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
					bad("metric %q has the value %s, which is not a finite number", name, v)
				}
			case nil:
				bad("metric %q has no value at the root of the document", name)
			default:
				bad("metric %q is %T at the root, not a number (a target cannot be nested or a string)", name, v)
			}
		}
	}
	if total > 100 {
		bad("%d metric definitions, over the limit of 100", total)
	}
	return out
}

// The validator is only worth trusting if it can fail. Each case here breaks one
// rule of the specification and must be reported.
func TestTheValidatorCatchesWhatCloudWatchWouldSilentlyDrop(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	ts := now.UnixMilli()
	good := fmt.Sprintf(`{"_aws":{"Timestamp":%d,"CloudWatchMetrics":[{"Namespace":"Indifferent/Story","Dimensions":[[]],"Metrics":[{"Name":"PlansReady","Unit":"Count"}]}]},"PlansReady":1}`, ts) + "\n"
	if p := emfProblems([]byte(good), now); len(p) != 0 {
		t.Fatalf("the validator rejects a valid document: %v", p)
	}
	rep := func(old, new string) string {
		if !strings.Contains(good, old) {
			t.Fatalf("test bug: %q not in the document", old)
		}
		return strings.Replace(good, old, new, 1)
	}
	for _, tc := range []struct{ name, doc, reason string }{
		{"no trailing newline", strings.TrimSuffix(good, "\n"), "exactly one line"},
		{"two lines", good + good, "exactly one line"},
		{"a log prefix", "2026/10/31 12:00:00 " + good, "not a JSON object"},
		{"text after the object", strings.TrimSuffix(good, "\n") + " x\n", "data after"},
		{"no timestamp", rep(fmt.Sprintf(`"Timestamp":%d,`, ts), ""), "Timestamp is missing"},
		{"timestamp as text", rep(fmt.Sprint(ts), fmt.Sprintf(`"%d"`, ts)), "Timestamp is missing or not a number"},
		{"timestamp in seconds", rep(fmt.Sprint(ts), fmt.Sprint(ts/1000)), "outside the accepted range"},
		{"timestamp 15 days old", rep(fmt.Sprint(ts), fmt.Sprint(now.Add(-15*24*time.Hour).UnixMilli())), "outside the accepted range"},
		{"timestamp 3 hours ahead", rep(fmt.Sprint(ts), fmt.Sprint(now.Add(3*time.Hour).UnixMilli())), "outside the accepted range"},
		{"empty namespace", rep(`"Indifferent/Story"`, `""`), "namespace"},
		{"namespace of only a space", rep(`"Indifferent/Story"`, `" "`), "namespace"},
		{"namespace not ASCII", rep(`"Indifferent/Story"`, `"Indifférent"`), "namespace"},
		{"no dimension sets", rep(`"Dimensions":[[]]`, `"Dimensions":[]`), "at least one set"},
		{"dimensions not nested", rep(`"Dimensions":[[]]`, `"Dimensions":["Code"]`), "is not an array"},
		{"dimension target missing", rep(`"Dimensions":[[]]`, `"Dimensions":[["Code"]]`), "not a string member"},
		{"dimension target a number", strings.Replace(rep(`"Dimensions":[[]]`, `"Dimensions":[["Code"]]`), `"PlansReady":1}`, `"PlansReady":1,"Code":5}`, 1), "not a string member"},
		{"dimension value empty", strings.Replace(rep(`"Dimensions":[[]]`, `"Dimensions":[["Code"]]`), `"PlansReady":1}`, `"PlansReady":1,"Code":""}`, 1), "printable ASCII"},
		{"dimension value not ASCII", strings.Replace(rep(`"Dimensions":[[]]`, `"Dimensions":[["Code"]]`), `"PlansReady":1}`, `"PlansReady":1,"Code":"é"}`, 1), "printable ASCII"},
		{"dimension key starts with a colon", strings.Replace(rep(`"Dimensions":[[]]`, `"Dimensions":[[":c"]]`), `"PlansReady":1}`, `"PlansReady":1,":c":"x"}`, 1), "dimension key"},
		{"unknown unit", rep(`"Count"`, `"Counts"`), "unit"},
		{"unknown directive member", rep(`"Namespace"`, `"Foo":1,"Namespace"`), "does not define"},
		{"unknown metric member", rep(`"Unit":"Count"`, `"Unit":"Count","Bar":2`), "does not define"},
		{"metric value missing", rep(`,"PlansReady":1}`, `}`), "no value at the root"},
		{"metric value as text", rep(`"PlansReady":1}`, `"PlansReady":"1"}`), "not a number"},
		{"metric value nested", rep(`"PlansReady":1}`, `"PlansReady":{"v":1}}`), "not a number"},
		{"metric name of only a space", rep(`"PlansReady","Unit"`, `" ","Unit"`), "metric name"},
		{"not JSON", "{ nope\n", "not a JSON object"},
		{"no _aws", `{"PlansReady":1}` + "\n", "no _aws"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := emfProblems([]byte(tc.doc), now)
			if len(p) == 0 {
				t.Fatalf("the validator accepted a document that breaks the specification:\n%s", tc.doc)
			}
			if !strings.Contains(strings.Join(p, "; "), tc.reason) {
				t.Errorf("rejected for the wrong reason:\n got  %v\n want a message containing %q", p, tc.reason)
			}
		})
	}
}

func TestTheValidatorCatchesTooManyMetrics(t *testing.T) {
	now := time.Now()
	var defs, vals []string
	for i := 0; i < 101; i++ {
		defs = append(defs, fmt.Sprintf(`{"Name":"M%d"}`, i))
		vals = append(vals, fmt.Sprintf(`"M%d":1`, i))
	}
	doc := fmt.Sprintf(`{"_aws":{"Timestamp":%d,"CloudWatchMetrics":[{"Namespace":"N","Dimensions":[[]],"Metrics":[%s]}]},%s}`+"\n",
		now.UnixMilli(), strings.Join(defs, ","), strings.Join(vals, ","))
	if p := emfProblems([]byte(doc), now); len(p) == 0 {
		t.Error("101 metric definitions were accepted")
	}
}
