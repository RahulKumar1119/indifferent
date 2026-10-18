#!/bin/bash
set -euo pipefail

# Job notification email infrastructure (issue #100, Part B)
# ===========================================================
#
# Wires up "email the user when a quiz video or AI Shorts job finishes or
# fails". Creates or updates, in this order:
#
#   1. SES configuration set  indifferent-notifications  (reputation metrics on)
#   2. SNS topic              indifferent-fun-email-alerts, with an email
#                             subscription to support@indifferent.fun
#                             (forwarded by ImprovMX; confirm it once)
#   3. CloudWatch alarms      SES bounce rate, SES complaint rate, notifier errors
#   4. IAM role               indifferent-fun-notifier-role (least privilege)
#   5. Lambda                 indifferent-fun-notifier (built from ./backend/cmd/notifier)
#   6. Step Functions         adds NotifyCompleted / NotifyFailed / NotifyEnd
#                             after MarkCompleted / MarkFailed in both pipelines
#
# Prerequisite: the SES identity for indifferent.fun exists and is verified
# (deploy/setup-ses-identity.sh) and SES is out of the sandbox.
#
# Usage:
#   bash deploy/setup-notifications.sh            # dry run: read-only calls, prints the plan
#   bash deploy/setup-notifications.sh --apply    # makes the changes
#
# Safety:
#   - Idempotent: every step checks current state first; re-running is safe.
#   - The dry run only makes read calls (describe/get/list/validate) and builds
#     the Lambda locally. It creates and changes nothing in AWS.
#   - State machines are patched from their LIVE definitions, not rewritten from
#     the repo, so nothing else about them changes. The previous definitions
#     are saved to /tmp/<state-machine>-before-<timestamp>.json first.
#     deploy/setup.sh and deploy/setup-shorts.sh contain the same states, so
#     re-running them keeps the wiring.
#   - Running executions finish on the definition they started with (no email);
#     executions started after the update send one.
#   - The notify states sit AFTER the final status update and catch every error,
#     so a mail problem can never change a job's outcome.
#
# Undo (see also the saved definitions):
#   aws stepfunctions update-state-machine --state-machine-arn <arn> \
#       --definition file:///tmp/<state-machine>-before-<timestamp>.json
#   aws lambda delete-function --function-name indifferent-fun-notifier
#   aws iam delete-role-policy --role-name indifferent-fun-notifier-role --policy-name indifferent-fun-notifier-permissions
#   aws iam detach-role-policy --role-name indifferent-fun-notifier-role --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
#   aws iam delete-role --role-name indifferent-fun-notifier-role
#   aws cloudwatch delete-alarms --alarm-names indifferent-fun-ses-bounce-rate indifferent-fun-ses-complaint-rate indifferent-fun-notifier-errors
#   aws sns delete-topic --topic-arn <topic arn>
#   aws sesv2 delete-configuration-set --configuration-set-name indifferent-notifications

REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

DOMAIN="indifferent.fun"
FROM_ADDRESS="Indifferent <notifications@${DOMAIN}>"
REPLY_TO="support@${DOMAIN}"
ALERT_EMAIL="support@${DOMAIN}"
APP_BASE_URL="https://${DOMAIN}"

CONFIG_SET="indifferent-notifications"
FUNCTION="${PROJECT}-notifier"
ROLE_NAME="${PROJECT}-notifier-role"
POLICY_NAME="${PROJECT}-notifier-permissions"
TOPIC_NAME="${PROJECT}-email-alerts"
PROJECTS_TABLE="${PROJECT}-projects"
USERS_TABLE="${PROJECT}-users"
SFN_ROLE_NAME="${PROJECT}-sfn-role"
STATE_MACHINES=("${PROJECT}-pipeline" "${PROJECT}-shorts-pipeline")

# Alarm thresholds. SES publishes these rates as fractions (0.05 = 5%).
# AWS recommends staying under 5% bounces and 0.1% complaints and reviews
# accounts above 10% and 0.5%; these fire well before that.
BOUNCE_THRESHOLD="0.02"
COMPLAINT_THRESHOLD="0.0005"
ERROR_THRESHOLD="5"

IDENTITY_ARN="arn:aws:ses:${REGION}:${ACCOUNT_ID}:identity/${DOMAIN}"
CONFIG_SET_ARN="arn:aws:ses:${REGION}:${ACCOUNT_ID}:configuration-set/${CONFIG_SET}"
TOPIC_ARN="arn:aws:sns:${REGION}:${ACCOUNT_ID}:${TOPIC_NAME}"
ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}"
FUNCTION_ARN="arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${FUNCTION}"

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
echo "=== 0. Preflight: SES identity ${DOMAIN} ==="
VERIFIED=$(aws sesv2 get-email-identity --email-identity "$DOMAIN" --region "$REGION" \
  --query 'VerifiedForSendingStatus' --output text 2>/dev/null || echo "missing")
if [ "$VERIFIED" != "True" ]; then
  echo "ERROR: ${DOMAIN} is not a verified SES identity in ${REGION} (status: ${VERIFIED})." >&2
  echo "Run deploy/setup-ses-identity.sh first." >&2
  exit 1
fi
echo "Verified for sending."

# ---------------------------------------------------------------------------
echo
echo "=== 1. SES configuration set ${CONFIG_SET} ==="
if aws sesv2 get-configuration-set --configuration-set-name "$CONFIG_SET" --region "$REGION" >/dev/null 2>&1; then
  REP=$(aws sesv2 get-configuration-set --configuration-set-name "$CONFIG_SET" --region "$REGION" \
    --query 'ReputationOptions.ReputationMetricsEnabled' --output text)
  if [ "$REP" = "True" ]; then
    echo "Exists, reputation metrics already on."
  else
    act "turn reputation metrics on for ${CONFIG_SET}" \
      aws sesv2 put-configuration-set-reputation-options \
      --configuration-set-name "$CONFIG_SET" --reputation-metrics-enabled --region "$REGION"
  fi
else
  act "create configuration set ${CONFIG_SET} (reputation metrics on, bounces and complaints suppressed)" \
    aws sesv2 create-configuration-set \
    --configuration-set-name "$CONFIG_SET" \
    --reputation-options ReputationMetricsEnabled=true \
    --sending-options SendingEnabled=true \
    --suppression-options SuppressedReasons=BOUNCE,COMPLAINT \
    --region "$REGION"
fi

# ---------------------------------------------------------------------------
echo
echo "=== 2. SNS topic ${TOPIC_NAME} and email subscription ==="
if aws sns get-topic-attributes --topic-arn "$TOPIC_ARN" --region "$REGION" >/dev/null 2>&1; then
  echo "Topic exists."
else
  act "create SNS topic ${TOPIC_NAME}" \
    aws sns create-topic --name "$TOPIC_NAME" --region "$REGION" --output text
fi
SUBSCRIPTION=$(aws sns list-subscriptions-by-topic --topic-arn "$TOPIC_ARN" --region "$REGION" \
  --query "Subscriptions[?Endpoint=='${ALERT_EMAIL}'].SubscriptionArn | [0]" --output text 2>/dev/null || true)
if [ -n "$SUBSCRIPTION" ] && [ "$SUBSCRIPTION" != "None" ]; then
  echo "Subscription for ${ALERT_EMAIL} exists (${SUBSCRIPTION##*:})."
else
  act "subscribe ${ALERT_EMAIL} to the topic (a confirmation email arrives; click the link once)" \
    aws sns subscribe --topic-arn "$TOPIC_ARN" --protocol email \
    --notification-endpoint "$ALERT_EMAIL" --region "$REGION" --output text
fi

# ---------------------------------------------------------------------------
echo
echo "=== 3. CloudWatch alarms ==="
put_alarm() {
  local name="$1" namespace="$2" metric="$3" statistic="$4" threshold="$5" description="$6"
  shift 6
  act "put alarm ${name} (${metric} >= ${threshold})" \
    aws cloudwatch put-metric-alarm \
    --alarm-name "$name" \
    --alarm-description "$description" \
    --namespace "$namespace" --metric-name "$metric" \
    --dimensions "$@" \
    --statistic "$statistic" --period 3600 --evaluation-periods 1 \
    --threshold "$threshold" --comparison-operator GreaterThanOrEqualToThreshold \
    --treat-missing-data ignore \
    --alarm-actions "$TOPIC_ARN" \
    --region "$REGION"
}
put_alarm "${PROJECT}-ses-bounce-rate" "AWS/SES" "Reputation.BounceRate" Average "$BOUNCE_THRESHOLD" \
  "Bounce rate of notification emails is high. SES reviews accounts above 10%; fix the cause before then." \
  "Name=ses:configuration-set,Value=${CONFIG_SET}"
put_alarm "${PROJECT}-ses-complaint-rate" "AWS/SES" "Reputation.ComplaintRate" Average "$COMPLAINT_THRESHOLD" \
  "Complaint rate of notification emails is high. SES reviews accounts above 0.5%." \
  "Name=ses:configuration-set,Value=${CONFIG_SET}"
put_alarm "${PROJECT}-notifier-errors" "AWS/Lambda" "Errors" Sum "$ERROR_THRESHOLD" \
  "The notifier Lambda is failing. Jobs still finish normally, but users are not getting email." \
  "Name=FunctionName,Value=${FUNCTION}"

# ---------------------------------------------------------------------------
echo
echo "=== 4. IAM role ${ROLE_NAME} ==="
cat > "$TMP/trust.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {"Service": "lambda.amazonaws.com"},
    "Action": "sts:AssumeRole"
  }]
}
EOF

cat > "$TMP/permissions.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SendFromVerifiedDomainOnly",
      "Effect": "Allow",
      "Action": "ses:SendEmail",
      "Resource": ["${IDENTITY_ARN}", "${CONFIG_SET_ARN}"]
    },
    {
      "Sid": "ReadProfilesAndJobs",
      "Effect": "Allow",
      "Action": "dynamodb:GetItem",
      "Resource": [
        "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${USERS_TABLE}",
        "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${PROJECTS_TABLE}"
      ]
    },
    {
      "Sid": "ClaimNotificationOnJob",
      "Effect": "Allow",
      "Action": "dynamodb:UpdateItem",
      "Resource": "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${PROJECTS_TABLE}"
    }
  ]
}
EOF

ROLE_CREATED=false
if aws iam get-role --role-name "$ROLE_NAME" >/dev/null 2>&1; then
  echo "Role exists."
else
  act "create role ${ROLE_NAME}" \
    aws iam create-role --role-name "$ROLE_NAME" \
    --assume-role-policy-document "file://${TMP}/trust.json" \
    --description "Notifier Lambda: send job emails from ${DOMAIN}" --output text --query 'Role.Arn'
  act "attach AWSLambdaBasicExecutionRole (CloudWatch Logs)" \
    aws iam attach-role-policy --role-name "$ROLE_NAME" \
    --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
  ROLE_CREATED=true
fi
act "put inline policy ${POLICY_NAME} (SES send on ${DOMAIN} only, DynamoDB get/update on two tables)" \
  aws iam put-role-policy --role-name "$ROLE_NAME" --policy-name "$POLICY_NAME" \
  --policy-document "file://${TMP}/permissions.json"

# ---------------------------------------------------------------------------
echo
echo "=== 5. Lambda ${FUNCTION} ==="
echo "Building ./cmd/notifier (linux/amd64)..."
(cd "$BACKEND_DIR" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "${TMP}/bootstrap" ./cmd/notifier)
(cd "$TMP" && zip -q -j notifier.zip bootstrap)
echo "Built $(du -h "${TMP}/notifier.zip" | cut -f1) package."

ENV_JSON=$(DYNAMODB_TABLE="$PROJECTS_TABLE" USERS_TABLE="$USERS_TABLE" SES_CONFIG_SET="$CONFIG_SET" \
  FROM_ADDRESS="$FROM_ADDRESS" REPLY_TO="$REPLY_TO" APP_BASE_URL="$APP_BASE_URL" python3 -c '
import json, os
keys = ["DYNAMODB_TABLE", "USERS_TABLE", "SES_CONFIG_SET", "FROM_ADDRESS", "REPLY_TO", "APP_BASE_URL"]
print(json.dumps({"Variables": {k: os.environ[k] for k in keys}}))')

create_function() {
  if $ROLE_CREATED; then
    echo "Waiting for the new role to propagate..."
    sleep 12
  fi
  local attempt
  for attempt in 1 2 3 4; do
    if aws lambda create-function \
      --function-name "$FUNCTION" --runtime provided.al2023 --handler bootstrap \
      --architectures x86_64 --role "$ROLE_ARN" \
      --zip-file "fileb://${TMP}/notifier.zip" \
      --memory-size 256 --timeout 30 \
      --description "Emails users when a quiz video or Shorts job finishes or fails" \
      --environment "$ENV_JSON" --region "$REGION" --query 'FunctionArn' --output text; then
      return 0
    fi
    echo "create-function failed (attempt ${attempt}); retrying in 10s (new roles can take a moment)..."
    sleep 10
  done
  return 1
}

update_function() {
  aws lambda update-function-code --function-name "$FUNCTION" \
    --zip-file "fileb://${TMP}/notifier.zip" --region "$REGION" --query 'LastModified' --output text
  aws lambda wait function-updated --function-name "$FUNCTION" --region "$REGION"
  aws lambda update-function-configuration --function-name "$FUNCTION" \
    --role "$ROLE_ARN" --timeout 30 --memory-size 256 \
    --environment "$ENV_JSON" --region "$REGION" --query 'LastModified' --output text
  aws lambda wait function-updated --function-name "$FUNCTION" --region "$REGION"
}

if aws lambda get-function --function-name "$FUNCTION" --region "$REGION" >/dev/null 2>&1; then
  act "update code and configuration of ${FUNCTION}" update_function
else
  act "create function ${FUNCTION} (provided.al2023, x86_64, 256 MB, 30 s)" create_function
fi

act "keep notifier logs for 30 days" bash -c "
  aws logs create-log-group --log-group-name /aws/lambda/${FUNCTION} --region ${REGION} 2>/dev/null || true
  aws logs put-retention-policy --log-group-name /aws/lambda/${FUNCTION} --retention-in-days 30 --region ${REGION}"

# ---------------------------------------------------------------------------
echo
echo "=== 6. Step Functions: add the notify states ==="

# The state machine role must be allowed to invoke the notifier.
ALLOWED=$(SFN_ROLE_NAME="$SFN_ROLE_NAME" FUNCTION_ARN="$FUNCTION_ARN" python3 - <<'PY'
import fnmatch, json, os, subprocess

role, arn = os.environ["SFN_ROLE_NAME"], os.environ["FUNCTION_ARN"]


def aws(*args):
    return json.loads(subprocess.check_output(["aws", *args, "--output", "json"]))


names = aws("iam", "list-role-policies", "--role-name", role)["PolicyNames"]
ok = False
for n in names:
    doc = aws("iam", "get-role-policy", "--role-name", role, "--policy-name", n)["PolicyDocument"]
    for st in doc["Statement"]:
        actions = st["Action"] if isinstance(st["Action"], list) else [st["Action"]]
        resources = st["Resource"] if isinstance(st["Resource"], list) else [st["Resource"]]
        if st.get("Effect") == "Allow" and any(fnmatch.fnmatch("lambda:InvokeFunction", a) for a in actions) \
                and any(fnmatch.fnmatch(arn, r) for r in resources):
            ok = True
print("yes" if ok else "no")
PY
)
if [ "$ALLOWED" = "yes" ]; then
  echo "State machine role ${SFN_ROLE_NAME} can already invoke ${FUNCTION} (no IAM change needed)."
else
  echo "ERROR: ${SFN_ROLE_NAME} has no lambda:InvokeFunction permission covering ${FUNCTION_ARN}." >&2
  echo "Add it to the role's policy before applying." >&2
  exit 1
fi

cat > "$TMP/patch.py" <<'PY'
import json
import os
import sys

src, out = sys.argv[1], sys.argv[2]
notifier = os.environ["FUNCTION_ARN"]
d = json.load(open(src))
S = d["States"]

if "NotifyCompleted" in S or "NotifyFailed" in S:
    print("already patched")
    sys.exit(3)

for name in ("MarkCompleted", "MarkFailed"):
    s = S.get(name)
    if not s or s.get("Type") != "Task" or s.get("End") is not True:
        sys.exit("%s is not the expected terminal Task; refusing to patch" % name)

shorts = "jobId.$" in S["MarkCompleted"]["Parameters"]
kind, id_key = ("shorts", "jobId") if shorts else ("quiz", "projectId")


def notify(outcome):
    return {
        "Type": "Task",
        "Comment": "Best-effort email. Any failure here continues to NotifyEnd; the job outcome is already recorded.",
        "Resource": notifier,
        "Parameters": {
            "kind": kind,
            "outcome": outcome,
            "userId.$": "$.userId",
            id_key + ".$": "$." + id_key,
            "executionId.$": "$$.Execution.Id",
        },
        "ResultPath": None,
        "Retry": [{"ErrorEquals": ["Throttled"], "IntervalSeconds": 10, "MaxAttempts": 4, "BackoffRate": 2.0}],
        "Catch": [{"ErrorEquals": ["States.ALL"], "ResultPath": "$.notifyError", "Next": "NotifyEnd"}],
        "End": True,
    }


for name, nxt in (("MarkCompleted", "NotifyCompleted"), ("MarkFailed", "NotifyFailed")):
    del S[name]["End"]
    S[name]["ResultPath"] = None
    S[name]["Next"] = nxt

S["NotifyCompleted"] = notify("completed")
S["NotifyFailed"] = notify("failed")
S["NotifyEnd"] = {"Type": "Pass", "End": True}

json.dump(d, open(out, "w"), indent=2)
print("patched (%s)" % kind)
PY

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
for SM in "${STATE_MACHINES[@]}"; do
  SM_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${SM}"
  echo
  echo "--- ${SM}"
  aws stepfunctions describe-state-machine --state-machine-arn "$SM_ARN" --region "$REGION" \
    --query 'definition' --output text > "${TMP}/${SM}.live.json"

  set +e
  FUNCTION_ARN="$FUNCTION_ARN" python3 "$TMP/patch.py" "${TMP}/${SM}.live.json" "${TMP}/${SM}.patched.json"
  PATCH_RC=$?
  set -e
  if [ "$PATCH_RC" -eq 3 ]; then
    echo "Already wired, nothing to do."
    continue
  elif [ "$PATCH_RC" -ne 0 ]; then
    echo "ERROR: could not patch ${SM}." >&2
    exit 1
  fi

  RESULT=$(aws stepfunctions validate-state-machine-definition --type STANDARD \
    --definition "file://${TMP}/${SM}.patched.json" --region "$REGION" --query 'result' --output text)
  if [ "$RESULT" != "OK" ]; then
    echo "ERROR: patched definition for ${SM} did not validate (${RESULT})." >&2
    aws stepfunctions validate-state-machine-definition --type STANDARD \
      --definition "file://${TMP}/${SM}.patched.json" --region "$REGION" --query 'diagnostics' >&2
    exit 1
  fi
  echo "Patched definition validates (AWS validator: OK)."
  echo "Flow after: MarkCompleted -> NotifyCompleted (end); MarkFailed -> NotifyFailed (end); notify errors -> NotifyEnd (end)"

  BACKUP="/tmp/${SM}-before-${TIMESTAMP}.json"
  if $APPLY; then
    cp "${TMP}/${SM}.live.json" "$BACKUP"
    echo ">> saved previous definition to ${BACKUP}"
    aws stepfunctions update-state-machine --state-machine-arn "$SM_ARN" \
      --definition "file://${TMP}/${SM}.patched.json" --region "$REGION" --query 'updateDate' --output text
  else
    echo "[dry-run] would: save the live definition to ${BACKUP}, then update ${SM}"
  fi
done

# ---------------------------------------------------------------------------
if $APPLY; then
  echo
  echo "=== 7. Smoke test: invalid payload must fail validation and send nothing ==="
  aws lambda invoke --function-name "$FUNCTION" --payload '{"kind":"smoke-test"}' \
    --cli-binary-format raw-in-base64-out --region "$REGION" "${TMP}/smoke.json" >/dev/null
  if grep -q '"errorType":"SendFailed"' "${TMP}/smoke.json"; then
    echo "OK: the notifier runs and rejects bad input (SendFailed)."
  else
    echo "WARNING: unexpected smoke test response:" >&2
    cat "${TMP}/smoke.json" >&2
    exit 1
  fi
fi

echo
if $APPLY; then
  echo "Done. Remaining manual steps:"
else
  echo "Dry run complete. Re-run with --apply to make these changes. Then:"
fi
cat <<EOF
  1. Confirm the SNS subscription: open the confirmation email sent to ${ALERT_EMAIL}
     (it arrives through ImprovMX) and click the link. Alarms cannot email you until you do.
  2. Redeploy the API Lambda so the new /settings/notifications endpoints exist
     (code only; this does not touch its environment variables):
       cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap ./cmd/api
       (cd /tmp && zip -j api.zip bootstrap)
       aws lambda update-function-code --function-name ${PROJECT}-api --zip-file fileb:///tmp/api.zip --region ${REGION}
  3. Deploy the frontend (Settings page).
  4. Finish a real job and check that the email arrives in Gmail with SPF, DKIM and DMARC passing.
EOF
