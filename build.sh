#!/usr/bin/env bash
set -e

NAME="clawbench"
DIST="dist"
ASSETS="assets"

# Parse arguments
TARGET_OS=""
TARGET_ARCH=""
BUILD_ANDROID=""
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
        --darwin-amd64)
            TARGET_OS="darwin"
            TARGET_ARCH="amd64"
            ;;
        --target=*)
            TARGET="${arg#--target=}"
            TARGET_OS="${TARGET%%/*}"
            TARGET_ARCH="${TARGET##*/}"
            ;;
        --android)
            BUILD_ANDROID=1
            ;;
    esac
done

echo "=== Building $NAME ==="

# Derive version from git (e.g. v1.0.0, v0.30.0-30-g830bb6c, or short SHA)
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS="-X clawbench/internal/version.Version=$VERSION"
# Derive versionCode from git commit count (monotonically increasing for Play Store)
VERSION_CODE=$(git rev-list --count HEAD 2>/dev/null || echo "1")
echo "  Version: $VERSION (code: $VERSION_CODE)"

# 1. Build Go backend
echo "[1/3] Building Go backend..."
if command -v go >/dev/null 2>&1; then
    if [ -n "$TARGET_OS" ] && [ -n "$TARGET_ARCH" ]; then
        BINARY_NAME="$NAME"
        if [ "$TARGET_OS" = "windows" ]; then
            BINARY_NAME="${NAME}.exe"
        fi
        GOOS=$TARGET_OS GOARCH=$TARGET_ARCH go build -ldflags "$LDFLAGS" -o "$BINARY_NAME" ./cmd/server
        echo "  Cross-compiled: $BINARY_NAME ($TARGET_OS/$TARGET_ARCH)"
    else
        go build -ldflags "$LDFLAGS" -o "$NAME" ./cmd/server
        echo "  Go binary: ./$NAME"
    fi
else
    echo "  Go not found, skipping backend build"
fi

# 2. Build Vue frontend
echo "[2/3] Building Vue frontend..."
if [ -f "package.json" ] && command -v npm >/dev/null 2>&1; then
    if [ ! -d "node_modules" ]; then
        echo "  Installing dependencies..."
        npm install
    fi
    # Clean stale hashed assets before rebuild (index-*.js, index-*.css, manifest-*.json)
    find public/ -maxdepth 1 -name 'index-*.js' -o -name 'index-*.css' -o -name 'manifest-*.json' | xargs rm -f 2>/dev/null || true
    npm run build
    echo "  Frontend: public/"
else
    echo "  npm not found or no package.json, skipping frontend build"
fi

# 3. Build Android APK (optional)
if [ -n "$BUILD_ANDROID" ]; then
    echo "[3/3] Building Android APK..."
    if [ -d "android" ] && [ -f "android/gradlew" ]; then
        (cd android && JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64 ./gradlew assembleRelease \
            -PversionCode=$VERSION_CODE -PversionName=$VERSION)
        echo "  APK: android/app/build/outputs/apk/release/clawbench-android.apk"
    else
        echo "  Android project not found, skipping APK build"
    fi
else
    echo "[3/3] Android APK skipped (use --android to build)"
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
echo "  ./build.sh --windows        # Windows amd64"
echo "  ./build.sh --linux          # Linux amd64"
echo "  ./build.sh --darwin         # macOS arm64 (Apple Silicon)"
echo "  ./build.sh --darwin-amd64   # macOS amd64 (Intel)"
echo "  ./build.sh --target=darwin/arm64"
echo "  ./build.sh --android          # Android APK (release)"
