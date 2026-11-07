#!/bin/bash
# Data protection and alerting (SEC-13).
#
# What it does (each step is skipped when it is already in place, except the
# alarms, which are simply put again: that is harmless and applies new thresholds):
#
#   1. Deletion protection on the users, projects and sessions tables, so that
#      a stray delete-table (or a wrong terraform/cdk run) fails instead of
#      destroying the data. It is free. Deleting a table on purpose then needs
#      one extra command first (see Undo).
#   2. Point-in-time recovery on the users and projects tables: restore to any
#      second in the last 35 days after a bad write or an accidental delete of
#      items. The tables are tiny (about 4 KB and 140 KB today), so this costs
#      pennies a month. It is NOT turned on for the sessions table, on purpose:
#      a restore would bring back sessions that were revoked or rotated since
#      (including stolen ones that reuse detection had deleted), which is the
#      opposite of what you want after an incident. Sessions are short lived and
#      users just sign in again. The stock-cache table is disposable.
#   3. A bucket policy on the assets bucket that refuses any request not made
#      over TLS (aws:SecureTransport). Everything the app does already uses
#      https (the SDKs, presigned URLs, the accelerate endpoint), so this is
#      defence in depth against a client or a script that would fall back to
#      plain http. It changes no existing access. If the bucket already has a
#      policy the script refuses to replace it.
#   4. CloudWatch alarms on the API, e-mailing the existing alerts topic
#      (indifferent-fun-email-alerts, the one the SES alarms use):
#        - indifferent-fun-api-5xx          5 or more server errors in 5 minutes
#        - indifferent-fun-api-4xx-spike    500 or more client errors in 5 minutes
#                                           (a scanner, a credential-stuffing run,
#                                           or a broken release)
#        - indifferent-fun-api-throttled    any Lambda throttle in 5 minutes (the
#                                           account is capped at 50 concurrent runs)
#        - indifferent-fun-api-waf-blocked  100 or more requests blocked by the WAF
#                                           in 5 minutes
#      Four alarms cost about $0.40 a month. Missing data counts as fine, so a
#      quiet period does not alarm.
#
# NOT done here, on purpose:
#   - S3 server access logging: needs a second bucket and its own lifecycle, and
#     CloudTrail data events cost money per request. Say if you want it.
#   - API Gateway tracing (X-Ray): access logs already exist (setup-edge-protection.sh).
#   - Expiring old object versions: done by setup-version-cleanup.sh.
#
# This touches live AWS. Without --apply it only reads, and prints what it would do.
#
# Usage:
#   bash deploy/setup-data-protection.sh            # show the plan
#   bash deploy/setup-data-protection.sh --apply
#
# Undo:
#   aws dynamodb update-table --table-name indifferent-fun-users --no-deletion-protection-enabled --region ap-south-1
#   aws dynamodb update-continuous-backups --table-name indifferent-fun-users --point-in-time-recovery-specification PointInTimeRecoveryEnabled=false --region ap-south-1
#   aws s3api delete-bucket-policy --bucket indifferent-fun-assets --region ap-south-1
#   aws cloudwatch delete-alarms --region ap-south-1 --alarm-names indifferent-fun-api-5xx indifferent-fun-api-4xx-spike indifferent-fun-api-throttled indifferent-fun-api-waf-blocked

set -euo pipefail

REGION="ap-south-1"
PROJECT="indifferent-fun"
BUCKET="${PROJECT}-assets"
API_ID="mr4uyoffe5"
API_NAME="${PROJECT}-api"
STAGE="prod"
WAF_ACL="${PROJECT}-api-waf"
FUNCTION="${PROJECT}-api"
TOPIC_NAME="${PROJECT}-email-alerts"
PROTECTED_TABLES=("${PROJECT}-users" "${PROJECT}-projects" "${PROJECT}-sessions")
PITR_TABLES=("${PROJECT}-users" "${PROJECT}-projects")
POLICY_SID="DenyInsecureTransport"

fail() { echo "ERROR: $*" >&2; exit 1; }

APPLY=false
case "${1:-}" in
  --apply) APPLY=true ;;
  "") ;;
  *) fail "unknown argument: $1 (use --apply or no argument)" ;;
esac

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
TOPIC_ARN="arn:aws:sns:${REGION}:${ACCOUNT_ID}:${TOPIC_NAME}"
TMP=$(mktemp -d)
chmod 700 "$TMP"
trap 'rm -rf "$TMP"' EXIT

if $APPLY; then
  echo "MODE: apply (changes will be made in account ${ACCOUNT_ID}, ${REGION})"
else
  echo "MODE: dry run (read only; nothing will be created or changed)"
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

# --- 0. preflight (read only) ------------------------------------------------
echo "=== 0. Preflight"
for t in "${PROTECTED_TABLES[@]}"; do
  S=$(aws dynamodb describe-table --table-name "$t" --region "$REGION" --query Table.TableStatus --output text 2>/dev/null || true)
  [ "$S" = "ACTIVE" ] || fail "table ${t} is not active (${S:-not found})"
done
echo "tables: ${PROTECTED_TABLES[*]} are active"
aws s3api head-bucket --bucket "$BUCKET" --region "$REGION" >/dev/null 2>&1 || fail "bucket ${BUCKET} not found or not reachable"
echo "bucket: ${BUCKET} reachable"
aws sns get-topic-attributes --topic-arn "$TOPIC_ARN" --region "$REGION" --query Attributes.TopicArn --output text >/dev/null 2>&1 ||
  fail "SNS topic ${TOPIC_NAME} not found. Run deploy/setup-notifications.sh first (it creates the alerts topic)."
echo "alerts topic: ${TOPIC_NAME} exists"
NAME=$(aws apigateway get-rest-api --rest-api-id "$API_ID" --region "$REGION" --query name --output text 2>/dev/null || true)
[ "$NAME" = "$API_NAME" ] || fail "REST API ${API_ID} is named '${NAME:-?}', expected ${API_NAME}; the alarms would watch the wrong API"
echo "api: ${API_ID} is ${API_NAME}"
echo

# --- 1. deletion protection --------------------------------------------------
echo "=== 1. DynamoDB deletion protection"
for t in "${PROTECTED_TABLES[@]}"; do
  CUR=$(aws dynamodb describe-table --table-name "$t" --region "$REGION" --query Table.DeletionProtectionEnabled --output text)
  if [ "$CUR" = "True" ]; then
    echo "${t}: already on"
  else
    act "turn deletion protection on for ${t}" \
      aws dynamodb update-table --table-name "$t" --deletion-protection-enabled --region "$REGION" --query Table.DeletionProtectionEnabled --output text
  fi
done
echo

# --- 2. point-in-time recovery -----------------------------------------------
echo "=== 2. DynamoDB point-in-time recovery (users, projects)"
for t in "${PITR_TABLES[@]}"; do
  CUR=$(aws dynamodb describe-continuous-backups --table-name "$t" --region "$REGION" --query ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus --output text)
  if [ "$CUR" = "ENABLED" ]; then
    echo "${t}: already enabled"
  else
    act "enable point-in-time recovery for ${t}" \
      aws dynamodb update-continuous-backups --table-name "$t" --point-in-time-recovery-specification PointInTimeRecoveryEnabled=true \
      --region "$REGION" --query ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus --output text
  fi
done
echo

# --- 3. TLS-only bucket policy -----------------------------------------------
echo "=== 3. TLS-only policy on ${BUCKET}"
cat >"$TMP/policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "${POLICY_SID}",
      "Effect": "Deny",
      "Principal": "*",
      "Action": "s3:*",
      "Resource": ["arn:aws:s3:::${BUCKET}", "arn:aws:s3:::${BUCKET}/*"],
      "Condition": {"Bool": {"aws:SecureTransport": "false"}}
    }
  ]
}
EOF
# Read-only policy linter. It needs the access-analyzer:ValidatePolicy
# permission; if that is missing this is only a warning.
FINDINGS=$(aws accessanalyzer validate-policy --policy-type RESOURCE_POLICY --validate-policy-resource-type AWS::S3::Bucket \
  --policy-document "file://${TMP}/policy.json" --region "$REGION" --query 'findings[?findingType==`ERROR` || findingType==`SECURITY_WARNING`].[findingType,issueCode]' --output text 2>"$TMP/analyzer.err" || true)
if [ -s "$TMP/analyzer.err" ]; then
  echo "note: could not run the policy linter ($(head -c 120 "$TMP/analyzer.err" | tr '\n' ' '))"
elif [ -n "$FINDINGS" ]; then
  fail "the policy linter reports: ${FINDINGS}"
else
  echo "policy linter: no errors or security warnings"
fi
EXISTING=$(aws s3api get-bucket-policy --bucket "$BUCKET" --region "$REGION" --query Policy --output text 2>"$TMP/policy.err" || true)
if [ -z "$EXISTING" ] || [ "$EXISTING" = "None" ]; then
  grep -q NoSuchBucketPolicy "$TMP/policy.err" || fail "could not read the bucket policy: $(head -c 200 "$TMP/policy.err")"
  act "put the TLS-only policy on ${BUCKET}" \
    aws s3api put-bucket-policy --bucket "$BUCKET" --region "$REGION" --policy "file://${TMP}/policy.json"
elif printf '%s' "$EXISTING" | grep -q "$POLICY_SID"; then
  echo "${BUCKET}: policy with ${POLICY_SID} already in place"
else
  fail "${BUCKET} already has a bucket policy without ${POLICY_SID}. Not replacing it. Add this statement to it by hand:
$(sed -n '/"Sid"/,/"Condition"/p' "$TMP/policy.json")"
fi
echo

# --- 4. alarms ---------------------------------------------------------------
echo "=== 4. CloudWatch alarms (to ${TOPIC_NAME})"
put_alarm() { # name namespace metric statistic threshold description dimension...
  local name="$1" namespace="$2" metric="$3" statistic="$4" threshold="$5" description="$6"
  shift 6
  act "put alarm ${name} (${metric} ${statistic} >= ${threshold} in 5 minutes)" \
    aws cloudwatch put-metric-alarm --alarm-name "$name" --alarm-description "$description" \
    --namespace "$namespace" --metric-name "$metric" --dimensions "$@" \
    --statistic "$statistic" --period 300 --evaluation-periods 1 \
    --threshold "$threshold" --comparison-operator GreaterThanOrEqualToThreshold \
    --treat-missing-data notBreaching --alarm-actions "$TOPIC_ARN" --region "$REGION"
}
put_alarm "${PROJECT}-api-5xx" AWS/ApiGateway 5XXError Sum 5 \
  "The API returned 5 or more server errors in 5 minutes." \
  "Name=ApiName,Value=${API_NAME}" "Name=Stage,Value=${STAGE}"
put_alarm "${PROJECT}-api-4xx-spike" AWS/ApiGateway 4XXError Sum 500 \
  "The API returned 500 or more client errors in 5 minutes: a scanner, a credential-stuffing run, or a broken release." \
  "Name=ApiName,Value=${API_NAME}" "Name=Stage,Value=${STAGE}"
put_alarm "${PROJECT}-api-throttled" AWS/Lambda Throttles Sum 1 \
  "The API Lambda was throttled. The account allows 50 concurrent runs for all functions." \
  "Name=FunctionName,Value=${FUNCTION}"
put_alarm "${PROJECT}-api-waf-blocked" AWS/WAFV2 BlockedRequests Sum 100 \
  "The WAF blocked 100 or more requests in 5 minutes: someone is hitting a rate limit." \
  "Name=WebACL,Value=${WAF_ACL}" "Name=Region,Value=${REGION}" "Name=Rule,Value=ALL"
echo

# --- result ------------------------------------------------------------------
if $APPLY; then
  echo "=== Result"
  for t in "${PROTECTED_TABLES[@]}"; do
    echo "${t}: deletion protection $(aws dynamodb describe-table --table-name "$t" --region "$REGION" --query Table.DeletionProtectionEnabled --output text)"
  done
  for t in "${PITR_TABLES[@]}"; do
    echo "${t}: point-in-time recovery $(aws dynamodb describe-continuous-backups --table-name "$t" --region "$REGION" --query ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus --output text)"
  done
  echo "${BUCKET}: policy statements: $(aws s3api get-bucket-policy --bucket "$BUCKET" --region "$REGION" --query Policy --output text | python3 -c 'import sys,json; print(", ".join(s["Sid"] for s in json.load(sys.stdin)["Statement"]))')"
  aws cloudwatch describe-alarms --region "$REGION" --alarm-name-prefix "${PROJECT}-api-" --query 'MetricAlarms[].[AlarmName,StateValue,length(AlarmActions)]' --output text
  echo
  echo "Check the TLS rule: a presigned https request works, the same one over http is refused:"
  echo "  U=\$(aws s3 presign s3://${BUCKET}/<any key> --region ${REGION}); curl -s -o /dev/null -w '%{http_code}\\n' \"\$U\"; curl -s -o /dev/null -w '%{http_code}\\n' \"\${U/https:/http:}\""
else
  echo "Dry run. Nothing was changed. Run again with --apply to make these changes."
fi
