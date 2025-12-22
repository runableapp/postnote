package stickynotes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Note represents a single sticky note
type Note struct {
	UUID         string
	Body         string
	Properties   map[string]interface{}
	Category     string
	LastModified time.Time
	GUI          *StickyNote
	NoteSet      *NoteSet
}

// NewNote creates a new note
func NewNote(content map[string]interface{}, guiClass func(*Note) *StickyNote, noteset *NoteSet, category string) *Note {
	note := &Note{
		Properties: make(map[string]interface{}),
		NoteSet:    noteset,
	}

	if content != nil {
		if uuidStr, ok := content["uuid"].(string); ok {
			note.UUID = uuidStr
		}
		if body, ok := content["body"].(string); ok {
			note.Body = body
		}
		if props, ok := content["properties"].(map[string]interface{}); ok {
			note.Properties = props
		}
		if cat, ok := content["cat"].(string); ok && cat != "" {
			note.Category = cat
		}
		if lastMod, ok := content["last_modified"].(string); ok {
			if t, err := time.Parse("2006-01-02T15:04:05", lastMod); err == nil {
				note.LastModified = t
			}
		}
	}

	// Only set category from parameter if it wasn't loaded from JSON
	if note.Category == "" {
		note.Category = category
	}

	// Don't clear category if it doesn't exist - GetCategoryProperty will handle it gracefully
	// Keep the category string so each note can have its own category

	if note.UUID == "" {
		note.UUID = uuid.New().String()
	}
	if note.LastModified.IsZero() {
		note.LastModified = time.Now()
	}

	return note
}

// Extract converts the note to a map for JSON serialization
func (n *Note) Extract() map[string]interface{} {
	if n.GUI != nil {
		n.GUI.UpdateNote()
		n.Properties = n.GUI.Properties()
	}

	return map[string]interface{}{
		"uuid":          n.UUID,
		"body":          n.Body,
		"last_modified": n.LastModified.Format("2006-01-02T15:04:05"),
		"properties":    n.Properties,
		"cat":           n.Category,
	}
}

// Update updates the note's body
func (n *Note) Update(body string) {
	n.Body = body
	n.LastModified = time.Now()
}

// Delete removes the note from its noteset
func (n *Note) Delete() {
	for i, note := range n.NoteSet.Notes {
		if note == n {
			n.NoteSet.Notes = append(n.NoteSet.Notes[:i], n.NoteSet.Notes[i+1:]...)
			break
		}
	}
	n.NoteSet.Save()
}

// Show displays the note's GUI
func (n *Note) Show() {
	if n.GUI == nil {
		n.GUI = NewStickyNote(n)
	} else {
		// Reload CSS in case category changed or CSS wasn't applied correctly
		n.GUI.LoadCSS()
		n.GUI.UpdateFont()
		n.GUI.Show()
	}
}

// Hide hides the note's GUI
func (n *Note) Hide() {
	if n.GUI != nil {
		n.GUI.Hide()
	}
}

// SetLockedState sets the locked state of the note
func (n *Note) SetLockedState(locked bool) {
	if n.GUI == nil {
		n.Properties["locked"] = locked
	} else {
		n.GUI.SetLockedState(locked)
	}
}

// CatProp gets a property of the note's category
func (n *Note) CatProp(prop string) interface{} {
	return n.NoteSet.GetCategoryProperty(n.Category, prop)
}

// NoteSet manages a collection of notes
type NoteSet struct {
	Notes      []*Note
	Properties map[string]interface{}
	Categories map[string]map[string]interface{}
	DataFile   string
	Indicator  interface{} // Use interface{} to avoid circular dependency
}

// NewNoteSet creates a new noteset
func NewNoteSet(dataFile string, indicator interface{}) *NoteSet {
	return &NoteSet{
		Notes:      make([]*Note, 0),
		Properties: make(map[string]interface{}),
		Categories: make(map[string]map[string]interface{}),
		DataFile:   dataFile,
		Indicator:  indicator,
	}
}

// Loads parses JSON and loads notes
func (ns *NoteSet) Loads(snoteset string) error {
	var notes map[string]interface{}
	if err := json.Unmarshal([]byte(snoteset), &notes); err != nil {
		return err
	}

	if props, ok := notes["properties"].(map[string]interface{}); ok {
		ns.Properties = props
	}
	if cats, ok := notes["categories"].(map[string]interface{}); ok {
		ns.Categories = make(map[string]map[string]interface{})
		for k, v := range cats {
			if catMap, ok := v.(map[string]interface{}); ok {
				ns.Categories[k] = catMap
			}
		}
	}
	if notesList, ok := notes["notes"].([]interface{}); ok {
		ns.Notes = make([]*Note, 0, len(notesList))
		for _, noteData := range notesList {
			if noteMap, ok := noteData.(map[string]interface{}); ok {
				note := NewNote(noteMap, NewStickyNote, ns, "")
				ns.Notes = append(ns.Notes, note)
			}
		}
	}

	return nil
}

// Dumps converts the noteset to JSON
func (ns *NoteSet) Dumps() string {
	notes := make([]map[string]interface{}, len(ns.Notes))
	for i, note := range ns.Notes {
		notes[i] = note.Extract()
	}

	data := map[string]interface{}{
		"notes":      notes,
		"properties": ns.Properties,
		"categories": ns.Categories,
	}

	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

// Save writes the noteset to disk
func (ns *NoteSet) Save() {
	output := ns.Dumps()
	path := ns.DataFile
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	os.WriteFile(path, []byte(output), 0644)
}

// Open reads the noteset from disk
func (ns *NoteSet) Open() error {
	path := ns.DataFile
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return ns.Loads(string(data))
}

// LoadFresh initializes an empty noteset
func (ns *NoteSet) LoadFresh() {
	ns.Loads("{}")
	ns.New()
}

// Merge merges data from another noteset
func (ns *NoteSet) Merge(data string) error {
	var jdata map[string]interface{}
	if err := json.Unmarshal([]byte(data), &jdata); err != nil {
		return err
	}

	ns.HideAll()

	if cats, ok := jdata["categories"].(map[string]interface{}); ok {
		for k, v := range cats {
			if catMap, ok := v.(map[string]interface{}); ok {
				if ns.Categories == nil {
					ns.Categories = make(map[string]map[string]interface{})
				}
				ns.Categories[k] = catMap
			}
		}
	}

	dnotes := make(map[string]*Note)
	for _, note := range ns.Notes {
		if note.UUID != "" {
			dnotes[note.UUID] = note
		}
	}

	if notesList, ok := jdata["notes"].([]interface{}); ok {
		for _, noteData := range notesList {
			if newNote, ok := noteData.(map[string]interface{}); ok {
				if uuidStr, ok := newNote["uuid"].(string); ok && uuidStr != "" {
					if orignote, exists := dnotes[uuidStr]; exists {
						if body, ok := newNote["body"].(string); ok {
							orignote.Body = body
						}
						if props, ok := newNote["properties"].(map[string]interface{}); ok {
							orignote.Properties = props
						}
						if cat, ok := newNote["cat"].(string); ok {
							orignote.Category = cat
						}
						continue
					}
				}
				note := NewNote(newNote, NewStickyNote, ns, "")
				if note.UUID == "" {
					note.UUID = uuid.New().String()
				}
				dnotes[note.UUID] = note
			}
		}
	}

	ns.Notes = make([]*Note, 0, len(dnotes))
	for _, note := range dnotes {
		ns.Notes = append(ns.Notes, note)
	}

	ns.ShowAll()
	return nil
}

// New creates a new note and adds it to the noteset
func (ns *NoteSet) New() *Note {
	defaultCat := ""
	if def, ok := ns.Properties["default_cat"].(string); ok {
		defaultCat = def
	}
	note := NewNote(nil, NewStickyNote, ns, defaultCat)
	ns.Notes = append(ns.Notes, note)
	note.Show()
	return note
}

// ShowAll shows all notes
func (ns *NoteSet) ShowAll() {
	for _, note := range ns.Notes {
		note.Show()
	}
	ns.Properties["all_visible"] = true
}

// AssignWindowIDs assigns window IDs to all notes that don't have one yet
// This should be called after all windows are shown and realized
func (ns *NoteSet) AssignWindowIDs() {
	for _, note := range ns.Notes {
		if note.GUI != nil && note.GUI.WinMain != nil && note.GUI.WindowID == 0 {
			note.GUI.assignWindowID()
		}
	}
}

// HideAll hides all notes
func (ns *NoteSet) HideAll() {
	ns.Save()
	for _, note := range ns.Notes {
		note.Hide()
	}
	ns.Properties["all_visible"] = false
}

// GetCategoryProperty gets a property of a category or the default
func (ns *NoteSet) GetCategoryProperty(cat, prop string) interface{} {
	// If category is empty, try default_cat
	if cat == "" {
		if ns.Properties["default_cat"] != nil {
			if def, ok := ns.Properties["default_cat"].(string); ok && def != "" {
				cat = def
			}
		}
	}

	// If category is specified and exists, get property from it
	// IMPORTANT: Only use the specified category, don't fall back to default_cat
	// if the category exists but property is missing
	if cat != "" {
		if ns.HasCategory(cat) {
			catData, hasCat := ns.Categories[cat]
			if hasCat {
				if val, ok := catData[prop]; ok {
					return val
				}
			}
		}
	}

	// Category doesn't exist, is empty, or property not found, use fallback
	if val, ok := FallbackProperties[prop]; ok {
		return val
	}

	return nil
}

// HasCategory checks if a category exists
func (ns *NoteSet) HasCategory(cat string) bool {
	_, ok := ns.Categories[cat]
	return ok
}
