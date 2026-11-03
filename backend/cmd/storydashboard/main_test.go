package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/storymetrics"
)

func runCmd(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestPrintsTheDashboardBodyAndNothingElse(t *testing.T) {
	code, out, errOut := runCmd("-region", "ap-south-1")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	// The script uploads standard output as it is: any extra byte would corrupt the body.
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("standard output is not exactly one JSON document: %v\n%s", err, out)
	}
	want, _ := storymetrics.Dashboard("ap-south-1")
	if strings.TrimSpace(out) != string(want) {
		t.Error("the command prints something other than storymetrics.Dashboard")
	}
}

func TestPrintsTheDashboardName(t *testing.T) {
	code, out, _ := runCmd("-name")
	if code != 0 || strings.TrimSpace(out) != storymetrics.DashboardName {
		t.Errorf("exit %d, output %q", code, out)
	}
}

func TestRefusesABadRegionOrArgumentWithNothingOnStandardOutput(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"-region", ""},
		{"-region", "mars"},
		{"-region", "ap-south-1; rm -rf /"},
		{"-region", "ap-south-1", "extra"},
		{"-nonsense"},
	} {
		code, out, errOut := runCmd(args...)
		if code == 0 || out != "" || errOut == "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want a refusal that prints nothing to stdout", args, code, out, errOut)
		}
	}
}

func TestAcceptsTheRegionsAnAccountCanHave(t *testing.T) {
	for _, r := range []string{"ap-south-1", "us-east-1", "eu-west-2", "ap-southeast-2", "us-gov-west-1", "me-central-1"} {
		if code, _, errOut := runCmd("-region", r); code != 0 {
			t.Errorf("%s refused: %s", r, errOut)
		}
	}
}
