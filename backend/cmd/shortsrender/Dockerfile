# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder

# go.mod may require a newer patch release than the base image ships; allow the
# Go toolchain to auto-download the required version.
ENV GOTOOLCHAIN=auto

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree
COPY . .

# Build a statically-linked binary for the shorts-render Fargate task
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/shortsrender ./cmd/shortsrender

# Stage 2: Download static FFmpeg binary
FROM alpine:3.21 AS ffmpeg-downloader

RUN apk add --no-cache curl tar xz \
    && mkdir -p /ffmpeg \
    && curl -L https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz \
       | tar xJ --strip-components=1 -C /ffmpeg

# Stage 3: Plain Alpine runtime (this is an ECS Fargate task, not a Lambda)
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

# Copy static FFmpeg binaries (include libx264, libmp3lame, subtitles filter, etc.)
COPY --from=ffmpeg-downloader /ffmpeg/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg-downloader /ffmpeg/ffprobe /usr/local/bin/ffprobe

# Copy the compiled Go binary
COPY --from=builder /out/shortsrender /app/shortsrender

# Ensure binaries are executable
RUN chmod +x /app/shortsrender /usr/local/bin/ffmpeg /usr/local/bin/ffprobe

# ECS task entry point — runs to completion and exits (no Lambda runtime)
ENTRYPOINT ["/app/shortsrender"]
