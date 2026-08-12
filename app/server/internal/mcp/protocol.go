package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 messages
// ---------------------------------------------------------------------------

// jsonRPCRequest is a JSON-RPC 2.0 request or notification.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"` // nil => notification
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 success response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
}

// jsonRPCErrorResponse is a JSON-RPC 2.0 error response.
type jsonRPCErrorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Error   *jsonRPCError   `json:"error"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternalError  = -32603
)

// MCP method names.
const (
	methodInitialize       = "initialize"
	methodInitialized      = "notifications/initialized"
	methodPing             = "ping"
	methodToolsList        = "tools/list"
	methodToolsCall        = "tools/call"
	methodResourcesList    = "resources/list"
	methodResourcesRead    = "resources/read"
	methodPromptsList      = "prompts/list"
	methodCompletion       = "completion/complete"
	methodSetLevel         = "logging/setLevel"
	methodRootsListChanged = "notifications/roots/list_changed"
	methodCancelled        = "notifications/cancelled"
)

// Protocol versions we accept (echoed back to the client).
const (
	protocolVersion20240618 = "2024-06-18"
	protocolVersion20241105 = "2024-11-05"
	protocolVersion20250326 = "2025-03-26"
	protocolVersion20250618 = "2025-06-18"
	defaultProtocolVersion   = protocolVersion20250326
)

// isSupportedProtocolVersion returns true if the version string is one we can
// speak. Unknown/newer versions are negotiated down to our default.
func isSupportedProtocolVersion(v string) bool {
	switch v {
	case protocolVersion20240618, protocolVersion20241105, protocolVersion20250326, protocolVersion20250618:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Streamable HTTP transport
// ---------------------------------------------------------------------------

const (
	headerMCPProtocolVersion = "MCP-Protocol-Version"
	headerMcpSessionID       = "Mcp-Session-Id"
	contentTypeJSON          = "application/json"
	contentTypeSSE           = "text/event-stream"
)

// handleRequest is the entry point for the Streamable HTTP transport.
// It routes GET / POST / DELETE on the /mcp endpoint.
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePOST(w, r)
	case http.MethodGet:
		// GET is optional per spec: used for SSE long-poll style connections.
		// We support it by holding the connection open and streaming responses
		// for an active session, but the primary flow is POST-based.
		s.handleGET(w, r)
	case http.MethodDelete:
		s.handleDELETE(w, r)
	case http.MethodOptions:
		s.handleCORS(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, MCP-Protocol-Version, Mcp-Session-Id, Accept, X-API-Key")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePOST(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		s.writeProtocolError(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Require the MCP-Protocol-Version header.
	proto := r.Header.Get(headerMCPProtocolVersion)
	if proto == "" {
		// Older clients may omit it; default to our supported version.
		proto = defaultProtocolVersion
	}

	body, err := readBody(r)
	if err != nil {
		s.writeProtocolError(w, r, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	var msg jsonRPCRequest
	if err := json.Unmarshal(body, &msg); err != nil {
		s.writeJSON(w, r, http.StatusBadRequest, jsonRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &jsonRPCError{Code: errParseError, Message: "parse error: " + err.Error()},
		})
		return
	}
	if msg.JSONRPC != "2.0" {
		s.writeJSON(w, r, http.StatusBadRequest, jsonRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error:   &jsonRPCError{Code: errInvalidRequest, Message: "invalid jsonrpc version"},
		})
		return
	}

	sessionID := r.Header.Get(headerMcpSessionID)

	// Rate-limit tool invocations per client IP so that a compromised or
	// misbehaving agent cannot hammer destructive tools (delete_log,
	// remove_kernel, cleanup_kernels) without bound.
	if msg.Method == methodToolsCall && !s.allowToolCall(r.RemoteAddr) {
		slog.Warn("mcp tools/call rate limit exceeded", "remote", r.RemoteAddr)
		s.writeJSON(w, r, http.StatusTooManyRequests, jsonRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error:   &jsonRPCError{Code: -32029, Message: "too many tool calls"},
		})
		return
	}

	// Process and capture the response (single JSON-RPC message or nil for
	// notifications). Streamable responses (tools/call with SSE) are handled
	// inside callTool.
	resp := s.processMessage(msg, sessionID)

	if resp == nil {
		// Notification: 202 Accepted, no body.
		w.Header().Set(headerMCPProtocolVersion, proto)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// For initialize responses, mint a session id and return it.
	if msg.Method == methodInitialize {
		sid := s.createSession()
		w.Header().Set(headerMcpSessionID, sid)
		// Associate session so later requests pass validation.
		s.markInitialized(sid)
	}

	if isSSEAccepted(r) {
		s.writeSSE(w, r, resp, proto)
		return
	}

	s.writeJSON(w, r, http.StatusOK, resp)
}

// handleGET supports SSE-based clients that prefer to open a stream. Per the
// Streamable HTTP spec, a GET establishes a long-lived SSE connection. We
// return the session endpoint and keep the connection open briefly for
// server-initiated messages (not currently used by our tools).
func (s *Server) handleGET(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		s.writeProtocolError(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}
	proto := r.Header.Get(headerMCPProtocolVersion)
	if proto == "" {
		proto = defaultProtocolVersion
	}
	w.Header().Set("Content-Type", contentTypeSSE)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	// Flush headers so the client receives the connection.
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	// Keep the connection open until the client disconnects.
	<-r.Context().Done()
}

func (s *Server) handleDELETE(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get(headerMcpSessionID)
	if sessionID != "" {
		s.deleteSession(sessionID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Response writers
// ---------------------------------------------------------------------------

func (s *Server) writeProtocolError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	proto := r.Header.Get(headerMCPProtocolVersion)
	if proto == "" {
		proto = defaultProtocolVersion
	}
	w.Header().Set(headerMCPProtocolVersion, proto)
	w.Header().Set("Content-Type", contentTypeJSON)
	// 明确声明这是 Bearer（静态 token）认证，而不是 OAuth。
	// 这让符合规范的 Streamable HTTP 客户端使用配置好的 Authorization: Bearer
	// 头直接认证，而不会误入 RFC 9728 OAuth 授权发现流程
	// （QwenPAW 会因服务器未暴露 OAuth 元数据而报 "Could not discover OAuth
	// authorization server for this MCP endpoint"）。
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="mcp", charset="UTF-8"`)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(jsonRPCErrorResponse{
		JSONRPC: "2.0",
		ID:      nil,
		Error:   &jsonRPCError{Code: errInvalidRequest, Message: msg},
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, payload interface{}) {
	proto := r.Header.Get(headerMCPProtocolVersion)
	if proto == "" {
		proto = defaultProtocolVersion
	}
	w.Header().Set(headerMCPProtocolVersion, proto)
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeSSE writes a single JSON-RPC message as an SSE event stream.
func (s *Server) writeSSE(w http.ResponseWriter, r *http.Request, payload interface{}, proto string) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("mcp: marshal SSE payload failed", "error", err)
		return
	}
	w.Header().Set(headerMCPProtocolVersion, proto)
	w.Header().Set("Content-Type", contentTypeSSE)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// SSE framing: one event with a single data line (JSON has no raw newlines
	// after Marshal unless they were in strings, which we escape).
	var buf bytes.Buffer
	buf.WriteString("event: message\n")
	buf.WriteString("data: ")
	buf.Write(data)
	buf.WriteString("\n\n")

	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.Warn("mcp: failed to write SSE response", "error", err)
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// isSSEAccepted reports whether the client asked for a streamed response.
//
// Streamable HTTP: a client may send "Accept: application/json, text/event-stream".
// For a single (non-streaming) response we prefer application/json, which is what
// most agents expect for initialize / tools/list. We only stream when the client
// explicitly prefers SSE and does NOT also accept plain JSON.
func isSSEAccepted(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	if strings.Contains(accept, "application/json") {
		return false
	}
	return strings.Contains(accept, contentTypeSSE) ||
		strings.Contains(accept, "application/x-ndjson")
}

// readBody reads the request body with a size limit.
func readBody(r *http.Request) ([]byte, error) {
	// Cap at 1 MiB.
	const maxBody = 1 << 20
	if r.ContentLength > maxBody {
		return nil, fmt.Errorf("request body too large")
	}
	buf := make([]byte, 0, r.ContentLength)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if len(buf) > maxBody {
			return nil, fmt.Errorf("request body too large")
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}
	return buf, nil
}
