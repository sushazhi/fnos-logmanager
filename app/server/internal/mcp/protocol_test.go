package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInitializeHandshake(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "")
	resp := s.processMessage(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  methodInitialize,
		Params:  json.RawMessage(`{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	}, "")
	if resp == nil {
		t.Fatal("expected a response for initialize")
	}
	r := resp.(*jsonRPCResponse)
	result, ok := r.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", r.Result)
	}
	if result["protocolVersion"] != "2025-03-26" {
		t.Errorf("expected protocolVersion 2025-03-26, got %v", result["protocolVersion"])
	}
	if result["serverInfo"] == nil {
		t.Error("expected serverInfo in initialize result")
	}
}

func TestInitializeNegotiatesUnknownVersion(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "")
	resp := s.processMessage(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  methodInitialize,
		Params:  json.RawMessage(`{"protocolVersion":"2099-01-01"}`),
	}, "")
	result := resp.(*jsonRPCResponse).Result.(map[string]interface{})
	if result["protocolVersion"] != defaultProtocolVersion {
		t.Errorf("expected fallback version %s, got %v", defaultProtocolVersion, result["protocolVersion"])
	}
}

func TestToolsListContainsCoreTools(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "")
	resp := s.processMessage(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  methodToolsList,
	}, "")
	result := resp.(*jsonRPCResponse).Result.(map[string]interface{})
	tools, ok := result["tools"].([]tool)
	if !ok {
		t.Fatalf("expected []tool, got %T", result["tools"])
	}
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.Name] = true
	}
	for _, want := range []string{
		"list_dirs", "list_logs", "search_logs", "read_log", "tail_log",
		"truncate_log", "delete_log", "clean_logs", "backup_logs",
		"list_docker_containers", "get_docker_logs", "get_event_logs",
		"list_kernels", "remove_kernel", "get_audit_logs",
		"clean_uninstalled_dirs", "list_recycle_items", "restore_recycle_item",
	} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestUnknownToolReturnsError(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "")
	resp := s.processMessage(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  methodToolsCall,
		Params:  json.RawMessage(`{"name":"does_not_exist","arguments":{}}`),
	}, "")
	r, ok := resp.(*jsonRPCResponse)
	if !ok {
		t.Fatalf("expected jsonRPCResponse, got %T", resp)
	}
	result, ok := r.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", r.Result)
	}
	if result["isError"] != true {
		t.Errorf("expected isError=true for unknown tool, got %v", result["isError"])
	}
	// invokeTool should fail with unknown tool
	if _, err := invokeTool("does_not_exist", nil); err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestMethodNotFound(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "")
	resp := s.processMessage(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "bogus/method",
	}, "")
	if _, ok := resp.(*jsonRPCErrorResponse); !ok {
		t.Fatalf("expected error response for unknown method, got %T", resp)
	}
}

func TestNotificationReturnsNil(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "")
	resp := s.processMessage(jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  methodInitialized,
	}, "")
	if resp != nil {
		t.Fatalf("expected nil response for notification, got %T", resp)
	}
}

// TestHTTPStreamableFlow drives a full Streamable HTTP exchange via httptest:
// initialize -> tools/list -> tools/call (returns text content).
func TestHTTPStreamableFlow(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "testkey")
	h := s.Handler()

	// initialize
	var sessionID string
	t.Run("initialize", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set(headerMCPProtocolVersion, "2025-03-26")
		req.Header.Set("Authorization", "Bearer testkey")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("initialize status = %d, body = %s", w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected application/json content type, got %q", ct)
		}
		sessionID = w.Header().Get(headerMcpSessionID)
		if sessionID == "" {
			t.Error("expected Mcp-Session-Id header on initialize response")
		}
		var rr jsonRPCResponse
		if err := json.Unmarshal(w.Body.Bytes(), &rr); err != nil {
			t.Fatalf("invalid json response: %v", err)
		}
		if rr.Result == nil {
			t.Error("expected result in initialize response")
		}
	})

	// tools/list
	t.Run("tools/list", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set(headerMCPProtocolVersion, "2025-03-26")
		req.Header.Set(headerMcpSessionID, sessionID)
		req.Header.Set("Authorization", "Bearer testkey")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("tools/list status = %d, body = %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "list_logs") {
			t.Errorf("tools/list missing list_logs: %s", w.Body.String())
		}
	})
}

func TestAuthRequired(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "secret")
	h := s.Handler()

	// No token from non-loopback should be rejected.
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerMCPProtocolVersion, "2025-03-26")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}

	// Wrong token should be rejected.
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerMCPProtocolVersion, "2025-03-26")
	req.Header.Set("Authorization", "Bearer wrong")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", w.Code)
	}
}

func TestSSEResponse(t *testing.T) {
	s := New("fnos-logmanager", "0.0.0", "testkey")
	h := s.Handler()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set(headerMCPProtocolVersion, "2025-03-26")
	req.Header.Set("Authorization", "Bearer testkey")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "event: message") || !strings.Contains(w.Body.String(), "data:") {
		t.Errorf("SSE framing missing: %q", w.Body.String())
	}
}

// rawID builds a json.RawMessage for an integer id.
func rawID(n int) *json.RawMessage {
	b, _ := json.Marshal(n)
	raw := json.RawMessage(b)
	return &raw
}
