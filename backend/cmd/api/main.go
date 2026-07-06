// Package main is the entry point for the API Lambda function.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/api"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/storage"
)

func main() {
	ctx := context.Background()

	// Load environment variables
	jwtSecret := os.Getenv("JWT_SECRET")
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirectURI := os.Getenv("GOOGLE_REDIRECT_URI")
	dynamoDBTable := os.Getenv("DYNAMODB_TABLE")
	s3Bucket := os.Getenv("S3_BUCKET")
	stateMachineARN := os.Getenv("STATE_MACHINE_ARN")
	usersTable := os.Getenv("USERS_TABLE")
	sessionTable := os.Getenv("SESSION_TABLE")

	// Validate required environment variables
	if jwtSecret == "" || googleClientID == "" || dynamoDBTable == "" || s3Bucket == "" || stateMachineARN == "" {
		log.Fatal("Missing required environment variables")
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Initialize AWS clients
	dbClient := dynamodb.NewFromConfig(awsCfg)
	sfnClient := sfn.NewFromConfig(awsCfg)

	// Initialize S3 client
	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}

	// Initialize auth services
	authConfig := auth.GoogleAuthConfig{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURI:  googleRedirectURI,
		JWTSecret:    jwtSecret,
		UsersTable:   usersTable,
		SessionTable: sessionTable,
	}
	authService := auth.NewGoogleAuthService(authConfig, dbClient, &http.Client{
		Timeout: 10 * time.Second,
	})
	jwtService := &auth.JWTService{
		Secret:       jwtSecret,
		DB:           dbClient,
		SessionTable: sessionTable,
		UsersTable:   usersTable,
	}

	// Create API handler
	handler := &api.APIHandler{
		AuthService:     authService,
		JWTService:      jwtService,
		DB:              dbClient,
		S3:              s3Client,
		SFN:             sfnClient,
		TableName:       dynamoDBTable,
		Bucket:          s3Bucket,
		StateMachineARN: stateMachineARN,
	}

	// Start Lambda
	lambda.Start(handler.HandleRequest)
}
