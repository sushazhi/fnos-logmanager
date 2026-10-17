//go:build windows

package services

// readMountTable 在 Windows 上不可用：既没有 /proc，也没有与 st_dev 等价的挂载点
// 权威列表。返回错误即代表「不知道」，调用方据此退回设备号比较（本平台上
// deviceOf 同样不可用，于是最终保留候选，不会误删真实磁盘）。
func readMountTable() (string, error) {
	return "", errMountTableUnavailable
}
