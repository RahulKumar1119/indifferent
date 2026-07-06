# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree
COPY . .

# Build a statically-linked binary for the renderer Lambda
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/bootstrap ./cmd/renderer

# Stage 2: Download static FFmpeg binary
FROM alpine:3.21 AS ffmpeg-downloader

RUN apk add --no-cache curl tar xz \
    && mkdir -p /ffmpeg \
    && curl -L https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz \
       | tar xJ --strip-components=1 -C /ffmpeg

# Stage 3: Runtime image based on AWS Lambda base for AL2023
FROM public.ecr.aws/lambda/provided:al2023

# Copy static FFmpeg binary (includes libx264, libmp3lame, etc.)
COPY --from=ffmpeg-downloader /ffmpeg/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg-downloader /ffmpeg/ffprobe /usr/local/bin/ffprobe

# Copy the compiled Go binary as the Lambda bootstrap handler
COPY --from=builder /out/bootstrap ${LAMBDA_TASK_ROOT}/bootstrap

# Ensure binaries are executable
RUN chmod +x ${LAMBDA_TASK_ROOT}/bootstrap /usr/local/bin/ffmpeg /usr/local/bin/ffprobe

# Lambda container image entry point
ENTRYPOINT ["/var/task/bootstrap"]
