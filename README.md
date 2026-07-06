# TXT-to-Video SaaS

A SaaS platform that converts text-based quiz files (.txt) into narrated video content, ready for YouTube or other platforms.

## Overview

Upload a TXT file containing quiz questions, choose a template and voice, and the platform generates a fully rendered MP4 video with narration, countdown timers, and answer reveals.

## Architecture

- **Backend**: Go 1.24+ (Lambda functions on AWS)
- **Frontend**: Angular 20 with Tailwind CSS 4, GSAP, Motion One, Lucide Icons
- **Infrastructure**: AWS (API Gateway, Lambda, DynamoDB, S3, Step Functions, CloudFront)
- **CI/CD**: GitHub Actions with Terraform

## UI Design

The frontend features a premium dark-mode-first design inspired by modern SaaS products:

- **Dark mode by default** with light mode toggle
- **Aurora gradient background** — animated radial gradients with subtle motion
- **Glassmorphism** — frosted glass cards with `backdrop-blur` and semi-transparent borders
- **GSAP animations** — hero text stagger reveals, timeline-based entrances
- **Motion One** — micro-interactions on hover, focus, and page transitions
- **Lucide Icons** — clean, consistent SVG icon set
- **Shimmer text** — animated gradient text effect on headings
- **Spotlight hover** — radial gradient follows cursor on interactive cards
- **Moving borders** — animated conic gradient borders on selected elements
- **Glow buttons** — hover state with box-shadow glow effect
- **shadcn-style tokens** — CSS custom properties for colors, radii, spacing

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
- Premium glassmorphism UI with aurora gradients
- GSAP-powered hero animations
- Responsive mobile-first design

## Tech Stack

### Frontend
| Technology | Purpose |
|------------|---------|
| Angular 20 | Framework (standalone components) |
| Tailwind CSS 4 | Utility-first styling |
| GSAP | Premium animations (hero reveals, timelines) |
| Motion One | Micro-interactions (hover, entrance) |
| Lucide Icons | SVG icon library |
| Angular Material | Complex UI components (stepper, table, expansion) |

### Backend
| Technology | Purpose |
|------------|---------|
| Go 1.24 | Lambda function runtime |
| AWS SDK v2 | Cloud service integration |
| Playwright | HTML-to-PNG slide rendering |
| Amazon Polly | Neural voice narration |
| FFmpeg | Video composition |

### Infrastructure
| Technology | Purpose |
|------------|---------|
| Terraform | Infrastructure as code |
| API Gateway | REST API with JWT authorizer |
| Lambda | Serverless compute |
| DynamoDB | NoSQL database (users, projects, sessions) |
| S3 | Object storage (uploads, output, temp) |
| Step Functions | Pipeline orchestration |
| CloudFront | CDN for frontend |

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
│   │   ├── slidegen/        # Playwright slide renderer
│   │   └── storage/         # S3 client wrapper
│   └── templates/           # HTML/CSS slide templates
│       └── classic/         # Classic quiz theme
├── frontend/                # Angular 20 SPA
│   └── src/
│       ├── styles.css       # Global design system (aurora, glass, tokens)
│       └── app/
│           ├── auth/        # OAuth login + guards
│           ├── core/        # Services + HTTP interceptors
│           ├── pages/       # Page components
│           │   ├── landing/         # GSAP-animated hero + features
│           │   ├── dashboard/       # Glass stat cards + activity
│           │   ├── projects/        # Glass table + status badges
│           │   ├── create-project/  # Template/voice selectors
│           │   ├── progress/        # Animated pipeline stepper
│           │   ├── preview/         # Video player + download
│           │   ├── account/         # User profile
│           │   ├── settings/        # Theme toggle
│           │   ├── help/            # FAQ accordion
│           │   └── not-found/       # Shimmer 404
│           └── shared/      # Models + utilities
├── infrastructure/          # Terraform modules
│   ├── modules/
│   │   ├── api-gateway/     # REST API + CORS
│   │   ├── lambda/          # Lambda functions
│   │   ├── dynamodb/        # Database tables
│   │   ├── s3/              # Storage bucket
│   │   ├── step-functions/  # Pipeline orchestration
│   │   └── iam/             # IAM roles + policies
│   └── environments/
│       ├── staging/
│       └── production/
└── .github/workflows/       # CI/CD pipelines
    ├── backend.yml
    ├── frontend.yml
    ├── infrastructure.yml
    └── release.yml
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

The dev server starts at `http://localhost:4200` with hot reload. Dark mode is enabled by default.

**Configuration:** Edit `src/environments/environment.ts`:
```typescript
export const environment = {
  production: false,
  apiUrl: 'http://localhost:3000',
  googleClientId: '',
  googleRedirectUri: 'http://localhost:4200/auth/callback',
};
```

### Backend (Local Development)

```bash
cd backend
go mod download
go build ./...
go test ./...
```

Individual Lambda handlers can be tested locally with [AWS SAM](https://aws.amazon.com/serverless/sam/).

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

| Variable               | Used By | Description                 |
|------------------------|---------|------------------------------|
| S3_BUCKET              | All     | S3 bucket name for storage   |
| DYNAMODB_TABLE         | API     | DynamoDB table name          |
| JWT_SECRET             | API     | JWT signing secret           |
| GOOGLE_CLIENT_ID       | API     | Google OAuth client ID       |
| GOOGLE_CLIENT_SECRET   | API     | Google OAuth client secret   |
| TEMPLATE_DIR           | SlideGen| Path to HTML templates       |

## Deployment

Deployments are automated via GitHub Actions:
- **Staging**: Triggers on push to `main`
- **Production**: Triggers on release tag creation

## License

This project is licensed under the [MIT License](LICENSE).
