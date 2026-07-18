package notify

import (
	"log/slog"
	"sync"
)

// ChannelRegistry manages registration and lookup of notification channels.
type ChannelRegistry struct {
	mu       sync.RWMutex
	channels map[string]NotifyChannel
}

// NewRegistry creates a new ChannelRegistry.
func NewRegistry() *ChannelRegistry {
	return &ChannelRegistry{
		channels: make(map[string]NotifyChannel),
	}
}

// Register adds a channel to the registry.
func (r *ChannelRegistry) Register(ch NotifyChannel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[ch.Name()] = ch
	slog.Debug("channel registered", "channel", ch.Name(), "enabled", ch.Enabled())
}

// Unregister removes a channel from the registry.
func (r *ChannelRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.channels, name)
	slog.Debug("channel unregistered", "channel", name)
}

// Get returns a channel by name.
func (r *ChannelRegistry) Get(name string) NotifyChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channels[name]
}

// GetAll returns all registered channels.
func (r *ChannelRegistry) GetAll() []NotifyChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]NotifyChannel, 0, len(r.channels))
	for _, ch := range r.channels {
		result = append(result, ch)
	}
	return result
}

// GetEnabled returns all enabled channels.
func (r *ChannelRegistry) GetEnabled() []NotifyChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []NotifyChannel
	for _, ch := range r.channels {
		if ch.Enabled() {
			result = append(result, ch)
		}
	}
	return result
}

// Enable enables a channel by name.
func (r *ChannelRegistry) Enable(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch, ok := r.channels[name]; ok {
		ch.SetEnabled(true)
		slog.Debug("channel enabled", "channel", name)
	}
}

// Disable disables a channel by name.
func (r *ChannelRegistry) Disable(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch, ok := r.channels[name]; ok {
		ch.SetEnabled(false)
		slog.Debug("channel disabled", "channel", name)
	}
}

// Has returns whether a channel exists.
func (r *ChannelRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.channels[name]
	return ok
}

// Size returns the number of registered channels.
func (r *ChannelRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.channels)
}

// Global registry singleton.
var Registry = NewRegistry()
