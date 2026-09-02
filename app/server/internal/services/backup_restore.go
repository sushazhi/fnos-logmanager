package services

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// restoreMaxMembers caps how many tar members are enumerated in one pass so a
// pathological archive cannot pin the server in an endless scan.
const restoreMaxMembers = 20000

// restoreMu serializes restore operations: two concurrent restores writing the
// same targets would interleave half-written files.
var restoreMu sync.Mutex

// ValidateBackupFilePath validates that a path points to a .tar.gz file inside
// the configured backup base directory and returns the normalized path.
func ValidateBackupFilePath(backupPath string) (string, error) {
	safePath := utils.SafePath(backupPath)
	if safePath == "" {
		return "", fmt.Errorf("无效的备份路径")
	}
	cfg := config.Get()
	baseDir := cfg.Backup.BaseDir
	if !strings.HasSuffix(baseDir, string(os.PathSeparator)) {
		baseDir += string(os.PathSeparator)
	}
	if !strings.HasPrefix(safePath, baseDir) {
		return "", fmt.Errorf("备份文件不在备份目录下")
	}
	if !strings.HasSuffix(safePath, ".tar.gz") {
		return "", fmt.Errorf("只能是 .tar.gz 备份文件")
	}
	return safePath, nil
}

// mapTarMember maps a tar member name (backup-<ts>/vol1/@appdata/...) to its
// restore target path. It performs string-level tar-slip rejection before any
// filesystem check: only members rooted at a "backup-" segment are accepted,
// every remaining segment must be a plain name.
func mapTarMember(name string) (string, bool) {
	name = strings.TrimPrefix(name, "./")
	if name == "" || strings.Contains(name, `\`) || strings.Contains(name, "\x00") {
		return "", false
	}
	parts := strings.Split(name, "/")
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "backup-") ||
		strings.Contains(parts[0], "..") {
		return "", false
	}
	for _, seg := range parts[1:] {
		if seg == "" || seg == "." || seg == ".." || strings.Contains(seg, "\x00") {
			return "", false
		}
	}
	return "/" + strings.Join(parts[1:], "/"), true
}

// safeRestoreTarget re-checks every existing path component for symlinks
// (IsSymlinkPath stops early when the leaf does not exist, which is the normal
// case when restoring a deleted file) and defers to IsAllowedPath for the
// directory whitelist.
func safeRestoreTarget(target string, allowedDirs []string) bool {
	if utils.SafePath(target) == "" {
		return false
	}
	parts := strings.Split(strings.Trim(target, "/"), "/")
	cur := ""
	for _, p := range parts {
		if cur == "" {
			cur = "/" + p
		} else {
			cur += "/" + p
		}
		info, err := os.Lstat(cur)
		if err != nil {
			break
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return utils.IsAllowedPath(target, allowedDirs)
}

func openBackupArchive(safePath string) (*tar.Reader, *os.File, error) {
	f, err := os.Open(safePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开备份文件失败: %w", err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, nil, fmt.Errorf("备份文件不是有效的 gzip 归档")
	}
	return tar.NewReader(gz), f, nil
}

// PreviewBackup lists the file contents of a backup archive and where each
// member would be restored to, without touching the filesystem.
func PreviewBackup(backupPath string, maxEntries int) (*types.BackupPreview, error) {
	safePath, err := ValidateBackupFilePath(backupPath)
	if err != nil {
		return nil, err
	}

	cfg := config.Get()
	tr, f, err := openBackupArchive(safePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if maxEntries <= 0 {
		maxEntries = 200
	}
	maxEntries = utils.Clamp(maxEntries, 1, 1000)

	preview := &types.BackupPreview{
		BackupPath: safePath,
		Entries:    []types.BackupPreviewEntry{},
	}

	for seen := 0; ; seen++ {
		if seen >= restoreMaxMembers {
			preview.Truncated = true
			break
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取归档失败: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		target, ok := mapTarMember(hdr.Name)
		if !ok || strings.HasPrefix(target, cfg.Backup.BaseDir+"/") {
			preview.DeniedFiles++
			continue
		}
		allowed := safeRestoreTarget(target, cfg.LogDirs)
		entry := types.BackupPreviewEntry{
			Name:          hdr.Name,
			TargetPath:    target,
			Size:          hdr.Size,
			SizeFormatted: utils.FormatBytes(hdr.Size),
			Exists:        fileExists(target),
			Denied:        !allowed,
		}
		if !allowed {
			preview.DeniedFiles++
		}
		preview.TotalFiles++
		preview.TotalSize += hdr.Size
		if len(preview.Entries) < maxEntries {
			preview.Entries = append(preview.Entries, entry)
		} else {
			preview.Truncated = true
		}
	}

	preview.TotalSizeFormatted = utils.FormatBytes(preview.TotalSize)
	return preview, nil
}

// RestoreBackup extracts a backup archive back to the original paths. Targets
// outside the configured log dirs, symlinks and non-regular members are
// refused; existing files are skipped unless opts.Overwrite is set.
func RestoreBackup(backupPath string, opts types.RestoreOptions) (*types.RestoreResult, error) {
	if !restoreMu.TryLock() {
		return nil, fmt.Errorf("恢复正在进行中，请稍后再试")
	}
	defer restoreMu.Unlock()

	safePath, err := ValidateBackupFilePath(backupPath)
	if err != nil {
		return nil, err
	}

	cfg := config.Get()
	maxTotal := cfg.Backup.MaxTotalBytes
	if maxTotal <= 0 {
		maxTotal = 2 * 1024 * 1024 * 1024
	}

	tr, f, err := openBackupArchive(safePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := &types.RestoreResult{
		Errors:  []string{},
		Details: []types.RestoreItemResult{},
	}
	addDetail := func(path, status, message string) {
		result.Details = append(result.Details, types.RestoreItemResult{
			Path: path, Status: status, Message: message,
		})
	}

	var totalBytes int64
	for seen := 0; ; seen++ {
		if seen >= restoreMaxMembers {
			result.Errors = append(result.Errors, "归档成员数超过上限，已中止")
			break
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("读取归档失败: %v", err))
			break
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if totalBytes+hdr.Size > maxTotal {
			result.Errors = append(result.Errors, "恢复内容超过大小限制，已中止")
			break
		}

		target, ok := mapTarMember(hdr.Name)
		if !ok {
			result.Skipped++
			addDetail(hdr.Name, "skipped", "不支持的归档成员")
			continue
		}
		if strings.HasPrefix(target, cfg.Backup.BaseDir+"/") {
			result.Skipped++
			addDetail(target, "skipped", "目标在备份目录内")
			continue
		}
		if !safeRestoreTarget(target, cfg.LogDirs) {
			result.Skipped++
			addDetail(target, "skipped", "目标不在允许的日志目录内")
			continue
		}
		if !opts.Overwrite && fileExists(target) {
			result.Skipped++
			addDetail(target, "skipped", "目标已存在")
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			result.Failed++
			addDetail(target, "failed", "创建目录失败: "+err.Error())
			result.Errors = append(result.Errors, target+": "+err.Error())
			continue
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			result.Failed++
			addDetail(target, "failed", "写入失败: "+err.Error())
			result.Errors = append(result.Errors, target+": "+err.Error())
			continue
		}
		_, copyErr := io.CopyN(out, tr, hdr.Size)
		closeErr := out.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(target)
			result.Failed++
			msg := "写入失败"
			if copyErr != nil {
				msg = "写入失败: " + copyErr.Error()
			} else {
				msg = "关闭文件失败: " + closeErr.Error()
			}
			addDetail(target, "failed", msg)
			result.Errors = append(result.Errors, target+": "+msg)
			continue
		}

		result.Restored++
		addDetail(target, "restored", "")
		totalBytes += hdr.Size
	}

	return result, nil
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
