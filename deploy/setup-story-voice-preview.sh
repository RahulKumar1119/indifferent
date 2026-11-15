#!/bin/bash
# Infrastructure for the story voice preview (#326). Dry run unless --apply.
#
# The preview keeps each voice take the user heard, private, in the assets bucket
# under story-preview/<user>/<hash>.mp3. The story render looks for the same take
# and uses it instead of calling Polly again. This script does the three things
# around that code:
#
#   1. Lifecycle   expire story-preview/ after 2 days (noncurrent versions after 1
#                  day, stray delete markers removed). MERGED into the bucket's
#                  existing rules: put-bucket-lifecycle-configuration replaces the
#                  whole configuration, so the current rules are read and written
#                  back with this one added. The rule is refused if its prefix
#                  could reach storyvideo/ (the finished videos).
#   2. Render role let the story render task READ story-preview/*. Without it the
#                  render cannot see the takes, makes the voice again with Polly,
#                  and the preview still works (it only loses the saving and the
#                  same-bytes guarantee). Read only: no write, no delete.
#   3. --enable    turn the feature on in the API Lambda (STORY_VOICE_PREVIEW_ENABLED
#                  =true). Off until then: the route answers 503 and the page shows
#                  no play buttons. The Lambda's environment holds secrets, so it
#                  is read and written back through a private file and only the
#                  one key is changed; values are never printed.
#
# The API Lambda role already allows polly:SynthesizeSpeech and S3 on the bucket.
#
# Usage:
#   bash deploy/setup-story-voice-preview.sh                # show the plan
#   bash deploy/setup-story-voice-preview.sh --apply        # steps 1 and 2
#   bash deploy/setup-story-voice-preview.sh --apply --enable   # also step 3
#   bash deploy/setup-story-voice-preview.sh --disable --apply # switch the feature off
#
# Undo:
#   step 1: put the configuration saved in /tmp/story-preview-lifecycle-before.json back
#   step 2: aws iam delete-role-policy --role-name indifferent-fun-story-render-task-role \
#             --policy-name indifferent-fun-story-render-preview-read
#   step 3: bash deploy/setup-story-voice-preview.sh --disable --apply
set -euo pipefail
REGION="ap-south-1"
PROJECT="indifferent-fun"
BUCKET="${PROJECT}-assets"
API_FUNCTION="${PROJECT}-api"
TASK_ROLE="${PROJECT}-story-render-task-role"
POLICY_NAME="${PROJECT}-story-render-preview-read"
PREFIX="story-preview/"
OUT_PREFIX="storyvideo/"
RULE_ID="story-preview-expiry"
APPLY=false
ENABLE=false
DISABLE=false
fail() { echo "ERROR: $*" >&2; exit 1; }
while [ $# -gt 0 ]; do
  case "$1" in
    --apply) APPLY=true ;;
    --enable) ENABLE=true ;;
    --disable) DISABLE=true ;;
    *) fail "unknown argument: $1" ;;
  esac
  shift
done
$ENABLE && $DISABLE && fail "--enable and --disable together make no sense"
case "$OUT_PREFIX" in "$PREFIX"*) fail "the preview prefix must not contain the output prefix" ;; esac
case "$PREFIX" in "$OUT_PREFIX"*) fail "the preview prefix must not be inside the output prefix" ;; esac
act() {
  if $APPLY; then
    "$@"
  else
    echo "  (dry run) would run: $*"
  fi
}
TMP=$(mktemp -d)
chmod 700 "$TMP"
trap 'rm -rf "$TMP"' EXIT

if ! $DISABLE; then
  echo "=== 1. S3 lifecycle: expire ${PREFIX} in ${BUCKET} ==="
  aws s3api get-bucket-lifecycle-configuration --bucket "$BUCKET" --region "$REGION" --output json > "$TMP/before.json"
  NEW=$(RULE_ID="$RULE_ID" PREFIX="$PREFIX" python3 - "$TMP/before.json" <<'PY'
import json, os, sys
cfg = json.load(open(sys.argv[1]))
rid, prefix = os.environ["RULE_ID"], os.environ["PREFIX"]
if any(r.get("ID") == rid for r in cfg["Rules"]):
    print("EXISTS")
else:
    cfg["Rules"].append({
        "ID": rid,
        "Filter": {"Prefix": prefix},
        "Status": "Enabled",
        "Expiration": {"Days": 2},
        "NoncurrentVersionExpiration": {"NoncurrentDays": 1},
        "AbortIncompleteMultipartUpload": {"DaysAfterInitiation": 1},
    })
    print(json.dumps({"Rules": cfg["Rules"]}))
PY
)
  if [ "$NEW" = "EXISTS" ]; then
    echo "rule ${RULE_ID} is already there."
  else
    echo "adding ${RULE_ID}: objects under ${PREFIX} expire after 2 days. Existing rules kept: $(python3 -c 'import json,sys; print(", ".join(r["ID"] for r in json.load(open(sys.argv[1]))["Rules"]))' "$TMP/before.json")"
    MIN_SIZE=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("TransitionDefaultMinimumObjectSize", ""))' "$TMP/before.json")
    echo "$NEW" > "$TMP/after.json"
    if $APPLY; then
      cp "$TMP/before.json" /tmp/story-preview-lifecycle-before.json
      SIZE_ARGS=()
      [ -z "$MIN_SIZE" ] || SIZE_ARGS=(--transition-default-minimum-object-size "$MIN_SIZE")
      aws s3api put-bucket-lifecycle-configuration --bucket "$BUCKET" --region "$REGION" \
        --lifecycle-configuration "file://$TMP/after.json" ${SIZE_ARGS[@]+"${SIZE_ARGS[@]}"}
      echo "done; the previous configuration is saved in /tmp/story-preview-lifecycle-before.json"
    else
      echo "  (dry run) would write the configuration above"
    fi
  fi

  echo
  echo "=== 2. Render role: read ${PREFIX}* ==="
  cat > "$TMP/policy.json" <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["s3:GetObject"], "Resource": "arn:aws:s3:::${BUCKET}/${PREFIX}*"}
  ]
}
POLICY
  if aws iam get-role --role-name "$TASK_ROLE" >/dev/null 2>&1; then
    act aws iam put-role-policy --role-name "$TASK_ROLE" --policy-name "$POLICY_NAME" --policy-document "file://$TMP/policy.json"
  else
    echo "role ${TASK_ROLE} does not exist yet (run deploy/setup-storyvideo.sh first); skipping."
  fi
fi

if $ENABLE || $DISABLE; then
  echo
  echo "=== 3. API Lambda: STORY_VOICE_PREVIEW_ENABLED ==="
  VALUE=true
  $DISABLE && VALUE=false
  REVISION=$(aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" --query RevisionId --output text)
  aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" --query 'Environment.Variables' --output json > "$TMP/env.json"
  chmod 600 "$TMP/env.json"
  VALUE="$VALUE" python3 - "$TMP/env.json" "$TMP/env-new.json" <<'PY'
import json, os, sys
env = json.load(open(sys.argv[1]))
env["STORY_VOICE_PREVIEW_ENABLED"] = os.environ["VALUE"]
json.dump({"Variables": env}, open(sys.argv[2], "w"))
print("environment has %d variables after the change; only STORY_VOICE_PREVIEW_ENABLED differs" % len(env))
PY
  chmod 600 "$TMP/env-new.json"
  if $APPLY; then
    aws lambda update-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
      --revision-id "$REVISION" --environment "file://$TMP/env-new.json" --query 'LastUpdateStatus' --output text
    aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"
    echo "STORY_VOICE_PREVIEW_ENABLED=${VALUE}"
  else
    echo "  (dry run) would set STORY_VOICE_PREVIEW_ENABLED=${VALUE}"
  fi
fi

if ! $APPLY; then
  echo
  echo "Dry run. Nothing was changed. Run again with --apply to make these changes."
fi
