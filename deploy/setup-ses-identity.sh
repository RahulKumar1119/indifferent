#!/bin/bash
set -euo pipefail

# SES sending identity for indifferent.fun
# ========================================
#
# Verifies indifferent.fun in Amazon SES (ap-south-1) with Easy DKIM and adds
# the DNS records in Route 53:
#   - three DKIM CNAMEs  <token>._domainkey.indifferent.fun
#   - a DMARC TXT record _dmarc.indifferent.fun (p=none, reports go to
#     dmarc@indifferent.fun, which ImprovMX forwards; see setup-support-mail.sh)
#
# Nothing is sent by this script. It only makes indifferent.fun usable as a
# "From" domain, for example notifications@indifferent.fun.
#
# Usage:  bash deploy/setup-ses-identity.sh
#
# Safety:
#   - Idempotent: an existing identity and matching DNS records are reused.
#   - Only adds records under _domainkey and _dmarc. It does not touch the apex
#     MX or SPF records used for support mail.
#   - If a _dmarc record already exists with a different value, it is left
#     alone and a warning is printed.
#
# Undo:
#   aws sesv2 delete-email-identity --email-identity indifferent.fun --region ap-south-1
#   then delete the three DKIM CNAMEs and the _dmarc TXT record in Route 53.

REGION="ap-south-1"
HOSTED_ZONE_ID="Z07057693GQ8ZIMN53CHD"
DOMAIN="indifferent.fun"
DMARC_VALUE="v=DMARC1; p=none; rua=mailto:dmarc@${DOMAIN}"

echo "=== SES identity for $DOMAIN ($REGION) ==="
if IDENTITY=$(aws sesv2 get-email-identity --email-identity "$DOMAIN" --region "$REGION" --output json 2>/tmp/ses-identity-err); then
  echo "Identity already exists, reusing it"
elif grep -q "NotFoundException" /tmp/ses-identity-err; then
  IDENTITY=$(aws sesv2 create-email-identity \
    --email-identity "$DOMAIN" \
    --dkim-signing-attributes NextSigningKeyLength=RSA_2048_BIT \
    --region "$REGION" --output json)
  echo "Identity created"
else
  cat /tmp/ses-identity-err >&2
  rm -f /tmp/ses-identity-err
  exit 1
fi
rm -f /tmp/ses-identity-err

echo "=== Reading current _dmarc record ==="
EXISTING=$(aws route53 list-resource-record-sets \
  --hosted-zone-id "$HOSTED_ZONE_ID" \
  --query "ResourceRecordSets[?Name=='_dmarc.${DOMAIN}.' && Type=='TXT']" \
  --output json)

BATCH=$(DOMAIN="$DOMAIN" DMARC_VALUE="$DMARC_VALUE" IDENTITY="$IDENTITY" EXISTING="$EXISTING" python3 - <<'PY'
import json
import os
import sys

domain = os.environ["DOMAIN"]
want_dmarc = '"%s"' % os.environ["DMARC_VALUE"]
tokens = json.loads(os.environ["IDENTITY"])["DkimAttributes"]["Tokens"]
existing = json.loads(os.environ["EXISTING"])

if len(tokens) != 3:
    sys.exit("Expected 3 DKIM tokens, got %d" % len(tokens))

changes = []
for t in tokens:
    changes.append({
        "Action": "UPSERT",
        "ResourceRecordSet": {
            "Name": "%s._domainkey.%s" % (t, domain),
            "Type": "CNAME",
            "TTL": 300,
            "ResourceRecords": [{"Value": "%s.dkim.amazonses.com" % t}],
        },
    })

have = [v["Value"] for r in existing for v in r["ResourceRecords"]]
if have == [want_dmarc]:
    print("DMARC: already set, no change", file=sys.stderr)
elif have:
    print("WARNING: _dmarc already has %s; leaving it unchanged" % have, file=sys.stderr)
else:
    changes.append({
        "Action": "UPSERT",
        "ResourceRecordSet": {
            "Name": "_dmarc.%s" % domain,
            "Type": "TXT",
            "TTL": 300,
            "ResourceRecords": [{"Value": want_dmarc}],
        },
    })
    print("DMARC: will add %s" % want_dmarc, file=sys.stderr)

print(json.dumps({"Changes": changes}))
PY
)

echo "=== Applying DNS records ==="
CHANGE_ID=$(aws route53 change-resource-record-sets \
  --hosted-zone-id "$HOSTED_ZONE_ID" \
  --change-batch "$BATCH" \
  --query 'ChangeInfo.Id' --output text)
aws route53 wait resource-record-sets-changed --id "$CHANGE_ID"
echo "Route 53 change applied: $CHANGE_ID"

echo "=== Waiting for SES to verify DKIM (up to 5 minutes) ==="
STATUS="PENDING"
for _ in $(seq 1 30); do
  STATUS=$(aws sesv2 get-email-identity --email-identity "$DOMAIN" --region "$REGION" \
    --query 'DkimAttributes.Status' --output text)
  if [ "$STATUS" = "SUCCESS" ]; then
    break
  fi
  sleep 10
done

aws sesv2 get-email-identity --email-identity "$DOMAIN" --region "$REGION" \
  --query '{Identity:IdentityType,VerifiedForSending:VerifiedForSendingStatus,Dkim:DkimAttributes.Status,SigningEnabled:DkimAttributes.SigningEnabled}' \
  --output table

if [ "$STATUS" != "SUCCESS" ]; then
  echo "DKIM is still $STATUS. DNS can take longer; re-run this script later to check again."
  exit 2
fi
echo "Done: $DOMAIN can now send from SES."
