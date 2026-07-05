# TXT-to-Video SaaS

A SaaS platform that converts text-based quiz files (.txt) into narrated video content, ready for YouTube or other platforms.

## Overview

Upload a TXT file containing quiz questions, choose a template and voice, and the platform generates a fully rendered MP4 video with narration, countdown timers, and answer reveals.

## Architecture

- **Backend**: Go (Lambda functions on AWS)
- **Frontend**: Angular 20 with Tailwind CSS 4
- **Infrastructure**: AWS (API Gateway, Lambda, DynamoDB, S3, Step Functions)
- **CI/CD**: GitHub Actions with Terraform

## Pipeline

1. **Parser** — Auto-detects TXT format (numbered, bulleted, tabbed) and extracts structured question data
2. **Slide Generator** — Renders HTML/CSS templates to PNG slides using Playwright
3. **Narrator** — Generates MP3 audio per question using Amazon Polly (5 voice options)
4. **Renderer** — Composes final MP4 video with FFmpeg (transitions, countdown, answer reveal, outro)

## Features

- Google OAuth authentication
- Project management (create, list, delete)
- Real-time pipeline progress tracking
- Video preview and signed-URL download
- Dark/light theme support
- Classic video template (more planned)

## Project Structure

```
backend/          # Go Lambda functions and shared packages
frontend/         # Angular 20 SPA
infrastructure/   # Terraform modules
.github/          # CI/CD workflows
```

## Getting Started

_Coming soon — project is under active development._

## License

This project is licensed under the [MIT License](LICENSE).
