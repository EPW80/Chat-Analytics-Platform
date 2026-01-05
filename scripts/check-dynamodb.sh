#!/bin/bash

echo "=========================================="
echo "DynamoDB Message Persistence Test"
echo "=========================================="
echo ""

# DynamoDB Local endpoint
ENDPOINT="http://localhost:8000"

echo "1. Listing all tables..."
aws dynamodb list-tables \
  --endpoint-url $ENDPOINT \
  --region us-east-1 \
  --no-cli-pager 2>/dev/null

echo ""
echo "2. Scanning Messages table..."
aws dynamodb scan \
  --table-name Messages \
  --endpoint-url $ENDPOINT \
  --region us-east-1 \
  --no-cli-pager 2>/dev/null | jq '.Items | length as $count | "Total messages: \($count)"'

echo ""
echo "3. Getting sample messages (first 5)..."
aws dynamodb scan \
  --table-name Messages \
  --endpoint-url $ENDPOINT \
  --region us-east-1 \
  --max-items 5 \
  --no-cli-pager 2>/dev/null | jq '.Items[] | {MessageID: .MessageID.S, UserID: .UserID.S, Username: .Username.S, Content: .Content.S, Type: .Type.S, Timestamp: .Timestamp.S}'

echo ""
echo "=========================================="
