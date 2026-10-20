#!/bin/bash
set -euo pipefail
# Shorts pipeline: start AI thumbnails automatically (issue #77)
# ==============================================================
#
# After transcription completes, the pipeline starts a thumbnails run for the
# job and carries straight on to ranking. Audio uploads are skipped. Two states
# are added:
#
#   ThumbnailsForVideo?   Choice: .mp4 / .mov source -> StartThumbnails, else ranking
#   StartThumbnails       starts the thumbnails state machine (fire and forget);
#                         any error continues to ranking, so thumbnails can never
#                         affect the clips
#
# and the COMPLETED branch of TranscriptionDone? points at ThumbnailsForVideo?
# instead of UpdateStatus_Ranking. backend/internal/shorts/pipeline/statemachine.json
# already contains the same states, so deploy/setup-shorts.sh keeps them.
#
# Run deploy/setup-thumbnails.sh --apply first. This patch is safe without it
# (the new step fails, is caught, and ranking goes ahead), but nothing would be
# generated.
#
# Usage:
#   bash deploy/patch-shorts-thumbnails.sh            # dry run (read-only)
#   bash deploy/patch-shorts-thumbnails.sh --apply    # updates the state machine
#
# Safety:
#   - Idempotent: does nothing if the states are already there.
#   - Patches the LIVE definition and refuses if the route is not exactly as
#     described. Nothing else in the definition changes.
#   - Validates the result with the Step Functions validator before updating.
#   - Saves the previous definition to /tmp/<state-machine>-before-<timestamp>.json.
#   - Executions already running keep the old definition and get no auto-start.
#
# Undo:
#   aws stepfunctions update-state-machine --state-machine-arn <arn> \
#       --definition file:///tmp/indifferent-fun-shorts-pipeline-before-<timestamp>.json
REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
SM_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${PROJECT}-shorts-pipeline"
THUMBS_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${PROJECT}-thumbnails"
APPLY=false
if [ "${1:-}" = "--apply" ]; then
  APPLY=true
elif [ -n "${1:-}" ]; then
  echo "Unknown argument: $1 (use --apply or no argument)" >&2
  exit 1
fi
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
if $APPLY; then echo "MODE: apply"; else echo "MODE: dry run (read-only)"; fi

if ! aws stepfunctions describe-state-machine --state-machine-arn "$THUMBS_ARN" --region "$REGION" >/dev/null 2>&1; then
  echo "NOTE: ${THUMBS_ARN} does not exist yet. Run deploy/setup-thumbnails.sh --apply first." >&2
fi

aws stepfunctions describe-state-machine --state-machine-arn "$SM_ARN" --region "$REGION" \
  --query 'definition' --output text > "$TMP/live.json"

cat > "$TMP/patch.py" <<'PY'
import json
import sys

src, out, thumbs_arn = sys.argv[1], sys.argv[2], sys.argv[3]
d = json.load(open(src))
S = d["States"]
if "StartThumbnails" in S:
    print("already patched")
    sys.exit(3)

def refuse(msg):
    sys.exit("refusing to patch: " + msg)

done = S.get("TranscriptionDone?")
if not done or done.get("Type") != "Choice":
    refuse("TranscriptionDone? is not the expected Choice state")
completed = [
    c for c in done["Choices"]
    if c.get("StringEquals") == "COMPLETED" and c.get("Variable") == "$.transcribeStatus.state"
]
if len(completed) != 1 or completed[0].get("Next") != "UpdateStatus_Ranking":
    refuse("expected exactly one COMPLETED branch going to UpdateStatus_Ranking")
if "UpdateStatus_Ranking" not in S:
    refuse("UpdateStatus_Ranking is missing")

completed[0]["Next"] = "ThumbnailsForVideo?"
S["ThumbnailsForVideo?"] = {
    "Type": "Choice",
    "Comment": "Thumbnails need a picture, so audio uploads skip them (issue #77).",
    "Choices": [{
        "Or": [{"Variable": "$.sourceKey", "StringMatches": ext} for ext in ("*.mp4", "*.MP4", "*.mov", "*.MOV")],
        "Next": "StartThumbnails",
    }],
    "Default": "UpdateStatus_Ranking",
}
S["StartThumbnails"] = {
    "Type": "Task",
    "Comment": "Fire and forget: starts AI thumbnail generation alongside the pipeline. Any failure here carries on to ranking, because thumbnails must never affect the clips. The fixed execution name makes this run once per job.",
    "Resource": "arn:aws:states:::states:startExecution",
    "Parameters": {
        "StateMachineArn": thumbs_arn,
        "Name.$": "States.Format('thumbnails-{}-auto', $.jobId)",
        "Input": {
            "jobId.$": "$.jobId",
            "userId.$": "$.userId",
            "sourceKey.$": "$.sourceKey",
            "transcriptKey.$": "$.transcribeStatus.transcriptKey",
            "sourceDuration.$": "$.sourceDuration",
        },
    },
    "ResultPath": None,
    "Catch": [{"ErrorEquals": ["States.ALL"], "ResultPath": "$.thumbnailsError", "Next": "UpdateStatus_Ranking"}],
    "Next": "UpdateStatus_Ranking",
}
json.dump(d, open(out, "w"), indent=2)
print("patched")
PY

set +e
python3 "$TMP/patch.py" "$TMP/live.json" "$TMP/patched.json" "$THUMBS_ARN"
RC=$?
set -e
if [ "$RC" -eq 3 ]; then
  echo "Nothing to do."
  exit 0
elif [ "$RC" -ne 0 ]; then
  echo "ERROR: could not patch the live definition." >&2
  exit 1
fi

RESULT=$(aws stepfunctions validate-state-machine-definition --type STANDARD \
  --definition "file://${TMP}/patched.json" --region "$REGION" --query 'result' --output text)
if [ "$RESULT" != "OK" ]; then
  echo "ERROR: patched definition did not validate (${RESULT})." >&2
  aws stepfunctions validate-state-machine-definition --type STANDARD \
    --definition "file://${TMP}/patched.json" --region "$REGION" --query 'diagnostics' >&2
  exit 1
fi
echo "Patched definition validates."

if ! $APPLY; then
  echo "[dry-run] would update ${SM_ARN} (saving the live definition first)."
  echo "Re-run with --apply to make the change."
  exit 0
fi

BACKUP="/tmp/${PROJECT}-shorts-pipeline-before-$(date -u +%Y%m%d-%H%M%S).json"
cp "$TMP/live.json" "$BACKUP"
echo "Previous definition saved to ${BACKUP}"
aws stepfunctions update-state-machine --state-machine-arn "$SM_ARN" \
  --definition "file://${TMP}/patched.json" --region "$REGION" >/dev/null
echo "Updated ${SM_ARN}."
echo "Undo: aws stepfunctions update-state-machine --state-machine-arn ${SM_ARN} --definition file://${BACKUP}"
