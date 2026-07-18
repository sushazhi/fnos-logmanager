package services

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// ArchiveResult represents the result of reading an archive.
type ArchiveResult struct {
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// ReadArchiveContent reads content from a compressed archive file.
func ReadArchiveContent(archivePath string, maxLines int) (ArchiveResult, error) {
	cfg := config.Get()
	safePath := utils.SafePath(archivePath)
	if safePath == "" || !utils.IsAllowedPath(archivePath, cfg.LogDirs) {
		return ArchiveResult{}, fmt.Errorf("不允许访问此文件")
	}

	// Determine the command based on file extension
	var cmd string
	var args []string

	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tar.gz"):
		cmd = "zcat"
		args = []string{archivePath}
	case strings.HasSuffix(lower, ".bz2") || strings.HasSuffix(lower, ".tar.bz2"):
		cmd = "bzcat"
		args = []string{archivePath}
	case strings.HasSuffix(lower, ".xz") || strings.HasSuffix(lower, ".tar.xz"):
		cmd = "xzcat"
		args = []string{archivePath}
	case strings.HasSuffix(lower, ".zip"):
		cmd = "unzip"
		args = []string{"-p", archivePath}
	case strings.HasSuffix(lower, ".7z"):
		cmd = "7z"
		args = []string{"x", "-so", archivePath}
	case strings.HasSuffix(lower, ".rar"):
		cmd = "unrar"
		args = []string{"p", archivePath}
	case strings.HasSuffix(lower, ".tar"):
		cmd = "tar"
		args = []string{"xf", archivePath, "-O"}
	default:
		return ArchiveResult{}, fmt.Errorf("不支持的归档格式: %s", archivePath)
	}

	var stdout, stderr bytes.Buffer
	proc := exec.Command(cmd, args...)
	proc.Stdout = &stdout
	proc.Stderr = &stderr

	if err := proc.Run(); err != nil {
		slog.Error("archive command failed", "cmd", cmd, "error", err, "stderr", stderr.String())
		return ArchiveResult{}, fmt.Errorf("解压失败: %s", stderr.String())
	}

	// Limit output
	output := stdout.String()
	if int64(len(output)) > cfg.Archive.MaxOutputBytes {
		output = output[:cfg.Archive.MaxOutputBytes]
		return ArchiveResult{
			Content: utils.FilterSensitiveInfo(output),
			Truncated: true,
		}, nil
	}

	// Limit lines
	lines := strings.Split(output, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		return ArchiveResult{
			Content:   utils.FilterSensitiveInfo(strings.Join(lines, "\n")),
			Truncated: true,
		}, nil
	}

	return ArchiveResult{
		Content:   utils.FilterSensitiveInfo(output),
		Truncated: false,
	}, nil
}
