package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// TrimAPIClient communicates with the fnOS backend open API via Unix socket.
//
// Reference: https://developer.fnnas.com/api/calling/
//
// 统一网关调用方式:
//   - Unix socket: /var/run/trim_open_gateway_apiscope.socket
//   - Endpoint:    POST /api/v1/trimapp
//   - Auth:        Authorization: Bearer <TRIM_API_TOKEN>
//   - 请求体格式:  { reqId, req, appName, data }
//   - 响应体格式:  { reqId, code, msg, data }
type TrimAPIClient struct {
	socketPath string
	apiToken   string
	appName    string
	httpClient *http.Client
	mu         sync.RWMutex
}

var (
	trimClient     *TrimAPIClient
	trimClientOnce sync.Once
)

const (
	defaultSocketPath  = "/var/run/trim_open_gateway_apiscope.socket"
	trimRequestTimeout = 10 * time.Second
)

// codeOf extracts the "code" field from a trim API error response body for
// diagnostics. It returns an empty string if the body is not JSON or lacks code.
func codeOf(body []byte) string {
	var parsed struct {
		Code json.Number `json:"code"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	return parsed.Code.String()
}

// GetTrimClient returns the singleton TrimAPIClient.
func GetTrimClient() *TrimAPIClient {
	trimClientOnce.Do(func() {
		token := os.Getenv("TRIM_API_TOKEN")
		appName := os.Getenv("TRIM_APP_NAME")
		if appName == "" {
			appName = "logmanager"
		}
		socketPath := os.Getenv("TRIM_API_SOCKET")
		if socketPath == "" {
			socketPath = defaultSocketPath
		}

		client := &TrimAPIClient{
			socketPath: socketPath,
			apiToken:   token,
			appName:    appName,
		}
		client.httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
			Timeout: trimRequestTimeout,
		}
		trimClient = client

		if token != "" {
			slog.Info("trim API client initialized",
				"socket", socketPath,
				"appName", appName,
				"hasToken", true)
		} else {
			slog.Warn("trim API client initialized without token (TRIM_API_TOKEN not set)")
		}
	})
	return trimClient
}

// GetBaseURL returns the socket path (for logging).
func (c *TrimAPIClient) GetBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.socketPath
}

// ---------------------------------------------------------------------------
// 统一网关请求格式
// ---------------------------------------------------------------------------

// apiRequest is the unified gateway request envelope.
// 参考文档: POST /api/v1/trimapp
type apiRequest struct {
	ReqID   string      `json:"reqId"`
	Req     string      `json:"req"`
	AppName string      `json:"appName"`
	Data    interface{} `json:"data"`
}

// apiResponse is the unified gateway response envelope.
type apiResponse struct {
	ReqID string          `json:"reqId"`
	Code  int             `json:"code"`
	Msg   string          `json:"msg"`
	Data  json.RawMessage `json:"data"`
}

// call sends a request to the unified gateway API endpoint.
func (c *TrimAPIClient) call(apiMethod string, data interface{}) (json.RawMessage, error) {
	c.mu.RLock()
	token := c.apiToken
	appName := c.appName
	c.mu.RUnlock()

	reqBody := apiRequest{
		ReqID:   fmt.Sprintf("%d", time.Now().UnixNano()),
		Req:     apiMethod,
		AppName: appName,
		Data:    data,
	}

	rawBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "http://unix/api/v1/trimapp", bytes.NewReader(rawBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("trim API request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		// 403/401 均为已知的开放平台能力降级场景，不影响核心日志管理功能。
		// 使用 Debug 级别记录（含 appName/socket/token 状态）便于按需排查，避免每次启动打印 WARN。
		if resp.StatusCode == http.StatusForbidden {
			slog.Debug("trim API 403 Forbidden",
				"api", apiMethod,
				"code", codeOf(respData),
				"appName", appName,
				"socket", c.socketPath,
				"hasToken", token != "")
		} else if resp.StatusCode == http.StatusUnauthorized && token == "" {
			slog.Debug("trim API 401: TRIM_API_TOKEN 未注入", "api", apiMethod)
		} else if resp.StatusCode == http.StatusUnauthorized {
			slog.Debug("trim API 401 Unauthorized", "api", apiMethod, "hasToken", token != "")
		}
		return nil, fmt.Errorf("trim API error (status %d): %s", resp.StatusCode, string(respData))
	}

	var envelope apiResponse
	if err := json.Unmarshal(respData, &envelope); err != nil {
		return nil, fmt.Errorf("parse response envelope: %w", err)
	}

	if envelope.Code != 0 {
		return nil, fmt.Errorf("trim API error (code %d): %s", envelope.Code, envelope.Msg)
	}

	return envelope.Data, nil
}

// ---------------------------------------------------------------------------
// trim.file.checkUserACL
// Scope: trim.file.userAcl
// 参考: https://developer.fnnas.com/api/authorization/file-acl/
// ---------------------------------------------------------------------------

// checkUserACLRequest 请求体
// path 支持 string 或 []string
type checkUserACLRequest struct {
	UID  interface{} `json:"uid"`
	Path interface{} `json:"path"` // string or []string
}

// checkUserACLItem 单条结果
type checkUserACLItem struct {
	Path      string `json:"path"`
	Readable  bool   `json:"readable"`
	Writable  bool   `json:"writable"`
	Deletable bool   `json:"deletable"`
}

// CheckUserACL 检查用户对路径的权限。
// userID 会被转为 number 类型（文档要求 uid 为 number）。
// action: "readable", "writable", "deletable"
// 返回 (allowed, reason, error)
func (c *TrimAPIClient) CheckUserACL(userID, path, action string) (bool, string, error) {
	reqData := checkUserACLRequest{
		UID:  toNumber(userID),
		Path: path,
	}

	data, err := c.call("trim.file.checkUserACL", reqData)
	if err != nil {
		slog.Warn("trim.file.checkUserACL failed, falling back to allow",
			"userId", userID, "path", path, "action", action, "error", err)
		return true, "trim API unavailable, access allowed by fallback", nil
	}

	var result []checkUserACLItem
	if err := json.Unmarshal(data, &result); err != nil {
		slog.Warn("trim.file.checkUserACL parse error", "error", err)
		return true, "parse error, access allowed by fallback", nil
	}

	if len(result) == 0 {
		return false, "路径不存在或无权限", nil
	}

	item := result[0]
	switch action {
	case "readable":
		return item.Readable, "", nil
	case "writable":
		return item.Writable, "", nil
	case "deletable":
		return item.Deletable, "", nil
	default:
		return item.Readable, "", nil
	}
}

// ---------------------------------------------------------------------------
// trim.file.convertPath
// Scope: trim.file.path
// 参考: https://developer.fnnas.com/api/authorization/path-convert/
// ---------------------------------------------------------------------------

// convertPathRequest 请求体
type convertPathRequest struct {
	Path     interface{} `json:"path"` // string or []string
	Language string      `json:"language"`
}

// convertPathResultItem 单条结果
type convertPathResultItem struct {
	Path         string `json:"path"`
	SemanticPath string `json:"semanticPath"`
}

// convertPathResponseData 文档描述的响应 data 字段（兼容对象格式）
type convertPathResponseData struct {
	Status int                     `json:"status"`
	Result []convertPathResultItem `json:"result"`
}

// parseConvertPathResult 解析 trim.file.convertPath 响应的 data 字段。
// 实际网关返回的是数组格式 [ {path, semanticPath}, ... ]，
// 部分版本返回文档描述的 { status, result: [...] } 对象格式。
// 这里同时兼容两种格式，取最稳妥的解析结果。
func parseConvertPathResult(data []byte) []convertPathResultItem {
	// 1. 数组格式：直接是 []convertPathResultItem
	var arr []convertPathResultItem
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}

	// 2. 对象格式：{ status, result: [...] }
	var obj convertPathResponseData
	if err := json.Unmarshal(data, &obj); err == nil {
		return obj.Result
	}

	return nil
}

// ConvertPath 将内部路径转换为语义化路径。
// language 可选，默认 zh-CN（文档要求必传，此处做容错）。
func (c *TrimAPIClient) ConvertPath(path, language string) (string, error) {
	if language == "" {
		language = "zh-CN"
	}

	reqData := convertPathRequest{
		Path:     path,
		Language: language,
	}

	data, err := c.call("trim.file.convertPath", reqData)
	if err != nil {
		// convertPath 失败时静默降级为原始路径，属正常兜底，用 Debug 级别避免刷屏。
		slog.Debug("trim.file.convertPath failed, using original path",
			"path", path, "error", err)
		return path, nil
	}

	items := parseConvertPathResult(data)
	if len(items) == 0 {
		slog.Debug("trim.file.convertPath parse error, using original path", "path", path)
		return path, nil
	}

	for _, item := range items {
		if item.Path == path && item.SemanticPath != "" {
			return item.SemanticPath, nil
		}
	}

	// 未匹配到该 path 时，退而返回第一条结果（兼容网关只回传一条的情况）
	if items[0].SemanticPath != "" {
		return items[0].SemanticPath, nil
	}

	return path, nil
}

// ConvertPaths 批量转换路径。
// language 可选，默认 zh-CN。
func (c *TrimAPIClient) ConvertPaths(paths []string, language string) map[string]string {
	result := make(map[string]string, len(paths))
	if len(paths) == 0 {
		return result
	}

	if language == "" {
		language = "zh-CN"
	}

	reqData := convertPathRequest{
		Path:     paths,
		Language: language,
	}

	data, err := c.call("trim.file.convertPath", reqData)
	if err != nil {
		slog.Debug("trim.file.convertPath batch failed", "count", len(paths), "error", err)
		for _, p := range paths {
			result[p] = p
		}
		return result
	}

	items := parseConvertPathResult(data)
	for _, item := range items {
		if item.SemanticPath != "" {
			result[item.Path] = item.SemanticPath
		} else {
			result[item.Path] = item.Path
		}
	}

	for _, p := range paths {
		if _, ok := result[p]; !ok {
			result[p] = p
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// trim.file.getUserAccessibleFolders
// Scope: trim.file.userAccess
// ---------------------------------------------------------------------------

// getUserAccessibleFoldersRequest 请求体
type getUserAccessibleFoldersRequest struct {
	UID interface{} `json:"uid"`
}

// getUserAccessibleFoldersResponse 响应 data
type getUserAccessibleFoldersResponse struct {
	Paths []string `json:"paths"`
}

// GetUserAccessibleFolders 查询当前用户的已授权目录列表。
func (c *TrimAPIClient) GetUserAccessibleFolders(uid string) ([]string, error) {
	reqData := getUserAccessibleFoldersRequest{
		UID: toNumber(uid),
	}

	data, err := c.call("trim.file.getUserAccessibleFolders", reqData)
	if err != nil {
		return nil, err
	}

	var resp getUserAccessibleFoldersResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return resp.Paths, nil
}

// ---------------------------------------------------------------------------
// trim.file.delUserAccessibleFolder
// Scope: trim.file.userAccess
// ---------------------------------------------------------------------------

// delUserAccessibleFolderRequest 请求体
type delUserAccessibleFolderRequest struct {
	UID  interface{} `json:"uid"`
	Path string      `json:"path"`
}

// delUserAccessibleFolderResponse 响应 data
type delUserAccessibleFolderResponse struct {
	Suc bool `json:"suc"`
}

// DelUserAccessibleFolder 删除用户授权目录。
func (c *TrimAPIClient) DelUserAccessibleFolder(uid, path string) (bool, error) {
	reqData := delUserAccessibleFolderRequest{
		UID:  toNumber(uid),
		Path: path,
	}

	data, err := c.call("trim.file.delUserAccessibleFolder", reqData)
	if err != nil {
		return false, err
	}

	var resp delUserAccessibleFolderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, fmt.Errorf("parse response: %w", err)
	}

	return resp.Suc, nil
}

// ---------------------------------------------------------------------------
// trim.file.getSharedAccessibleFolders
// Scope: trim.file.sharedAccess
// ---------------------------------------------------------------------------

// GetSharedAccessibleFolders 查询应用共享授权目录列表。
func (c *TrimAPIClient) GetSharedAccessibleFolders() ([]string, error) {
	// 该接口不需要额外 data 字段
	data, err := c.call("trim.file.getSharedAccessibleFolders", nil)
	if err != nil {
		return nil, err
	}

	var resp getUserAccessibleFoldersResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return resp.Paths, nil
}

// ---------------------------------------------------------------------------
// trim.file.delSharedAccessibleFolder
// Scope: trim.file.sharedAccess
// ---------------------------------------------------------------------------

// DelSharedAccessibleFolder 删除共享授权目录。
func (c *TrimAPIClient) DelSharedAccessibleFolder(path string) (bool, error) {
	reqData := delUserAccessibleFolderRequest{
		Path: path,
	}

	data, err := c.call("trim.file.delSharedAccessibleFolder", reqData)
	if err != nil {
		return false, err
	}

	var resp delUserAccessibleFolderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, fmt.Errorf("parse response: %w", err)
	}

	return resp.Suc, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// toNumber converts a string uid to a number for JSON serialization.
// If the string is a valid integer, it returns an int64; otherwise returns the string as-is.
// 文档要求 uid 为 number 类型。
func toNumber(s string) interface{} {
	n, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return n
	}
	return s
}
