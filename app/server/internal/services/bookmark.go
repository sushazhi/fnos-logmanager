package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
)

// Bookmark represents a bookmarked log file or container.
type Bookmark struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	DisplayPath string `json:"displayPath,omitempty"`
	IsDocker    bool   `json:"isDocker"`
}

var (
	bookmarks      []Bookmark
	bookmarksMu    sync.RWMutex
	bookmarksDirty bool
)

func bookmarksFilePath() string {
	return filepath.Join(config.Get().DataDir, "bookmarks.json")
}

func loadBookmarks() {
	bookmarksMu.Lock()
	defer bookmarksMu.Unlock()

	data, err := os.ReadFile(bookmarksFilePath())
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("failed to read bookmarks", "error", err)
		}
		return
	}

	// Try new format (direct array) first
	if err := json.Unmarshal(data, &bookmarks); err != nil {
		// Try old format (wrapped in object: {"bookmarks": [...]})
		var oldFormat struct {
			Bookmarks []Bookmark `json:"bookmarks"`
		}
		if err2 := json.Unmarshal(data, &oldFormat); err2 != nil {
			slog.Error("failed to parse bookmarks (both formats)", "new", err, "old", err2)
			return
		}
		bookmarks = oldFormat.Bookmarks
		if bookmarks == nil {
			bookmarks = []Bookmark{}
		}
		slog.Info("migrated bookmarks from old format", "count", len(bookmarks))
	}

	// Deduplicate: keep only the first occurrence of each path+isDocker combination
	seen := make(map[string]bool)
	deduped := make([]Bookmark, 0, len(bookmarks))
	dupCount := 0
	for _, b := range bookmarks {
		key := fmt.Sprintf("%s|%v", b.Path, b.IsDocker)
		if seen[key] {
			dupCount++
			continue
		}
		seen[key] = true
		deduped = append(deduped, b)
	}
	if dupCount > 0 {
		bookmarks = deduped
		slog.Info("deduplicated bookmarks on load", "removed", dupCount, "remaining", len(bookmarks))
	}
}

func saveBookmarks() {
	bookmarksMu.RLock()
	data, err := json.MarshalIndent(bookmarks, "", "  ")
	bookmarksMu.RUnlock()

	if err != nil {
		slog.Error("failed to marshal bookmarks", "error", err)
		return
	}

	os.MkdirAll(config.Get().DataDir, 0700)
	tmpFile := bookmarksFilePath() + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		slog.Error("failed to write bookmarks", "error", err)
		return
	}
	os.Rename(tmpFile, bookmarksFilePath())
}

// LoadBookmarks returns all bookmarks with display paths resolved.
func LoadBookmarks() ([]Bookmark, error) {
	bookmarksMu.RLock()
	defer bookmarksMu.RUnlock()
	result := make([]Bookmark, len(bookmarks))
	copy(result, bookmarks)
	// P2: Fill displayPath via trim API
	trimClient := GetTrimClient()
	for i := range result {
		if !result[i].IsDocker && result[i].Path != "" {
			dp, err := trimClient.ConvertPath(result[i].Path, "")
			if err == nil && dp != "" && dp != result[i].Path {
				result[i].DisplayPath = dp
			}
		}
	}
	return result, nil
}

// AddBookmark adds a new bookmark. Returns the existing bookmark if one
// with the same path+isDocker already exists (idempotent).
func AddBookmark(name, path string, isDocker bool) (Bookmark, error) {
	// Check for existing bookmark with same path+isDocker
	bookmarksMu.RLock()
	for _, b := range bookmarks {
		if b.Path == path && b.IsDocker == isDocker {
			bookmarksMu.RUnlock()
			return b, nil // idempotent: return existing, no error
		}
	}
	bookmarksMu.RUnlock()

	id := generateID("bm")
	b := Bookmark{
		ID:       id,
		Name:     name,
		Path:     path,
		IsDocker: isDocker,
	}

	bookmarksMu.Lock()
	bookmarks = append(bookmarks, b)
	bookmarksMu.Unlock()

	saveBookmarks()
	return b, nil
}

// DeleteBookmark removes a bookmark by ID.
func DeleteBookmark(id string) bool {
	bookmarksMu.Lock()

	for i, b := range bookmarks {
		if b.ID == id {
			bookmarks = append(bookmarks[:i], bookmarks[i+1:]...)
			bookmarksMu.Unlock()
			saveBookmarks()
			return true
		}
	}
	bookmarksMu.Unlock()
	return false
}

// DeleteBookmarkByPath removes a bookmark by its path and docker flag.
// Used as a fallback when the bookmark ID is missing (e.g. legacy data).
func DeleteBookmarkByPath(path string, isDocker bool) bool {
	if path == "" {
		return false
	}
	bookmarksMu.Lock()

	for i, b := range bookmarks {
		if b.Path == path && b.IsDocker == isDocker {
			bookmarks = append(bookmarks[:i], bookmarks[i+1:]...)
			bookmarksMu.Unlock()
			saveBookmarks()
			return true
		}
	}
	bookmarksMu.Unlock()
	return false
}

// UpdateBookmark updates a bookmark's name.
func UpdateBookmark(id, name string) *Bookmark {
	bookmarksMu.Lock()

	for i, b := range bookmarks {
		if b.ID == id {
			bookmarks[i].Name = name
			copied := bookmarks[i]
			bookmarksMu.Unlock()
			saveBookmarks()
			return &copied
		}
	}
	bookmarksMu.Unlock()
	return nil
}

// generateID creates a unique identifier with the given prefix.
func generateID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixMilli(), hex.EncodeToString(b))
}

func init() {
	loadBookmarks()
}
