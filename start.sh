#!/bin/bash
# Start the Real-Time Chat Analytics Platform
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==================================="
echo "Chat Analytics Platform - Starting"
echo "==================================="

# Parse flags
SKIP_INIT=false
for arg in "$@"; do
  case $arg in
    --skip-init) SKIP_INIT=true ;;
    --help)
      echo "Usage: ./start.sh [--skip-init]"
      echo ""
      echo "  --skip-init  Skip DynamoDB table initialization (use if tables already exist)"
      exit 0
      ;;
  esac
done

# Check dependencies
for cmd in docker go; do
  if ! command -v $cmd &>/dev/null; then
    echo "ERROR: '$cmd' is required but not installed."
    exit 1
  fi
done

# Detect docker compose command (v2 plugin vs v1 standalone)
if docker compose version &>/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose &>/dev/null; then
  COMPOSE="docker-compose"
else
  echo "ERROR: Neither 'docker compose' nor 'docker-compose' is available."
  exit 1
fi

# Start Docker services
echo ""
echo "Step 1: Starting Docker services (DynamoDB + Backend)..."
cd "$SCRIPT_DIR"
$COMPOSE up -d --build

# Wait for backend health
echo ""
echo "Step 2: Waiting for backend to be healthy..."
attempt=0
max_attempts=30
until curl -sf http://localhost:8080/health > /dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ $attempt -ge $max_attempts ]; then
    echo "ERROR: Backend did not become healthy in time."
    echo "Run '$COMPOSE logs backend' to inspect."
    exit 1
  fi
  echo "  Attempt $attempt/$max_attempts..."
  sleep 3
done
echo "  Backend is healthy!"

# Initialize DynamoDB tables
if [ "$SKIP_INIT" = false ]; then
  echo ""
  echo "Step 3: Initializing DynamoDB tables..."
  "$SCRIPT_DIR/scripts/init-tables.sh"
else
  echo ""
  echo "Step 3: Skipping table initialization (--skip-init)."
fi

echo ""
echo "==================================="
echo "Platform is running!"
echo "==================================="
echo "  Backend health:  http://localhost:8080/health"
echo "  WebSocket:       ws://localhost:8080/ws"
echo "  DynamoDB Local:  http://localhost:8000"
echo ""
echo "To stop: $COMPOSE down"
echo "Logs:    $COMPOSE logs -f"
