#!/bin/bash
set -euo pipefail

# Shorts pipeline: how often to check for a finished transcription
# ================================================================
#
# The state machine waits between transcription status checks
# (WaitForTranscription). A finished transcription is noticed up to one
# interval late, about half an interval on average. Issue #137 changes the
# interval from 15 s to 5 s. Each extra check costs one small Lambda call and
# three state transitions (Wait, Check, Choice).
#
# backend/internal/shorts/pipeline/statemachine.json already holds the new
# value, so deploy/setup-shorts.sh keeps it. This script applies it to the
# LIVE state machine without re-running that large script.
#
# Usage:
#   bash deploy/patch-shorts-poll-interval.sh                  # dry run (read-only)
#   bash deploy/patch-shorts-poll-interval.sh --apply          # set 5 s
#   bash deploy/patch-shorts-poll-interval.sh --seconds 15 --apply   # roll back to 15 s
#
# Safety:
#   - Idempotent: does nothing if the wait already has the requested value.
#   - Refuses unless WaitForTranscription is a Wait state with a fixed
#     "Seconds" value, so it cannot patch the wrong thing.
#   - Validates the result with the Step Functions validator before updating.
#   - Saves the previous definition to /tmp/<state-machine>-before-<timestamp>.json.
#   - Executions already running keep the old definition.

REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
SM_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${PROJECT}-shorts-pipeline"

SECONDS_WANTED=5
APPLY=false
while [ $# -gt 0 ]; do
  case "$1" in
    --apply) APPLY=true ;;
    --seconds)
      shift
      SECONDS_WANTED="${1:-}"
      ;;
    *)
      echo "Unknown argument: $1 (use --apply and/or --seconds N)" >&2
      exit 1
      ;;
  esac
  shift
done
case "$SECONDS_WANTED" in
  '' | *[!0-9]*) echo "--seconds must be a whole number" >&2; exit 1 ;;
esac
if [ "$SECONDS_WANTED" -lt 1 ] || [ "$SECONDS_WANTED" -gt 60 ]; then
  echo "--seconds must be between 1 and 60" >&2
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

src, out, wanted = sys.argv[1], sys.argv[2], int(sys.argv[3])
d = json.load(open(src))
wait = d["States"].get("WaitForTranscription")

if not wait or wait.get("Type") != "Wait" or not isinstance(wait.get("Seconds"), int):
    sys.exit("refusing to patch: WaitForTranscription is not a Wait state with a fixed Seconds value")
if wait.get("Next") != "CheckTranscription":
    sys.exit("refusing to patch: WaitForTranscription no longer leads to CheckTranscription")

current = wait["Seconds"]
if current == wanted:
    print("already %d s" % wanted)
    sys.exit(3)

wait["Seconds"] = wanted
json.dump(d, open(out, "w"), indent=2)
print("will change WaitForTranscription from %d s to %d s" % (current, wanted))
PY

set +e
python3 "$TMP/patch.py" "$TMP/live.json" "$TMP/patched.json" "$SECONDS_WANTED"
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
