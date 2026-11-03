// Command storydashboard prints the body of the CloudWatch dashboard for the
// Story2Shorts success metrics (issue #194). deploy/setup-story-metrics.sh runs
// it, so the dashboard that is deployed is built by the same code the tests
// check against the metrics the services emit, not by a copy in shell.
//
// It prints the JSON body and nothing else on standard output.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/rahul/indifferent/backend/internal/storymetrics"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

var regionPattern = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("storydashboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	region := fs.String("region", "", "AWS region the metrics are in, for example ap-south-1")
	name := fs.Bool("name", false, "print the dashboard's name instead of its body")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "storydashboard: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if *name {
		fmt.Fprintln(stdout, storymetrics.DashboardName)
		return 0
	}
	if !regionPattern.MatchString(*region) {
		fmt.Fprintf(stderr, "storydashboard: -region must look like ap-south-1, got %q\n", *region)
		return 2
	}
	body, err := storymetrics.Dashboard(*region)
	if err != nil {
		fmt.Fprintf(stderr, "storydashboard: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(body))
	return 0
}
