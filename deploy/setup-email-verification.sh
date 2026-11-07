#!/bin/bash
# Email confirmation for password accounts (SEC-08): what the AWS side needs.
#
# The API Lambda now mails a confirmation link to people who sign up with an
# email address and password, and holds back costly actions (new projects,
# Shorts, story videos, uploads) until the link is opened. Two things have to be
# true in AWS for that to work, and one switch turns it on:
#
#   1. IAM    the API Lambda's role may send mail, and only from
#             notifications@indifferent.fun, only through the indifferent.fun
#             identity and the indifferent-notifications configuration set (the
#             same ones the notification emails use, so bounces and complaints
#             show up in the same SES reputation metrics and alarms).
#             The role is shared by several Lambdas (see SEC-09), so the
#             FromAddress condition is what keeps the new permission narrow.
#   2. Switch EMAIL_VERIFICATION_ENABLED=true on the API Lambda. Until it is
#             set, the code mails nothing and holds nothing back, so the code can
#             be deployed first and the feature turned on once the permission is
#             in place. Clearing it turns the feature off again for everyone,
#             including accounts that signed up while it was on.
#
# Order:  --apply-iam  ->  deploy the API Lambda  ->  --enable
#
# This touches live AWS. It prints the plan and changes nothing unless you pass
# one of --apply-iam, --enable or --disable.
#
# Usage:
#   bash deploy/setup-email-verification.sh                # show the plan and check the policy
#   bash deploy/setup-email-verification.sh --apply-iam
#   bash deploy/setup-email-verification.sh --enable
#   bash deploy/setup-email-verification.sh --disable
#
# Undo:
#   bash deploy/setup-email-verification.sh --disable
#   aws iam delete-role-policy --role-name indifferent-fun-lambda-role \
#     --policy-name indifferent-fun-api-send-verification

set -euo pipefail

REGION="ap-south-1"
PROJECT="indifferent-fun"
DOMAIN="indifferent.fun"
FROM_ADDRESS="notifications@indifferent.fun"
CONFIG_SET="indifferent-notifications"
ROLE_NAME="${PROJECT}-lambda-role"
POLICY_NAME="${PROJECT}-api-send-verification"
API_FUNCTION="${PROJECT}-api"
ENV_KEY="EMAIL_VERIFICATION_ENABLED"

MODE=plan
fail() { echo "ERROR: $*" >&2; exit 1; }
for arg in "$@"; do
  case "$arg" in
    --apply-iam) MODE=apply-iam ;;
    --enable) MODE=enable ;;
    --disable) MODE=disable ;;
    *) fail "unknown argument: $arg" ;;
  esac
done

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
IDENTITY_ARN="arn:aws:ses:${REGION}:${ACCOUNT_ID}:identity/${DOMAIN}"
CONFIG_SET_ARN="arn:aws:ses:${REGION}:${ACCOUNT_ID}:configuration-set/${CONFIG_SET}"

TMP=$(mktemp -d)
chmod 700 "$TMP"
trap 'rm -rf "$TMP"' EXIT

cat >"$TMP/policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SendConfirmationEmail",
      "Effect": "Allow",
      "Action": "ses:SendEmail",
      "Resource": ["${IDENTITY_ARN}", "${CONFIG_SET_ARN}"],
      "Condition": {"StringEquals": {"ses:FromAddress": "${FROM_ADDRESS}"}}
    }
  ]
}
EOF

echo "Account ${ACCOUNT_ID}, region ${REGION}"
echo "Plan:"
echo "  1. role ${ROLE_NAME}: inline policy ${POLICY_NAME}"
echo "       ses:SendEmail on ${IDENTITY_ARN}"
echo "                    and ${CONFIG_SET_ARN}"
echo "       only when the From address is ${FROM_ADDRESS}"
echo "  2. ${API_FUNCTION}: environment variable ${ENV_KEY}=true (--enable) or removed (--disable)"
echo

# --- read-only checks, always run ------------------------------------------
echo "--- Checks (read only)"
STATUS=$(aws sesv2 get-email-identity --email-identity "$DOMAIN" --region "$REGION" --query VerifiedForSendingStatus --output text 2>/dev/null || true)
[ "$STATUS" = "True" ] || fail "SES identity ${DOMAIN} is not verified for sending (status: ${STATUS:-not found}). Run deploy/setup-ses-identity.sh first."
echo "SES identity ${DOMAIN}: verified"
aws sesv2 get-configuration-set --configuration-set-name "$CONFIG_SET" --region "$REGION" --query ConfigurationSetName --output text >/dev/null 2>&1 ||
  fail "SES configuration set ${CONFIG_SET} not found. Run deploy/setup-notifications.sh first."
echo "SES configuration set ${CONFIG_SET}: exists"
PROD=$(aws sesv2 get-account --region "$REGION" --query ProductionAccessEnabled --output text)
[ "$PROD" = "True" ] || fail "SES is still in the sandbox; confirmation emails could only reach verified addresses."
echo "SES production access: enabled"

# The policy text, tested by IAM itself without applying anything: the right
# From address is allowed, another one is not.
python3 -c 'import json,sys; print(json.dumps([open(sys.argv[1]).read()]))' "$TMP/policy.json" >"$TMP/policy-list.json"
sim() {
  aws iam simulate-custom-policy --policy-input-list "file://${TMP}/policy-list.json" \
    --action-names ses:SendEmail --resource-arns "$IDENTITY_ARN" "$CONFIG_SET_ARN" \
    --context-entries "ContextKeyName=ses:FromAddress,ContextKeyValues=$1,ContextKeyType=string" \
    --query 'EvaluationResults[0].EvalDecision' --output text
}
[ "$(sim "$FROM_ADDRESS")" = "allowed" ] || fail "the policy does not allow sending from ${FROM_ADDRESS}"
[ "$(sim "someone-else@${DOMAIN}")" != "allowed" ] || fail "the policy allows sending from other addresses"
echo "Policy check: allowed from ${FROM_ADDRESS}, refused from any other address"

CURRENT=$(aws iam get-role-policy --role-name "$ROLE_NAME" --policy-name "$POLICY_NAME" --query PolicyName --output text 2>/dev/null || true)
if [ "$CURRENT" = "$POLICY_NAME" ]; then echo "Inline policy ${POLICY_NAME}: already on the role"; else echo "Inline policy ${POLICY_NAME}: not on the role yet"; fi

# --- helpers for the Lambda environment --------------------------------------
# update-function-configuration REPLACES the whole environment, and this one
# holds secrets. So it is read into a private temporary file, edited there, and
# written back from there; --query keeps the command's own answer (which echoes
# the environment) from being printed, and --revision-id makes the write fail
# instead of overwriting if the function changed since it was read.
env_edit() { # $1 = set | unset
  (umask 077 && aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" --output json >"$TMP/config.json") ||
    fail "could not read ${API_FUNCTION}"
  python3 - "$1" "$ENV_KEY" "$TMP/config.json" "$TMP/env.json" <<'PY'
import json, sys
action, key, src, dst = sys.argv[1:5]
cfg = json.load(open(src))
if cfg.get("LastUpdateStatus") not in (None, "Successful") or cfg.get("State") not in (None, "Active"):
    sys.exit("function is not ready for a change (state %s, last update %s)" % (cfg.get("State"), cfg.get("LastUpdateStatus")))
env = (cfg.get("Environment") or {}).get("Variables") or {}
if action == "set":
    env[key] = "true"
else:
    env.pop(key, None)
json.dump({"Variables": env}, open(dst, "w"))
print("%s %s; %d other variable(s) kept as they were" % ("set" if action == "set" else "removed", key, len(env) - (1 if action == "set" else 0)))
PY
  local rev
  rev=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["RevisionId"])' "$TMP/config.json")
  aws lambda update-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
    --revision-id "$rev" --environment "file://${TMP}/env.json" --query LastModified --output text
  aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"
}

case "$MODE" in
  plan)
    echo
    echo "Dry run. Nothing was changed. Order: --apply-iam, deploy the API Lambda, then --enable."
    ;;
  apply-iam)
    echo
    echo "--- Applying the inline policy"
    aws iam put-role-policy --role-name "$ROLE_NAME" --policy-name "$POLICY_NAME" --policy-document "file://${TMP}/policy.json"
    echo "done. IAM changes can take a few seconds to reach Lambda."
    ;;
  enable)
    echo
    [ "$CURRENT" = "$POLICY_NAME" ] || fail "the IAM policy is not on the role yet; run --apply-iam first, or every confirmation email will fail."
    echo "--- Switching confirmation on"
    env_edit set
    ;;
  disable)
    echo
    echo "--- Switching confirmation off"
    env_edit unset
    ;;
esac
