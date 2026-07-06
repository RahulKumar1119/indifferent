#!/bin/bash
set -euo pipefail

# API Gateway setup for indifferent-fun-api Lambda
# This script creates/recreates the API Gateway REST API with Lambda proxy integration.
#
# Current API ID: mr4uyoffe5
# Invoke URL: https://mr4uyoffe5.execute-api.ap-south-1.amazonaws.com/prod

REGION="ap-south-1"
ACCOUNT_ID="438097524343"
LAMBDA_FUNCTION="indifferent-fun-api"

echo "=== Creating API Gateway REST API ==="

API_ID=$(aws apigateway create-rest-api \
  --name "$LAMBDA_FUNCTION" \
  --description "TXT-to-Video SaaS API" \
  --endpoint-configuration "types=REGIONAL" \
  --region $REGION \
  --query 'id' --output text)

echo "API ID: $API_ID"

echo "=== Getting root resource ID ==="

ROOT_ID=$(aws apigateway get-resources \
  --rest-api-id "$API_ID" \
  --region $REGION \
  --query 'items[?path==`/`].id' --output text)

echo "Root resource ID: $ROOT_ID"

echo "=== Creating {proxy+} resource ==="

PROXY_ID=$(aws apigateway create-resource \
  --rest-api-id "$API_ID" \
  --parent-id "$ROOT_ID" \
  --path-part "{proxy+}" \
  --region $REGION \
  --query 'id' --output text)

echo "Proxy resource ID: $PROXY_ID"

echo "=== Creating ANY method on proxy resource ==="

aws apigateway put-method \
  --rest-api-id "$API_ID" \
  --resource-id "$PROXY_ID" \
  --http-method ANY \
  --authorization-type NONE \
  --region $REGION > /dev/null

LAMBDA_URI="arn:aws:apigateway:${REGION}:lambda:path/2015-03-31/functions/arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${LAMBDA_FUNCTION}/invocations"

echo "=== Setting up Lambda proxy integration on proxy resource ==="

aws apigateway put-integration \
  --rest-api-id "$API_ID" \
  --resource-id "$PROXY_ID" \
  --http-method ANY \
  --type AWS_PROXY \
  --integration-http-method POST \
  --uri "$LAMBDA_URI" \
  --region $REGION > /dev/null

echo "=== Creating ANY method on root resource ==="

aws apigateway put-method \
  --rest-api-id "$API_ID" \
  --resource-id "$ROOT_ID" \
  --http-method ANY \
  --authorization-type NONE \
  --region $REGION > /dev/null

aws apigateway put-integration \
  --rest-api-id "$API_ID" \
  --resource-id "$ROOT_ID" \
  --http-method ANY \
  --type AWS_PROXY \
  --integration-http-method POST \
  --uri "$LAMBDA_URI" \
  --region $REGION > /dev/null

echo "=== Granting API Gateway permission to invoke Lambda ==="

aws lambda add-permission \
  --function-name "$LAMBDA_FUNCTION" \
  --statement-id "apigateway-invoke-${API_ID}" \
  --action lambda:InvokeFunction \
  --principal apigateway.amazonaws.com \
  --source-arn "arn:aws:execute-api:${REGION}:${ACCOUNT_ID}:${API_ID}/*" \
  --region $REGION > /dev/null 2>&1 || echo "Permission already exists"

echo "=== Initial deployment to prod stage ==="

aws apigateway create-deployment \
  --rest-api-id "$API_ID" \
  --stage-name prod \
  --region $REGION > /dev/null

echo "=== Setting up CORS (OPTIONS method on proxy resource) ==="

aws apigateway put-method \
  --rest-api-id "$API_ID" \
  --resource-id "$PROXY_ID" \
  --http-method OPTIONS \
  --authorization-type NONE \
  --region $REGION > /dev/null

aws apigateway put-integration \
  --rest-api-id "$API_ID" \
  --resource-id "$PROXY_ID" \
  --http-method OPTIONS \
  --type MOCK \
  --request-templates '{"application/json":"{\"statusCode\":200}"}' \
  --region $REGION > /dev/null

aws apigateway put-method-response \
  --rest-api-id "$API_ID" \
  --resource-id "$PROXY_ID" \
  --http-method OPTIONS \
  --status-code 200 \
  --response-parameters '{"method.response.header.Access-Control-Allow-Headers":true,"method.response.header.Access-Control-Allow-Methods":true,"method.response.header.Access-Control-Allow-Origin":true,"method.response.header.Access-Control-Allow-Credentials":true}' \
  --response-models '{"application/json":"Empty"}' \
  --region $REGION > /dev/null

aws apigateway put-integration-response \
  --rest-api-id "$API_ID" \
  --resource-id "$PROXY_ID" \
  --http-method OPTIONS \
  --status-code 200 \
  --response-parameters '{"method.response.header.Access-Control-Allow-Headers":"'"'"'Content-Type,Authorization'"'"'","method.response.header.Access-Control-Allow-Methods":"'"'"'GET,POST,DELETE,OPTIONS'"'"'","method.response.header.Access-Control-Allow-Origin":"'"'"'https://indifferent.fun'"'"'","method.response.header.Access-Control-Allow-Credentials":"'"'"'true'"'"'"}' \
  --region $REGION > /dev/null

echo "=== Redeploying after CORS changes ==="

aws apigateway create-deployment \
  --rest-api-id "$API_ID" \
  --stage-name prod \
  --region $REGION > /dev/null

echo "=== Done! ==="
echo "API Gateway ID: $API_ID"
echo "Invoke URL: https://${API_ID}.execute-api.${REGION}.amazonaws.com/prod"
