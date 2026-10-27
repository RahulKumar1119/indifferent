# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS gobuild

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

# Stage 2: Plain Alpine runtime (this is an ECS Fargate task, not a Lambda).
#
# FFmpeg comes from Alpine's own package, not the johnvansickle static build
# the thumbnails task avoids as well (#77). The static build is compiled
# without libfreetype, so it has no drawtext filter, and the freeze effect's
# impact text silently renders nothing (#174). Alpine's ffmpeg is built with
# libass AND libfreetype, so both the subtitles (caption) filter and drawtext
# work.
#
# fontconfig + DejaVu + Noto Devanagari so libass and drawtext can shape and
# rasterize text in both Latin and Hindi. Without fonts the subtitles filter
# silently produces captionless video, and without a Devanagari font Hindi
# captions render as missing-glyph boxes. ttf-dejavu installs DejaVuSans at
# /usr/share/fonts/dejavu/DejaVuSans.ttf, which impactFontOption() probes.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates ffmpeg fontconfig ttf-dejavu font-noto-devanagari

# Copy the compiled Go binary
COPY --from=gobuild /out/shortsrender /app/shortsrender

# Ensure the binary is executable
RUN chmod +x /app/shortsrender

# ECS task entry point — runs to completion and exits (no Lambda runtime)
ENTRYPOINT ["/app/shortsrender"]
