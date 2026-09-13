package services

import (
	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

// StartDingTalkRegistration 发起钉钉应用扫码注册，返回二维码信息。
func StartDingTalkRegistration() (success bool, verificationURL, deviceCode, userCode string, pollIntervalMs int, message string) {
	reg, err := notify.StartDingTalkRegistration()
	if err != nil {
		return false, "", "", "", 0, err.Error()
	}
	return true, reg.VerificationURL, reg.DeviceCode, reg.UserCode, reg.PollIntervalMs, ""
}

// PollDingTalkRegistration 轮询钉钉扫码注册状态。
// 成功时返回 clientID/clientSecret，调用方据此填入 DD_BOT_TOKEN / DD_BOT_SECRET。
func PollDingTalkRegistration(deviceCode string) (success bool, status, clientID, clientSecret, message string) {
	result, err := notify.PollDingTalkRegistration(deviceCode)
	if err != nil {
		return false, "failed", "", "", err.Error()
	}
	msg := result.FailReason
	if msg == "" && result.Status == "EXPIRED" {
		msg = "二维码已过期，请重新获取"
	}
	return true, result.Status, result.ClientID, result.ClientSecret, msg
}

// CancelDingTalkRegistration 取消一次钉钉扫码注册会话。
func CancelDingTalkRegistration(deviceCode string) {
	notify.CancelDingTalkRegistration(deviceCode)
}
