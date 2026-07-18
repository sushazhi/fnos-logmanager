package services

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// Log extensions to consider as log files
var logExtensions = []string{".log"}
var archiveExtensions = []string{".gz", ".bz2", ".xz", ".zip", ".tar", ".tar.gz", ".tar.bz2", ".tar.xz", ".7z", ".rar"}

// Installed apps cache
var (
	cachedInstalledApps   []string
	cachedInstalledAppsMu sync.RWMutex
	installedAppsCacheAt  time.Time
	installedAppsCacheTTL = 60 * time.Second
)

func isArchiveFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range archiveExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// extractAppNameFromPath extracts the application name from a log file path.
func extractAppNameFromPath(logPath string) string {
	parts := strings.Split(logPath, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "@") && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	if strings.HasPrefix(logPath, "/var/log/apps/") {
		rest := strings.TrimPrefix(logPath, "/var/log/apps/")
		appName := strings.Split(rest, "/")[0]
		re := regexp.MustCompile(`\.log(-\d{8})?(\.\d+)?\.(gz|bz2|xz|zip|tar(\.gz|\.bz2|\.xz)?|7z|rar)$`)
		appName = re.ReplaceAllString(appName, "")
		appName = strings.TrimSuffix(appName, ".log")
		if appName != "" {
			return appName
		}
	}

	re := regexp.MustCompile(`/vol\d+/@appdata/([^/]+)`)
	if m := re.FindStringSubmatch(logPath); len(m) > 1 {
		return m[1]
	}

	re = regexp.MustCompile(`/vol\d+/@appshare/([^/]+)`)
	if m := re.FindStringSubmatch(logPath); len(m) > 1 {
		return m[1]
	}

	return ""
}

// findFiles recursively walks a directory and returns files matching the filter function.
func findFiles(dir string, filterFn func(string) bool, limit int) ([]string, error) {
	var results []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip permission errors
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		if len(results) >= limit {
			return filepath.SkipDir
		}
		if !info.IsDir() && filterFn(info.Name()) {
			results = append(results, path)
		}
		return nil
	})

	if err != nil {
		return results[:min(len(results), limit)], nil
	}
	return results[:min(len(results), limit)], nil
}

// getInstalledApps returns the list of installed apps, with caching.
func getInstalledApps() ([]string, error) {
	cachedInstalledAppsMu.RLock()
	if cachedInstalledApps != nil && time.Since(installedAppsCacheAt) < installedAppsCacheTTL {
		defer cachedInstalledAppsMu.RUnlock()
		return cachedInstalledApps, nil
	}
	cachedInstalledAppsMu.RUnlock()

	cachedInstalledAppsMu.Lock()
	defer cachedInstalledAppsMu.Unlock()

	// Double-check after acquiring write lock
	if cachedInstalledApps != nil && time.Since(installedAppsCacheAt) < installedAppsCacheTTL {
		return cachedInstalledApps, nil
	}

	apps, err := execAppcenterList()
	if err != nil {
		slog.Warn("failed to get installed apps from appcenter-cli", "error", err)
		cachedInstalledApps = []string{}
		installedAppsCacheAt = time.Now()
		return cachedInstalledApps, nil
	}

	cachedInstalledApps = apps
	installedAppsCacheAt = time.Now()
	return apps, nil
}

func execAppcenterList() ([]string, error) {
	output, err := execCommand("appcenter-cli", []string{"list"}, 10000)
	if err != nil {
		return nil, err
	}

	var apps []string
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "│") && !strings.Contains(line, "APP NAME") && !strings.Contains(line, "────") {
			parts := strings.Split(line, "│")
			if len(parts) >= 2 {
				appName := strings.TrimSpace(parts[1])
				if appName != "" {
					apps = append(apps, appName)
				}
			}
		}
	}
	return apps, nil
}

func execCommand(cmd string, args []string, timeoutMs int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	var stdout, stderr bytes.Buffer
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdout = &stdout
	c.Stderr = &stderr

	if err := c.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("命令执行超时: %s %v", cmd, args)
		}
		return "", fmt.Errorf("命令执行失败: %s: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

// GetAppNames returns all unique application names from log files.
func GetAppNames() ([]string, error) {
	appNameSet := make(map[string]bool)

	for _, dir := range config.Get().LogDirs {
		normalizedDir := utils.SafePath(dir)
		if normalizedDir == "" {
			continue
		}
		if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
			continue
		}

		files, err := findFiles(normalizedDir, isLogFile, 10000)
		if err != nil {
			continue
		}

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil || info.Size() == 0 {
				continue
			}

			if appName := extractAppNameFromPath(file); appName != "" {
				appNameSet[appName] = true
			}
		}
	}

	var result []string
	for name := range appNameSet {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

// ListLogFiles lists log files in the specified directories.
func ListLogFiles(dir string, limit int) ([]types.LogFile, error) {
	searchDirs := config.Get().LogDirs
	if dir != "" {
		searchDirs = []string{dir}
	}

	var results []types.LogFile
	installedApps, _ := getInstalledApps()
	installedSet := make(map[string]bool)
	for _, app := range installedApps {
		installedSet[app] = true
	}

	for _, searchDir := range searchDirs {
		normalizedDir := utils.SafePath(searchDir)
		if normalizedDir == "" {
			continue
		}
		if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
			continue
		}
		if dir != "" && !utils.IsAllowedPath(dir, config.Get().LogDirs) {
			continue
		}

		files, err := findFiles(normalizedDir, isLogFile, limit)
		if err != nil {
			continue
		}

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}

			appName := extractAppNameFromPath(file)
			canDelete := false
			if appName != "" {
				canDelete = !installedSet[appName]
			}

			results = append(results, types.LogFile{
				Path:          file,
				Size:          info.Size(),
				SizeFormatted: utils.FormatBytes(info.Size()),
				Modified:      info.ModTime(),
				AppName:       &appName,
				CanDelete:     canDelete,
			})
		}

		// When browsing a specific directory, also include archive files
		// so backup .tar.gz files and compressed logs appear in the listing.
		if dir != "" {
			archiveFiles, err := findFiles(normalizedDir, isArchiveFile, limit)
			if err == nil {
				for _, file := range archiveFiles {
					info, err := os.Stat(file)
					if err != nil {
						continue
					}

					appName := extractAppNameFromPath(file)

					results = append(results, types.LogFile{
						Path:          file,
						Size:          info.Size(),
						SizeFormatted: utils.FormatBytes(info.Size()),
						Modified:      info.ModTime(),
						AppName:       &appName,
						IsArchive:     true,
					})
				}
			}
		}
	}

	// Sort by modified time descending (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Modified.After(results[j].Modified)
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// ListArchiveFiles lists archive files in log directories.
func ListArchiveFiles(limit int) ([]types.ArchiveFile, error) {
	var results []types.ArchiveFile

	for _, dir := range config.Get().LogDirs {
		normalizedDir := utils.SafePath(dir)
		if normalizedDir == "" {
			continue
		}
		if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
			continue
		}

		files, err := findFiles(normalizedDir, isArchiveFile, limit)
		if err != nil {
			continue
		}

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}

			results = append(results, types.ArchiveFile{
				Path:          file,
				Size:          info.Size(),
				SizeFormatted: utils.FormatBytes(info.Size()),
				Modified:      info.ModTime(),
				Type:          filepath.Ext(file),
			})
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// ListLargeLogFiles lists log files above a size threshold.
func ListLargeLogFiles(thresholdBytes int64, limit int) ([]types.LogFile, error) {
	var results []types.LogFile
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, dir := range config.Get().LogDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()

			normalizedDir := utils.SafePath(d)
			if normalizedDir == "" {
				return
			}
			if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
				return
			}

			files, err := findFiles(normalizedDir, isLogFile, limit*2)
			if err != nil {
				return
			}

			for _, file := range files {
				info, err := os.Stat(file)
				if err != nil {
					continue
				}
				if info.Size() >= thresholdBytes {
					mu.Lock()
					results = append(results, types.LogFile{
						Path:          file,
						Size:          info.Size(),
						SizeFormatted: utils.FormatBytes(info.Size()),
						Modified:      info.ModTime(),
					})
					mu.Unlock()
				}
			}
		}(dir)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Size > results[j].Size
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// SearchLogFilesByName searches for log files by name pattern.
func SearchLogFilesByName(pattern string, limit int) ([]types.LogFile, error) {
	safePattern := utils.EscapeRegExp(pattern)
	re, err := regexp.Compile("(?i)" + safePattern)
	if err != nil {
		return nil, err
	}

	var results []types.LogFile
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, dir := range config.Get().LogDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()

			normalizedDir := utils.SafePath(d)
			if normalizedDir == "" {
				return
			}
			if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
				return
			}

			files, err := findFiles(normalizedDir, func(name string) bool {
				return isLogFile(name) && re.MatchString(name)
			}, limit)
			if err != nil {
				return
			}

			for _, file := range files {
				info, err := os.Stat(file)
				if err != nil {
					continue
				}
				appName := extractAppNameFromPath(file)

				mu.Lock()
				results = append(results, types.LogFile{
					Path:          file,
					Size:          info.Size(),
					SizeFormatted: utils.FormatBytes(info.Size()),
					Modified:      info.ModTime(),
					AppName:       &appName,
				})
				mu.Unlock()
			}
		}(dir)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Modified.After(results[j].Modified)
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// GetLogStats returns aggregate statistics about all log files.
func GetLogStats() (types.LogStats, error) {
	var stats types.LogStats

	for _, dir := range config.Get().LogDirs {
		normalizedDir := utils.SafePath(dir)
		if normalizedDir == "" {
			continue
		}
		if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
			continue
		}

		logFiles, _ := findFiles(normalizedDir, isLogFile, 10000)
		stats.TotalLogs += len(logFiles)

		for _, file := range logFiles {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}
			stats.TotalSize += info.Size()
			if info.Size() >= 10*1024*1024 {
				stats.LargeFiles++
			}
		}

		archiveFiles, _ := findFiles(normalizedDir, isArchiveFile, 10000)
		stats.TotalArchives += len(archiveFiles)
	}

	stats.TotalSizeFormatted = utils.FormatBytes(stats.TotalSize)
	return stats, nil
}

// ReadLogFile reads content from a log file.
func ReadLogFile(filePath string, options types.ReadLogOptions) (types.ReadLogResult, error) {
	normalizedPath := utils.SafePath(filePath)
	if normalizedPath == "" || !utils.IsAllowedPath(filePath, config.Get().LogDirs) {
		return types.ReadLogResult{}, fmt.Errorf("不允许访问此文件")
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		return types.ReadLogResult{}, err
	}
	if info.IsDir() {
		return types.ReadLogResult{}, fmt.Errorf("不是有效的文件")
	}

	maxLines := options.MaxLines
	if maxLines <= 0 {
		maxLines = config.Get().LogFile.MaxPreviewLines
	}
	maxPreviewBytes := config.Get().LogFile.MaxPreviewBytes

	// Small file: read entirely into memory
	if info.Size() <= maxPreviewBytes {
		data, err := os.ReadFile(normalizedPath)
		if err != nil {
			return types.ReadLogResult{}, err
		}

		content := string(data)
		allLines := strings.Split(content, "\n")
		totalLines := len(allLines)

		var selectedLines []string
		var truncated bool

		if options.Tail {
			start := 0
			if totalLines > maxLines {
				start = totalLines - maxLines
				truncated = true
			}
			selectedLines = allLines[start:]
		} else {
			end := options.Offset + maxLines
			if end > totalLines {
				end = totalLines
			}
			if options.Offset < totalLines {
				selectedLines = allLines[options.Offset:end]
			}
			if end < totalLines {
				truncated = true
			}
		}

		return types.ReadLogResult{
			Content:       strings.Join(selectedLines, "\n"),
			TotalLines:    totalLines,
			Size:          info.Size(),
			SizeFormatted: utils.FormatBytes(info.Size()),
			Truncated:     truncated,
			HasMore:       truncated,
		}, nil
	}

	// Large file: stream read
	return readLargeFileStreaming(normalizedPath, options, info.Size())
}

func readLargeFileStreaming(filePath string, options types.ReadLogOptions, fileSize int64) (types.ReadLogResult, error) {
	maxLines := options.MaxLines
	if maxLines <= 0 {
		maxLines = config.Get().LogFile.MaxPreviewLines
	}

	f, err := os.Open(filePath)
	if err != nil {
		return types.ReadLogResult{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Increase buffer for long lines
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	totalLines := 0
	var lineBuffer []string
	maxBuf := maxLines

	for scanner.Scan() {
		totalLines++
		line := scanner.Text()

		if options.Tail {
			lineBuffer = append(lineBuffer, line)
			if len(lineBuffer) > maxBuf {
				lineBuffer = lineBuffer[1:]
			}
		} else {
			if totalLines > options.Offset && len(lineBuffer) < maxBuf {
				lineBuffer = append(lineBuffer, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("scanner error reading large file", "path", filePath, "error", err)
	}

	content := strings.Join(lineBuffer, "\n")
	truncated := false
	if options.Tail {
		truncated = len(lineBuffer) < totalLines
	} else {
		truncated = (options.Offset + maxLines) < totalLines
	}

	return types.ReadLogResult{
		Content:       utils.FilterSensitiveInfo(content),
		TotalLines:    totalLines,
		Size:          fileSize,
		SizeFormatted: utils.FormatBytes(fileSize),
		Truncated:     truncated,
		HasMore:       truncated,
	}, nil
}

// TruncateLogFile empties a log file.
func TruncateLogFile(filePath string) error {
	normalizedPath := utils.SafePath(filePath)
	if normalizedPath == "" || !utils.IsAllowedPath(filePath, config.Get().LogDirs) {
		return fmt.Errorf("不允许访问此文件")
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("文件不存在")
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("不能清空目录")
	}

	return os.WriteFile(normalizedPath, []byte{}, 0644)
}

// DeleteLogFile deletes a log file.
func DeleteLogFile(filePath string) error {
	normalizedPath := utils.SafePath(filePath)
	if normalizedPath == "" || !utils.IsAllowedPath(filePath, config.Get().LogDirs) {
		return fmt.Errorf("不允许访问此文件")
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("不能删除目录")
	}

	return os.Remove(normalizedPath)
}

// GetDirsInfo returns information about all log directories.
func GetDirsInfo() ([]types.DirInfo, error) {
	var results []types.DirInfo

	for _, dir := range config.Get().LogDirs {
		normalizedDir := utils.SafePath(dir)
		exists := false
		logCount := 0
		archiveCount := 0
		var totalSize int64

		if normalizedDir != "" {
			if info, err := os.Stat(normalizedDir); err == nil && info.IsDir() {
				exists = true

				logFiles, _ := findFiles(normalizedDir, isLogFile, 10000)
				logCount = len(logFiles)

				archiveFiles, _ := findFiles(normalizedDir, isArchiveFile, 10000)
				archiveCount = len(archiveFiles)

				allFiles := append(logFiles, archiveFiles...)
				for _, f := range allFiles {
					if info, err := os.Stat(f); err == nil {
						totalSize += info.Size()
					}
				}
			}
		}

		di := types.DirInfo{
			Path:         dir,
			Exists:       exists,
			LogCount:     logCount,
			ArchiveCount: archiveCount,
			TotalSize:    utils.FormatBytes(totalSize),
		}
		results = append(results, di)
	}

	return results, nil
}

// CleanUninstalledLogs deletes log files belonging to uninstalled applications.
func CleanUninstalledLogs() (types.CleanLogResult, error) {
	result := types.CleanLogResult{}
	installedApps, err := getInstalledApps()
	if err != nil {
		return result, err
	}
	installedSet := make(map[string]bool)
	for _, app := range installedApps {
		installedSet[app] = true
	}

	for _, dir := range config.Get().LogDirs {
		normalizedDir := utils.SafePath(dir)
		if normalizedDir == "" {
			continue
		}
		if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
			continue
		}

		files, err := findFiles(normalizedDir, isLogFile, 50000)
		if err != nil {
			continue
		}

		for _, file := range files {
			appName := extractAppNameFromPath(file)
			if appName == "" || installedSet[appName] {
				continue
			}
			if err := os.Remove(file); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", file, err.Error()))
			} else {
				result.Cleaned++
			}
		}
	}

	return result, nil
}

// CleanLogFiles cleans log files based on criteria.
func CleanLogFiles(options types.CleanLogOptions) (types.CleanLogResult, error) {
	result := types.CleanLogResult{}
	var cutoffTime time.Time
	if options.Days != nil {
		cutoffTime = time.Now().AddDate(0, 0, -*options.Days)
	}

	for _, dir := range config.Get().LogDirs {
		normalizedDir := utils.SafePath(dir)
		if normalizedDir == "" {
			continue
		}
		if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
			continue
		}

		files, err := findFiles(normalizedDir, func(name string) bool {
			if options.Action == "delete" && options.Days != nil {
				return isArchiveFile(name) || isLogFile(name)
			}
			if options.Days != nil {
				return isArchiveFile(name)
			}
			return isLogFile(name)
		}, 10000)
		if err != nil {
			continue
		}

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}

			// Apply size threshold only for truncate
			if options.Action == "truncate" && options.ThresholdBytes != nil &&
				info.Size() < *options.ThresholdBytes {
				continue
			}

			// Apply days filter
			if options.Days != nil && info.ModTime().After(cutoffTime) {
				continue
			}

			if options.Action == "delete" {
				if err := os.Remove(file); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", file, err.Error()))
				} else {
					result.Cleaned++
				}
			} else if options.Action == "truncate" {
				if err := os.WriteFile(file, []byte{}, 0644); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", file, err.Error()))
				} else {
					result.Cleaned++
				}
			}
		}
	}

	return result, nil
}

var appDirsToClean = []string{
	"/vol1/@appcenter",
	"/vol1/@appconf",
	"/vol1/@appdata",
	"/vol1/@apphome",
	"/vol1/@appmeta",
	"/vol1/@apptemp",
	"/vol1/@appshare",
}

func isDirEmpty(dirPath string) bool {
	f, err := os.Open(dirPath)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err != nil // directory is empty if ReadDirNames returns an error
}

func removeEmptyDirsRecursively(dirPath string, removedDirs *[]string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			removeEmptyDirsRecursively(filepath.Join(dirPath, entry.Name()), removedDirs)
		}
	}

	if isDirEmpty(dirPath) {
		if err := os.Remove(dirPath); err == nil {
			*removedDirs = append(*removedDirs, dirPath)
		}
	}
}

// CleanEmptyAppDirs removes empty directories of uninstalled applications.
func CleanEmptyAppDirs() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"cleaned": 0,
		"dirs":    []string{},
		"errors":  []string{},
	}

	installedApps, _ := getInstalledApps()
	installedSet := make(map[string]bool)
	for _, app := range installedApps {
		installedSet[app] = true
	}

	var dirs []string
	var errs []string

	for _, baseDir := range appDirsToClean {
		normalizedDir := utils.SafePath(baseDir)
		if normalizedDir == "" {
			continue
		}
		if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(normalizedDir)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", baseDir, err.Error()))
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			appName := entry.Name()
			if installedSet[appName] {
				continue
			}

			appDirPath := filepath.Join(normalizedDir, appName)
			var removedDirs []string
			removeEmptyDirsRecursively(appDirPath, &removedDirs)

			if _, err := os.Stat(appDirPath); os.IsNotExist(err) {
				dirs = append(dirs, appDirPath)
			} else if len(removedDirs) > 0 {
				dirs = append(dirs, removedDirs...)
			}
		}
	}

	result["cleaned"] = len(dirs)
	result["dirs"] = dirs
	result["errors"] = errs
	return result, nil
}
