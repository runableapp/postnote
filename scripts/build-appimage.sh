#!/bin/bash
# Build script for creating AppImage of indicator-stickynotes (Go version)

set -e

APP_NAME="postnote"
APP_VERSION="0.1a"
APPDIR="AppDir"
BIN_DIR="bin"
DIST_DIR="dist"
BINARY="$BIN_DIR/postnote"

echo "Building AppImage for $APP_NAME $APP_VERSION (Go version)"

# Get the project root directory (parent of scripts/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SCRIPT_DIR"

# Clean previous build
rm -rf "$APPDIR"
mkdir -p "$DIST_DIR"

# Build the Go binary first
# Note: When run via 'task appimage', 'task build' already ran as a dependency.
# This ensures the binary exists if the script is run directly.
# Go's build cache will make this fast if the binary is already up-to-date.
echo "Building Go binary..."
export PKG_CONFIG_PATH="$SCRIPT_DIR/build/.pkgconfig:/usr/lib/x86_64-linux-gnu/pkgconfig:$PKG_CONFIG_PATH"
export CGO_CFLAGS="-I$SCRIPT_DIR/build/include $CGO_CFLAGS"
task build 2>/dev/null || (mkdir -p "$BIN_DIR" && go build -ldflags '-s -w' -o "$BINARY" .)

# Create AppDir structure
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/indicator-stickynotes"
mkdir -p "$APPDIR/usr/share/icons/hicolor/48x48/apps"
mkdir -p "$APPDIR/usr/share/icons/hicolor/scalable/apps"

# Copy binary
echo "Copying binary..."
cp "$BINARY" "$APPDIR/usr/bin/"

# Copy application files (UI files, CSS, etc.) from assets directory
echo "Copying application files..."
cp "assets/StickyNotes.ui" "$APPDIR/usr/share/indicator-stickynotes/" 2>/dev/null || true
cp "assets/GlobalDialogs.ui" "$APPDIR/usr/share/indicator-stickynotes/" 2>/dev/null || true
cp "assets/SettingsCategory.ui" "$APPDIR/usr/share/indicator-stickynotes/" 2>/dev/null || true
cp "assets/style.css" "$APPDIR/usr/share/indicator-stickynotes/" 2>/dev/null || true
cp "assets/style_global.css" "$APPDIR/usr/share/indicator-stickynotes/" 2>/dev/null || true

# Copy icons from assets/Icons directory
echo "Copying icons..."
if [ -d "assets/Icons" ]; then
    # Try to copy hicolor theme icons if they exist
    if [ -f "assets/Icons/hicolor/48x48/apps/indicator-stickynotes.png" ]; then
        cp "assets/Icons/hicolor/48x48/apps/indicator-stickynotes.png" "$APPDIR/usr/share/icons/hicolor/48x48/apps/" 2>/dev/null || true
    fi
    if [ -f "assets/Icons/hicolor/scalable/apps/indicator-stickynotes.svg" ]; then
        cp "assets/Icons/hicolor/scalable/apps/indicator-stickynotes.svg" "$APPDIR/usr/share/icons/hicolor/scalable/apps/" 2>/dev/null || true
    fi
    # Copy main icon to AppDir root
    if [ -f "assets/Icons/indicator-stickynotes.png" ]; then
        cp "assets/Icons/indicator-stickynotes.png" "$APPDIR/" 2>/dev/null || true
    fi
    # Copy all icon files that UI files reference
    mkdir -p "$APPDIR/usr/share/indicator-stickynotes/Icons"
    cp assets/Icons/*.png "$APPDIR/usr/share/indicator-stickynotes/Icons/" 2>/dev/null || true
    cp assets/Icons/*.svg "$APPDIR/usr/share/indicator-stickynotes/Icons/" 2>/dev/null || true
fi

# Copy desktop file
if [ -f "../indicator-stickynotes.desktop" ]; then
    cp "../indicator-stickynotes.desktop" "$APPDIR/usr/share/applications/"
    cp "../indicator-stickynotes.desktop" "$APPDIR/"
else
    # Create a basic desktop file
    cat > "$APPDIR/indicator-stickynotes.desktop" << 'EOF'
[Desktop Entry]
Name=Go Indicator Stickynotes
Comment=Sticky Notes Indicator (Go version)
Exec=postnote
Icon=indicator-stickynotes
Type=Application
Categories=Utility;
EOF
    cp "$APPDIR/indicator-stickynotes.desktop" "$APPDIR/usr/share/applications/"
fi

# Create AppRun script
cat > "$APPDIR/AppRun" << 'EOF'
#!/bin/bash
# AppRun - Entry point for AppImage

# Get the directory where AppRun is located
HERE="$(dirname "$(readlink -f "${0}")")"

# Set environment variables
export PATH="${HERE}/usr/bin:${PATH}"
export XDG_DATA_DIRS="${HERE}/usr/share:${XDG_DATA_DIRS}"

# Set library path for GTK (if bundled)
if [ -d "${HERE}/usr/lib" ]; then
    export LD_LIBRARY_PATH="${HERE}/usr/lib:${LD_LIBRARY_PATH}"
fi

# Set PKG_CONFIG_PATH for AppIndicator (if needed)
if [ -d "${HERE}/usr/lib/pkgconfig" ]; then
    export PKG_CONFIG_PATH="${HERE}/usr/lib/pkgconfig:${PKG_CONFIG_PATH}"
fi

# Run the application
exec "${HERE}/usr/bin/postnote" "$@"
EOF

chmod +x "$APPDIR/AppRun"

# Download appimagetool if not present
TOOLS_DIR="$SCRIPT_DIR/tools"
mkdir -p "$TOOLS_DIR"
APPIMAGETOOL="$TOOLS_DIR/appimagetool-x86_64.AppImage"

if [ ! -f "$APPIMAGETOOL" ]; then
    echo "Downloading appimagetool..."
    wget -q -O "$APPIMAGETOOL" https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
    chmod +x "$APPIMAGETOOL"
fi

# Build AppImage
echo "Building AppImage..."
APPIMAGE_OUTPUT="$DIST_DIR/${APP_NAME}-${APP_VERSION}-x86_64.AppImage"
ARCH=x86_64 "$APPIMAGETOOL" "$APPDIR" "$APPIMAGE_OUTPUT"

echo ""
echo "✅ AppImage built successfully: $APPIMAGE_OUTPUT"
echo ""
echo "To test:"
echo "  chmod +x $APPIMAGE_OUTPUT"
echo "  ./$APPIMAGE_OUTPUT"
