# Build context is the backend/ directory:
#   docker build -f cmd/storyrender/Dockerfile -t indifferent-fun-story-render backend/

# Stage 1: build the Go binary
FROM golang:1.26-alpine AS gobuild

# go.mod may require a newer patch release than the base image ships; allow the
# Go toolchain to download the required version.
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/storyrender ./cmd/storyrender

# Stage 2: plain Alpine runtime (an ECS Fargate task, not a Lambda).
#
# Same packages as the shorts-render image, for the same reasons:
#   - ffmpeg from Alpine's own package. The johnvansickle static build has no
#     libfreetype and so no drawtext, and Alpine's has libass for the burned-in
#     captions (#174). ffprobe comes with it.
#   - fontconfig + DejaVu + Noto Devanagari, so captions render in Latin and
#     Hindi. Without a font libass draws nothing and the video has no captions
#     and no error; the worker checks for fonts at start-up and fails if none.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates ffmpeg fontconfig ttf-dejavu font-noto-devanagari

COPY --from=gobuild /out/storyrender /app/storyrender
RUN chmod +x /app/storyrender

# Runs to completion and exits; the exit code is the contract with the state
# machine (see internal/storyjob/worker.go).
ENTRYPOINT ["/app/storyrender"]
