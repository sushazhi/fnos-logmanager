//go:build !windows

package services

import (
	"os"
	"syscall"
)

// diskUsageOf returns (total, free, available) bytes for the filesystem holding
// path. `free` is the raw free space; `available` is what a non-root process can
// actually use (Bavail), which is what the UI should display as usable space.
func diskUsageOf(path string) (total uint64, free uint64, avail uint64, err error) {
	var st syscall.Statfs_t
	if err = syscall.Statfs(path, &st); err != nil {
		// Fall back to os.Statfs-equivalent behaviour by reporting the error.
		return 0, 0, 0, err
	}
	bsize := uint64(st.Bsize)
	return st.Blocks * bsize, st.Bfree * bsize, st.Bavail * bsize, nil
}

// deviceOf returns the filesystem device id holding path.
//
// Comparing two paths' ids answers "are these on the same filesystem?" without
// parsing /proc/mounts and without relying on mount-point naming: it also
// distinguishes a real directory from a tmpfs mounted over it, which is the
// question that matters for /tmp (see systemTempOnSystemDisk).
//
// The id comes from stat(2)'s st_dev, not statfs(2): Linux's Statfs_t carries
// Fsid (a filesystem *identifier*) but no device number, so statfs cannot answer
// this question at all. For a mount point, stat returns the device of the mounted
// filesystem, which is exactly what "is /tmp on the system disk" means.
func deviceOf(path string) (uint64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, errDeviceInfoUnavailable
	}
	return uint64(st.Dev), nil
}

// currentUID returns the effective uid as a decimal string, used as the recycle
// bin owner when the request carries no fnOS gateway uid (standalone mode).
func currentUID() string {
	return itoa(os.Getuid())
}

// processAlive reports whether pid names a live process. signal 0 performs only
// the existence and permission checks and sends no signal.
func init() {
	processAlive = func(pid int) bool {
		return syscall.Kill(pid, 0) == nil
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
