package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// FilesystemHandler handles filesystem browsing for the local folder picker.
type FilesystemHandler struct{}

// NewFilesystemHandler creates a new filesystem handler.
func NewFilesystemHandler() *FilesystemHandler {
	return &FilesystemHandler{}
}

// FSEntry represents a single filesystem entry (file or directory).
type FSEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

// BrowseResponse is the response for the browse endpoint.
type BrowseResponse struct {
	CurrentPath string    `json:"current_path"`
	Parent      string    `json:"parent"`
	Entries     []FSEntry `json:"entries"`
}

// Browse handles GET /api/filesystem/browse?path=C:\Users\...
// Returns the contents of the specified directory.
func (h *FilesystemHandler) Browse(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Query().Get("path")

	// Default to common starting points
	if requestedPath == "" {
		if runtime.GOOS == "windows" {
			requestedPath = "C:\\"
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				requestedPath = "/"
			} else {
				requestedPath = home
			}
		}
	}

	// Clean path
	requestedPath = filepath.Clean(requestedPath)

	// Verify the path exists and is a directory
	info, err := os.Stat(requestedPath)
	if err != nil {
		http.Error(w, `{"error":"Ruta no encontrada"}`, http.StatusNotFound)
		return
	}
	if !info.IsDir() {
		// If a file is selected, return the parent directory
		requestedPath = filepath.Dir(requestedPath)
	}

	// Read directory contents
	dirEntries, err := os.ReadDir(requestedPath)
	if err != nil {
		log.Printf("[Filesystem] Error reading directory %s: %v", requestedPath, err)
		http.Error(w, `{"error":"No se puede leer la carpeta"}`, http.StatusForbidden)
		return
	}

	// Filter by optional type parameter
	filterType := r.URL.Query().Get("type")       // "folder", "txt", or empty for all
	filterExt := r.URL.Query().Get("ext")          // e.g. ".txt"

	var entries []FSEntry
	for _, entry := range dirEntries {
		name := entry.Name()

		// Skip hidden files/folders (starting with .)
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Skip system folders on Windows
		if runtime.GOOS == "windows" {
			lower := strings.ToLower(name)
			if lower == "$recycle.bin" || lower == "system volume information" || lower == "recovery" {
				continue
			}
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(requestedPath, name)
		isDir := entry.IsDir()

		// Apply filters
		if filterType == "folder" && !isDir {
			continue
		}
		if filterExt != "" && !isDir {
			if !strings.HasSuffix(strings.ToLower(name), strings.ToLower(filterExt)) {
				continue
			}
		}

		fsEntry := FSEntry{
			Name:  name,
			Path:  fullPath,
			IsDir: isDir,
		}
		if !isDir {
			fsEntry.Size = entryInfo.Size()
		}
		entries = append(entries, fsEntry)
	}

	// Sort: directories first, then alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	// Calculate parent path
	parent := filepath.Dir(requestedPath)
	if parent == requestedPath {
		parent = "" // We're at root
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BrowseResponse{
		CurrentPath: requestedPath,
		Parent:      parent,
		Entries:     entries,
	})
}

// Drives handles GET /api/filesystem/drives (Windows only).
// Returns available drive letters.
func (h *FilesystemHandler) Drives(w http.ResponseWriter, r *http.Request) {
	var drives []FSEntry

	if runtime.GOOS == "windows" {
		for letter := 'A'; letter <= 'Z'; letter++ {
			drivePath := string(letter) + ":\\"
			if _, err := os.Stat(drivePath); err == nil {
				drives = append(drives, FSEntry{
					Name:  string(letter) + ":",
					Path:  drivePath,
					IsDir: true,
				})
			}
		}
	} else {
		drives = append(drives, FSEntry{
			Name:  "/",
			Path:  "/",
			IsDir: true,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(drives)
}
