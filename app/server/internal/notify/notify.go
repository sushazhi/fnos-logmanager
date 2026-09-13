package notify

import (
	"fmt"
	"log/slog"
)

// ChannelOutcome 是单个渠道的投递结果明细。
type ChannelOutcome struct {
	Name    string
	Result  NotifyResult
	Outcome Outcome
}

// SendNotifyResult 是聚合投递结果，保留逐渠道明细。
//
// 之所以不再只返回一个 bool：调用方（如 logfile.go 的残留清理提醒）依赖
// 该结果决定是否重试，而「部分渠道成功」曾被当成整体成功，导致失败渠道
// 不再重试，叠加冷却窗口后通知彻底丢失。
type SendNotifyResult struct {
	// Results 为逐渠道明细，顺序与启用渠道一致。
	Results []ChannelOutcome
	// Total 为参与投递的渠道数。
	Total int
	// Delivered 为明确送达的渠道数。
	Delivered int
	// Failed 为明确失败的渠道数。
	Failed int
	// Unknown 为结果未知的渠道数。
	Unknown int
}

// AllDelivered 表示所有渠道都明确送达。
func (r SendNotifyResult) AllDelivered() bool {
	return r.Total > 0 && r.Delivered == r.Total
}

// HasRetryable 表示存在可以安全重试的明确失败渠道。
func (r SendNotifyResult) HasRetryable() bool {
	for _, item := range r.Results {
		if item.Outcome == OutcomeFailed {
			return true
		}
	}
	return false
}

// Summary 生成形如 "2/3（1 未知）" 的可读摘要。
func (r SendNotifyResult) Summary() string {
	s := fmt.Sprintf("%d/%d", r.Delivered, r.Total)
	if r.Unknown > 0 {
		s += fmt.Sprintf("（%d 未知）", r.Unknown)
	}
	return s
}

// SendNotifyDetailed 向所有启用渠道投递，并返回逐渠道明细。
func SendNotifyDetailed(text, desp string) SendNotifyResult {
	channels := Registry.GetEnabled()
	result := SendNotifyResult{Total: len(channels)}
	if len(channels) == 0 {
		slog.Warn("no enabled notification channels")
		return result
	}

	chNames := make([]string, len(channels))
	for i, ch := range channels {
		chNames[i] = ch.Name()
	}
	slog.Info("sending notification", "channels", chNames)

	result.Results = make([]ChannelOutcome, 0, len(channels))
	for _, ch := range channels {
		res := ch.Send(text, desp)
		outcome := res.ResolveOutcome()
		// 消息可能来自第三方响应体，下发前统一脱敏
		res.Message = SanitizeForMessage(res.Message)
		result.Results = append(result.Results, ChannelOutcome{
			Name:    ch.Name(),
			Result:  res,
			Outcome: outcome,
		})
		switch outcome {
		case OutcomeDelivered:
			result.Delivered++
			slog.Info("notification sent", "channel", ch.Name())
		case OutcomeUnknown:
			result.Unknown++
			slog.Warn("notification outcome unknown", "channel", ch.Name(), "message", res.Message)
		default:
			result.Failed++
			slog.Warn("notification failed", "channel", ch.Name(), "message", res.Message)
		}
	}

	slog.Info("notification finished",
		"total", result.Total, "delivered", result.Delivered,
		"failed", result.Failed, "unknown", result.Unknown)

	return result
}

// SendNotify sends a notification through all enabled channels.
//
// 返回的 Success 仅当「没有任何明确失败的渠道」时为 true：结果未知不算失败
// （重试会造成重复消息），但也不能声称成功，因此 Unknown 时返回 false 且
// Message 中说明。需要精确决策的调用方请使用 SendNotifyDetailed。
func SendNotify(text, desp string) NotifyResult {
	detailed := SendNotifyDetailed(text, desp)

	if detailed.Total == 0 {
		return NotifyResult{
			Success: false,
			Message: "没有启用的通知渠道",
			Outcome: OutcomeFailed,
		}
	}

	if detailed.AllDelivered() {
		return NotifyResult{
			Success: true,
			Message: "成功: " + detailed.Summary(),
			Outcome: OutcomeDelivered,
		}
	}

	// 存在未知渠道时，语义上既非成功也非可安全重试的失败
	if detailed.Unknown > 0 {
		return NotifyResult{
			Success: false,
			Message: "部分未确认: " + detailed.Summary(),
			Outcome: OutcomeUnknown,
		}
	}

	return NotifyResult{
		Success: false,
		Message: "成功: " + detailed.Summary(),
		Outcome: OutcomeFailed,
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
