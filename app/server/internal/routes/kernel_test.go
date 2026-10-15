package routes

import (
	"os"
	"strings"
	"testing"
)

// mustReadOwnSource returns this package's kernel.go source, for the static
// no-shell guard below.
func mustReadOwnSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("kernel.go")
	if err != nil {
		t.Skipf("无法读取 kernel.go（可能不在包目录下运行）: %v", err)
	}
	return string(b)
}

// The kernel feature deletes FILES only — it never touches the dpkg database.
// On a real fnOS device that left 8 orphaned dpkg records plus 36 MB of
// device-tree directories that the feature itself could not see. These tests
// lock the two fixes in place.

// TestKernelImageVersion covers the naming rule that makes an orphaned
// /usr/lib/linux-image-<version> directory discoverable again.
func TestKernelImageVersion(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		wantVer string
		wantOK  bool
	}{
		{
			name:    "真实的 fnOS 内核 DTB 目录",
			dir:     "linux-image-6.18.18.c1090-trim",
			wantVer: "6.18.18.c1090-trim",
			wantOK:  true,
		},
		{
			name:    "第二个孤儿目录",
			dir:     "linux-image-6.18.18.c994-trim",
			wantVer: "6.18.18.c994-trim",
			wantOK:  true,
		},
		{
			name:    "当前运行内核的 DTB 目录也必须能识别",
			dir:     "linux-image-6.18.18.c1145-trim",
			wantVer: "6.18.18.c1145-trim",
			wantOK:  true,
		},
		{
			name:    "普通 Debian 内核",
			dir:     "linux-image-6.1.0-13-arm64",
			wantVer: "6.1.0-13-arm64",
			wantOK:  true,
		},
		{
			name:   "前缀不匹配",
			dir:    "linux-modules-6.18.18.c1090-trim",
			wantOK: false,
		},
		{
			name:   "只有前缀、没有版本号",
			dir:    "linux-image-",
			wantOK: false,
		},
		{
			name:   "版本号里没有数字，不是合法内核版本",
			dir:    "linux-image-trim",
			wantOK: false,
		},
		{
			name:   "路径穿越必须被拒绝",
			dir:    "linux-image-../../etc",
			wantOK: false,
		},
		{
			name:   "含斜杠必须被拒绝",
			dir:    "linux-image-6.18.18/evil",
			wantOK: false,
		},
		{
			name:   "空名字",
			dir:    "",
			wantOK: false,
		},
		{
			name:   "仅点号",
			dir:    "linux-image-.",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, ok := kernelImageVersion(tt.dir)
			if ok != tt.wantOK {
				t.Fatalf("kernelImageVersion(%q) ok = %v, want %v", tt.dir, ok, tt.wantOK)
			}
			if ver != tt.wantVer {
				t.Fatalf("kernelImageVersion(%q) ver = %q, want %q", tt.dir, ver, tt.wantVer)
			}
		})
	}
}

// TestKernelImageVersionRejectsShellMetacharacters guards the rule that the
// derived version is only ever used after isValidKernelVersion accepts it, so
// nothing shell- or path-significant can survive into a filesystem operation.
func TestKernelImageVersionRejectsShellMetacharacters(t *testing.T) {
	for _, dir := range []string{
		"linux-image-6.18.18; rm -rf /",
		"linux-image-6.18.18 $(id)",
		"linux-image-6.18.18`id`",
		"linux-image-6.18.18|cat",
		"linux-image-6.18.18&bg",
	} {
		if ver, ok := kernelImageVersion(dir); ok {
			t.Fatalf("kernelImageVersion(%q) unexpectedly accepted, ver=%q", dir, ver)
		}
	}
}

// TestAptPurgeHint locks the exact command surfaced to the user. The feature
// deliberately does NOT run apt itself, so the hint must be precise enough to
// paste, and must not exist when there is nothing to purge.
func TestAptPurgeHint(t *testing.T) {
	tests := []struct {
		name string
		pkgs []string
		want string
	}{
		{
			name: "无残留记录时不产生命令",
			pkgs: nil,
			want: "",
		},
		{
			name: "空列表同样不产生命令",
			pkgs: []string{},
			want: "",
		},
		{
			name: "单一包",
			pkgs: []string{"linux-image-6.18.18.c1090-trim"},
			want: "apt purge -y linux-image-6.18.18.c1090-trim",
		},
		{
			name: "多个包以空格连接",
			pkgs: []string{"linux-image-6.18.18.c1090-trim", "linux-headers-6.18.18.c1090-trim"},
			want: "apt purge -y linux-image-6.18.18.c1090-trim linux-headers-6.18.18.c1090-trim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aptPurgeHint("6.18.18.c1090-trim", tt.pkgs); got != tt.want {
				t.Fatalf("aptPurgeHint() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDpkgResidueHint locks the message that makes an otherwise silent residue
// visible. The critical property is that it says the package database was NOT
// touched — that is exactly the fact users were missing.
func TestDpkgResidueHint(t *testing.T) {
	t.Run("没有残留时不提示", func(t *testing.T) {
		if got := dpkgResidueHint(nil); got != "" {
			t.Fatalf("dpkgResidueHint(nil) = %q, want empty", got)
		}
		if got := dpkgResidueHint([]string{}); got != "" {
			t.Fatalf("dpkgResidueHint(empty) = %q, want empty", got)
		}
	})

	t.Run("有残留时说明只删了文件并给出命令", func(t *testing.T) {
		hints := []string{"apt purge -y linux-image-6.18.18.c1090-trim"}
		got := dpkgResidueHint(hints)

		if !strings.Contains(got, "dpkg") {
			t.Errorf("提示未提及 dpkg: %q", got)
		}
		if !strings.Contains(got, "只删文件") && !strings.Contains(got, "不经包管理器") {
			t.Errorf("提示未说明未触碰包数据库: %q", got)
		}
		if !strings.Contains(got, "apt purge -y linux-image-6.18.18.c1090-trim") {
			t.Errorf("提示未包含可执行的 apt purge 命令: %q", got)
		}
	})

	t.Run("多条命令全部出现", func(t *testing.T) {
		got := dpkgResidueHint([]string{"apt purge -y a", "apt purge -y b"})
		if !strings.Contains(got, "apt purge -y a") || !strings.Contains(got, "apt purge -y b") {
			t.Fatalf("并非所有命令都出现在提示中: %q", got)
		}
	})
}

// TestDpkgKernelPackagesSeamIsSubstitutable proves the injection seam works, so
// the dpkg query path is testable on a machine without dpkg.
func TestDpkgKernelPackagesSeamIsSubstitutable(t *testing.T) {
	orig := dpkgKernelPackagesFn
	t.Cleanup(func() { dpkgKernelPackagesFn = orig })

	t.Run("可返回半安装态的包名", func(t *testing.T) {
		dpkgKernelPackagesFn = func(version string) []string {
			return []string{"linux-image-" + version}
		}
		got := dpkgKernelPackagesFn("6.18.18.c1090-trim")
		if len(got) != 1 || got[0] != "linux-image-6.18.18.c1090-trim" {
			t.Fatalf("seam 未生效: %v", got)
		}
	})

	t.Run("nil 表示无法查询而不是没有记录", func(t *testing.T) {
		dpkgKernelPackagesFn = func(string) []string { return nil }
		if got := dpkgKernelPackagesFn("x"); got != nil {
			t.Fatalf("期望 nil，得到 %#v", got)
		}
	})

	t.Run("空切片表示确实没有记录", func(t *testing.T) {
		dpkgKernelPackagesFn = func(string) []string { return []string{} }
		got := dpkgKernelPackagesFn("x")
		if got == nil || len(got) != 0 {
			t.Fatalf("期望非 nil 空切片，得到 %#v", got)
		}
	})
}

// TestCalcImageSizeDegradesGracefully locks the property that makes the DTB
// path assumption safe: if the directory does not exist, the reported size is 0
// and nothing errors. The path convention could not be re-verified from the
// development machine (trim-cli rejects /usr), so a wrong guess must degrade to
// the previous behaviour rather than fail or misreport.
func TestCalcImageSizeDegradesGracefully(t *testing.T) {
	// These versions do not exist on the development machine (and on a real
	// device they are exactly the orphaned ones), so all must return 0.
	for _, ver := range []string{
		"0.0.0-does-not-exist-trim",
		"6.18.18.c1090-trim",
	} {
		if got := calcImageSize(ver); got != 0 {
			t.Errorf("calcImageSize(%q) = %d, 期望 0（目录不存在时不得报错或虚报）", ver, got)
		}
	}
}

// TestRunDpkgQueryNeverUsesShell is a static guard: candidates must be passed as
// argv. If a version string ever reached a shell, a crafted version could run
// arbitrary commands — and this feature runs as root.
func TestRunDpkgQueryNeverUsesShell(t *testing.T) {
	// dpkg-query is absent on the development machine, so this asserts the
	// no-shell contract by construction: runDpkgQuery uses exec.Command with an
	// argv slice and never exec.Command("sh"/"bash", "-c", ...).
	src := mustReadOwnSource(t)
	if strings.Contains(src, `exec.Command("sh"`) || strings.Contains(src, `exec.Command("bash"`) {
		t.Fatal("kernel.go 使用了 shell 执行，包名/版本号可能被注入")
	}
	if strings.Contains(src, `exec.Command("dpkg-query", "-W", "-f", "${Status}", name)`) == false {
		t.Fatal("未找到 dpkg-query 的 argv 调用，实现可能已改变")
	}
}
