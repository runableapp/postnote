package main

import (
	"embed"
	"fmt"
)

//go:embed assets/StickyNotes.ui assets/GlobalDialogs.ui assets/SettingsCategory.ui assets/style.css assets/style_global.css
var uiFiles embed.FS

//go:embed assets/Icons
var iconFiles embed.FS

// GetEmbeddedUI returns the UI file content as a string from embedded resources.
// Returns empty string and error if file not found.
func GetEmbeddedUI(filename string) (string, error) {
	// Map common filenames to embed paths
	embedPath := "assets/" + filename
	data, err := uiFiles.ReadFile(embedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded UI file %s: %w", filename, err)
	}
	return string(data), nil
}

// GetEmbeddedCSS returns the CSS file content as a string from embedded resources.
// Returns empty string and error if file not found.
func GetEmbeddedCSS(filename string) (string, error) {
	embedPath := "assets/" + filename
	data, err := uiFiles.ReadFile(embedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded CSS file %s: %w", filename, err)
	}
	return string(data), nil
}

// GetEmbeddedIcon returns the icon file content as bytes from embedded resources.
// iconPath should be relative to Icons directory (e.g., "indicator-stickynotes.png" or "add.png").
// Returns nil and error if file not found.
func GetEmbeddedIcon(iconPath string) ([]byte, error) {
	embedPath := "assets/Icons/" + iconPath
	data, err := iconFiles.ReadFile(embedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded icon %s: %w", iconPath, err)
	}
	return data, nil
}
