#!/bin/bash
set -euo pipefail
# AI thumbnails infrastructure (issue #77)
# =========================================
#
# Creates or updates, in this order:
#
#   1. ECR repository       indifferent-fun-thumbnails  (image built from backend/cmd/thumbnails)
#   2. CloudWatch log group /ecs/indifferent-fun-thumbnails
#   3. IAM task role        indifferent-fun-thumbnails-task-role (least privilege:
#                           S3 on the assets bucket, DynamoDB on the projects table,
#                           Rekognition detect, Bedrock invoke)
#   4. ECS task definition  indifferent-fun-thumbnails  (2 vCPU / 4 GB, Fargate)
#   5. Step Functions       indifferent-fun-thumbnails  (runs the task, Spot first
#                           then on-demand; marks the record failed if the task dies)
#      + permissions added to the existing Step Functions role
#   6. API Lambda           code deploy, states:StartExecution permission and the
#                           THUMBNAIL_STATE_MACHINE_ARN setting
#
# It reuses the Shorts cluster, subnets, locked-down security group (egress 443
# only) and task execution role.
#
# Run deploy/patch-shorts-thumbnails.sh afterwards to make the Shorts pipeline
# start thumbnails automatically once a video has been transcribed. Until then
# thumbnails start when the user presses Generate.
#
# Prerequisites: Bedrock access to the model in THUMBNAIL_MODEL (default
# global.anthropic.claude-haiku-4-5-20251001-v1:0) and Rekognition, which the
# Shorts renderer already uses. Docker, Go and zip are needed to apply.
#
# Usage:
#   bash deploy/setup-thumbnails.sh            # dry run: read-only calls, prints the plan
#   bash deploy/setup-thumbnails.sh --apply    # makes the changes
#
# Safety:
#   - Idempotent: each step checks current state first; re-running is safe.
#   - The dry run makes only read calls and a local Go build. Nothing is created
#     or changed in AWS.
#   - Nothing existing is modified except: the Step Functions role and the API
#     Lambda role each gain one new inline policy, and the API Lambda gets one
#     new environment variable and new code. The Shorts state machine is NOT
#     touched here.
#   - Until THUMBNAIL_STATE_MACHINE_ARN is set the thumbnail endpoints answer
#     503, so deploying the API code first is harmless.
#
# Undo:
#   aws lambda update-function-configuration --function-name indifferent-fun-api \
#       --environment <current variables without THUMBNAIL_STATE_MACHINE_ARN>
#   aws stepfunctions delete-state-machine --state-machine-arn <thumbnails arn>
#   aws iam delete-role-policy --role-name indifferent-fun-sfn-role --policy-name indifferent-fun-thumbnails-sfn-permissions
#   aws iam delete-role-policy --role-name indifferent-fun-lambda-role --policy-name indifferent-fun-thumbnails-lambda-permissions
#   aws ecs deregister-task-definition --task-definition indifferent-fun-thumbnails:<revision>
#   aws iam delete-role-policy --role-name indifferent-fun-thumbnails-task-role --policy-name indifferent-fun-thumbnails-task-permissions
#   aws iam delete-role --role-name indifferent-fun-thumbnails-task-role
#   aws ecr delete-repository --repository-name indifferent-fun-thumbnails --force
#   aws logs delete-log-group --log-group-name /ecs/indifferent-fun-thumbnails
# The API code rolls back like any other Lambda deploy (re-upload the previous zip).
REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
BUCKET="${PROJECT}-assets"
TABLE="${PROJECT}-projects"
MODEL="${THUMBNAIL_MODEL:-}"
REPO="${PROJECT}-thumbnails"
IMAGE_TAG="t77-$(date -u +%Y%m%d-%H%M)"
IMAGE_BASE="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${REPO}"
LOG_GROUP="/ecs/${PROJECT}-thumbnails"
TASK_ROLE="${PROJECT}-thumbnails-task-role"
EXEC_ROLE="${PROJECT}-shorts-task-exec-role"
TASK_FAMILY="${PROJECT}-thumbnails"
TASK_CPU="${THUMBNAILS_CPU:-2048}"
TASK_MEMORY="${THUMBNAILS_MEMORY:-4096}"
CLUSTER="${PROJECT}-shorts"
SM_NAME="${PROJECT}-thumbnails"
SM_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${SM_NAME}"
SFN_ROLE="${PROJECT}-sfn-role"
LAMBDA_ROLE="${PROJECT}-lambda-role"
API_FUNCTION="${PROJECT}-api"
SG_NAME="${PROJECT}-shorts-task-sg"
SUBNET_IDS="${SHORTS_SUBNET_IDS:-subnet-f01f6e8b,subnet-880ef0e3,subnet-7bf7d837}"
BACKEND_DIR="$(cd "$(dirname "$0")/../backend" && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

APPLY=false
if [ "${1:-}" = "--apply" ]; then
  APPLY=true
elif [ -n "${1:-}" ]; then
  echo "Unknown argument: $1 (use --apply or no argument)" >&2
  exit 1
fi
if $APPLY; then
  echo "MODE: apply (changes will be made in account ${ACCOUNT_ID}, ${REGION})"
else
  echo "MODE: dry run (read-only; nothing will be created or changed)"
fi
echo

# act "<what>" <command...>: runs the command when applying, otherwise prints it.
act() {
  local what="$1"
  shift
  if $APPLY; then
    echo ">> ${what}"
    "$@"
  else
    echo "[dry-run] would: ${what}"
  fi
}

# ---------------------------------------------------------------------------
echo "=== 0. Preflight ==="
aws ecs describe-clusters --clusters "$CLUSTER" --region "$REGION" \
  --query 'clusters[0].status' --output text | grep -qx ACTIVE \
  || { echo "ERROR: ECS cluster ${CLUSTER} is not ACTIVE (run deploy/setup-shorts.sh first)." >&2; exit 1; }
SECURITY_GROUP_ID=$(aws ec2 describe-security-groups --filters "Name=group-name,Values=${SG_NAME}" \
  --region "$REGION" --query 'SecurityGroups[0].GroupId' --output text)
[ -n "$SECURITY_GROUP_ID" ] && [ "$SECURITY_GROUP_ID" != "None" ] \
  || { echo "ERROR: security group ${SG_NAME} not found (run deploy/setup-shorts.sh first)." >&2; exit 1; }
aws iam get-role --role-name "$EXEC_ROLE" >/dev/null \
  || { echo "ERROR: role ${EXEC_ROLE} not found." >&2; exit 1; }
aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" >/dev/null \
  || { echo "ERROR: Lambda ${API_FUNCTION} not found." >&2; exit 1; }
echo "cluster ${CLUSTER}, security group ${SECURITY_GROUP_ID}, role ${EXEC_ROLE}, Lambda ${API_FUNCTION}: ok"
echo "Local build check:"
( cd "$BACKEND_DIR" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$TMP/thumbnails" ./cmd/thumbnails ) \
  && echo "  cmd/thumbnails builds"
echo

# ---------------------------------------------------------------------------
echo "=== 1. ECR repository and image ==="
if aws ecr describe-repositories --repository-names "$REPO" --region "$REGION" >/dev/null 2>&1; then
  echo "repository ${REPO} exists"
else
  act "create ECR repository ${REPO}" aws ecr create-repository --repository-name "$REPO" --region "$REGION" --query repository.repositoryUri --output text
fi
if $APPLY; then
  aws ecr get-login-password --region "$REGION" \
    | docker login --username AWS --password-stdin "${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
  ( cd "$BACKEND_DIR" && docker build -f cmd/thumbnails/Dockerfile -t "${IMAGE_BASE}:${IMAGE_TAG}" -t "${IMAGE_BASE}:latest" . )
  docker push "${IMAGE_BASE}:${IMAGE_TAG}"
  docker push "${IMAGE_BASE}:latest"
else
  echo "[dry-run] would: build and push ${IMAGE_BASE}:${IMAGE_TAG} and :latest"
fi
echo

# ---------------------------------------------------------------------------
echo "=== 2. Log group ==="
if [ "$(aws logs describe-log-groups --log-group-name-prefix "$LOG_GROUP" --region "$REGION" \
    --query "length(logGroups[?logGroupName=='${LOG_GROUP}'])" --output text)" != "0" ]; then
  echo "log group ${LOG_GROUP} exists"
else
  act "create log group ${LOG_GROUP}" aws logs create-log-group --log-group-name "$LOG_GROUP" --region "$REGION"
  act "keep logs for 30 days" aws logs put-retention-policy --log-group-name "$LOG_GROUP" --retention-in-days 30 --region "$REGION"
fi
echo

# ---------------------------------------------------------------------------
echo "=== 3. Task role ==="
ECS_TRUST='{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs-tasks.amazonaws.com"},"Action":"sts:AssumeRole"}]}'
if aws iam get-role --role-name "$TASK_ROLE" >/dev/null 2>&1; then
  echo "role ${TASK_ROLE} exists"
else
  act "create role ${TASK_ROLE}" aws iam create-role --role-name "$TASK_ROLE" --assume-role-policy-document "$ECS_TRUST" --query Role.Arn --output text
fi
cat > "$TMP/task-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ReadSourceAndWriteThumbnails",
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
      "Resource": "arn:aws:s3:::${BUCKET}/*"
    },
    {
      "Sid": "ListForCleanupAndCleanNotFound",
      "Effect": "Allow",
      "Action": ["s3:ListBucket"],
      "Resource": "arn:aws:s3:::${BUCKET}"
    },
    {
      "Sid": "RecordProgressAndFindBranding",
      "Effect": "Allow",
      "Action": ["dynamodb:GetItem", "dynamodb:UpdateItem"],
      "Resource": "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${TABLE}"
    },
    {
      "Sid": "FacesAndObjects",
      "Effect": "Allow",
      "Action": ["rekognition:DetectFaces", "rekognition:DetectLabels"],
      "Resource": "*"
    },
    {
      "Sid": "AnalyzeFramesAndTranscript",
      "Effect": "Allow",
      "Action": ["bedrock:InvokeModel"],
      "Resource": [
        "arn:aws:bedrock:*::foundation-model/*",
        "arn:aws:bedrock:*:${ACCOUNT_ID}:inference-profile/*"
      ]
    }
  ]
}
EOF
act "set permissions on ${TASK_ROLE}" aws iam put-role-policy --role-name "$TASK_ROLE" \
  --policy-name "${PROJECT}-thumbnails-task-permissions" --policy-document "file://${TMP}/task-policy.json"
echo

# ---------------------------------------------------------------------------
echo "=== 4. Task definition ==="
cat > "$TMP/containers.json" <<EOF
[{
  "name": "thumbnails",
  "image": "${IMAGE_BASE}:${IMAGE_TAG}",
  "essential": true,
  "environment": [
    {"name": "S3_BUCKET", "value": "${BUCKET}"},
    {"name": "DYNAMODB_TABLE", "value": "${TABLE}"},
    {"name": "THUMBNAIL_MODEL", "value": "${MODEL}"}
  ],
  "logConfiguration": {
    "logDriver": "awslogs",
    "options": {
      "awslogs-group": "${LOG_GROUP}",
      "awslogs-region": "${REGION}",
      "awslogs-stream-prefix": "thumbnails"
    }
  }
}]
EOF
if $APPLY; then
  # IAM changes can take a few seconds to be usable by ECS.
  sleep 10
fi
act "register task definition ${TASK_FAMILY} (${TASK_CPU} CPU / ${TASK_MEMORY} MB, image ${IMAGE_TAG})" \
  aws ecs register-task-definition --family "$TASK_FAMILY" --requires-compatibilities FARGATE \
  --network-mode awsvpc --cpu "$TASK_CPU" --memory "$TASK_MEMORY" \
  --execution-role-arn "arn:aws:iam::${ACCOUNT_ID}:role/${EXEC_ROLE}" \
  --task-role-arn "arn:aws:iam::${ACCOUNT_ID}:role/${TASK_ROLE}" \
  --container-definitions "file://${TMP}/containers.json" --region "$REGION" --query 'taskDefinition.revision' --output text
echo

# ---------------------------------------------------------------------------
echo "=== 5. Step Functions ==="
cat > "$TMP/sfn-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["ecs:RunTask", "ecs:StopTask", "ecs:DescribeTasks"],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": ["iam:PassRole"],
      "Resource": [
        "arn:aws:iam::${ACCOUNT_ID}:role/${EXEC_ROLE}",
        "arn:aws:iam::${ACCOUNT_ID}:role/${TASK_ROLE}"
      ]
    },
    {
      "Effect": "Allow",
      "Action": ["events:PutTargets", "events:PutRule", "events:DescribeRule"],
      "Resource": "arn:aws:events:${REGION}:${ACCOUNT_ID}:rule/StepFunctionsGetEventsForECSTaskRule"
    },
    {
      "Effect": "Allow",
      "Action": ["dynamodb:UpdateItem"],
      "Resource": "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${TABLE}"
    },
    {
      "Sid": "ShortsPipelineStartsThumbnails",
      "Effect": "Allow",
      "Action": ["states:StartExecution"],
      "Resource": "${SM_ARN}"
    }
  ]
}
EOF
act "add permissions to ${SFN_ROLE}" aws iam put-role-policy --role-name "$SFN_ROLE" \
  --policy-name "${PROJECT}-thumbnails-sfn-permissions" --policy-document "file://${TMP}/sfn-policy.json"

SUBNET_JSON="[$(echo "$SUBNET_IDS" | awk -F, '{for(i=1;i<=NF;i++){printf (i>1?",":"") "\"" $i "\""}}')]"
sed \
  -e "s|\${EcsClusterArn}|arn:aws:ecs:${REGION}:${ACCOUNT_ID}:cluster/${CLUSTER}|g" \
  -e "s|\${ThumbnailsTaskDefArn}|arn:aws:ecs:${REGION}:${ACCOUNT_ID}:task-definition/${TASK_FAMILY}|g" \
  -e "s|\[\"\${SubnetIds}\"\]|${SUBNET_JSON}|g" \
  -e "s|\${SecurityGroupId}|${SECURITY_GROUP_ID}|g" \
  -e "s|\${DynamoTable}|${TABLE}|g" \
  "${BACKEND_DIR}/internal/shorts/pipeline/thumbnails.json" > "$TMP/thumbnails-sm.json"
if grep -q '\${' "$TMP/thumbnails-sm.json"; then
  echo "ERROR: unreplaced placeholders in the state machine definition." >&2
  exit 1
fi
RESULT=$(aws stepfunctions validate-state-machine-definition --type STANDARD \
  --definition "file://${TMP}/thumbnails-sm.json" --region "$REGION" --query 'result' --output text)
if [ "$RESULT" != "OK" ]; then
  echo "ERROR: state machine definition did not validate (${RESULT})." >&2
  exit 1
fi
echo "state machine definition validates"
SFN_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${SFN_ROLE}"
if aws stepfunctions describe-state-machine --state-machine-arn "$SM_ARN" --region "$REGION" >/dev/null 2>&1; then
  act "update state machine ${SM_NAME}" aws stepfunctions update-state-machine --state-machine-arn "$SM_ARN" \
    --definition "file://${TMP}/thumbnails-sm.json" --role-arn "$SFN_ROLE_ARN" --region "$REGION" --query updateDate --output text
else
  act "create state machine ${SM_NAME}" aws stepfunctions create-state-machine --name "$SM_NAME" \
    --definition "file://${TMP}/thumbnails-sm.json" --role-arn "$SFN_ROLE_ARN" --type STANDARD --region "$REGION" \
    --query stateMachineArn --output text
fi
echo

# ---------------------------------------------------------------------------
echo "=== 6. API Lambda ==="
cat > "$TMP/lambda-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["states:StartExecution"],
      "Resource": "${SM_ARN}"
    }
  ]
}
EOF
act "allow ${LAMBDA_ROLE} to start ${SM_NAME}" aws iam put-role-policy --role-name "$LAMBDA_ROLE" \
  --policy-name "${PROJECT}-thumbnails-lambda-permissions" --policy-document "file://${TMP}/lambda-policy.json"

if $APPLY; then
  echo ">> build and deploy the API Lambda"
  ( cd "$BACKEND_DIR" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "$TMP/bootstrap" ./cmd/api )
  ( cd "$TMP" && zip -q -j api.zip bootstrap )
  # Keep the code that is live now, in case the new build has to be rolled back.
  PREV_URL=$(aws lambda get-function --function-name "$API_FUNCTION" --region "$REGION" --query 'Code.Location' --output text)
  PREV_ZIP="/tmp/${API_FUNCTION}-before-thumbnails-$(date -u +%Y%m%d-%H%M%S).zip"
  curl -fsS -o "$PREV_ZIP" "$PREV_URL"
  echo "   previous code saved to ${PREV_ZIP}"
  aws lambda update-function-code --function-name "$API_FUNCTION" --zip-file "fileb://${TMP}/api.zip" --region "$REGION" >/dev/null
  aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"

  CURRENT_ENV=$(aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
    --query 'Environment.Variables' --output json)
  echo "$CURRENT_ENV" | python3 -c "
import json, sys
raw = sys.stdin.read().strip()
env = json.loads(raw) if raw and raw != 'null' else {}
env['THUMBNAIL_STATE_MACHINE_ARN'] = sys.argv[1]
json.dump({'Variables': env}, open(sys.argv[2], 'w'))
" "$SM_ARN" "$TMP/api-env.json"
  aws lambda update-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
    --environment "file://${TMP}/api-env.json" >/dev/null
  aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"
else
  echo "[dry-run] would: build and deploy the API Lambda (saving the live code first)"
  echo "[dry-run] would: set THUMBNAIL_STATE_MACHINE_ARN=${SM_ARN} on ${API_FUNCTION}"
fi
echo

if $APPLY; then
  echo "Done. Next: bash deploy/patch-shorts-thumbnails.sh --apply   (start thumbnails automatically)"
else
  echo "Dry run complete. Re-run with --apply to make these changes."
fi
