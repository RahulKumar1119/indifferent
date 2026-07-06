#!/bin/bash
set -euo pipefail

# Custom Domain Setup for api.indifferent.fun
# ============================================
#
# STATUS: COMPLETE (2026-07-05)
#
# This script documents the full custom domain setup for the API Gateway.
# All steps have been executed successfully.
#
# Resources created:
#   - Route 53 Hosted Zone: Z07057693GQ8ZIMN53CHD (indifferent.fun)
#   - ACM Certificate: arn:aws:acm:ap-south-1:438097524343:certificate/d99b1557-2bc2-43fa-a600-a4a2dd0c3ad8
#   - API Gateway Custom Domain: api.indifferent.fun
#   - API Gateway Regional Domain: d-tavtc71gq4.execute-api.ap-south-1.amazonaws.com
#   - Base Path Mapping: / -> mr4uyoffe5 (prod stage)
#   - Route 53 A Record Alias: api.indifferent.fun -> d-tavtc71gq4.execute-api.ap-south-1.amazonaws.com
#
# DNS Propagation:
#   The A record alias may take a few minutes to propagate globally.
#   Test with: curl https://api.indifferent.fun/health

REGION="ap-south-1"
ACCOUNT_ID="438097524343"
DOMAIN="api.indifferent.fun"
API_ID="mr4uyoffe5"
HOSTED_ZONE_ID="Z07057693GQ8ZIMN53CHD"
CERT_ARN="arn:aws:acm:ap-south-1:438097524343:certificate/d99b1557-2bc2-43fa-a600-a4a2dd0c3ad8"

# ============================================
# Step 1: Request ACM Certificate (DONE)
# ============================================
request_certificate() {
  echo "=== Requesting ACM Certificate for $DOMAIN ==="
  CERT_ARN=$(aws acm request-certificate \
    --domain-name "$DOMAIN" \
    --validation-method DNS \
    --region $REGION \
    --query 'CertificateArn' --output text)
  echo "Certificate ARN: $CERT_ARN"
}

# ============================================
# Step 2: Add DNS Validation Record (DONE)
# ============================================
add_validation_record() {
  echo "=== Getting DNS validation record ==="
  VALIDATION=$(aws acm describe-certificate \
    --certificate-arn "$CERT_ARN" \
    --region $REGION \
    --query 'Certificate.DomainValidationOptions[0].ResourceRecord' \
    --output json)

  CNAME_NAME=$(echo "$VALIDATION" | python3 -c "import sys,json; print(json.load(sys.stdin)['Name'])")
  CNAME_VALUE=$(echo "$VALIDATION" | python3 -c "import sys,json; print(json.load(sys.stdin)['Value'])")

  echo "Adding CNAME: $CNAME_NAME -> $CNAME_VALUE"

  aws route53 change-resource-record-sets \
    --hosted-zone-id "$HOSTED_ZONE_ID" \
    --change-batch "{
      \"Changes\": [{
        \"Action\": \"UPSERT\",
        \"ResourceRecordSet\": {
          \"Name\": \"$CNAME_NAME\",
          \"Type\": \"CNAME\",
          \"TTL\": 300,
          \"ResourceRecords\": [{\"Value\": \"$CNAME_VALUE\"}]
        }
      }]
    }" --region $REGION
}

# ============================================
# Step 3: Wait for Certificate Validation (DONE)
# ============================================
wait_for_certificate() {
  echo "=== Waiting for certificate validation ==="
  aws acm wait certificate-validated \
    --certificate-arn "$CERT_ARN" \
    --region $REGION
  echo "Certificate validated!"
}

# ============================================
# Step 4: Create API Gateway Custom Domain (DONE)
# ============================================
create_custom_domain() {
  echo "=== Creating API Gateway custom domain ==="
  DOMAIN_INFO=$(aws apigateway create-domain-name \
    --domain-name "$DOMAIN" \
    --regional-certificate-arn "$CERT_ARN" \
    --endpoint-configuration "types=REGIONAL" \
    --region $REGION \
    --output json)

  REGIONAL_DOMAIN=$(echo "$DOMAIN_INFO" | python3 -c "import sys,json; print(json.load(sys.stdin)['regionalDomainName'])")
  REGIONAL_ZONE_ID=$(echo "$DOMAIN_INFO" | python3 -c "import sys,json; print(json.load(sys.stdin)['regionalHostedZoneId'])")

  echo "Regional domain: $REGIONAL_DOMAIN"
  echo "Regional hosted zone: $REGIONAL_ZONE_ID"
}

# ============================================
# Step 5: Create Base Path Mapping (DONE)
# ============================================
create_base_path_mapping() {
  echo "=== Creating base path mapping ==="
  aws apigateway create-base-path-mapping \
    --domain-name "$DOMAIN" \
    --rest-api-id "$API_ID" \
    --stage prod \
    --region $REGION
  echo "Base path mapping created: $DOMAIN -> $API_ID/prod"
}

# ============================================
# Step 6: Create Route 53 A Record Alias (DONE)
# ============================================
create_alias_record() {
  local REGIONAL_DOMAIN="d-tavtc71gq4.execute-api.ap-south-1.amazonaws.com"
  local REGIONAL_ZONE_ID="Z3VO1THU9YC4UR"

  echo "=== Creating Route 53 A record alias ==="
  aws route53 change-resource-record-sets \
    --hosted-zone-id "$HOSTED_ZONE_ID" \
    --change-batch "{
      \"Changes\": [{
        \"Action\": \"UPSERT\",
        \"ResourceRecordSet\": {
          \"Name\": \"$DOMAIN.\",
          \"Type\": \"A\",
          \"AliasTarget\": {
            \"HostedZoneId\": \"$REGIONAL_ZONE_ID\",
            \"DNSName\": \"$REGIONAL_DOMAIN\",
            \"EvaluateTargetHealth\": true
          }
        }
      }]
    }" --region $REGION
  echo "A record alias created!"
}

# ============================================
# Verification
# ============================================
verify() {
  echo "=== Verifying setup ==="
  echo ""
  echo "Certificate status:"
  aws acm describe-certificate \
    --certificate-arn "$CERT_ARN" \
    --region $REGION \
    --query 'Certificate.Status' --output text

  echo ""
  echo "Custom domain status:"
  aws apigateway get-domain-name \
    --domain-name "$DOMAIN" \
    --region $REGION \
    --query '{Status:domainNameStatus,RegionalDomain:regionalDomainName}' --output json

  echo ""
  echo "DNS test (may take a few minutes to propagate):"
  dig +short "$DOMAIN" || echo "(dig not available, try: nslookup $DOMAIN)"

  echo ""
  echo "HTTP test:"
  curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" "https://$DOMAIN/health" || echo "(DNS may not have propagated yet)"
}

# ============================================
# Main
# ============================================
case "${1:-verify}" in
  all)
    request_certificate
    add_validation_record
    wait_for_certificate
    create_custom_domain
    create_base_path_mapping
    create_alias_record
    verify
    ;;
  verify)
    verify
    ;;
  *)
    echo "Usage: $0 [all|verify]"
    echo "  all    - Run full setup (only needed if starting from scratch)"
    echo "  verify - Check current status (default)"
    ;;
esac
