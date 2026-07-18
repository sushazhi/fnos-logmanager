package services

import (
	"github.com/sushazhi/fnos-logmanager/internal/notify/channels"
)

// GetClawBotQRCode gets a QR code for WeChat ClawBot login.
func GetClawBotQRCode() (success bool, qrcode, qrcodeURL, message string) {
	return channels.GetClawBotQRCode()
}

// CheckClawBotQRCodeStatus checks the QR code scan status.
func CheckClawBotQRCodeStatus(qrcode string) (success bool, status, token, accountID, scannerID, message string) {
	return channels.CheckClawBotQRCodeStatus(qrcode)
}

// GetClawBotCaptured returns the captured credentials.
func GetClawBotCaptured() (botToken, accountID, baseURL string) {
	return channels.GetCapturedCredentials()
}

// GetClawBotUpdates polls for unread messages to capture scanner user ID.
func GetClawBotUpdates() (success bool, messages []map[string]string, message string) {
	return channels.GetClawBotUpdates()
}

// SetClawBotLastInteractedUser sets the last interacted user for WeChat ClawBot.
func SetClawBotLastInteractedUser(userID string) {
	channels.SetLastInteractedUser(userID)
}
