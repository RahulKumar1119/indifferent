// Package main provides the Lambda entry point for the job notifier.
// Step Functions invokes it as the last state of the quiz-video and AI Shorts
// pipelines to email the user that a job finished or failed.
package main

import (
	"context"
	"log"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/rahul/indifferent/backend/internal/notifier"
)

const (
	defaultFrom    = "Indifferent <notifications@indifferent.fun>"
	defaultReplyTo = "support@indifferent.fun"
	defaultBaseURL = "https://indifferent.fun"
)

func main() {
	ctx := context.Background()

	projectsTable := os.Getenv("DYNAMODB_TABLE")
	usersTable := os.Getenv("USERS_TABLE")
	configSet := os.Getenv("SES_CONFIG_SET")
	if projectsTable == "" || usersTable == "" || configSet == "" {
		log.Fatal("DYNAMODB_TABLE, USERS_TABLE and SES_CONFIG_SET environment variables are required")
	}

	baseURL := envOr("APP_BASE_URL", defaultBaseURL)
	if u, err := url.Parse(baseURL); err != nil || u.Scheme != "https" || u.Host == "" {
		log.Fatalf("APP_BASE_URL must be an https URL, got %q", baseURL)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	n := &notifier.Notifier{
		DB: dynamodb.NewFromConfig(cfg),
		Mailer: &notifier.SESMailer{
			Client:    sesv2.NewFromConfig(cfg),
			From:      envOr("FROM_ADDRESS", defaultFrom),
			ReplyTo:   envOr("REPLY_TO", defaultReplyTo),
			ConfigSet: configSet,
		},
		ProjectsTable: projectsTable,
		UsersTable:    usersTable,
		BaseURL:       baseURL,
	}

	lambda.Start(n.Handle)
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
