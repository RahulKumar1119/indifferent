// Package pipeline holds the Step Functions definition of the Story2Shorts video
// job and the code to fill in its placeholders, so the same code is used by the
// tests and by anything that deploys it.
package pipeline

import (
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed statemachine.json
var template string

// Template is the definition with its ${Placeholder} markers still in it.
func Template() string { return template }

// Params are the values the template needs.
type Params struct {
	EcsClusterArn         string
	StoryRenderTaskDefArn string
	SubnetIds             []string
	SecurityGroupId       string
	JobsTable             string
	NotifierArn           string
}

var placeholder = regexp.MustCompile(`\$\{[A-Za-z]+\}`)

// Placeholders lists the markers in the template, sorted and without repeats.
func Placeholders() []string {
	seen := map[string]bool{}
	for _, m := range placeholder.FindAllString(template, -1) {
		seen[m] = true
	}
	var out []string
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// Render fills in the template. It refuses an empty value, and a value that
// would break the JSON around it, and it checks that no marker is left.
func Render(p Params) (string, error) {
	if len(p.SubnetIds) == 0 {
		return "", fmt.Errorf("no subnets")
	}
	for _, s := range p.SubnetIds {
		if !safeID.MatchString(s) {
			return "", fmt.Errorf("subnet %q is not a subnet id", s)
		}
	}
	subnets := `"` + strings.Join(p.SubnetIds, `","`) + `"`
	vals := map[string]string{
		"${EcsClusterArn}":         p.EcsClusterArn,
		"${StoryRenderTaskDefArn}": p.StoryRenderTaskDefArn,
		"${SecurityGroupId}":       p.SecurityGroupId,
		"${JobsTable}":             p.JobsTable,
		"${NotifierArn}":           p.NotifierArn,
	}
	out := template
	// The template holds the subnet list as the single-element array ["${SubnetIds}"].
	if !strings.Contains(out, `["${SubnetIds}"]`) {
		return "", fmt.Errorf("the template has no subnet list")
	}
	out = strings.ReplaceAll(out, `["${SubnetIds}"]`, "["+subnets+"]")
	for k, v := range vals {
		if v == "" {
			return "", fmt.Errorf("%s is empty", k)
		}
		if strings.ContainsAny(v, "\"\\\n") {
			return "", fmt.Errorf("%s contains a character that would break the JSON", k)
		}
		out = strings.ReplaceAll(out, k, v)
	}
	if left := placeholder.FindAllString(out, -1); len(left) > 0 {
		return "", fmt.Errorf("placeholders left unfilled: %v", left)
	}
	return out, nil
}

var safeID = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
