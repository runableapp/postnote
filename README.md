# Go Indicator Stickynotes

Go Indicator Stickynotes is a modern rewrite of the original [Indicator Stickynotes](https://launchpad.net/indicator-stickynotes) Python application, written in Go for better performance and Wayland compatibility.

## About

This is a Go-based rewrite of the original Python GTK3 application. The design, color scheme, window layout, and icons are reused from the original Indicator Stickynotes by Umang Varma.

**Original Python version:** Written by Umang Varma  
**Go version:** Developed with AI for Linux on Wayland

## Features

- System tray indicator for sticky notes
- Multiple notes with category support
- Customizable colors and fonts per category
- Lock/unlock notes
- Export/import note data
- Keyboard shortcuts (Ctrl+W: Delete, Ctrl+L: Lock, Ctrl+N: New)

## Wayland Window Position Support

The application supports saving and restoring window positions on Wayland when using the [window-calls GNOME extension](https://github.com/ickyicky/window-calls).

**With the extension installed:**
- Window positions are automatically tracked and saved
- Notes restore to their previous positions on restart
- Position updates happen in real-time as windows are moved

**Without the extension (or on X11):**
- On X11: Window positions work normally using GTK methods
- On Wayland without extension: Window positions cannot be saved (Wayland security limitation)

See [docs/WAYLAND_WINDOW_CALLS.md](docs/WAYLAND_WINDOW_CALLS.md) for detailed information about the window-calls integration.

## Requirements

### APT Packages (Ubuntu/Debian)

To build and run this application, you need the following packages:

```bash
sudo apt-get install \
    libgtk-3-dev \
    libayatana-appindicator3-dev \
    pkg-config \
    libgirepository1.0-dev \
    gobject-introspection \
    libgraphene-1.0-dev \
    wget
```

**For building:**
- `libgtk-3-dev` - GTK3 development files
- `libayatana-appindicator3-dev` - AppIndicator development files (for system tray)
- `pkg-config` - Package configuration tool
- `libgirepository1.0-dev` - GObject Introspection repository
- `gobject-introspection` - GObject Introspection tools
- `libgraphene-1.0-dev` - Graphene graphics library
- `wget` - For downloading appimagetool

**Runtime dependencies** (usually pre-installed):
- `libgtk-3-0` - GTK3 runtime library
- `libayatana-appindicator3-0.1` - AppIndicator runtime library

## Building

### Quick Start

1. **Setup build environment** (first time only):
   ```bash
   ./scripts/setup-build.sh
   ```

2. **Build the binary:**
   ```bash
   task build
   ```
   Creates: `go-indicator-stickynotes`

3. **Build the AppImage:**
   ```bash
   task appimage
   ```
   Creates: `dist/go-indicator-stickynotes-0.1a-x86_64.AppImage`

### Prerequisites

1. Install Go (version 1.18 or later)
2. Install the APT packages listed above
3. Install `go-task` for build automation:
   ```bash
   # Using snap
   sudo snap install task --classic
   
   # Or using go install
   go install github.com/go-task/task/v3/cmd/task@latest
   ```

### Setup Build Environment

**First-time setup:** After cloning the repository, run the setup script to create necessary symlinks for CGO compilation:

```bash
./setup-build.sh
```

This script will:
- Check for required packages (GTK3, AppIndicator, pkg-config, Go)
- Create build directories
- Create symlinks for pkg-config and header files (needed because the Go library expects old package names, but Ubuntu provides new names)
- Verify the setup is correct

**Note:** The setup script only needs to be run once, or after cleaning the build directory. The `task build` command will automatically run setup if needed.

### Build the Binary

Build the `go-indicator-stickynotes` binary:

```bash
task build
```

This will create the `go-indicator-stickynotes` binary (approximately 5.3MB) in the current directory.

**Alternative:** If you prefer to build without `task`:

```bash
PKG_CONFIG_PATH="$PWD/build/.pkgconfig:/usr/lib/x86_64-linux-gnu/pkgconfig" \
CGO_CFLAGS="-I$PWD/build/include" \
mkdir -p bin && go build -ldflags '-s -w' -o bin/go-indicator-stickynotes .
```

### Build the AppImage

To create a portable AppImage package (`dist/go-indicator-stickynotes-0.1a-x86_64.AppImage`):

```bash
task appimage
```

This will:
1. Build the Go binary (if not already built)
2. Create the AppDir structure
3. Copy all necessary files (UI files, CSS, icons)
4. Download `appimagetool` if needed
5. Create the final AppImage: `dist/go-indicator-stickynotes-0.1a-x86_64.AppImage` (approximately 1.8MB)

**Alternative:** Run the build script directly:

```bash
./scripts/build-appimage.sh
```

### Build Options

- `task build` - Build the `go-indicator-stickynotes` binary
- `task appimage` - Build both the binary and the AppImage package
- `task clean` - Clean build artifacts
- `task fmt` - Format Go code
- `task vet` - Run go vet
- `task lint` - Run linter (fmt + vet)
- `task test` - Run tests

### AppImage Requirements

The AppImage uses system GTK libraries (not bundled), so the target system must have:
- GTK3 runtime libraries
- Ayatana AppIndicator libraries
- Required system libraries

This keeps the AppImage size smaller but requires these libraries to be installed on the target system.

### Running the AppImage

```bash
chmod +x dist/go-indicator-stickynotes-0.1a-x86_64.AppImage
./dist/go-indicator-stickynotes-0.1a-x86_64.AppImage
```

## Running from Source

After building the binary:

```bash
./bin/go-indicator-stickynotes
```

The application will:
- Create a data file at `~/.config/indicator-stickynotes`
- Show a system tray icon
- Allow you to create and manage sticky notes

## Project Structure

```
go-indicator-stickynotes/
├── main.go                # Main application entry point
├── resources.go           # Embedded resources (UI, CSS, icons)
├── stickynotes/           # Application package
│   ├── backend.go         # Note data management
│   ├── gui.go             # GTK3 UI components
│   ├── settings.go        # Settings dialog
│   ├── window_calls.go    # Wayland window position support
│   └── info.go            # Constants and configuration
├── assets/                # UI resources (embedded into binary)
│   ├── StickyNotes.ui     # Main note window UI
│   ├── GlobalDialogs.ui   # About and Settings dialogs
│   ├── SettingsCategory.ui # Category settings UI
│   ├── style.css          # Note styling CSS
│   ├── style_global.css  # Global CSS
│   └── Icons/             # Application icons
├── scripts/               # Build and utility scripts
│   ├── build-appimage.sh  # AppImage build script
│   ├── setup-build.sh     # Build environment setup
│   ├── test-dbus.sh       # D-Bus testing script
│   └── test-window-details.sh # Window details testing
├── docs/                  # Documentation
│   ├── BUILD_EXPLANATION.md
│   ├── BUILD_CACHE_EXPLANATION.md
│   ├── DEBUGGING.md
│   ├── WAYLAND_WINDOW_CALLS.md
│   └── EMBEDDED_RESOURCES.md
├── bin/                   # Compiled binaries (generated)
│   └── go-indicator-stickynotes
├── dist/                  # Distribution artifacts (generated)
│   └── go-indicator-stickynotes-*.AppImage
├── build/                 # Build artifacts (generated)
├── tools/                 # Build tools (generated)
│   └── appimagetool-x86_64.AppImage
├── Taskfile.yml           # Build automation
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
└── README.md               # This file
```

## License

Go indicator-stickynotes is free and open-source software, released for unrestricted use. Feel free to use, modify, and distribute it as you wish.

## Credits

- **Original Python version:** Umang Varma
- **Go rewrite:** Developed with AI for Linux on Wayland
- **Design, icons, and UI:** Reused from the original Indicator Stickynotes

## Keyboard Shortcuts

- `Ctrl + W` - Delete note
- `Ctrl + L` - Lock/unlock note
- `Ctrl + N` - New note

## Known Issues

- Window positions on Wayland require the [window-calls GNOME extension](https://github.com/ickyicky/window-calls) to be installed and enabled
- "Always on top" feature is disabled on Wayland (not supported)
- Requires GTK3 and AppIndicator libraries to be installed on the system

## Debug Output

The application includes debug logging for troubleshooting window position tracking. Debug messages are prefixed with tags like:
- `[WindowCalls]` - D-Bus calls to window-calls extension
- `[onConfigure]` - Window move/resize events
- `[Properties]` - Property retrieval and updates

These debug messages help diagnose issues with window position tracking on Wayland.

## Contributing

This is a Go rewrite of the original Python application. Contributions are welcome!

## Version

Current version: **0.1a**
