# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

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

# Stage 2: Runtime image based on AWS Lambda base for AL2023
FROM public.ecr.aws/lambda/provided:al2023

# Install FFmpeg with H.264 (libx264), MP3, and PNG support
RUN dnf install -y \
        ffmpeg \
        libx264 \
        libpng \
        libmp3lame \
    && dnf clean all \
    && rm -rf /var/cache/dnf

# Copy the compiled Go binary as the Lambda bootstrap handler
COPY --from=builder /out/bootstrap ${LAMBDA_TASK_ROOT}/bootstrap

# Ensure the binary is executable
RUN chmod +x ${LAMBDA_TASK_ROOT}/bootstrap

# Lambda container image entry point
ENTRYPOINT ["/var/task/bootstrap"]
