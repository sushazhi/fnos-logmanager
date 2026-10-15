//go:build linux

package services

import "os"

// listOpenPathsImpl 枚举当前所有进程正在使用的路径。
//
// 解析逻辑放在可移植文件里（collectOpenPaths），这里只负责把"/proc 从哪读"和
// "怎么解析软链接"接上去，使核心判定在任何平台上都能被测到。
func listOpenPathsImpl() map[string]bool {
	return collectOpenPaths("/proc", os.ReadDir, os.Readlink)
}

// procSymlinkTarget 读取 /proc/<pid>/{cwd,exe} 与 /proc/<pid>/fd/<n> 的目标。
//
// 单独包一层，是为了让读不到的情形（进程已退出、受限的 hidepid 挂载等）
// 有统一的落点：返回错误后会被计入"信息不完整"，最终表现为拒绝清理。
func procSymlinkTarget(link string) (string, error) {
	target, err := os.Readlink(link)
	if err != nil {
		return "", err
	}
	return target, nil
}
