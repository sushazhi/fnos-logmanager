package notify

import (
	"regexp"
	"strings"
)

// 本文件提供跨渠道共用的文本处理 helper，供各渠道实现复用，
// 避免每个渠道各自实现一份（历史上 telegram/dingtalk 用字节截断，
// qmsg 用字符截断，行为不一致）。

// TruncateRunes 按字符（而非字节）截断字符串。
//
// 按字节截断会切碎多字节的 UTF-8 中文，产生乱码；部分平台还会因此
// 拒绝整条消息。所有面向用户的文本截断都应使用本函数。
func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// 形如 key=value、key: value 或 JSON 的 "key": "value" 赋值片段。
// 键名两侧允许出现引号与空白，值允许被引号包裹（JSON 响应体最常见）。
var secretAssignment = regexp.MustCompile(
	`(?i)("?[a-z0-9_]*(?:token|secret|password|passwd|key|sign|signature|credential)[a-z0-9_]*"?\s*[=:]\s*)("?[^"\s,;&}\]]*"?)`,
)

// 明显的裸凭据形态（长十六进制/base64 串），单独出现时也打码。
var bareCredential = regexp.MustCompile(`\b[A-Za-z0-9_\-]{24,}\b`)

// 已知的敏感字段名，出现时整体替换，避免值以任何形式泄漏。
var sensitiveWords = []string{
	"access_token", "refresh_token", "app_secret", "client_secret",
	"client_secret", "secret", "password", "passwd", "private_key",
	"api_key", "apikey", "bot_token", "appsecret",
}

// SanitizeForMessage 对将要写入 NotifyResult.Message 的文本做脱敏。
//
// 该文本会经 API 与通知历史回到前端，而第三方响应体经常原样包含
// 凭据（例如 token=xxx、client_secret=yyy），因此必须在下发前打码。
func SanitizeForMessage(s string) string {
	if s == "" {
		return ""
	}

	// 1) key=value / key: value 形式：保留键名，值打码
	out := secretAssignment.ReplaceAllString(s, "${1}••••••")

	// 2) 出现敏感字段名但没有可识别的赋值结构时，整体替换为占位符，
	//    避免把值以其它形式带出。
	lower := strings.ToLower(out)
	for _, word := range sensitiveWords {
		if idx := strings.Index(lower, word); idx >= 0 {
			// 保留字段名本身，抹掉紧随其后的内容片段。
			out = out[:idx] + word + "=••••••" + sanitizeTail(out[idx+len(word):])
			lower = strings.ToLower(out)
		}
	}

	// 3) 兜底：把独立出现的超长凭据样式串打码
	out = bareCredential.ReplaceAllStringFunc(out, func(match string) string {
		// 保留纯数字串（时间戳、ID）以免影响排错可读性
		if isAllDigits(match) {
			return match
		}
		return "••••••"
	})

	return out
}

// sanitizeTail 处理敏感字段名之后的残余文本，只保留结构性字符。
func sanitizeTail(tail string) string {
	// 截到下一个明显的分隔符，避免把后续正常内容一并吞掉
	for i, r := range tail {
		if r == ' ' || r == ',' || r == ';' || r == '，' || r == '。' || r == '\n' || r == '"' || r == '\'' {
			return tail[i:]
		}
	}
	return ""
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
