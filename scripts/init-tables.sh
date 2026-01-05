#!/bin/bash
# Initialize DynamoDB tables
# This script waits for DynamoDB to be ready and then creates the required tables

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration
DYNAMODB_HOST="${DYNAMODB_HOST:-localhost}"
DYNAMODB_PORT="${DYNAMODB_PORT:-8000}"
DYNAMODB_ENDPOINT="http://$DYNAMODB_HOST:$DYNAMODB_PORT"

echo "==================================="
echo "DynamoDB Table Initialization"
echo "==================================="
echo "Endpoint: $DYNAMODB_ENDPOINT"
echo ""

# Wait for DynamoDB to be ready
echo "Step 1: Waiting for DynamoDB..."
"$SCRIPT_DIR/wait-for-dynamodb.sh" "$DYNAMODB_HOST" "$DYNAMODB_PORT"
echo ""

# Set environment variables for Go script
export DYNAMODB_ENDPOINT="$DYNAMODB_ENDPOINT"
export DYNAMODB_REGION="${DYNAMODB_REGION:-us-east-1}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-dummy}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-dummy}"

# Run the Go initialization script
echo "Step 2: Creating tables..."
cd "$PROJECT_ROOT/backend"
go run ./scripts/init-dynamodb.go

echo ""
echo "==================================="
echo "✅ Table initialization complete!"
echo "==================================="
