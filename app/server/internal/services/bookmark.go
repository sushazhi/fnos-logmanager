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
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDocker bool   `json:"isDocker"`
}

var (
	bookmarks     []Bookmark
	bookmarksMu   sync.RWMutex
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

// LoadBookmarks returns all bookmarks.
func LoadBookmarks() ([]Bookmark, error) {
	bookmarksMu.RLock()
	defer bookmarksMu.RUnlock()
	result := make([]Bookmark, len(bookmarks))
	copy(result, bookmarks)
	return result, nil
}

// AddBookmark adds a new bookmark.
func AddBookmark(name, path string, isDocker bool) (Bookmark, error) {
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
	defer bookmarksMu.Unlock()

	for i, b := range bookmarks {
		if b.ID == id {
			bookmarks = append(bookmarks[:i], bookmarks[i+1:]...)
			saveBookmarks()
			return true
		}
	}
	return false
}

// UpdateBookmark updates a bookmark's name.
func UpdateBookmark(id, name string) *Bookmark {
	bookmarksMu.Lock()
	defer bookmarksMu.Unlock()

	for i, b := range bookmarks {
		if b.ID == id {
			bookmarks[i].Name = name
			saveBookmarks()
			return &bookmarks[i]
		}
	}
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
