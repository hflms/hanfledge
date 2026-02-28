package plugin

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// ── Subscribe & Publish Tests ───────────────────────────────

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	eb := NewEventBus()
	called := false

	eb.Subscribe(HookBeforeSlicing, "test-plugin", func(ctx context.Context, event HookEvent) error {
		called = true
		return nil
	})

	err := eb.Publish(context.Background(), HookEvent{
		Hook:    HookBeforeSlicing,
		Payload: map[string]interface{}{"key": "value"},
	})

	if err != nil {
		t.Errorf("Publish() returned error: %v", err)
	}
	if !called {
		t.Errorf("Handler was not called")
	}
}

func TestEventBus_PublishNoSubscribers(t *testing.T) {
	eb := NewEventBus()

	err := eb.Publish(context.Background(), HookEvent{
		Hook: HookAfterSlicing,
	})

	if err != nil {
		t.Errorf("Publish() with no subscribers should return nil, got: %v", err)
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	eb := NewEventBus()
	callOrder := []string{}

	eb.Subscribe(HookAfterSlicing, "plugin-a", func(ctx context.Context, event HookEvent) error {
		callOrder = append(callOrder, "a")
		return nil
	})
	eb.Subscribe(HookAfterSlicing, "plugin-b", func(ctx context.Context, event HookEvent) error {
		callOrder = append(callOrder, "b")
		return nil
	})

	eb.Publish(context.Background(), HookEvent{Hook: HookAfterSlicing})

	if len(callOrder) != 2 {
		t.Fatalf("Expected 2 handler calls, got %d", len(callOrder))
	}
	if callOrder[0] != "a" || callOrder[1] != "b" {
		t.Errorf("Handler call order = %v, want [a, b]", callOrder)
	}
}

func TestEventBus_PublishReturnsFirstError(t *testing.T) {
	eb := NewEventBus()
	callCount := 0

	eb.Subscribe(HookBeforeLLMCall, "plugin-a", func(ctx context.Context, event HookEvent) error {
		callCount++
		return fmt.Errorf("error from a")
	})
	eb.Subscribe(HookBeforeLLMCall, "plugin-b", func(ctx context.Context, event HookEvent) error {
		callCount++
		return nil
	})

	err := eb.Publish(context.Background(), HookEvent{Hook: HookBeforeLLMCall})

	if err == nil {
		t.Errorf("Publish() should return first error")
	}
	// Publish 不会中断链 — 所有 handler 都应被调用
	if callCount != 2 {
		t.Errorf("All handlers should be called even with errors, got %d calls", callCount)
	}
}

// ── PublishAbortable Tests ──────────────────────────────────

func TestEventBus_PublishAbortable_Success(t *testing.T) {
	eb := NewEventBus()
	callCount := 0

	eb.Subscribe(HookBeforeStudentQuery, "plugin-a", func(ctx context.Context, event HookEvent) error {
		callCount++
		return nil
	})
	eb.Subscribe(HookBeforeStudentQuery, "plugin-b", func(ctx context.Context, event HookEvent) error {
		callCount++
		return nil
	})

	err := eb.PublishAbortable(context.Background(), HookEvent{Hook: HookBeforeStudentQuery})

	if err != nil {
		t.Errorf("PublishAbortable() should return nil, got: %v", err)
	}
	if callCount != 2 {
		t.Errorf("All handlers should be called, got %d", callCount)
	}
}

func TestEventBus_PublishAbortable_Aborts(t *testing.T) {
	eb := NewEventBus()
	callCount := 0

	eb.Subscribe(HookBeforeEmbedding, "plugin-a", func(ctx context.Context, event HookEvent) error {
		callCount++
		return fmt.Errorf("abort!")
	})
	eb.Subscribe(HookBeforeEmbedding, "plugin-b", func(ctx context.Context, event HookEvent) error {
		callCount++
		return nil
	})

	err := eb.PublishAbortable(context.Background(), HookEvent{Hook: HookBeforeEmbedding})

	if err == nil {
		t.Errorf("PublishAbortable() should return error")
	}
	// 第一个 handler 返回错误后应中断
	if callCount != 1 {
		t.Errorf("Only first handler should run before abort, got %d calls", callCount)
	}
}

// ── Unsubscribe Tests ───────────────────────────────────────

func TestEventBus_Unsubscribe(t *testing.T) {
	eb := NewEventBus()

	eb.Subscribe(HookAfterLLMResponse, "plugin-to-remove", func(ctx context.Context, event HookEvent) error {
		t.Errorf("Handler should not be called after unsubscribe")
		return nil
	})
	eb.Subscribe(HookAfterLLMResponse, "plugin-to-keep", func(ctx context.Context, event HookEvent) error {
		return nil
	})

	eb.Unsubscribe("plugin-to-remove")

	eb.Publish(context.Background(), HookEvent{Hook: HookAfterLLMResponse})

	if eb.SubscriberCount(HookAfterLLMResponse) != 1 {
		t.Errorf("SubscriberCount = %d, want 1 after unsubscribe",
			eb.SubscriberCount(HookAfterLLMResponse))
	}
}

func TestEventBus_UnsubscribeRemovesFromAllHooks(t *testing.T) {
	eb := NewEventBus()

	handler := func(ctx context.Context, event HookEvent) error { return nil }
	eb.Subscribe(HookBeforeSlicing, "multi-hook", handler)
	eb.Subscribe(HookAfterSlicing, "multi-hook", handler)
	eb.Subscribe(HookAfterGraphBuild, "multi-hook", handler)

	eb.Unsubscribe("multi-hook")

	if eb.SubscriberCount(HookBeforeSlicing) != 0 {
		t.Errorf("HookBeforeSlicing should have 0 subscribers, got %d",
			eb.SubscriberCount(HookBeforeSlicing))
	}
	if eb.SubscriberCount(HookAfterSlicing) != 0 {
		t.Errorf("HookAfterSlicing should have 0 subscribers, got %d",
			eb.SubscriberCount(HookAfterSlicing))
	}
	if eb.SubscriberCount(HookAfterGraphBuild) != 0 {
		t.Errorf("HookAfterGraphBuild should have 0 subscribers, got %d",
			eb.SubscriberCount(HookAfterGraphBuild))
	}
}

// ── SubscriberCount Tests ───────────────────────────────────

func TestEventBus_SubscriberCount(t *testing.T) {
	eb := NewEventBus()

	if eb.SubscriberCount(HookOnUserLogin) != 0 {
		t.Errorf("New EventBus should have 0 subscribers")
	}

	eb.Subscribe(HookOnUserLogin, "plugin-a", func(ctx context.Context, event HookEvent) error { return nil })
	eb.Subscribe(HookOnUserLogin, "plugin-b", func(ctx context.Context, event HookEvent) error { return nil })

	if eb.SubscriberCount(HookOnUserLogin) != 2 {
		t.Errorf("SubscriberCount = %d, want 2", eb.SubscriberCount(HookOnUserLogin))
	}
}

// ── Concurrency Safety Tests ────────────────────────────────

func TestEventBus_ConcurrentPublish(t *testing.T) {
	eb := NewEventBus()
	var mu sync.Mutex
	callCount := 0

	eb.Subscribe(HookOnMasteryChange, "concurrent-plugin", func(ctx context.Context, event HookEvent) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			eb.Publish(context.Background(), HookEvent{Hook: HookOnMasteryChange})
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if callCount != 100 {
		t.Errorf("Concurrent publish: callCount = %d, want 100", callCount)
	}
}

func TestEventBus_PayloadPassthrough(t *testing.T) {
	eb := NewEventBus()
	var receivedPayload map[string]interface{}

	eb.Subscribe(HookAfterEvaluation, "payload-plugin", func(ctx context.Context, event HookEvent) error {
		receivedPayload = event.Payload
		return nil
	})

	payload := map[string]interface{}{
		"student_id": float64(42),
		"mastery":    0.75,
	}
	eb.Publish(context.Background(), HookEvent{
		Hook:    HookAfterEvaluation,
		Payload: payload,
	})

	if receivedPayload == nil {
		t.Fatalf("Handler did not receive payload")
	}
	if receivedPayload["student_id"] != float64(42) {
		t.Errorf("student_id = %v, want 42", receivedPayload["student_id"])
	}
}
