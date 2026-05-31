#!/usr/bin/env bash
set -e

# One-step Docker build & run for ClawBench testing
#
# Usage:
#   ./scripts/docker-build.sh           # build + run (port 20300)
#   ./scripts/docker-build.sh --stop    # stop and remove container
#   ./scripts/docker-build.sh --clean   # stop + remove container + volume

PORT=20300
NAME="clawbench-test"

# Stop existing container
if docker ps -a --format '{{.Names}}' | grep -q "^${NAME}$"; then
    echo "Stopping existing container..."
    docker stop "$NAME" >/dev/null && docker rm "$NAME" >/dev/null
fi

# Handle --stop / --clean
if [ "$1" = "--stop" ]; then
    echo "Container stopped."
    exit 0
fi

if [ "$1" = "--clean" ]; then
    echo "Removing volume..."
    docker volume rm clawbench_clawbench-data 2>/dev/null || true
    echo "Clean complete."
    exit 0
fi

# Ensure binary is built
if [ ! -f "./clawbench" ]; then
    echo "Binary not found. Running ./build.sh..."
    ./build.sh
fi

# Prepare staging directory for Pi binary (optional)
rm -rf docker-staging
mkdir -p docker-staging/pi
if [ -d ".clawbench/pi" ] && [ -f ".clawbench/pi/pi" ]; then
    cp -r .clawbench/pi/* docker-staging/pi/
    echo "Pi binary included in image (with dependencies)"
else
    echo "Pi binary not found — setup wizard will not be available"
    echo "  (Run ./build.sh --with-pi to download it)"
fi

# Build and run via docker compose (staging dir must exist during build)
echo "Building and starting container on port ${PORT}..."
docker compose up -d --build 2>&1 | grep -v "^#" | grep -v "^$" || true

# Clean up staging (after compose build is done)
rm -rf docker-staging

# Wait for server to start
sleep 3
echo ""
echo "=== Server logs ==="
docker logs "$NAME" 2>&1 | tail -5

# Extract auto-password
echo ""
PASS=$(docker exec "$NAME" cat /data/.clawbench/auto-password 2>/dev/null || docker exec "$NAME" cat /app/.clawbench/auto-password 2>/dev/null || echo "")
if [ -n "$PASS" ]; then
    echo "╔══════════════════════════════════════╗"
    echo "║  Auto-generated password: $PASS  ║"
    echo "╚══════════════════════════════════════╝"
else
    echo "No password configured (open access)"
fi

echo ""
echo "Access: http://localhost:${PORT}"
