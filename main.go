package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"indicator-stickynotes/stickynotes"

	"github.com/dawidd6/go-appindicator"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// embeddedResourceGetter implements stickynotes.ResourceGetter interface
type embeddedResourceGetter struct{}

func (g *embeddedResourceGetter) GetEmbeddedUI(filename string) (string, error) {
	return GetEmbeddedUI(filename)
}

func (g *embeddedResourceGetter) GetEmbeddedCSS(filename string) (string, error) {
	return GetEmbeddedCSS(filename)
}

func (g *embeddedResourceGetter) GetEmbeddedIcon(iconPath string) ([]byte, error) {
	return GetEmbeddedIcon(iconPath)
}

// IndicatorStickyNotes manages the system tray indicator
type IndicatorStickyNotes struct {
	Args      *Args
	DataFile  string
	NoteSet   *stickynotes.NoteSet
	Indicator *appindicator.Indicator
	Menu      *gtk.Menu
}

type Args struct {
	Dev bool
}

func main() {
	// Initialize GTK
	gtk.Init(nil)

	// Set up embedded resource getter for stickynotes package
	// This allows stickynotes to access embedded resources without importing main
	stickynotes.SetResourceGetter(&embeddedResourceGetter{})

	// Parse arguments
	args := &Args{}
	flag.BoolVar(&args.Dev, "d", false, "use the development data file")
	flag.Parse()

	// Determine data file
	dataFile := stickynotes.SettingsFile
	if args.Dev {
		dataFile = stickynotes.DebugSettingsFile
	}

	// Create indicator
	indicator := NewIndicatorStickyNotes(args, dataFile)

	// Load global CSS
	stickynotes.LoadGlobalCSS()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		indicator.Save()
		gtk.MainQuit()
	}()

	// Run GTK main loop
	gtk.Main()

	// Final save
	indicator.Save()
}

func NewIndicatorStickyNotes(args *Args, dataFile string) *IndicatorStickyNotes {
	ind := &IndicatorStickyNotes{
		Args:     args,
		DataFile: dataFile,
	}

	// Initialize NoteSet
	ind.NoteSet = stickynotes.NewNoteSet(dataFile, ind)

	// Try to open existing data
	if err := ind.NoteSet.Open(); err != nil {
		if os.IsNotExist(err) {
			ind.NoteSet.LoadFresh()
		} else {
			// Show error dialog
			dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_NONE, "Error reading data file. Do you want to backup the current data?")
			dialog.AddButton("Cancel", gtk.RESPONSE_REJECT)
			dialog.AddButton("Backup", gtk.RESPONSE_ACCEPT)
			response := dialog.Run()
			dialog.Destroy()

			if response == gtk.RESPONSE_ACCEPT {
				ind.BackupDataFile()
			}
			ind.NoteSet.LoadFresh()
		}
	}

	// Show all notes if they were visible previously
	if allVisible, ok := ind.NoteSet.Properties["all_visible"].(bool); ok && allVisible {
		ind.NoteSet.ShowAll()

		// After showing all notes, assign window IDs with a delay to ensure windows are realized
		if stickynotes.IsWindowCallsAvailable() {
			glib.TimeoutAdd(1000, func() bool {
				ind.NoteSet.AssignWindowIDs()
				return false // Don't repeat
			})
		}
	}

	// Note: We don't need periodic position updates because onConfigure() handles
	// position updates when windows are moved or resized. This avoids unnecessary
	// D-Bus calls every 2 seconds.
	// If you need periodic updates for other reasons, uncomment the following:
	// if stickynotes.IsWindowCallsAvailable() {
	// 	ind.startPositionUpdates()
	// }

	// Create AppIndicator
	ind.createIndicator()

	return ind
}

// startPositionUpdates starts periodic position updates using the window-calls extension
// This must be called from the main GTK thread
func (ind *IndicatorStickyNotes) startPositionUpdates() {
	// Use glib timeout to update positions every 2 seconds
	// This ensures we're on the main GTK thread
	glib.TimeoutAdd(2000, func() bool {
		ind.NoteSet.UpdateNotePositionsFromWindowCalls()
		return true // Continue calling
	})
}

func (ind *IndicatorStickyNotes) createIndicator() {
	// Create AppIndicator
	ind.Indicator = appindicator.New("indicator-stickynotes", "indicator-stickynotes-mono", appindicator.CategoryApplicationStatus)

	// AppIndicator requires a file system path for icons, so we need to extract the indicator icon
	// to a temporary location. Try embedded first, then fallback to file system.
	iconPath := ind.getIndicatorIconPath()
	if iconPath != "" {
		// Extract base name without extension for SetIcon
		baseName := strings.TrimSuffix(filepath.Base(iconPath), filepath.Ext(iconPath))
		ind.Indicator.SetIconThemePath(filepath.Dir(iconPath))
		ind.Indicator.SetIcon(baseName)
	} else {
		// Fallback to file system path
		fsIconPath := filepath.Join(stickynotes.GetBasePath(), "Icons")
		ind.Indicator.SetIconThemePath(fsIconPath)
		ind.Indicator.SetIcon("indicator-stickynotes-mono")
	}

	ind.Indicator.SetStatus(appindicator.StatusActive)
	ind.Indicator.SetTitle("Sticky Notes")

	// Create menu
	ind.createMenu()

	// Set menu
	ind.Indicator.SetMenu(ind.Menu)

	// Set secondary activate target (middle click)
	ind.connectSecondaryActivate()
}

// getIndicatorIconPath extracts the indicator icon to a temporary directory and returns the path.
// Returns empty string if extraction fails (will fallback to file system).
func (ind *IndicatorStickyNotes) getIndicatorIconPath() string {
	// Try different icon name variations (AppIndicator expects "indicator-stickynotes-mono")
	// On Wayland, use blue icon; otherwise use default yellow icon
	var iconNames []string
	if stickynotes.IsWayland() {
		iconNames = []string{
			"indicator-stickynotes-wayland.svg", // Bright green icon for Wayland
			"indicator-stickynotes.svg",        // Fallback to default
			"indicator-stickynotes.png",
			"indicator-stickynotes-greyscale.svg",
			"indicator-stickynotes-light.svg",
		}
	} else {
		iconNames = []string{
			"indicator-stickynotes.svg", // Try SVG first (better quality)
			"indicator-stickynotes.png",
			"indicator-stickynotes-greyscale.svg",
			"indicator-stickynotes-light.svg",
		}
	}

	var iconData []byte
	var err error

	for _, name := range iconNames {
		iconData, err = GetEmbeddedIcon(name)
		if err == nil {
			break
		}
	}

	if iconData == nil {
		return ""
	}

	// Create temp directory for indicator icon
	tmpDir, err := os.MkdirTemp("", "go-indicator-stickynotes-icon-*")
	if err != nil {
		return ""
	}

	// AppIndicator expects "indicator-stickynotes-mono" as the icon name
	// Determine extension from iconData (SVG starts with <?xml or <svg, PNG starts with PNG signature)
	ext := ".svg"
	if len(iconData) > 3 && string(iconData[1:4]) == "PNG" {
		ext = ".png"
	}

	iconPath := filepath.Join(tmpDir, "indicator-stickynotes-mono"+ext)
	if err := os.WriteFile(iconPath, iconData, 0644); err != nil {
		os.RemoveAll(tmpDir)
		return ""
	}

	return iconPath
}

func (ind *IndicatorStickyNotes) connectSecondaryActivate() {
	if allVisible, ok := ind.NoteSet.Properties["all_visible"].(bool); ok && allVisible {
		// Find Hide All menu item
		children := ind.Menu.GetChildren()
		if children != nil {
			children.Foreach(func(item interface{}) {
				if menuItem, ok := item.(*gtk.MenuItem); ok {
					label := menuItem.GetLabel()
					if label == "Hide All" {
						ind.Indicator.SetSecondaryActivateTarget(menuItem)
					}
				}
			})
		}
	} else {
		// Find Show All menu item
		children := ind.Menu.GetChildren()
		if children != nil {
			children.Foreach(func(item interface{}) {
				if menuItem, ok := item.(*gtk.MenuItem); ok {
					label := menuItem.GetLabel()
					if label == "Show All" {
						ind.Indicator.SetSecondaryActivateTarget(menuItem)
					}
				}
			})
		}
	}
}

func (ind *IndicatorStickyNotes) createMenu() {
	ind.Menu, _ = gtk.MenuNew()

	// New Note
	mNewNote, _ := gtk.MenuItemNewWithLabel("New Note")
	mNewNote.Connect("activate", ind.NewNote)
	ind.Menu.Append(mNewNote)
	mNewNote.Show()

	// Separator
	sep, _ := gtk.SeparatorMenuItemNew()
	ind.Menu.Append(sep)
	sep.Show()

	// Show All
	mShowAll, _ := gtk.MenuItemNewWithLabel("Show All")
	mShowAll.Connect("activate", ind.ShowAll)
	ind.Menu.Append(mShowAll)
	mShowAll.Show()

	// Hide All
	mHideAll, _ := gtk.MenuItemNewWithLabel("Hide All")
	mHideAll.Connect("activate", ind.HideAll)
	ind.Menu.Append(mHideAll)
	mHideAll.Show()

	// Separator
	sep, _ = gtk.SeparatorMenuItemNew()
	ind.Menu.Append(sep)
	sep.Show()

	// Lock All
	mLockAll, _ := gtk.MenuItemNewWithLabel("Lock All")
	mLockAll.Connect("activate", ind.LockAll)
	ind.Menu.Append(mLockAll)
	mLockAll.Show()

	// Unlock All
	mUnlockAll, _ := gtk.MenuItemNewWithLabel("Unlock All")
	mUnlockAll.Connect("activate", ind.UnlockAll)
	ind.Menu.Append(mUnlockAll)
	mUnlockAll.Show()

	// Separator
	sep, _ = gtk.SeparatorMenuItemNew()
	ind.Menu.Append(sep)
	sep.Show()

	// Export Data
	mExport, _ := gtk.MenuItemNewWithLabel("Export Data")
	mExport.Connect("activate", ind.ExportDataFile)
	ind.Menu.Append(mExport)
	mExport.Show()

	// Import Data
	mImport, _ := gtk.MenuItemNewWithLabel("Import Data")
	mImport.Connect("activate", ind.ImportDataFile)
	ind.Menu.Append(mImport)
	mImport.Show()

	// Separator
	sep, _ = gtk.SeparatorMenuItemNew()
	ind.Menu.Append(sep)
	sep.Show()

	// About
	mAbout, _ := gtk.MenuItemNewWithLabel("About")
	mAbout.Connect("activate", ind.ShowAbout)
	ind.Menu.Append(mAbout)
	mAbout.Show()

	// Settings
	mSettings, _ := gtk.MenuItemNewWithLabel("Settings")
	mSettings.Connect("activate", ind.ShowSettings)
	ind.Menu.Append(mSettings)
	mSettings.Show()

	// Separator
	sep, _ = gtk.SeparatorMenuItemNew()
	ind.Menu.Append(sep)
	sep.Show()

	// Quit
	mQuit, _ := gtk.MenuItemNewWithLabel("Quit")
	mQuit.Connect("activate", func() {
		ind.Save()
		gtk.MainQuit()
	})
	ind.Menu.Append(mQuit)
	mQuit.Show()
}

func (ind *IndicatorStickyNotes) NewNote() {
	ind.NoteSet.New()
}

func (ind *IndicatorStickyNotes) ShowAll() {
	ind.NoteSet.ShowAll()
	ind.connectSecondaryActivate()
}

func (ind *IndicatorStickyNotes) HideAll() {
	ind.NoteSet.HideAll()
	ind.connectSecondaryActivate()
}

func (ind *IndicatorStickyNotes) LockAll() {
	for _, note := range ind.NoteSet.Notes {
		note.SetLockedState(true)
	}
	ind.Save()
}

func (ind *IndicatorStickyNotes) UnlockAll() {
	for _, note := range ind.NoteSet.Notes {
		note.SetLockedState(false)
	}
	ind.Save()
}

func (ind *IndicatorStickyNotes) BackupDataFile() {
	dialog, _ := gtk.FileChooserDialogNewWith2Buttons("Export Data", nil, gtk.FILE_CHOOSER_ACTION_SAVE, "Cancel", gtk.RESPONSE_CANCEL, "Save", gtk.RESPONSE_ACCEPT)
	dialog.SetDoOverwriteConfirmation(true)
	response := dialog.Run()
	backupFile := dialog.GetFilename()
	dialog.Destroy()

	if response == gtk.RESPONSE_ACCEPT && backupFile != "" {
		srcPath := ind.DataFile
		if srcPath[0] == '~' {
			home, _ := os.UserHomeDir()
			srcPath = filepath.Join(home, srcPath[2:])
		}
		data, err := os.ReadFile(srcPath)
		if err == nil {
			os.WriteFile(backupFile, data, 0644)
		}
	}
}

func (ind *IndicatorStickyNotes) ExportDataFile() {
	ind.BackupDataFile()
}

func (ind *IndicatorStickyNotes) ImportDataFile() {
	dialog, _ := gtk.FileChooserDialogNewWith2Buttons("Import Data", nil, gtk.FILE_CHOOSER_ACTION_OPEN, "Cancel", gtk.RESPONSE_CANCEL, "Open", gtk.RESPONSE_ACCEPT)
	response := dialog.Run()
	importFile := dialog.GetFilename()
	dialog.Destroy()

	if response == gtk.RESPONSE_ACCEPT && importFile != "" {
		data, err := os.ReadFile(importFile)
		if err == nil {
			ind.NoteSet.Merge(string(data))
		} else {
			dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_CLOSE, "Error importing data.")
			dialog.Run()
			dialog.Destroy()
		}
	}
}

func (ind *IndicatorStickyNotes) ShowAbout() {
	// Load about dialog from embedded UI file
	uiContent, err := GetEmbeddedUI("GlobalDialogs.ui")
	var builder *gtk.Builder
	if err != nil {
		// Fallback to file system
		uiPath := filepath.Join(stickynotes.GetBasePath(), "GlobalDialogs.ui")
		builder, err = gtk.BuilderNewFromFile(uiPath)
		if err != nil {
			fmt.Printf("Error loading UI file: %v\n", err)
			return
		}
	} else {
		// Use in-memory API
		builder, err = gtk.BuilderNewFromString(uiContent)
		if err != nil {
			fmt.Printf("Error loading UI from embedded resources: %v\n", err)
			return
		}
	}

	obj, err := builder.GetObject("AboutWindow")
	if err != nil {
		fmt.Printf("Error getting AboutWindow: %v\n", err)
		return
	}

	aboutDialog := obj.(*gtk.Dialog)

	// Set icon for About tab
	if imgObj, err := builder.GetObject("imgAboutIcon"); err == nil && imgObj != nil {
		img := imgObj.(*gtk.Image)
		// Try embedded icon first
		iconData, err := GetEmbeddedIcon("indicator-stickynotes.png")
		if err == nil {
			// Load from bytes using PixbufLoader
			loader, err := gdk.PixbufLoaderNew()
			if err == nil {
				if _, err := loader.Write(iconData); err == nil {
					loader.Close()
					if pixbuf, err := loader.GetPixbuf(); err == nil {
						img.SetFromPixbuf(pixbuf)
					}
				}
			}
		} else {
			// Fallback to file system
			iconPath := filepath.Join(stickynotes.GetBasePath(), "Icons", "indicator-stickynotes.png")
			if _, err := os.Stat(iconPath); err == nil {
				if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
					img.SetFromPixbuf(pixbuf)
				}
			}
		}
	}

	// Get text views for each tab
	tvAboutObj, _ := builder.GetObject("tvAbout")
	tvCreditObj, _ := builder.GetObject("tvCredit")
	tvLicenseObj, _ := builder.GetObject("tvLicense")

	var tvAbout, tvCredit, tvLicense *gtk.TextView
	if tvAboutObj != nil {
		tvAbout = tvAboutObj.(*gtk.TextView)
	}
	if tvCreditObj != nil {
		tvCredit = tvCreditObj.(*gtk.TextView)
	}
	if tvLicenseObj != nil {
		tvLicense = tvLicenseObj.(*gtk.TextView)
	}

	// Set About tab text (centered)
	aboutText := `Go Indicator Stickynotes
0.1a

Keyboard shortcuts:
Ctrl + W:  Delete note
Ctrl + L:  Lock note
Ctrl + N:  New note

Due to Wayland restrictions, window positions cannot be saved. Installing the 
Window Calls GNOME extension 
(https://extensions.gnome.org/extension/4724/window-calls/) 
enables window position saving.

Copyleft 🄯 2025 Runable.App`

	// Set Credit tab text (centered)
	creditText := `Indicator Stickynotes was originally written in Python by Umang Varma.
Go Indicator Stickynotes is a modern rewrite in Go for Linux on Wayland, developed with AI.
The design, color scheme, window layout, and icons are reused from Indicator Stickynotes.`

	// Set License tab text (centered)
	licenseText := `Go indicator-stickynotes is free and open-source software, released for unrestricted use.
Feel free to use, modify, and distribute it as you wish.`

	// Get text buffers and set text
	if tvAbout != nil {
		buffer, _ := tvAbout.GetBuffer()
		buffer.SetText(aboutText)
	}
	if tvCredit != nil {
		buffer, _ := tvCredit.GetBuffer()
		buffer.SetText(creditText)
	}
	if tvLicense != nil {
		buffer, _ := tvLicense.GetBuffer()
		buffer.SetText(licenseText)
	}

	// Connect close button
	if btnObj, err := builder.GetObject("bAboutClose"); err == nil && btnObj != nil {
		btn := btnObj.(*gtk.Button)
		btn.Connect("clicked", func() {
			aboutDialog.Response(gtk.RESPONSE_CLOSE)
		})
	}

	aboutDialog.Run()
	aboutDialog.Destroy()
}

func (ind *IndicatorStickyNotes) ShowSettings() {
	stickynotes.NewSettingsDialog(ind.NoteSet)
	ind.NoteSet.Save()
}

func (ind *IndicatorStickyNotes) Save() {
	// Update all note positions before saving
	for _, note := range ind.NoteSet.Notes {
		if note.GUI != nil {
			note.GUI.UpdateNote()
		}
	}
	ind.NoteSet.Save()
}
