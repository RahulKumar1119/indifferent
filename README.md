# TXT-to-Video SaaS

A SaaS platform that converts text-based quiz files (.txt) into narrated video content, ready for YouTube or other platforms.

## Overview

Upload a TXT file containing quiz questions, choose a template and voice, and the platform generates a fully rendered MP4 video with narration, countdown timers, and answer reveals.

## Architecture

- **Backend**: Go 1.24+ (Lambda functions on AWS)
- **Frontend**: Angular 20 with Tailwind CSS 4 and Angular Material
- **Infrastructure**: AWS (API Gateway, Lambda, DynamoDB, S3, Step Functions, CloudFront)
- **CI/CD**: GitHub Actions with Terraform

## Pipeline

1. **Parser** — Auto-detects TXT format (numbered, bulleted, tabbed) and extracts structured question data
2. **Slide Generator** — Renders HTML/CSS templates to PNG slides using Playwright
3. **Narrator** — Generates MP3 audio per question using Amazon Polly (5 neural voice options)
4. **Renderer** — Composes final MP4 video with FFmpeg (transitions, countdown, answer reveal, outro)

## Features

- Google OAuth authentication with JWT tokens
- Project management (create, list, delete)
- Real-time pipeline progress tracking with adaptive polling
- Video preview and signed-URL download (24-hour expiration)
- Dark/light theme with localStorage persistence
- PWA support with service worker
- Classic video template (more planned)
- File validation (TXT only, max 5MB, content sanitization)

## Project Structure

```
├── backend/                 # Go Lambda functions and shared packages
│   ├── cmd/                 # Lambda entry points
│   │   ├── api/             # REST API handler
│   │   ├── parser/          # TXT parsing Lambda
│   │   ├── slidegen/        # Slide generation Lambda
│   │   ├── narrator/        # Audio narration Lambda
│   │   └── renderer/        # Video rendering Lambda (container)
│   ├── internal/            # Shared internal packages
│   │   ├── auth/            # Google OAuth + JWT
│   │   ├── config/          # Configuration
│   │   ├── models/          # Data models
│   │   ├── narrator/        # Polly narration service
│   │   ├── parser/          # Format detection + extraction
│   │   └── storage/         # S3 client wrapper
│   └── templates/           # HTML/CSS slide templates
│       └── classic/         # Classic quiz theme
├── frontend/                # Angular 20 SPA
│   └── src/app/
│       ├── auth/            # OAuth login + guards
│       ├── core/            # Services + HTTP interceptors
│       ├── pages/           # Page components
│       │   ├── landing/     # Landing page
│       │   ├── dashboard/   # Project dashboard
│       │   ├── projects/    # Project list
│       │   ├── create-project/  # Create + upload flow
│       │   ├── progress/    # Rendering progress
│       │   ├── preview/     # Video preview + download
│       │   ├── account/     # User profile
│       │   ├── settings/    # Theme + preferences
│       │   └── help/        # FAQ + usage guide
│       └── shared/          # Models + utilities
├── infrastructure/          # Terraform modules
│   ├── modules/             # Reusable infra modules
│   │   ├── api-gateway/     # REST API definition
│   │   ├── lambda/          # Lambda functions
│   │   ├── dynamodb/        # Database tables
│   │   ├── s3/              # Storage bucket
│   │   ├── step-functions/  # Pipeline orchestration
│   │   └── iam/             # IAM roles + policies
│   └── environments/        # Per-environment configs
│       ├── staging/
│       └── production/
└── .github/workflows/       # CI/CD pipelines
    ├── backend.yml          # Go build + test + deploy
    ├── frontend.yml         # Angular build + test + deploy
    ├── infrastructure.yml   # Terraform plan/apply
    └── release.yml          # Production deployment
```

## Getting Started

### Prerequisites

- **Go** 1.24+
- **Node.js** 22+
- **npm** (comes with Node.js)
- **Terraform** 1.9+ (for infrastructure)
- **AWS CLI** configured with credentials (for deployment)

### Frontend (Local Development)

```bash
cd frontend
npm install
npm start
```

The dev server starts at `http://localhost:4200` with hot reload.

**Configuration:** Edit `src/environments/environment.ts`:
```typescript
export const environment = {
  production: false,
  apiUrl: 'http://localhost:3000',  // Backend API URL
  googleClientId: '',               // Your Google OAuth client ID
};
```

### Backend (Local Development)

```bash
cd backend
go mod download
go build ./...
go test ./...
```

Individual Lambda handlers can be tested locally with tools like [AWS SAM](https://aws.amazon.com/serverless/sam/) or invoked directly.

### Infrastructure

```bash
cd infrastructure/environments/staging
terraform init
terraform plan
terraform apply
```

## Supported TXT Formats

### Numbered
```
1. What is the capital of France?
A) London
B) Paris *
C) Berlin
D) Madrid
```

### Bulleted
```
What is the largest planet?
- Jupiter *
- Saturn
- Earth
- Mars
```

### Tabbed
```
What is the speed of light?
	A) 300,000 km/s *
	B) 150,000 km/s
	C) 1,000 km/s
```

Mark the correct answer with `*` suffix or `(correct)` annotation.

## Available Voices

| Voice     | Description          |
|-----------|---------------------|
| Joanna    | US English, Female   |
| Matthew   | US English, Male     |
| Ruth      | US English, Female   |
| Danielle  | US English, Female   |
| Aditi     | Indian English, Female |

## API Endpoints

| Method | Path                          | Description                    |
|--------|-------------------------------|--------------------------------|
| POST   | /auth/google/callback         | Exchange OAuth code for tokens |
| POST   | /auth/refresh                 | Refresh access token           |
| POST   | /auth/logout                  | Invalidate session             |
| GET    | /projects                     | List user's projects           |
| POST   | /projects                     | Create new project             |
| GET    | /projects/:id                 | Get project details            |
| DELETE | /projects/:id                 | Delete project                 |
| POST   | /projects/:id/upload          | Get signed upload URL          |
| GET    | /projects/:id/status          | Get pipeline progress          |
| GET    | /projects/:id/download        | Get signed download URL        |

## Scripts

### Frontend
```bash
npm start       # Dev server (port 4200)
npm run build   # Production build
npm test        # Run unit tests
```

### Backend
```bash
go test ./...                    # Run all tests
go build ./cmd/api/              # Build API Lambda
go build ./cmd/parser/           # Build Parser Lambda
go build ./cmd/slidegen/         # Build Slide Generator
go build ./cmd/narrator/         # Build Narrator
go build ./cmd/renderer/         # Build Renderer
```

## Environment Variables (Lambda)

| Variable          | Used By    | Description                          |
|-------------------|-----------|--------------------------------------|
| S3_BUCKET         | All       | S3 bucket name for storage           |
| DYNAMODB_TABLE    | API       | DynamoDB table name                  |
| JWT_SECRET        | API       | JWT signing secret                   |
| GOOGLE_CLIENT_ID  | API       | Google OAuth client ID               |
| GOOGLE_CLIENT_SECRET | API    | Google OAuth client secret           |

## Deployment

Deployments are automated via GitHub Actions:
- **Staging**: Triggers on push to `main`
- **Production**: Triggers on release tag creation

## License

This project is licensed under the [MIT License](LICENSE).
