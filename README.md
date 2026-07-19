# TXT2Video

A SaaS platform that converts text-based quiz files (.txt) into narrated video content with answer reveals, ready for YouTube or any platform.

**Live:** https://indifferent.fun

## Overview

Upload a TXT file containing multiple-choice questions → choose a voice → get a professional MP4 video with narration, answer highlights, and smooth transitions — all automatically.

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
4. **Render** — Composites slides + audio into MP4 using FFmpeg (ultrafast preset)
5. **MarkCompleted** — Updates database with video/thumbnail keys
6. **MarkFailed** — Records error reason on any stage failure

## Architecture

- **Frontend**: Angular 20, Tailwind CSS 4, GSAP, Lucide Icons
- **Backend**: Go 1.24+ (Lambda functions)
- **Infrastructure**: AWS (API Gateway, Lambda, DynamoDB, S3, Step Functions, Amplify, Polly)

## Features

- Google OAuth authentication with JWT tokens
- Upload TXT quiz files with multiple-choice questions
- Support for multiple correct answers (e.g., "Select TWO")
- 5 AI voice options (Joanna, Matthew, Amy, Brian, Aditi) via Amazon Polly
- White-background slides with readable fonts
- Answer reveal with green highlighting + narration
- Real-time pipeline progress tracking
- Video preview and download
- Light/dark theme toggle
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
| Frontend | Angular 20 + Tailwind CSS 4 | SPA |
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
| CDN | Amplify Hosting | Frontend hosting |
| API | API Gateway | REST API with custom domain |
| DNS | Route 53 | Custom domain |
| SSL | ACM | HTTPS certificates |

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
├── deploy/                      # Deployment scripts
│   ├── setup.sh                 # One-time infra setup
│   ├── setup-apigateway.sh      # API Gateway config
│   ├── setup-custom-domain.sh   # Custom domain setup
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
go test ./...
```

### Deployment

See `deploy/.env.example` for required environment variables. All deploy scripts use environment variables for secrets — no hardcoded credentials.

```bash
# Set required secrets (see deploy/.env.example)
source deploy/.env

# One-time infrastructure setup
bash deploy/setup.sh

# Redeploy frontend
bash deploy/deploy-frontend.sh

# Redeploy a Lambda
bash deploy/deploy-lambda.sh <function-name>
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
- Codec: H.264
- Audio: AAC 128kbps
- FPS: 30
- Structure per question: Question slide (with narration) → Answer slide (with answer narration)
- Multiple correct answers highlighted in green with ✓

## License

MIT License — see [LICENSE](LICENSE)
