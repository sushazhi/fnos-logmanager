package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeDirEntry 实现 os.DirEntry，用于构造假的 /proc 目录列表。
type fakeDirEntry struct {
	name  string
	isDir bool
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return f.isDir }
func (f fakeDirEntry) Type() os.FileMode          { return 0 }
func (f fakeDirEntry) Info() (os.FileInfo, error) { return nil, errors.New("not implemented") }

// fakeProc 描述一棵假的 /proc 树：目录路径 -> 目录项；软链接路径 -> 目标。
type fakeProc struct {
	dirs  map[string][]string
	links map[string]string
	// badDirs 中的目录读取时报错，用于模拟权限不足。
	badDirs map[string]bool
}

func newFakeProc() *fakeProc {
	return &fakeProc{
		dirs:    map[string][]string{},
		links:   map[string]string{},
		badDirs: map[string]bool{},
	}
}

func (f *fakeProc) reader(p string) ([]os.DirEntry, error) {
	if f.badDirs[p] {
		return nil, os.ErrPermission
	}
	names, ok := f.dirs[p]
	if !ok {
		// 真实 /proc 里 fd/ 目录会随进程退出而消失：返回未找到而非空列表。
		return nil, os.ErrNotExist
	}
	out := make([]os.DirEntry, 0, len(names))
	for _, n := range names {
		out = append(out, fakeDirEntry{name: n, isDir: true})
	}
	return out, nil
}

func (f *fakeProc) readlink(p string) (string, error) {
	if t, ok := f.links[p]; ok {
		return t, nil
	}
	return "", os.ErrNotExist
}

// TestCollectOpenPathsRealisticProc 用一棵贴近真机的假 /proc 树验证解析结果。
func TestCollectOpenPathsRealisticProc(t *testing.T) {
	f := newFakeProc()
	// 根目录同时包含 pid 目录与非 pid 条目，后者必须被忽略。
	f.dirs["/proc"] = []string{"1", "428", "1337", "self", "sys", "uptime"}

	// pid 1：仅有一个已删除文件的 fd，外加 cwd/exe。
	f.dirs["/proc/1/fd"] = []string{"0", "3"}
	f.links["/proc/1/fd/0"] = "/dev/null"
	f.links["/proc/1/fd/3"] = "/tmp/inflight.log (deleted)"
	f.links["/proc/1/cwd"] = "/"
	f.links["/proc/1/exe"] = "/usr/bin/init"

	// pid 428：fd 目录读不到（模拟权限/竞争），但 cwd 指向候选清理目录。
	f.links["/proc/428/cwd"] = "/tmp/build-work"
	f.links["/proc/428/exe"] = "/usr/bin/make"

	// pid 1337：fd 指向候选目录内部的真实文件。
	f.dirs["/proc/1337/fd"] = []string{"1", "2", "7"}
	f.links["/proc/1337/fd/1"] = "/tmp/build-work/out.o"
	f.links["/proc/1337/fd/2"] = "socket:[12345]"
	f.links["/proc/1337/fd/7"] = "pipe:[999]"
	f.links["/proc/1337/cwd"] = "/vol1/@appdata/moviepilot"

	got := collectOpenPaths("/proc", f.reader, f.readlink)
	if got == nil {
		t.Fatal("识别出 pid 目录时应返回集合，不应为 nil")
	}

	wantPresent := []string{
		"/dev/null",
		"/tmp/inflight.log", // (deleted) 后缀必须被去掉
		"/",
		"/usr/bin/init",
		"/tmp/build-work",
		"/usr/bin/make",
		"/tmp/build-work/out.o",
		"/vol1/@appdata/moviepilot",
	}
	for _, p := range wantPresent {
		if !got[p] {
			t.Errorf("应包含 %q", p)
		}
	}

	wantAbsent := []string{"socket:[12345]", "pipe:[999]", "", "/tmp/inflight.log (deleted)"}
	for _, p := range wantAbsent {
		if got[p] {
			t.Errorf("不应包含 %q", p)
		}
	}
}

// TestCollectOpenPathsUnavailableReturnsNil 确认"读不到"返回 nil 而不是空集合。
// 空集合的意思是"确认没有任何进程持有文件"，把二者混同会导致误删。
func TestCollectOpenPathsUnavailableReturnsNil(t *testing.T) {
	f := newFakeProc()
	f.badDirs["/proc"] = true
	if got := collectOpenPaths("/proc", f.reader, f.readlink); got != nil {
		t.Errorf("根目录读不到时应返回 nil，实际 %v", got)
	}

	// 目录里没有任何 pid 条目：结构不认识，同样视为无法判定。
	f2 := newFakeProc()
	f2.dirs["/proc"] = []string{"self", "sys", "uptime"}
	if got := collectOpenPaths("/proc", f2.reader, f2.readlink); got != nil {
		t.Errorf("未识别出任何 pid 时应返回 nil，实际 %v", got)
	}

	// 空目录同理。
	f3 := newFakeProc()
	f3.dirs["/proc"] = []string{}
	if got := collectOpenPaths("/proc", f3.reader, f3.readlink); got != nil {
		t.Errorf("空 /proc 应返回 nil，实际 %v", got)
	}
}

// TestCollectOpenPathsSkipsVanishedProcess 确认枚举期间退出的进程被跳过，
// 而不是让整次扫描变成"无法判定"——否则进程频繁启停的机器上清理永远不可用。
func TestCollectOpenPathsSkipsVanishedProcess(t *testing.T) {
	f := newFakeProc()
	f.dirs["/proc"] = []string{"1", "2", "3"}
	// pid 1、2 已完全退出（连 cwd 都读不到），pid 3 仍在运行。
	f.dirs["/proc/3/fd"] = []string{"5"}
	f.links["/proc/3/fd/5"] = "/tmp/live.file"
	f.links["/proc/3/cwd"] = "/tmp/live.dir"

	got := collectOpenPaths("/proc", f.reader, f.readlink)
	if got == nil {
		t.Fatal("部分进程退出不应导致整体无法判定")
	}
	if !got["/tmp/live.file"] || !got["/tmp/live.dir"] {
		t.Errorf("应仍能收集到存活进程的路径，实际 %v", got)
	}
}

// TestNormalizeOpenPath 覆盖目标路径的取舍规则。
func TestNormalizeOpenPath(t *testing.T) {
	cases := map[string]string{
		"/tmp/a":                  "/tmp/a",
		"/tmp/a (deleted)":        "/tmp/a",
		"/var/tmp/x (deleted)":    "/var/tmp/x",
		"":                        "",
		"socket:[1]":              "",
		"pipe:[2]":                "",
		"anon_inode:[eventpoll]":  "",
		"relative/path":           "",
		"/tmp/with (deleted) mid": "/tmp/with (deleted) mid",
	}
	for in, want := range cases {
		if got := normalizeOpenPath(in); got != want {
			t.Errorf("normalizeOpenPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIsNumericName 确认只有纯数字目录名被当作 pid。
func TestIsNumericName(t *testing.T) {
	yes := []string{"1", "42", "1337", "0"}
	no := []string{"", "self", "sys", "1a", "a1", "-1", "1.5", " 1", "1 "}
	for _, s := range yes {
		if !isNumericName(s) {
			t.Errorf("isNumericName(%q) 应为 true", s)
		}
	}
	for _, s := range no {
		if isNumericName(s) {
			t.Errorf("isNumericName(%q) 应为 false", s)
		}
	}
}

// TestCollectOpenPathsEndToEndWithDeletionGuard 把假 /proc 的结果接到实际判定链路上，
// 验证"进程正在使用"确实会阻止删除。
//
// 分成两层来测，因为两层的真实输入形状不同：
//   - collectOpenPaths 的输入是 POSIX 路径（/proc 永远给出正斜杠绝对路径）；
//   - matchSystemTempForPath 的候选路径来自本机文件系统，开发机上是 Windows 形式。
//
// 中间那层 normalizeOpenPath 由 TestNormalizeOpenPath 单独覆盖。
func TestCollectOpenPathsEndToEndWithDeletionGuard(t *testing.T) {
	t.Run("POSIX 路径的占用判定", func(t *testing.T) {
		f := newFakeProc()
		f.dirs["/proc"] = []string{"777"}
		f.dirs["/proc/777/fd"] = []string{"4"}
		// 某进程持有 /tmp/busy 内部文件的 fd（真机常见：解压完成后仍持有映射文件）。
		f.links["/proc/777/fd/4"] = "/tmp/busy/out.o"
		f.links["/proc/777/cwd"] = "/tmp"

		open := collectOpenPaths("/proc", f.reader, f.readlink)
		if open == nil {
			t.Fatal("应识别出 pid 777")
		}
		if !spacePathInUse("/tmp/busy", open) {
			t.Error("/tmp/busy 应被判定为使用中")
		}
		if !spacePathInUse("/tmp", open) {
			t.Error("作为 cwd 的 /tmp 应被判定为使用中")
		}
		if spacePathInUse("/tmp/idle", open) {
			t.Error("/tmp/idle 不应被判定为使用中")
		}
		// 已删除文件的 fd 仍应保护其原路径。
		f2 := newFakeProc()
		f2.dirs["/proc"] = []string{"888"}
		f2.dirs["/proc/888/fd"] = []string{"9"}
		f2.links["/proc/888/fd/9"] = "/tmp/inflight/x (deleted)"
		open2 := collectOpenPaths("/proc", f2.reader, f2.readlink)
		if !spacePathInUse("/tmp/inflight", open2) {
			t.Error("持有已删除文件 fd 时仍应保护原路径")
		}
	})

	t.Run("占用判定接入清理准入", func(t *testing.T) {
		root := t.TempDir()
		stubSystemTempRoots(t, root)
		busyDir := makeTempEntry(t, root, "busy", true, 48*time.Hour)
		idleDir := makeTempEntry(t, root, "idle", true, 48*time.Hour)

		// 直接把"某进程打开了 busy 内部的文件"注入接缝，路径用本机形式，
		// 与生产环境中 matchSystemTempForPath 收到的候选路径形状一致。
		stubOpenPaths(t, filepath.Join(busyDir, "f"))

		if _, ok := matchSystemTempForPath(busyDir); ok {
			t.Error("内部文件被打开的目录必须拒绝清理")
		}
		if _, ok := matchCleanablePath(busyDir); ok {
			t.Error("matchCleanablePath 必须拒绝使用中的系统临时目录")
		}
		if _, ok := matchSystemTempForPath(idleDir); !ok {
			t.Errorf("闲置目录应放行，实际原因: %s", systemTempBlockReason(idleDir))
		}
		if _, ok := matchCleanablePath(idleDir); !ok {
			t.Error("matchCleanablePath 应放行闲置的系统临时目录")
		}
	})
}
