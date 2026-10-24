#!/bin/bash
set -euo pipefail
# AI Shorts multipart upload infrastructure (issue #151)
# =======================================================
#
# Large sources are uploaded to S3 as parallel parts that can be retried and
# resumed. The routes live on the existing API Lambda
# (POST /shorts/{id}/upload/multipart/{initiate,parts,complete},
# GET and DELETE /shorts/{id}/upload/multipart). This script prepares what they
# need, in this order:
#
#   0. Preflight          reads only: bucket, Lambda, and the API Gateway
#                         {proxy+} resource that already forwards every path
#                         and method (DELETE included) to the Lambda, so there
#                         are no routes to register
#   1. IAM                the API Lambda role gets s3:AbortMultipartUpload and
#                         s3:ListMultipartUploadParts on the assets bucket.
#                         Creating, uploading and completing a multipart
#                         upload are already covered by s3:PutObject.
#   2. Bucket CORS        the browser must be allowed to read the ETag header
#                         S3 returns for each part. Changed only if it is not
#                         exposed already; every other rule is kept as it is.
#   3. Bucket lifecycle   incomplete multipart uploads are aborted after 1 day,
#                         so abandoned uploads stop costing storage. The new
#                         rule is added next to the existing rules.
#   4. API Lambda         builds and uploads the new code (before the flag
#                         below, and after the permissions above exist, so
#                         the routes go live with everything they need)
#   5. S3 Transfer        sets S3_ACCELERATE=true on the API Lambda, so the
#      Acceleration       presigned part URLs use
#                         <bucket>.s3-accelerate.amazonaws.com. Only after
#                         step 4, because the code treats a missing flag as
#                         off. Only when the bucket reports acceleration as
#                         Enabled (step 0); this script never enables it.
#
# Deploy order across the system: run this script (backend), then merge the
# frontend. The old frontend keeps working against the new backend. A new
# frontend against an old backend falls back to the single upload.
#
# Transfer Acceleration (issue #151, part 3). It speeds up uploads from users
# far from ap-south-1, and only the browser's part uploads use it. Files of
# 16 MiB or less keep the single presigned PUT, and every call the Lambda makes
# itself keeps the regional endpoint. Cost: uploads through acceleration are
# charged $0.04/GB via US, EU and Japan edge locations and $0.08/GB via other
# edge locations, and are not charged when AWS judges the transfer not faster.
# The regional endpoint is free for inbound data. The bucket owner turns it on:
#   aws s3api put-bucket-accelerate-configuration --bucket indifferent-fun-assets --accelerate-configuration Status=Enabled
#
# Usage:
#   bash deploy/setup-multipart-upload.sh            # dry run: read-only calls, prints the plan
#   bash deploy/setup-multipart-upload.sh --apply    # makes the changes
#   bash deploy/setup-multipart-upload.sh --accelerate-off [--apply]
#                                                    # only sets S3_ACCELERATE=false (see Undo)
#
# Safety:
#   - Idempotent: each step reads the current state first and does nothing if
#     it is already right.
#   - The dry run makes only read calls and a local Go build.
#   - Nothing existing is replaced: the Lambda role gains one new inline
#     policy, the CORS rules are only extended, the lifecycle rules only gain
#     one rule (and TransitionDefaultMinimumObjectSize is passed through
#     unchanged).
#   - Before changing the CORS or lifecycle configuration the current one is
#     saved to /tmp/<bucket>-<cors|lifecycle>-before-<timestamp>.json.
#   - The Lambda environment holds secrets (JWT_SECRET, GOOGLE_CLIENT_SECRET,
#     ...). "aws lambda update-function-configuration --environment" REPLACES
#     the whole map, so step 5 reads the live map, changes the one key and
#     writes the whole map back (guarded by the function's revision ID, and
#     refused unless every variable the API needs to start is present). It
#     never prints a value, only key names. The map is held in memory, except
#     for one file, written only with --apply, readable by the owner only
#     (mode 600) and deleted right after the call.
#
# Undo:
#   aws iam delete-role-policy --role-name indifferent-fun-lambda-role --policy-name indifferent-fun-multipart-lambda-permissions
#   aws s3api put-bucket-lifecycle-configuration --bucket indifferent-fun-assets \
#       --lifecycle-configuration file:///tmp/indifferent-fun-assets-lifecycle-before-<timestamp>.json \
#       --transition-default-minimum-object-size <the value printed when the script ran>
#   aws s3api put-bucket-cors --bucket indifferent-fun-assets --cors-configuration file:///tmp/indifferent-fun-assets-cors-before-<timestamp>.json
# The API code rolls back like any other Lambda deploy (re-upload the previous
# zip, saved to /tmp/indifferent-fun-api-before-multipart-<timestamp>.zip).
#
# Undo for Transfer Acceleration, with no redeploy: the flag is the kill switch.
# Part URLs go back to the regional endpoint on the next invocation:
#   bash deploy/setup-multipart-upload.sh --accelerate-off --apply
# That sets S3_ACCELERATE=false by the same merge as step 5, touching no other
# key (any value but true or 1 means off, so removing the key works too). Do
# not do it by hand with "aws lambda update-function-configuration
# --environment Variables={S3_ACCELERATE=false}": that replaces the map and
# deletes every other variable, which stops the API.

REGION="ap-south-1"
PROJECT="indifferent-fun"
BUCKET="${PROJECT}-assets"
LAMBDA_ROLE="${PROJECT}-lambda-role"
API_FUNCTION="${PROJECT}-api"
ORIGIN="https://indifferent.fun"
POLICY_NAME="${PROJECT}-multipart-lambda-permissions"
LIFECYCLE_RULE_ID="AbortIncompleteMultipartUploads"
ABORT_AFTER_DAYS=1
BACKEND_DIR="$(cd "$(dirname "$0")/../backend" && pwd)"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

APPLY=false
ACCEL_OFF=false
for arg in "$@"; do
  case "$arg" in
    --apply) APPLY=true ;;
    --accelerate-off) ACCEL_OFF=true ;;
    *)
      echo "Unknown argument: $arg (use --apply and/or --accelerate-off)" >&2
      exit 1
      ;;
  esac
done
if $APPLY; then
  echo "MODE: apply (changes will be made in ${REGION})"
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

# merge_lambda_env <KEY> <value>: sets one variable on the API Lambda and keeps
# every other one. The map holds secrets: only key names are ever printed.
cat > "$TMP/env_merge.py" <<'PY'
import json
import os
import sys

mode, key, value, arg = sys.argv[1:5]
# What cmd/api needs to start. If the live map lacks any of these, the read
# went wrong and writing a map back would stop the API.
REQUIRED = [
    "JWT_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI",
    "DYNAMODB_TABLE", "S3_BUCKET", "STATE_MACHINE_ARN", "SHORTS_STATE_MACHINE_ARN",
    "USERS_TABLE", "SESSION_TABLE",
]
live = json.load(sys.stdin)
env = live.get("v") or {}
keys_file = os.path.join(arg, "lambda-env.keys")

if mode == "plan":
    write = sys.argv[5] == "1"
    print("live environment keys (%d): %s" % (len(env), ", ".join(sorted(env))))
    missing = [k for k in REQUIRED if k not in env]
    if missing:
        sys.exit("refusing to continue: the live environment has no " + ", ".join(missing))
    if env.get(key) == value:
        print("%s is already set to %s" % (key, value))
        sys.exit(3)
    print("will %s %s and keep the other %d keys as they are" % (
        "change" if key in env else "add", key, len([k for k in env if k != key])))
    open(keys_file, "w").write("\n".join(sorted(env)) + "\n")
    open(os.path.join(arg, "lambda-env.rev"), "w").write(live["r"])
    if write:
        merged = dict(env)
        merged[key] = value
        fd = os.open(os.path.join(arg, "lambda-env.json"), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
        with os.fdopen(fd, "w") as f:
            json.dump({"Variables": merged}, f)
else:
    before = set(open(keys_file).read().split())
    lost = sorted(before - set(env))
    if lost or env.get(key) != value:
        sys.exit("VERIFY FAILED: keys lost: " + (", ".join(lost) or "none") + "; " + key + " is " +
                 ("set to the wanted value" if env.get(key) == value else "not the wanted value"))
    print("verified: all %d earlier keys are still there, %s=%s" % (len(before), key, value))
PY

merge_lambda_env() {
  local key="$1" value="$2" write=0 rcs
  if $APPLY; then write=1; fi
  set +e
  aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
    --query '{v: Environment.Variables, r: RevisionId}' --output json \
    | python3 "$TMP/env_merge.py" plan "$key" "$value" "$TMP" "$write"
  rcs=("${PIPESTATUS[@]}")
  set -e
  if [ "${rcs[0]}" -ne 0 ]; then
    echo "ERROR: could not read the environment of ${API_FUNCTION}." >&2
    exit 1
  fi
  if [ "${rcs[1]}" -eq 3 ]; then
    echo "Nothing to do."
    return 0
  elif [ "${rcs[1]}" -ne 0 ]; then
    echo "ERROR: not changing the environment of ${API_FUNCTION}." >&2
    exit 1
  fi
  # --query keeps the reply (which would list every variable) off the screen.
  act "set ${key}=${value} on ${API_FUNCTION}, merged into its existing environment" \
    aws lambda update-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
      --environment "file://${TMP}/lambda-env.json" --revision-id "$(cat "$TMP/lambda-env.rev")" \
      --query LastModified --output text
  if $APPLY; then
    rm -f "$TMP/lambda-env.json"
    aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"
    aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
      --query '{v: Environment.Variables, r: RevisionId}' --output json \
      | python3 "$TMP/env_merge.py" verify "$key" "$value" "$TMP"
  fi
}

if $ACCEL_OFF; then
  echo "=== Rollback: S3_ACCELERATE=false on ${API_FUNCTION} ==="
  merge_lambda_env S3_ACCELERATE false
  exit 0
fi

# ---------------------------------------------------------------------------
echo "=== 0. Preflight ==="
aws s3api head-bucket --bucket "$BUCKET" --region "$REGION" >/dev/null \
  || { echo "ERROR: bucket ${BUCKET} is not reachable." >&2; exit 1; }
aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" >/dev/null \
  || { echo "ERROR: Lambda ${API_FUNCTION} not found." >&2; exit 1; }
aws iam get-role --role-name "$LAMBDA_ROLE" >/dev/null \
  || { echo "ERROR: role ${LAMBDA_ROLE} not found." >&2; exit 1; }
API_ID=$(aws apigateway get-rest-apis --region "$REGION" \
  --query "items[?name=='${API_FUNCTION}'].id | [0]" --output text)
if [ -n "$API_ID" ] && [ "$API_ID" != "None" ]; then
  PROXY_METHODS=$(aws apigateway get-resources --rest-api-id "$API_ID" --region "$REGION" \
    --query 'items[?path==`/{proxy+}`].resourceMethods | [0] | keys(@)' --output text | tr '\t' ' ')
  case " $PROXY_METHODS " in
    *" ANY "*) echo "API Gateway ${API_ID}: /{proxy+} forwards ANY method to the Lambda, no routes to add" ;;
    *) echo "WARNING: /{proxy+} on API ${API_ID} does not have an ANY method (found: ${PROXY_METHODS:-none}); the new routes would not be reachable." >&2 ;;
  esac
else
  echo "WARNING: no REST API named ${API_FUNCTION} found; check that the routes reach the Lambda." >&2
fi
echo "bucket ${BUCKET}, role ${LAMBDA_ROLE}, Lambda ${API_FUNCTION}: ok"
ACCEL_STATUS=$(aws s3api get-bucket-accelerate-configuration --bucket "$BUCKET" --region "$REGION" \
  --query Status --output text 2>/dev/null || echo unknown)
if [ "$ACCEL_STATUS" = "Enabled" ]; then
  echo "bucket ${BUCKET}: Transfer Acceleration is Enabled"
else
  echo "WARNING: bucket ${BUCKET}: Transfer Acceleration is not Enabled (status: ${ACCEL_STATUS})." >&2
  echo "  Step 5 will be skipped. To turn it on (this script never does; it adds a per-GB charge):" >&2
  echo "  aws s3api put-bucket-accelerate-configuration --bucket ${BUCKET} --accelerate-configuration Status=Enabled" >&2
fi
echo

# ---------------------------------------------------------------------------
echo "=== 1. IAM: abort and list multipart uploads ==="
cat > "$TMP/policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AbortAndListMultipartUploads",
      "Effect": "Allow",
      "Action": ["s3:AbortMultipartUpload", "s3:ListMultipartUploadParts"],
      "Resource": "arn:aws:s3:::${BUCKET}/*"
    }
  ]
}
EOF
if aws iam get-role-policy --role-name "$LAMBDA_ROLE" --policy-name "$POLICY_NAME" \
    --query PolicyDocument --output json > "$TMP/policy-live.json" 2>/dev/null \
  && python3 -c "
import json, sys
sys.exit(0 if json.load(open(sys.argv[1])) == json.load(open(sys.argv[2])) else 1)
" "$TMP/policy-live.json" "$TMP/policy.json"; then
  echo "policy ${POLICY_NAME} is already in place"
else
  act "set inline policy ${POLICY_NAME} on ${LAMBDA_ROLE}" aws iam put-role-policy --role-name "$LAMBDA_ROLE" \
    --policy-name "$POLICY_NAME" --policy-document "file://${TMP}/policy.json"
fi
echo

# ---------------------------------------------------------------------------
echo "=== 2. Bucket CORS: expose the ETag header ==="
aws s3api get-bucket-cors --bucket "$BUCKET" --region "$REGION" --output json > "$TMP/cors-live.json"
cat > "$TMP/cors.py" <<'PY'
import json
import sys

src, out, origin = sys.argv[1], sys.argv[2], sys.argv[3]
cfg = json.load(open(src))
rules = cfg.get("CORSRules", [])

def serves_uploads(rule):
    return origin in rule.get("AllowedOrigins", []) and "PUT" in rule.get("AllowedMethods", [])

def exposes_etag(rule):
    return any(h.lower() == "etag" for h in rule.get("ExposeHeaders", []))

targets = [r for r in rules if serves_uploads(r)]
if not targets:
    sys.exit("refusing to change CORS: no rule allows PUT from " + origin)
if all(exposes_etag(r) for r in targets):
    print("ETag is already exposed")
    sys.exit(3)
for r in targets:
    if not exposes_etag(r):
        r["ExposeHeaders"] = r.get("ExposeHeaders", []) + ["ETag"]
json.dump({"CORSRules": rules}, open(out, "w"), indent=2)
print("ETag added to ExposeHeaders on the rule that allows PUT from " + origin)
PY
set +e
python3 "$TMP/cors.py" "$TMP/cors-live.json" "$TMP/cors-new.json" "$ORIGIN"
RC=$?
set -e
if [ "$RC" -eq 3 ]; then
  echo "Nothing to do."
elif [ "$RC" -ne 0 ]; then
  echo "ERROR: could not prepare the CORS change." >&2
  exit 1
else
  if $APPLY; then
    CORS_BACKUP="/tmp/${BUCKET}-cors-before-${STAMP}.json"
    cp "$TMP/cors-live.json" "$CORS_BACKUP"
    echo "previous CORS saved to ${CORS_BACKUP}"
  fi
  act "put bucket CORS with ETag exposed" aws s3api put-bucket-cors --bucket "$BUCKET" --region "$REGION" \
    --cors-configuration "file://${TMP}/cors-new.json"
fi
echo

# ---------------------------------------------------------------------------
echo "=== 3. Bucket lifecycle: abort incomplete multipart uploads after ${ABORT_AFTER_DAYS} day ==="
if aws s3api get-bucket-lifecycle-configuration --bucket "$BUCKET" --region "$REGION" --output json \
    > "$TMP/lifecycle-live.json" 2> "$TMP/lifecycle.err"; then
  :
elif grep -q NoSuchLifecycleConfiguration "$TMP/lifecycle.err"; then
  echo '{"Rules": []}' > "$TMP/lifecycle-live.json"
else
  cat "$TMP/lifecycle.err" >&2
  echo "ERROR: could not read the lifecycle configuration." >&2
  exit 1
fi
cat > "$TMP/lifecycle.py" <<'PY'
import json
import sys

src, out, rule_id, days = sys.argv[1], sys.argv[2], sys.argv[3], int(sys.argv[4])
cfg = json.load(open(src))
rules = cfg.get("Rules", [])

def whole_bucket(rule):
    f = rule.get("Filter")
    return not f or f == {"Prefix": ""} or ("Prefix" in rule and not rule["Prefix"] and not f)

if any(r.get("Status") == "Enabled" and "AbortIncompleteMultipartUpload" in r and whole_bucket(r) for r in rules):
    print("a rule already aborts incomplete multipart uploads bucket-wide")
    sys.exit(3)
if any(r.get("ID") == rule_id for r in rules):
    sys.exit("refusing: a rule with ID " + rule_id + " exists but is not the expected one; fix it by hand")
rules.append({
    "ID": rule_id,
    "Filter": {"Prefix": ""},
    "Status": "Enabled",
    "AbortIncompleteMultipartUpload": {"DaysAfterInitiation": days},
})
json.dump({"Rules": rules}, open(out, "w"), indent=2)
print("existing rules kept: " + ", ".join(r.get("ID", "(no id)") for r in rules[:-1]) + "; added " + rule_id)
PY
set +e
python3 "$TMP/lifecycle.py" "$TMP/lifecycle-live.json" "$TMP/lifecycle-new.json" "$LIFECYCLE_RULE_ID" "$ABORT_AFTER_DAYS"
RC=$?
set -e
if [ "$RC" -eq 3 ]; then
  echo "Nothing to do."
elif [ "$RC" -ne 0 ]; then
  echo "ERROR: could not prepare the lifecycle change." >&2
  exit 1
else
  # The minimum object size for transitions is a separate setting: pass the current value through
  # so replacing the rules cannot change when objects move to Glacier.
  MIN_SIZE=$(python3 -c "
import json, sys
print(json.load(open(sys.argv[1])).get('TransitionDefaultMinimumObjectSize', ''))
" "$TMP/lifecycle-live.json")
  LC_ARGS=(--bucket "$BUCKET" --region "$REGION" --lifecycle-configuration "file://${TMP}/lifecycle-new.json")
  if [ -n "$MIN_SIZE" ]; then
    LC_ARGS+=(--transition-default-minimum-object-size "$MIN_SIZE")
  fi
  if $APPLY; then
    LC_BACKUP="/tmp/${BUCKET}-lifecycle-before-${STAMP}.json"
    python3 -c "
import json, sys
json.dump({'Rules': json.load(open(sys.argv[1])).get('Rules', [])}, open(sys.argv[2], 'w'), indent=2)
" "$TMP/lifecycle-live.json" "$LC_BACKUP"
    echo "previous lifecycle rules saved to ${LC_BACKUP} (transition minimum size was ${MIN_SIZE:-unset})"
  fi
  act "put bucket lifecycle with the new rule (transition minimum size ${MIN_SIZE:-unchanged})" \
    aws s3api put-bucket-lifecycle-configuration "${LC_ARGS[@]}"
fi
echo

# ---------------------------------------------------------------------------
echo "=== 4. API Lambda ==="
( cd "$BACKEND_DIR" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "$TMP/bootstrap" ./cmd/api ) \
  && echo "cmd/api builds"
if $APPLY; then
  ( cd "$TMP" && zip -q -j api.zip bootstrap )
  # Keep the code that is live now, in case the new build has to be rolled back.
  PREV_URL=$(aws lambda get-function --function-name "$API_FUNCTION" --region "$REGION" --query 'Code.Location' --output text)
  PREV_ZIP="/tmp/${API_FUNCTION}-before-multipart-${STAMP}.zip"
  curl -fsS -o "$PREV_ZIP" "$PREV_URL"
  echo ">> previous code saved to ${PREV_ZIP}"
  aws lambda update-function-code --function-name "$API_FUNCTION" --zip-file "fileb://${TMP}/api.zip" --region "$REGION" >/dev/null
  aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"
  echo ">> deployed the API Lambda"
else
  echo "[dry-run] would: deploy the API Lambda (saving the live code first)"
fi
echo

# ---------------------------------------------------------------------------
echo "=== 5. Lambda environment: S3_ACCELERATE=true (S3 Transfer Acceleration for part uploads) ==="
if [ "$ACCEL_STATUS" = "Enabled" ]; then
  merge_lambda_env S3_ACCELERATE true
else
  echo "Skipped: the bucket does not have Transfer Acceleration enabled (see the preflight)."
  echo "The code deployed in step 4 treats a missing S3_ACCELERATE as off, so this is safe. Run the script again after enabling it."
fi
echo

if $APPLY; then
  echo "Done. The frontend can be merged now."
else
  echo "Dry run complete. Re-run with --apply to make these changes."
fi
