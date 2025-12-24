# Wayland Window Position Support via window-calls Extension

## Overview

This application now supports getting window positions on Wayland by using the [window-calls GNOME extension](https://github.com/ickyicky/window-calls). This overcomes Wayland's security restrictions that prevent applications from directly accessing window coordinates.

## How It Works

### window-calls Extension

The window-calls extension provides D-Bus methods to:
- List all windows with their properties (PID, ID, position, size, etc.)
- Get detailed information about specific windows
- Move windows between workspaces

### Integration

1. **Wayland Detection**: The application first checks if it's running on Wayland.

2. **Extension Detection**: If on Wayland, the application checks if the window-calls extension is available by calling the `List` method.

3. **Conditional Usage**: The D-Bus methods are **only used when both conditions are met**:
   - Running on Wayland
   - window-calls extension is installed and enabled

2. **Window Matching**: When a note window is created, the application:
   - Gets all windows belonging to the current process (by PID)
   - Matches note windows to window IDs by size and position
   - Stores the window ID for future position queries

3. **Position Updates**: 
   - On `configure-event` (window moved/resized), the application queries the window-calls extension for the current position
   - A periodic timer (every 2 seconds) also updates positions to catch any changes
   - Positions are saved to the data file

4. **Fallback**: 
   - On X11: Uses GTK's `GetPosition()` method (works normally)
   - On Wayland without extension: Uses GTK's `GetPosition()` method (returns 0,0, positions can't be saved)
   - On Wayland with extension: Uses window-calls D-Bus methods (positions work correctly)

## Installation

To enable Wayland window position support:

1. Install the window-calls GNOME extension:
   - Visit: https://extensions.gnome.org/extension/4724/window-calls/ (or install from source)
   - Or install from source: https://github.com/ickyicky/window-calls

2. Enable the extension in GNOME Extensions

3. Restart the application

## Usage

Once the extension is installed and enabled:

- Window positions are automatically tracked and saved
- Notes will restore to their previous positions on restart
- Position updates happen in real-time as windows are moved

## Technical Details

### D-Bus Calls

The application uses these D-Bus methods:

1. **List Windows**:
   ```bash
   gdbus call --session --dest org.gnome.Shell \
     --object-path /org/gnome/Shell/Extensions/Windows \
     --method org.gnome.Shell.Extensions.Windows.List
   ```

2. **Get Window Details** (position, size, etc.):
   ```bash
   gdbus call --session --dest org.gnome.Shell \
     --object-path /org/gnome/Shell/Extensions/Windows \
     --method org.gnome.Shell.Extensions.Windows.Details <window_id>
   ```

### Window Matching Strategy

Since all note windows belong to the same process (same PID), we:
1. Get all windows for the current PID
2. Match windows to notes by:
   - Window size (within 10 pixels tolerance)
   - Window position (if available, within 50 pixels tolerance)
3. Store the window ID for future queries

### Code Structure

- **`stickynotes/window_calls.go`**: D-Bus integration and window query functions
- **`stickynotes/gui.go`**: Updated to use window-calls when available
- **`main.go`**: Starts periodic position updates if extension is available

## Benefits

- ✅ Window positions work on Wayland
- ✅ Notes restore to previous positions
- ✅ Real-time position tracking
- ✅ Graceful fallback if extension not available
- ✅ No changes needed for X11 (still uses GTK methods)

## Limitations

- Requires the window-calls extension to be installed and enabled
- Window matching by size/position may occasionally match the wrong window if multiple notes have similar sizes
- Position updates happen every 2 seconds (configurable)

## Future Improvements

- Store window IDs in the data file for more reliable matching
- Use window titles or other properties for better matching
- Reduce update frequency or make it configurable
- Add window title tracking if needed

