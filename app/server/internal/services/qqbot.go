package services

import (
	"github.com/sushazhi/fnos-logmanager/internal/notify/channels"
)

// StartQQBotListen starts the QQ Bot WebSocket listener for capturing openIDs.
func StartQQBotListen() (success bool, message string) {
	return channels.StartQQBotListener()
}

// StopQQBotListen stops the QQ Bot WebSocket listener.
func StopQQBotListen() {
	channels.StopQQBotListener()
}

// GetQQBotCaptured returns the captured QQ Bot openID and groupOpenID.
func GetQQBotCaptured() (openID, groupOpenID string) {
	return channels.GetCapturedOpenIds()
}

// HandleQQBotEvent processes a QQ Bot event from HTTP callback.
func HandleQQBotEvent(event map[string]interface{}) {
	channels.HandleQQBotEvent(event)
}
