#!/usr/bin/env bash
set -e

NAME="clawbench"
DIST="dist"
ASSETS="assets"

echo "=== Building $NAME ==="

# 1. Build Go backend
echo "[1/2] Building Go backend..."
if command -v go >/dev/null 2>&1; then
    go build -o "$NAME" ./cmd/server
    echo "  Go binary: ./$NAME"
else
    echo "  Go not found, skipping backend build"
fi

# 2. Build Vue frontend
echo "[2/2] Building Vue frontend..."
if [ -f "package.json" ] && command -v npm >/dev/null 2>&1; then
    if [ ! -d "node_modules" ]; then
        echo "  Installing dependencies..."
        npm install
    fi
    npm run build
    echo "  Frontend: public/"
else
    echo "  npm not found or no package.json, skipping frontend build"
fi

echo ""
echo "=== Build complete ==="
echo "  ./$NAME              # Go binary"
echo "  public/              # Frontend (if built)"
echo ""
echo "Run with: ./$NAME"
