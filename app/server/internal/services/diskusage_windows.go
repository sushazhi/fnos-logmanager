//go:build windows

package services

// diskUsageOf has no Statfs equivalent on Windows; the space-clean feature
// targets fnOS (Linux). The stub keeps cross-platform builds compiling.
func diskUsageOf(path string) (total uint64, free uint64, avail uint64, err error) {
	return 0, 0, 0, nil
}

// deviceOf has no Windows equivalent; returning an error keeps the caller's
// fail-closed path (make no claim about where the temp roots live) in effect.
func deviceOf(path string) (uint64, error) {
	return 0, errDeviceInfoUnavailable
}

// currentUID has no meaning on Windows.
func currentUID() string { return "" }

// There is no signal-0 equivalent on Windows, so processAlive keeps the
// conservative default declared in apppid.go (always "not alive"). That keeps the
// portable pid-directory logic unit-testable on a development machine while never
// claiming a process is alive when it cannot actually be checked.
