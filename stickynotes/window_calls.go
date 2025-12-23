package stickynotes

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
)

// WindowInfo represents window information from window-calls extension
type WindowInfo struct {
	ID      uint32 `json:"id"` // D-Bus expects uint32 (u), not int64 (x)
	PID     int    `json:"pid"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	WMClass string `json:"wm_class"`
	Title   string `json:"title,omitempty"`
}

// WindowDetails represents detailed window information
type WindowDetails struct {
	ID        uint32 `json:"id"` // D-Bus expects uint32 (u), not int64 (x)
	PID       int    `json:"pid"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	WMClass   string `json:"wm_class"`
	Title     string `json:"title,omitempty"`
	Maximized int    `json:"maximized"`
	Focus     bool   `json:"focus"`
}

var (
	windowCallsAvailable bool
	windowCallsChecked   bool // Track if we've already checked (to avoid repeated failures)
	currentPID           int
	dbusConn             *dbus.Conn // D-Bus connection (cached)
)

func init() {
	currentPID = os.Getpid()
	// Only check for extension if we're on Wayland
	if IsWayland() {
		windowCallsAvailable = checkWindowCallsExtension()
		windowCallsChecked = true
	} else {
		windowCallsAvailable = false
		windowCallsChecked = true
	}
}

// getDBusConnection gets or creates a D-Bus session connection
func getDBusConnection() (*dbus.Conn, error) {
	if dbusConn != nil {
		return dbusConn, nil
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	dbusConn = conn
	return conn, nil
}

// checkWindowCallsExtension checks if the window-calls GNOME extension is available
func checkWindowCallsExtension() bool {
	conn, err := getDBusConnection()
	if err != nil {
		fmt.Printf("[WindowCalls] Failed to connect to D-Bus: %v\n", err)
		return false
	}

	// Create the bus object
	obj := conn.Object("org.gnome.Shell", dbus.ObjectPath("/org/gnome/Shell/Extensions/Windows"))

	// Try to call the List method - if it succeeds, extension is available
	var out string
	err = obj.Call("org.gnome.Shell.Extensions.Windows.List", 0).Store(&out)

	if err != nil {
		// Log the error only once during init
		fmt.Printf("[WindowCalls] Extension check failed: %v\n", err)
		return false
	}

	// Check if we got a valid response (should be JSON array)
	if len(out) > 0 && (out[0] == '[' || out[0] == '{') {
		fmt.Printf("[WindowCalls] Extension is available and enabled\n")
		return true
	}

	return false
}

// IsWindowCallsAvailable returns whether the window-calls extension is available
// Only returns true if running on Wayland AND extension is installed
func IsWindowCallsAvailable() bool {
	return IsWayland() && windowCallsAvailable
}

// ListWindows gets all windows from the window-calls extension
func ListWindows() ([]WindowInfo, error) {
	if !IsWindowCallsAvailable() {
		// Don't return error if we've already checked and it's not available
		// This prevents spam in logs
		return nil, nil
	}

	conn, err := getDBusConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to D-Bus: %w", err)
	}

	// Create the bus object
	obj := conn.Object("org.gnome.Shell", dbus.ObjectPath("/org/gnome/Shell/Extensions/Windows"))

	// Call the List method
	var out string
	err = obj.Call("org.gnome.Shell.Extensions.Windows.List", 0).Store(&out)
	if err != nil {
		// If extension is not available, don't spam errors
		if dbusErr, ok := err.(dbus.Error); ok {
			if dbusErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown" ||
				dbusErr.Name == "org.freedesktop.DBus.Error.UnknownMethod" {
				fmt.Printf("[WindowCalls] Service/Method not found, marking extension as unavailable\n")
				windowCallsAvailable = false
				windowCallsChecked = true
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to call List: %w", err)
	}

	// fmt.Printf("[WindowCalls] Full response:\n%s\n", out)

	// Parse the JSON output directly (no need to unwrap gdbus format)
	var windows []WindowInfo
	if err := json.Unmarshal([]byte(out), &windows); err != nil {
		fmt.Printf("[WindowCalls] JSON parsing failed: %v\n", err)
		return nil, fmt.Errorf("failed to parse window list: %w (output: %s)", err, out[:min(100, len(out))])
	}

	return windows, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetWindowDetails gets detailed information about a specific window
// windowID must be uint32 (D-Bus type 'u')
func GetWindowDetails(windowID uint32) (*WindowDetails, error) {
	if !IsWindowCallsAvailable() {
		// Don't return error if we've already checked and it's not available
		return nil, nil
	}

	conn, err := getDBusConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to D-Bus: %w", err)
	}

	// Create the bus object
	obj := conn.Object("org.gnome.Shell", dbus.ObjectPath("/org/gnome/Shell/Extensions/Windows"))

	// Call the Details method with window ID
	var out string
	err = obj.Call("org.gnome.Shell.Extensions.Windows.Details", 0, windowID).Store(&out)
	if err != nil {
		// If extension is not available, don't spam errors
		if dbusErr, ok := err.(dbus.Error); ok {
			// Suppress JavaScript errors from the extension (window might not exist or be invalid)
			if dbusErr.Name == "org.gnome.gjs.JSError.Error" {
				// Window ID might be invalid or window doesn't exist - silently ignore
				return nil, nil
			}
			if dbusErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown" ||
				dbusErr.Name == "org.freedesktop.DBus.Error.UnknownMethod" {
				fmt.Printf("[WindowCalls] Service/Method not found, marking extension as unavailable\n")
				windowCallsAvailable = false
				windowCallsChecked = true
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to call Details: %w", err)
	}

	// Parse the JSON output directly
	var details WindowDetails
	if err := json.Unmarshal([]byte(out), &details); err != nil {
		fmt.Printf("[WindowCalls] JSON parsing failed: %v\n", err)
		return nil, fmt.Errorf("failed to parse window details: %w (output: %s)", err, out[:min(100, len(out))])
	}

	// fmt.Printf("[WindowCalls] ===== GetWindowDetails RETURN VALUES for windowID=%d =====\n", windowID)
	// fmt.Printf("[WindowCalls]   Returning: ID=%d, Pos=(%d,%d), Size=(%d,%d), Title='%s', PID=%d\n",
	// 	details.ID, details.X, details.Y, details.Width, details.Height, details.Title, details.PID)
	// fmt.Printf("[WindowCalls] ===== END GetWindowDetails RETURN =====\n")

	return &details, nil
}

// FindWindowByPID finds a window ID for a given PID
// Returns the first matching window ID, or 0 if not found
func FindWindowByPID(pid int) (uint32, error) {
	windows, err := ListWindows()
	if err != nil {
		return 0, err
	}

	for _, win := range windows {
		if win.PID == pid {
			return win.ID, nil
		}
	}

	return 0, fmt.Errorf("no window found for PID %d", pid)
}

// GetWindowPosition gets the position of a window using window-calls extension
// Returns x, y coordinates and error
func GetWindowPosition(windowID uint32) (int, int, error) {
	details, err := GetWindowDetails(windowID)
	if err != nil {
		return 0, 0, err
	}

	return details.X, details.Y, nil
}

// GetCurrentProcessWindows finds all windows belonging to the current process
// Filters by both PID and title "Sticky Notes" for more accurate matching
func GetCurrentProcessWindows() ([]WindowInfo, error) {
	windows, err := ListWindows()
	if err != nil {
		return nil, err
	}

	if windows == nil {
		return nil, fmt.Errorf("window-calls extension not available")
	}

	var ourWindows []WindowInfo
	for _, win := range windows {
		pidMatch := win.PID == currentPID
		titleMatch := strings.Contains(win.Title, "Sticky Notes")

		// Match by PID AND title for more accurate filtering
		if pidMatch && titleMatch {
			ourWindows = append(ourWindows, win)
		} else if pidMatch {
			// Fallback: if PID matches but title doesn't, still include it (might be a new window without title yet)
			ourWindows = append(ourWindows, win)
		}
	}

	// if len(ourWindows) > 0 {
	// 	fmt.Printf("[WindowCalls] ===== FILTERED STICKY NOTES WINDOW IDs =====\n")
	// 	for i, win := range ourWindows {
	// 		fmt.Printf("[WindowCalls]   OurWindow[%d]: ID=%d, Title='%s', Pos=(%d,%d), Size=(%d,%d), WMClass=%s\n",
	// 			i, win.ID, win.Title, win.X, win.Y, win.Width, win.Height, win.WMClass)
	// 	}
	// 	fmt.Printf("[WindowCalls] ===== END FILTERED WINDOW IDs =====\n")
	// } else {
	// 	fmt.Printf("[WindowCalls] WARNING: No filtered windows found!\n")
	// }

	return ourWindows, nil
}

// UpdateNotePositionsFromWindowCalls updates note positions using window-calls extension
// This is called from onConfigure() when windows are moved/resized, not periodically
// Only works on Wayland when the extension is installed
func (ns *NoteSet) UpdateNotePositionsFromWindowCalls() {
	if !IsWindowCallsAvailable() {
		return
	}

	// Get all windows for our process
	windows, err := GetCurrentProcessWindows()
	if err != nil {
		// Only log error if we haven't checked yet, or if it's a new error
		// This prevents spam when extension is not available
		if !windowCallsChecked {
			fmt.Printf("[WindowCalls] Failed to get windows: %v\n", err)
		}
		return
	}

	if len(windows) == 0 {
		return
	}

	updated := false

	// For each note with a GUI, try to find its window and update position
	for _, note := range ns.Notes {
		if note.GUI == nil || note.GUI.WinMain == nil {
			continue
		}

		// If we already have a window ID, use it directly
		if note.GUI.WindowID != 0 {
			details, err := GetWindowDetails(note.GUI.WindowID)
			if err == nil && details != nil {
				oldPos := note.GUI.LastKnownPos
				// oldSize := note.GUI.LastKnownSize
				newPos := [2]int{details.X, details.Y}
				newSize := [2]int{details.Width, details.Height}

				note.GUI.LastKnownPos = newPos
				note.GUI.LastKnownSize = newSize

				// Only mark as updated if position actually changed
				if oldPos[0] != newPos[0] || oldPos[1] != newPos[1] {
					updated = true
				}
				continue
			} else {
				fmt.Printf("[WindowCalls] Failed to get details for note %s window ID %d: %v\n",
					note.UUID[:8], note.GUI.WindowID, err)
			}
		}

		// Try to match window by size
		w, h := note.GUI.WinMain.GetSize()
		for _, win := range windows {
			// Skip if this window ID is already assigned to another note
			alreadyAssigned := false
			for _, otherNote := range ns.Notes {
				if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != note {
					alreadyAssigned = true
					break
				}
			}
			if alreadyAssigned {
				continue
			}

			details, err := GetWindowDetails(win.ID)
			if err != nil || details == nil {
				continue
			}

			// Match by size (within 10 pixels tolerance)
			if absInt(details.Width-w) < 10 && absInt(details.Height-h) < 10 {
				fmt.Printf("[WindowCalls: UpdateNotePositionsFromWindowCalls] Note %s: Matched window ID %d with size (%d, %d)\n", note.UUID[:8], win.ID, w, h)
				note.GUI.WindowID = win.ID
				// oldPos := note.GUI.LastKnownPos
				// oldSize := note.GUI.LastKnownSize
				newPos := [2]int{details.X, details.Y}
				newSize := [2]int{details.Width, details.Height}

				note.GUI.LastKnownPos = newPos
				note.GUI.LastKnownSize = newSize
				updated = true
				break
			}
		}
	}

	// Save if any positions were updated
	if updated {
		ns.Save()
	}
}

// MoveWindow moves a window to the specified position using window-calls extension
// This works on Wayland where GTK's Move() doesn't work
// Parameters: windowID (uint32), x (int), y (int)
func MoveWindow(windowID uint32, x, y int) error {
	if !IsWindowCallsAvailable() {
		return fmt.Errorf("window-calls extension not available")
	}

	conn, err := getDBusConnection()
	if err != nil {
		return err
	}

	// Create the bus object
	obj := conn.Object("org.gnome.Shell", dbus.ObjectPath("/org/gnome/Shell/Extensions/Windows"))

	// Call the Move method with window ID, x, y
	// The method signature is: Move(winid: u, x: i, y: i)
	err = obj.Call("org.gnome.Shell.Extensions.Windows.Move", 0, windowID, int32(x), int32(y)).Err
	if err != nil {
		if dbusErr, ok := err.(dbus.Error); ok {
			fmt.Printf("[WindowCalls] D-Bus error name: %s\n", dbusErr.Name)
		}
		return err
	}

	return nil
}
