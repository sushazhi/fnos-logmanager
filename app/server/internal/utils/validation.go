package utils

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SafePath normalizes and validates a file path.
func SafePath(inputPath string) string {
	if inputPath == "" || len(inputPath) > 4096 {
		return ""
	}

	if strings.Contains(inputPath, "\x00") ||
		strings.Contains(inputPath, "\r") ||
		strings.Contains(inputPath, "\n") {
		return ""
	}

	normalized := filepath.Clean(inputPath)

	if strings.Contains(normalized, "..") {
		return ""
	}

	if !strings.HasPrefix(normalized, "/") {
		return ""
	}

	abs, err := filepath.Abs(normalized)
	if err != nil {
		return ""
	}

	return abs
}

// IsSymlinkPath checks if the given path is a symlink, or any component in its path is a symlink.
func IsSymlinkPath(filePath string) bool {
	// Check the target file itself
	info, err := os.Lstat(filePath)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}

	// Recursively check each path component
	parts := strings.Split(filePath, "/")
	currentPath := ""
	if filePath[0] == '/' {
		currentPath = "/"
	}

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		if currentPath == "/" {
			currentPath = "/" + part
		} else {
			currentPath = currentPath + "/" + part
		}

		info, err := os.Lstat(currentPath)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}

	return false
}

// IsAllowedPath checks if a path is within the allowed directories.
func IsAllowedPath(inputPath string, allowedDirs []string) bool {
	if inputPath == "" {
		return false
	}
	normalized := SafePath(inputPath)
	if normalized == "" {
		return false
	}

	if IsSymlinkPath(normalized) {
		return false
	}

	for _, allowedDir := range allowedDirs {
		if IsSymlinkPath(allowedDir) {
			continue
		}
		if normalized == allowedDir || strings.HasPrefix(normalized, allowedDir+"/") {
			return true
		}
	}
	return false
}

// IsValidContainerName validates a Docker container name.
func IsValidContainerName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`, name)
	return matched
}

// FormatBytes converts bytes to a human-readable string.
func FormatBytes(bytes int64) string {
	if bytes == 0 {
		return "0B"
	}
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := int(math.Floor(math.Log(float64(bytes)) / math.Log(1024)))
	if i >= len(sizes) {
		i = len(sizes) - 1
	}
	val := float64(bytes) / math.Pow(1024, float64(i))
	return fmt.Sprintf("%.2f%s", val, sizes[i])
}

// EscapeRegExp escapes special regex characters in a string.
func EscapeRegExp(str string) string {
	re := regexp.MustCompile(`[.*+?^${}()|[\]\\]`)
	return re.ReplaceAllString(str, `\$&`)
}

// IsValidURL checks if a URL is valid with http/https scheme.
func IsValidURL(rawURL string) bool {
	if rawURL == "" || len(rawURL) > 2048 {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// IsValidGitHubURL checks if a URL is a valid GitHub URL.
func IsValidGitHubURL(rawURL string) bool {
	if !IsValidURL(rawURL) {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Hostname() == "github.com" ||
		parsed.Hostname() == "api.github.com" ||
		parsed.Hostname() == "objects.githubusercontent.com"
}

// IsValidAction checks if the action string is valid.
func IsValidAction(action string) bool {
	return action == "truncate" || action == "delete" || action == "deleteUninstalled"
}

// IsValidDays checks if the days value is valid.
func IsValidDays(days int) bool {
	return days >= 1 && days <= 365
}

// IsValidThreshold checks if the threshold string is valid.
func IsValidThreshold(threshold string) bool {
	if threshold == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[0-9]+[KMGT]?$`, threshold)
	return matched
}

// Clamp restricts a value to the range [min, max].
func Clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampInt64 restricts an int64 value to the range [min, max].
func ClampInt64(value, min, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
