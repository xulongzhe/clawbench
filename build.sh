#!/usr/bin/env bash
set -e

NAME="clawbench"
DIST="dist"
ASSETS="assets"

# Parse arguments
TARGET_OS=""
TARGET_ARCH=""
for arg in "$@"; do
    case "$arg" in
        --windows)
            TARGET_OS="windows"
            TARGET_ARCH="amd64"
            ;;
        --linux)
            TARGET_OS="linux"
            TARGET_ARCH="amd64"
            ;;
        --darwin)
            TARGET_OS="darwin"
            TARGET_ARCH="arm64"
            ;;
        --target=*)
            TARGET="${arg#--target=}"
            TARGET_OS="${TARGET%%/*}"
            TARGET_ARCH="${TARGET##*/}"
            ;;
    esac
done

echo "=== Building $NAME ==="

# 1. Build Go backend
echo "[1/2] Building Go backend..."
if command -v go >/dev/null 2>&1; then
    if [ -n "$TARGET_OS" ] && [ -n "$TARGET_ARCH" ]; then
        BINARY_NAME="$NAME"
        if [ "$TARGET_OS" = "windows" ]; then
            BINARY_NAME="${NAME}.exe"
        fi
        GOOS=$TARGET_OS GOARCH=$TARGET_ARCH go build -o "$BINARY_NAME" ./cmd/server
        echo "  Cross-compiled: $BINARY_NAME ($TARGET_OS/$TARGET_ARCH)"
    else
        go build -o "$NAME" ./cmd/server
        echo "  Go binary: ./$NAME"
    fi
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
if [ -n "$TARGET_OS" ] && [ -n "$TARGET_ARCH" ]; then
    BINARY_NAME="$NAME"
    [ "$TARGET_OS" = "windows" ] && BINARY_NAME="${NAME}.exe"
    echo "  ./$BINARY_NAME       # Go binary ($TARGET_OS/$TARGET_ARCH)"
else
    echo "  ./$NAME              # Go binary"
fi
echo "  public/              # Frontend (if built)"
echo ""
echo "Run with: ./$NAME"
echo ""
echo "Cross-compile targets:"
echo "  ./build.sh --windows    # Windows amd64"
echo "  ./build.sh --linux      # Linux amd64"
echo "  ./build.sh --darwin     # macOS arm64"
echo "  ./build.sh --target=darwin/arm64"
