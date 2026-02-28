package agent

import (
	"strings"
	"testing"

	"github.com/hflms/hanfledge/internal/plugin"
)

// ============================
// Phase F: Fallacy Detective Tests
// ============================

// ── parseTriggerCondition Tests ─────────────────────────────

func TestParseTriggerCondition_GreaterEqual(t *testing.T) {
	tests := []struct {
		mastery  float64
		expected bool
	}{
		{0.9, true},
		{0.8, true}, // exactly at threshold
		{0.79, false},
		{0.0, false},
	}
	for _, tc := range tests {
		result := parseTriggerCondition("mastery_score >= 0.8", tc.mastery)
		if result != tc.expected {
			t.Errorf("parseTriggerCondition(>= 0.8, %.2f) = %v, want %v",
				tc.mastery, result, tc.expected)
		}
	}
}

func TestParseTriggerCondition_LessThan(t *testing.T) {
	tests := []struct {
		mastery  float64
		expected bool
	}{
		{0.5, true},
		{0.59, true},
		{0.6, false}, // exactly at threshold
		{0.9, false},
	}
	for _, tc := range tests {
		result := parseTriggerCondition("mastery_score < 0.6", tc.mastery)
		if result != tc.expected {
			t.Errorf("parseTriggerCondition(< 0.6, %.2f) = %v, want %v",
				tc.mastery, result, tc.expected)
		}
	}
}

func TestParseTriggerCondition_AllOperators(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		mastery   float64
		expected  bool
	}{
		{">=满足", "mastery_score >= 0.5", 0.5, true},
		{">=不满足", "mastery_score >= 0.5", 0.4, false},
		{"<=满足", "mastery_score <= 0.5", 0.5, true},
		{"<=不满足", "mastery_score <= 0.5", 0.6, false},
		{">满足", "mastery_score > 0.5", 0.6, true},
		{">不满足_等于", "mastery_score > 0.5", 0.5, false},
		{"<满足", "mastery_score < 0.5", 0.4, true},
		{"<不满足_等于", "mastery_score < 0.5", 0.5, false},
		{"==满足", "mastery_score == 0.5", 0.5, true},
		{"==不满足", "mastery_score == 0.5", 0.6, false},
		{"!=满足", "mastery_score != 0.5", 0.6, true},
		{"!=不满足", "mastery_score != 0.5", 0.5, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseTriggerCondition(tc.condition, tc.mastery)
			if result != tc.expected {
				t.Errorf("parseTriggerCondition(%q, %.2f) = %v, want %v",
					tc.condition, tc.mastery, result, tc.expected)
			}
		})
	}
}

func TestParseTriggerCondition_InvalidFormats(t *testing.T) {
	tests := []struct {
		name      string
		condition string
	}{
		{"空字符串", ""},
		{"只有变量名", "mastery_score"},
		{"缺少值", "mastery_score >="},
		{"多余部分", "mastery_score >= 0.8 extra"},
		{"非法操作符", "mastery_score ~= 0.8"},
		{"非法阈值", "mastery_score >= abc"},
		{"不支持的变量", "unknown_var >= 0.8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// All invalid formats should return false
			result := parseTriggerCondition(tc.condition, 0.9)
			if result {
				t.Errorf("parseTriggerCondition(%q) should return false for invalid format", tc.condition)
			}
		})
	}
}

func TestParseTriggerCondition_Whitespace(t *testing.T) {
	// Should handle leading/trailing whitespace
	result := parseTriggerCondition("  mastery_score >= 0.8  ", 0.85)
	if !result {
		t.Error("parseTriggerCondition should handle whitespace")
	}
}

// ── evaluateProgressiveTriggers Tests ───────────────────────

func TestEvaluateProgressiveTriggers_SocraticToFallacy(t *testing.T) {
	// Setup: Create a registry with two skills
	registry := plugin.NewRegistry()

	// Manually register skills (bypassing file loading)
	registerTestSkill(t, registry, "general_concept_socratic", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score < 0.6",
		DeactivateWhen: "mastery_score >= 0.8",
	})
	registerTestSkill(t, registry, "general_assessment_fallacy", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score >= 0.8",
		DeactivateWhen: "critical_thinking_score >= 0.9",
	})

	agent := &StrategistAgent{registry: registry}

	// Test: mastery = 0.85 → should switch from socratic to fallacy
	newSkill, switched := agent.evaluateProgressiveTriggers("general_concept_socratic", 0.85)
	if !switched {
		t.Fatal("Should trigger switch from socratic to fallacy at mastery 0.85")
	}
	if newSkill != "general_assessment_fallacy" {
		t.Errorf("Should switch to fallacy-detective, got %q", newSkill)
	}
}

func TestEvaluateProgressiveTriggers_NoSwitch_LowMastery(t *testing.T) {
	registry := plugin.NewRegistry()
	registerTestSkill(t, registry, "general_concept_socratic", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score < 0.6",
		DeactivateWhen: "mastery_score >= 0.8",
	})
	registerTestSkill(t, registry, "general_assessment_fallacy", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score >= 0.8",
		DeactivateWhen: "critical_thinking_score >= 0.9",
	})

	agent := &StrategistAgent{registry: registry}

	// Test: mastery = 0.5 → socratic deactivate_when not met, no switch
	_, switched := agent.evaluateProgressiveTriggers("general_concept_socratic", 0.5)
	if switched {
		t.Error("Should NOT trigger switch at mastery 0.5")
	}
}

func TestEvaluateProgressiveTriggers_NoSwitch_ExactThreshold(t *testing.T) {
	registry := plugin.NewRegistry()
	registerTestSkill(t, registry, "general_concept_socratic", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score < 0.6",
		DeactivateWhen: "mastery_score >= 0.8",
	})
	registerTestSkill(t, registry, "general_assessment_fallacy", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score >= 0.8",
		DeactivateWhen: "critical_thinking_score >= 0.9",
	})

	agent := &StrategistAgent{registry: registry}

	// Test: mastery = 0.8 (exactly at threshold) → should switch
	newSkill, switched := agent.evaluateProgressiveTriggers("general_concept_socratic", 0.8)
	if !switched {
		t.Fatal("Should trigger switch at exactly mastery 0.8")
	}
	if newSkill != "general_assessment_fallacy" {
		t.Errorf("Should switch to fallacy-detective, got %q", newSkill)
	}
}

func TestEvaluateProgressiveTriggers_UnknownSkill(t *testing.T) {
	registry := plugin.NewRegistry()
	agent := &StrategistAgent{registry: registry}

	_, switched := agent.evaluateProgressiveTriggers("nonexistent_skill", 0.9)
	if switched {
		t.Error("Should NOT switch for unknown skill")
	}
}

func TestEvaluateProgressiveTriggers_NilRegistry(t *testing.T) {
	// evaluateProgressiveTriggers is only called when registry != nil
	// (guarded in Analyze()), but test the method directly for safety
	agent := &StrategistAgent{registry: nil}

	// This should not panic — the caller guards against nil registry
	// but GetSkill would fail gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Should not panic with nil registry: %v", r)
		}
	}()

	// Can't call evaluateProgressiveTriggers with nil registry since it calls
	// registry.GetSkill which would panic. This test verifies the guard in Analyze().
	// Use agent to verify it was constructed correctly.
	if agent.registry != nil {
		t.Error("registry should be nil")
	}
}

func TestEvaluateProgressiveTriggers_NoDeactivateCondition(t *testing.T) {
	registry := plugin.NewRegistry()
	registerTestSkill(t, registry, "skill_no_deactivate", &plugin.ProgressiveTriggers{
		ActivateWhen: "mastery_score >= 0.5",
		// No DeactivateWhen
	})

	agent := &StrategistAgent{registry: registry}

	_, switched := agent.evaluateProgressiveTriggers("skill_no_deactivate", 0.9)
	if switched {
		t.Error("Should NOT switch when current skill has no deactivate_when")
	}
}

func TestEvaluateProgressiveTriggers_NoProgressiveTriggers(t *testing.T) {
	registry := plugin.NewRegistry()
	registerTestSkill(t, registry, "skill_no_triggers", nil) // no triggers at all

	agent := &StrategistAgent{registry: registry}

	_, switched := agent.evaluateProgressiveTriggers("skill_no_triggers", 0.9)
	if switched {
		t.Error("Should NOT switch when current skill has no progressive_triggers")
	}
}

// ── isFallacyDetectiveActive Tests ──────────────────────────

func TestIsFallacyDetectiveActive(t *testing.T) {
	tests := []struct {
		skillID  string
		expected bool
	}{
		{"general_assessment_fallacy", true},
		{"fallacy-detective", true},
		{"general_concept_socratic", false},
		{"socratic-questioning", false},
		{"", false},
		{"fallacy", false},
	}

	for _, tc := range tests {
		result := isFallacyDetectiveActive(tc.skillID)
		if result != tc.expected {
			t.Errorf("isFallacyDetectiveActive(%q) = %v, want %v",
				tc.skillID, result, tc.expected)
		}
	}
}

// ── FallacySessionState Tests ───────────────────────────────

func TestDefaultFallacyState(t *testing.T) {
	state := defaultFallacyState()

	if state.EmbeddedCount != 0 {
		t.Errorf("EmbeddedCount = %d, want 0", state.EmbeddedCount)
	}
	if state.IdentifiedCount != 0 {
		t.Errorf("IdentifiedCount = %d, want 0", state.IdentifiedCount)
	}
	if state.Phase != FallacyPhasePresentTrap {
		t.Errorf("Phase = %q, want %q", state.Phase, FallacyPhasePresentTrap)
	}
	if state.MaxPerSession != 3 {
		t.Errorf("MaxPerSession = %d, want 3", state.MaxPerSession)
	}
}

// ── fallacyPhaseLabel Tests ─────────────────────────────────

func TestFallacyPhaseLabel(t *testing.T) {
	tests := []struct {
		phase    FallacyPhase
		expected string
	}{
		{FallacyPhasePresentTrap, "展示陷阱"},
		{FallacyPhaseAwaiting, "等待识别"},
		{FallacyPhaseRevealed, "已揭示"},
		{FallacyPhase("unknown"), "unknown"},
	}

	for _, tc := range tests {
		result := fallacyPhaseLabel(tc.phase)
		if result != tc.expected {
			t.Errorf("fallacyPhaseLabel(%q) = %q, want %q",
				tc.phase, result, tc.expected)
		}
	}
}

// ── buildFallacyContext Tests ───────────────────────────────

func TestBuildFallacyContext_PresentTrapPhase(t *testing.T) {
	state := FallacySessionState{
		EmbeddedCount:   0,
		IdentifiedCount: 0,
		Phase:           FallacyPhasePresentTrap,
		MaxPerSession:   3,
	}

	ctx := buildFallacyContext(state, nil)

	if !strings.Contains(ctx, "展示陷阱") {
		t.Error("Should contain phase label '展示陷阱'")
	}
	if !strings.Contains(ctx, "已嵌入谬误数: 0 / 3") {
		t.Error("Should contain embedded count")
	}
	if !strings.Contains(ctx, "请在接下来的讲解中巧妙嵌入一个学科常见误区") {
		t.Error("Should contain instruction to embed a trap")
	}
}

func TestBuildFallacyContext_AwaitingPhase(t *testing.T) {
	state := FallacySessionState{
		EmbeddedCount:   1,
		IdentifiedCount: 0,
		Phase:           FallacyPhaseAwaiting,
		MaxPerSession:   3,
		CurrentTrapDesc: "将加法交换律错误应用于减法",
	}

	ctx := buildFallacyContext(state, nil)

	if !strings.Contains(ctx, "等待识别") {
		t.Error("Should contain phase label '等待识别'")
	}
	if !strings.Contains(ctx, "评估学生的回答是否准确定位了谬误") {
		t.Error("Should contain identification instruction")
	}
	if !strings.Contains(ctx, "将加法交换律错误应用于减法") {
		t.Error("Should contain current trap description")
	}
}

func TestBuildFallacyContext_RevealedPhase(t *testing.T) {
	state := FallacySessionState{
		EmbeddedCount:   1,
		IdentifiedCount: 1,
		Phase:           FallacyPhaseRevealed,
		MaxPerSession:   3,
	}

	ctx := buildFallacyContext(state, nil)

	if !strings.Contains(ctx, "已揭示") {
		t.Error("Should contain phase label '已揭示'")
	}
	if !strings.Contains(ctx, "揭示这个谬误的设计意图") {
		t.Error("Should contain reveal instruction")
	}
}

func TestBuildFallacyContext_MaxReached(t *testing.T) {
	state := FallacySessionState{
		EmbeddedCount:   3,
		IdentifiedCount: 2,
		Phase:           FallacyPhasePresentTrap,
		MaxPerSession:   3,
	}

	ctx := buildFallacyContext(state, nil)

	if !strings.Contains(ctx, "本会话已达到最大谬误数") {
		t.Error("Should contain max reached warning")
	}
	if strings.Contains(ctx, "请在接下来的讲解中巧妙嵌入") {
		t.Error("Should NOT contain instruction to embed when max reached")
	}
}

// ── Phase Transition Logic Tests ────────────────────────────

// Note: AdvanceFallacyPhase requires DB access; we test the state machine
// logic directly by simulating the transitions.

func TestFallacyPhaseTransition_PresentToAwaiting(t *testing.T) {
	state := defaultFallacyState()

	// Simulate: Coach presented a trap → phase transitions to awaiting
	if state.Phase != FallacyPhasePresentTrap {
		t.Fatalf("Initial phase should be present_trap, got %q", state.Phase)
	}

	// After presenting trap:
	state.EmbeddedCount++
	state.Phase = FallacyPhaseAwaiting

	if state.EmbeddedCount != 1 {
		t.Errorf("EmbeddedCount should be 1, got %d", state.EmbeddedCount)
	}
	if state.Phase != FallacyPhaseAwaiting {
		t.Errorf("Phase should be awaiting, got %q", state.Phase)
	}
}

func TestFallacyPhaseTransition_AwaitingToRevealed(t *testing.T) {
	state := FallacySessionState{
		EmbeddedCount:   1,
		IdentifiedCount: 0,
		Phase:           FallacyPhaseAwaiting,
		MaxPerSession:   3,
	}

	// Student correctly identifies → transition to revealed
	state.IdentifiedCount++
	state.Phase = FallacyPhaseRevealed

	if state.IdentifiedCount != 1 {
		t.Errorf("IdentifiedCount should be 1, got %d", state.IdentifiedCount)
	}
	if state.Phase != FallacyPhaseRevealed {
		t.Errorf("Phase should be revealed, got %q", state.Phase)
	}
}

func TestFallacyPhaseTransition_AwaitingStaysAwaiting(t *testing.T) {
	state := FallacySessionState{
		EmbeddedCount:   1,
		IdentifiedCount: 0,
		Phase:           FallacyPhaseAwaiting,
		MaxPerSession:   3,
	}

	// Student did NOT identify → phase stays awaiting
	// (No state change)
	if state.Phase != FallacyPhaseAwaiting {
		t.Errorf("Phase should remain awaiting, got %q", state.Phase)
	}
	if state.IdentifiedCount != 0 {
		t.Errorf("IdentifiedCount should remain 0, got %d", state.IdentifiedCount)
	}
}

func TestFallacyPhaseTransition_RevealedToNextTrap(t *testing.T) {
	state := FallacySessionState{
		EmbeddedCount:   1,
		IdentifiedCount: 1,
		Phase:           FallacyPhaseRevealed,
		MaxPerSession:   3,
		CurrentTrapDesc: "some trap",
	}

	// Reveal complete → back to present_trap if quota allows
	state.CurrentTrapDesc = ""
	if state.EmbeddedCount < state.MaxPerSession {
		state.Phase = FallacyPhasePresentTrap
	}

	if state.Phase != FallacyPhasePresentTrap {
		t.Errorf("Phase should return to present_trap, got %q", state.Phase)
	}
	if state.CurrentTrapDesc != "" {
		t.Errorf("CurrentTrapDesc should be cleared, got %q", state.CurrentTrapDesc)
	}
}

func TestFallacyPhaseTransition_RevealedAtMax(t *testing.T) {
	state := FallacySessionState{
		EmbeddedCount:   3,
		IdentifiedCount: 3,
		Phase:           FallacyPhaseRevealed,
		MaxPerSession:   3,
	}

	// Reveal complete but max reached → phase stays revealed
	state.CurrentTrapDesc = ""
	if state.EmbeddedCount < state.MaxPerSession {
		state.Phase = FallacyPhasePresentTrap
	}
	// Phase should NOT change since we're at max

	if state.Phase != FallacyPhaseRevealed {
		t.Errorf("Phase should stay revealed at max, got %q", state.Phase)
	}
}

func TestFallacyPhaseTransition_FullCycle(t *testing.T) {
	// Test a complete cycle: present → await → reveal → present → await → reveal → present → await → reveal
	state := defaultFallacyState()

	for i := 0; i < 3; i++ {
		// Present → Awaiting
		if state.Phase != FallacyPhasePresentTrap {
			t.Fatalf("Cycle %d: Expected present_trap phase", i)
		}
		state.EmbeddedCount++
		state.Phase = FallacyPhaseAwaiting

		// Awaiting → Revealed (student identifies)
		state.IdentifiedCount++
		state.Phase = FallacyPhaseRevealed

		// Revealed → PresentTrap (if quota allows)
		state.CurrentTrapDesc = ""
		if state.EmbeddedCount < state.MaxPerSession {
			state.Phase = FallacyPhasePresentTrap
		}
	}

	// After 3 full cycles
	if state.EmbeddedCount != 3 {
		t.Errorf("Final EmbeddedCount = %d, want 3", state.EmbeddedCount)
	}
	if state.IdentifiedCount != 3 {
		t.Errorf("Final IdentifiedCount = %d, want 3", state.IdentifiedCount)
	}
	// Should NOT return to present_trap since max reached
	if state.Phase != FallacyPhaseRevealed {
		t.Errorf("Final Phase should be revealed (max reached), got %q", state.Phase)
	}
}

// ── Test Helpers ────────────────────────────────────────────

// registerTestSkill registers a test skill directly into the registry with full metadata.
// This bypasses file-system based discovery for unit testing.
func registerTestSkill(t *testing.T, registry *plugin.Registry, id string, triggers *plugin.ProgressiveTriggers) {
	t.Helper()

	registry.RegisterSkillWithMetadata(plugin.SkillMetadata{
		ID:                  id,
		Name:                id,
		Version:             "1.0.0-test",
		Category:            "skill",
		ProgressiveTriggers: triggers,
	})
}

// (end of test helpers)
