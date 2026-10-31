#!/bin/bash
set -euo pipefail
# Flush the shared stock-media cache (issue #194)
# ===============================================
# Makes every cached stock-media search and lookup stop being used, in every
# container, without a deploy. Use it when something that changes what a provider
# returns has changed, and the 24 hours Pixabay requires entries to be kept would
# otherwise hide it. The case this exists for: Pixabay approves the API key for full
# API access (https://pixabay.com/api/request_full_access/). Photos then come with a
# 1920 px link (fullHDURL), but searches already cached keep the 1280 px ones for up
# to a day, so the approval would look like a refusal.
#
# How it works: the cache stores the flush "epoch" in one item of its table
# (PK = EPOCH, attribute epoch) and hashes it into every entry's key. This script
# writes a new epoch. Entries written under the old one are never read again, and
# DynamoDB's TTL removes them later. Containers re-read the epoch once a minute, so
# a flush reaches all of them within about a minute. The previous epoch is printed so
# it can be put back (which brings the old entries back, if they have not expired).
#
# Usage:
#   bash deploy/flush-stockmedia-cache.sh            # dry run: shows the current epoch and the new one
#   bash deploy/flush-stockmedia-cache.sh --apply    # writes the new epoch
#
# Safety:
#   - Writes one item, EPOCH, in indifferent-fun-stock-cache, and nothing else. It
#     does not create the table (deploy/setup-stock-cache.sh does) and refuses if the
#     table is missing or has the wrong shape.
#   - The dry run only reads.
#   - Flushing costs the providers a few requests as the cache refills (Pixabay's
#     limit is 100 a minute). Do not run it in a loop.
#   - Needs: aws.
#
# After it, to see it took effect: the API's log shows 'stockcache_epoch_changed'
# within a minute, and a search that was cached now logs cached=false once. To check
# the Pixabay key itself, from a fresh process (an empty cache, so nothing is stale):
#   cd backend && PIXABAY_API_KEY=... go test -tags live -run TestLive_PixabayFullAccess -v ./internal/stockmedia/

APPLY=false
for arg in "$@"; do
  case "$arg" in
    "") ;;
    --apply) APPLY=true ;;
    *)
      echo "Unknown argument: $arg (use --apply, or no argument)" >&2
      exit 1
      ;;
  esac
done

REGION="ap-south-1"
PROJECT="indifferent-fun"
TABLE="${PROJECT}-stock-cache"
EPOCH_KEY="EPOCH"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

if $APPLY; then
  echo "MODE: apply (the flush epoch of ${TABLE} will be changed)"
else
  echo "MODE: dry run (read-only; nothing will be changed)"
fi
echo

TABLE_STATUS=$(aws dynamodb describe-table --table-name "$TABLE" --region "$REGION" --query 'Table.TableStatus' --output text 2>/dev/null || true)
[ "$TABLE_STATUS" = "ACTIVE" ] ||
  fail "table ${TABLE} is not active (${TABLE_STATUS:-not found}). Run deploy/setup-stock-cache.sh --apply first; until it exists there is nothing to flush."
KEYS=$(aws dynamodb describe-table --table-name "$TABLE" --region "$REGION" --query 'Table.KeySchema[].[AttributeName,KeyType]' --output text)
[ "$KEYS" = "$(printf 'PK\tHASH')" ] ||
  fail "table ${TABLE} does not have a single string hash key named PK, so it is not the stock cache. Nothing was changed."

# A consistent read, so a flush made a moment ago is seen.
CURRENT=$(aws dynamodb get-item --table-name "$TABLE" --region "$REGION" --consistent-read \
  --key "{\"PK\":{\"S\":\"${EPOCH_KEY}\"}}" --query 'Item.epoch.S' --output text)
case "$CURRENT" in
  None | "") CURRENT_SHOWN="(none: the cache has never been flushed)" ;;
  *) CURRENT_SHOWN="$CURRENT" ;;
esac
NEW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
[ "$CURRENT" != "$NEW" ] || fail "the epoch is already ${NEW} (a flush in the same second); wait a second and run it again."

echo "Current epoch: ${CURRENT_SHOWN}"
echo "New epoch:     ${NEW}"
echo

write_epoch() {
  aws dynamodb put-item --table-name "$TABLE" --region "$REGION" \
    --item "{\"PK\":{\"S\":\"${EPOCH_KEY}\"},\"epoch\":{\"S\":\"${NEW}\"}}"
}
if $APPLY; then
  echo ">> write the new epoch"
  write_epoch
  echo
  echo "Done. Every container stops using the old entries within about a minute."
  case "$CURRENT" in
    None | "") echo "To undo: the old entries were under the empty epoch; delete the EPOCH item: aws dynamodb delete-item --table-name ${TABLE} --region ${REGION} --key '{\"PK\":{\"S\":\"${EPOCH_KEY}\"}}'" ;;
    *) echo "To undo (brings the old entries back, if they have not expired): put epoch '${CURRENT}' back with the same put-item." ;;
  esac
else
  echo "[dry-run] would: write the new epoch"
  echo
  echo "Dry run complete. Re-run with --apply to flush."
fi
