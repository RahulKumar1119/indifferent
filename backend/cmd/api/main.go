// Package main is the entry point for the API Lambda function.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/api"
	"github.com/rahul/indifferent/backend/internal/auth"
	appconfig "github.com/rahul/indifferent/backend/internal/config"
	"github.com/rahul/indifferent/backend/internal/shorts/rank"
	"github.com/rahul/indifferent/backend/internal/slidegen"
	"github.com/rahul/indifferent/backend/internal/stockcache"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/storyplan"
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
	shortsStateMachineARN := os.Getenv("SHORTS_STATE_MACHINE_ARN")
	// Optional: without it the thumbnail endpoints answer 503.
	thumbnailStateMachineARN := os.Getenv("THUMBNAIL_STATE_MACHINE_ARN")
	// Optional: without it POST /story-videos answers 503 and the other story
	// video routes keep working (issue #193).
	storyVideoStateMachineARN := os.Getenv("STORYVIDEO_STATE_MACHINE_ARN")
	usersTable := os.Getenv("USERS_TABLE")
	sessionTable := os.Getenv("SESSION_TABLE")

	// Validate required environment variables
	if jwtSecret == "" || googleClientID == "" || dynamoDBTable == "" || s3Bucket == "" || stateMachineARN == "" {
		log.Fatal("Missing required environment variables")
	}
	if googleClientSecret == "" || googleRedirectURI == "" {
		log.Fatal("Missing required environment variables: GOOGLE_CLIENT_SECRET and GOOGLE_REDIRECT_URI must both be set or Google OAuth fails at runtime")
	}
	if usersTable == "" || sessionTable == "" || shortsStateMachineARN == "" {
		log.Fatal("Missing required environment variables: USERS_TABLE, SESSION_TABLE and SHORTS_STATE_MACHINE_ARN must all be set")
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

	// Story planning (issue #191). Off unless STORY_PLAN_ENABLED is true or 1:
	// the endpoints then answer 503 and nothing calls Bedrock, so this binary
	// can be deployed before the feature is switched on, and switched off again
	// by clearing the variable. PLANNING_MODEL takes the same values as
	// RANKING_MODEL (kimi, nova, claude); it defaults to kimi, the model whose
	// latency and Hindi output were measured.
	var storyPlanner api.StoryPlanner
	if appconfig.Enabled(os.Getenv("STORY_PLAN_ENABLED")) {
		model := rank.ModelFromString(os.Getenv("PLANNING_MODEL"))
		storyPlanner = storyplan.NewService(rank.NewService(bedrockruntime.NewFromConfig(awsCfg), model), string(model))
	}

	// Stock media search (issue #192). Built from PEXELS_API_KEY and
	// PIXABAY_API_KEY: either alone is enough, and with neither set the
	// /stock-media routes answer 503 instead of failing at startup, so the API
	// deploys before the keys exist. The variable stays a true nil interface in
	// that case, which the handlers check.
	//
	// STOCK_CACHE_TABLE, when set, adds a cache shared by every container (see
	// package stockcache); unset, each container keeps its own.
	var stockMedia api.StockMedia
	if svc, err := stockmedia.NewServiceFromEnv(stockcache.ServiceOptions(dbClient, os.Getenv, log.Printf)...); err == nil {
		stockMedia = svc
		log.Printf("stock media enabled providers=%v", svc.Providers())
	} else if !errors.Is(err, stockmedia.ErrNotConfigured) {
		log.Fatalf("stock media: %v", err)
	}

	// Create API handler
	handler := &api.APIHandler{
		AuthService:           authService,
		JWTService:            jwtService,
		PasswordAuth:          auth.NewPasswordAuthService(dbClient, jwtService, usersTable, sessionTable),
		DB:                    dbClient,
		S3:                    s3Client,
		SFN:                   sfnClient,
		TableName:             dynamoDBTable,
		UsersTable:            usersTable,
		Bucket:                s3Bucket,
		StateMachineARN:       stateMachineARN,
		ShortsStateMachineARN: shortsStateMachineARN,

		ThumbnailStateMachineARN:  thumbnailStateMachineARN,
		StoryVideoStateMachineARN: storyVideoStateMachineARN,
		// Optional, off unless S3_ACCELERATE is true or 1.
		S3Accelerate:    appconfig.S3Accelerate(),
		PreviewRenderer: slidegen.NewNativeRenderer(),
		StoryPlanner:    storyPlanner,
		StockMedia:      stockMedia,
	}

	// Start Lambda
	lambda.Start(handler.HandleRequest)
}
