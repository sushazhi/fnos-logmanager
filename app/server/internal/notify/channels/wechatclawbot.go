package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

const defaultClawBotBaseURL = "https://ilinkai.weixin.qq.com"

// In-memory cache for last interacted user (for when no toUser is configured)
var clawBotLastInteractedUser string

// -------- Global captured credentials (in-memory cache) --------

var (
	clawMu              sync.RWMutex
	clawCapturedBotToken    string
	clawCapturedAccountID   string
	clawCapturedBaseURL     string
)

// GetCapturedCredentials returns the cached bot token and account ID.
func GetCapturedCredentials() (botToken, accountID, baseURL string) {
	clawMu.RLock()
	defer clawMu.RUnlock()
	return clawCapturedBotToken, clawCapturedAccountID, clawCapturedBaseURL
}

// SetLastInteractedUser sets the last interacted user ID.
func SetLastInteractedUser(userID string) {
	if userID != "" {
		clawBotLastInteractedUser = userID
	}
}

// GetLastInteractedUser returns the last interacted user ID.
func GetLastInteractedUser() string {
	return clawBotLastInteractedUser
}

const defaultClawTimeout = 20 * time.Second

// iLinkClient is the WeChat iLink API client, ported from Node.js version.
type iLinkClient struct {
	baseURL   string
	botToken  string
	accountID string
	syncBuf   string
}

func newILinkClient(baseURL string) *iLinkClient {
	return &iLinkClient{
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (c *iLinkClient) setCredentials(botToken, accountID, syncBuf string) {
	if botToken != "" {
		c.botToken = botToken
	}
	if accountID != "" {
		c.accountID = accountID
	}
	if syncBuf != "" {
		c.syncBuf = syncBuf
	}
}

func (c *iLinkClient) buildHeaders(authRequired bool) map[string]string {
	h := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/plain, */*",
		"User-Agent":   "LogManager-WechatClawBot/1.0",
	}
	if authRequired && c.botToken != "" {
		h["AuthorizationType"] = "ilink_bot_token"
		h["Authorization"] = "Bearer " + c.botToken
		h["X-WECHAT-UIN"] = randHex(8)
	}
	return h
}

// parseJSON attempts to parse a JSON string into a map.
func parseJSON(body string) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return nil
	}
	return m
}

// isOk checks if the API response indicates success.
func isOk(payload map[string]interface{}) bool {
	if len(payload) == 0 {
		return false
	}
	// Check errcode/code/ret
	for _, key := range []string{"errcode", "code", "ret"} {
		if v, ok := payload[key]; ok {
			switch val := v.(type) {
			case float64:
				return val == 0
			case int:
				return val == 0
			}
		}
	}
	// Check error/message
	errMsg, _ := payload["errmsg"].(string)
	if errMsg != "" && !strings.EqualFold(errMsg, "ok") && !strings.EqualFold(errMsg, "success") {
		return false
	}
	msg, _ := payload["message"].(string)
	if msg != "" && !strings.EqualFold(msg, "ok") && !strings.EqualFold(msg, "success") {
		return false
	}
	err, _ := payload["error"].(string)
	if err != "" && !strings.EqualFold(err, "ok") && !strings.EqualFold(err, "success") {
		return false
	}
	return true
}

// findFirstValue recursively searches for the first non-empty value matching any of the keys.
func findFirstValue(data interface{}, keys []string, maxDepth int) interface{} {
	if maxDepth < 0 || data == nil {
		return nil
	}
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}
	for _, k := range keys {
		if v, exists := m[k]; exists && v != nil && v != "" {
			return v
		}
	}
	for _, v := range m {
		if vm, ok := v.(map[string]interface{}); ok {
			if found := findFirstValue(vm, keys, maxDepth-1); found != nil {
				return found
			}
		}
		if va, ok := v.([]interface{}); ok {
			for _, item := range va {
				if found := findFirstValue(item, keys, maxDepth-1); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

// getData extracts the data/result sub-object from a response payload.
func getData(payload map[string]interface{}) map[string]interface{} {
	for _, key := range []string{"data", "result"} {
		if d, ok := payload[key].(map[string]interface{}); ok {
			return d
		}
	}
	return payload
}

// getQRCode calls the iLink API to get a QR code for login.
// POST {baseURL}/ilink/bot/get_bot_qrcode?bot_type=3
func (c *iLinkClient) getQRCode() (success bool, qrcode, qrcodeURL, message string) {
	url := c.baseURL + "/ilink/bot/get_bot_qrcode?bot_type=3"
	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		Headers: c.buildHeaders(false),
		Timeout: defaultClawTimeout,
	})
	if err != nil {
		return false, "", "", "获取二维码异常: " + err.Error()
	}
	payload := parseJSON(resp.Body)
	if payload == nil || len(payload) == 0 {
		return false, "", "", "获取二维码失败: 空响应"
	}
	data := getData(payload)

	qrcode, _ = data["qrcode"].(string)
	if qrcode == "" {
		if v, ok := data["qr_code"].(string); ok {
			qrcode = v
		}
	}
	if qrcode == "" {
		if v, ok := data["qrcode_id"].(string); ok {
			qrcode = v
		}
	}
	if qrcode == "" {
		if v, ok := data["ticket"].(string); ok {
			qrcode = v
		}
	}

	// Build qrcodeURL from various response fields
	qrcodeURL = normalizeQRCodeUrl(firstNonEmpty(
		getString(data, "qrcode_url"),
		getString(data, "url"),
		getString(data, "qrcodeUrl"),
		getString(data, "qr_url"),
		getString(data, "qrcode_img_content"),
		getString(data, "qrcode_img_url"),
		getString(data, "qr_img"),
	))
	if qrcodeURL == "" && qrcode != "" {
		qrcodeURL = "https://liteapp.weixin.qq.com/q/7GiQu1?qrcode=" + qrcode + "&bot_type=3"
	}

	if isOk(payload) && (qrcode != "" || qrcodeURL != "") {
		return true, qrcode, qrcodeURL, "获取二维码成功"
	}
	return false, "", "", firstNonEmpty(
		getString(payload, "errmsg"),
		getString(payload, "message"),
		"获取二维码失败",
	)
}

// getQRCodeStatus polls the QR code scan status.
// GET {baseURL}/ilink/bot/get_qrcode_status?qrcode=xxx
func (c *iLinkClient) getQRCodeStatus(qrcode string) (success bool, status, token, accountID, scannerID, message string) {
	fullURL := c.baseURL + "/ilink/bot/get_qrcode_status?qrcode=" + urlQueryEscape(qrcode)

	var payload map[string]interface{}

	// Try with query param first
	resp, err := notify.HTTPGet(fullURL, notify.HttpRequestOptions{
		Headers: c.buildHeaders(false),
		Timeout: defaultClawTimeout,
	})
	if err == nil {
		payload = parseJSON(resp.Body)
	}
	// Retry without param if first attempt failed
	if payload == nil {
		resp2, err2 := notify.HTTPGet(c.baseURL+"/ilink/bot/get_qrcode_status", notify.HttpRequestOptions{
			Headers: c.buildHeaders(false),
			Timeout: defaultClawTimeout,
		})
		if err2 == nil {
			payload = parseJSON(resp2.Body)
		}
	}
	if payload == nil {
		return false, "waiting", "", "", "", "查询二维码状态失败: 空响应"
	}

	data := getData(payload)

	token = firstNonEmpty(
		getString(data, "bot_token"),
		getString(data, "token"),
		getString(data, "access_token"),
		toString(findFirstValue(data, []string{"bot_token", "access_token", "token", "jwt", "auth_token"}, 5)),
	)
	accountID = firstNonEmpty(
		getString(data, "account_id"),
		getString(data, "ilink_bot_id"),
		getString(data, "wxid"),
		getString(data, "uid"),
		getString(data, "user_id"),
		toString(findFirstValue(data, []string{"account_id", "ilink_bot_id", "wxid", "uid", "user_id", "from_user"}, 5)),
	)
	newBaseURL := firstNonEmpty(
		getString(data, "baseurl"),
		getString(data, "base_url"),
		getString(payload, "baseurl"),
		getString(payload, "base_url"),
	)
	scannerID = firstNonEmpty(
		getString(data, "wxid"),
		getString(data, "from_user"),
		getString(data, "uid"),
		getString(data, "user_id"),
		getString(data, "from_uid"),
		toString(findFirstValue(data, []string{"wxid", "from_user", "from_user_id", "from_uid", "uid", "user_id"}, 5)),
	)
	status = strings.ToLower(firstNonEmpty(
		getString(data, "status"),
		getString(data, "state"),
		getString(payload, "status"),
		getString(payload, "state"),
		toString(findFirstValue(data, []string{"status", "state", "scan_status"}, 5)),
		"waiting",
	))

	// On successful scan, save credentials
	clawMu.Lock()
	if (status == "scanned" || status == "confirmed" || status == "success" || status == "ok") && token != "" {
		c.botToken = token
		clawCapturedBotToken = token
		if accountID != "" {
			c.accountID = accountID
			clawCapturedAccountID = accountID
		}
		if newBaseURL != "" {
			c.baseURL = strings.TrimRight(newBaseURL, "/")
			clawCapturedBaseURL = c.baseURL
		}
		if scannerID != "" {
			clawBotLastInteractedUser = scannerID
		}
		slog.Info("微信 ClawBot 扫码登录成功，已获取凭证", "accountId", accountID)
	}
	clawMu.Unlock()

	return isOk(payload), status, token, accountID, scannerID,
		firstNonEmpty(getString(payload, "errmsg"), getString(payload, "message"))
}

// getUpdates polls for unread messages to capture the scanner's user ID.
// POST {baseURL}/ilink/bot/getupdates
func (c *iLinkClient) getUpdates() (success bool, messages []map[string]string, syncBuf, message string) {
	if c.botToken == "" {
		return false, nil, "", "BotToken 未配置"
	}

	url := c.baseURL + "/ilink/bot/getupdates"
	body := map[string]interface{}{
		"sync_buf": c.syncBuf,
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON:    body,
		Headers: c.buildHeaders(true),
		Timeout: defaultClawTimeout,
	})
	if err != nil {
		return false, nil, "", "getUpdates 异常: " + err.Error()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, nil, "", fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	payload := parseJSON(resp.Body)
	if payload == nil {
		return false, nil, "", "getUpdates 返回空响应"
	}

	// Update syncBuf cursor
	if newSyncBuf := toString(findFirstValue(payload, []string{"sync_buf", "syncBuf", "sync_buf_id"}, 3)); newSyncBuf != "" {
		c.syncBuf = newSyncBuf
	}

	data := getData(payload)

	// Extract message list
	var items []interface{}
	for _, key := range []string{"msgs", "messages", "msg_list", "message_list", "items", "list", "data", "records", "events", "updates", "messageList", "msgList"} {
		if arr, ok := data[key].([]interface{}); ok && len(arr) > 0 {
			items = arr
			break
		}
	}
	if len(items) == 0 {
		if found := findFirstValue(data, []string{"msgs", "messages", "msg_list", "message_list", "items", "list", "data", "records", "events", "updates", "messageList", "msgList"}, 5); found != nil {
			if arr, ok := found.([]interface{}); ok {
				items = arr
			}
		}
	}
	// Top-level array — check payload.data / payload.result for arrays (matching Node.js behavior)
	if len(items) == 0 {
		for _, key := range []string{"data", "result"} {
			if arr, ok := payload[key].([]interface{}); ok {
				items = arr
				break
			}
		}
	}

	messages = make([]map[string]string, 0)
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		userID := firstNonEmpty(
			getString(obj, "from_user_id"),
			getString(obj, "fromUserId"),
			getString(obj, "from_user"),
			getString(obj, "fromUser"),
			getString(obj, "sender"),
			getString(obj, "userid"),
			getString(obj, "userId"),
			getString(obj, "from_uid"),
		)
		if userID == "" {
			continue
		}
		// Extract text
		text := ""
		itemList, _ := obj["item_list"].([]interface{})
		if itemList == nil {
			itemList, _ = obj["items"].([]interface{})
		}
		if itemList == nil {
			itemList, _ = obj["itemList"].([]interface{})
		}
		if itemList != nil {
			for _, it := range itemList {
				if itm, ok := it.(map[string]interface{}); ok {
					textItem, _ := itm["text_item"].(map[string]interface{})
					if textItem == nil {
						textItem, _ = itm["textItem"].(map[string]interface{})
					}
					if textItem != nil {
						if t, ok := textItem["text"].(string); ok {
							text = t
							break
						}
					}
				}
			}
		}
		if text == "" {
			text = firstNonEmpty(
				getString(obj, "text"),
				getString(obj, "content"),
				getString(obj, "msg"),
				getString(obj, "message"),
			)
		}
		if len(text) > 200 {
			text = text[:200]
		}
		messages = append(messages, map[string]string{
			"userId": userID,
			"text":   text,
		})
	}

	return true, messages, c.syncBuf, ""
}

// -------- Exported helpers (used by routes for QR login flow) --------

var (
	clientMu     sync.Mutex
	clientOnce   sync.Once
	clientInst   *iLinkClient
)

func getClient() *iLinkClient {
	clientOnce.Do(func() {
		baseURL := notify.GetConfig("WECHAT_CLAWBOT_BASE_URL")
		if baseURL == "" {
			baseURL = defaultClawBotBaseURL
		}
		clientInst = newILinkClient(baseURL)
		botToken := notify.GetConfig("WECHAT_CLAWBOT_BOT_TOKEN")
		accountID := notify.GetConfig("WECHAT_CLAWBOT_ACCOUNT_ID")
		if botToken != "" || accountID != "" {
			clientInst.setCredentials(botToken, accountID, "")
		}
	})
	return clientInst
}

// GetClawBotQRCode gets a QR code for WeChat ClawBot login.
func GetClawBotQRCode() (success bool, qrcode, qrcodeURL, message string) {
	return getClient().getQRCode()
}

// GetClawBotUpdates polls for unread messages to capture the scanner's user ID.
func GetClawBotUpdates() (success bool, messages []map[string]string, message string) {
	clawMu.RLock()
	bt := clawCapturedBotToken
	aid := clawCapturedAccountID
	bu := clawCapturedBaseURL
	clawMu.RUnlock()
	if bt == "" {
		bt = notify.GetConfig("WECHAT_CLAWBOT_BOT_TOKEN")
	}
	if bu == "" {
		bu = notify.GetConfig("WECHAT_CLAWBOT_BASE_URL")
	}
	if bu == "" {
		bu = defaultClawBotBaseURL
	}
	client := newILinkClient(bu)
	client.setCredentials(bt, aid, "")
	ok, msgs, _, msg := client.getUpdates()
	if ok && len(msgs) > 0 {
		if uid, ok := msgs[0]["userId"]; ok && uid != "" {
			clawBotLastInteractedUser = uid
			slog.Info("微信 ClawBot 通过 getUpdates 获取到互动用户", "userId", uid)
		}
	}
	return ok, msgs, msg
}

// CheckClawBotQRCodeStatus checks the QR code scan status.
func CheckClawBotQRCodeStatus(qrcode string) (success bool, status, token, accountID, scannerID, message string) {
	return getClient().getQRCodeStatus(qrcode)
}

// -------- wechatClawBotChannel --------

type WechatClawBot struct {
	enabled bool
}

func (c *WechatClawBot) Name() string {
	return "wechat-claw-bot"
}

func (c *WechatClawBot) Enabled() bool {
	return c.enabled
}

func (c *WechatClawBot) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *WechatClawBot) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("WECHAT_CLAWBOT_BOT_TOKEN") {
		return notify.NotifyResult{Success: false, Message: "WECHAT_CLAWBOT_BOT_TOKEN 未配置，请先扫码登录"}
	}

	botToken := notify.GetConfig("WECHAT_CLAWBOT_BOT_TOKEN")
	toUser := notify.GetConfig("WECHAT_CLAWBOT_TO_USER")
	baseURL := notify.GetConfig("WECHAT_CLAWBOT_BASE_URL")
	accountID := notify.GetConfig("WECHAT_CLAWBOT_ACCOUNT_ID")

	if baseURL == "" {
		baseURL = defaultClawBotBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Use last interacted user if no toUser configured
	if toUser == "" {
		if clawBotLastInteractedUser != "" {
			toUser = clawBotLastInteractedUser
		} else {
			return notify.NotifyResult{Success: false, Message: "WECHAT_CLAWBOT_TO_USER 未配置，且无最近互动用户"}
		}
	}

	// Build fromUserId with @im.bot suffix if needed
	fromUserID := ""
	if accountID != "" {
		if strings.Contains(accountID, "@") {
			fromUserID = accountID
		} else {
			fromUserID = accountID + "@im.bot"
		}
	}

	content := text
	if desp != "" {
		content = "**" + text + "**\n\n" + desp
	}

	// URL candidates (MoviePilot uses these two)
	urlCandidates := []string{
		baseURL + "/ilink/bot/sendmessage",
		baseURL + "/ilink/bot/sendmessage?bot_type=3",
	}

	// User ID candidates (original + various suffix variants)
	userCandidates := []string{toUser}
	if strings.Contains(toUser, "@") {
		userCandidates = append(userCandidates, strings.SplitN(toUser, "@", 2)[0])
	}
	if strings.HasSuffix(toUser, "@im.wechat") {
		userCandidates = append(userCandidates, strings.TrimSuffix(toUser, "@im.wechat"))
	} else {
		userCandidates = append(userCandidates, toUser+"@im.wechat")
	}
	// Deduplicate
	seen := make(map[string]bool)
	uniqueUsers := make([]string, 0, len(userCandidates))
	for _, u := range userCandidates {
		if !seen[u] {
			seen[u] = true
			uniqueUsers = append(uniqueUsers, u)
		}
	}
	userCandidates = uniqueUsers

	headers := map[string]string{
		"Content-Type":      "application/json",
		"Authorization":     "Bearer " + botToken,
		"AuthorizationType": "ilink_bot_token",
		"User-Agent":        "LogManager-WechatClawBot/1.0",
		"X-WECHAT-UIN":      randHex(8),
	}

	lastError := ""
	for _, userID := range userCandidates {
		// Build payloads for this user ID
		payloads := []map[string]interface{}{
			// Protocol format (primary, same as MoviePilot)
			{
				"base_info": map[string]string{"channel_version": "1.0.2"},
				"msg": map[string]interface{}{
					"from_user_id":  fromUserID,
					"to_user_id":    userID,
					"client_id":     fmt.Sprintf("go-%d", time.Now().UnixMilli()),
					"message_type":  2,
					"message_state": 2,
					"item_list": []map[string]interface{}{
						{ "type": 1, "text_item": map[string]string{"text": content} },
					},
				},
			},
			// Simple formats
			{"base_info": map[string]string{"channel_version": "1.0.2"}, "to_user": userID, "msg_type": "text", "text": map[string]string{"content": content}},
			{"to_user": userID, "msg_type": "text", "text": map[string]string{"content": content}},
			{"touser": userID, "msgtype": "text", "text": map[string]string{"content": content}},
			{"to_user_id": userID, "msg_type": "text", "content": content},
		}

		for _, u := range urlCandidates {
			for _, payload := range payloads {
				resp, err := notify.HTTPPost(u, notify.HttpRequestOptions{
					JSON:    payload,
					Headers: headers,
					Timeout: 15 * time.Second,
				})
				if err != nil {
					lastError = fmt.Sprintf("user=%s, %s", userID, err.Error())
					continue
				}
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					bodyPayload := parseJSON(resp.Body)
					if bodyPayload != nil && isOk(bodyPayload) {
						slog.Info("微信 ClawBot 通知发送成功", "toUser", toUser)
						return notify.NotifyResult{Success: true, Message: "发送成功"}
					}
					errMsg := firstNonEmpty(
						getString(bodyPayload, "errmsg"),
						getString(bodyPayload, "message"),
						getString(bodyPayload, "error"),
					)
					if errMsg == "" {
						errMsg = "API 返回异常"
					}
					lastError = fmt.Sprintf("user=%s, HTTP 200 但接口返回错误: %s", userID, errMsg)
					continue
				}
				lastError = fmt.Sprintf("user=%s, HTTP %d: %s", userID, resp.StatusCode, resp.Body)
			}
		}
	}

	slog.Warn("微信 ClawBot 所有发送方式均失败", "lastError", lastError)
	return notify.NotifyResult{Success: false, Message: "发送失败: " + lastError}
}

// -------- Helper functions --------

// normalizeQRCodeUrl normalizes QR code display fields, compatible with
// image URLs, data URLs, and raw base64 strings.
// Ported from Node.js version ILinkClient.normalizeQRCodeUrl.
func normalizeQRCodeUrl(raw string) string {
	if raw == "" {
		return ""
	}
	lowered := strings.ToLower(raw)
	// data:image/ prefix → pass through
	if strings.HasPrefix(lowered, "data:image/") {
		return raw
	}
	// // prefix → prepend https:
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	// Bare base64: length >= 128 and only base64 chars
	if len(raw) >= 128 {
		isBase64 := true
		for _, c := range raw {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' || c == '-' || c == '_') {
				isBase64 = false
				break
			}
		}
		if isBase64 {
			return "data:image/png;base64," + raw
		}
	}
	return raw
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "%2F", "/")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case fmt.Stringer:
		return s.String()
	default:
		return fmt.Sprint(v)
	}
}

func randHex(n int) string {
	const letters = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func init() {
	ch := &WechatClawBot{}
	if notify.HasConfig("WECHAT_CLAWBOT_BOT_TOKEN") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
