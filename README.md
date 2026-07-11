# TXT2Video — indifferent.fun

A SaaS platform that converts text-based quiz files (.txt) into narrated video content with answer reveals, ready for YouTube or any platform.

**Live:** https://indifferent.fun

## Overview

Upload a TXT file containing multiple-choice questions → choose a voice → get a professional MP4 video with narration, answer highlights, and smooth transitions — all automatically in under 2 minutes.

## Step Functions Pipeline

```
┌─────────┐     ┌────────────────┐     ┌──────────┐     ┌────────┐
│  Start  │────▶│   ParseTXT     │────▶│ Generate │────▶│Narrate │
└─────────┘     │   (Lambda)     │     │  Slides  │     │(Polly) │
                └────────────────┘     └──────────┘     └────────┘
                                                              │
                ┌────────────────┐     ┌──────────┐           │
                │ MarkCompleted  │◀────│  Render  │◀──────────┘
                │   (Lambda)     │     │ (FFmpeg) │
                └────────────────┘     └──────────┘
                        │
                        ▼                    ┌──────────────┐
                     [End]                   │  MarkFailed  │
                                             │  (on error)  │
                                             └──────────────┘
```

Each stage has retry policies (2 retries, exponential backoff). On failure at any stage, the pipeline catches the error and routes to `MarkFailed`.

**Pipeline stages:**
1. **ParseTXT** — Auto-detects format (numbered/bulleted/tabbed), extracts questions + correct answers
2. **GenerateSlides** — Renders question slides + answer reveal slides as 1920×1080 PNGs
3. **Narrate** — Generates question audio + answer audio using Amazon Polly (per question)
4. **Render** — Composites slides + audio into MP4 using FFmpeg (ultrafast preset, 10GB Lambda)
5. **MarkCompleted** — Updates DynamoDB with video/thumbnail S3 keys
6. **MarkFailed** — Records error reason on any stage failure

## Architecture

- **Frontend**: Angular 20, Tailwind CSS 4, GSAP, Lucide Icons
- **Backend**: Go 1.24+ (Lambda functions on AWS)
- **Infrastructure**: AWS (API Gateway, Lambda, DynamoDB, S3, Step Functions, CloudFront, Polly)
- **Region**: ap-south-1 (Mumbai)
- **Domain**: indifferent.fun (frontend) / api.indifferent.fun (API)

## Features

- Google OAuth authentication with JWT tokens
- Upload TXT quiz files with multiple-choice questions
- Support for multiple correct answers (e.g., "Select TWO")
- 5 AI voice options (Joanna, Matthew, Amy, Brian, Aditi) via Amazon Polly
- White-background slides with purple accent and readable fonts
- Answer reveal with green highlighting + narration ("The correct answer is...")
- Real-time pipeline progress tracking
- Video preview and download
- Dark/light theme toggle
- PWA support

## Supported TXT Formats

### Numbered (with correct answer marked by `*`)
```
1. What is the capital of France?
A) London
B) Paris *
C) Berlin
D) Madrid

2. Select TWO correct answers.
A) Wrong
B) Correct one *
C) Wrong
D) Wrong
E) Correct two *
```

### Bulleted
```
What is the largest planet?
- Jupiter *
- Saturn
- Earth
```

### Tabbed
```
What is 2+2?
	A) 3
	B) 4 *
	C) 5
```

Mark correct answers with `*` suffix. Multiple `*` markers are supported for multi-select questions.

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Frontend | Angular 20 + Tailwind CSS 4 | SPA with glassmorphism UI |
| Animations | GSAP + Motion One | Hero reveals, micro-interactions |
| Icons | Lucide Icons | SVG icon library |
| Backend | Go 1.24 | Lambda functions |
| Auth | Google OAuth + JWT | Authentication |
| Database | DynamoDB | Users, projects, sessions |
| Storage | S3 | Uploads, slides, audio, videos |
| Narration | Amazon Polly (Standard) | 5 English voices |
| Video | FFmpeg (static binary) | MP4 composition |
| Slides | Go native image rendering | 1920×1080 PNG generation |
| Orchestration | Step Functions | Pipeline state machine |
| CDN | CloudFront | Frontend hosting |
| API | API Gateway (Regional) | REST API with custom domain |
| DNS | Route 53 | indifferent.fun |
| SSL | ACM | HTTPS certificates |

## AWS Resources (ap-south-1)

| Resource | Name |
|----------|------|
| DynamoDB Tables | `indifferent-fun-users`, `indifferent-fun-projects`, `indifferent-fun-sessions` |
| S3 Buckets | `indifferent-fun-assets` (pipeline), `indifferent-fun-frontend` (static site) |
| Lambda Functions | `indifferent-fun-api`, `-parser`, `-slidegen`, `-narrator`, `-renderer`, `-statusupdater` |
| Step Functions | `indifferent-fun-pipeline` |
| API Gateway | `indifferent-fun-api` → `api.indifferent.fun` |
| CloudFront | `E8T1WZPS2A2JW` → `indifferent.fun` |
| ECR | `indifferent-fun-renderer` (container image with FFmpeg) |

## Project Structure

```
├── backend/                     # Go Lambda functions
│   ├── cmd/
│   │   ├── api/                 # REST API (all endpoints)
│   │   ├── parser/              # TXT parsing
│   │   ├── slidegen/            # PNG slide generation
│   │   ├── narrator/            # Polly audio generation
│   │   ├── renderer/            # FFmpeg video composition (container)
│   │   └── statusupdater/       # DynamoDB status updates
│   ├── internal/
│   │   ├── api/                 # API handler + routing
│   │   ├── auth/                # Google OAuth + JWT
│   │   ├── models/              # Data structs
│   │   ├── narrator/            # Polly service + handler
│   │   ├── parser/              # Format detection + extraction
│   │   ├── pipeline/            # Step Functions status
│   │   ├── renderer/            # FFmpeg compositor + handler
│   │   ├── slidegen/            # Native image renderer + handler
│   │   └── storage/             # S3 client
│   └── templates/classic/       # HTML slide templates (legacy)
├── frontend/                    # Angular 20 SPA
│   └── src/app/
│       ├── auth/                # OAuth login + guards
│       ├── core/                # Services + interceptors
│       ├── pages/               # All page components
│       └── shared/              # Models
├── deploy/                      # AWS CLI deployment scripts
│   ├── setup.sh                 # One-time infra setup
│   ├── setup-apigateway.sh      # API Gateway config
│   ├── setup-custom-domain.sh   # api.indifferent.fun
│   └── setup-cloudfront.sh      # CloudFront + S3 hosting
└── .github/workflows/           # CI/CD
    ├── backend.yml
    ├── frontend.yml
    └── release.yml
```

## Getting Started

### Prerequisites

- Go 1.24+
- Node.js 22+
- Docker (for renderer container)
- AWS CLI configured

### Local Development

```bash
# Frontend
cd frontend
npm install
npm start  # http://localhost:4200

# Backend
cd backend
go mod download
go build ./...
go test ./...  # 128+ tests
```

### Deploy Scripts

```bash
# Set secrets
export GOOGLE_CLIENT_ID='your-id.apps.googleusercontent.com'
export GOOGLE_CLIENT_SECRET='your-secret'
export JWT_SECRET=$(openssl rand -hex 32)

# One-time infrastructure setup
bash deploy/setup.sh

# Redeploy a Lambda
cd backend
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap ./cmd/api
cd /tmp && zip -j api.zip bootstrap
aws lambda update-function-code --function-name indifferent-fun-api --zip-file fileb:///tmp/api.zip --region ap-south-1

# Redeploy frontend
cd frontend
npx ng build --configuration=production
aws s3 sync dist/frontend/browser/ s3://indifferent-fun-frontend/ --delete --cache-control "public, max-age=31536000, immutable" --exclude "index.html" --region ap-south-1
aws s3 cp dist/frontend/browser/index.html s3://indifferent-fun-frontend/index.html --cache-control "no-cache, no-store, must-revalidate" --region ap-south-1
aws cloudfront create-invalidation --distribution-id E8T1WZPS2A2JW --paths "/*"

# Rebuild renderer container
docker build -t indifferent-fun-renderer:latest -f cmd/renderer/Dockerfile .
aws ecr get-login-password --region ap-south-1 | docker login --username AWS --password-stdin 438097524343.dkr.ecr.ap-south-1.amazonaws.com
docker tag indifferent-fun-renderer:latest 438097524343.dkr.ecr.ap-south-1.amazonaws.com/indifferent-fun-renderer:latest
docker push 438097524343.dkr.ecr.ap-south-1.amazonaws.com/indifferent-fun-renderer:latest
aws lambda update-function-code --function-name indifferent-fun-renderer --image-uri 438097524343.dkr.ecr.ap-south-1.amazonaws.com/indifferent-fun-renderer:latest --region ap-south-1
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /auth/google/callback | No | Exchange OAuth code for tokens |
| POST | /auth/refresh | No | Refresh access token |
| POST | /auth/logout | No | Invalidate session |
| GET | /projects | Yes | List user's projects |
| POST | /projects | Yes | Create new project |
| GET | /projects/:id | Yes | Get project details |
| DELETE | /projects/:id | Yes | Delete project |
| POST | /projects/:id/upload | Yes | Get signed upload URL |
| POST | /projects/:id/start | Yes | Start processing pipeline |
| GET | /projects/:id/status | Yes | Get pipeline progress |
| GET | /projects/:id/download | Yes | Get signed download URL |

## Video Output

- Resolution: 1920×1080
- Codec: H.264 (libx264, ultrafast preset)
- Audio: AAC 128kbps
- FPS: 30
- Structure per question: Question slide (with narration) → Answer slide (with answer narration)
- Multiple correct answers highlighted in green with ✓

## Environment Variables (Lambda)

| Variable | Lambda | Description |
|----------|--------|-------------|
| S3_BUCKET | All | `indifferent-fun-assets` |
| DYNAMODB_TABLE | API, StatusUpdater | `indifferent-fun-projects` |
| USERS_TABLE | API | `indifferent-fun-users` |
| SESSION_TABLE | API | `indifferent-fun-sessions` |
| JWT_SECRET | API | JWT signing key |
| GOOGLE_CLIENT_ID | API | OAuth client ID |
| GOOGLE_CLIENT_SECRET | API | OAuth client secret |
| GOOGLE_REDIRECT_URI | API | `https://indifferent.fun/auth/callback` |
| STATE_MACHINE_ARN | API | Step Functions ARN |

## License

MIT License — see [LICENSE](LICENSE)
