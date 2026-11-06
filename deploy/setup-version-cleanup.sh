#!/bin/bash
# Expire old object versions in the assets bucket (SEC-02).
#
# The bucket is versioned, and no lifecycle rule removes noncurrent versions
# outside storyvideo-work/. Two things pile up for ever:
#   - every overwrite of a key (a presigned PUT can be repeated any number of
#     times, so one account can stack versions of the same upload), and
#   - every "deleted" video, upload and thumbnail: deleting an object in a
#     versioned bucket only adds a delete marker; the data stays.
#
# This adds one rule, ExpireOldVersions, to the bucket's lifecycle:
#   - noncurrent versions are removed 30 days after they stop being current
#   - delete markers that no longer hide anything are removed
# Current objects are not touched, so shorts sources (which move to Glacier
# after 30 days and can be restored) stay as they are.
#
# put-bucket-lifecycle-configuration replaces the whole configuration, so this
# reads the current rules and writes them back with the new rule added. It
# changes nothing if the rule is already there. Dry run unless --apply.
#
# Usage:
#   bash deploy/setup-version-cleanup.sh            # show what would be written
#   bash deploy/setup-version-cleanup.sh --apply
#   bash deploy/setup-version-cleanup.sh --apply --days 14
#
# Undo: put the configuration back without the ExpireOldVersions rule. This
# script saves the configuration it replaced to /tmp/lifecycle-before.json.
#
# Not covered: expiring uploads that were never started. S3 cannot tell an
# unused upload from a finished one without a tag, so that needs a code change
# (tag the object when its job starts) and is not done here.

set -euo pipefail

REGION="ap-south-1"
BUCKET="indifferent-fun-assets"
RULE_ID="ExpireOldVersions"
DAYS=30
APPLY=false

fail() { echo "ERROR: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --apply) APPLY=true ;;
    --days) shift; DAYS="${1:-}" ;;
    *) fail "unknown argument: $1" ;;
  esac
  shift
done
case "$DAYS" in '' | *[!0-9]*) fail "--days must be a whole number" ;; esac
[ "$DAYS" -ge 1 ] || fail "--days must be at least 1"

VERSIONING=$(aws s3api get-bucket-versioning --bucket "$BUCKET" --region "$REGION" --query Status --output text)
[ "$VERSIONING" = "Enabled" ] || fail "versioning on ${BUCKET} is '${VERSIONING}', expected Enabled; nothing to clean"

aws s3api get-bucket-lifecycle-configuration --bucket "$BUCKET" --region "$REGION" --output json > /tmp/lifecycle-before.json

NEW=$(DAYS="$DAYS" RULE_ID="$RULE_ID" python3 - <<'EOF'
import json, os
cfg = json.load(open("/tmp/lifecycle-before.json"))
rid = os.environ["RULE_ID"]
if any(r.get("ID") == rid for r in cfg["Rules"]):
    print("EXISTS")
else:
    cfg["Rules"].append({
        "ID": rid,
        "Filter": {"Prefix": ""},
        "Status": "Enabled",
        "Expiration": {"ExpiredObjectDeleteMarker": True},
        "NoncurrentVersionExpiration": {"NoncurrentDays": int(os.environ["DAYS"])},
    })
    print(json.dumps({"Rules": cfg["Rules"]}))
EOF
)
# The minimum object size for transitions is a separate setting on the call,
# not part of the rules. Passing it back keeps the current behaviour.
MIN_SIZE=$(python3 -c 'import json; print(json.load(open("/tmp/lifecycle-before.json")).get("TransitionDefaultMinimumObjectSize", ""))')

if [ "$NEW" = "EXISTS" ]; then
  echo "Rule ${RULE_ID} is already in ${BUCKET}'s lifecycle. Nothing to do."
  exit 0
fi

echo "Bucket ${BUCKET}: adding rule ${RULE_ID} (noncurrent versions expire after ${DAYS} days; stray delete markers removed)."
echo "Existing rules kept: $(python3 -c 'import json; print(", ".join(r["ID"] for r in json.load(open("/tmp/lifecycle-before.json"))["Rules"]))')"
if [ "$APPLY" != true ]; then
  echo
  echo "Dry run. Nothing was changed. Run again with --apply to make this change."
  exit 0
fi

echo "$NEW" > /tmp/lifecycle-after.json
SIZE_ARGS=()
[ -z "$MIN_SIZE" ] || SIZE_ARGS=(--transition-default-minimum-object-size "$MIN_SIZE")
aws s3api put-bucket-lifecycle-configuration --bucket "$BUCKET" --region "$REGION" \
  --lifecycle-configuration file:///tmp/lifecycle-after.json ${SIZE_ARGS[@]+"${SIZE_ARGS[@]}"}
echo "Done. Previous configuration saved in /tmp/lifecycle-before.json."
aws s3api get-bucket-lifecycle-configuration --bucket "$BUCKET" --region "$REGION" \
  --query 'Rules[].ID' --output text
