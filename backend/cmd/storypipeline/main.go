// Command storypipeline prints the Step Functions definition of the Story2Shorts
// video job with its placeholders filled in. deploy/setup-storyvideo.sh runs it,
// so the definition that is deployed is rendered by the same code the tests
// check, not by a second copy of the substitution written in shell.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rahul/indifferent/backend/internal/storyjob/pipeline"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("storypipeline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var p pipeline.Params
	var subnets string
	fs.StringVar(&p.EcsClusterArn, "cluster-arn", "", "ARN of the ECS cluster")
	fs.StringVar(&p.StoryRenderTaskDefArn, "task-definition-arn", "", "ARN of the story-render task definition (without a revision, so the latest is run)")
	fs.StringVar(&subnets, "subnets", "", "comma-separated subnet ids")
	fs.StringVar(&p.SecurityGroupId, "security-group", "", "security group id")
	fs.StringVar(&p.JobsTable, "table", "", "DynamoDB table holding the job records")
	fs.StringVar(&p.NotifierArn, "notifier-arn", "", "ARN of the notifier Lambda")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "storypipeline: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	for _, s := range strings.Split(subnets, ",") {
		if s = strings.TrimSpace(s); s != "" {
			p.SubnetIds = append(p.SubnetIds, s)
		}
	}
	def, err := pipeline.Render(p)
	if err != nil {
		fmt.Fprintf(stderr, "storypipeline: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, def)
	return 0
}
