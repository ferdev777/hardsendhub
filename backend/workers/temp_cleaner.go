package workers

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// TempCleaner periodically removes old temporary files.
type TempCleaner struct {
	tempDir  string
	maxAge   time.Duration
	interval time.Duration
}

// NewTempCleaner creates a cleaner that deletes files older than maxAge, checking every interval.
func NewTempCleaner(tempDir string, maxAgeDays int, checkIntervalHours int) *TempCleaner {
	return &TempCleaner{
		tempDir:  tempDir,
		maxAge:   time.Duration(maxAgeDays) * 24 * time.Hour,
		interval: time.Duration(checkIntervalHours) * time.Hour,
	}
}

// Start begins the periodic cleanup in a background goroutine.
func (c *TempCleaner) Start() {
	log.Printf("[TempCleaner] Started. Cleaning files older than %d days every %d hours from %s",
		int(c.maxAge.Hours()/24), int(c.interval.Hours()), c.tempDir)

	// Run initial cleanup
	go func() {
		c.cleanup()

		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for range ticker.C {
			c.cleanup()
		}
	}()
}

// cleanup walks the temp directory and removes old files and empty directories.
func (c *TempCleaner) cleanup() {
	cutoff := time.Now().Add(-c.maxAge)
	var removedFiles, removedDirs int

	err := filepath.Walk(c.tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip files we can't access
		}

		// Skip the root temp directory itself
		if path == c.tempDir {
			return nil
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) {
			if removeErr := os.Remove(path); removeErr == nil {
				removedFiles++
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("[TempCleaner] Error walking temp directory: %v", err)
	}

	// Second pass: remove empty directories
	if removedFiles > 0 {
		filepath.Walk(c.tempDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || path == c.tempDir || !info.IsDir() {
				return nil
			}

			entries, readErr := os.ReadDir(path)
			if readErr == nil && len(entries) == 0 {
				if os.Remove(path) == nil {
					removedDirs++
				}
			}
			return nil
		})
	}

	if removedFiles > 0 || removedDirs > 0 {
		log.Printf("[TempCleaner] Cleaned up %d files and %d directories older than %d days",
			removedFiles, removedDirs, int(c.maxAge.Hours()/24))
	}
}
