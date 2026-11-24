#!/bin/bash
# Switch the title, description and chapters helper (#331) on or off in the API
# Lambda. Dry run unless --apply.
#
# The helper needs no new infrastructure: it asks the same Bedrock models as story
# planning (PLANNING_MODEL, PLANNING_FALLBACK_MODEL), which the API Lambda role
# already may call, and it stores nothing. It is off until COPY_HELPER_ENABLED is
# true: the routes then answer 503 and nothing calls the model.
#
# The Lambda's environment holds secrets, so it is read and written back through a
# private file and only the one key is changed; values are never printed.
#
# The copy for each platform for a clip or a story video (#330) has its own switch,
# POST_COPY_ENABLED, with the same behaviour: off until it is true, then its routes
# answer instead of 503. Choose it with --post-copy. It shares the model, the
# fallback model and the daily allowance with the helper.
#
# Usage:
#   bash deploy/setup-copy-helper.sh --enable             # show the change
#   bash deploy/setup-copy-helper.sh --enable --apply     # switch it on
#   bash deploy/setup-copy-helper.sh --disable --apply    # switch it off
#   bash deploy/setup-copy-helper.sh --post-copy --enable [--apply]   # the same, for #330
set -euo pipefail
REGION="ap-south-1"
API_FUNCTION="indifferent-fun-api"
KEY="COPY_HELPER_ENABLED"
APPLY=false
VALUE=""

fail() { echo "ERROR: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --apply) APPLY=true ;;
    --post-copy) KEY="POST_COPY_ENABLED" ;;
    --enable) VALUE=true ;;
    --disable) VALUE=false ;;
    *) fail "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$VALUE" ] || fail "say --enable or --disable"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
chmod 700 "$TMP"

REVISION=$(aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" --query RevisionId --output text)
aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" --query 'Environment.Variables' --output json > "$TMP/env.json"
chmod 600 "$TMP/env.json"
VALUE="$VALUE" KEY="$KEY" python3 - "$TMP/env.json" "$TMP/env-new.json" <<'PY'
import json, os, sys
env = json.load(open(sys.argv[1]))
key = os.environ["KEY"]
before = env.get(key)
env[key] = os.environ["VALUE"]
json.dump({"Variables": env}, open(sys.argv[2], "w"))
print("%s is %s now; it will be %s. %d variables in all; only %s differs." % (key, before if before is not None else "not set", os.environ["VALUE"], len(env), key))
PY
chmod 600 "$TMP/env-new.json"

if $APPLY; then
  aws lambda update-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
    --revision-id "$REVISION" --environment "file://$TMP/env-new.json" --query 'LastUpdateStatus' --output text
  aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"
  echo "${KEY}=${VALUE}"
else
  echo "Dry run. Nothing was changed. Run again with --apply to make this change."
fi
