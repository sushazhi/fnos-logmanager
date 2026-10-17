//go:build !windows

package services

import "os"

// readMountTable 读取挂载表原始内容。
//
// 优先 /proc/self/mountinfo：它是内核直接给出的权威列表，且一定与当前挂载命名
// 空间一致。/proc/mounts 作为兜底（老内核，或 /proc/self 被限制的环境）。
//
// 本应用以 root 身份在宿主机上运行，因此这两条路径都能读到；读不到时返回错误即
// 代表「不知道」，由 isMount 退回设备号比较，绝不把「读不到」当成「没挂载」。
func readMountTable() (string, error) {
	if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil && len(data) > 0 {
		return string(data), nil
	}
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
