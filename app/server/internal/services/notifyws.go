package services

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// NotifyClient represents a connected notification WebSocket client.
type NotifyClient struct {
	hub           *NotifyHub
	ws            *websocket.Conn
	subscriptions map[string]bool // "monitor", "history", "rules"
	send          chan []byte
	clientIP      string
}

// NotifyHub manages all notification WebSocket connections.
type NotifyHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]*NotifyClient
	upgrader websocket.Upgrader
}

// Global notification hub instance.
var globalNotifyHub *NotifyHub

// GetNotifyHub returns the global notification hub, creating it if needed.
func GetNotifyHub() *NotifyHub {
	if globalNotifyHub == nil {
		globalNotifyHub = NewNotifyHub()
	}
	return globalNotifyHub
}

// NewNotifyHub creates a new NotifyHub.
func NewNotifyHub() *NotifyHub {
	return &NotifyHub{
		clients: make(map[*websocket.Conn]*NotifyClient),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Gateway mode: skip origin check (gateway handles it)
				if config.IsGatewayMode() {
					return true
				}
				// Direct mode: validate origin
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				host := r.Host
				// Simple origin check
				return utils.IsValidOrigin(origin, host)
			},
		},
	}
}

// ServeWS handles an HTTP upgrade to WebSocket for the notification channel.
func (h *NotifyHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	clientIP := utils.GetClientIP(r)

	// Authenticate
	sessionToken := getSessionToken(r)
	isGatewayMode := config.IsGatewayMode()

	// In gateway mode the fnOS gateway injects X-Trim-Userid for authenticated
	// users. This header is only trustworthy when the connection genuinely comes
	// from the local gateway proxy (loopback); otherwise a remote client could
	// forge it to bypass authentication, matching the HTTP-side ValidateToken.
	if isGatewayMode {
		uid := r.Header.Get("X-Trim-Userid")
		if uid != "" && utils.IsLoopbackAddr(r.RemoteAddr) {
			// Gateway authenticated the user; proceed without further checks.
		} else if uid != "" {
			AddAuditLog("auth_failed", map[string]interface{}{
				"path":       r.URL.Path,
				"clientIP":   clientIP,
				"ws":         true,
				"reason":     "x-trim-userid from non-loopback",
				"remoteAddr": r.RemoteAddr,
			}, nil)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else if sessionToken == "" || !ValidateSession(sessionToken) {
			// No gateway header — fall back to validating the session token.
			AddAuditLog("auth_failed", map[string]interface{}{
				"path":     r.URL.Path,
				"clientIP": clientIP,
				"ws":       true,
			}, nil)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	} else if sessionToken == "" || !ValidateSession(sessionToken) {
		AddAuditLog("auth_failed", map[string]interface{}{
			"path":     r.URL.Path,
			"clientIP": clientIP,
			"ws":       true,
		}, nil)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("notify WebSocket upgrade failed", "error", err, "clientIP", clientIP)
		return
	}

	client := &NotifyClient{
		hub:           h,
		ws:            conn,
		subscriptions: make(map[string]bool),
		send:          make(chan []byte, 256),
		clientIP:      clientIP,
	}

	h.mu.Lock()
	h.clients[conn] = client
	h.mu.Unlock()

	slog.Info("notify WebSocket client connected", "clientIP", clientIP)

	// Send initial connected message
	conn.WriteJSON(map[string]interface{}{"type": "connected"})

	// Push current state
	h.pushInitialState(client)

	// Start write pump
	go client.writePump()

	// Read pump (blocks)
	client.readPump()
}

// pushInitialState sends current monitor status, rules, and recent history.
func (h *NotifyHub) pushInitialState(client *NotifyClient) {
	// Monitor status
	if status := GetMonitorStatus(); status != nil {
		client.sendJSON(map[string]interface{}{
			"type": "status",
			"data": status,
		})
	}

	// Rules
	hub := GetConfigStore()
	if hub != nil {
		rules := hub.GetRules()
		if rules != nil {
			client.sendJSON(map[string]interface{}{
				"type": "rules",
				"data": rules,
			})
		}

		// Recent history
		history := hub.GetHistory(5)
		if history != nil {
			client.sendJSON(map[string]interface{}{
				"type": "history",
				"data": history,
			})
		}
	}
}

// readPump reads messages from the WebSocket connection.
func (c *NotifyClient) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.ws.Close()
	}()

	c.ws.SetReadLimit(4096)
	c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("notify WebSocket read error", "error", err, "clientIP", c.clientIP)
			}
			break
		}

		var msg struct {
			Type     string   `json:"type"`
			Channels []string `json:"channels"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			for _, ch := range msg.Channels {
				if ch == "monitor" || ch == "history" || ch == "rules" {
					c.subscriptions[ch] = true
				}
			}
		case "unsubscribe":
			for _, ch := range msg.Channels {
				delete(c.subscriptions, ch)
			}
		case "ping":
			c.sendJSON(map[string]string{"type": "pong"})
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
func (c *NotifyClient) writePump() {
	ticker := time.NewTicker(30 * time.Second) // ping interval
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendJSON sends a JSON-encoded message to this client.
func (c *NotifyClient) sendJSON(data interface{}) {
	msg, err := json.Marshal(data)
	if err != nil {
		return
	}
	select {
	case c.send <- msg:
	default:
		// Client send buffer full, drop message
	}
}

// register adds a client to the hub.
func (h *NotifyHub) register(c *NotifyClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.ws] = c
}

// unregister removes a client from the hub.
func (h *NotifyHub) unregister(c *NotifyClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c.ws]; ok {
		delete(h.clients, c.ws)
		close(c.send)
		slog.Info("notify WebSocket client disconnected", "clientIP", c.clientIP)
	}
}

// Broadcast sends a message to all clients subscribed to the given channel.
func (h *NotifyHub) Broadcast(channel string, data interface{}) {
	msg, err := json.Marshal(map[string]interface{}{
		"type":      "update",
		"channel":   channel,
		"data":      data,
		"timestamp": time.Now().UnixMilli(),
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		if client.subscriptions[channel] {
			select {
			case client.send <- msg:
			default:
				// Client buffer full, skip
			}
		}
	}
}

// BroadcastToAll sends a message to all connected clients regardless of subscription.
func (h *NotifyHub) BroadcastToAll(data interface{}) {
	msg, err := json.Marshal(data)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.send <- msg:
		default:
		}
	}
}

// CloseAll disconnects all clients and shuts down the hub.
func (h *NotifyHub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, client := range h.clients {
		client.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1001, "Server shutting down"))
		client.ws.Close()
		close(client.send)
	}
	h.clients = make(map[*websocket.Conn]*NotifyClient)
}

// GetNotifyHubStats returns connection statistics.
func (h *NotifyHub) GetNotifyHubStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subCounts := map[string]int{"monitor": 0, "history": 0, "rules": 0}
	for _, client := range h.clients {
		for ch := range client.subscriptions {
			subCounts[ch]++
		}
	}

	return map[string]interface{}{
		"totalClients":  len(h.clients),
		"subscriptions": subCounts,
	}
}

// getSessionToken extracts the session token from the request.
func getSessionToken(r *http.Request) string {
	// Check cookie first
	cookie, err := r.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// NOTE: query-parameter token (?token=) is intentionally NOT accepted here.
	// The frontend no longer sends it (it relied on the session cookie / Bearer header),
	// and accepting it would leak the session token into access logs, Referer headers,
	// and browser history.

	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}

	return ""
}

// GetConfigStore returns the global notification store.
func GetConfigStore() *NotificationStore {
	return notifStore
}

// Ensure the notification store reference is accessible.
var notifStore *NotificationStore

// SetNotifyStore sets the global notification store reference.
func SetNotifyStore(ns *NotificationStore) {
	notifStore = ns
}
