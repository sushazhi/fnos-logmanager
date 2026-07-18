package utils

import (
	"regexp"
	"sync"

	"github.com/sushazhi/fnos-logmanager/internal/config"
)

var (
	filterEnabled = true
	filterMu      sync.RWMutex
	compiled      []*regexp.Regexp
	compileOnce   sync.Once
)

func compilePatterns() {
	compileOnce.Do(func() {
		patterns := config.Get().SensitivePatterns
		compiled = make([]*regexp.Regexp, 0, len(patterns))
		for _, p := range patterns {
			if re, err := regexp.Compile(p); err == nil {
				compiled = append(compiled, re)
			}
		}
	})
}

// IsFilterEnabled returns whether the sensitive info filter is enabled.
func IsFilterEnabled() bool {
	filterMu.RLock()
	defer filterMu.RUnlock()
	return filterEnabled
}

// SetFilterEnabled enables or disables the sensitive info filter.
func SetFilterEnabled(enabled bool) {
	filterMu.Lock()
	defer filterMu.Unlock()
	filterEnabled = enabled
}

// FilterSensitiveInfo filters sensitive information from a string.
func FilterSensitiveInfo(content string) string {
	compilePatterns()

	filterMu.RLock()
	enabled := filterEnabled
	filterMu.RUnlock()

	if !enabled {
		return content
	}

	for _, re := range compiled {
		content = re.ReplaceAllString(content, "[FILTERED]")
	}
	return content
}
