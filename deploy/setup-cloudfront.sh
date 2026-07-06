#!/bin/bash
# Setup CloudFront + S3 for hosting Angular frontend at indifferent.fun
# Run from the project root directory
#
# Created resources:
#   S3 Bucket: indifferent-fun-frontend (ap-south-1)
#   ACM Certificate: arn:aws:acm:us-east-1:438097524343:certificate/c1e18293-3967-4ab2-b65a-aa1b4db77d2f
#   CloudFront OAC: E3FWF3BTXNES9J
#   CloudFront Distribution: E8T1WZPS2A2JW (d190xi4bdc55hh.cloudfront.net)
#   Route 53 A Record: indifferent.fun -> CloudFront

set -euo pipefail

REGION="ap-south-1"
BUCKET_NAME="indifferent-fun-frontend"
DOMAIN="indifferent.fun"
HOSTED_ZONE_ID="Z07057693GQ8ZIMN53CHD"
ACCOUNT_ID="438097524343"
CLOUDFRONT_HOSTED_ZONE_ID="Z2FDTNDATAQYW2"

echo "=== CloudFront + S3 Frontend Setup for $DOMAIN ==="

# Step 1: Create S3 bucket
echo ""
echo "--- Step 1: Creating S3 bucket ---"
aws s3api create-bucket \
  --bucket "$BUCKET_NAME" \
  --region "$REGION" \
  --create-bucket-configuration LocationConstraint="$REGION" 2>/dev/null || echo "Bucket already exists"

aws s3api put-public-access-block \
  --bucket "$BUCKET_NAME" \
  --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true \
  --region "$REGION"
echo "S3 bucket configured with public access blocked"

# Step 2: Request ACM certificate in us-east-1 (required for CloudFront)
echo ""
echo "--- Step 2: Requesting ACM certificate ---"
CERT_ARN=$(aws acm request-certificate \
  --domain-name "$DOMAIN" \
  --subject-alternative-names "*.$DOMAIN" \
  --validation-method DNS \
  --region us-east-1 \
  --query 'CertificateArn' --output text)
echo "Certificate ARN: $CERT_ARN"

# Step 3: Get DNS validation records and add to Route 53
echo ""
echo "--- Step 3: Adding DNS validation records ---"
sleep 5
VALIDATION=$(aws acm describe-certificate \
  --certificate-arn "$CERT_ARN" \
  --region us-east-1 \
  --query 'Certificate.DomainValidationOptions[0].ResourceRecord' \
  --output json)

CNAME_NAME=$(echo "$VALIDATION" | python3 -c "import sys,json; print(json.load(sys.stdin)['Name'])")
CNAME_VALUE=$(echo "$VALIDATION" | python3 -c "import sys,json; print(json.load(sys.stdin)['Value'])")

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
  }"
echo "DNS validation record added"

# Step 4: Wait for certificate validation
echo ""
echo "--- Step 4: Waiting for certificate validation (this may take a few minutes) ---"
aws acm wait certificate-validated --certificate-arn "$CERT_ARN" --region us-east-1
echo "Certificate validated!"

# Step 5: Create CloudFront Origin Access Control
echo ""
echo "--- Step 5: Creating OAC ---"
OAC_ID=$(aws cloudfront create-origin-access-control \
  --origin-access-control-config "{
    \"Name\": \"${BUCKET_NAME}-oac\",
    \"Description\": \"OAC for frontend S3 bucket\",
    \"SigningProtocol\": \"sigv4\",
    \"SigningBehavior\": \"always\",
    \"OriginAccessControlOriginType\": \"s3\"
  }" \
  --query 'OriginAccessControl.Id' --output text)
echo "OAC ID: $OAC_ID"

# Step 6: Create CloudFront distribution
echo ""
echo "--- Step 6: Creating CloudFront distribution ---"
CALLER_REF="${BUCKET_NAME}-$(date +%s)"

cat > /tmp/cloudfront-config.json << EOF
{
  "CallerReference": "$CALLER_REF",
  "Aliases": {
    "Quantity": 1,
    "Items": ["$DOMAIN"]
  },
  "DefaultRootObject": "index.html",
  "Origins": {
    "Quantity": 1,
    "Items": [
      {
        "Id": "S3-${BUCKET_NAME}",
        "DomainName": "${BUCKET_NAME}.s3.${REGION}.amazonaws.com",
        "OriginAccessControlId": "$OAC_ID",
        "S3OriginConfig": {
          "OriginAccessIdentity": ""
        }
      }
    ]
  },
  "DefaultCacheBehavior": {
    "TargetOriginId": "S3-${BUCKET_NAME}",
    "ViewerProtocolPolicy": "redirect-to-https",
    "AllowedMethods": {
      "Quantity": 2,
      "Items": ["GET", "HEAD"],
      "CachedMethods": {
        "Quantity": 2,
        "Items": ["GET", "HEAD"]
      }
    },
    "Compress": true,
    "ForwardedValues": {
      "QueryString": false,
      "Cookies": {"Forward": "none"}
    },
    "MinTTL": 0,
    "DefaultTTL": 86400,
    "MaxTTL": 31536000
  },
  "CustomErrorResponses": {
    "Quantity": 1,
    "Items": [
      {
        "ErrorCode": 403,
        "ResponsePagePath": "/index.html",
        "ResponseCode": "200",
        "ErrorCachingMinTTL": 300
      }
    ]
  },
  "Comment": "$DOMAIN frontend",
  "Enabled": true,
  "ViewerCertificate": {
    "ACMCertificateArn": "$CERT_ARN",
    "SSLSupportMethod": "sni-only",
    "MinimumProtocolVersion": "TLSv1.2_2021"
  },
  "HttpVersion": "http2and3",
  "PriceClass": "PriceClass_200"
}
EOF

DISTRIBUTION=$(aws cloudfront create-distribution \
  --distribution-config file:///tmp/cloudfront-config.json \
  --output json)

DIST_ID=$(echo "$DISTRIBUTION" | python3 -c "import sys,json; print(json.load(sys.stdin)['Distribution']['Id'])")
DIST_DOMAIN=$(echo "$DISTRIBUTION" | python3 -c "import sys,json; print(json.load(sys.stdin)['Distribution']['DomainName'])")
echo "Distribution ID: $DIST_ID"
echo "Distribution Domain: $DIST_DOMAIN"

# Step 7: Add S3 bucket policy for CloudFront OAC
echo ""
echo "--- Step 7: Configuring bucket policy ---"
cat > /tmp/bucket-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowCloudFrontServicePrincipalReadOnly",
      "Effect": "Allow",
      "Principal": {
        "Service": "cloudfront.amazonaws.com"
      },
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::${BUCKET_NAME}/*",
      "Condition": {
        "StringEquals": {
          "AWS:SourceArn": "arn:aws:cloudfront::${ACCOUNT_ID}:distribution/$DIST_ID"
        }
      }
    }
  ]
}
EOF

aws s3api put-bucket-policy \
  --bucket "$BUCKET_NAME" \
  --policy file:///tmp/bucket-policy.json \
  --region "$REGION"
echo "Bucket policy applied"

# Step 8: Create Route 53 A record alias
echo ""
echo "--- Step 8: Creating Route 53 A record ---"
aws route53 change-resource-record-sets \
  --hosted-zone-id "$HOSTED_ZONE_ID" \
  --change-batch "{
    \"Changes\": [{
      \"Action\": \"UPSERT\",
      \"ResourceRecordSet\": {
        \"Name\": \"$DOMAIN\",
        \"Type\": \"A\",
        \"AliasTarget\": {
          \"HostedZoneId\": \"$CLOUDFRONT_HOSTED_ZONE_ID\",
          \"DNSName\": \"$DIST_DOMAIN\",
          \"EvaluateTargetHealth\": false
        }
      }
    }]
  }"
echo "Route 53 A record created"

# Step 9: Build and deploy frontend
echo ""
echo "--- Step 9: Building and deploying frontend ---"
cd frontend
npx ng build --configuration=production

aws s3 sync dist/frontend/browser/ "s3://${BUCKET_NAME}/" \
  --delete \
  --cache-control "public, max-age=31536000, immutable" \
  --exclude "index.html" \
  --region "$REGION"

aws s3 cp dist/frontend/browser/index.html "s3://${BUCKET_NAME}/index.html" \
  --cache-control "no-cache, no-store, must-revalidate" \
  --region "$REGION"
echo "Frontend deployed to S3"

# Summary
echo ""
echo "=== Setup Complete ==="
echo "S3 Bucket:              $BUCKET_NAME"
echo "ACM Certificate ARN:    $CERT_ARN"
echo "OAC ID:                 $OAC_ID"
echo "CloudFront Dist ID:     $DIST_ID"
echo "CloudFront Domain:      $DIST_DOMAIN"
echo "Custom Domain:          https://$DOMAIN"
echo ""
echo "Note: CloudFront distribution may take 5-15 minutes to fully deploy."
echo "Check status: aws cloudfront get-distribution --id $DIST_ID --query 'Distribution.Status'"
