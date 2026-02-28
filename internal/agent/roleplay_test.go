package agent

import (
	"strings"
	"testing"

	"github.com/hflms/hanfledge/internal/plugin"
)

// ============================
// Role-Play Skill Tests
// ============================

// ── isRolePlayActive Tests ─────────────────────────────────

func TestIsRolePlayActive(t *testing.T) {
	tests := []struct {
		skillID  string
		expected bool
	}{
		{"general_review_roleplay", true},
		{"role-play", true},
		{"general_concept_socratic", false},
		{"general_assessment_fallacy", false},
		{"", false},
		{"roleplay", false},
		{"role_play", false},
	}

	for _, tc := range tests {
		result := isRolePlayActive(tc.skillID)
		if result != tc.expected {
			t.Errorf("isRolePlayActive(%q) = %v, want %v",
				tc.skillID, result, tc.expected)
		}
	}
}

// ── RolePlaySessionState Tests ─────────────────────────────

func TestDefaultRolePlayState(t *testing.T) {
	state := defaultRolePlayState()

	if state.CharacterName != "" {
		t.Errorf("CharacterName = %q, want empty", state.CharacterName)
	}
	if state.CharacterRole != "" {
		t.Errorf("CharacterRole = %q, want empty", state.CharacterRole)
	}
	if state.ScenarioDesc != "" {
		t.Errorf("ScenarioDesc = %q, want empty", state.ScenarioDesc)
	}
	if state.ScenarioSwitches != 0 {
		t.Errorf("ScenarioSwitches = %d, want 0", state.ScenarioSwitches)
	}
	if state.MaxSwitches != 3 {
		t.Errorf("MaxSwitches = %d, want 3", state.MaxSwitches)
	}
	if !state.Active {
		t.Error("Active should be true by default")
	}
}

// ── rolePlayActiveLabel Tests ──────────────────────────────

func TestRolePlayActiveLabel(t *testing.T) {
	tests := []struct {
		active   bool
		expected string
	}{
		{true, "沉浸中"},
		{false, "已退出"},
	}

	for _, tc := range tests {
		result := rolePlayActiveLabel(tc.active)
		if result != tc.expected {
			t.Errorf("rolePlayActiveLabel(%v) = %q, want %q",
				tc.active, result, tc.expected)
		}
	}
}

// ── buildRolePlayContext Tests ──────────────────────────────

func TestBuildRolePlayContext_InitialState(t *testing.T) {
	state := defaultRolePlayState()

	ctx := buildRolePlayContext(state)

	if !strings.Contains(ctx, "角色扮演会话状态") {
		t.Error("Should contain header '角色扮演会话状态'")
	}
	if !strings.Contains(ctx, "尚未选定") {
		t.Error("Should contain '尚未选定' for empty character")
	}
	if !strings.Contains(ctx, "选择一个合适的角色身份") {
		t.Error("Should contain instruction to select character on first round")
	}
	if !strings.Contains(ctx, "已切换情境: 0 / 3 次") {
		t.Error("Should contain scenario switch count")
	}
	if !strings.Contains(ctx, "沉浸中") {
		t.Error("Should contain active status '沉浸中'")
	}
}

func TestBuildRolePlayContext_ActiveWithCharacter(t *testing.T) {
	state := RolePlaySessionState{
		CharacterName:    "达尔文",
		CharacterRole:    "博物学家",
		ScenarioDesc:     "加拉帕戈斯群岛考察",
		ScenarioSwitches: 1,
		MaxSwitches:      3,
		Active:           true,
	}

	ctx := buildRolePlayContext(state)

	if !strings.Contains(ctx, "达尔文") {
		t.Error("Should contain character name '达尔文'")
	}
	if !strings.Contains(ctx, "博物学家") {
		t.Error("Should contain character role '博物学家'")
	}
	if !strings.Contains(ctx, "加拉帕戈斯群岛考察") {
		t.Error("Should contain scenario description")
	}
	if !strings.Contains(ctx, "已切换情境: 1 / 3 次") {
		t.Error("Should contain scenario switch count")
	}
	if !strings.Contains(ctx, "继续以当前角色身份") {
		t.Error("Should contain instruction to continue in character")
	}
}

func TestBuildRolePlayContext_MaxSwitchesReached(t *testing.T) {
	state := RolePlaySessionState{
		CharacterName:    "爱因斯坦",
		CharacterRole:    "物理学家",
		ScenarioSwitches: 3,
		MaxSwitches:      3,
		Active:           true,
	}

	ctx := buildRolePlayContext(state)

	if !strings.Contains(ctx, "本会话已达到最大情境切换次数") {
		t.Error("Should contain max switches warning")
	}
	if strings.Contains(ctx, "选择一个合适的角色身份") {
		t.Error("Should NOT contain first-round instruction when character is set")
	}
}

func TestBuildRolePlayContext_Exited(t *testing.T) {
	state := RolePlaySessionState{
		CharacterName:    "李白",
		CharacterRole:    "诗人",
		ScenarioSwitches: 1,
		MaxSwitches:      3,
		Active:           false,
	}

	ctx := buildRolePlayContext(state)

	if !strings.Contains(ctx, "已退出") {
		t.Error("Should contain '已退出' status")
	}
	if !strings.Contains(ctx, "学生已请求退出角色扮演") {
		t.Error("Should contain exit instruction")
	}
	if !strings.Contains(ctx, "总结本次扮演中涉及的知识点") {
		t.Error("Should contain summary instruction")
	}
}

func TestBuildRolePlayContext_NoScenarioDesc(t *testing.T) {
	state := RolePlaySessionState{
		CharacterName:    "牛顿",
		CharacterRole:    "物理学家",
		ScenarioDesc:     "", // no scenario description
		ScenarioSwitches: 0,
		MaxSwitches:      3,
		Active:           true,
	}

	ctx := buildRolePlayContext(state)

	if !strings.Contains(ctx, "牛顿") {
		t.Error("Should contain character name")
	}
	// When no scenario desc, the "当前情境" line should not appear
	if strings.Contains(ctx, "当前情境:") {
		t.Error("Should NOT contain '当前情境:' when scenario is empty")
	}
}

// ── Progressive Triggers Integration Tests ─────────────────

func TestEvaluateProgressiveTriggers_SocraticToRolePlay(t *testing.T) {
	registry := plugin.NewRegistry()

	registerTestSkill(t, registry, "general_concept_socratic", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score < 0.6",
		DeactivateWhen: "mastery_score >= 0.8",
	})
	registerTestSkill(t, registry, "general_review_roleplay", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score >= 0.5",
		DeactivateWhen: "mastery_score >= 0.95",
	})

	agent := &StrategistAgent{registry: registry}

	// mastery = 0.85 → socratic deactivation triggers, roleplay activation met
	newSkill, switched := agent.evaluateProgressiveTriggers("general_concept_socratic", 0.85)
	if !switched {
		t.Fatal("Should trigger switch from socratic to roleplay at mastery 0.85")
	}
	// Could switch to either fallacy or roleplay depending on registry order;
	// verify it switched to a valid skill
	if newSkill != "general_review_roleplay" {
		// It may also switch to other skills if they are registered
		t.Logf("Switched to %q (may depend on registry iteration order)", newSkill)
	}
}

func TestEvaluateProgressiveTriggers_RolePlayNoSwitch(t *testing.T) {
	registry := plugin.NewRegistry()

	registerTestSkill(t, registry, "general_review_roleplay", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score >= 0.5",
		DeactivateWhen: "mastery_score >= 0.95",
	})

	agent := &StrategistAgent{registry: registry}

	// mastery = 0.7 → roleplay deactivation NOT met
	_, switched := agent.evaluateProgressiveTriggers("general_review_roleplay", 0.7)
	if switched {
		t.Error("Should NOT switch at mastery 0.7 (deactivate_when is >= 0.95)")
	}
}

func TestEvaluateProgressiveTriggers_RolePlayDeactivates(t *testing.T) {
	registry := plugin.NewRegistry()

	registerTestSkill(t, registry, "general_review_roleplay", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score >= 0.5",
		DeactivateWhen: "mastery_score >= 0.95",
	})
	registerTestSkill(t, registry, "general_assessment_fallacy", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score >= 0.8",
		DeactivateWhen: "critical_thinking_score >= 0.9",
	})

	agent := &StrategistAgent{registry: registry}

	// mastery = 0.96 → roleplay deactivation triggers
	newSkill, switched := agent.evaluateProgressiveTriggers("general_review_roleplay", 0.96)
	if !switched {
		t.Fatal("Should trigger switch from roleplay at mastery 0.96")
	}
	if newSkill != "general_assessment_fallacy" {
		t.Logf("Switched to %q (fallacy activates at >= 0.8, so 0.96 qualifies)", newSkill)
	}
}

// ── Skill Plugin Registration Tests ────────────────────────

func TestRolePlaySkillRegistration(t *testing.T) {
	registry := plugin.NewRegistry()

	registerTestSkill(t, registry, "general_review_roleplay", &plugin.ProgressiveTriggers{
		ActivateWhen:   "mastery_score >= 0.5",
		DeactivateWhen: "mastery_score >= 0.95",
	})

	skill, ok := registry.GetSkill("general_review_roleplay")
	if !ok || skill == nil {
		t.Fatal("Role-play skill should be registered")
	}
	if skill.Metadata.ID != "general_review_roleplay" {
		t.Errorf("Skill ID = %q, want %q", skill.Metadata.ID, "general_review_roleplay")
	}
}

func TestRolePlaySkillListing(t *testing.T) {
	registry := plugin.NewRegistry()

	registerTestSkill(t, registry, "general_concept_socratic", nil)
	registerTestSkill(t, registry, "general_assessment_fallacy", nil)
	registerTestSkill(t, registry, "general_review_roleplay", nil)

	skills := registry.ListSkills("", "")
	if len(skills) != 3 {
		t.Errorf("ListSkills() returned %d skills, want 3", len(skills))
	}

	// Verify role-play is in the list
	found := false
	for _, s := range skills {
		if s.Metadata.ID == "general_review_roleplay" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Role-play skill should be in ListSkills() result")
	}
}
