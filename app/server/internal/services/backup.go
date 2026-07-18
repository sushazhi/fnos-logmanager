package services

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

const backupBaseDir = "" // set dynamically from config

// ensureBackupDir ensures the backup directory exists.
func ensureBackupDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// isSafeBackupBase checks if the backup base directory is safe.
func isSafeBackupBase(dir string) bool {
	normalized := utils.SafePath(dir)
	if normalized == "" {
		return false
	}
	if !strings.HasPrefix(normalized, "/") {
		return false
	}
	for _, dangerous := range []string{"/etc", "/proc", "/sys"} {
		if strings.HasPrefix(normalized, dangerous) {
			return false
		}
	}
	return true
}

// isLogFile checks if a filename looks like a log file.
func isLogFile(filename string) bool {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".log") {
		return true
	}
	if strings.Contains(lower, ".log.") {
		return true
	}
	if strings.Contains(lower, "log") && strings.HasSuffix(lower, ".txt") {
		return true
	}
	return false
}

type collectedFile struct {
	FullPath     string
	RelativePath string
}

// collectLogFiles recursively collects log files from a directory.
func collectLogFiles(dir, baseDir string, maxFiles int, maxFileSize int64) ([]collectedFile, error) {
	var files []collectedFile

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip permission errors
		}
		if len(files) >= maxFiles {
			return io.EOF // stop walk
		}
		if d.IsDir() {
			return nil
		}
		if !isLogFile(d.Name()) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > maxFileSize {
			return nil
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}
		files = append(files, collectedFile{FullPath: path, RelativePath: relPath})
		return nil
	})
	if err == io.EOF {
		err = nil
	}
	return files, err
}

// copyFileToBackup copies a file to the backup directory.
func copyFileToBackup(src, dest string) error {
	destDir := filepath.Dir(dest)
	if err := ensureBackupDir(destDir); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

// createTarGz creates a tar.gz archive from a source directory.
func createTarGz(sourceDir, outputFile string) error {
	safeSourceDir := utils.SafePath(sourceDir)
	safeOutputFile := utils.SafePath(outputFile)
	if safeSourceDir == "" || safeOutputFile == "" {
		return fmt.Errorf("无效的路径")
	}

	baseName := filepath.Base(safeSourceDir)
	parentDir := filepath.Dir(safeSourceDir)

	cmd := exec.Command("tar", "-czf", safeOutputFile, "-C", parentDir, baseName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar 命令失败: %s: %w", string(output), err)
	}
	return nil
}

// removeDir safely removes a directory tree.
func removeDir(dir string) error {
	safeDir := utils.SafePath(dir)
	if safeDir == "" {
		return nil
	}

	dangerousPaths := []string{"/", "/var", "/etc", "/usr", "/bin", "/sbin", "/lib", "/boot"}
	for _, d := range dangerousPaths {
		if safeDir == d {
			return fmt.Errorf("不允许删除系统目录")
		}
	}

	return os.RemoveAll(safeDir)
}

// PerformBackup creates a full backup of all log directories.
func PerformBackup() types.BackupResult {
	cfg := config.Get()
	backupDir := cfg.Backup.BaseDir

	if !isSafeBackupBase(backupDir) {
		return types.BackupResult{
			BackupPath: "",
			Files:      0,
			BackupSize: "0B",
			Errors:     []string{"备份目录不安全"},
		}
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	tmpBackupDir := filepath.Join(backupDir, "backup-"+timestamp)
	backupFile := tmpBackupDir + ".tar.gz"

	result := types.BackupResult{
		BackupPath: backupFile,
		Files:      0,
		BackupSize: "0B",
		Errors:     []string{},
	}

	if err := ensureBackupDir(backupDir); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("创建备份目录失败: %v", err))
		return result
	}
	if err := ensureBackupDir(tmpBackupDir); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("创建临时目录失败: %v", err))
		return result
	}

	var totalBytes int64

	for _, logDir := range cfg.LogDirs {
		normalizedDir := utils.SafePath(logDir)
		if normalizedDir == "" {
			continue
		}

		stat, err := os.Stat(normalizedDir)
		if err != nil || !stat.IsDir() {
			continue
		}

		dirName := filepath.Base(normalizedDir)
		targetDir := filepath.Join(tmpBackupDir, dirName)

		files, err := collectLogFiles(normalizedDir, normalizedDir, cfg.Backup.MaxFiles, cfg.Backup.MaxFileSizeBytes)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("收集文件失败 %s: %v", logDir, err))
			continue
		}

		for _, f := range files {
			if totalBytes >= cfg.Backup.MaxTotalBytes {
				result.Errors = append(result.Errors, "备份内容超过大小限制")
				break
			}

			info, err := os.Stat(f.FullPath)
			if err != nil {
				continue
			}
			if totalBytes+info.Size() > cfg.Backup.MaxTotalBytes {
				result.Errors = append(result.Errors, "备份内容超过大小限制")
				break
			}

			targetPath := filepath.Join(targetDir, f.RelativePath)
			if err := copyFileToBackup(f.FullPath, targetPath); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("复制失败: %s - %v", f.FullPath, err))
				continue
			}
			result.Files++
			totalBytes += info.Size()
		}

		if totalBytes >= cfg.Backup.MaxTotalBytes {
			break
		}
	}

	if result.Files > 0 {
		if err := createTarGz(tmpBackupDir, backupFile); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("创建归档失败: %v", err))
			return result
		}

		if err := removeDir(tmpBackupDir); err != nil {
			slog.Warn("清理临时目录失败", "error", err)
		}

		stat, err := os.Stat(backupFile)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("检查备份文件失败: %v", err))
			return result
		}
		if stat.Size() > cfg.Backup.MaxTotalBytes {
			os.Remove(backupFile)
			result.Errors = append(result.Errors, "备份文件超过大小限制")
			return result
		}
		result.BackupSize = utils.FormatBytes(stat.Size())
	} else {
		removeDir(tmpBackupDir)
		result.Errors = append(result.Errors, "没有找到日志文件")
	}

	return result
}

// ListBackups returns a list of existing backup files.
func ListBackups() []types.BackupInfo {
	cfg := config.Get()
	backupDir := cfg.Backup.BaseDir

	var backups []types.BackupInfo

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return backups
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}

		fullPath := filepath.Join(backupDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, types.BackupInfo{
			Name:    entry.Name(),
			Path:    fullPath,
			Size:    utils.FormatBytes(info.Size()),
			Created: info.ModTime(),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Created.After(backups[j].Created)
	})

	return backups
}

// DeleteBackup deletes a backup file.
func DeleteBackup(backupPath string) error {
	safePath := utils.SafePath(backupPath)
	if safePath == "" {
		return fmt.Errorf("无效的路径")
	}

	cfg := config.Get()
	if !strings.HasPrefix(safePath, cfg.Backup.BaseDir) {
		return fmt.Errorf("只能删除备份目录下的文件")
	}

	if !strings.HasSuffix(safePath, ".tar.gz") {
		return fmt.Errorf("只能删除备份文件")
	}

	return os.Remove(safePath)
}

// CleanOldBackups removes backups older than keepDays.
func CleanOldBackups(keepDays int) (int, error) {
	backups := ListBackups()
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	deleted := 0

	for _, b := range backups {
		if b.Created.Before(cutoff) {
			if err := DeleteBackup(b.Path); err != nil {
				slog.Warn("删除旧备份失败", "path", b.Path, "error", err)
				continue
			}
			deleted++
		}
	}
	return deleted, nil
}
