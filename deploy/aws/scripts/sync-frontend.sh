#!/usr/bin/env bash
# Build the Vite app for production URLs and sync to S3 + invalidate CloudFront.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

load_params
require_cmd aws npm

: "${API_URL:?}"
: "${WS_URL:?}"
: "${S3_BUCKET:?}"
: "${CLOUDFRONT_DISTRIBUTION_ID:?}"

FRONTEND_DIR="$ROOT_DIR/frontend"

echo "==> Installing frontend deps"
(cd "$FRONTEND_DIR" && npm ci)

echo "==> Building with VITE_API_URL=${API_URL} VITE_WS_URL=${WS_URL}"
(cd "$FRONTEND_DIR" && \
  VITE_API_URL="$API_URL" \
  VITE_WS_URL="$WS_URL" \
  npm run build)

echo "==> Syncing dist/ → s3://${S3_BUCKET}"
aws s3 sync "$FRONTEND_DIR/dist/" "s3://${S3_BUCKET}/" \
  --region "$AWS_REGION" \
  --delete \
  --cache-control "public,max-age=31536000,immutable" \
  --exclude "index.html" \
  --exclude "*.html"

# HTML should revalidate so clients pick up new hashed asset names.
aws s3 sync "$FRONTEND_DIR/dist/" "s3://${S3_BUCKET}/" \
  --region "$AWS_REGION" \
  --exclude "*" \
  --include "*.html" \
  --cache-control "public,max-age=0,must-revalidate" \
  --content-type "text/html"

echo "==> Invalidating CloudFront ${CLOUDFRONT_DISTRIBUTION_ID}"
INVALIDATION_ID=$(aws cloudfront create-invalidation \
  --distribution-id "$CLOUDFRONT_DISTRIBUTION_ID" \
  --paths "/*" \
  --query 'Invalidation.Id' \
  --output text)

echo "Invalidation ${INVALIDATION_ID} submitted"
echo "Frontend sync complete."
