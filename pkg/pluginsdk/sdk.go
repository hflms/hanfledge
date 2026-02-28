// Package pluginsdk provides the public API for developing Hanfledge plugins.
//
// Plugin developers implement the Plugin interface and register their plugin
// using the Register function. The host process discovers and initializes
// plugins via the plugin registry.
//
// Example:
//
//	func init() {
//		pluginsdk.Register(&MyPlugin{})
//	}
package pluginsdk

import (
	"context"
)

// -- Plugin Interface (mirrors internal/plugin.Plugin) -----------

// PluginInfo describes a plugin's identity.
type PluginInfo struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Type       string   `json:"type"`        // "skill", "llm", "storage", "auth", "lms", "editor", "theme"
	TrustLevel string   `json:"trust_level"` // "core", "domain", "community"
	Hooks      []string `json:"hooks,omitempty"`
}

// HealthStatus reports plugin health.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// HookEvent carries data through the event bus.
type HookEvent struct {
	Hook    string                 `json:"hook"`
	Payload map[string]interface{} `json:"payload"`
}

// HookHandler handles a hook event.
type HookHandler func(ctx context.Context, event HookEvent) error

// EventBus allows plugins to subscribe to system events.
type EventBus interface {
	Subscribe(hook string, pluginID string, handler HookHandler)
}

// Deps are injected into plugins during initialization.
type Deps struct {
	EventBus EventBus
	// Future: Logger, ConfigStore, KnowledgeAccessor
}

// Plugin is the interface that all Hanfledge plugins must implement.
type Plugin interface {
	// Info returns the plugin's identity and configuration.
	Info() PluginInfo

	// Init initializes the plugin with injected dependencies.
	Init(ctx context.Context, deps Deps) error

	// Health returns the plugin's current health status.
	Health(ctx context.Context) HealthStatus

	// Shutdown gracefully stops the plugin.
	Shutdown(ctx context.Context) error
}

// -- Plugin Registration ----------------------------------------

var registeredPlugin Plugin

// Register registers a plugin with the SDK.
// This should be called from the plugin's init() function.
func Register(p Plugin) {
	registeredPlugin = p
}

// GetRegistered returns the registered plugin (used by the host).
func GetRegistered() Plugin {
	return registeredPlugin
}
