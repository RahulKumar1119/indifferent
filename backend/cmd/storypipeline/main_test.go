package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

var good = []string{
	"-cluster-arn", "arn:aws:ecs:ap-south-1:123456789012:cluster/c",
	"-task-definition-arn", "arn:aws:ecs:ap-south-1:123456789012:task-definition/t",
	"-subnets", "subnet-aaa, subnet-bbb ,subnet-ccc",
	"-security-group", "sg-0123",
	"-table", "tbl",
	"-notifier-arn", "arn:aws:lambda:ap-south-1:123456789012:function:n",
}

func TestRun_PrintsAFilledInDefinition(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(good, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if strings.Contains(out.String(), "${") {
		t.Error("a placeholder is left")
	}
	var d struct {
		States map[string]struct {
			Parameters struct {
				NetworkConfiguration struct {
					AwsvpcConfiguration struct{ Subnets []string }
				}
			}
		}
	}
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	got := d.States["RunOnSpot"].Parameters.NetworkConfiguration.AwsvpcConfiguration.Subnets
	if len(got) != 3 || got[1] != "subnet-bbb" {
		t.Errorf("the comma list, with spaces, came out as %v", got)
	}
}

func TestRun_RefusesWhatItCannotRenderAndPrintsNothing(t *testing.T) {
	cases := map[string][]string{
		"nothing":         {},
		"a stray word":    append(append([]string{}, good...), "extra"),
		"an unknown flag": {"-nope"},
		"no subnets":      {"-cluster-arn", "a", "-task-definition-arn", "b", "-security-group", "c", "-table", "d", "-notifier-arn", "e"},
	}
	for name, args := range cases {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code == 0 {
			t.Errorf("%s: exit 0", name)
		}
		if out.Len() != 0 {
			t.Errorf("%s: printed a definition: the deploy would use it", name)
		}
	}
}
