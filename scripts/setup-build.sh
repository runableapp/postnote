#!/bin/bash
# Setup script for building indicator-stickynotes
# This script creates necessary symlinks for CGO compilation with GTK3 and AppIndicator

set -e

# Get the project root directory (parent of scripts/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"

echo "Setting up build environment for indicator-stickynotes..."
echo ""

# Check if required packages are installed
echo "Checking for required packages..."

MISSING_PACKAGES=()

check_package() {
    local pkg_name=$1
    local check_cmd=$2
    
    if eval "$check_cmd" >/dev/null 2>&1; then
        echo "  ✓ $pkg_name found"
        return 0
    else
        echo "  ✗ $pkg_name NOT found"
        MISSING_PACKAGES+=("$pkg_name")
        return 1
    fi
}

# Check for pkg-config
check_package "pkg-config" "which pkg-config"

# Check for GTK3 development files
check_package "libgtk-3-dev" "pkg-config --exists gtk+-3.0"

# Check for AppIndicator development files
check_package "libayatana-appindicator3-dev" "pkg-config --exists ayatana-appindicator3-0.1"

# Check for Go
check_package "Go" "which go"

if [ ${#MISSING_PACKAGES[@]} -gt 0 ]; then
    echo ""
    echo "ERROR: Missing required packages!"
    echo ""
    echo "Please install the missing packages with:"
    echo "  sudo apt-get install \\"
    echo "    libgtk-3-dev \\"
    echo "    libayatana-appindicator3-dev \\"
    echo "    pkg-config \\"
    echo "    libgirepository1.0-dev \\"
    echo "    gobject-introspection \\"
    echo "    libgraphene-1.0-dev"
    echo ""
    echo "For Go, see: https://go.dev/doc/install"
    exit 1
fi

echo ""
echo "All required packages are installed."
echo ""

# Create build directories
echo "Creating build directories..."
mkdir -p "$BUILD_DIR/.pkgconfig"
mkdir -p "$BUILD_DIR/include/libappindicator"
echo "  ✓ Directories created"

# Create pkg-config symlink
PKG_CONFIG_SOURCE="/usr/lib/x86_64-linux-gnu/pkgconfig/ayatana-appindicator3-0.1.pc"
PKG_CONFIG_LINK="$BUILD_DIR/.pkgconfig/appindicator3-0.1.pc"

if [ ! -f "$PKG_CONFIG_SOURCE" ]; then
    echo ""
    echo "ERROR: Cannot find $PKG_CONFIG_SOURCE"
    echo "Please install libayatana-appindicator3-dev:"
    echo "  sudo apt-get install libayatana-appindicator3-dev"
    exit 1
fi

if [ -L "$PKG_CONFIG_LINK" ] || [ -f "$PKG_CONFIG_LINK" ]; then
    # Remove existing symlink or file
    rm -f "$PKG_CONFIG_LINK"
fi

ln -sf "$PKG_CONFIG_SOURCE" "$PKG_CONFIG_LINK"
echo "  ✓ Created symlink: $PKG_CONFIG_LINK -> $PKG_CONFIG_SOURCE"

# Create header file symlink
HEADER_SOURCE="/usr/include/libayatana-appindicator3-0.1/libayatana-appindicator/app-indicator.h"
HEADER_LINK="$BUILD_DIR/include/libappindicator/app-indicator.h"

if [ ! -f "$HEADER_SOURCE" ]; then
    echo ""
    echo "ERROR: Cannot find $HEADER_SOURCE"
    echo "Please install libayatana-appindicator3-dev:"
    echo "  sudo apt-get install libayatana-appindicator3-dev"
    exit 1
fi

if [ -L "$HEADER_LINK" ] || [ -f "$HEADER_LINK" ]; then
    # Remove existing symlink or file
    rm -f "$HEADER_LINK"
fi

ln -sf "$HEADER_SOURCE" "$HEADER_LINK"
echo "  ✓ Created symlink: $HEADER_LINK -> $HEADER_SOURCE"

# Verify symlinks work
echo ""
echo "Verifying setup..."

if PKG_CONFIG_PATH="$BUILD_DIR/.pkgconfig:/usr/lib/x86_64-linux-gnu/pkgconfig" pkg-config --exists appindicator3-0.1 2>/dev/null; then
    echo "  ✓ pkg-config can find appindicator3-0.1"
else
    echo "  ✗ pkg-config cannot find appindicator3-0.1"
    exit 1
fi

if [ -f "$HEADER_LINK" ]; then
    echo "  ✓ Header file symlink is valid"
else
    echo "  ✗ Header file symlink is invalid"
    exit 1
fi

echo ""
echo "=========================================="
echo "Build environment setup complete!"
echo "=========================================="
echo ""
echo "You can now build the project with:"
echo "  task build"
echo ""
echo "Or directly with:"
echo "  PKG_CONFIG_PATH=\"\$PWD/build/.pkgconfig:/usr/lib/x86_64-linux-gnu/pkgconfig\" \\"
echo "  CGO_CFLAGS=\"-I\$PWD/build/include\" \\"
echo "  go build -ldflags '-s -w' -o go-indicator-stickynotes ."
echo ""

