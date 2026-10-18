#!/bin/bash
set -euo pipefail

# Shorts pipeline: make every failure path reach MarkFailed with an error
# ========================================================================
#
# MarkFailed reads $.error. Three routes into it came from Choice states, which
# cannot set $.error, so MarkFailed itself failed with States.Runtime:
#
#   TranscriptionDone?  state FAILED   -> MarkFailed
#   TranscriptionDone?  Default        -> MarkFailed
#   AnySegments?        Default        -> MarkFailed
#
# The job was then never marked failed and, since the notify state comes after
# MarkFailed, never emailed either. This script routes each of them through a
# Pass state that sets $.error first:
#
#   TranscriptionFailedError, TranscriptionStatusUnknownError, NoSegmentsError
#
# backend/internal/shorts/pipeline/statemachine.json already contains the same
# states, so deploy/setup-shorts.sh keeps them.
#
# Usage:
#   bash deploy/patch-shorts-failure-paths.sh            # dry run (read-only)
#   bash deploy/patch-shorts-failure-paths.sh --apply    # updates the state machine
#
# Safety:
#   - Idempotent: does nothing if the states are already there.
#   - Patches the LIVE definition and refuses if the routes are not exactly as
#     described above. Nothing else in the definition changes.
#   - Validates the result with the Step Functions validator before updating.
#   - Saves the previous definition to /tmp/<state-machine>-before-<timestamp>.json.
#   - Executions already running keep the old definition.
#
# Undo:
#   aws stepfunctions update-state-machine --state-machine-arn <arn> \
#       --definition file:///tmp/indifferent-fun-shorts-pipeline-before-<timestamp>.json

REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
SM_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${PROJECT}-shorts-pipeline"

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

aws stepfunctions describe-state-machine --state-machine-arn "$SM_ARN" --region "$REGION" \
  --query 'definition' --output text > "$TMP/live.json"

cat > "$TMP/patch.py" <<'PY'
import json
import sys

src, out = sys.argv[1], sys.argv[2]
d = json.load(open(src))
S = d["States"]

if "NoSegmentsError" in S:
    print("already patched")
    sys.exit(3)


def refuse(msg):
    sys.exit("refusing to patch: " + msg)


done = S.get("TranscriptionDone?")
any_seg = S.get("AnySegments?")
if not done or done.get("Type") != "Choice" or not any_seg or any_seg.get("Type") != "Choice":
    refuse("TranscriptionDone? / AnySegments? are not the expected Choice states")

failed_branches = [
    c for c in done["Choices"]
    if c.get("StringEquals") == "FAILED" and c.get("Variable") == "$.transcribeStatus.state" and c.get("Next") == "MarkFailed"
]
if len(failed_branches) != 1:
    refuse("expected exactly one TranscriptionDone? FAILED branch going to MarkFailed")
if done.get("Default") != "MarkFailed":
    refuse("TranscriptionDone? Default is not MarkFailed")
if any_seg.get("Default") != "MarkFailed":
    refuse("AnySegments? Default is not MarkFailed")


def error_state(error, cause, comment):
    return {
        "Type": "Pass",
        "Comment": comment,
        "Result": {"Error": error, "Cause": cause},
        "ResultPath": "$.error",
        "Next": "MarkFailed",
    }


failed_branches[0]["Next"] = "TranscriptionFailedError"
done["Default"] = "TranscriptionStatusUnknownError"
any_seg["Default"] = "NoSegmentsError"

S["TranscriptionFailedError"] = error_state(
    "TranscriptionFailed", "Transcription failed.",
    "MarkFailed reads $.error; a Choice branch cannot set it, so this does.")
S["TranscriptionStatusUnknownError"] = error_state(
    "TranscriptionStatusUnknown", "Transcription returned an unexpected status.",
    "MarkFailed reads $.error; a Choice default cannot set it, so this does.")
S["NoSegmentsError"] = error_state(
    "NoSegments", "No segments were found to turn into clips.",
    "MarkFailed reads $.error; a Choice default cannot set it, so this does.")

json.dump(d, open(out, "w"), indent=2)
print("patched")
PY

set +e
python3 "$TMP/patch.py" "$TMP/live.json" "$TMP/patched.json"
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
echo "Patched definition validates (AWS validator: OK)."
echo "  TranscriptionDone? FAILED  -> TranscriptionFailedError         -> MarkFailed"
echo "  TranscriptionDone? Default -> TranscriptionStatusUnknownError  -> MarkFailed"
echo "  AnySegments? Default       -> NoSegmentsError                  -> MarkFailed"

BACKUP="/tmp/${PROJECT}-shorts-pipeline-before-$(date +%Y%m%d-%H%M%S).json"
if $APPLY; then
  cp "$TMP/live.json" "$BACKUP"
  echo ">> saved previous definition to ${BACKUP}"
  aws stepfunctions update-state-machine --state-machine-arn "$SM_ARN" \
    --definition "file://${TMP}/patched.json" --region "$REGION" --query 'updateDate' --output text
  echo "Done."
else
  echo "[dry-run] would: save the live definition to ${BACKUP}, then update the state machine"
  echo "Re-run with --apply to make the change."
fi
