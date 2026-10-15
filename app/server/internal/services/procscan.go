package services

import (
	"os"
	"path"
)

// ---------------------------------------------------------------------------
// /proc 枚举的可移植核心
// ---------------------------------------------------------------------------
//
// 真机上这部分代码跑在 Linux，但开发机是 Windows，"遇到真实 /proc 会怎样" 恰恰
// 是整套判定里最需要验证的部分。因此把目录遍历与软链接解析抽成参数注入的形式：
// 生产环境传入 os.ReadDir/os.Readlink 与 "/proc"，测试传入构造好的假 /proc 树。
//
// readDir 的签名与 os.ReadDir 一致，readlink 与 os.Readlink 一致，
// 这样 systemtmp_linux.go 可以直接把它们传进来，不需要任何适配层。

// collectOpenPaths 遍历 procRoot 下的所有进程目录，汇总正在使用的绝对路径。
//
// 返回 nil 表示"无法判定"，调用方必须按拒绝清理处理。触发条件有两类：
//   - 连 procRoot 都打不开（例如不是 Linux、权限不足）；
//   - 一个进程目录都没识别出来（真实系统上不可能，说明读取出错）。
//
// 单个进程目录读不到不算致命：进程在枚举期间退出是常态，跳过它既正确又不影响
// 其余进程的判定。这一点很关键——若把"某个 pid 消失"当成无法判定，
// 那么在进程频繁启停的机器上清理将永远不可用。
func collectOpenPaths(
	procRoot string,
	readDir func(string) ([]os.DirEntry, error),
	readlink func(string) (string, error),
) map[string]bool {
	procs, err := readDir(procRoot)
	if err != nil {
		return nil
	}

	out := make(map[string]bool, 1024)
	seenPid := 0

	for _, p := range procs {
		if !p.IsDir() || !isNumericName(p.Name()) {
			continue
		}
		seenPid++
		// 用 path 而不是 filepath：/proc 永远是 POSIX 路径，
		// filepath.Join 在 Windows 上会拼出反斜杠，导致本地测试与真机行为不一致。
		base := path.Join(procRoot, p.Name())

		// fd 目录读不到（进程已退出）时仍继续处理 cwd/exe：
		// 一个进程可能没有任何 fd 却把目录当作工作目录。
		if fds, err := readDir(path.Join(base, "fd")); err == nil {
			for _, fd := range fds {
				target, err := readlink(path.Join(base, "fd", fd.Name()))
				if err != nil {
					continue
				}
				if clean := normalizeOpenPath(target); clean != "" {
					out[clean] = true
				}
			}
		}

		for _, link := range []string{"cwd", "exe"} {
			target, err := readlink(path.Join(base, link))
			if err != nil {
				continue
			}
			if clean := normalizeOpenPath(target); clean != "" {
				out[clean] = true
			}
		}
	}

	if seenPid == 0 {
		// 一个 pid 目录都没有，说明 procRoot 不是真的 /proc（或结构完全不认识），
		// 此时返回空集合会被误读成"没有任何文件在被使用"。
		return nil
	}
	return out
}

// normalizeOpenPath 把内核给出的软链接目标整理成可比较的绝对路径。
//
// 处理两件事：
//   - 去掉已删除文件的后缀 " (deleted)"，使「进程仍持有已 unlink 文件的 fd」
//     也能被识别为该路径在使用；
//   - 拒绝非绝对路径（套接字、管道、匿名 inode 等），它们与文件清理无关。
//
// 返回 "" 表示该目标应被忽略。
func normalizeOpenPath(target string) string {
	if target == "" || target[0] != '/' {
		return ""
	}
	return stripDeletedSuffix(target)
}

// isNumericName 报告 name 是否完全由数字组成（即 /proc 下的 pid 目录名）。
//
// 不能用 strconv.Atoi 直接判断：它会接受 "-1"、"+1" 这类带符号的写法，
// 而它们不可能是 pid 目录名。逐字节检查更严格也更直观。
func isNumericName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
}
