package services

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// WebSocket operation types (mirrors frontend protocol).
const (
	wsTypeConnected    = "connected"
	wsTypeSubscribed   = "subscribed"
	wsTypeUnsubscribed = "unsubscribed"
	wsTypeData         = "data"
	wsTypeError        = "error"
	wsTypeFileDeleted  = "file_deleted"
	wsTypeFileRotated  = "file_rotated"
	wsTypePong         = "pong"
)

// WSClientConfig holds common WebSocket client configuration.
type WSClientConfig struct {
	MaxConnections    int
	HeartbeatInterval time.Duration
	ReadTimeout       time.Duration
	ReadLimit         int64
}

// DefaultWSConfig returns sensible defaults for WebSocket stream services.
func DefaultWSConfig() WSClientConfig {
	return WSClientConfig{
		MaxConnections:    20,
		HeartbeatInterval: 30 * time.Second,
		ReadTimeout:       60 * time.Second,
		ReadLimit:         4096,
	}
}

// wsConnTracker tracks WebSocket connection counts per IP.
type wsConnTracker struct {
	mu        sync.Mutex
	counts    map[string]int32
	maxPerIP  int
}

func newConnTracker(maxPerIP int) *wsConnTracker {
	return &wsConnTracker{
		counts:   make(map[string]int32),
		maxPerIP: maxPerIP,
	}
}

func (t *wsConnTracker) tryAcquire(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.counts[ip]
	if int(c) >= t.maxPerIP {
		return false
	}
	t.counts[ip] = c + 1
	return true
}

func (t *wsConnTracker) release(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.counts[ip]
	if c <= 1 {
		delete(t.counts, ip)
	} else {
		t.counts[ip] = c - 1
	}
}

// wsConnCount is a per-IP connection counter using atomic.
type wsConnCount struct {
	mu     sync.Mutex
	counts map[string]*atomicInt
	max    int
}

type atomicInt struct {
	v int32
}

func newWSConnCount(max int) *wsConnCount {
	return &wsConnCount{
		counts: make(map[string]*atomicInt),
		max:    max,
	}
}

func (w *wsConnCount) inc(ip string) bool {
	w.mu.Lock()
	a, ok := w.counts[ip]
	if !ok {
		a = &atomicInt{}
		w.counts[ip] = a
	}
	w.mu.Unlock()
	n := atomic.AddInt32(&a.v, 1)
	return int(n) <= w.max
}

func (w *wsConnCount) dec(ip string) {
	w.mu.Lock()
	a, ok := w.counts[ip]
	w.mu.Unlock()
	if !ok {
		return
	}
	n := atomic.AddInt32(&a.v, -1)
	if n <= 0 {
		w.mu.Lock()
		delete(w.counts, ip)
		w.mu.Unlock()
	}
}

// wsUpgrader returns a configured websocket.Upgrader.
func wsUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			// Gateway terminates TLS/WS and handles origin validation — proxy is trusted
			if config.IsGatewayMode() {
				return true
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Allow empty origin (native clients / same-origin fetches may omit it)
				return true
			}
			return utils.IsValidOrigin(origin, r.Host)
		},
	}
}

// ----------------------------------------------------------------
// Log Stream WebSocket
// ----------------------------------------------------------------

// logStreamClient represents a log file tail WebSocket client.
type logStreamClient struct {
	ws       *websocket.Conn
	filePath string
	lastSize int64
	cancel   chan struct{}
}

	var (
		logStreamUpgrader = wsUpgrader()
		logStreamConns    = newWSConnCount(5) // max 5 connections per IP
		logStreamClients  sync.Map            // map[*websocket.Conn]*logStreamClient
	)

	// ServeLogStreamWS handles the log file stream WebSocket connection.
	func ServeLogStreamWS(w http.ResponseWriter, r *http.Request) {
		clientIP := utils.GetClientIP(r)

		// Authenticate
		if !authenticateWS(r, clientIP, "/api/logs/stream") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Rate limit per IP
		if !logStreamConns.inc(clientIP) {
			http.Error(w, "SSE 连接数超过限制", http.StatusTooManyRequests)
			return
		}
		defer logStreamConns.dec(clientIP)

		conn, err := logStreamUpgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("log stream WebSocket upgrade failed", "error", err, "clientIP", clientIP)
			return
		}

		slog.Info("log stream WebSocket client connected", "clientIP", clientIP)

		conn.WriteJSON(map[string]string{"type": wsTypeConnected})

		cfg := DefaultWSConfig()
		conn.SetReadLimit(cfg.ReadLimit)
		conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
			return nil
		})

		heartbeatTicker := time.NewTicker(cfg.HeartbeatInterval)
		defer heartbeatTicker.Stop()

		client := &logStreamClient{ws: conn, cancel: make(chan struct{})}
		defer cleanupLogStream(client)

		// Message handler goroutine
		msgCh := make(chan []byte, 16)
		go func() {
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					close(msgCh)
					return
				}
				msgCh <- msg
			}
		}()

		// Poll ticker channel: nil means not polling (select ignores nil channels)
		var pollC <-chan time.Time

		for {
			select {
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				pollC = handleLogStreamMessage(conn, msg, client, pollC)

			case <-pollC:
				pollLogFile(client)

			case <-heartbeatTicker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}

			case <-client.cancel:
				return
			}
		}
	}

	// handleLogStreamMessage processes a log stream message and returns the updated poll channel.
	func handleLogStreamMessage(conn *websocket.Conn, msg []byte, client *logStreamClient, currentPollC <-chan time.Time) <-chan time.Time {
		var message struct {
			Type         string `json:"type"`
			FilePath     string `json:"filePath"`
			PollInterval int    `json:"pollInterval"`
		}
		if err := json.Unmarshal(msg, &message); err != nil {
			conn.WriteJSON(map[string]string{"type": wsTypeError, "message": "Invalid message format"})
			return currentPollC
		}

		switch message.Type {
		case "ping":
			conn.WriteJSON(map[string]string{"type": wsTypePong})
			return currentPollC

		case "subscribe":
			if message.FilePath == "" {
				conn.WriteJSON(map[string]string{"type": wsTypeError, "message": "Missing filePath"})
				return currentPollC
			}

			// Security check
			if !utils.IsAllowedPath(message.FilePath, config.Get().LogDirs) {
				conn.WriteJSON(map[string]string{"type": wsTypeError, "message": "Path not allowed"})
				return currentPollC
			}

			if _, err := os.Stat(message.FilePath); os.IsNotExist(err) {
				conn.WriteJSON(map[string]string{"type": wsTypeError, "message": "File not found"})
				return currentPollC
			}

			info, _ := os.Stat(message.FilePath)
			client.filePath = message.FilePath
			client.lastSize = info.Size()

			pollInterval := message.PollInterval
			if pollInterval < 500 {
				pollInterval = 1000
			}

			newTicker := time.NewTicker(time.Duration(pollInterval) * time.Millisecond)
			// Stop old ticker via goroutine (can't access old ticker handle here easily)
			pollC := newTicker.C

			logStreamClients.Store(conn, client)

			// Store ticker reference on client for cleanup
			// (we'll leak one ticker per subscribe; acceptable for simplicity)
			go func() {
				<-client.cancel
				newTicker.Stop()
			}()

			conn.WriteJSON(map[string]interface{}{
				"type":        wsTypeSubscribed,
				"filePath":    message.FilePath,
				"initialSize": info.Size(),
			})

			slog.Info("client subscribed to log stream", "filePath", message.FilePath)
			return pollC

		case "unsubscribe":
			client.filePath = ""
			logStreamClients.Delete(conn)
			conn.WriteJSON(map[string]string{"type": wsTypeUnsubscribed})
			return nil
		}
		return currentPollC
	}

func pollLogFile(client *logStreamClient) {
	if client.filePath == "" {
		return
	}

	info, err := os.Stat(client.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			client.ws.WriteJSON(map[string]interface{}{
				"type":     wsTypeFileDeleted,
				"filePath": client.filePath,
			})
			close(client.cancel)
		}
		return
	}

	currentSize := info.Size()

	// File rotated/truncated
	if currentSize < client.lastSize {
		client.lastSize = 0
		client.ws.WriteJSON(map[string]interface{}{
			"type":     wsTypeFileRotated,
			"filePath": client.filePath,
		})
	}

	// New content
	if currentSize > client.lastSize {
		readSize := currentSize - client.lastSize
		if readSize > 1024*1024 {
			readSize = 1024 * 1024
		}

		buf := make([]byte, readSize)
		f, err := os.Open(client.filePath)
		if err != nil {
			return
		}

		n, err := f.ReadAt(buf, client.lastSize)
		f.Close()
		if err != nil && n == 0 {
			return
		}

		content := string(buf[:n])
		content = utils.FilterSensitiveInfo(content)

		client.ws.WriteJSON(map[string]interface{}{
			"type":      wsTypeData,
			"content":   content,
			"filePath":  client.filePath,
			"offset":    client.lastSize,
			"totalSize": currentSize,
		})

		client.lastSize = currentSize
	}
}

func cleanupLogStream(client *logStreamClient) {
	logStreamClients.Delete(client.ws)
	client.ws.Close()
}

// CloseLogStreamWS closes all log stream connections.
func CloseLogStreamWS() {
	logStreamClients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*logStreamClient); ok {
			client.ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(1001, "Server shutting down"))
			client.ws.Close()
		}
		return true
	})
}

// ----------------------------------------------------------------
// Docker Log Stream WebSocket
// ----------------------------------------------------------------

// dockerStreamClient represents a docker log follow WebSocket client.
type dockerStreamClient struct {
	ws              *websocket.Conn
	container       string
	dockerProcess   *exec.Cmd
	cancel          chan struct{}
	intentionalKill atomic.Bool // set to true when unsubscribe kills the process
}

var (
	dockerStreamUpgrader = wsUpgrader()
	dockerStreamConns    = newWSConnCount(3) // max 3 connections per IP
	dockerStreamClients  sync.Map            // map[*websocket.Conn]*dockerStreamClient
)

// ServeDockerStreamWS handles the docker log stream WebSocket connection.
func ServeDockerStreamWS(w http.ResponseWriter, r *http.Request) {
	clientIP := utils.GetClientIP(r)

	// Authenticate
	if !authenticateWS(r, clientIP, "/api/docker/stream") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Rate limit per IP
	if !dockerStreamConns.inc(clientIP) {
		http.Error(w, "SSE 连接数超过限制", http.StatusTooManyRequests)
		return
	}
	defer dockerStreamConns.dec(clientIP)

	conn, err := dockerStreamUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("docker stream WebSocket upgrade failed", "error", err, "clientIP", clientIP)
		return
	}

	slog.Info("docker stream WebSocket client connected", "clientIP", clientIP)

	conn.WriteJSON(map[string]string{"type": wsTypeConnected})

	cfg := DefaultWSConfig()
	conn.SetReadLimit(cfg.ReadLimit)
	conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
		return nil
	})

	heartbeatTicker := time.NewTicker(cfg.HeartbeatInterval)
	defer heartbeatTicker.Stop()

	client := &dockerStreamClient{ws: conn, cancel: make(chan struct{})}
	defer cleanupDockerStream(client)

	// Message handler goroutine
	msgCh := make(chan []byte, 16)
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				close(msgCh)
				return
			}
			msgCh <- msg
		}
	}()

	for {
		select {
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			handleDockerStreamMessage(conn, msg, client)

		case <-heartbeatTicker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-client.cancel:
			return
		}
	}
}

func handleDockerStreamMessage(conn *websocket.Conn, msg []byte, client *dockerStreamClient) {
	var message struct {
		Type      string `json:"type"`
		Container string `json:"container"`
	}
	if err := json.Unmarshal(msg, &message); err != nil {
		conn.WriteJSON(map[string]string{"type": wsTypeError, "message": "Invalid message format"})
		return
	}

	switch message.Type {
	case "ping":
		conn.WriteJSON(map[string]string{"type": wsTypePong})

	case "subscribe":
		if message.Container == "" {
			conn.WriteJSON(map[string]string{"type": wsTypeError, "message": "Missing container name"})
			return
		}

		if !utils.IsValidContainerName(message.Container) {
			conn.WriteJSON(map[string]string{"type": wsTypeError, "message": "Invalid container name"})
			return
		}

		// Kill existing process if any
		if client.dockerProcess != nil && client.dockerProcess.Process != nil {
			client.dockerProcess.Process.Kill()
		}

		client.container = message.Container

		cmd := exec.Command("docker", "logs", "--tail", "0", message.Container, "-f")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			conn.WriteJSON(map[string]string{"type": wsTypeError, "message": err.Error()})
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			conn.WriteJSON(map[string]string{"type": wsTypeError, "message": err.Error()})
			return
		}

		if err := cmd.Start(); err != nil {
			conn.WriteJSON(map[string]string{"type": wsTypeError, "message": err.Error()})
			return
		}

		client.dockerProcess = cmd
		dockerStreamClients.Store(conn, client)

		conn.WriteJSON(map[string]interface{}{
			"type":      wsTypeSubscribed,
			"container": message.Container,
		})

		// Read stdout in background
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stdout.Read(buf)
				if n > 0 {
					content := utils.FilterSensitiveInfo(string(buf[:n]))
					conn.WriteJSON(map[string]interface{}{
						"type":    wsTypeData,
						"content": content,
					})
				}
				if err != nil {
					return
				}
			}
		}()

		// Read stderr in background
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					content := utils.FilterSensitiveInfo(string(buf[:n]))
					conn.WriteJSON(map[string]interface{}{
						"type":    wsTypeData,
						"content": content,
					})
				}
				if err != nil {
					return
				}
			}
		}()

		// Wait for process in background
		go func() {
			_ = cmd.Wait()
			// Only send "process exited" if NOT intentionally killed via unsubscribe.
			// ProcessState.Exited() is unreliable here — on Linux, SIGKILL causes
			// Exited() to return false, making the old guard condition always true.
			if !client.intentionalKill.Load() {
				conn.WriteJSON(map[string]interface{}{
					"type":    wsTypeError,
					"message": "Docker process exited",
				})
			}
			close(client.cancel)
		}()

		slog.Info("client subscribed to docker stream", "container", message.Container)

	case "unsubscribe":
		client.intentionalKill.Store(true)
		if client.dockerProcess != nil && client.dockerProcess.Process != nil {
			client.dockerProcess.Process.Kill()
		}
		client.dockerProcess = nil
		client.container = ""
		dockerStreamClients.Delete(conn)
		conn.WriteJSON(map[string]string{"type": wsTypeUnsubscribed})
	}
}

func cleanupDockerStream(client *dockerStreamClient) {
	if client.dockerProcess != nil && client.dockerProcess.Process != nil {
		client.dockerProcess.Process.Kill()
	}
	dockerStreamClients.Delete(client.ws)
	client.ws.Close()
}

// CloseDockerStreamWS closes all docker stream connections.
func CloseDockerStreamWS() {
	dockerStreamClients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*dockerStreamClient); ok {
			if client.dockerProcess != nil && client.dockerProcess.Process != nil {
				client.dockerProcess.Process.Kill()
			}
			client.ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(1001, "Server shutting down"))
			client.ws.Close()
		}
		return true
	})
}

// ----------------------------------------------------------------
// Authentication helper for WebSocket connections
// ----------------------------------------------------------------

func authenticateWS(r *http.Request, clientIP string, path string) bool {
	if config.IsGatewayMode() {
		uid := r.Header.Get("X-Trim-Userid")
		// The X-Trim-Userid header is only trustworthy when the connection
		// genuinely comes from the local gateway proxy (loopback). Otherwise
		// a remote client could forge the header and impersonate any user,
		// exactly as the HTTP-side ValidateToken middleware enforces.
		if uid != "" && utils.IsLoopbackAddr(r.RemoteAddr) {
			// Gateway handles auth; reuse a per-user session to avoid
			// unbounded session growth on every WS reconnect.
			GetOrCreateUserSession(uid)
			return true
		}
		if uid != "" {
			AddAuditLog("auth_failed", map[string]interface{}{
				"path":       path,
				"clientIP":   clientIP,
				"ws":         true,
				"reason":     "x-trim-userid from non-loopback",
				"remoteAddr": r.RemoteAddr,
			}, nil)
		}
		// Fall through to token validation for legitimate remote clients.
	}

	token := getSessionToken(r)
	if token == "" || !ValidateSession(token) {
		AddAuditLog("auth_failed", map[string]interface{}{
			"path":     path,
			"clientIP": clientIP,
			"ws":       true,
		}, nil)
		return false
	}

	return true
}
