package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================
// Anti-God Skill Validator Tests (§8.2.1)
// ============================

func TestDefaultSkillValidator(t *testing.T) {
	v := DefaultSkillValidator()
	if v.MaxSkillMDTokens != 2000 {
		t.Errorf("MaxSkillMDTokens = %d, want 2000", v.MaxSkillMDTokens)
	}
	if v.MaxEvalDimensions != 5 {
		t.Errorf("MaxEvalDimensions = %d, want 5", v.MaxEvalDimensions)
	}
	if v.MaxSubjectSpan != 3 {
		t.Errorf("MaxSubjectSpan = %d, want 3", v.MaxSubjectSpan)
	}
}

// -- Namespace Validation ---

func TestValidateNamespace_Valid3Segment(t *testing.T) {
	v := DefaultSkillValidator()
	tests := []struct {
		id    string
		valid bool
	}{
		{"math_concept_socratic", true},
		{"physics_homework_fallacy", true},
		{"english_review_roleplay", true},
		{"general_assessment_quiz", true},
		{"cs_debug_guided", true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			err := v.ValidateNamespaceOnly(tt.id)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestValidateNamespace_Invalid(t *testing.T) {
	v := DefaultSkillValidator()
	tests := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"single segment", "socratic"},
		{"two segments", "math_socratic"},
		{"has hyphen", "math-concept-socratic"},
		{"has uppercase", "Math_concept_socratic"},
		{"empty subject", "_concept_socratic"},
		{"empty scenario", "math__socratic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateNamespaceOnly(tt.id)
			if err == nil {
				t.Errorf("expected error for %q, got nil", tt.id)
			}
		})
	}
}

func TestValidateNamespace_UnknownSubjectWarning(t *testing.T) {
	v := DefaultSkillValidator()
	skill := &RegisteredSkill{
		Metadata: SkillMetadata{
			ID:   "cooking_recipe_guided",
			Name: "Test Skill",
		},
		SkillMDPath: "", // 跳过文件检查
	}

	result := v.Validate(skill)
	// "cooking" 不在已知学科列表中，应有警告
	if len(result.Warnings) == 0 {
		t.Error("expected warning for unknown subject 'cooking'")
	}

	foundWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "cooking") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected warning mentioning 'cooking'")
	}
}

// -- Eval Dimensions ---

func TestValidateEvalDimensions_WithinLimit(t *testing.T) {
	v := DefaultSkillValidator()
	skill := &RegisteredSkill{
		Metadata: SkillMetadata{
			ID:             "math_concept_socratic",
			Name:           "Test",
			EvalDimensions: []string{"a", "b", "c", "d", "e"},
		},
	}

	result := v.Validate(skill)
	for _, e := range result.Errors {
		if containsStr(e, "评估维度") {
			t.Errorf("5 dimensions should be OK, but got error: %s", e)
		}
	}
}

func TestValidateEvalDimensions_ExceedsLimit(t *testing.T) {
	v := DefaultSkillValidator()
	skill := &RegisteredSkill{
		Metadata: SkillMetadata{
			ID:             "math_concept_socratic",
			Name:           "Test",
			EvalDimensions: []string{"a", "b", "c", "d", "e", "f"},
		},
	}

	result := v.Validate(skill)
	if result.Valid {
		t.Error("6 dimensions should fail validation")
	}

	foundError := false
	for _, e := range result.Errors {
		if containsStr(e, "评估维度") {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected error about eval dimensions")
	}
}

// -- Subject Span ---

func TestValidateSubjectSpan_WithinLimit(t *testing.T) {
	v := DefaultSkillValidator()
	skill := &RegisteredSkill{
		Metadata: SkillMetadata{
			ID:       "math_concept_socratic",
			Name:     "Test",
			Subjects: []string{"math", "physics"},
		},
	}

	result := v.Validate(skill)
	for _, w := range result.Warnings {
		if containsStr(w, "横跨") {
			t.Errorf("2 subjects should be OK, but got warning: %s", w)
		}
	}
}

func TestValidateSubjectSpan_ExceedsLimit(t *testing.T) {
	v := DefaultSkillValidator()
	skill := &RegisteredSkill{
		Metadata: SkillMetadata{
			ID:       "general_concept_socratic",
			Name:     "Test",
			Subjects: []string{"math", "physics", "chemistry", "biology"},
		},
	}

	result := v.Validate(skill)
	foundWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "横跨") || containsStr(w, "拆分") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("4 subjects should trigger split warning")
	}
}

// -- SKILL.md Token Limit ---

func TestValidateSkillMDSize_WithinLimit(t *testing.T) {
	// 创建临时 SKILL.md
	tmpDir := t.TempDir()
	skillMD := filepath.Join(tmpDir, "SKILL.md")
	os.WriteFile(skillMD, []byte("Short skill instructions"), 0644)

	v := DefaultSkillValidator()
	skill := &RegisteredSkill{
		Metadata:    SkillMetadata{ID: "math_concept_socratic", Name: "Test"},
		SkillMDPath: skillMD,
	}

	result := v.Validate(skill)
	for _, e := range result.Errors {
		if containsStr(e, "Token 限制") {
			t.Errorf("short SKILL.md should be OK, but got error: %s", e)
		}
	}
}

func TestValidateSkillMDSize_ExceedsLimit(t *testing.T) {
	// 创建一个超长的 SKILL.md
	tmpDir := t.TempDir()
	skillMD := filepath.Join(tmpDir, "SKILL.md")

	// 生成超过 2000 token 的中文内容 (约 3000 中文字 = 2000 tokens)
	longContent := ""
	for i := 0; i < 3500; i++ {
		longContent += "这"
	}
	os.WriteFile(skillMD, []byte(longContent), 0644)

	v := DefaultSkillValidator()
	skill := &RegisteredSkill{
		Metadata:    SkillMetadata{ID: "math_concept_socratic", Name: "Test"},
		SkillMDPath: skillMD,
	}

	result := v.Validate(skill)
	if result.Valid {
		t.Error("oversized SKILL.md should fail validation")
	}

	foundError := false
	for _, e := range result.Errors {
		if containsStr(e, "Token 限制") || containsStr(e, "超出") {
			foundError = true
		}
	}
	if !foundError {
		t.Errorf("expected token limit error, errors: %v", result.Errors)
	}
}

// -- Full Validation ---

func TestValidate_FullValid(t *testing.T) {
	tmpDir := t.TempDir()
	skillMD := filepath.Join(tmpDir, "SKILL.md")
	os.WriteFile(skillMD, []byte("Valid skill instructions."), 0644)

	v := DefaultSkillValidator()
	skill := &RegisteredSkill{
		Metadata: SkillMetadata{
			ID:             "math_concept_socratic",
			Name:           "苏格拉底数学概念引导",
			Subjects:       []string{"math"},
			EvalDimensions: []string{"depth", "leakage"},
		},
		SkillMDPath: skillMD,
	}

	result := v.Validate(skill)
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	v := &SkillValidator{
		MaxSkillMDTokens:  2000,
		MaxEvalDimensions: 2, // 严格限制
		MaxSubjectSpan:    1,
	}

	skill := &RegisteredSkill{
		Metadata: SkillMetadata{
			ID:             "bad", // 只有 1 段
			Name:           "Test",
			EvalDimensions: []string{"a", "b", "c"},     // 超过 2 个
			Subjects:       []string{"math", "physics"}, // 超过 1 个
		},
	}

	result := v.Validate(skill)
	if result.Valid {
		t.Error("expected invalid")
	}
	if len(result.Errors) < 2 {
		t.Errorf("expected multiple errors, got %d", len(result.Errors))
	}
}

func TestEstimateSkillTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		min  int
		max  int
	}{
		{"empty", "", 0, 2},
		{"english", "hello world", 2, 10},
		{"chinese", "你好世界", 2, 5},
		{"mixed", "Hello 世界", 2, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := estimateSkillTokens(tt.text)
			if tokens < tt.min || tokens > tt.max {
				t.Errorf("estimateSkillTokens(%q) = %d, want in [%d, %d]",
					tt.text, tokens, tt.min, tt.max)
			}
		})
	}
}

// -- Helper --

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
