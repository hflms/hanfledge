package plugin

import (
	"context"
	"sync"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogEventBus = logger.L("EventBus")

// ============================
// EventBus -- Plugin Event Bus
// ============================
//
// Responsibilities: Decouple Agent-to-Agent and Plugin-to-Plugin communication.
// Plugins subscribe to HookPoint events via PluginDeps.EventBus.
// The core system Publishes events at critical pipeline stages.
//
// Reference: design.md section 7.5

// -- Hook Point Constants ----------------------------------------

// HookPoint defines system hook points that plugins can subscribe to.
type HookPoint string

const (
	// Knowledge Engineering Hooks
	HookBeforeSlicing   HookPoint = "knowledge.before_slicing"
	HookAfterSlicing    HookPoint = "knowledge.after_slicing"
	HookBeforeEmbedding HookPoint = "knowledge.before_embedding"
	HookAfterGraphBuild HookPoint = "knowledge.after_graph_build"

	// Student Interaction Hooks
	HookBeforeStudentQuery HookPoint = "interaction.before_query"
	HookAfterSkillMatch    HookPoint = "interaction.after_skill_match"
	HookBeforeLLMCall      HookPoint = "interaction.before_llm_call"
	HookAfterLLMResponse   HookPoint = "interaction.after_llm_response"
	HookAfterStudentAnswer HookPoint = "interaction.after_student_answer"

	// Assessment Hooks
	HookAfterEvaluation HookPoint = "assessment.after_evaluation"
	HookOnMasteryChange HookPoint = "assessment.on_mastery_change"

	// System Hooks
	HookOnUserLogin       HookPoint = "system.on_user_login"
	HookOnActivityPublish HookPoint = "system.on_activity_publish"
)

// -- Event Types ------------------------------------------------

// HookEvent carries data through the event bus.
type HookEvent struct {
	Hook    HookPoint              `json:"hook"`
	Payload map[string]interface{} `json:"payload"`
}

// HookHandler is a function that handles a hook event.
// Returning a non-nil error signals the handler encountered a problem.
// For "before" hooks, returning an error may abort the operation.
type HookHandler func(ctx context.Context, event HookEvent) error

// -- EventBus Implementation ------------------------------------

// EventBus manages publish/subscribe for system hook events.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[HookPoint][]subscriberEntry
}

type subscriberEntry struct {
	pluginID string
	handler  HookHandler
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[HookPoint][]subscriberEntry),
	}
}

// Subscribe registers a handler for a specific hook point.
// pluginID identifies which plugin is subscribing (for logging/debugging).
func (eb *EventBus) Subscribe(hook HookPoint, pluginID string, handler HookHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[hook] = append(eb.subscribers[hook], subscriberEntry{
		pluginID: pluginID,
		handler:  handler,
	})

	slogEventBus.Debug("subscribed", "plugin", pluginID, "hook", hook)
}

// Unsubscribe removes all handlers registered by a specific plugin.
func (eb *EventBus) Unsubscribe(pluginID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for hook, subs := range eb.subscribers {
		filtered := make([]subscriberEntry, 0, len(subs))
		for _, s := range subs {
			if s.pluginID != pluginID {
				filtered = append(filtered, s)
			}
		}
		eb.subscribers[hook] = filtered
	}

	slogEventBus.Debug("unsubscribed from all hooks", "plugin", pluginID)
}

// Publish fires an event to all subscribers of the given hook point.
// Handlers are called synchronously in subscription order.
// Errors from individual handlers are logged but do not prevent other handlers from running.
// Returns the first error encountered (if any).
func (eb *EventBus) Publish(ctx context.Context, event HookEvent) error {
	eb.mu.RLock()
	subs := eb.subscribers[event.Hook]
	subsCopy := make([]subscriberEntry, len(subs))
	copy(subsCopy, subs)
	eb.mu.RUnlock()

	if len(subsCopy) == 0 {
		return nil
	}

	var firstErr error
	for _, sub := range subsCopy {
		if err := sub.handler(ctx, event); err != nil {
			slogEventBus.Warn("handler failed",
				"plugin", sub.pluginID, "hook", event.Hook, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// PublishAbortable fires a "before" event where any handler error aborts the chain.
// Returns the error from the first failing handler (subsequent handlers are skipped).
func (eb *EventBus) PublishAbortable(ctx context.Context, event HookEvent) error {
	eb.mu.RLock()
	subs := eb.subscribers[event.Hook]
	subsCopy := make([]subscriberEntry, len(subs))
	copy(subsCopy, subs)
	eb.mu.RUnlock()

	for _, sub := range subsCopy {
		if err := sub.handler(ctx, event); err != nil {
			slogEventBus.Warn("aborted by handler",
				"plugin", sub.pluginID, "hook", event.Hook, "err", err)
			return err
		}
	}

	return nil
}

// SubscriberCount returns the number of subscribers for a given hook.
func (eb *EventBus) SubscriberCount(hook HookPoint) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.subscribers[hook])
}

// -- Nil-safe Helpers -------------------------------------------

// PublishEvent fires an event if the EventBus is non-nil (no-op otherwise).
// Use this instead of duplicating nil-guard boilerplate in every caller.
func PublishEvent(eb *EventBus, ctx context.Context, hook HookPoint, payload map[string]interface{}) {
	if eb == nil {
		return
	}
	eb.Publish(ctx, HookEvent{Hook: hook, Payload: payload})
}

// PublishAbortableEvent fires an abortable event if the EventBus is non-nil.
// Returns nil when the bus is nil. Returns error if any handler aborts.
func PublishAbortableEvent(eb *EventBus, ctx context.Context, hook HookPoint, payload map[string]interface{}) error {
	if eb == nil {
		return nil
	}
	return eb.PublishAbortable(ctx, HookEvent{Hook: hook, Payload: payload})
}
