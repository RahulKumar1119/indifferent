#!/bin/bash
# Runs the single-clip render record tests (#329) against DynamoDB Local, so the
# conditional writes that stop two renders racing are checked by DynamoDB itself.
# Read-only on the repository and on AWS: it starts a local container and removes it.
#
# Needs: docker, go.
#
#   scripts/ops/cliprender_dynamo_check.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE="amazon/dynamodb-local:2.5.4"
PORT="${PORT:-18000}"
# The JVM in the image cannot start its threads under some hosts' default seccomp
# profile (pthread_create EPERM); the container is local and short-lived.
cid=$(docker run -d --rm --security-opt seccomp=unconfined -p "127.0.0.1:${PORT}:8000" "$IMAGE" -jar DynamoDBLocal.jar -inMemory)
trap 'docker stop "$cid" >/dev/null 2>&1 || true' EXIT
for _ in $(seq 1 30); do
  if curl -s -o /dev/null "http://127.0.0.1:${PORT}"; then break; fi
  sleep 1
done
cd "$ROOT/backend"
DYNAMODB_LOCAL_ENDPOINT="http://127.0.0.1:${PORT}" go test -count=1 -v -run 'TestLocal' ./internal/shorts/cliprender/ ./internal/api/ ./cmd/shortsrender/
