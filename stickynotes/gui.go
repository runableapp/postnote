package stickynotes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// ResourceGetter interface for accessing embedded resources
// This allows the stickynotes package to access embedded resources without importing main
type ResourceGetter interface {
	GetEmbeddedUI(filename string) (string, error)
	GetEmbeddedCSS(filename string) (string, error)
	GetEmbeddedIcon(iconPath string) ([]byte, error)
}

var globalResourceGetter ResourceGetter

// SetResourceGetter sets the global resource getter (called from main package)
func SetResourceGetter(getter ResourceGetter) {
	globalResourceGetter = getter
}

// getEmbeddedUI tries to get UI content from embedded resources, falls back to file system
func getEmbeddedUI(filename string) (string, error) {
	if globalResourceGetter != nil {
		if content, err := globalResourceGetter.GetEmbeddedUI(filename); err == nil {
			return content, nil
		}
	}
	// Fallback to file system
	path := GetBasePath()
	uiPath := filepath.Join(path, filename)
	data, err := os.ReadFile(uiPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// getEmbeddedIcon tries to get icon from embedded resources, falls back to file system
func getEmbeddedIcon(iconPath string) ([]byte, error) {
	if globalResourceGetter != nil {
		if data, err := globalResourceGetter.GetEmbeddedIcon(iconPath); err == nil {
			return data, nil
		}
	}
	// Fallback to file system
	path := GetBasePath()
	iconFilePath := filepath.Join(path, "Icons", iconPath)
	return os.ReadFile(iconFilePath)
}

// Helper function for absolute value of integers
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// removePixbufProperties removes pixbuf properties from UI XML to prevent GTK Builder
// from trying to load icons from file system. Icons will be loaded manually after widgets are created.
func removePixbufProperties(xml string) string {
	// Use regex to remove <property name="pixbuf">...</property> blocks
	// Pattern matches: <property name="pixbuf">Icons/...</property>
	lines := strings.Split(xml, "\n")
	var result []string
	skipNext := false
	for _, line := range lines {
		if skipNext {
			skipNext = false
			continue
		}

		// Check if this line contains pixbuf property opening
		if strings.Contains(line, `<property name="pixbuf">`) {
			// Check if the closing tag is on the same line
			if strings.Contains(line, `</property>`) {
				// Single line: <property name="pixbuf">Icons/add.png</property>
				continue
			}
			// Multi-line: skip this line and the next (which has the path and closing tag)
			skipNext = true
			continue
		}

		// Skip lines that are just the icon path and closing tag
		if strings.Contains(line, `Icons/`) && strings.Contains(line, `</property>`) {
			continue
		}

		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// IsWayland checks if the application is running on Wayland
func IsWayland() bool {
	// Check XDG_SESSION_TYPE environment variable
	if sessionType := os.Getenv("XDG_SESSION_TYPE"); sessionType == "wayland" {
		return true
	}

	// Also check WAYLAND_DISPLAY
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}

	return false
}

// LoadGlobalCSS loads the global CSS stylesheet
func LoadGlobalCSS() error {
	cssProvider, err := gtk.CssProviderNew()
	if err != nil {
		return err
	}

	// Try to load from embedded resources first
	var cssContent string
	if globalResourceGetter != nil {
		if content, err := globalResourceGetter.GetEmbeddedCSS("style_global.css"); err == nil {
			cssContent = content
		}
	}

	// Fallback to file system if embedded not available
	if cssContent == "" {
		path := filepath.Join(getBasePath(), "style_global.css")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		cssContent = string(data)
	}

	// Load from in-memory data
	err = cssProvider.LoadFromData(cssContent)
	if err != nil {
		return err
	}

	screen, err := gdk.ScreenGetDefault()
	if err != nil {
		return err
	}

	gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	return nil
}

// StickyNote manages the GUI of an individual sticky note
type StickyNote struct {
	Path              string
	Note              *Note
	NoteSet           *NoteSet
	Locked            bool
	Builder           *gtk.Builder
	WinMain           *gtk.Window
	TxtNote           *gtk.TextView
	BBody             *gtk.TextBuffer
	BAdd              *gtk.Button
	BClose            *gtk.Button
	BLock             *gtk.Button
	BMenu             *gtk.Button
	ImgAdd            *gtk.Image
	ImgClose          *gtk.Image
	ImgLock           *gtk.Image
	ImgUnlock         *gtk.Image
	ImgResizeR        *gtk.Image
	EResizeR          *gtk.EventBox
	MoveBox1          *gtk.EventBox
	MoveBox2          *gtk.EventBox
	Menu              *gtk.Menu
	LastKnownPos      [2]int
	LastKnownSize     [2]int
	CSSProvider       *gtk.CssProvider
	menuHideConnected bool
	WindowID          uint32            // Window ID from window-calls extension (D-Bus uint32)
	saveTimeoutID     glib.SourceHandle // Timeout ID for debounced save
}

// NewStickyNote creates a new sticky note GUI
func NewStickyNote(note *Note) *StickyNote {
	sn := &StickyNote{
		Path:    getBasePath(),
		Note:    note,
		NoteSet: note.NoteSet,
		Locked:  false,
	}

	if locked, ok := note.Properties["locked"].(bool); ok {
		sn.Locked = locked
	}

	sn.buildNote()
	return sn
}

func (sn *StickyNote) buildNote() {
	var err error

	// Load UI file from embedded resources (in-memory)
	uiContent, err := getEmbeddedUI("StickyNotes.ui")
	if err != nil {
		// Fallback to file system if embedded not available
		uiPath := filepath.Join(sn.Path, "StickyNotes.ui")
		sn.Builder, err = gtk.BuilderNewFromFile(uiPath)
		if err != nil {
			fmt.Printf("Error loading UI file: %v\n", err)
			return
		}
	} else {
		// Remove pixbuf properties from XML to prevent GTK Builder from trying to load icons
		// We'll load them manually after the builder creates the widgets
		uiContent = removePixbufProperties(uiContent)

		// Use in-memory API
		sn.Builder, err = gtk.BuilderNewFromString(uiContent)
		if err != nil {
			fmt.Printf("Error loading UI from embedded resources: %v\n", err)
			return
		}
	}

	// Get main window
	obj, err := sn.Builder.GetObject("MainWindow")
	if err != nil {
		fmt.Printf("Error getting MainWindow: %v\n", err)
		return
	}
	sn.WinMain = obj.(*gtk.Window)

	// Get widgets
	sn.TxtNote, _ = getObject[*gtk.TextView](sn.Builder, "txtNote")
	sn.BAdd, _ = getObject[*gtk.Button](sn.Builder, "bAdd")
	sn.BClose, _ = getObject[*gtk.Button](sn.Builder, "bClose")
	sn.BLock, _ = getObject[*gtk.Button](sn.Builder, "bLock")
	sn.BMenu, _ = getObject[*gtk.Button](sn.Builder, "bMenu")
	sn.ImgAdd, _ = getObject[*gtk.Image](sn.Builder, "imgAdd")
	sn.ImgClose, _ = getObject[*gtk.Image](sn.Builder, "imgClose")
	sn.ImgLock, _ = getObject[*gtk.Image](sn.Builder, "imgLock")
	sn.ImgUnlock, _ = getObject[*gtk.Image](sn.Builder, "imgUnlock")
	sn.ImgResizeR, _ = getObject[*gtk.Image](sn.Builder, "imgResizeR")
	sn.EResizeR, _ = getObject[*gtk.EventBox](sn.Builder, "eResizeR")
	sn.MoveBox1, _ = getObject[*gtk.EventBox](sn.Builder, "movebox1")
	sn.MoveBox2, _ = getObject[*gtk.EventBox](sn.Builder, "movebox2")

	// Get imgDropdown (used by bMenu button)
	imgDropdown, _ := getObject[*gtk.Image](sn.Builder, "imgDropdown")

	// Load icons from embedded resources (since UI file references Icons/ paths)
	// GTK Builder will fail to load these from file system when using BuilderNewFromString
	// So we manually set them using embedded data
	sn.loadIconsFromEmbedded(imgDropdown)

	// Connect signals
	sn.BAdd.Connect("clicked", sn.onAdd)
	sn.BClose.Connect("clicked", sn.onDelete)
	sn.BLock.Connect("clicked", sn.onLockClicked)
	sn.BMenu.Connect("clicked", sn.onPopupMenu)
	sn.EResizeR.Connect("button-press-event", sn.onResize)
	sn.MoveBox1.Connect("button-press-event", sn.onMove)
	sn.MoveBox2.Connect("button-press-event", sn.onMove)
	sn.WinMain.Connect("focus-out-event", sn.onFocusOut)
	sn.WinMain.Connect("configure-event", sn.onConfigure)
	sn.WinMain.Connect("delete-event", sn.onWindowDelete)

	// Create text buffer
	sn.BBody, _ = gtk.TextBufferNew(nil)
	sn.BBody.SetText(sn.Note.Body)
	sn.TxtNote.SetBuffer(sn.BBody)

	// Create menu
	sn.Menu, _ = gtk.MenuNew()
	sn.PopulateMenu()

	// Note: CSS will be loaded in show() after window is ready
	// This ensures category properties are available and window is realized

	// Set position and size
	// On Wayland, Move() must be called AFTER ShowAll() to work properly
	// So we'll store the position and apply it after ShowAll()
	restorePos := [2]int{10, 10}
	if pos, ok := sn.Note.Properties["position"].([]interface{}); ok && len(pos) >= 2 {
		if x, ok := pos[0].(float64); ok {
			if y, ok := pos[1].(float64); ok {
				restorePos = [2]int{int(x), int(y)}
				sn.LastKnownPos = [2]int{int(x), int(y)}
			}
		}
	} else {
		// For new notes, use a cascaded position to avoid overlapping
		// Calculate offset based on note index to prevent all notes at same position
		noteIndex := 0
		for i, note := range sn.NoteSet.Notes {
			if note == sn.Note {
				// Use sn.WindowID directly since we're building sn right now
				// (note.GUI is nil at this point because it's assigned after NewStickyNote returns)
				noteIndex = i
				break
			}
		}
		restorePos = [2]int{10 + noteIndex*30, 10 + noteIndex*30}
		sn.LastKnownPos = restorePos
	}

	if size, ok := sn.Note.Properties["size"].([]interface{}); ok && len(size) >= 2 {
		if w, ok := size[0].(float64); ok {
			if h, ok := size[1].(float64); ok {
				sn.WinMain.Resize(int(w), int(h))
				sn.LastKnownSize = [2]int{int(w), int(h)}
			}
		}
	} else {
		sn.LastKnownSize = [2]int{200, 150}
		sn.WinMain.Resize(200, 150)
	}

	// Set locked state
	sn.SetLockedState(sn.Locked)

	// Set widget names to match CSS selectors
	sn.WinMain.SetName("main-window")
	sn.TxtNote.SetName("txt-note")

	// Set unique window title for identification via D-Bus
	// Format: "Sticky Notes - <UUID>" - this allows us to match windows by title
	// The title is not visible in the UI (window is undecorated) but is available via D-Bus
	sn.WinMain.SetTitle(fmt.Sprintf("Sticky Notes - %s", sn.Note.UUID[:8]))

	// Initialize Provider: Create the CssProvider and add it to the context NOW
	// This must be done BEFORE loading data and BEFORE ShowAll()
	// This ensures the provider is attached when the widget is realized
	if sn.CSSProvider == nil {
		sn.CSSProvider, _ = gtk.CssProviderNew()
	}

	// Get style contexts and add provider BEFORE loading data
	// This matches the Python version's behavior
	winContext, _ := sn.WinMain.GetStyleContext()
	txtContext, _ := sn.TxtNote.GetStyleContext()
	winContext.AddProvider(sn.CSSProvider, gtk.STYLE_PROVIDER_PRIORITY_USER)
	txtContext.AddProvider(sn.CSSProvider, gtk.STYLE_PROVIDER_PRIORITY_USER)

	// Load Data: Call LoadCSS() logic (generates CSS string and loads into provider)
	// This happens while the window is still hidden
	sn.LoadCSS()
	sn.UpdateFont()

	// Strategy: Make window invisible, show it, move it, then make it visible
	// This prevents the visual "jump" from default position to saved position
	sn.WinMain.SetOpacity(0.0) // Make window invisible

	// FINALLY call ShowAll() - window is shown but invisible
	sn.WinMain.SetSkipPagerHint(true)
	sn.WinMain.ShowAll()

	// On Wayland, GTK's Move() doesn't work, so we must use D-Bus via window-calls extension
	// Note: We cannot move the window before showing it because:
	// - GTK Move() doesn't work on Wayland
	// - D-Bus Move() requires window ID
	// - Window ID can only be obtained after window is shown and registered with window manager
	// So there will be a brief visual "jump" from default position to saved position

	// On Wayland, we need to wait a bit for windows to get their actual size before matching
	// Use a timeout to allow windows to be fully realized
	if IsWindowCallsAvailable() {
		// Wait 300ms for windows to be fully realized and get their sizes
		glib.TimeoutAdd(300, func() bool {

			// Try to get window ID if not assigned yet (match by title)
			if sn.WindowID == 0 {
				expectedTitle := fmt.Sprintf("Sticky Notes - %s", sn.Note.UUID[:8])
				windows, err := GetCurrentProcessWindows()
				if err == nil && windows != nil {
					for _, win := range windows {
						// Skip if already assigned to another note
						alreadyAssigned := false
						for _, otherNote := range sn.NoteSet.Notes {
							if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
								alreadyAssigned = true
								break
							}
						}
						if alreadyAssigned {
							continue
						}

						// Get details to check title
						details, err := GetWindowDetails(win.ID)
						if err == nil && details != nil {
							// Match by title (exact match)
							if details.Title == expectedTitle {
								// Double-check: make sure no other note has this ID
								conflict := false
								for _, otherNote := range sn.NoteSet.Notes {
									if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
										conflict = true
										break
									}
								}
								if !conflict {
									// Final atomic check: verify no other note has this ID RIGHT NOW
									// This prevents race conditions where two notes might assign the same ID simultaneously
									finalConflict := false
									for _, otherNote := range sn.NoteSet.Notes {
										if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
											finalConflict = true
											break
										}
									}
									if !finalConflict {
										// ONE MORE CHECK: Make absolutely sure no other note has this ID
										// This is a last-ditch effort to prevent duplicate assignments
										for _, otherNote := range sn.NoteSet.Notes {
											if otherNote != sn.Note && otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID {
												fmt.Printf("[buildNote] Note %s: ABORT! Window ID %d is already assigned to note %s, NOT assigning\n", sn.Note.UUID[:8], win.ID, otherNote.UUID[:8])
												break // Don't assign, break out of window loop
											}
										}
										// Check one more time before assigning (in case another note assigned it in the meantime)
										stillAvailable := true
										for _, otherNote := range sn.NoteSet.Notes {
											if otherNote != sn.Note && otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID {
												stillAvailable = false
												break
											}
										}
										if stillAvailable {
											sn.WindowID = win.ID
											break
										}
									}
								}
							}
						} else {
							// fmt.Printf("[# buildNote] Note %s: Could not get details for window ID %d: %v\n", sn.Note.UUID[:8], win.ID, err)
						}
					}
				} else {
					// fmt.Printf("[# buildNote] Note %s: Error getting windows: %v\n", sn.Note.UUID[:8], err)
				}
			} else {
				// fmt.Printf("[# buildNote] Note %s: Window ID already assigned: %d\n", sn.Note.UUID[:8], sn.WindowID)
			}

			if sn.WindowID != 0 {
				err := MoveWindow(sn.WindowID, restorePos[0], restorePos[1])
				if err == nil {
					sn.WinMain.SetOpacity(1.0) // Make window visible after moving
				} else {
					// Fallback to GTK Move() (might not work on Wayland but worth trying)
					sn.WinMain.Move(restorePos[0], restorePos[1])
					sn.WinMain.SetOpacity(1.0) // Make window visible after moving
				}
			} else {
				// Fallback to GTK Move() (might not work on Wayland but worth trying)
				// Also try to move immediately on X11 to prevent appearing at (0,0)
				if !IsWindowCallsAvailable() {
					sn.WinMain.Move(restorePos[0], restorePos[1])
				}
				sn.WinMain.SetOpacity(1.0) // Make window visible after moving
				// On Wayland, if we still don't have window ID, try GTK Move as last resort
				if IsWindowCallsAvailable() {
					sn.WinMain.Move(restorePos[0], restorePos[1])
				}
			}

			return false // Don't repeat
		})
	} else {
		// On X11 or extension not available, use GTK Move() immediately
		glib.IdleAdd(func() bool {
			sn.WinMain.Move(restorePos[0], restorePos[1])
			sn.WinMain.SetOpacity(1.0) // Make window visible after moving
			return false               // Don't repeat
		})
	}

	// Check actual position from D-Bus after a delay to allow window to move and get ID assigned
	/*
		if IsWindowCallsAvailable() {
			// Use TimeoutAdd to check position after a delay
			// We wait 1500ms to ensure both the move and assignWindowID() have completed
			fmt.Printf("[buildNote:1500] Note %s: Checking actual position from D-Bus after a delay to allow window to move and get ID assigned\n", sn.Note.UUID[:8])
			glib.TimeoutAdd(1500, func() bool {

				// If Window ID is still 0, call assignWindowID() directly to get it
				if sn.WindowID == 0 {
					fmt.Printf("[buildNote:1500ms] Note %s: Window ID still 0, calling assignWindowID()\n", sn.Note.UUID[:8])

					sn.assignWindowID()
					if sn.WindowID == 0 {
						return false // Don't repeat
					}
				}

				// Now we have Window ID, verify the position
				details, err := GetWindowDetails(sn.WindowID)
				if err == nil && details != nil {
					// Position verification (no action needed)
				}
				return false // Don't repeat
			})
			fmt.Printf("[buildNote] Note %s: 1500ms timeout completed\n", sn.Note.UUID[:8])
		}
	*/
}

// assignWindowID gets and stores the window ID for this note from window-calls extension
// Matches windows by unique title: "Sticky Notes - <UUID>"
func (sn *StickyNote) assignWindowID() {
	fmt.Printf("[assignWindowID] Note %s: assignWindowID() called, current WindowID=%d\n", sn.Note.UUID[:8], sn.WindowID)
	if sn.WindowID != 0 {
		// Already assigned
		fmt.Printf("[assignWindowID] Note %s: Window ID already assigned: %d\n", sn.Note.UUID[:8], sn.WindowID)
		return
	}

	windows, err := GetCurrentProcessWindows()
	if err != nil {
		fmt.Printf("[assignWindowID] Note %s: Error getting windows: %v\n", sn.Note.UUID[:8], err)
		return
	}

	if len(windows) == 0 {
		fmt.Printf("[assignWindowID] Note %s: No windows found\n", sn.Note.UUID[:8])
		return
	}

	// Match by unique title
	expectedTitle := fmt.Sprintf("Sticky Notes - %s", sn.Note.UUID[:8])
	fmt.Printf("[assignWindowID] Note %s: Looking for window with title: %s\n", sn.Note.UUID[:8], expectedTitle)
	fmt.Printf("[assignWindowID] Note %s: Found %d windows\n", sn.Note.UUID[:8], len(windows))
	// Debug: Print all window IDs and their current assignments
	fmt.Printf("[assignWindowID] Note %s: Current window ID assignments:\n", sn.Note.UUID[:8])
	for _, otherNote := range sn.NoteSet.Notes {
		if otherNote.GUI != nil && otherNote.GUI.WindowID != 0 {
			fmt.Printf("[assignWindowID]   Note %s -> Window ID %d\n", otherNote.UUID[:8], otherNote.GUI.WindowID)
		}
	}
	for _, win := range windows {
		// Skip if this window ID is already assigned to another note
		alreadyAssigned := false
		for _, otherNote := range sn.NoteSet.Notes {
			if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
				alreadyAssigned = true
				fmt.Printf("[assignWindowID] Note %s: Window ID %d already assigned to note %s, skipping\n", sn.Note.UUID[:8], win.ID, otherNote.UUID[:8])
				break
			}
		}
		if alreadyAssigned {
			continue
		}

		// Get details to check title (List() might not have full title info)
		details, err := GetWindowDetails(win.ID)
		if err != nil || details == nil {
			// Fallback: try to match using title from List() if available
			if win.Title == expectedTitle {
				// Double-check: make sure no other note has this ID
				conflict := false
				for _, otherNote := range sn.NoteSet.Notes {
					if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
						conflict = true
						fmt.Printf("[assignWindowID] Note %s: CONFLICT! Window ID %d already assigned to note %s, NOT assigning\n", sn.Note.UUID[:8], win.ID, otherNote.UUID[:8])
						break
					}
				}
				if !conflict {
					// Final atomic check: verify no other note has this ID RIGHT NOW
					// This prevents race conditions where two notes might assign the same ID simultaneously
					finalConflict := false
					for _, otherNote := range sn.NoteSet.Notes {
						if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
							finalConflict = true
							fmt.Printf("[assignWindowID] Note %s: FINAL CONFLICT CHECK! Window ID %d already assigned to note %s, NOT assigning\n", sn.Note.UUID[:8], win.ID, otherNote.UUID[:8])
							break
						}
					}
					if !finalConflict {
						// ONE MORE CHECK: Make absolutely sure no other note has this ID
						// This is a last-ditch effort to prevent duplicate assignments
						for _, otherNote := range sn.NoteSet.Notes {
							if otherNote != sn.Note && otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID {
								fmt.Printf("[assignWindowID] Note %s: ABORT! Window ID %d is already assigned to note %s, NOT assigning\n", sn.Note.UUID[:8], win.ID, otherNote.UUID[:8])
								return // Don't assign, just return
							}
						}
						sn.WindowID = win.ID
						fmt.Printf("[assignWindowID] Note %s: Matched window ID %d with title from List(): %s\n", sn.Note.UUID[:8], win.ID, win.Title)
						return
					}
				}
			}
			continue
		}

		fmt.Printf("[assignWindowID] Note %s: Window ID %d has title: %s\n", sn.Note.UUID[:8], win.ID, details.Title)
		// Match by title (exact match)
		if details.Title == expectedTitle {
			// Double-check: make sure no other note has this ID
			conflict := false
			for _, otherNote := range sn.NoteSet.Notes {
				if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
					conflict = true
					fmt.Printf("[assignWindowID] Note %s: CONFLICT! Window ID %d already assigned to note %s, NOT assigning\n", sn.Note.UUID[:8], win.ID, otherNote.UUID[:8])
					break
				}
			}
			if !conflict {
				// Final atomic check: verify no other note has this ID RIGHT NOW
				// This prevents race conditions where two notes might assign the same ID simultaneously
				finalConflict := false
				for _, otherNote := range sn.NoteSet.Notes {
					if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
						finalConflict = true
						fmt.Printf("[assignWindowID] Note %s: FINAL CONFLICT CHECK! Window ID %d already assigned to note %s, NOT assigning\n", sn.Note.UUID[:8], win.ID, otherNote.UUID[:8])
						break
					}
				}
				if !finalConflict {
					// ONE MORE CHECK: Make absolutely sure no other note has this ID
					// This is a last-ditch effort to prevent duplicate assignments
					for _, otherNote := range sn.NoteSet.Notes {
						if otherNote != sn.Note && otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID {
							fmt.Printf("[assignWindowID] Note %s: ABORT! Window ID %d is already assigned to note %s, NOT assigning\n", sn.Note.UUID[:8], win.ID, otherNote.UUID[:8])
							return // Don't assign, just return
						}
					}
					sn.WindowID = win.ID
					fmt.Printf("[assignWindowID] Note %s: Matched window ID %d with title: %s\n", sn.Note.UUID[:8], win.ID, details.Title)
					return
				}
			}
		}
	}
	fmt.Printf("[assignWindowID] Note %s: No matching window found\n", sn.Note.UUID[:8])
}

func (sn *StickyNote) Show() {
	// Check if window was destroyed (can happen if note was deleted)
	if sn.WinMain == nil {
		// Window was destroyed, need to rebuild
		sn.buildNote()
		return
	}

	if sn.WinMain != nil {
		// IMPORTANT: Save LastKnownPos BEFORE calling UpdateNote(), because UpdateNote()
		// might reset it if the window is hidden (GetPosition returns 0,0)
		savedLastKnownPos := sn.LastKnownPos

		// Don't call UpdateNote() here - it will reset LastKnownPos if window is hidden
		// We'll call it after the window is shown and positioned

		// Reload CSS when showing existing note to ensure correct colors
		sn.LoadCSS()
		sn.UpdateFont()

		// Ensure unique window title is set (in case it was lost)
		sn.WinMain.SetTitle(fmt.Sprintf("Sticky Notes - %s", sn.Note.UUID[:8]))

		// Check if window is already visible - if so, preserve its current position
		// This prevents existing notes from being repositioned when a new note is created
		isVisible := sn.WinMain.GetVisible()

		// Restore saved position and size (same logic as buildNote)
		restorePos := [2]int{10, 10}
		shouldMove := true // Only move window if it's not already visible and positioned

		if pos, ok := sn.Note.Properties["position"].([]interface{}); ok && len(pos) >= 2 {
			if x, ok := pos[0].(float64); ok {
				if y, ok := pos[1].(float64); ok {
					restorePos = [2]int{int(x), int(y)}
					sn.LastKnownPos = [2]int{int(x), int(y)}
					// If window is already visible at this position, don't move it
					if isVisible && savedLastKnownPos[0] == int(x) && savedLastKnownPos[1] == int(y) {
						shouldMove = false
					}
				}
			}
		} else {
			// If no saved position in Properties, check if window is already visible
			if isVisible {
				// Window is already visible - preserve its current position
				// Use savedLastKnownPos if it's meaningful, otherwise keep current position
				if savedLastKnownPos[0] != 0 && savedLastKnownPos[1] != 0 {
					restorePos = savedLastKnownPos
					sn.LastKnownPos = savedLastKnownPos
				} else {
					// Keep current LastKnownPos (don't change it)
					restorePos = sn.LastKnownPos
				}
				shouldMove = false
			} else {
				// Window is not visible - this is a new note or note being shown for first time
				// Use savedLastKnownPos if meaningful, otherwise calculate based on index
				if (savedLastKnownPos[0] != 0 && savedLastKnownPos[1] != 0) &&
					(savedLastKnownPos[0] != 10 || savedLastKnownPos[1] != 10) {
					restorePos = savedLastKnownPos
					sn.LastKnownPos = savedLastKnownPos
				} else {
					// Calculate offset based on note index for new notes
					noteIndex := 0
					for i, note := range sn.NoteSet.Notes {
						if note == sn.Note {
							noteIndex = i
							break
						}
					}
					restorePos = [2]int{10 + noteIndex*30, 10 + noteIndex*30}
					sn.LastKnownPos = restorePos
				}
			}
		}

		if size, ok := sn.Note.Properties["size"].([]interface{}); ok && len(size) >= 2 {
			if w, ok := size[0].(float64); ok {
				if h, ok := size[1].(float64); ok {
					sn.WinMain.Resize(int(w), int(h))
					sn.LastKnownSize = [2]int{int(w), int(h)}
				}
			}
		}

		// If window is already visible and positioned, skip repositioning
		// This prevents existing notes from moving when new notes are created
		if isVisible && !shouldMove {
			// Window is already visible and positioned correctly, just ensure it's shown
			sn.WinMain.ShowAll()
			return
		}

		// Strategy: Make window invisible, show it, move it, then make it visible
		// This prevents the visual "jump" from default position to saved position
		// Use same logic as buildNote()
		sn.WinMain.SetOpacity(0.0)        // Make window invisible
		sn.WinMain.SetSkipPagerHint(true) // Same as buildNote()
		sn.WinMain.ShowAll()

		// Restore position after showing (same logic as buildNote)
		if IsWindowCallsAvailable() {
			// Wait 300ms for windows to be fully realized and get their sizes (same as buildNote)
			glib.TimeoutAdd(300, func() bool {
				// Only try to assign window ID if it's not already assigned AND note has saved position
				// For new notes (no saved position), buildNote() already handles window ID assignment,
				// so we skip it here to avoid duplicate assignments that can cause wrong window matching
				hasSavedPosition := false
				if pos, ok := sn.Note.Properties["position"].([]interface{}); ok && len(pos) >= 2 {
					hasSavedPosition = true
				}
				// Only assign window ID for existing notes (have saved position) that lost their window ID
				// New notes are handled by buildNote()'s timeout
				if sn.WindowID == 0 && hasSavedPosition {
					expectedTitle := fmt.Sprintf("Sticky Notes - %s", sn.Note.UUID[:8])
					windows, err := GetCurrentProcessWindows()
					if err == nil && windows != nil {
						// Debug: Print all window IDs and their current assignments
						// for _, otherNote := range sn.NoteSet.Notes {
						// 	if otherNote.GUI != nil && otherNote.GUI.WindowID != 0 {
						// 		fmt.Printf("[Show]   Note %s -> Window ID %d\n", otherNote.UUID[:8], otherNote.GUI.WindowID)
						// 	}
						// }
						for _, win := range windows {
							// Skip if already assigned to another note
							alreadyAssigned := false
							for _, otherNote := range sn.NoteSet.Notes {
								if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
									alreadyAssigned = true
									break
								}
							}
							if alreadyAssigned {
								continue
							}

							// Get details to check title
							details, err := GetWindowDetails(win.ID)
							if err == nil && details != nil {
								// Match by title (exact match)
								if details.Title == expectedTitle {
									// Double-check: make sure no other note has this ID
									conflict := false
									for _, otherNote := range sn.NoteSet.Notes {
										if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
											conflict = true
											break
										}
									}
									if !conflict {
										// Final atomic check: verify no other note has this ID RIGHT NOW
										// This prevents race conditions where two notes might assign the same ID simultaneously
										finalConflict := false
										for _, otherNote := range sn.NoteSet.Notes {
											if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
												finalConflict = true
												break
											}
										}
										if !finalConflict {
											sn.WindowID = win.ID
											break
										}
									}
								}
							} else {
								// fmt.Printf("[Show] Note %s: Could not get details for window ID %d: %v\n", sn.Note.UUID[:8], win.ID, err)
							}
						}
					} else {
						// fmt.Printf("[Show] Note %s: Error getting windows: %v\n", sn.Note.UUID[:8], err)
					}
				} else {
					// fmt.Printf("[Show] Note %s: Window ID already assigned: %d\n", sn.Note.UUID[:8], sn.WindowID)
				}

				// Move window to saved position (same logic as buildNote)
				if sn.WindowID != 0 {
					err := MoveWindow(sn.WindowID, restorePos[0], restorePos[1])
					if err == nil {
						sn.WinMain.SetOpacity(1.0) // Make window visible after moving
					} else {
						// Fallback to GTK Move() (might not work on Wayland but worth trying)
						sn.WinMain.Move(restorePos[0], restorePos[1])
						sn.WinMain.SetOpacity(1.0) // Make window visible after moving
					}
				} else {
					// Fallback to GTK Move() (might not work on Wayland but worth trying)
					sn.WinMain.Move(restorePos[0], restorePos[1])
					sn.WinMain.SetOpacity(1.0) // Make window visible after moving
				}
				// Update note after positioning (called regardless of which path was taken)
				sn.UpdateNote()

				return false // Don't repeat
			})
		} else {
			// On X11 or extension not available, use GTK Move() immediately (same as buildNote)
			glib.IdleAdd(func() bool {
				sn.WinMain.Move(restorePos[0], restorePos[1])
				sn.WinMain.SetOpacity(1.0) // Make window visible after moving
				// Update note after positioning
				sn.UpdateNote()
				return false // Don't repeat
			})
		}
	} else {
		sn.buildNote()
	}
}

func (sn *StickyNote) Hide() {
	// Cancel any pending save timeout
	if sn.saveTimeoutID != 0 {
		glib.SourceRemove(sn.saveTimeoutID)
		sn.saveTimeoutID = 0
	}
	if sn.WinMain != nil {
		// Reset WindowID because it will be invalid after hiding
		// The window will get a new ID when shown again, and we'll match it by title
		sn.WindowID = 0
		sn.WinMain.Hide()
	}
}

func (sn *StickyNote) UpdateNote() {
	start, end := sn.BBody.GetBounds()
	text, _ := sn.BBody.GetText(start, end, true)
	sn.Note.Update(text)

	// Update position and size
	if sn.WinMain != nil {
		// Try window-calls first (works on Wayland)
		if IsWindowCallsAvailable() && sn.WindowID != 0 {
			details, err := GetWindowDetails(sn.WindowID)
			if err == nil && details != nil {
				sn.LastKnownPos = [2]int{details.X, details.Y}
				sn.LastKnownSize = [2]int{details.Width, details.Height}
				return
			}
		}

		// Fallback to GTK (works on X11)
		x, y := sn.WinMain.GetPosition()
		w, h := sn.WinMain.GetSize()
		sn.LastKnownPos = [2]int{x, y}
		sn.LastKnownSize = [2]int{w, h}
	}
}

func (sn *StickyNote) Properties() map[string]interface{} {
	pos := sn.LastKnownPos
	size := sn.LastKnownSize

	if sn.WinMain != nil {
		// On Wayland, GetPosition() returns (0,0), so prioritize LastKnownPos
		// Only use GTK GetPosition if we're on X11 (position is non-zero) AND LastKnownPos is default
		x, y := sn.WinMain.GetPosition()
		w, h := sn.WinMain.GetSize()

		// Only use GTK position if it's non-zero AND LastKnownPos is default (10,10) or (0,0)
		// This ensures we preserve saved positions on Wayland
		if (x != 0 || y != 0) && (pos[0] == 10 && pos[1] == 10 || pos[0] == 0 && pos[1] == 0) {
			pos = [2]int{x, y}
		}
		if w > 1 && h > 1 {
			size = [2]int{w, h}
		}
	}

	result := map[string]interface{}{
		"position": []int{pos[0], pos[1]},
		"size":     []int{size[0], size[1]},
		"locked":   sn.Locked,
	}

	return result
}

func (sn *StickyNote) onAdd() {
	newNote := sn.NoteSet.New()
	newNote.Category = sn.Note.Category
	if newNote.GUI != nil {
		// Reload CSS and font after setting category to ensure correct colors
		newNote.GUI.LoadCSS()
		newNote.GUI.UpdateFont()
		newNote.GUI.PopulateMenu()
		// Note: Don't move the new note - let Show() handle positioning
	}
}

func (sn *StickyNote) onDelete() {
	// Cancel any pending save timeout
	if sn.saveTimeoutID != 0 {
		glib.SourceRemove(sn.saveTimeoutID)
		sn.saveTimeoutID = 0
	}
	dialog := gtk.MessageDialogNew(sn.WinMain, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_NONE, "Are you sure you want to delete this note?")
	dialog.AddButton("Cancel", gtk.RESPONSE_REJECT)
	dialog.AddButton("Delete", gtk.RESPONSE_ACCEPT)
	response := dialog.Run()
	dialog.Destroy()

	if response == gtk.RESPONSE_ACCEPT {
		sn.Note.Delete()
		if sn.WinMain != nil {
			sn.WinMain.Destroy()
		}
		// Clear GUI reference to prevent trying to use destroyed window
		sn.Note.GUI = nil
	}
}

func (sn *StickyNote) onWindowDelete(win *gtk.Window, event *gdk.Event) bool {
	// When window is closed via window manager (like X button in Activities Overview),
	// we should delete the note
	sn.Note.Delete()
	if sn.WinMain != nil {
		sn.WinMain.Destroy()
	}
	// Clear GUI reference to prevent trying to use destroyed window
	sn.Note.GUI = nil
	// Return false to allow default handling (window destruction)
	return false
}

func (sn *StickyNote) onLockClicked() {
	sn.SetLockedState(!sn.Locked)
}

// loadIconsFromEmbedded loads icons from embedded resources and sets them on the image widgets
// Tries SVG first (better quality), then falls back to PNG
func (sn *StickyNote) loadIconsFromEmbedded(imgDropdown *gtk.Image) {
	iconMap := map[*gtk.Image]string{
		sn.ImgAdd:     "add",
		sn.ImgClose:   "close",
		sn.ImgLock:    "lock",
		sn.ImgUnlock:  "unlock",
		sn.ImgResizeR: "resizer",
	}

	// Add dropdown/menu icon if available
	if imgDropdown != nil {
		iconMap[imgDropdown] = "menu"
	}

	for img, iconBase := range iconMap {
		if img == nil {
			continue
		}

		var iconData []byte
		var err error
		var iconName string

		// Try SVG first (better quality), then fall back to PNG
		iconName = iconBase + ".svg"
		iconData, err = getEmbeddedIcon(iconName)
		if err != nil {
			// Fallback to PNG
			iconName = iconBase + ".png"
			iconData, err = getEmbeddedIcon(iconName)
		}

		if err != nil {
			// Fallback: try to load from file system (try SVG first, then PNG)
			svgPath := filepath.Join(sn.Path, "Icons", iconBase+".svg")
			pngPath := filepath.Join(sn.Path, "Icons", iconBase+".png")

			if _, err := os.Stat(svgPath); err == nil {
				if pixbuf, err := gdk.PixbufNewFromFile(svgPath); err == nil {
					img.SetFromPixbuf(pixbuf)
					continue
				}
			}
			if _, err := os.Stat(pngPath); err == nil {
				if pixbuf, err := gdk.PixbufNewFromFile(pngPath); err == nil {
					img.SetFromPixbuf(pixbuf)
				}
			}
			continue
		}

		// Load from embedded bytes using PixbufLoader
		// Don't scale - let GTK handle scaling naturally based on display DPI
		loader, err := gdk.PixbufLoaderNew()
		if err != nil {
			continue
		}

		if _, err := loader.Write(iconData); err != nil {
			loader.Close()
			continue
		}

		// Close loader to finalize pixbuf
		if err := loader.Close(); err != nil {
			continue
		}

		pixbuf, err := loader.GetPixbuf()
		if err == nil && pixbuf != nil {
			img.SetFromPixbuf(pixbuf)
		}
	}
}

func (sn *StickyNote) SetLockedState(locked bool) {
	sn.Locked = locked
	if sn.TxtNote != nil {
		sn.TxtNote.SetEditable(!locked)
		sn.TxtNote.SetCursorVisible(!locked)
	}
	if sn.BLock != nil {
		if locked {
			sn.BLock.SetImage(sn.ImgLock)
			sn.BLock.SetTooltipText("Unlock")
		} else {
			sn.BLock.SetImage(sn.ImgUnlock)
			sn.BLock.SetTooltipText("Lock")
		}
	}
}

func (sn *StickyNote) onMove(widget *gtk.EventBox, event *gdk.Event) bool {
	// Calculate and print the relative pointer position within the window (as a simple move vector).
	buttonEvent := gdk.EventButtonNewFromEvent(event)

	if buttonEvent.Button() == gdk.BUTTON_PRIMARY { // Left button
		sn.WinMain.BeginMoveDrag(buttonEvent.Button(), int(buttonEvent.XRoot()), int(buttonEvent.YRoot()), buttonEvent.Time())
	}
	return false
}

func (sn *StickyNote) onResize(widget *gtk.EventBox, event *gdk.Event) bool {
	buttonEvent := gdk.EventButtonNewFromEvent(event)
	if buttonEvent.Button() == gdk.BUTTON_PRIMARY {
		sn.WinMain.BeginResizeDrag(gdk.WINDOW_EDGE_SOUTH_EAST, buttonEvent.Button(), int(buttonEvent.XRoot()), int(buttonEvent.YRoot()), buttonEvent.Time())
	}
	return true
}

func (sn *StickyNote) onFocusOut() {
	sn.UpdateNote()
	sn.NoteSet.Save()
}

func (sn *StickyNote) onConfigure() {
	if sn.WinMain == nil {
		return
	}

	// Cancel any pending save timeout
	if sn.saveTimeoutID != 0 {
		glib.SourceRemove(sn.saveTimeoutID)
		sn.saveTimeoutID = 0
	}

	// Try to get position from window-calls extension first (works on Wayland)
	if IsWindowCallsAvailable() {

		// If we don't have a window ID yet, try to find it by matching title
		if sn.WindowID == 0 {
			expectedTitle := fmt.Sprintf("Sticky Notes - %s", sn.Note.UUID[:8])
			windows, err := GetCurrentProcessWindows()
			if err == nil && windows != nil {
				for _, win := range windows {
					// Skip if already assigned to another note
					alreadyAssigned := false
					for _, otherNote := range sn.NoteSet.Notes {
						if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
							alreadyAssigned = true
							break
						}
					}
					if alreadyAssigned {
						continue
					}

					details, err := GetWindowDetails(win.ID)
					if err == nil && details != nil {
						// Match by title (exact match)
						if details.Title == expectedTitle {
							// Double-check: make sure no other note has this ID
							conflict := false
							for _, otherNote := range sn.NoteSet.Notes {
								if otherNote.GUI != nil && otherNote.GUI.WindowID == win.ID && otherNote != sn.Note {
									conflict = true
									break
								}
							}
							if !conflict {
								sn.WindowID = win.ID
								break
							}
						}
					}
				}
			}
		}

		// If we have a window ID, get position from window-calls
		if sn.WindowID != 0 {
			details, err := GetWindowDetails(sn.WindowID)
			if err == nil && details != nil {
				newPos := [2]int{details.X, details.Y}
				newSize := [2]int{details.Width, details.Height}

				sn.LastKnownPos = newPos
				sn.LastKnownSize = newSize

				// Schedule debounced save (500ms delay)
				sn.saveTimeoutID = glib.TimeoutAdd(500, func() bool {
					sn.NoteSet.Save()
					sn.saveTimeoutID = 0
					return false // Don't repeat
				})
				return
			}
		}
	}

	// Fallback to GTK GetPosition (works on X11, returns 0,0 on Wayland)
	x, y := sn.WinMain.GetPosition()
	w, h := sn.WinMain.GetSize()

	if x != 0 || y != 0 {
		sn.LastKnownPos = [2]int{x, y}
	}
	if w > 1 && h > 1 {
		sn.LastKnownSize = [2]int{w, h}
	}

	// Schedule debounced save (500ms delay)
	sn.saveTimeoutID = glib.TimeoutAdd(500, func() bool {
		sn.NoteSet.Save()
		sn.saveTimeoutID = 0
		return false // Don't repeat
	})
}

func (sn *StickyNote) PopulateMenu() {
	// Clear existing menu items
	// Menu is a Container, so we can get children directly
	container := &gtk.Container{Widget: sn.Menu.Widget}
	children := container.GetChildren()
	if children != nil {
		children.Foreach(func(item interface{}) {
			if widget, ok := item.(gtk.IWidget); ok {
				sn.Menu.Remove(widget)
			}
		})
	}

	// Always on top (disabled on Wayland as it doesn't work)
	if !IsWayland() {
		aot, _ := gtk.CheckMenuItemNewWithLabel("Always on top")
		aot.Connect("toggled", func() {
			sn.WinMain.SetKeepAbove(aot.GetActive())
		})
		sn.Menu.Append(aot)
		aot.Show()
	}

	// Settings
	mset, _ := gtk.MenuItemNewWithLabel("Settings")
	mset.Connect("activate", func() {
		// Call ShowSettings through interface
		if indicator, ok := sn.NoteSet.Indicator.(interface{ ShowSettings() }); ok {
			indicator.ShowSettings()
		}
	})
	sn.Menu.Append(mset)
	mset.Show()

	// Separator
	sep, _ := gtk.SeparatorMenuItemNew()
	sn.Menu.Append(sep)
	sep.Show()

	// Categories
	mcats, _ := gtk.MenuItemNewWithLabel("Categories:")
	mcats.SetSensitive(false)
	sn.Menu.Append(mcats)
	mcats.Show()

	var catGroup *glib.SList
	for cid, cdata := range sn.NoteSet.Categories {
		catName := "New Category"
		if name, ok := cdata["name"].(string); ok {
			catName = name
		}
		mitem, _ := gtk.RadioMenuItemNewWithLabel(catGroup, catName)
		catID := cid // Capture for closure

		// Connect signal BEFORE setting active to avoid triggering unwanted category changes
		mitem.Connect("activate", func() {
			// Only change category if it's different (prevents PopulateMenu from changing categories)
			if sn.Note.Category != catID {
				sn.setCategory(catID)
			}
		})

		// Set active AFTER connecting signal, but the guard in setCategory will prevent unwanted changes
		if cid == sn.Note.Category {
			mitem.SetActive(true)
		}

		sn.Menu.Append(mitem)
		mitem.Show()
		catGroup, _ = mitem.GetGroup()
	}
}

func (sn *StickyNote) setCategory(cat string) {
	if !sn.NoteSet.HasCategory(cat) {
		return
	}
	// Don't change category if it's already set to this value
	// This prevents PopulateMenu from changing categories when setting radio button active
	if sn.Note.Category == cat {
		return
	}
	sn.Note.Category = cat
	sn.LoadCSS()
	sn.UpdateFont()
	// Save the category change to disk
	sn.NoteSet.Save()
}

func (sn *StickyNote) onPopupMenu() {
	// Connect to menu hide signal to clear button's active state
	// This prevents the button from staying in pressed/active state
	if !sn.menuHideConnected {
		sn.Menu.Connect("hide", func() {
			// Clear button's active/pressed state
			if sn.BMenu != nil {
				// Use glib.IdleAdd to ensure this runs after the menu is fully hidden
				glib.IdleAdd(func() bool {
					// Remove focus from button to clear visual active state
					if sn.BMenu.HasFocus() {
						sn.WinMain.GrabFocus()
					}
					// Force button state update - set to normal state
					sn.BMenu.SetStateFlags(gtk.STATE_FLAG_NORMAL, true)
					// Force a redraw to clear the pressed appearance
					sn.BMenu.QueueDraw()
					return false // Don't repeat
				})
			}
		})
		sn.menuHideConnected = true
	}

	// Show menu
	sn.Menu.PopupAtWidget(sn.BMenu, gdk.GDK_GRAVITY_SOUTH_EAST, gdk.GDK_GRAVITY_NORTH_WEST, nil)
}

func (sn *StickyNote) LoadCSS() {
	// Load CSS template from embedded resources or file system
	var cssTemplate string
	if globalResourceGetter != nil {
		if content, err := globalResourceGetter.GetEmbeddedCSS("style.css"); err == nil {
			cssTemplate = content
		}
	}

	// Fallback to file system if embedded not available
	if cssTemplate == "" {
		cssPath := filepath.Join(sn.Path, "style.css")
		cssData, err := os.ReadFile(cssPath)
		if err != nil {
			return
		}
		cssTemplate = string(cssData)
	}

	// Get colors from category
	// Always try to get category properties, even if category is empty (will use default)
	bgHSVInterface := sn.Note.CatProp("bgcolor_hsv")
	textColorInterface := sn.Note.CatProp("textcolor")

	// Convert interface{} to []float64
	var bgHSV []float64
	if bgHSVInterface != nil {
		if bgHSVList, ok := bgHSVInterface.([]interface{}); ok && len(bgHSVList) >= 3 {
			bgHSV = make([]float64, 3)
			if h, ok := bgHSVList[0].(float64); ok {
				bgHSV[0] = h
			}
			if s, ok := bgHSVList[1].(float64); ok {
				bgHSV[1] = s
			}
			if v, ok := bgHSVList[2].(float64); ok {
				bgHSV[2] = v
			}
		} else if bgHSVList, ok := bgHSVInterface.([]float64); ok && len(bgHSVList) >= 3 {
			bgHSV = bgHSVList
		}
	}
	// Use default if not found or invalid
	if len(bgHSV) < 3 {
		bgHSV = []float64{48.0 / 360, 1, 1} // Default
	}

	var textColor []float64
	if textColorInterface != nil {
		if textColorList, ok := textColorInterface.([]interface{}); ok && len(textColorList) >= 3 {
			textColor = make([]float64, 3)
			if r, ok := textColorList[0].(float64); ok {
				textColor[0] = r
			}
			if g, ok := textColorList[1].(float64); ok {
				textColor[1] = g
			}
			if b, ok := textColorList[2].(float64); ok {
				textColor[2] = b
			}
		} else if textColorList, ok := textColorInterface.([]float64); ok && len(textColorList) >= 3 {
			textColor = textColorList
		}
	}
	// Use default if not found or invalid
	if len(textColor) < 3 {
		textColor = []float64{32.0 / 255, 32.0 / 255, 32.0 / 255} // Default
	}

	// Convert HSV to RGB
	bgRGB := hsvToRGB(bgHSV[0], bgHSV[1], bgHSV[2])
	bgHex := rgbToHex(bgRGB[0], bgRGB[1], bgRGB[2])
	textHex := rgbToHex(textColor[0], textColor[1], textColor[2])

	// Substitute in template
	css := strings.ReplaceAll(cssTemplate, "$bgcolor_hex", bgHex)
	css = strings.ReplaceAll(css, "$text_color", textHex)

	// Create provider if it doesn't exist (for cases where LoadCSS is called before buildNote completes)
	if sn.CSSProvider == nil {
		sn.CSSProvider, _ = gtk.CssProviderNew()
	}

	// Load the CSS data into the provider
	if err := sn.CSSProvider.LoadFromData(css); err != nil {
		return
	}

	// If provider is not yet added to contexts (e.g., called from Show() or setCategory()),
	// add it now. Otherwise, the data update will automatically refresh the styles.
	winContext, _ := sn.WinMain.GetStyleContext()
	txtContext, _ := sn.TxtNote.GetStyleContext()

	// Check if provider is already added by trying to remove it
	// If it's not added, this is a no-op, then we add it
	winContext.RemoveProvider(sn.CSSProvider)
	txtContext.RemoveProvider(sn.CSSProvider)
	winContext.AddProvider(sn.CSSProvider, gtk.STYLE_PROVIDER_PRIORITY_USER)
	txtContext.AddProvider(sn.CSSProvider, gtk.STYLE_PROVIDER_PRIORITY_USER)

	// Force a redraw to apply the CSS
	sn.WinMain.QueueDraw()
	sn.TxtNote.QueueDraw()
}

func (sn *StickyNote) UpdateFont() {
	fontName := ""
	if font, ok := sn.Note.CatProp("font").(string); ok {
		fontName = font
	}
	if fontName == "" {
		fontName = "Sans 12"
	}

	// Apply font through CSS
	// Note: OverrideFont is deprecated in GTK3, use CSS instead
	// We'll add font styling to the CSS provider
	context, _ := sn.TxtNote.GetStyleContext()
	context.AddClass("custom-font")
	// Font will be applied via CSS in the style.css template
}

// Helper functions
func getObject[T any](builder *gtk.Builder, name string) (T, error) {
	obj, err := builder.GetObject(name)
	if err != nil {
		var zero T
		return zero, err
	}
	return obj.(T), nil
}

func getBasePath() string {
	// Try to get path from executable
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)

		// First, check if UI files exist in the same directory as the executable
		// This handles the case when running from the build directory
		uiPath := filepath.Join(dir, "StickyNotes.ui")
		if info, err := os.Stat(uiPath); err == nil && !info.IsDir() {
			return dir
		}

		// Check if we're running from AppImage
		// AppImage extracts to /tmp/.mount_* or /tmp/appimage_extracted_*
		// and executable is at usr/bin/indicator-stickynotes
		if strings.Contains(dir, ".mount_") || strings.Contains(dir, "appimage_extracted_") {
			// We're in usr/bin, go up to usr, then to usr/share/indicator-stickynotes
			usrDir := filepath.Join(dir, "..")
			shareDir := filepath.Join(usrDir, "share", "indicator-stickynotes")
			if info, err := os.Stat(shareDir); err == nil && info.IsDir() {
				return shareDir
			}
		}

		// Check if we're in AppDir (during build/testing)
		if strings.Contains(dir, "AppDir") {
			// If we're in AppDir/usr/bin, go to AppDir/usr/share/indicator-stickynotes
			if strings.HasSuffix(dir, "usr/bin") {
				return filepath.Join(dir, "..", "share", "indicator-stickynotes")
			}
			// If we're in AppDir root, go to AppDir/usr/share/indicator-stickynotes
			return filepath.Join(dir, "usr/share/indicator-stickynotes")
		}

		// If executable is in golang directory, use that directory
		if strings.HasSuffix(dir, "golang") || filepath.Base(dir) == "golang" {
			return dir
		}

		// Check if we're in usr/bin (installed system-wide)
		if strings.HasSuffix(dir, "usr/bin") || strings.HasSuffix(dir, "bin") {
			// Try /usr/share/indicator-stickynotes
			shareDir := "/usr/share/indicator-stickynotes"
			if info, err := os.Stat(shareDir); err == nil && info.IsDir() {
				return shareDir
			}
		}

		// Otherwise, try parent directory
		return filepath.Join(dir, "..")
	}

	// Fallback - try to find golang directory relative to current working directory
	if wd, err := os.Getwd(); err == nil {
		if strings.Contains(wd, "golang") {
			return wd
		}
		golangPath := filepath.Join(wd, "golang")
		if info, err := os.Stat(golangPath); err == nil && info.IsDir() {
			return golangPath
		}
	}

	// Last resort
	return "."
}

// GetBasePath is exported for use in main package
func GetBasePath() string {
	return getBasePath()
}

func hsvToRGB(h, s, v float64) [3]float64 {
	// Ensure h is in [0, 1) range
	for h < 0 {
		h += 1
	}
	for h >= 1 {
		h -= 1
	}

	// Standard HSV to RGB conversion
	h6 := h * 6
	i := int(h6)
	f := h6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}

	// Clamp values to [0, 1] range
	if r < 0 {
		r = 0
	} else if r > 1 {
		r = 1
	}
	if g < 0 {
		g = 0
	} else if g > 1 {
		g = 1
	}
	if b < 0 {
		b = 0
	} else if b > 1 {
		b = 1
	}

	return [3]float64{r, g, b}
}

func rgbToHex(r, g, b float64) string {
	return fmt.Sprintf("#%02x%02x%02x", int(r*255), int(g*255), int(b*255))
}
