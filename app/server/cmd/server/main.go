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

	// Create HTTP server (bind to 127.0.0.1 in gateway mode to prevent bypassing the gateway)
	bindAddr := "0.0.0.0"
	if cfg.GatewaySocket != "" {
		bindAddr = "127.0.0.1"
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
	_ = services.StopMonitor()
	services.CloseLogStreamWS()
	services.CloseDockerStreamWS()
	services.GetNotifyHub().CloseAll()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("服务关闭异常", "error", err)
	}

	slog.Info("服务已关闭")
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
		"../../ui/dist",         // from cmd/server/ (dev: Vite dist output)
		"../../ui",              // from cmd/server/ (source)
		"../ui/dist",            // from app/server/
		"../ui",                 // from app/server/
		"/app/logmanager/ui",    // fnOS install path
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
