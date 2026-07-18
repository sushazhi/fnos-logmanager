package middleware

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// ServeStatic matches Express's express.static(root) behavior:
// it tries to serve ANY request path from the given root directory.
// If the file exists, it's served; otherwise the request passes to the next handler.
//
// Unlike Gin's r.Static("/assets", dir) which only matches a specific prefix,
// this middleware handles cases where:
//   - The gateway prefix is unknown or mismatched
//   - URL resolution produces paths outside expected prefix
//     (e.g. browser at /app/logmanager resolves ./assets/foo.js to /app/assets/foo.js)
//   - The app is accessed behind arbitrary reverse proxy paths
//
// The walk-up strategy: if the full path doesn't resolve to a file, it strips path
// segments one by one from the LEFT, trying to find a matching file.
// (e.g. /app/logmanager/assets/foo.js → assets/foo.js → found in root)
func ServeStatic(root string) gin.HandlerFunc {
	if root == "" {
		return func(c *gin.Context) { c.Next() }
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		panic("static root directory resolve failed: " + err.Error())
	}

	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}

		reqPath := c.Request.URL.Path

		// Skip API paths
		if strings.HasPrefix(reqPath, "/api") {
			c.Next()
			return
		}

		cleaned := path.Clean(reqPath)
		if cleaned == "." || cleaned == "/" {
			c.Next()
			return
		}

		// Build all candidate paths by walking up from full path
		// e.g. /app/logmanager/assets/foo.js produces:
		//   root/app/logmanager/assets/foo.js → not found
		//   root/logmanager/assets/foo.js      → not found
		//   root/assets/foo.js                 → FOUND ✓
		if file, found := findFile(absRoot, cleaned); found {
			c.File(file)
			c.Abort()
			return
		}

		c.Next()
	}
}

// findFile walks up the path segments trying to find an existing file.
func findFile(absRoot, reqPath string) (string, bool) {
	// Normalize to forward slashes for consistent segment splitting
	normalized := filepath.ToSlash(reqPath)
	normalized = strings.TrimPrefix(normalized, "/")

	// Split into segments
	segments := strings.Split(normalized, "/")

	// Try from longest to shortest suffix
	for start := 0; start < len(segments); start++ {
		suffix := filepath.Join(segments[start:]...)
		fsPath := filepath.Join(absRoot, suffix)

		// Path traversal check
		cleanPath := filepath.Clean(fsPath)
		if !strings.HasPrefix(cleanPath, absRoot) {
			continue
		}

		info, err := os.Stat(cleanPath)
		if err == nil && !info.IsDir() {
			return cleanPath, true
		}
	}

	return "", false
}

var (
	indexHTMLCache []byte
	indexHTMLOnce  sync.Once
)

// ServeIndexWithBase serves index.html with an injected <base> tag so that
// relative asset URLs (from Vite's base: './') resolve against the gateway
// prefix rather than the current browser URL path.
//
// This prevents the classic SPA bug: when the browser URL is /app/logmanager
// (no trailing slash), ./assets/foo.css resolves to /app/assets/foo.css
// instead of /app/logmanager/assets/foo.css.
func ServeIndexWithBase(uiDir, baseHref string) gin.HandlerFunc {
	if baseHref == "" || baseHref == "/" {
		// No base tag needed, serve directly
		htmlPath := filepath.Join(uiDir, "index.html")
		return func(c *gin.Context) {
			c.File(htmlPath)
		}
	}

	// Ensure baseHref has a trailing slash
	if !strings.HasSuffix(baseHref, "/") {
		baseHref += "/"
	}

	// Build the cached version once
	indexHTMLOnce.Do(func() {
		htmlPath := filepath.Join(uiDir, "index.html")
		htmlData, err := os.ReadFile(htmlPath)
		if err != nil {
			return
		}

		baseTag := fmt.Sprintf(`<base href="%s">`, baseHref)
		modified := strings.Replace(string(htmlData), "<head>", "<head>"+baseTag, 1)
		if modified != string(htmlData) {
			indexHTMLCache = []byte(modified)
		}
	})

	return func(c *gin.Context) {
		if len(indexHTMLCache) > 0 {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTMLCache)
		} else {
			c.File(filepath.Join(uiDir, "index.html"))
		}
	}
}
