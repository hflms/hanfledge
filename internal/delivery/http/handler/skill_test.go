package handler

import (
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Skill Handler Unit Tests
// ============================

// -- marshalJSON Tests ----------------------------------------

func TestMarshalJSON_SimpleMap(t *testing.T) {
	input := map[string]interface{}{"key": "value"}
	data, err := marshalJSON(input)
	if err != nil {
		t.Fatalf("marshalJSON error: %v", err)
	}
	expected := `{"key":"value"}`
	if string(data) != expected {
		t.Errorf("marshalJSON = %s, want %s", string(data), expected)
	}
}

func TestMarshalJSON_EmptyMap(t *testing.T) {
	input := map[string]interface{}{}
	data, err := marshalJSON(input)
	if err != nil {
		t.Fatalf("marshalJSON error: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("marshalJSON(empty) = %s, want {}", string(data))
	}
}

func TestMarshalJSON_NestedMap(t *testing.T) {
	input := map[string]interface{}{
		"level": "high",
		"thresholds": map[string]interface{}{
			"min": 0.3,
			"max": 0.8,
		},
	}
	data, err := marshalJSON(input)
	if err != nil {
		t.Fatalf("marshalJSON error: %v", err)
	}
	if len(data) == 0 {
		t.Error("marshalJSON returned empty bytes")
	}
}

func TestMarshalJSON_Nil(t *testing.T) {
	data, err := marshalJSON(nil)
	if err != nil {
		t.Fatalf("marshalJSON(nil) error: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("marshalJSON(nil) = %s, want null", string(data))
	}
}

func TestMarshalJSON_String(t *testing.T) {
	data, err := marshalJSON("hello")
	if err != nil {
		t.Fatalf("marshalJSON error: %v", err)
	}
	if string(data) != `"hello"` {
		t.Errorf("marshalJSON(string) = %s, want \"hello\"", string(data))
	}
}

func TestMarshalJSON_SliceOfInts(t *testing.T) {
	data, err := marshalJSON([]int{1, 2, 3})
	if err != nil {
		t.Fatalf("marshalJSON error: %v", err)
	}
	if string(data) != "[1,2,3]" {
		t.Errorf("marshalJSON(slice) = %s, want [1,2,3]", string(data))
	}
}

// -- ScaffoldLevel Validation Tests ---------------------------

func TestScaffoldLevelValidation(t *testing.T) {
	tests := []struct {
		name  string
		level model.ScaffoldLevel
		valid bool
	}{
		{"high is valid", model.ScaffoldHigh, true},
		{"medium is valid", model.ScaffoldMedium, true},
		{"low is valid", model.ScaffoldLow, true},
		{"empty is invalid", "", false},
		{"unknown is invalid", "unknown", false},
		{"uppercase HIGH is invalid", "HIGH", false},
		{"mixed case is invalid", "High", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid := tc.level == model.ScaffoldHigh ||
				tc.level == model.ScaffoldMedium ||
				tc.level == model.ScaffoldLow
			if valid != tc.valid {
				t.Errorf("ScaffoldLevel %q: valid = %v, want %v",
					tc.level, valid, tc.valid)
			}
		})
	}
}

// -- ScaffoldLevel Constants Tests ----------------------------

func TestScaffoldLevelConstants(t *testing.T) {
	if string(model.ScaffoldHigh) != "high" {
		t.Errorf("ScaffoldHigh = %q, want %q", model.ScaffoldHigh, "high")
	}
	if string(model.ScaffoldMedium) != "medium" {
		t.Errorf("ScaffoldMedium = %q, want %q", model.ScaffoldMedium, "medium")
	}
	if string(model.ScaffoldLow) != "low" {
		t.Errorf("ScaffoldLow = %q, want %q", model.ScaffoldLow, "low")
	}
}

// -- MountSkillRequest Fields Test ----------------------------

func TestMountSkillRequestDefaults(t *testing.T) {
	req := MountSkillRequest{}
	if req.SkillID != "" {
		t.Error("default SkillID should be empty")
	}
	if req.ScaffoldLevel != "" {
		t.Error("default ScaffoldLevel should be empty")
	}
	if req.Priority != 0 {
		t.Errorf("default Priority = %d, want 0", req.Priority)
	}
	if req.ConstraintsJSON != nil {
		t.Error("default ConstraintsJSON should be nil")
	}
	if req.ProgressiveRule != nil {
		t.Error("default ProgressiveRule should be nil")
	}
}

// -- SkillHandler Constructor Test ----------------------------

func TestNewSkillHandler(t *testing.T) {
	h := NewSkillHandler(nil, nil, nil)
	if h == nil {
		t.Fatal("NewSkillHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB")
	}
	if h.Registry != nil {
		t.Error("expected nil Registry")
	}
	if h.LLMProvider != nil {
		t.Error("expected nil LLMProvider")
	}
}
