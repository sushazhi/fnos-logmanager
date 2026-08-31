//go:build !windows

package services

import (
	"os"
	"strconv"
	"syscall"
)

// fileOwnerUID returns the numeric owner uid of path ("" on failure).
func fileOwnerUID(path string) string {
	fi, err := os.Lstat(path)
	if err != nil {
		return ""
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return strconv.FormatUint(uint64(st.Uid), 10)
	}
	return ""
}
