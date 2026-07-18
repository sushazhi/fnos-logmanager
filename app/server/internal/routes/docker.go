package routes

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// RegisterDockerRoutes registers docker-related routes under the given router group.
func RegisterDockerRoutes(rg *gin.RouterGroup) {
	rg.GET("/docker/containers", middleware.ValidateToken, dockerContainersHandler)
	rg.GET("/docker/logs", middleware.ValidateToken, dockerLogsHandler)
	rg.GET("/docker/tail", middleware.ValidateToken, dockerTailHandler)
	rg.GET("/docker/stream", middleware.ValidateToken, dockerStreamHandler)
	rg.GET("/docker/export", middleware.ValidateToken, dockerExportHandler)
}

func execDocker(args []string, timeoutMs int64, maxBytes int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Check if docker binary exists in PATH
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return "", fmt.Errorf("Docker 命令未找到: %w", err)
	}

	cmd := exec.CommandContext(ctx, dockerPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("Docker 命令执行超时 (路径: %s)", dockerPath)
		}
		stderrStr := strings.TrimSpace(stderr.String())
		return "", fmt.Errorf("Docker 命令执行失败 (路径: %s): %s", dockerPath, stderrStr)
	}

	if int64(stdout.Len()+stderr.Len()) > maxBytes {
		return "", fmt.Errorf("Docker 输出过大")
	}

	return stdout.String() + stderr.String(), nil
}

type dockerContainer struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Image  string `json:"image"`
}

func dockerContainersHandler(c *gin.Context) {
	dockerCfg := config.Get().Docker

	stdout, err := execDocker([]string{"ps", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}"}, dockerCfg.ListTimeoutMs, dockerCfg.MaxOutputBytes)
	if err != nil {
		slog.Warn("docker ps failed", "error", err)
		c.JSON(http.StatusOK, gin.H{"containers": []dockerContainer{}, "error": "Docker未安装或未运行", "detail": err.Error()})
		return
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	containers := make([]dockerContainer, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		name := ""
		if len(parts) > 0 {
			name = parts[0]
		}
		if !utils.IsValidContainerName(name) {
			continue
		}
		container := dockerContainer{Name: name}
		if len(parts) > 1 {
			container.Status = parts[1]
		}
		if len(parts) > 2 {
			container.Image = parts[2]
		}
		containers = append(containers, container)
	}

	c.JSON(http.StatusOK, gin.H{"containers": containers})
}

func dockerLogsHandler(c *gin.Context) {
	container := c.Query("container")
	if container == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少容器名称"})
		return
	}
	if !utils.IsValidContainerName(container) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的容器名称"})
		return
	}

	dockerCfg := config.Get().Docker
	var args []string

	if linesStr := c.Query("lines"); linesStr != "" {
		lines := 0
		if _, err := fmt.Sscanf(linesStr, "%d", &lines); err == nil && lines > 0 {
			if lines > dockerCfg.MaxLogLines {
				lines = dockerCfg.MaxLogLines
			}
			args = []string{"logs", container, "--tail", fmt.Sprintf("%d", lines)}
		}
	}
	if args == nil {
		args = []string{"logs", container}
	}

	stdout, err := execDocker(args, dockerCfg.LogsTimeoutMs, dockerCfg.MaxOutputBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取Docker日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": utils.FilterSensitiveInfo(stdout)})
}

func dockerTailHandler(c *gin.Context) {
	container := c.Query("container")
	if !utils.IsValidContainerName(container) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的容器名称"})
		return
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &offset)
		if offset < 0 {
			offset = 0
		}
	}

	dockerCfg := config.Get().Docker
	stdout, err := execDocker([]string{"logs", container, "--since", "1m"}, dockerCfg.LogsTimeoutMs, dockerCfg.MaxOutputBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取Docker日志增量失败"})
		return
	}

	lines := strings.Split(stdout, "\n")
	totalLines := len(lines)
	content := ""
	if offset < totalLines {
		content = strings.Join(lines[offset:], "\n")
	}

	if utils.IsFilterEnabled() {
		content = utils.FilterSensitiveInfo(content)
	}

	c.JSON(http.StatusOK, gin.H{
		"content": content,
		"offset":  totalLines,
	})
}

func dockerStreamHandler(c *gin.Context) {
	services.ServeDockerStreamWS(c.Writer, c.Request)
}

func dockerExportHandler(c *gin.Context) {
	container := c.Query("container")
	if container == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少容器名称"})
		return
	}
	if !utils.IsValidContainerName(container) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的容器名称"})
		return
	}

	format := c.DefaultQuery("format", "txt")
	if format != "txt" && format != "json" && format != "csv" {
		format = "txt"
	}

	dockerCfg := config.Get().Docker
	stdout, err := execDocker([]string{"logs", container}, dockerCfg.LogsTimeoutMs, dockerCfg.MaxOutputBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出Docker日志失败"})
		return
	}
	content := utils.FilterSensitiveInfo(stdout)

	safeName := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	).Replace(container)
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	exportName := fmt.Sprintf("docker_%s_%s", safeName, timestamp)

	switch format {
	case "txt":
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.txt"`, exportName))
		c.String(http.StatusOK, content)

	case "json":
		lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
		data := make([]map[string]interface{}, 0, len(lines))
		for i, line := range lines {
			data = append(data, map[string]interface{}{
				"line":    i + 1,
				"content": line,
			})
		}
		result := map[string]interface{}{
			"source":     fmt.Sprintf("docker:%s", container),
			"exportedAt": time.Now().Format(time.RFC3339),
			"totalLines": len(lines),
			"lines":      data,
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, exportName))
		c.JSON(http.StatusOK, result)

	case "csv":
		lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
		var buf bytes.Buffer
		buf.WriteString("\"line\",\"content\"\n")
		for i, line := range lines {
			escaped := strings.ReplaceAll(line, "\"", "\"\"")
			buf.WriteString(fmt.Sprintf("\"%d\",\"%s\"\n", i+1, escaped))
		}
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, exportName))
		c.String(http.StatusOK, buf.String())
	}
}
