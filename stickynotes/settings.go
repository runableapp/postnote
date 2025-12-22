package stickynotes

import (
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

// SettingsCategory manages the widgets for a single category
type SettingsCategory struct {
	SettingsDialog *SettingsDialog
	NoteSet        *NoteSet
	Cat            string
	Builder        *gtk.Builder
	CatExpander    *gtk.Expander
	LExp           *gtk.Label
	CbBG           *gtk.ColorButton
	CbText         *gtk.ColorButton
	EName          *gtk.Entry
	FbFont         *gtk.FontButton
}

// NewSettingsCategory creates a new settings category widget
func NewSettingsCategory(settingsDialog *SettingsDialog, cat string) *SettingsCategory {
	sc := &SettingsCategory{
		SettingsDialog: settingsDialog,
		NoteSet:        settingsDialog.NoteSet,
		Cat:            cat,
	}

	path := GetBasePath()
	uiPath := filepath.Join(path, "SettingsCategory.ui")
	sc.Builder, _ = gtk.BuilderNewFromFile(uiPath)
	// Connect signals manually since ConnectSignals needs a map
	sc.connectSignals()

	// Get the catExpander object
	// The UI file has winCategory as root, with catExpander as child
	// We need to extract just the expander, not the window
	sc.CatExpander, _ = getObject[*gtk.Expander](sc.Builder, "catExpander")

	// The catExpander is a child of winCategory in the UI file
	// We need to remove it from winCategory so we can add it to our box
	// This prevents the GTK-CRITICAL error about widget already having a parent
	if winCategory, err := getObject[*gtk.Window](sc.Builder, "winCategory"); err == nil {
		// Remove the expander from the window
		winCategory.Remove(sc.CatExpander)
	}
	sc.LExp, _ = getObject[*gtk.Label](sc.Builder, "lExp")
	sc.CbBG, _ = getObject[*gtk.ColorButton](sc.Builder, "cbBG")
	sc.CbText, _ = getObject[*gtk.ColorButton](sc.Builder, "cbText")
	sc.EName, _ = getObject[*gtk.Entry](sc.Builder, "eName")
	sc.FbFont, _ = getObject[*gtk.FontButton](sc.Builder, "fbFont")

	// Set initial values
	name := "New Category"
	if catData, ok := sc.NoteSet.Categories[cat]; ok {
		if n, ok := catData["name"].(string); ok {
			name = n
		}
	}
	sc.EName.SetText(name)
	sc.RefreshTitle()

	// Set background color
	bgHSV := sc.NoteSet.GetCategoryProperty(cat, "bgcolor_hsv")
	var h, s, v float64
	ok := false

	// Try to extract HSV values - handle both []interface{} (from JSON) and []float64
	if bgHSV != nil {
		switch val := bgHSV.(type) {
		case []interface{}:
			if len(val) >= 3 {
				if h1, ok1 := val[0].(float64); ok1 {
					if s1, ok2 := val[1].(float64); ok2 {
						if v1, ok3 := val[2].(float64); ok3 {
							h, s, v = h1, s1, v1
							ok = true
						}
					}
				}
			}
		case []float64:
			if len(val) >= 3 {
				h, s, v = val[0], val[1], val[2]
				ok = true
			}
		}
	}

	if ok && h >= 0 && h <= 1 && s >= 0 && s <= 1 && v >= 0 && v <= 1 {
		rgb := hsvToRGB(h, s, v)
		rgba := gdk.NewRGBA(rgb[0], rgb[1], rgb[2], 1.0)
		sc.CbBG.SetRGBA(rgba)
	} else {
		// Use default color if loading fails
		defaultHSV := []float64{48.0 / 360, 1, 1}
		rgb := hsvToRGB(defaultHSV[0], defaultHSV[1], defaultHSV[2])
		rgba := gdk.NewRGBA(rgb[0], rgb[1], rgb[2], 1.0)
		sc.CbBG.SetRGBA(rgba)
		// Also save the default if category didn't have a color
		if sc.NoteSet.Categories[cat] == nil {
			sc.NoteSet.Categories[cat] = make(map[string]interface{})
		}
		sc.NoteSet.Categories[cat]["bgcolor_hsv"] = defaultHSV
	}

	// Set text color
	textColor := sc.NoteSet.GetCategoryProperty(cat, "textcolor")
	var r, g, b float64
	ok = false

	// Try to extract RGB values - handle both []interface{} (from JSON) and []float64
	if textColor != nil {
		switch val := textColor.(type) {
		case []interface{}:
			if len(val) >= 3 {
				if r1, ok1 := val[0].(float64); ok1 {
					if g1, ok2 := val[1].(float64); ok2 {
						if b1, ok3 := val[2].(float64); ok3 {
							r, g, b = r1, g1, b1
							ok = true
						}
					}
				}
			}
		case []float64:
			if len(val) >= 3 {
				r, g, b = val[0], val[1], val[2]
				ok = true
			}
		}
	}

	if ok && r >= 0 && r <= 1 && g >= 0 && g <= 1 && b >= 0 && b <= 1 {
		rgba := gdk.NewRGBA(r, g, b, 1.0)
		sc.CbText.SetRGBA(rgba)
	} else {
		// Use default color if loading fails
		defaultColor := []float64{32.0 / 255, 32.0 / 255, 32.0 / 255}
		rgba := gdk.NewRGBA(defaultColor[0], defaultColor[1], defaultColor[2], 1.0)
		sc.CbText.SetRGBA(rgba)
		// Also save the default if category didn't have a color
		if sc.NoteSet.Categories[cat] == nil {
			sc.NoteSet.Categories[cat] = make(map[string]interface{})
		}
		sc.NoteSet.Categories[cat]["textcolor"] = defaultColor
	}

	// Set font
	fontName := ""
	if font, ok := sc.NoteSet.GetCategoryProperty(cat, "font").(string); ok {
		fontName = font
	}
	if fontName == "" {
		fontName = "Sans 12"
	}
	sc.FbFont.SetFont(fontName)

	// Connect signals
	sc.EName.Connect("changed", sc.OnENameChanged)
	sc.CbBG.Connect("color-set", sc.OnUpdateBG)
	sc.CbText.Connect("color-set", sc.OnUpdateTextColor)
	sc.FbFont.Connect("font-set", sc.OnUpdateFont)

	return sc
}

func (sc *SettingsCategory) connectSignals() {
	// Connect signals manually
	if btn, err := getObject[*gtk.ToolButton](sc.Builder, "tbMkDef"); err == nil {
		btn.Connect("clicked", sc.OnMakeDefault)
	}
	if btn, err := getObject[*gtk.ToolButton](sc.Builder, "tbDelete"); err == nil {
		btn.Connect("clicked", sc.OnDeleteCat)
	}
}

func (sc *SettingsCategory) RefreshTitle() {
	name := "New Category"
	if catData, ok := sc.NoteSet.Categories[sc.Cat]; ok {
		if n, ok := catData["name"].(string); ok {
			name = n
		}
	}
	if defaultCat, ok := sc.NoteSet.Properties["default_cat"].(string); ok && defaultCat == sc.Cat {
		name += " (Default Category)"
	}
	sc.LExp.SetText(name)
}

func (sc *SettingsCategory) OnENameChanged() {
	text, _ := sc.EName.GetText()
	if sc.NoteSet.Categories[sc.Cat] == nil {
		sc.NoteSet.Categories[sc.Cat] = make(map[string]interface{})
	}
	sc.NoteSet.Categories[sc.Cat]["name"] = text
	sc.RefreshTitle()
	// Update all note menus
	for _, note := range sc.NoteSet.Notes {
		if note.GUI != nil {
			note.GUI.PopulateMenu()
		}
	}
}

func (sc *SettingsCategory) OnUpdateBG() {
	rgba := sc.CbBG.GetRGBA()
	// Get RGB values (0.0 to 1.0 range)
	r := rgba.GetRed()
	g := rgba.GetGreen()
	b := rgba.GetBlue()

	// Clamp values to valid range
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

	// Convert RGB to HSV
	hsv := rgbToHSV(r, g, b)

	// Ensure category map exists
	if sc.NoteSet.Categories[sc.Cat] == nil {
		sc.NoteSet.Categories[sc.Cat] = make(map[string]interface{})
	}

	// Save HSV values - ensure they're in valid ranges
	h := hsv[0]
	if h < 0 {
		h = 0
	} else if h >= 1 {
		h = h - float64(int(h))
	}
	s := hsv[1]
	if s < 0 {
		s = 0
	} else if s > 1 {
		s = 1
	}
	v := hsv[2]
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}

	sc.NoteSet.Categories[sc.Cat]["bgcolor_hsv"] = []float64{h, s, v}

	// Save immediately
	sc.NoteSet.Save()

	// Update all notes
	for _, note := range sc.NoteSet.Notes {
		if note.GUI != nil {
			note.GUI.LoadCSS()
		}
	}
	// Reload global CSS
	LoadGlobalCSS()
}

func (sc *SettingsCategory) OnUpdateTextColor() {
	rgba := sc.CbText.GetRGBA()
	// Get RGB values (0.0 to 1.0 range)
	r := rgba.GetRed()
	g := rgba.GetGreen()
	b := rgba.GetBlue()

	// Clamp values to valid range
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

	// Ensure category map exists
	if sc.NoteSet.Categories[sc.Cat] == nil {
		sc.NoteSet.Categories[sc.Cat] = make(map[string]interface{})
	}

	// Save RGB values (textcolor is stored as RGB, not HSV)
	sc.NoteSet.Categories[sc.Cat]["textcolor"] = []float64{r, g, b}

	// Save immediately
	sc.NoteSet.Save()

	// Update all notes
	for _, note := range sc.NoteSet.Notes {
		if note.GUI != nil {
			note.GUI.LoadCSS()
		}
	}
}

func (sc *SettingsCategory) OnUpdateFont() {
	fontName := sc.FbFont.GetFont()
	if sc.NoteSet.Categories[sc.Cat] == nil {
		sc.NoteSet.Categories[sc.Cat] = make(map[string]interface{})
	}
	sc.NoteSet.Categories[sc.Cat]["font"] = fontName
	// Update all notes
	for _, note := range sc.NoteSet.Notes {
		if note.GUI != nil {
			note.GUI.UpdateFont()
		}
	}
}

func (sc *SettingsCategory) OnMakeDefault() {
	sc.NoteSet.Properties["default_cat"] = sc.Cat
	sc.SettingsDialog.RefreshCategoryTitles()
	for _, note := range sc.NoteSet.Notes {
		if note.GUI != nil {
			note.GUI.LoadCSS()
			note.GUI.UpdateFont()
		}
	}
}

func (sc *SettingsCategory) OnDeleteCat() {
	dialog := gtk.MessageDialogNew(sc.SettingsDialog.WSettings, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_NONE, "Are you sure you want to delete this category?")
	dialog.AddButton("Cancel", gtk.RESPONSE_REJECT)
	dialog.AddButton("Delete", gtk.RESPONSE_ACCEPT)
	response := dialog.Run()
	dialog.Destroy()

	if response == gtk.RESPONSE_ACCEPT {
		sc.SettingsDialog.DeleteCategory(sc.Cat)
	}
}

// SettingsDialog manages the settings dialog
type SettingsDialog struct {
	NoteSet       *NoteSet
	Categories    map[string]*SettingsCategory
	Builder       *gtk.Builder
	WSettings     *gtk.Dialog
	BoxCategories *gtk.Box
}

// NewSettingsDialog creates and shows the settings dialog
func NewSettingsDialog(noteset *NoteSet) *SettingsDialog {
	sd := &SettingsDialog{
		NoteSet:    noteset,
		Categories: make(map[string]*SettingsCategory),
	}

	path := GetBasePath()
	uiPath := filepath.Join(path, "GlobalDialogs.ui")
	sd.Builder, _ = gtk.BuilderNewFromFile(uiPath)
	sd.connectSignals()

	sd.WSettings, _ = getObject[*gtk.Dialog](sd.Builder, "wSettings")
	sd.BoxCategories, _ = getObject[*gtk.Box](sd.Builder, "boxCategories")

	// Clear any existing placeholders in the box (if any)
	// Note: This should be empty initially, but clear just in case
	container := &gtk.Container{Widget: sd.BoxCategories.Widget}
	children := container.GetChildren()
	if children != nil && children.Length() > 0 {
		children.Foreach(func(item interface{}) {
			if widget, ok := item.(gtk.IWidget); ok {
				sd.BoxCategories.Remove(widget)
			}
		})
	}

	// Add category widgets for all existing categories
	// Make sure we iterate in a consistent order
	cats := make([]string, 0, len(sd.NoteSet.Categories))
	for cat := range sd.NoteSet.Categories {
		cats = append(cats, cat)
	}
	for _, cat := range cats {
		sd.AddCategoryWidgets(cat)
	}

	// Show the dialog
	sd.WSettings.ShowAll()

	// Connect new category button
	if newBtn, err := getObject[*gtk.ToolButton](sd.Builder, "catNew"); err == nil {
		newBtn.Connect("clicked", sd.OnNewCategory)
	}

	sd.WSettings.Run()
	sd.WSettings.Destroy()

	return sd
}

func (sd *SettingsDialog) AddCategoryWidgets(cat string) {
	// Check if category already exists in our map
	if _, exists := sd.Categories[cat]; exists {
		return
	}

	// Create the settings category widget
	sd.Categories[cat] = NewSettingsCategory(sd, cat)

	// Check if widget already has a parent before packing
	// This prevents the GTK-CRITICAL error about widget already having a parent
	parent, err := sd.Categories[cat].CatExpander.GetParent()
	if err != nil || parent == nil {
		// Widget has no parent, safe to add
		sd.BoxCategories.PackStart(sd.Categories[cat].CatExpander, false, false, 0)
		sd.Categories[cat].CatExpander.ShowAll()
		// Force the box to update and queue a redraw
		sd.BoxCategories.ShowAll()
		sd.BoxCategories.QueueDraw()
	}
}

func (sd *SettingsDialog) OnNewCategory() {
	cid := uuid.New().String()
	sd.NoteSet.Categories[cid] = make(map[string]interface{})
	sd.AddCategoryWidgets(cid)
	// Save immediately so the category persists
	sd.NoteSet.Save()
}

func (sd *SettingsDialog) DeleteCategory(cat string) {
	delete(sd.NoteSet.Categories, cat)
	if sc, ok := sd.Categories[cat]; ok {
		sc.CatExpander.Destroy()
		delete(sd.Categories, cat)
	}
	// Update all notes
	for _, note := range sd.NoteSet.Notes {
		if note.GUI != nil {
			note.GUI.PopulateMenu()
			note.GUI.LoadCSS()
			note.GUI.UpdateFont()
		}
	}
}

func (sd *SettingsDialog) RefreshCategoryTitles() {
	for _, sc := range sd.Categories {
		sc.RefreshTitle()
	}
}

func (sd *SettingsDialog) connectSignals() {
	// Signals are connected in OnNewCategory
}

// Helper functions
func rgbToHSV(r, g, b float64) [3]float64 {
	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}
	min := r
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}

	v := max
	s := 0.0
	if max != 0 {
		s = (max - min) / max
	}

	h := 0.0
	if s != 0 {
		delta := max - min
		if r == max {
			h = (g - b) / delta
		} else if g == max {
			h = 2 + (b-r)/delta
		} else {
			h = 4 + (r-g)/delta
		}
		h /= 6
		if h < 0 {
			h += 1
		}
	}

	return [3]float64{h, s, v}
}
