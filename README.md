[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/G2G21R1BX8)

# PostNote

PostNote is a modern rewrite of the original [Indicator Stickynotes](https://launchpad.net/indicator-stickynotes) Python application, written in Go for better performance and Wayland compatibility.

## About

This is a Go-based rewrite of the original Python GTK3 application. The design, color scheme, window layout, and icons are reused from the original Indicator Stickynotes by Umang Varma.

**Original Python version:** Written by Umang Varma  
**Go version:** Developed for Linux on Wayland

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

See [WAYLAND_WINDOW_CALLS.md](WAYLAND_WINDOW_CALLS.md) for detailed information about the window-calls integration.

## Requirements

### APT Packages (Ubuntu/Debian)

**Runtime dependencies** (usually pre-installed):
- `libgtk-3-0` - GTK3 runtime library
- `libayatana-appindicator3-0.1` - AppIndicator runtime library

**For building:**
- `libgtk-3-dev` - GTK3 development files
- `libayatana-appindicator3-dev` - AppIndicator development files (for system tray)
- `pkg-config` - Package configuration tool
- `libgirepository1.0-dev` - GObject Introspection repository
- `gobject-introspection` - GObject Introspection tools
- `libgraphene-1.0-dev` - Graphene graphics library
- `wget` - For downloading appimagetool


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
   Creates: `bin/postnote`

3. **Build the AppImage:**
   ```bash
   task appimage
   ```
   Creates: `dist/postnote-0.1a-x86_64.AppImage`

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

Build the `postnote` binary:

```bash
task build
```

This will create the `postnote` binary (approximately 5.3MB) in the `bin/` directory.

**Alternative:** If you prefer to build without `task`:

```bash
PKG_CONFIG_PATH="$PWD/build/.pkgconfig:/usr/lib/x86_64-linux-gnu/pkgconfig" \
CGO_CFLAGS="-I$PWD/build/include" \
mkdir -p bin && go build -ldflags '-s -w' -o bin/postnote .
```

### Build the AppImage

To create a portable AppImage package (`dist/postnote-0.1a-x86_64.AppImage`):

```bash
task appimage
```

This will:
1. Build the Go binary (if not already built)
2. Create the AppDir structure
3. Copy all necessary files (UI files, CSS, icons)
4. Download `appimagetool` if needed
5. Create the final AppImage: `dist/postnote-0.1a-x86_64.AppImage` (approximately 1.8MB)

**Alternative:** Run the build script directly:

```bash
./scripts/build-appimage.sh
```

### Build Options

- `task build` - Build the `postnote` binary
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
chmod +x dist/postnote-0.1a-x86_64.AppImage
./dist/postnote-0.1a-x86_64.AppImage
```

## Running from Source

After building the binary:

```bash
./bin/postnote
```

The application will:
- Create a data file at `~/.config/indicator-stickynotes`
- Show a system tray icon
- Allow you to create and manage sticky notes

## Project Structure

```
postnote/
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
│   ├── style_global.css   # Global CSS
│   └── Icons/             # Application icons
├── scripts/               # Build and utility scripts
│   ├── build-appimage.sh  # AppImage build script
│   └── setup-build.sh     # Build environment setup
├── docs/                  # Documentation
│   ├── index.html         # Project documentation page
│   ├── PostNote.png       # Project logo
│   └── runnableapp.png    # Runnable.App logo
├── Taskfile.yml           # Build automation
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
└── README.md              # This file
```

## License

PostNote is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.
 
PostNote is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public License for more details.
 
You should have received a copy of the GNU General Public License along with PostNote.  If not, see http://www.gnu.org/licenses/

© 2025 Runable.App


## Credits

This application is based on indicator-stickynotes.  PostNote borrows ideas, workflow, icons, and UI elements from the original project. indicator-stickynotes is © 2012–2018 Umang Varma https://launchpad.net/indicator-stickynotes/

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

## Version

Current version: **0.1a**
