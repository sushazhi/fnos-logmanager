package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
)

// rateLimitEntry is a simple fixed-window counter keyed by client IP.
type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// Server is the MCP (Model Context Protocol) Streamable HTTP server for
// fnos-logmanager. It exposes the app's full capability set as MCP tools so
// that AI agents (QwenPAW, OpenClaw, Hermes, etc.) can manage logs, docker
// containers, event logs, kernels, backups and audit data via JSON-RPC.
type Server struct {
	mu         sync.RWMutex
	sessions   map[string]*sessionState
	apiKey     string
	appName    string
	appVersion string
	serverInfo map[string]interface{}

	toolCallsMu     sync.Mutex
	toolCallsWindow time.Duration
	toolCallsMax    int
	toolCalls       map[string]*rateLimitEntry
}

// toolCallRateLimit defines the per-IP budget for MCP tools/call invocations.
// AI agents can make bursts of calls, so this is deliberately generous yet still
// bounds runaway/abusive clients that hammer destructive tools (delete_log,
// remove_kernel, cleanup_kernels).
const (
	toolCallRateLimitMax    = 120
	toolCallRateLimitWindow = 60 * time.Second
)

// sessionState tracks the initialization state of an MCP client session.
type sessionState struct {
	createdAt       time.Time
	initialized     bool
	protocolVersion string
}

// New creates a new MCP Server.
func New(appName, appVersion, apiKey string) *Server {
	return &Server{
		sessions:        make(map[string]*sessionState),
		apiKey:          apiKey,
		appName:         appName,
		appVersion:      appVersion,
		toolCallsWindow: toolCallRateLimitWindow,
		toolCallsMax:    toolCallRateLimitMax,
		toolCalls:       make(map[string]*rateLimitEntry),
	}
}

// allowToolCall applies a fixed-window per-IP rate limit to tools/call requests
// and returns true when the call may proceed. It returns false (and triggers a
// cleanup of expired entries) when the client has exceeded the budget.
func (s *Server) allowToolCall(remoteAddr string) bool {
	ip := remoteAddr
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		ip = host
	}

	s.toolCallsMu.Lock()
	defer s.toolCallsMu.Unlock()

	now := time.Now()
	// Opportunistic cleanup to bound memory usage.
	if len(s.toolCalls) > 256 {
		for k, e := range s.toolCalls {
			if now.Sub(e.resetTime) >= s.toolCallsWindow {
				delete(s.toolCalls, k)
			}
		}
	}

	entry := s.toolCalls[ip]
	if entry == nil || now.Sub(entry.resetTime) >= s.toolCallsWindow {
		s.toolCalls[ip] = &rateLimitEntry{count: 1, resetTime: now.Add(s.toolCallsWindow)}
		return true
	}
	if entry.count >= s.toolCallsMax {
		return false
	}
	entry.count++
	return true
}

// Handler returns an http.Handler that serves the MCP endpoint.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.handleHTTP)
}

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

// isAuthorized checks the request against the configured API key. The key is
// read from the live config each request so that frontend updates take effect
// without restarting the process.
// Supported credential sources:
//   - Authorization: Bearer <key>
//   - X-API-Key: <key>
//   - (fallback) loopback clients when no key is configured (single-user/local)
func (s *Server) isAuthorized(r *http.Request) bool {
	// If MCP is disabled at runtime, reject all requests.
	live := config.LoadMCPConfig()
	if !live.Enabled {
		return false
	}

	// Prefer the live API key so frontend updates take effect without a
	// restart. Fall back to the startup value (used by tests / direct embedders).
	key := live.APIKey
	if key == "" {
		key = s.apiKey
	}

	if key == "" {
		// No key configured: allow loopback only (local trusted access).
		return isLoopback(r.RemoteAddr)
	}

	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") && constantTimeEqual(auth[7:], key) {
		return true
	}
	if k := r.Header.Get("X-API-Key"); k != "" && constantTimeEqual(k, key) {
		return true
	}
	return false
}

// constantTimeEqual compares two strings in constant time to avoid timing
// side-channel leaks of the API key.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func isLoopback(remoteAddr string) bool {
	host := remoteAddr
	if i := strings.LastIndex(remoteAddr, ":"); i >= 0 {
		host = remoteAddr[:i]
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

func (s *Server) createSession() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	sid := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[sid] = &sessionState{
		createdAt: time.Now(),
	}
	s.mu.Unlock()
	return sid
}

func (s *Server) markInitialized(sid string) {
	s.mu.Lock()
	if st, ok := s.sessions[sid]; ok {
		st.initialized = true
	}
	s.mu.Unlock()
}

func (s *Server) deleteSession(sid string) {
	s.mu.Lock()
	delete(s.sessions, sid)
	s.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Message processing
// ---------------------------------------------------------------------------

// processMessage handles a single JSON-RPC message and returns the response
// (nil for notifications).
func (s *Server) processMessage(msg jsonRPCRequest, sessionID string) interface{} {
	switch msg.Method {
	case methodInitialize:
		return s.handleInitialize(msg)

	case methodInitialized, methodRootsListChanged, methodCancelled:
		// Notifications: no response.
		return nil

	case methodPing:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result:  map[string]interface{}{},
		}

	case methodToolsList:
		return s.handleToolsList(msg)

	case methodToolsCall:
		return s.handleToolsCall(msg)

	case methodResourcesList, methodResourcesRead, methodPromptsList, methodCompletion, methodSetLevel:
		// Not implemented: return an empty result so clients don't fail hard.
		if msg.Method == methodResourcesList {
			return &jsonRPCResponse{JSONRPC: "2.0", ID: msg.ID, Result: map[string]interface{}{"resources": []interface{}{}}}
		}
		if msg.Method == methodPromptsList {
			return &jsonRPCResponse{JSONRPC: "2.0", ID: msg.ID, Result: map[string]interface{}{"prompts": []interface{}{}}}
		}
		return &jsonRPCResponse{JSONRPC: "2.0", ID: msg.ID, Result: map[string]interface{}{}}

	default:
		return &jsonRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error:   &jsonRPCError{Code: errMethodNotFound, Message: "method not found: " + msg.Method},
		}
	}
}

// handleInitialize responds to the MCP initialize handshake.
func (s *Server) handleInitialize(msg jsonRPCRequest) interface{} {
	clientVersion := defaultProtocolVersion
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(msg.Params) > 0 {
		if err := json.Unmarshal(msg.Params, &params); err == nil && params.ProtocolVersion != "" {
			clientVersion = params.ProtocolVersion
		}
	}

	// Negotiate: if the client asks for a version we understand, echo it;
	// otherwise fall back to our default.
	negotiated := defaultProtocolVersion
	if isSupportedProtocolVersion(clientVersion) {
		negotiated = clientVersion
	}

	result := map[string]interface{}{
		"protocolVersion": negotiated,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": false,
			},
		},
		"serverInfo": map[string]interface{}{
			"name":    s.appName,
			"version": s.appVersion,
		},
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  result,
	}
}

// handleToolsList returns the list of available tools.
func (s *Server) handleToolsList(msg jsonRPCRequest) interface{} {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"tools": allTools(),
		},
	}
}

// handleToolsCall dispatches a tool invocation.
func (s *Server) handleToolsCall(msg jsonRPCRequest) interface{} {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &jsonRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error:   &jsonRPCError{Code: errInvalidParams, Message: "invalid params: " + err.Error()},
		}
	}

	if params.Name == "" {
		return &jsonRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error:   &jsonRPCError{Code: errInvalidParams, Message: "missing tool name"},
		}
	}

	text, err := invokeTool(params.Name, params.Arguments)
	result := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": text,
			},
		},
	}
	if err != nil {
		result["isError"] = true
		// Append the error into the text so the model can act on it.
		if te, ok := result["content"].([]interface{}); ok && len(te) > 0 {
			te[0].(map[string]interface{})["text"] = text + "\n\n[错误] " + err.Error()
		}
	}

	slog.Debug("mcp tools/call", "tool", params.Name, "err", err)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  result,
	}
}
