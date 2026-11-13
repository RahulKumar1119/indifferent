// Package main is the entry point for the API Lambda function.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/api"
	"github.com/rahul/indifferent/backend/internal/auth"
	appconfig "github.com/rahul/indifferent/backend/internal/config"
	"github.com/rahul/indifferent/backend/internal/notifier"
	"github.com/rahul/indifferent/backend/internal/shorts/rank"
	"github.com/rahul/indifferent/backend/internal/slidegen"
	"github.com/rahul/indifferent/backend/internal/stockcache"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/storymetrics"
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

	// Owner alerts (#297): with ADMIN_ALERT_EMAIL set, every new account sends
	// one email there, numbered by a counter in the users table. Unset sends
	// nothing. Best effort: it can add at most a few seconds to a first sign-in
	// and never fails it.
	var signups *auth.SignupAlerter
	if to := strings.TrimSpace(os.Getenv("ADMIN_ALERT_EMAIL")); to != "" {
		signups = &auth.SignupAlerter{
			DB:         dbClient,
			UsersTable: usersTable,
			Notifier: notifier.SignupMailer{To: to, Mailer: &notifier.SESMailer{
				Client:    sesv2.NewFromConfig(awsCfg),
				From:      envOr("FROM_ADDRESS", "Indifferent <notifications@indifferent.fun>"),
				ConfigSet: envOr("SES_CONFIG_SET", "indifferent-notifications"),
			}},
		}
		log.Printf("signup alerts on")
	}

	// Initialize auth services
	authConfig := auth.GoogleAuthConfig{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURI:  googleRedirectURI,
		JWTSecret:    jwtSecret,
		UsersTable:   usersTable,
		SessionTable: sessionTable,
		Signups:      signups,
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
	// by clearing the variable.
	//
	// PLANNING_MODEL picks the model (qwen, deepseek, kimi, nova, claude) and
	// defaults to qwen. PLANNING_FALLBACK_MODEL picks the one asked when the
	// first times out, fails or answers with an unusable plan; it defaults to
	// deepseek, and "none" turns the fallback off. Both defaults were measured
	// over 21 plans each, English and Hindi (#293).
	var storyPlanner api.StoryPlanner
	if appconfig.Enabled(os.Getenv("STORY_PLAN_ENABLED")) {
		bedrock := bedrockruntime.NewFromConfig(awsCfg)
		model := planningModel(os.Getenv("PLANNING_MODEL"), rank.ModelQwen3)
		var opts []storyplan.Option
		if fb := os.Getenv("PLANNING_FALLBACK_MODEL"); !strings.EqualFold(strings.TrimSpace(fb), "none") {
			if fallback := planningModel(fb, rank.ModelDeepSeek); fallback != model {
				opts = append(opts, storyplan.WithFallback(rank.NewService(bedrock, fallback), string(fallback)))
				log.Printf("story planning model=%s fallback=%s", model, fallback)
			}
		}
		storyPlanner = storyplan.NewService(rank.NewService(bedrock, model), string(model), opts...)
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
	// Auto-pick (#302): a Pixabay picture for each scene of a new plan. Off
	// unless STORY_AUTOPICK_ENABLED is true or 1. Turn it on only once Pixabay
	// has confirmed its API terms allow searches made from a plan.
	autoPick := appconfig.Enabled(os.Getenv("STORY_AUTOPICK_ENABLED"))
	if autoPick && stockMedia != nil {
		log.Printf("story autopick on provider=pixabay")
	}

	// Email confirmation for password sign-ups (SEC-08). Off unless
	// EMAIL_VERIFICATION_ENABLED is true or 1: nothing is mailed, new accounts are
	// not held back, and old ones are not either. Clear the variable to switch it
	// off again. Turn it on only after the API role may send mail
	// (deploy/setup-email-verification.sh); until then every confirmation email
	// would fail and new accounts would wait for a link that never arrives.
	var verifier *auth.EmailVerifier
	if v := os.Getenv("EMAIL_VERIFICATION_ENABLED"); v == "true" || v == "1" {
		verifier = &auth.EmailVerifier{
			DB:         dbClient,
			UsersTable: usersTable,
			TokenTable: sessionTable,
			BaseURL:    envOr("APP_BASE_URL", "https://indifferent.fun"),
			Mailer: notifier.VerifyMailer{Mailer: &notifier.SESMailer{
				Client:    sesv2.NewFromConfig(awsCfg),
				From:      envOr("FROM_ADDRESS", "Indifferent <notifications@indifferent.fun>"),
				ReplyTo:   envOr("REPLY_TO", "support@indifferent.fun"),
				ConfigSet: envOr("SES_CONFIG_SET", "indifferent-notifications"),
			}},
		}
	}
	passwordAuth := auth.NewPasswordAuthService(dbClient, jwtService, usersTable, sessionTable)
	passwordAuth.Verifier = verifier
	passwordAuth.Signups = signups

	// Create API handler
	handler := &api.APIHandler{
		AuthService:           authService,
		JWTService:            jwtService,
		PasswordAuth:          passwordAuth,
		Verifier:              verifier,
		DB:                    dbClient,
		S3:                    s3Client,
		SFN:                   sfnClient,
		TableName:             dynamoDBTable,
		QuotaTable:            sessionTable,
		UsersTable:            usersTable,
		Bucket:                s3Bucket,
		StateMachineARN:       stateMachineARN,
		ShortsStateMachineARN: shortsStateMachineARN,

		ThumbnailStateMachineARN:  thumbnailStateMachineARN,
		StoryVideoStateMachineARN: storyVideoStateMachineARN,
		// The Story2Shorts success metrics (issue #194), as CloudWatch embedded
		// metric lines on stdout. Not through log, whose timestamp prefix would
		// make the line invalid.
		Metrics: storymetrics.New(os.Stdout),
		// Optional, off unless S3_ACCELERATE is true or 1.
		S3Accelerate:    appconfig.S3Accelerate(),
		PreviewRenderer: slidegen.NewNativeRenderer(),
		StoryPlanner:    storyPlanner,
		StockMedia:      stockMedia,
		AutoPick:        autoPick,
	}

	// Start Lambda
	lambda.Start(handler.HandleRequest)
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// planningModel maps a PLANNING_MODEL or PLANNING_FALLBACK_MODEL value to a
// model. Empty means def. An unknown name also means def, and is logged:
// rank.ModelFromString would turn it into Kimi, which is neither default.
func planningModel(v string, def rank.ModelID) rank.ModelID {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return def
	case "qwen", "deepseek", "kimi", "nova", "claude":
		return rank.ModelFromString(v)
	}
	log.Printf("story planning: unknown model %q, using %s", v, def)
	return def
}
