#!/bin/bash
set -euo pipefail

# Inbound support mail for indifferent.fun (ImprovMX free forwarding)
# ===================================================================
#
# Adds the two MX records and the SPF TXT record that ImprovMX needs on the
# apex of indifferent.fun, so mail sent to support@indifferent.fun is
# forwarded to the owner's inbox.
#
# Values are the ones ImprovMX documents for its DNS guides
# (https://improvmx.com/guides/cloudflare). If the ImprovMX dashboard shows a
# different "Recommended Value" for SPF, change SPF_VALUE below first.
#
# Usage:
#   bash deploy/setup-support-mail.sh           # dry run: prints the change, applies nothing
#   bash deploy/setup-support-mail.sh --apply   # applies the change to Route 53
#
# Prerequisite (done in the ImprovMX dashboard, not here):
#   1. Add indifferent.fun to ImprovMX (free plan).
#   2. Create the aliases support@, dmarc@ and notifications@ pointing at the
#      owner's inbox.
#
# Safety:
#   - Idempotent: re-running when the records are already correct changes nothing.
#   - Keeps existing apex TXT values (for example google-site-verification).
#     A Route 53 UPSERT replaces the whole record set, so the script appends to
#     the current values instead of overwriting them.
#   - Refuses to run if the apex already has different MX records or a
#     different SPF record. A domain may have only one SPF record, so that case
#     needs a manual merge.
#
# Undo: delete the apex MX record set and remove the SPF value from the apex
# TXT record set.

HOSTED_ZONE_ID="Z07057693GQ8ZIMN53CHD"
DOMAIN="indifferent.fun"
MX_VALUES="10 mx1.improvmx.com|20 mx2.improvmx.com"
SPF_VALUE="v=spf1 include:spf.improvmx.com ~all"

APPLY=false
if [ "${1:-}" = "--apply" ]; then
  APPLY=true
elif [ -n "${1:-}" ]; then
  echo "Unknown argument: $1 (use --apply or no argument)" >&2
  exit 1
fi

echo "=== Reading current apex MX and TXT records for $DOMAIN ==="
RECORDS=$(aws route53 list-resource-record-sets \
  --hosted-zone-id "$HOSTED_ZONE_ID" \
  --query "ResourceRecordSets[?Name=='${DOMAIN}.' && (Type=='MX' || Type=='TXT')]" \
  --output json)

BATCH=$(DOMAIN="$DOMAIN" MX_VALUES="$MX_VALUES" SPF_VALUE="$SPF_VALUE" RECORDS="$RECORDS" python3 - <<'PY'
import json
import os
import sys

domain = os.environ["DOMAIN"]
want_mx = os.environ["MX_VALUES"].split("|")
want_spf = '"%s"' % os.environ["SPF_VALUE"]
by_type = {r["Type"]: r for r in json.loads(os.environ["RECORDS"])}


def upsert(rtype, values, ttl=300):
    return {
        "Action": "UPSERT",
        "ResourceRecordSet": {
            "Name": domain,
            "Type": rtype,
            "TTL": ttl,
            "ResourceRecords": [{"Value": v} for v in values],
        },
    }


changes = []

mx = by_type.get("MX")
if mx:
    have = [v["Value"] for v in mx["ResourceRecords"]]
    if sorted(have) == sorted(want_mx):
        print("MX: already set, no change", file=sys.stderr)
    else:
        sys.exit("MX: apex already has different records %s; refusing to overwrite" % have)
else:
    changes.append(upsert("MX", want_mx))
    print("MX: will add %s" % want_mx, file=sys.stderr)

txt = by_type.get("TXT")
if txt:
    have = [v["Value"] for v in txt["ResourceRecords"]]
    spf = [v for v in have if v.strip('"').startswith("v=spf1")]
    if spf == [want_spf]:
        print("SPF: already set, no change", file=sys.stderr)
    elif spf:
        sys.exit("SPF: apex already has a different SPF record %s; merge by hand (only one SPF record is allowed)" % spf)
    else:
        changes.append(upsert("TXT", have + [want_spf], txt.get("TTL", 300)))
        print("SPF: will append to existing TXT values (%d kept)" % len(have), file=sys.stderr)
else:
    changes.append(upsert("TXT", [want_spf]))
    print("SPF: will create TXT record", file=sys.stderr)

if changes:
    print(json.dumps({"Changes": changes}, indent=2))
PY
)

if [ -z "$BATCH" ]; then
  echo "Nothing to do: records are already configured."
  exit 0
fi

echo
echo "=== Planned Route 53 change ==="
echo "$BATCH"

if [ "$APPLY" != "true" ]; then
  echo
  echo "Dry run only. Re-run with --apply to make this change."
  exit 0
fi

echo
echo "=== Applying ==="
CHANGE_ID=$(aws route53 change-resource-record-sets \
  --hosted-zone-id "$HOSTED_ZONE_ID" \
  --change-batch "$BATCH" \
  --query 'ChangeInfo.Id' --output text)
aws route53 wait resource-record-sets-changed --id "$CHANGE_ID"

echo "Applied (change $CHANGE_ID). Verify with:"
echo "  dig +short MX $DOMAIN"
echo "  dig +short TXT $DOMAIN"
echo "Then click 'Check Again' in the ImprovMX dashboard and send a test mail to support@$DOMAIN."
