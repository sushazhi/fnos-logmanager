package services

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// processAlive reports whether pid names a live process. It is a variable so
// each platform can install its own probe (signal 0 on Unix, a conservative stub
// on Windows) and so the pid-discovery logic below can be tested
// deterministically on any platform. The default is the safe answer: "not alive".
var processAlive = func(pid int) bool { return false }

// appPidDirs returns the directories that may hold an fnOS app's pid file, in
// priority order. The list is deliberately portable (no syscalls) so the
// "which app is running" decision — the part that actually guards deletions — can
// be unit-tested on any platform.
//
// 真机上 pid 文件的位置没有统一约定，至少存在三种写法，必须逐个检查：
//
//  1. /volN/@appdata/{app}/*.pid —— 本仓库 cmd/main 的
//     PID_FILE="${TRIM_PKGVAR}/app.pid"（TRIM_PKGVAR 即 @appdata/{app}），
//     真机上的 deepseek.harness、jdck 也是这样；
//  2. /volN/@apptemp/{app}/*.pid —— moviepilot(moviepilot-super.pid)、
//     qbittorrent(qbittorrent.pid)、transmission(transmission-daemon*.pid) 的
//     @appdata 目录下**完全没有** pid 文件，pid 只在这里。曾因只查 @appdata
//     而把这三个正在运行的应用的临时目录误判为可清理；
//  3. /var/apps/{app}            —— 老版本布局的兜底。
//
// app 名会被严格校验：清洗不掉的非法名字直接返回 nil，绝不拼接进路径。
func appPidDirs(vol, app string) []string {
	if app == "" || strings.ContainsAny(app, "/\\\x00") || strings.Contains(app, "..") {
		return nil
	}
	var dirs []string
	if vol != "" {
		dirs = append(dirs,
			filepath.Join(vol, "@appdata", app),
			filepath.Join(vol, "@apptemp", app),
		)
	}
	dirs = append(dirs, filepath.Join("/var/apps", app))
	return dirs
}

// parsePidFile extracts a pid from the raw contents of a pid file. It reports
// ok=false for anything that is not a positive integer, so a truncated or
// hand-edited pid file makes the app read as "not running" instead of panicking
// or, worse, being parsed as pid 0.
//
// Only the first line is authoritative: pid files written by shell scripts
// commonly end with a newline, and some tools append diagnostics afterwards.
func parsePidFile(raw []byte) (int, bool) {
	line := string(raw)
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, false
	}
	pid, err := strconv.Atoi(line)
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// pidFileAliveIn reports whether dir holds a *.pid file naming a live process.
// Only *.pid files are read; lock files and backups are ignored.
func pidFileAliveIn(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".pid") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if pid, ok := parsePidFile(raw); ok && processAlive(pid) {
			return true
		}
	}
	return false
}

// appProcessAlive reports whether an fnOS app is currently running.
//
// A pid file naming a still-live process means "running". Every failure mode
// (unreadable directory, unparsable pid, dead pid) reports false: when liveness
// cannot be established the app is treated as stopped, so that a missing status
// file never permanently blocks cleaning of something that is genuinely idle.
func appProcessAlive(vol, app string) bool {
	for _, dir := range appPidDirs(vol, app) {
		if pidFileAliveIn(dir) {
			return true
		}
	}
	return false
}
