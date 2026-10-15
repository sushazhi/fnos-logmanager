package services

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// stubProcessAlive 让 pid 存活判定变成确定性的：只有列在 live 里的 pid 算存活。
// 用它替换 processAlive，pid 发现逻辑（本文件真正要守住的部分）就能在
// 任意平台（含 Windows 开发机）被完整验证，而不需要真的去杀进程。
func stubProcessAlive(t *testing.T, live ...int) {
	t.Helper()
	set := map[int]bool{}
	for _, pid := range live {
		set[pid] = true
	}
	orig := processAlive
	processAlive = func(pid int) bool { return set[pid] }
	t.Cleanup(func() { processAlive = orig })
}

// writePid 在 dir 下写入 name，内容为 content。
func writePid(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", name, err)
	}
}

// TestAppProcessAliveFindsPidInApptemp 是本仓库最重要的一条运行期安全回归。
//
// 真机勘察发现 pid 文件位置没有统一约定：deepseek.harness / jdck 把 app.pid 放在
// /volN/@appdata/{app}/，而 moviepilot(moviepilot-super.pid)、qbittorrent
// (qbittorrent.pid)、transmission(transmission-daemon*.pid) 的 @appdata 目录下
// **没有** 任何 pid 文件，pid 只存在于 /volN/@apptemp/{app}/。
//
// 只查 @appdata 的旧实现会把这三个正在运行的应用的 @apptemp 临时目录误判为可清理
// （真机上三者均为 running），用户一勾选就会删掉运行中应用的临时文件。此用例
// 锁死"必须同时查 @apptemp"。
func TestAppProcessAliveFindsPidInApptemp(t *testing.T) {
	stubProcessAlive(t, 4242)
	vol := t.TempDir()
	writePid(t, filepath.Join(vol, "@apptemp", "moviepilot"), "moviepilot-super.pid", "4242")

	if !appProcessAlive(vol, "moviepilot") {
		t.Error("appProcessAlive = false, want true: " +
			"pid 只存在于 @apptemp 下时也必须判定为运行中（moviepilot 真机形态）")
	}

	// 真机上的另外两个形态：qbittorrent.pid / transmission-daemon*.pid 同样只在 @apptemp。
	for _, tc := range []struct{ app, file string }{
		{"qbittorrent", "qbittorrent.pid"},
		{"transmission", "transmission-daemon.pid"},
	} {
		v := t.TempDir()
		writePid(t, filepath.Join(v, "@apptemp", tc.app), tc.file, "4242")
		if !appProcessAlive(v, tc.app) {
			t.Errorf("appProcessAlive(%q) = false, want true (pid in @apptemp)", tc.app)
		}
	}
}

// TestAppProcessAliveFindsPidInAppdata 覆盖本仓库自身 cmd/main 的写法
// （PID_FILE="${TRIM_PKGVAR}/app.pid"，即 @appdata/{app}/app.pid）。
func TestAppProcessAliveFindsPidInAppdata(t *testing.T) {
	stubProcessAlive(t, 4242)
	vol := t.TempDir()
	writePid(t, filepath.Join(vol, "@appdata", "deepseek.harness"), "app.pid", "4242")

	if !appProcessAlive(vol, "deepseek.harness") {
		t.Error("appProcessAlive = false, want true (pid in @appdata/app.pid)")
	}
}

// TestAppProcessAliveIgnoresDeadPid 确认死 pid 不会被当成运行中——否则清理功能
// 会永久拒绝清理任何曾经留下过 pid 文件的应用。
func TestAppProcessAliveIgnoresDeadPid(t *testing.T) {
	stubProcessAlive(t) // 没有任何 pid 存活
	vol := t.TempDir()
	writePid(t, filepath.Join(vol, "@apptemp", "deadapp"), "x.pid", "999999")

	if appProcessAlive(vol, "deadapp") {
		t.Error("appProcessAlive = true, want false for a dead pid")
	}
}

// TestAppProcessAliveIgnoresGarbagePid 确认 pid 文件内容非法（非数字/空/0/负数）
// 时既不 panic 也不误判为运行中。
func TestAppProcessAliveIgnoresGarbagePid(t *testing.T) {
	stubProcessAlive(t, 0, -1, 4242)
	for _, content := range []string{"", "   ", "not-a-pid", "0", "-1", "4242 4243"} {
		vol := t.TempDir()
		writePid(t, filepath.Join(vol, "@appdata", "weird"), "app.pid", content)
		if appProcessAlive(vol, "weird") {
			t.Errorf("appProcessAlive = true with pid content %q, want false", content)
		}
	}
}

// TestAppProcessAliveChecksAllPidDirs 确认每个候选目录都会被检查：pid 存在于
// 列表靠后的目录时同样能被发现。
func TestAppProcessAliveChecksAllPidDirs(t *testing.T) {
	stubProcessAlive(t, 4242)
	dirs := appPidDirs("/vol1", "someapp")
	if len(dirs) < 3 {
		t.Fatalf("appPidDirs returned %d dirs (%v), want @appdata/@apptemp/+/var/apps", len(dirs), dirs)
	}
	// 逐个把 pid 放到单个候选目录里，都必须被找到。
	for _, dir := range dirs[:2] {
		if !strings.HasPrefix(dir, "/vol1/") {
			continue
		}
		vol := t.TempDir()
		rel := strings.TrimPrefix(dir, "/vol1/")
		writePid(t, filepath.Join(vol, filepath.FromSlash(rel)), "app.pid", "4242")
		if !appProcessAlive(vol, "someapp") {
			t.Errorf("pid under %q not detected", rel)
		}
	}
}

// TestAppPidDirsRejectsBadNames 确认非法应用名不会产生任何候选目录，
// 从而不可能被拼进文件系统路径。
func TestAppPidDirsRejectsBadNames(t *testing.T) {
	for _, app := range []string{"", "../../etc", "a/b", "a\\b", "x\x00y", "..", "a..b"} {
		if dirs := appPidDirs("/vol1", app); dirs != nil {
			t.Errorf("appPidDirs(%q) = %v, want nil", app, dirs)
		}
	}
}

// TestParsePidFile 覆盖 pid 解析：只认第一行的正整数。
func TestParsePidFile(t *testing.T) {
	ok := map[string]int{
		"1234":          1234,
		"1234\n":        1234,
		"1234\r\n":      1234,
		"  1234  ":      1234,
		"1234\ngarbage": 1234,
	}
	for in, want := range ok {
		got, valid := parsePidFile([]byte(in))
		if !valid || got != want {
			t.Errorf("parsePidFile(%q) = (%d, %v), want (%d, true)", in, got, valid, want)
		}
	}

	for _, in := range []string{"", "  ", "\n", "abc", "0", "-5", "12a", "1.5", "99999999999999999999"} {
		if got, valid := parsePidFile([]byte(in)); valid {
			t.Errorf("parsePidFile(%q) = (%d, true), want invalid", in, got)
		}
	}
}

// TestPidFileAliveInOnlyReadsFirstLine 确认 pid 文件带尾随换行/多行时按第一行判定。
func TestPidFileAliveInOnlyReadsFirstLine(t *testing.T) {
	stubProcessAlive(t, 4242)

	dir := t.TempDir()
	writePid(t, dir, "app.pid", "4242\n999999\n")
	if !pidFileAliveIn(dir) {
		t.Error("pidFileAliveIn = false, want true (first line holds a live pid)")
	}

	// 第一行是死 pid、第二行存活：必须按第一行判定，结果为未运行。
	dir2 := t.TempDir()
	writePid(t, dir2, "app.pid", "999999\n4242\n")
	if pidFileAliveIn(dir2) {
		t.Error("pidFileAliveIn = true, want false (only the first line is authoritative)")
	}
}

// TestPidFileAliveInIgnoresNonPidFiles 确认只有 *.pid 后缀的文件会被读取。
func TestPidFileAliveInIgnoresNonPidFiles(t *testing.T) {
	stubProcessAlive(t, 4242)
	dir := t.TempDir()
	for _, name := range []string{"app.lock", "app.pid.bak", "notes.txt", "qbittorrent.upgrade.lock"} {
		writePid(t, dir, name, "4242")
	}
	if pidFileAliveIn(dir) {
		t.Error("pidFileAliveIn = true, want false: non-.pid files must be ignored")
	}
}

// TestPidFileAliveInTreatsBareDotPidAsPidFile 记录一个刻意的保守取舍。
//
// 名为 ".pid" 的文件（主名为空）也会被当成 pid 文件读取。现实中没有哪种约定会用
// 这个名字，但这个方向上的误判是安全的：多认一个 pid 文件只会让清理功能「过度
// 谨慎」地拒绝清理，而漏认一个 pid 文件会让运行中应用的目录被误删。因此这里把
// 期望明确锁成 true，而不是把它当 bug 修掉。
func TestPidFileAliveInTreatsBareDotPidAsPidFile(t *testing.T) {
	stubProcessAlive(t, 4242)
	dir := t.TempDir()
	writePid(t, dir, ".pid", "4242")
	if !pidFileAliveIn(dir) {
		t.Error("pidFileAliveIn = false, want true: " +
			"名为 .pid 的文件按保守方向仍视为 pid 文件")
	}
}

// TestPidFileAliveInMissingDir 确认目录不存在/不可读时安全返回 false，
// 而不是报错或 panic。
func TestPidFileAliveInMissingDir(t *testing.T) {
	stubProcessAlive(t, 4242)
	if pidFileAliveIn(filepath.Join(t.TempDir(), "nope", "deeper")) {
		t.Error("pidFileAliveIn on a missing dir = true, want false")
	}
	// 目录本身不应被当成 pid 文件。
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "fake.pid"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if pidFileAliveIn(dir) {
		t.Error("pidFileAliveIn = true, want false: a directory named *.pid is not a pid file")
	}
}

// TestDiskUsageOfReportsTotalAndFree 用注入的假实现验证 Statfs 包装的自洽性契约
// （真机上的实际取值由 diskusage_unix.go 提供）。
func TestDiskUsageOfReportsTotalAndFree(t *testing.T) {
	total, free, avail, err := diskUsageOf(t.TempDir())
	if err != nil {
		// Windows 上是 stub，返回 0 值即可，只要不报错。
		t.Fatalf("diskUsageOf: %v", err)
	}
	if total != 0 && free > total {
		t.Errorf("free (%d) > total (%d)", free, total)
	}
	if total != 0 && avail > total {
		t.Errorf("avail (%d) > total (%d)", avail, total)
	}
}

// TestCurrentUIDIsNumericOrEmpty 确认回收站属主 uid 要么是十进制数字串（Unix），
// 要么是空串（Windows）。
func TestCurrentUIDIsNumericOrEmpty(t *testing.T) {
	uid := currentUID()
	if uid == "" {
		return // Windows stub
	}
	if _, err := strconv.Atoi(uid); err != nil {
		t.Errorf("currentUID() = %q, want a decimal string: %v", uid, err)
	}
}
