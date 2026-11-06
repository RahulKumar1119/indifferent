#!/bin/bash
# Edge protection for the API (SEC-01): throttling, a WAF with per-IP rate
# limits, reserved Lambda concurrency and access logs.
#
# The API is a single {proxy+} ANY method, so API Gateway cannot throttle one
# path (such as /auth/login) on its own. Stage throttling is therefore one
# overall ceiling; the per-path and per-IP limits live in the WAF.
#
# What it does (each step is skipped if already in place):
#   1. Stage throttle      default rate/burst for the prod stage
#   2. Reserved concurrency  on indifferent-fun-api, so a flood of API calls
#                          cannot use up the 50-slot account pool that the
#                          render, parser and narrator functions share
#   3. WAF web ACL         indifferent-fun-api-waf, attached to the prod stage:
#                            - auth-per-ip     blocks an IP above 150 requests
#                                              per 5 min to /auth/*
#                            - all-per-ip      blocks an IP above 3000 requests
#                                              per 5 min to anything
#                            - AWS managed Common and IP reputation groups,
#                              in COUNT mode first (see --managed-mode)
#   4. Access logs         JSON lines in /aws/apigateway/indifferent-fun-api-access
#
# Costs: a WAF ACL is about $5 a month, plus about $1 for each rule or managed
# group (4 here) and $0.60 per million requests inspected.
#
# This touches live AWS. It prints the plan and changes nothing unless you
# pass --apply.
#
# Usage:
#   bash deploy/setup-edge-protection.sh                      # show the plan
#   bash deploy/setup-edge-protection.sh --apply
#   bash deploy/setup-edge-protection.sh --apply --managed-mode block
#
# Options:
#   --apply               make the changes
#   --managed-mode MODE   count (default) or block, for the two AWS managed
#                         groups. Run in count, look at the WAF metrics
#                         (AllowedRequests / CountedRequests per rule) for a
#                         few days, then re-run with block on a new ACL.
#   --rate N --burst N    stage throttle (default 100 / 200 requests a second)
#   --reserve N           reserved concurrency for the API Lambda (default 20;
#                         at most 40, because AWS keeps 10 unreserved)
#
# Undo:
#   aws wafv2 disassociate-web-acl --resource-arn <stage arn> --region ap-south-1
#   aws lambda delete-function-concurrency --function-name indifferent-fun-api --region ap-south-1
#   aws apigateway update-stage --rest-api-id mr4uyoffe5 --stage-name prod --region ap-south-1 \
#     --patch-operations op=remove,path=/*/*/throttling/rateLimit op=remove,path=/*/*/throttling/burstLimit \
#     op=remove,path=/accessLogSettings/destinationArn op=remove,path=/accessLogSettings/format
#   (and wafv2 delete-web-acl for the ACL, which needs its id and lock token)

set -euo pipefail

REGION="ap-south-1"
PROJECT="indifferent-fun"
API_ID="mr4uyoffe5"
STAGE="prod"
FUNCTION="${PROJECT}-api"
ACL_NAME="${PROJECT}-api-waf"
LOG_GROUP="/aws/apigateway/${PROJECT}-api-access"

APPLY=false
MANAGED_MODE=count
RATE=100
BURST=200
RESERVE=20

fail() { echo "ERROR: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --apply) APPLY=true ;;
    --managed-mode) shift; MANAGED_MODE="${1:-}" ;;
    --rate) shift; RATE="${1:-}" ;;
    --burst) shift; BURST="${1:-}" ;;
    --reserve) shift; RESERVE="${1:-}" ;;
    *) fail "unknown argument: $1" ;;
  esac
  shift
done

case "$MANAGED_MODE" in count | block) ;; *) fail "--managed-mode must be count or block" ;; esac
for v in "$RATE" "$BURST" "$RESERVE"; do
  case "$v" in '' | *[!0-9]*) fail "--rate, --burst and --reserve must be whole numbers" ;; esac
done
[ "$RESERVE" -le 40 ] || fail "--reserve must be 40 or less (the account has 50 slots and AWS keeps 10 unreserved)"
[ "$RESERVE" -ge 1 ] || fail "--reserve must be at least 1"

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
STAGE_ARN="arn:aws:apigateway:${REGION}::/restapis/${API_ID}/stages/${STAGE}"
LOG_GROUP_ARN="arn:aws:logs:${REGION}:${ACCOUNT_ID}:log-group:${LOG_GROUP}"

if [ "$MANAGED_MODE" = block ]; then
  OVERRIDE='{"None":{}}'
else
  OVERRIDE='{"Count":{}}'
fi

echo "Account ${ACCOUNT_ID}, region ${REGION}, API ${API_ID} stage ${STAGE}"
echo "Plan:"
echo "  1. stage throttle ${RATE} req/s, burst ${BURST}"
echo "  2. reserved concurrency ${RESERVE} on ${FUNCTION}"
echo "  3. WAF ${ACL_NAME} (rate rules: block; managed groups: ${MANAGED_MODE}) attached to the stage"
echo "  4. access logs to ${LOG_GROUP}"
if [ "$APPLY" != true ]; then
  echo
  echo "Dry run. Nothing was changed. Run again with --apply to make these changes."
  exit 0
fi
echo

# --- 2. reserved concurrency (before the throttle so a failure here stops early)
echo "--- Reserved concurrency"
CURRENT=$(aws lambda get-function-concurrency --function-name "$FUNCTION" --region "$REGION" \
  --query ReservedConcurrentExecutions --output text 2>/dev/null || true)
if [ "$CURRENT" = "$RESERVE" ]; then
  echo "already ${RESERVE}"
else
  # AWS refuses a reservation that leaves fewer than 50 slots unreserved. With
  # the account limit at 50 that means none can be reserved until the limit is
  # raised (Service Quotas, Lambda "Concurrent executions"). That must not stop
  # the throttle, WAF and logs below, so it is a warning.
  if ! aws lambda put-function-concurrency --function-name "$FUNCTION" \
    --reserved-concurrent-executions "$RESERVE" --region "$REGION" \
    --query ReservedConcurrentExecutions --output text; then
    echo "WARNING: reserved concurrency was not set (see the message above). Raise the account's Lambda concurrency limit, then run this again." >&2
  fi
fi

# --- 4 (log group first, the stage patch below needs it)
echo "--- Access log group"
aws logs create-log-group --log-group-name "$LOG_GROUP" --region "$REGION" 2>/dev/null || echo "log group exists"
aws logs put-retention-policy --log-group-name "$LOG_GROUP" --retention-in-days 30 --region "$REGION"

# --- 1 + 4. one stage update: throttle and access log settings
echo "--- Stage throttle and access logs"
# The format has no request or response bodies and no headers: only who (IP),
# when, which route, and the outcome.
LOG_FORMAT='{"requestId":"$context.requestId","ip":"$context.identity.sourceIp","time":"$context.requestTime","method":"$context.httpMethod","path":"$context.path","status":"$context.status","bytes":"$context.responseLength","latencyMs":"$context.responseLatency","ua":"$context.identity.userAgent"}'
# JSON, not the shorthand form: the log format contains commas.
PATCH=$(RATE="$RATE" BURST="$BURST" ARN="$LOG_GROUP_ARN" FMT="$LOG_FORMAT" python3 -c '
import json, os
print(json.dumps([
  {"op": "replace", "path": "/*/*/throttling/rateLimit", "value": os.environ["RATE"]},
  {"op": "replace", "path": "/*/*/throttling/burstLimit", "value": os.environ["BURST"]},
  {"op": "replace", "path": "/accessLogSettings/destinationArn", "value": os.environ["ARN"]},
  {"op": "replace", "path": "/accessLogSettings/format", "value": os.environ["FMT"]},
]))')
aws apigateway update-stage --rest-api-id "$API_ID" --stage-name "$STAGE" --region "$REGION" \
  --patch-operations "$PATCH" --query 'accessLogSettings.destinationArn' --output text

# --- 3. WAF
echo "--- WAF web ACL"
ACL_ID=$(aws wafv2 list-web-acls --scope REGIONAL --region "$REGION" \
  --query "WebACLs[?Name=='${ACL_NAME}'].Id" --output text)
if [ -n "$ACL_ID" ] && [ "$ACL_ID" != None ]; then
  echo "${ACL_NAME} already exists (${ACL_ID}); its rules were not changed."
else
  # SearchString is base64 in the CLI JSON: L2F1dGgv is "/auth/".
  RULES=$(cat <<EOF
[
  {
    "Name": "auth-per-ip",
    "Priority": 0,
    "Action": {"Block": {"CustomResponse": {"ResponseCode": 429}}},
    "VisibilityConfig": {"SampledRequestsEnabled": true, "CloudWatchMetricsEnabled": true, "MetricName": "auth-per-ip"},
    "Statement": {"RateBasedStatement": {
      "Limit": 150, "EvaluationWindowSec": 300, "AggregateKeyType": "IP",
      "ScopeDownStatement": {"ByteMatchStatement": {
        "SearchString": "L2F1dGgv", "FieldToMatch": {"UriPath": {}},
        "TextTransformations": [{"Priority": 0, "Type": "LOWERCASE"}],
        "PositionalConstraint": "STARTS_WITH"}}}}
  },
  {
    "Name": "all-per-ip",
    "Priority": 1,
    "Action": {"Block": {"CustomResponse": {"ResponseCode": 429}}},
    "VisibilityConfig": {"SampledRequestsEnabled": true, "CloudWatchMetricsEnabled": true, "MetricName": "all-per-ip"},
    "Statement": {"RateBasedStatement": {"Limit": 3000, "EvaluationWindowSec": 300, "AggregateKeyType": "IP"}}
  },
  {
    "Name": "aws-ip-reputation",
    "Priority": 2,
    "OverrideAction": ${OVERRIDE},
    "VisibilityConfig": {"SampledRequestsEnabled": true, "CloudWatchMetricsEnabled": true, "MetricName": "aws-ip-reputation"},
    "Statement": {"ManagedRuleGroupStatement": {"VendorName": "AWS", "Name": "AWSManagedRulesAmazonIpReputationList"}}
  },
  {
    "Name": "aws-common",
    "Priority": 3,
    "OverrideAction": ${OVERRIDE},
    "VisibilityConfig": {"SampledRequestsEnabled": true, "CloudWatchMetricsEnabled": true, "MetricName": "aws-common"},
    "Statement": {"ManagedRuleGroupStatement": {"VendorName": "AWS", "Name": "AWSManagedRulesCommonRuleSet",
      "RuleActionOverrides": [{"Name": "SizeRestrictions_BODY", "ActionToUse": {"Count": {}}}]}}
  }
]
EOF
)
  ACL_ARN=$(aws wafv2 create-web-acl --name "$ACL_NAME" --scope REGIONAL --region "$REGION" \
    --default-action '{"Allow":{}}' \
    --visibility-config "SampledRequestsEnabled=true,CloudWatchMetricsEnabled=true,MetricName=${ACL_NAME}" \
    --rules "$RULES" \
    --query Summary.ARN --output text)
  echo "created ${ACL_ARN}"
fi

ACL_ARN=$(aws wafv2 list-web-acls --scope REGIONAL --region "$REGION" \
  --query "WebACLs[?Name=='${ACL_NAME}'].ARN" --output text)
ATTACHED=$(aws wafv2 get-web-acl-for-resource --resource-arn "$STAGE_ARN" --region "$REGION" \
  --query WebACL.ARN --output text 2>/dev/null || true)
if [ "$ATTACHED" = "$ACL_ARN" ]; then
  echo "already attached to ${STAGE}"
else
  aws wafv2 associate-web-acl --web-acl-arn "$ACL_ARN" --resource-arn "$STAGE_ARN" --region "$REGION"
  echo "attached to ${STAGE}"
fi

echo
echo "Done. Check:"
echo "  aws apigateway get-stage --rest-api-id ${API_ID} --stage-name ${STAGE} --region ${REGION} --query '{m:methodSettings,w:webAclArn,a:accessLogSettings}'"
echo "  aws lambda get-function-concurrency --function-name ${FUNCTION} --region ${REGION}"
