#!/bin/bash
set -euo pipefail

# Required environment variables (set these before running):
# GOOGLE_CLIENT_ID - Google OAuth Client ID
# GOOGLE_CLIENT_SECRET - Google OAuth Client Secret
# JWT_SECRET - JWT signing secret (generate with: openssl rand -hex 32)

if [ -z "${GOOGLE_CLIENT_ID:-}" ] || [ -z "${GOOGLE_CLIENT_SECRET:-}" ] || [ -z "${JWT_SECRET:-}" ]; then
  echo "ERROR: Set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and JWT_SECRET environment variables"
  echo "Example: export GOOGLE_CLIENT_ID='your-id.apps.googleusercontent.com'"
  exit 1
fi

REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

echo "=== Creating DynamoDB Tables ==="

# Users table
aws dynamodb create-table \
  --table-name ${PROJECT}-users \
  --attribute-definitions AttributeName=PK,AttributeType=S AttributeName=SK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH AttributeName=SK,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --region $REGION 2>/dev/null || echo "Users table already exists"

# Projects table
aws dynamodb create-table \
  --table-name ${PROJECT}-projects \
  --attribute-definitions AttributeName=PK,AttributeType=S AttributeName=SK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH AttributeName=SK,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --region $REGION 2>/dev/null || echo "Projects table already exists"

# Sessions table (with TTL)
aws dynamodb create-table \
  --table-name ${PROJECT}-sessions \
  --attribute-definitions AttributeName=PK,AttributeType=S AttributeName=SK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH AttributeName=SK,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --region $REGION 2>/dev/null || echo "Sessions table already exists"

aws dynamodb update-time-to-live \
  --table-name ${PROJECT}-sessions \
  --time-to-live-specification Enabled=true,AttributeName=expiresAt \
  --region $REGION 2>/dev/null || echo "TTL already enabled"

echo "=== Creating S3 Bucket ==="

aws s3api create-bucket \
  --bucket ${PROJECT}-assets \
  --region $REGION \
  --create-bucket-configuration LocationConstraint=$REGION 2>/dev/null || echo "Bucket already exists"

aws s3api put-bucket-versioning \
  --bucket ${PROJECT}-assets \
  --versioning-configuration Status=Enabled \
  --region $REGION

aws s3api put-bucket-encryption \
  --bucket ${PROJECT}-assets \
  --server-side-encryption-configuration '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}' \
  --region $REGION

aws s3api put-public-access-block \
  --bucket ${PROJECT}-assets \
  --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true \
  --region $REGION

# CORS configuration for presigned URL uploads/downloads from frontend
aws s3api put-bucket-cors \
  --bucket ${PROJECT}-assets \
  --cors-configuration '{
    "CORSRules": [
      {
        "AllowedOrigins": ["https://indifferent.fun"],
        "AllowedMethods": ["GET", "PUT", "POST", "HEAD"],
        "AllowedHeaders": ["*"],
        "ExposeHeaders": ["ETag", "x-amz-request-id"],
        "MaxAgeSeconds": 3600
      }
    ]
  }' \
  --region $REGION

# Lifecycle rule: delete temp files after 7 days
aws s3api put-bucket-lifecycle-configuration \
  --bucket ${PROJECT}-assets \
  --lifecycle-configuration '{
    "Rules": [
      {
        "ID": "CleanupTempFiles",
        "Filter": {"Prefix": "temp/"},
        "Status": "Enabled",
        "Expiration": {"Days": 7}
      }
    ]
  }' \
  --region $REGION

echo "=== Creating IAM Roles ==="

# Lambda execution role
TRUST_POLICY='{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}'

aws iam create-role \
  --role-name ${PROJECT}-lambda-role \
  --assume-role-policy-document "$TRUST_POLICY" 2>/dev/null || echo "Lambda role already exists"

# Attach policies
aws iam attach-role-policy \
  --role-name ${PROJECT}-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

# Custom policy for DynamoDB, S3, Polly, Step Functions
cat > /tmp/lambda-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["dynamodb:PutItem","dynamodb:GetItem","dynamodb:DeleteItem","dynamodb:Query","dynamodb:UpdateItem"],
      "Resource": "arn:aws:dynamodb:ap-south-1:*:table/indifferent-fun-*"
    },
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject","s3:PutObject","s3:DeleteObject"],
      "Resource": "arn:aws:s3:::indifferent-fun-assets/*"
    },
    {
      "Effect": "Allow",
      "Action": ["polly:SynthesizeSpeech"],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": ["states:StartExecution"],
      "Resource": "arn:aws:states:ap-south-1:*:stateMachine:indifferent-fun-*"
    }
  ]
}
EOF

aws iam put-role-policy \
  --role-name ${PROJECT}-lambda-role \
  --policy-name ${PROJECT}-lambda-permissions \
  --policy-document file:///tmp/lambda-policy.json

# Step Functions role
SFN_TRUST='{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"states.amazonaws.com"},"Action":"sts:AssumeRole"}]}'

aws iam create-role \
  --role-name ${PROJECT}-sfn-role \
  --assume-role-policy-document "$SFN_TRUST" 2>/dev/null || echo "SFN role already exists"

cat > /tmp/sfn-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["lambda:InvokeFunction"],
      "Resource": "arn:aws:lambda:ap-south-1:*:function:indifferent-fun-*"
    },
    {
      "Effect": "Allow",
      "Action": ["dynamodb:UpdateItem"],
      "Resource": "arn:aws:dynamodb:ap-south-1:*:table/indifferent-fun-*"
    }
  ]
}
EOF

aws iam put-role-policy \
  --role-name ${PROJECT}-sfn-role \
  --policy-name ${PROJECT}-sfn-permissions \
  --policy-document file:///tmp/sfn-policy.json

echo "=== Waiting for IAM propagation (10s) ==="
sleep 10

echo "=== Creating Lambda Functions ==="

ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${PROJECT}-lambda-role"
RENDERER_IMAGE="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/indifferent-fun-renderer:latest"

# Build Lambda binaries
cd /home/rahul/Documents/indifferent/backend
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap-api ./cmd/api
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap-parser ./cmd/parser
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap-slidegen ./cmd/slidegen
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap-narrator ./cmd/narrator
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap-statusupdater ./cmd/statusupdater

# Package zips
for fn in api parser slidegen narrator statusupdater; do
  cp /tmp/bootstrap-${fn} /tmp/bootstrap
  cd /tmp && zip -j ${fn}.zip bootstrap && cd -
done

# Common env vars
ENV_VARS="{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\",\"DYNAMODB_TABLE\":\"${PROJECT}-projects\",\"USERS_TABLE\":\"${PROJECT}-users\",\"SESSION_TABLE\":\"${PROJECT}-sessions\",\"JWT_SECRET\":\"${JWT_SECRET}\",\"GOOGLE_CLIENT_ID\":\"${GOOGLE_CLIENT_ID}\",\"GOOGLE_CLIENT_SECRET\":\"${GOOGLE_CLIENT_SECRET}\",\"GOOGLE_REDIRECT_URI\":\"https://indifferent.fun/auth/callback\"}}"

# API Lambda
aws lambda create-function \
  --function-name ${PROJECT}-api \
  --runtime provided.al2023 \
  --handler bootstrap \
  --role $ROLE_ARN \
  --zip-file fileb:///tmp/api.zip \
  --memory-size 256 \
  --timeout 30 \
  --environment "$ENV_VARS" \
  --region $REGION 2>/dev/null || \
aws lambda update-function-code \
  --function-name ${PROJECT}-api \
  --zip-file fileb:///tmp/api.zip \
  --region $REGION

# Parser Lambda
PARSER_ENV="{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\"}}"
aws lambda create-function \
  --function-name ${PROJECT}-parser \
  --runtime provided.al2023 \
  --handler bootstrap \
  --role $ROLE_ARN \
  --zip-file fileb:///tmp/parser.zip \
  --memory-size 256 \
  --timeout 60 \
  --environment "$PARSER_ENV" \
  --region $REGION 2>/dev/null || \
aws lambda update-function-code \
  --function-name ${PROJECT}-parser \
  --zip-file fileb:///tmp/parser.zip \
  --region $REGION

# SlideGen Lambda
SLIDEGEN_ENV="{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\",\"TEMPLATE_DIR\":\"/var/task/templates\"}}"
aws lambda create-function \
  --function-name ${PROJECT}-slidegen \
  --runtime provided.al2023 \
  --handler bootstrap \
  --role $ROLE_ARN \
  --zip-file fileb:///tmp/slidegen.zip \
  --memory-size 1024 \
  --timeout 300 \
  --environment "$SLIDEGEN_ENV" \
  --region $REGION 2>/dev/null || \
aws lambda update-function-code \
  --function-name ${PROJECT}-slidegen \
  --zip-file fileb:///tmp/slidegen.zip \
  --region $REGION

# Narrator Lambda
NARRATOR_ENV="{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\"}}"
aws lambda create-function \
  --function-name ${PROJECT}-narrator \
  --runtime provided.al2023 \
  --handler bootstrap \
  --role $ROLE_ARN \
  --zip-file fileb:///tmp/narrator.zip \
  --memory-size 256 \
  --timeout 300 \
  --environment "$NARRATOR_ENV" \
  --region $REGION 2>/dev/null || \
aws lambda update-function-code \
  --function-name ${PROJECT}-narrator \
  --zip-file fileb:///tmp/narrator.zip \
  --region $REGION

# StatusUpdater Lambda
STATUS_ENV="{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\",\"DYNAMODB_TABLE\":\"${PROJECT}-projects\"}}"
aws lambda create-function \
  --function-name ${PROJECT}-statusupdater \
  --runtime provided.al2023 \
  --handler bootstrap \
  --role $ROLE_ARN \
  --zip-file fileb:///tmp/statusupdater.zip \
  --memory-size 256 \
  --timeout 30 \
  --environment "$STATUS_ENV" \
  --region $REGION 2>/dev/null || \
aws lambda update-function-code \
  --function-name ${PROJECT}-statusupdater \
  --zip-file fileb:///tmp/statusupdater.zip \
  --region $REGION

# Renderer Lambda (container image)
aws lambda create-function \
  --function-name ${PROJECT}-renderer \
  --package-type Image \
  --code ImageUri=$RENDERER_IMAGE \
  --role $ROLE_ARN \
  --memory-size 3008 \
  --timeout 900 \
  --environment "{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\"}}" \
  --region $REGION 2>/dev/null || \
aws lambda update-function-code \
  --function-name ${PROJECT}-renderer \
  --image-uri $RENDERER_IMAGE \
  --region $REGION

echo "=== Creating Step Functions State Machine ==="

# Read state machine definition and substitute ARNs
STATE_MACHINE=$(cat << EOF
{
  "Comment": "TXT-to-Video Pipeline",
  "StartAt": "ParseTXT",
  "States": {
    "ParseTXT": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-parser",
      "ResultPath": "$.parserOutput",
      "Retry": [{"ErrorEquals":["States.TaskFailed"],"IntervalSeconds":3,"MaxAttempts":2,"BackoffRate":2}],
      "Catch": [{"ErrorEquals":["States.ALL"],"ResultPath":"$.error","Next":"MarkFailed"}],
      "Next": "GenerateSlides"
    },
    "GenerateSlides": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-slidegen",
      "Parameters": {"projectId.$":"$.projectId","jsonKey.$":"$.parserOutput.s3Key","template.$":"$.template"},
      "ResultPath": "$.slideGenOutput",
      "Retry": [{"ErrorEquals":["States.TaskFailed"],"IntervalSeconds":5,"MaxAttempts":2,"BackoffRate":2}],
      "Catch": [{"ErrorEquals":["States.ALL"],"ResultPath":"$.error","Next":"MarkFailed"}],
      "Next": "Narrate"
    },
    "Narrate": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-narrator",
      "Parameters": {"projectId.$":"$.projectId","jsonKey.$":"$.parserOutput.s3Key","voice.$":"$.voice"},
      "ResultPath": "$.narratorOutput",
      "Retry": [{"ErrorEquals":["States.TaskFailed"],"IntervalSeconds":5,"MaxAttempts":2,"BackoffRate":2}],
      "Catch": [{"ErrorEquals":["States.ALL"],"ResultPath":"$.error","Next":"MarkFailed"}],
      "Next": "Render"
    },
    "Render": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-renderer",
      "Parameters": {"projectId.$":"$.projectId","slideKeys.$":"$.slideGenOutput.slideKeys","audioKeys.$":"$.narratorOutput.audioKeys","jsonKey.$":"$.parserOutput.s3Key"},
      "ResultPath": "$.rendererOutput",
      "Retry": [{"ErrorEquals":["States.TaskFailed"],"IntervalSeconds":10,"MaxAttempts":2,"BackoffRate":2}],
      "Catch": [{"ErrorEquals":["States.ALL"],"ResultPath":"$.error","Next":"MarkFailed"}],
      "Next": "MarkCompleted"
    },
    "MarkCompleted": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-statusupdater",
      "Parameters": {"projectId.$":"$.projectId","userId.$":"$.userId","status":"completed","videoKey.$":"$.rendererOutput.videoKey","thumbnailKey.$":"$.rendererOutput.thumbnailKey"},
      "End": true
    },
    "MarkFailed": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${PROJECT}-statusupdater",
      "Parameters": {"projectId.$":"$.projectId","userId.$":"$.userId","status":"failed","error.$":"$.error"},
      "End": true
    }
  }
}
EOF
)

SFN_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${PROJECT}-sfn-role"

aws stepfunctions create-state-machine \
  --name ${PROJECT}-pipeline \
  --definition "$STATE_MACHINE" \
  --role-arn $SFN_ROLE_ARN \
  --type STANDARD \
  --region $REGION 2>/dev/null || \
aws stepfunctions update-state-machine \
  --state-machine-arn "arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${PROJECT}-pipeline" \
  --definition "$STATE_MACHINE" \
  --role-arn $SFN_ROLE_ARN \
  --region $REGION

echo "=== Updating API Lambda with STATE_MACHINE_ARN ==="
SFN_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${PROJECT}-pipeline"
FULL_ENV="{\"Variables\":{\"S3_BUCKET\":\"${PROJECT}-assets\",\"DYNAMODB_TABLE\":\"${PROJECT}-projects\",\"USERS_TABLE\":\"${PROJECT}-users\",\"SESSION_TABLE\":\"${PROJECT}-sessions\",\"JWT_SECRET\":\"${JWT_SECRET}\",\"GOOGLE_CLIENT_ID\":\"${GOOGLE_CLIENT_ID}\",\"GOOGLE_CLIENT_SECRET\":\"${GOOGLE_CLIENT_SECRET}\",\"GOOGLE_REDIRECT_URI\":\"https://indifferent.fun/auth/callback\",\"STATE_MACHINE_ARN\":\"${SFN_ARN}\"}}"

aws lambda update-function-configuration \
  --function-name ${PROJECT}-api \
  --environment "$FULL_ENV" \
  --region $REGION

echo "=== Done! All AWS resources created ==="
echo "API Lambda: ${PROJECT}-api"
echo "State Machine: ${SFN_ARN}"
echo "S3 Bucket: ${PROJECT}-assets"
