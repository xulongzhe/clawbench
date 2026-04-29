#!/bin/bash
# Setup script for the ClawBench Android project
# Run this once to initialize the Gradle wrapper (requires Java 17+ and Android SDK)
#
# Prerequisites:
#   1. Install Java 17+: sudo apt install openjdk-17-jdk
#   2. Install Android SDK: https://developer.android.com/studio#command-line-tools-only
#   3. Set ANDROID_HOME: export ANDROID_HOME=$HOME/Android/Sdk
#
# Usage:
#   cd android && ./setup.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== ClawBench Android Setup ==="

# Check Java
if ! command -v java &>/dev/null; then
    echo "ERROR: Java not found. Install JDK 17+ first:"
    echo "  sudo apt install openjdk-17-jdk"
    exit 1
fi

JAVA_VERSION=$(java -version 2>&1 | head -1 | cut -d'"' -f2 | cut -d'.' -f1)
echo "Java version: $JAVA_VERSION"

# Check Android SDK
if [ -z "$ANDROID_HOME" ]; then
    echo "WARNING: ANDROID_HOME not set."
    echo "  Install Android SDK command-line tools and set:"
    echo "  export ANDROID_HOME=\$HOME/Android/Sdk"
    echo ""
    echo "  Required SDK components:"
    echo "    sdkmanager 'platforms;android-34'"
    echo "    sdkmanager 'build-tools;34.0.0'"
    echo "    sdkmanager 'platform-tools'"
fi

# Generate Gradle wrapper if not present
if [ ! -f "gradle/wrapper/gradle-wrapper.jar" ]; then
    echo ""
    echo "Generating Gradle wrapper..."
    if command -v gradle &>/dev/null; then
        gradle wrapper --gradle-version 8.5
    else
        echo "Gradle CLI not found. Downloading wrapper jar directly..."
        WRAPPER_URL="https://raw.githubusercontent.com/gradle/gradle/v8.5.0/gradle/wrapper/gradle-wrapper.jar"
        curl -fsSL "$WRAPPER_URL" -o gradle/wrapper/gradle-wrapper.jar 2>/dev/null || {
            echo ""
            echo "Could not download gradle-wrapper.jar automatically."
            echo "Please install Gradle and run:"
            echo "  cd $SCRIPT_DIR && gradle wrapper --gradle-version 8.5"
            exit 1
        }
    fi
fi

# Create local.properties with SDK path
if [ -n "$ANDROID_HOME" ] && [ ! -f "local.properties" ]; then
    echo "sdk.dir=$ANDROID_HOME" > local.properties
    echo "Created local.properties with ANDROID_HOME=$ANDROID_HOME"
fi

echo ""
echo "=== Setup complete ==="
echo ""
echo "To build the debug APK:"
echo "  cd $SCRIPT_DIR && ./gradlew assembleDebug"
echo ""
echo "To install on a connected device:"
echo "  cd $SCRIPT_DIR && ./gradlew installDebug"
