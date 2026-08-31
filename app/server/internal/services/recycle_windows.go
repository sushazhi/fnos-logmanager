//go:build windows

package services

// fileOwnerUID has no meaning on Windows; the cleanup feature targets fnOS
// (Linux). The stub keeps cross-platform builds compiling.
func fileOwnerUID(path string) string { return "" }
