#!/bin/bash
# Wait for DynamoDB Local to be ready

set -e

host="${1:-localhost}"
port="${2:-8000}"
max_attempts="${3:-30}"

echo "Waiting for DynamoDB at $host:$port..."

attempt=0
while [ $attempt -lt $max_attempts ]; do
    if curl -sf "http://$host:$port/" > /dev/null 2>&1; then
        echo "DynamoDB is ready!"
        exit 0
    fi

    attempt=$((attempt + 1))
    echo "Attempt $attempt/$max_attempts: DynamoDB not ready yet..."
    sleep 2
done

echo "ERROR: DynamoDB did not become ready in time"
exit 1
