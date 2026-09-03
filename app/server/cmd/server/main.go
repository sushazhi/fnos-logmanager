package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/logger"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/routes"
	"github.com/sushazhi/fnos-logmanager/internal/services"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

func main() {
	os.Setenv("TZ", "Asia/Shanghai")

	// Initialize logger
	logger.Init(os.Getenv("LOG_LEVEL"))

	// Validate environment
	valid, errs := middleware.ValidateEnv()
	if !valid {
		for _, e := range errs {
			slog.Error("环境验证失败", "error", e)
		}
		os.Exit(1)
	}

	// Check dependencies
	depsValid, missing := middleware.CheckDependencies()
	if !depsValid {
		slog.Warn("缺少依赖", "missing", missing)
	}

	cfg := config.Get()
	slog.Info("配置已加载",
		"port", cfg.Port,
		"dataDir", cfg.DataDir,
		"gatewaySocket", cfg.GatewaySocket,
	)

	// P0: Initialize fnOS trim API client and register ACL checker
	trimClient := services.GetTrimClient()
	utils.SetACLChecker(trimClient)
	slog.Info("ACL 权限检查已启用",
		"socket", trimClient.GetBaseURL(),
		"hasToken", os.Getenv("TRIM_API_TOKEN") != "")

	// 说明：fnOS 系统语言/版本/主题等信息已由前端 JS SDK 的 getPlatformConfig()
	// 获取（无需开放平台 scope，见 app/ui/src/App.vue）。后端不再调用
	// trim.system.getPlatformConfig（该 scope 在部分安装下返回 403，且非核心功能所需）。

	// Initialize service directories
	initServiceDirs(cfg.DataDir)

	// Initialize notification store and link to services
	routes.InitNotificationStore(cfg.DataDir)
	services.SetNotifyStore(routes.GetNotifStore())

	// Sync persisted notification channel configs to runtime registry
	// Ensures channels configured via the UI remain enabled across restarts/upgrades
	services.SyncNotifyStoreToRegistry()

	// Initialize auto-clean service
	if err := services.InitAutoClean(cfg.DataDir); err != nil {
		slog.Error("自动清理服务初始化失败", "error", err)
	}

	// Initialize recycle-bin cleaner (auto-purges moved app leftovers past the configurable retention)
	if err := services.InitRecycleCleaner(cfg.DataDir); err != nil {
		slog.Error("回收站清理服务初始化失败", "error", err)
	}

	// Initialize event logger (always try — handles upgrades where DB exists but config wasn't persisted)
	if err := services.InitEventLogger(cfg.EventLogger.DBPath); err != nil {
		slog.Warn("事件日志服务初始化失败（非致命）", "error", err)
	} else if cfg.EventLogger.Enabled {
		// Auto-start only if explicitly enabled in config
		if err := services.StartEventLogger(); err != nil {
			slog.Warn("事件日志服务启动失败", "error", err)
		} else {
			slog.Info("事件日志服务已自动启动")
		}
	}

	// Initialize log monitor
	if err := services.InitMonitor(filepath.Join(cfg.DataDir, "config")); err != nil {
		slog.Warn("日志监控初始化失败", "error", err)
	}

	// Auto-start log monitor if enabled in notification settings (persists across upgrades)
	if settings := routes.GetNotifStore().GetSettings(); settings.Enabled {
		if err := services.StartMonitor(); err != nil {
			slog.Warn("日志监控自动启动失败", "error", err)
		} else {
			slog.Info("日志监控已根据配置自动启动")
		}
	}

	// Setup Gin router
	uiDir := findUIDir()
	router := routes.SetupRouter(uiDir)

	// Wrap handler with gateway prefix stripping (same as Node.js http.createServer)
	gatewayPrefix := config.Get().GatewayPrefix
	var handler http.Handler = router
	if gatewayPrefix != "" {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.URL.Path) >= len(gatewayPrefix) && r.URL.Path[:len(gatewayPrefix)] == gatewayPrefix {
				trimmed := r.URL.Path[len(gatewayPrefix):]
				if trimmed == "" {
					trimmed = "/"
				}
				r.URL.Path = trimmed
				r.RequestURI = trimmed
			}
			router.ServeHTTP(w, r)
		})
	}

	// Start auto-clean scheduler
	if err := services.StartAutoClean(); err != nil {
		slog.Warn("自动清理调度启动失败", "error", err)
	}

	// Start recycle-bin auto-purge scheduler (retention is user-configurable)
	if err := services.StartRecycleCleaner(); err != nil {
		slog.Warn("回收站自动清空调度启动失败", "error", err)
	}

	// Create HTTP server.
	// Gateway mode always binds 127.0.0.1 (only reachable via the gateway proxy).
	// Standalone mode defaults to 127.0.0.1 as well: it has no authentication,
	// so exposing it on all interfaces would leave every API (log deletion,
	// kernel removal, docker, updates) open to the whole network. Set
	// LOGMANAGER_BIND_ADDR=0.0.0.0 explicitly if external access is required.
	bindAddr := "127.0.0.1"
	if cfg.GatewaySocket == "" {
		if v := os.Getenv("LOGMANAGER_BIND_ADDR"); v != "" {
			bindAddr = v
		}
	}
	if bindAddr != "127.0.0.1" && bindAddr != "::1" && cfg.GatewaySocket == "" {
		slog.Warn("独立模式监听非回环地址，所有接口将无认证暴露，请确保网络可信", "bindAddr", bindAddr)
	}
	addr := fmt.Sprintf("%s:%d", bindAddr, cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  65 * time.Second,
	}

	// Start gateway proxy if configured
	if cfg.GatewaySocket != "" {
		startGatewayProxy(cfg.GatewaySocket, cfg.Port)
	}

	// Start server in background
	go func() {
		slog.Info("服务启动", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务启动失败", "error", err)
			os.Exit(1)
		}
	}()

	// Start a dedicated MCP listener if configured. External AI agents
	// (QwenPAW, OpenClaw, Hermes) cannot authenticate against the fnOS gateway,
	// so a dedicated TCP listener (protected by MCP_API_KEY) is the reliable way
	// to expose the MCP endpoint on the LAN.
	//
	// 动态管理：周期读取 MCP 配置的 Port/BindAddr，保存端口配置后自动启停监听器，
	// 无需重启进程（前端在设置里配置独立端口后即可生效）。
	mcpStop := make(chan struct{})
	go manageMCPListener(handler, mcpStop)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("收到关闭信号", "signal", sig.String())

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop services
	_ = services.StopAutoClean()
	_ = services.StopRecycleCleaner()
	_ = services.StopMonitor()
	services.CloseLogStreamWS()
	services.CloseDockerStreamWS()
	services.GetNotifyHub().CloseAll()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("服务关闭异常", "error", err)
	}
	// 通知 MCP 独立监听器管理器优雅停止
	close(mcpStop)

	// FIX(bug 1): flush any unsaved session/CSRF changes to disk before exiting,
	// otherwise a shutdown that races the 5s save timer would drop every login.
	services.FlushSessions()

	slog.Info("服务已关闭")
}

// manageMCPListener 动态管理 MCP 独立监听器。
// 周期读取 MCP 配置（LoadMCPConfig），当 Port 变化时自动启停监听器，
// 使前端保存独立端口配置后无需重启进程即可生效。
// 在收到 stop 通道关闭信号时优雅停止所有监听器。
func manageMCPListener(handler http.Handler, stop <-chan struct{}) {
	var current *http.Server
	currentPort := -1

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	stopMCP := func() {
		if current != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = current.Shutdown(ctx)
			current = nil
		}
		currentPort = -1
	}

	for {
		select {
		case <-stop:
			stopMCP()
			return
		case <-ticker.C:
			live := config.LoadMCPConfig()
			port := live.Port
			if port == currentPort {
				continue
			}
			// 端口变化：先停止旧监听器
			stopMCP()
			currentPort = port
			// 再按需启动新监听器
			if port <= 0 {
				continue
			}
			mcpSrv := routes.GetMCPServer()
			if mcpSrv == nil {
				continue
			}
			bind := live.BindAddr
			if bind == "" {
				// Default to loopback: if MCP_API_KEY is not configured, only
				// loopback callers are authorized anyway. Listening on 0.0.0.0
				// by default would expose destructive tools (delete_log,
				// remove_kernel, cleanup_kernels) to the whole LAN. Operators
				// who intentionally expose MCP must set an explicit bind addr
				// together with an API key.
				bind = "127.0.0.1"
			}
			addr := fmt.Sprintf("%s:%d", bind, port)
			current = &http.Server{
				Addr:         addr,
				Handler:      mcpSrv.Handler(),
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 120 * time.Second,
				IdleTimeout:  65 * time.Second,
			}
			srv := current
			go func() {
				slog.Info("MCP 独立监听已启动（供外部 AI Agent 接入）", "addr", addr)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					slog.Error("MCP 独立监听启动失败", "error", err)
				}
			}()
		}
	}
}

func initServiceDirs(dataDir string) {
	dirs := []string{
		filepath.Join(dataDir, "config"),
		filepath.Join(dataDir, "sessions"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("创建目录失败", "dir", dir, "error", err)
		}
	}
}

func findUIDir() string {
	// Priority: dist/ subdirectory (Vite build output) > plain ui directory (fnOS install)
	// This ensures the production build (dist/index.html with hashed assets) is served,
	// not the dev template (ui/index.html with /src/main.ts).
	candidates := []string{
		"../../ui/dist",      // from cmd/server/ (dev: Vite dist output)
		"../../ui",           // from cmd/server/ (source)
		"../ui/dist",         // from app/server/
		"../ui",              // from app/server/
		"/app/logmanager/ui", // fnOS install path
	}

	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append([]string{
			filepath.Join(exeDir, "ui"),
			filepath.Join(exeDir, "../../ui/dist"),
			filepath.Join(exeDir, "../../ui"),
		}, candidates...)
	}

	for _, dir := range candidates {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absDir); err == nil && info.IsDir() {
			indexHTML := filepath.Join(absDir, "index.html")
			if _, err := os.Stat(indexHTML); err == nil {
				return absDir
			}
		}
	}

	// Fallback
	return "../ui/dist"
}

func startGatewayProxy(socketPath string, targetPort int) {
	// Clean up existing socket
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}

	// Ensure parent directory exists
	socketDir := filepath.Dir(socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		slog.Error("无法创建网关socket目录", "dir", socketDir, "error", err)
		return
	}
	// Restrict socket directory to owner+group: the Unix socket is a trusted
	// local tunnel for the fnOS gateway proxy.  Tightening dir permissions
	// prevents other local processes from reaching the socket.
	os.Chmod(socketDir, 0750)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		slog.Error("无法监听网关socket", "path", socketPath, "error", err)
		return
	}

	// Set permissions
	os.Chmod(socketPath, 0660)

	targetAddr := fmt.Sprintf("127.0.0.1:%d", targetPort)

	go func() {
		defer ln.Close()
		slog.Info("Unix socket 代理已启动", "socket", socketPath, "target", targetAddr)

		for {
			conn, err := ln.Accept()
			if err != nil {
				if !isClosedError(err) {
					slog.Error("Unix socket接受连接失败", "error", err)
				}
				return
			}

			go func() {
				defer conn.Close()

				target, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
				if err != nil {
					slog.Warn("无法连接到主服务器", "target", targetAddr, "error", err)
					return
				}
				defer target.Close()

				// Bidirectional copy
				errChan := make(chan error, 2)
				go func() {
					_, err := copyBuffer(conn, target)
					errChan <- err
				}()
				go func() {
					_, err := copyBuffer(target, conn)
					errChan <- err
				}()
				<-errChan
			}()
		}
	}()
}

func copyBuffer(dst net.Conn, src net.Conn) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			total += int64(nw)
			if ew != nil {
				return total, ew
			}
			if nr != nw {
				return total, fmt.Errorf("short write")
			}
		}
		if er != nil {
			return total, er
		}
	}
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "use of closed network connection" ||
		err.Error() == "closed"
}
