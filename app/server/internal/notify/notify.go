package notify

import (
	"fmt"
	"log/slog"
)

// SendNotify sends a notification through all enabled channels.
// Returns overall success/failure and a summary message.
func SendNotify(text, desp string) NotifyResult {
	channels := Registry.GetEnabled()
	if len(channels) == 0 {
		slog.Warn("no enabled notification channels")
		return NotifyResult{Success: false, Message: "没有启用的通知渠道"}
	}

	chNames := make([]string, len(channels))
	for i, ch := range channels {
		chNames[i] = ch.Name()
	}
	slog.Info("sending notification", "channels", chNames)

	type channelResult struct {
		name   string
		result NotifyResult
	}

	results := make([]channelResult, 0, len(channels))
	for _, ch := range channels {
		result := ch.Send(text, desp)
		results = append(results, channelResult{name: ch.Name(), result: result})
		if result.Success {
			slog.Info("notification sent", "channel", ch.Name())
		} else {
			slog.Warn("notification failed", "channel", ch.Name(), "message", result.Message)
		}
	}

	successCount := 0
	for _, r := range results {
		if r.result.Success {
			successCount++
		}
	}

	slog.Info("notification finished", "total", len(channels), "success", successCount)

	return NotifyResult{
		Success: successCount > 0,
		Message: "成功: " + formatCount(successCount, len(channels)),
	}
}

func formatCount(success, total int) string {
	return fmt.Sprintf("%d/%d", success, total)
}

// GetRegisteredChannels returns all registered channel names.
func GetRegisteredChannels() []string {
	all := Registry.GetAll()
	names := make([]string, len(all))
	for i, ch := range all {
		names[i] = ch.Name()
	}
	return names
}

// GetEnabledChannels returns all enabled channel names.
func GetEnabledChannels() []string {
	enabled := Registry.GetEnabled()
	names := make([]string, len(enabled))
	for i, ch := range enabled {
		names[i] = ch.Name()
	}
	return names
}

// EnableChannel enables a channel by name.
func EnableChannel(name string) {
	Registry.Enable(name)
}

// DisableChannel disables a channel by name.
func DisableChannel(name string) {
	Registry.Disable(name)
}
