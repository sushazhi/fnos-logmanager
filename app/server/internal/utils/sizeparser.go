package utils

import (
	"regexp"
	"strings"
)

// ParseSizeThreshold parses a size string (e.g., "10M", "1G") into bytes.
func ParseSizeThreshold(threshold string) int64 {
	if threshold == "" {
		return 0
	}

	re := regexp.MustCompile(`^([0-9]+)([KMGT])?$`)
	matches := re.FindStringSubmatch(strings.ToUpper(threshold))
	if len(matches) < 2 {
		return 0
	}

	value := parseInt64(matches[1])
	if len(matches) < 3 || matches[2] == "" {
		return value
	}

	switch matches[2] {
	case "K":
		return value * 1024
	case "M":
		return value * 1024 * 1024
	case "G":
		return value * 1024 * 1024 * 1024
	case "T":
		return value * 1024 * 1024 * 1024 * 1024
	default:
		return value
	}
}

func parseInt64(s string) int64 {
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		} else {
			break
		}
	}
	return n
}
