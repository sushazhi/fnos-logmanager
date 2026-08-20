package services

import (
	"bytes"
	"fmt"
	"io"
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

	// Stream stdout and stop reading at the output cap so a huge archive is
	// never fully decompressed into memory. The subprocess is killed once the
	// limit is reached; stderr is still captured for error reporting.
	proc := exec.Command(cmd, args...)
	var stderr bytes.Buffer
	proc.Stderr = &stderr

	stdout, err := proc.StdoutPipe()
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("创建输出管道失败: %w", err)
	}
	if err := proc.Start(); err != nil {
		slog.Error("archive command failed to start", "cmd", cmd, "error", err)
		return ArchiveResult{}, fmt.Errorf("解压失败: %w", err)
	}

	// Read up to MaxOutputBytes+1; the extra byte detects truncation without
	// needing to read the whole stream.
	maxBytes := cfg.Archive.MaxOutputBytes
	data, readErr := io.ReadAll(io.LimitReader(stdout, maxBytes+1))
	truncated := int64(len(data)) > maxBytes
	if readErr != nil && proc.Process != nil {
		_ = proc.Process.Kill()
	}
	waitErr := proc.Wait()
	// A killed process after an intentional truncation is expected (the pipe
	// closed early); only surface a non-truncated failure as an error.
	if waitErr != nil && !truncated {
		slog.Error("archive command failed", "cmd", cmd, "error", waitErr, "stderr", stderr.String())
		return ArchiveResult{}, fmt.Errorf("解压失败: %s", stderr.String())
	}
	if waitErr != nil {
		slog.Debug("archive output truncated, subprocess stopped", "cmd", cmd)
	}

	output := string(data)
	if truncated {
		output = output[:maxBytes]
	}

	// Limit lines
	lines := strings.Split(output, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	return ArchiveResult{
		Content:   utils.FilterSensitiveInfo(output),
		Truncated: truncated,
	}, nil
}
