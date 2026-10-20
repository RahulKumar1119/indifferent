# Thumbnail generation task (issue #77). Built from the backend module root:
#   docker build -f cmd/thumbnails/Dockerfile -t <image> .
#
# A static Go binary plus FFmpeg on Alpine.

# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS builder
# go.mod may require a newer patch release than the base image ships; allow the
# Go toolchain to auto-download the required version.
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/thumbnails ./cmd/thumbnails

# Stage 2: Plain Alpine runtime (this is an ECS Fargate task, not a Lambda).
# FFmpeg comes from the distribution package: this task only decodes video,
# scales and writes JPEGs, which the stock build does, and it avoids depending
# on a third-party download host at build time. No fonts are installed either:
# the thumbnail text is drawn by the binary with fonts compiled in.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates ffmpeg
COPY --from=builder /out/thumbnails /app/thumbnails
RUN chmod +x /app/thumbnails

# Runs to completion and exits (no Lambda runtime).
ENTRYPOINT ["/app/thumbnails"]
