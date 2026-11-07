#!/usr/bin/env bash
# Build backend/Dockerfile and push to ECR.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

load_params
require_cmd aws docker

ECR_REPOSITORY="${ECR_REPOSITORY:-chat-analytics-backend}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
REPO_URI="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPOSITORY}"

echo "==> Ensuring ECR repository exists: ${ECR_REPOSITORY}"
aws ecr describe-repositories --repository-names "$ECR_REPOSITORY" --region "$AWS_REGION" >/dev/null 2>&1 \
  || aws ecr create-repository --repository-name "$ECR_REPOSITORY" --region "$AWS_REGION" >/dev/null

echo "==> Logging in to ECR"
aws ecr get-login-password --region "$AWS_REGION" \
  | docker login --username AWS --password-stdin "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

echo "==> Building ${REPO_URI}:${IMAGE_TAG}"
docker build -t "${ECR_REPOSITORY}:${IMAGE_TAG}" -f "$ROOT_DIR/backend/Dockerfile" "$ROOT_DIR/backend"
docker tag "${ECR_REPOSITORY}:${IMAGE_TAG}" "${REPO_URI}:${IMAGE_TAG}"
docker tag "${ECR_REPOSITORY}:${IMAGE_TAG}" "${REPO_URI}:latest"

echo "==> Pushing"
docker push "${REPO_URI}:${IMAGE_TAG}"
docker push "${REPO_URI}:latest"

echo "Pushed ${REPO_URI}:${IMAGE_TAG}"
