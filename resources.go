package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/StickyNotes.ui assets/GlobalDialogs.ui assets/SettingsCategory.ui assets/style.css assets/style_global.css
var uiFiles embed.FS

//go:embed assets/Icons
var iconFiles embed.FS

var embeddedResourcesPath string

// initEmbeddedResources extracts embedded resources to a user cache directory
// and returns the path. This should be called once at application startup.
func initEmbeddedResources() (string, error) {
	if embeddedResourcesPath != "" {
		return embeddedResourcesPath, nil
	}

	// Use user cache directory (e.g., ~/.cache/go-indicator-stickynotes/resources)
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		// Fallback to temp directory if UserCacheDir fails
		tmpDir, fallbackErr := os.MkdirTemp("", "go-indicator-stickynotes-*")
		if fallbackErr != nil {
			return "", fmt.Errorf("failed to get cache dir: %w, fallback also failed: %v", err, fallbackErr)
		}
		embeddedResourcesPath = tmpDir
		return embeddedResourcesPath, nil
	}

	// Create resources directory in user cache
	resourcesDir := filepath.Join(cacheDir, "go-indicator-stickynotes", "resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create resources directory: %w", err)
	}

	embeddedResourcesPath = resourcesDir

	// Extract UI files
	uiFilesToExtract := map[string]string{
		"assets/StickyNotes.ui":      "StickyNotes.ui",
		"assets/GlobalDialogs.ui":    "GlobalDialogs.ui",
		"assets/SettingsCategory.ui": "SettingsCategory.ui",
		"assets/style.css":           "style.css",
		"assets/style_global.css":    "style_global.css",
	}

	for embedPath, fileName := range uiFilesToExtract {
		data, err := uiFiles.ReadFile(embedPath)
		if err != nil {
			// If file doesn't exist in embed, skip it (fallback to file system)
			continue
		}

		destPath := filepath.Join(embeddedResourcesPath, fileName)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return "", fmt.Errorf("failed to write %s: %w", fileName, err)
		}
	}

	// Extract icon files
	iconsDir := filepath.Join(embeddedResourcesPath, "Icons")
	if err := os.MkdirAll(iconsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create Icons directory: %w", err)
	}

	err = fs.WalkDir(iconFiles, "assets/Icons", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Strip "assets/" prefix from path for destination
		relPath := path
		if strings.HasPrefix(relPath, "assets/") {
			relPath = strings.TrimPrefix(relPath, "assets/")
		}

		if d.IsDir() {
			// Create subdirectories
			destDir := filepath.Join(embeddedResourcesPath, relPath)
			return os.MkdirAll(destDir, 0755)
		}

		// Read and write file
		data, err := iconFiles.ReadFile(path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(embeddedResourcesPath, relPath)
		return os.WriteFile(destPath, data, 0644)
	})

	if err != nil {
		return "", fmt.Errorf("failed to extract icons: %w", err)
	}

	return embeddedResourcesPath, nil
}

// getEmbeddedResourcesPath returns the path to extracted embedded resources.
// Returns empty string if resources haven't been initialized yet.
func getEmbeddedResourcesPath() string {
	return embeddedResourcesPath
}

// cleanupEmbeddedResources removes the cache directory with embedded resources.
// Note: This is optional - the cache directory can persist across sessions.
// If you want to keep resources cached, you can make this a no-op.
func cleanupEmbeddedResources() error {
	if embeddedResourcesPath == "" {
		return nil
	}

	// Option 1: Remove resources on exit (cleanup)
	// err := os.RemoveAll(embeddedResourcesPath)
	
	// Option 2: Keep resources cached (faster startup next time)
	// Just clear the path variable, don't delete the directory
	embeddedResourcesPath = ""
	return nil
}
