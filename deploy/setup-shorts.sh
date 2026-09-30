#!/bin/bash
set -euo pipefail

# =============================================================================
# AI Shorts Generator infrastructure setup.
#
# Adds the shorts pipeline resources on top of the base stack created by
# deploy/setup.sh:
#   - two new Lambdas (shorts-transcribe, shorts-rank)
#   - an ECS cluster with FARGATE + FARGATE_SPOT capacity providers
#   - the shorts-render container image (ECR) + Fargate task definition
#   - IAM roles (ECS task execution, ECS task) and policy additions to the
#     existing Lambda and Step Functions roles
#   - the shorts Step Functions state machine
#   - the SHORTS_STATE_MACHINE_ARN env var on the API Lambda
#
# It reuses the same REGION / PROJECT / ACCOUNT_ID conventions as setup.sh and
# the existing assets bucket and DynamoDB table.
#
# NOTE: This script is not executed automatically; it touches live AWS. Run it
# manually once the base stack exists.
#
# PREREQUISITE (manual, not scriptable as one idempotent call):
#   Enable Bedrock model access in the target region console before the ranking
#   Lambda can invoke it. The default ranking model is Moonshot Kimi
#   (moonshotai.kimi-k3, set via the RANKING_MODEL env var, default = kimi).
#   Amazon Nova Pro and Anthropic Claude are selectable fallbacks
#   (RANKING_MODEL=nova or claude) if Kimi is unavailable in the region.
# =============================================================================

REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

# ---------------------------------------------------------------------------
# Secure Fargate networking (vpc-1369b878, ap-south-1).
#
# Security model:
#   - A DEDICATED task security group with NO inbound rules and egress limited
#     to HTTPS (443) only — nothing can reach the tasks, and tasks can only make
#     outbound TLS calls.
#   - VPC ENDPOINTS keep AWS-service traffic inside the AWS network:
#       * Gateway endpoints (free): S3, DynamoDB
#       * Interface endpoints: ECR api+dkr, CloudWatch Logs, STS, Bedrock
#         runtime, Transcribe
#   - Tasks still run in the (public) subnets and get a public IP so the ECR
#     image pull and any non-endpoint call works, but the locked-down SG means
#     only outbound 443 is permitted.
# Override any value via env vars if the VPC changes.
# ---------------------------------------------------------------------------
VPC_ID="${SHORTS_VPC_ID:-vpc-1369b878}"
SUBNET_IDS="${SHORTS_SUBNET_IDS:-subnet-f01f6e8b,subnet-880ef0e3,subnet-7bf7d837}"
ROUTE_TABLE_ID="${SHORTS_ROUTE_TABLE_ID:-rtb-23871148}"

echo "=== Creating dedicated Fargate task security group ==="

# Dedicated SG: no inbound, egress 443 only.
SECURITY_GROUP_ID=$(aws ec2 create-security-group \
  --group-name ${PROJECT}-shorts-task-sg \
  --description "Locked-down SG for shorts Fargate render tasks (egress 443 only)" \
  --vpc-id "$VPC_ID" \
  --region $REGION \
  --query 'GroupId' --output text 2>/dev/null) || \
SECURITY_GROUP_ID=$(aws ec2 describe-security-groups \
  --filters "Name=group-name,Values=${PROJECT}-shorts-task-sg" "Name=vpc-id,Values=$VPC_ID" \
  --region $REGION --query 'SecurityGroups[0].GroupId' --output text)

# create-security-group already grants default allow-all egress; revoke it and
# add HTTPS-only egress. Ignore errors if already in the desired state.
aws ec2 revoke-security-group-egress \
  --group-id "$SECURITY_GROUP_ID" \
  --protocol -1 --port -1 --cidr 0.0.0.0/0 \
  --region $REGION 2>/dev/null || true

aws ec2 authorize-security-group-egress \
  --group-id "$SECURITY_GROUP_ID" \
  --protocol tcp --port 443 --cidr 0.0.0.0/0 \
  --region $REGION 2>/dev/null || echo "Egress 443 rule already present"

echo "Task security group: $SECURITY_GROUP_ID"

echo "=== Creating FREE gateway VPC endpoints (S3 + DynamoDB) ==="

# Cost-optimized posture: only the FREE gateway endpoints are created. They
# keep the largest traffic (S3 video objects + DynamoDB) inside the AWS network
# at zero hourly cost. Interface endpoints (ECR/Logs/Bedrock/Transcribe) are
# intentionally omitted to avoid ~$42/mo of idle charges; that traffic still
# leaves via the locked-down task SG (egress 443 only, TLS + IAM authenticated),
# which is an appropriate posture at low volume.
for svc in s3 dynamodb; do
  aws ec2 create-vpc-endpoint \
    --vpc-id "$VPC_ID" \
    --vpc-endpoint-type Gateway \
    --service-name com.amazonaws.${REGION}.${svc} \
    --route-table-ids "$ROUTE_TABLE_ID" \
    --region $REGION 2>/dev/null || echo "Gateway endpoint ${svc} already exists"
done

LAMBDA_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${PROJECT}-lambda-role"

# ===========================================================================
# 13.1  New Lambdas: shorts-transcribe, shorts-rank
# ===========================================================================
echo "=== Building and deploying shorts Lambdas ==="

cd "$(dirname "$0")/../backend"

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap-shortstranscribe ./cmd/shortstranscribe
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap-shortsrank ./cmd/shortsrank

for fn in shortstranscribe shortsrank; do
  cp /tmp/bootstrap-${fn} /tmp/bootstrap
  (cd /tmp && zip -j ${fn}.zip bootstrap)
done

# shorts-transcribe (timeout 60s)
TRANSCRIBE_ENV="{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\",\"DYNAMODB_TABLE\":\"${PROJECT}-projects\"}}"
if aws lambda get-function --function-name ${PROJECT}-shorts-transcribe --region $REGION >/dev/null 2>&1; then
  aws lambda update-function-code \
    --function-name ${PROJECT}-shorts-transcribe \
    --zip-file fileb:///tmp/shortstranscribe.zip \
    --region $REGION
else
  aws lambda create-function \
    --function-name ${PROJECT}-shorts-transcribe \
    --runtime provided.al2023 \
    --handler bootstrap \
    --role $LAMBDA_ROLE_ARN \
    --zip-file fileb:///tmp/shortstranscribe.zip \
    --memory-size 256 \
    --timeout 60 \
    --environment "$TRANSCRIBE_ENV" \
    --region $REGION
fi

# shorts-rank (timeout 120s). RANKING_MODEL default empty => service default.
RANK_ENV="{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\",\"RANKING_MODEL\":\"${RANKING_MODEL:-}\"}}"
if aws lambda get-function --function-name ${PROJECT}-shorts-rank --region $REGION >/dev/null 2>&1; then
  aws lambda update-function-code \
    --function-name ${PROJECT}-shorts-rank \
    --zip-file fileb:///tmp/shortsrank.zip \
    --region $REGION
else
  aws lambda create-function \
    --function-name ${PROJECT}-shorts-rank \
    --runtime provided.al2023 \
    --handler bootstrap \
    --role $LAMBDA_ROLE_ARN \
    --zip-file fileb:///tmp/shortsrank.zip \
    --memory-size 512 \
    --timeout 120 \
    --environment "$RANK_ENV" \
    --region $REGION
fi

# ===========================================================================
# 13.3  IAM roles (created before the task definition references them)
# ===========================================================================
echo "=== Creating ECS IAM roles ==="

ECS_TRUST='{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs-tasks.amazonaws.com"},"Action":"sts:AssumeRole"}]}'

# Task execution role: ECR pull + CloudWatch Logs.
aws iam create-role \
  --role-name ${PROJECT}-shorts-task-exec-role \
  --assume-role-policy-document "$ECS_TRUST" 2>/dev/null || echo "Task exec role already exists"

aws iam attach-role-policy \
  --role-name ${PROJECT}-shorts-task-exec-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy

# Task role: S3 read/write + DynamoDB update for the render container.
aws iam create-role \
  --role-name ${PROJECT}-shorts-task-role \
  --assume-role-policy-document "$ECS_TRUST" 2>/dev/null || echo "Task role already exists"

cat > /tmp/shorts-task-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:HeadObject"],
      "Resource": "arn:aws:s3:::${PROJECT}-assets/*"
    },
    {
      "Effect": "Allow",
      "Action": ["dynamodb:UpdateItem", "dynamodb:GetItem"],
      "Resource": "arn:aws:dynamodb:${REGION}:*:table/${PROJECT}-*"
    }
  ]
}
EOF

aws iam put-role-policy \
  --role-name ${PROJECT}-shorts-task-role \
  --policy-name ${PROJECT}-shorts-task-permissions \
  --policy-document file:///tmp/shorts-task-policy.json

# Lambda role additions: Transcribe + Bedrock (plus existing S3/DynamoDB).
cat > /tmp/shorts-lambda-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["transcribe:StartTranscriptionJob", "transcribe:GetTranscriptionJob"],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": ["bedrock:InvokeModel"],
      "Resource": "*"
    }
  ]
}
EOF

aws iam put-role-policy \
  --role-name ${PROJECT}-lambda-role \
  --policy-name ${PROJECT}-shorts-lambda-permissions \
  --policy-document file:///tmp/shorts-lambda-policy.json

# Step Functions role additions: ECS runTask.sync managed integration.
cat > /tmp/shorts-sfn-policy.json << EOF
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
        "arn:aws:iam::${ACCOUNT_ID}:role/${PROJECT}-shorts-task-exec-role",
        "arn:aws:iam::${ACCOUNT_ID}:role/${PROJECT}-shorts-task-role"
      ]
    },
    {
      "Effect": "Allow",
      "Action": ["events:PutTargets", "events:PutRule", "events:DescribeRule"],
      "Resource": "arn:aws:events:${REGION}:${ACCOUNT_ID}:rule/StepFunctionsGetEventsForECSTaskRule"
    },
    {
      "Effect": "Allow",
      "Action": ["lambda:InvokeFunction"],
      "Resource": "arn:aws:lambda:${REGION}:*:function:${PROJECT}-shorts-*"
    }
  ]
}
EOF

aws iam put-role-policy \
  --role-name ${PROJECT}-sfn-role \
  --policy-name ${PROJECT}-shorts-sfn-permissions \
  --policy-document file:///tmp/shorts-sfn-policy.json

echo "=== Waiting for IAM propagation (10s) ==="
sleep 10

# ===========================================================================
# 13.2  ECS cluster + Fargate Spot + render task definition
# ===========================================================================
echo "=== Creating ECS cluster ==="

aws ecs create-cluster \
  --cluster-name ${PROJECT}-shorts \
  --capacity-providers FARGATE FARGATE_SPOT \
  --default-capacity-provider-strategy capacityProvider=FARGATE_SPOT,weight=1 \
  --region $REGION 2>/dev/null || echo "Cluster already exists"

echo "=== Building and pushing shorts-render image to ECR ==="

SHORTS_RENDER_REPO="${PROJECT}-shorts-render"
SHORTS_RENDER_IMAGE="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${SHORTS_RENDER_REPO}:latest"

aws ecr create-repository \
  --repository-name ${SHORTS_RENDER_REPO} \
  --region $REGION 2>/dev/null || echo "ECR repo already exists"

aws ecr get-login-password --region $REGION \
  | docker login --username AWS --password-stdin ${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com

# Build from the backend module root so the Dockerfile can COPY the source tree.
docker build \
  -f cmd/shortsrender/Dockerfile \
  -t ${SHORTS_RENDER_IMAGE} \
  .
docker push ${SHORTS_RENDER_IMAGE}

# CloudWatch Logs group for the task.
aws logs create-log-group \
  --log-group-name /ecs/${PROJECT}-shorts-render \
  --region $REGION 2>/dev/null || echo "Log group already exists"

echo "=== Registering shorts-render task definition ==="

aws ecs register-task-definition \
  --family ${PROJECT}-shorts-render \
  --requires-compatibilities FARGATE \
  --network-mode awsvpc \
  --cpu 1024 --memory 4096 \
  --execution-role-arn arn:aws:iam::${ACCOUNT_ID}:role/${PROJECT}-shorts-task-exec-role \
  --task-role-arn arn:aws:iam::${ACCOUNT_ID}:role/${PROJECT}-shorts-task-role \
  --container-definitions "[{\"name\":\"shorts-render\",\"image\":\"${SHORTS_RENDER_IMAGE}\",\"essential\":true,\"environment\":[{\"name\":\"S3_BUCKET\",\"value\":\"${PROJECT}-assets\"}],\"logConfiguration\":{\"logDriver\":\"awslogs\",\"options\":{\"awslogs-group\":\"/ecs/${PROJECT}-shorts-render\",\"awslogs-region\":\"${REGION}\",\"awslogs-stream-prefix\":\"render\"}}}]" \
  --region $REGION

# ===========================================================================
# 13.4  Shorts Step Functions state machine + wire API env var
# ===========================================================================
echo "=== Creating shorts Step Functions state machine ==="

ECS_CLUSTER_ARN="arn:aws:ecs:${REGION}:${ACCOUNT_ID}:cluster/${PROJECT}-shorts"
SHORTS_RENDER_TASKDEF_ARN="arn:aws:ecs:${REGION}:${ACCOUNT_ID}:task-definition/${PROJECT}-shorts-render"
STATUS_UPDATER_ARN="arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-statusupdater"
TRANSCRIBE_ARN="arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-shorts-transcribe"
RANK_ARN="arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-shorts-rank"
SFN_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${PROJECT}-sfn-role"

# Substitute all ${...} placeholders in the state machine definition.
# The statemachine.json holds the subnet list as the single-element JSON array
# ["${SubnetIds}"]; expand the comma-separated $SUBNET_IDS into a JSON array and
# replace the whole token so multiple subnets are supported.
SUBNET_JSON="[$(echo "$SUBNET_IDS" | awk -F, '{for(i=1;i<=NF;i++){printf (i>1?",":"") "\"" $i "\""}}')]"

DEFINITION=$(sed \
  -e "s|\${StatusUpdaterShortsArn}|${STATUS_UPDATER_ARN}|g" \
  -e "s|\${ShortsTranscribeLambdaArn}|${TRANSCRIBE_ARN}|g" \
  -e "s|\${ShortsRankLambdaArn}|${RANK_ARN}|g" \
  -e "s|\${EcsClusterArn}|${ECS_CLUSTER_ARN}|g" \
  -e "s|\${ShortsRenderTaskDefArn}|${SHORTS_RENDER_TASKDEF_ARN}|g" \
  -e "s|\[\"\${SubnetIds}\"\]|${SUBNET_JSON}|g" \
  -e "s|\${SecurityGroupId}|${SECURITY_GROUP_ID}|g" \
  ../backend/internal/shorts/pipeline/statemachine.json)

echo "$DEFINITION" > /tmp/shorts-statemachine.json

aws stepfunctions create-state-machine \
  --name ${PROJECT}-shorts-pipeline \
  --definition file:///tmp/shorts-statemachine.json \
  --role-arn $SFN_ROLE_ARN \
  --type STANDARD \
  --region $REGION 2>/dev/null || \
aws stepfunctions update-state-machine \
  --state-machine-arn "arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${PROJECT}-shorts-pipeline" \
  --definition file:///tmp/shorts-statemachine.json \
  --role-arn $SFN_ROLE_ARN \
  --region $REGION

echo "=== Wiring SHORTS_STATE_MACHINE_ARN into the API Lambda ==="

SHORTS_SFN_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${PROJECT}-shorts-pipeline"

# Merge the new env var into the API Lambda's existing environment.
CURRENT_ENV=$(aws lambda get-function-configuration \
  --function-name ${PROJECT}-api \
  --region $REGION \
  --query 'Environment.Variables' --output json)

MERGED_ENV=$(echo "$CURRENT_ENV" | python3 -c "
import json, sys
raw = sys.stdin.read().strip()
e = json.loads(raw) if raw and raw != 'null' else {}
e['SHORTS_STATE_MACHINE_ARN'] = sys.argv[1]
print(json.dumps({'Variables': e}))
" "$SHORTS_SFN_ARN")

aws lambda update-function-configuration \
  --function-name ${PROJECT}-api \
  --environment "$MERGED_ENV" \
  --region $REGION

echo "=== Done! Shorts infrastructure created ==="
echo "Transcribe Lambda: ${PROJECT}-shorts-transcribe"
echo "Rank Lambda:       ${PROJECT}-shorts-rank"
echo "ECS Cluster:       ${PROJECT}-shorts"
echo "Render Task Def:   ${PROJECT}-shorts-render"
echo "State Machine:     ${SHORTS_SFN_ARN}"
