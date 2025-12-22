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

// Helper function for absolute value of integers
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// isWayland checks if the application is running on Wayland
func isWayland() bool {
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
	path := filepath.Join(getBasePath(), "style_global.css")
	cssProvider, err := gtk.CssProviderNew()
	if err != nil {
		return err
	}

	err = cssProvider.LoadFromPath(path)
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
	WindowID          uint32 // Window ID from window-calls extension (D-Bus uint32)
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

	// Load UI file
	uiPath := filepath.Join(sn.Path, "StickyNotes.ui")
	sn.Builder, err = gtk.BuilderNewFromFile(uiPath)
	if err != nil {
		fmt.Printf("Error loading UI file: %v\n", err)
		return
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
		sn.LastKnownPos = [2]int{10, 10}
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

			// Try to get window ID if not assigned yet
			if sn.WindowID == 0 {
				// Try a match by size
				w, h := sn.WinMain.GetSize()
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

						// Get details to check actual size (List() often returns 0,0)
						details, err := GetWindowDetails(win.ID)
						if err == nil && details != nil {

							// Match by size (within 10 pixels)
							if absInt(details.Width-w) < 10 && absInt(details.Height-h) < 10 {
								sn.WindowID = win.ID
								break
							} else {
							}
						} else {
						}
					}

					if sn.WindowID == 0 {
					}
				} else {
				}
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
				sn.WinMain.Move(restorePos[0], restorePos[1])
				sn.WinMain.SetOpacity(1.0) // Make window visible after moving
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
	if IsWindowCallsAvailable() {
		// Use TimeoutAdd to check position after a delay
		// We wait 1500ms to ensure both the move and assignWindowID() have completed
		glib.TimeoutAdd(1500, func() bool {

			// If Window ID is still 0, call assignWindowID() directly to get it
			if sn.WindowID == 0 {
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
	}

	// After window is shown, try to get window ID from window-calls extension
	// This allows us to track window position on Wayland
	if IsWindowCallsAvailable() {
		// Use glib.IdleAdd with a small delay to ensure window is fully realized and registered
		glib.TimeoutAdd(500, func() bool {
			sn.assignWindowID()
			return false // Don't repeat
		})
	} else {
	}
}

// assignWindowID gets and stores the window ID for this note from window-calls extension
func (sn *StickyNote) assignWindowID() {
	if sn.WindowID != 0 {
		// Already assigned
		return
	}

	windows, err := GetCurrentProcessWindows()
	if err != nil {
		return
	}

	if len(windows) == 0 {
		return
	}

	// Get current window size and position for matching
	w, h := sn.WinMain.GetSize()
	expectedX, expectedY := sn.LastKnownPos[0], sn.LastKnownPos[1]

	// Try to find the best matching window
	bestMatch := struct {
		windowID uint32
		score    int
	}{windowID: 0, score: 0}

	for _, win := range windows {
		// Skip if this window ID is already assigned to another note
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

		// Use the window info directly from ListWindows (already has position and size)
		// This avoids an extra D-Bus call to GetWindowDetails
		details := &WindowDetails{
			ID:      win.ID,
			PID:     win.PID,
			X:       win.X,
			Y:       win.Y,
			Width:   win.Width,
			Height:  win.Height,
			WMClass: win.WMClass,
			Title:   win.Title,
		}

		// If position/size are 0, try to get details (might not be in List response)
		if details.Width == 0 && details.Height == 0 {
			detailsFromDBus, err := GetWindowDetails(win.ID)
			if err != nil || detailsFromDBus == nil {
				continue
			}
			details = detailsFromDBus
		}

		// Calculate match score
		score := 0
		// Size match (within 10 pixels) = 10 points
		if absInt(details.Width-w) < 10 && absInt(details.Height-h) < 10 {
			score += 10
		}
		// Position match (within 50 pixels) = 5 points
		if expectedX != 0 || expectedY != 0 {
			if absInt(details.X-expectedX) < 50 && absInt(details.Y-expectedY) < 50 {
				score += 5
			}
		}

		if score > bestMatch.score {
			bestMatch.windowID = win.ID
			bestMatch.score = score
		}
	}

	if bestMatch.windowID != 0 {
		sn.WindowID = bestMatch.windowID
	}
}

func (sn *StickyNote) Show() {
	if sn.WinMain != nil {
		sn.UpdateNote()
		// Reload CSS when showing existing note to ensure correct colors
		sn.LoadCSS()
		sn.UpdateFont()
		sn.WinMain.ShowAll()

		// If window ID not assigned yet, try to assign it
		if IsWindowCallsAvailable() && sn.WindowID == 0 {
			glib.TimeoutAdd(500, func() bool {
				sn.assignWindowID()
				return false // Don't repeat
			})
		}
	} else {
		sn.buildNote()
	}
}

func (sn *StickyNote) Hide() {
	if sn.WinMain != nil {
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
		newNote.GUI.PopulateMenu()
		x, y := sn.WinMain.GetPosition()
		_, h := sn.WinMain.GetSize()
		newY := y + h + 10
		newNote.GUI.WinMain.Move(x, newY)
	}
}

func (sn *StickyNote) onDelete() {
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
	}
}

func (sn *StickyNote) onLockClicked() {
	sn.SetLockedState(!sn.Locked)
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

	// Try to get position from window-calls extension first (works on Wayland)
	if IsWindowCallsAvailable() {

		// If we don't have a window ID yet, try to find it
		if sn.WindowID == 0 {
			windows, err := GetCurrentProcessWindows()
			if err == nil && windows != nil {
				// Try to match by size
				w, h := sn.WinMain.GetSize()
				for _, win := range windows {
					details, err := GetWindowDetails(win.ID)
					if err == nil && details != nil {
						// Match by size (within 10 pixels)
						if absInt(details.Width-w) < 10 && absInt(details.Height-h) < 10 {
							sn.WindowID = win.ID
							break
						}
					}
				}
			} else {
			}
		} else {
		}

		// If we have a window ID, get position from window-calls
		if sn.WindowID != 0 {
			details, err := GetWindowDetails(sn.WindowID)
			if err == nil && details != nil {
				newPos := [2]int{details.X, details.Y}
				newSize := [2]int{details.Width, details.Height}

				sn.LastKnownPos = newPos
				sn.LastKnownSize = newSize

				sn.NoteSet.Save()
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

	sn.NoteSet.Save()
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
	if !isWayland() {
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
	// Load CSS template
	cssPath := filepath.Join(sn.Path, "style.css")
	cssData, err := os.ReadFile(cssPath)
	if err != nil {
		return
	}

	cssTemplate := string(cssData)

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
	// First, check if embedded resources are available (set by main package)
	if embeddedPath := os.Getenv("GO_INDICATOR_STICKYNOTES_EMBEDDED_PATH"); embeddedPath != "" {
		// Verify embedded resources are actually there
		uiPath := filepath.Join(embeddedPath, "StickyNotes.ui")
		if info, err := os.Stat(uiPath); err == nil && !info.IsDir() {
			return embeddedPath
		}
	}

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
