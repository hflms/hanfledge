package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ============================
// Plugin Registry Unit Tests
// ============================

// -- Constructor Tests ----------------------------------------

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.eventBus == nil {
		t.Error("EventBus should not be nil")
	}
	if r.validator == nil {
		t.Error("Validator should not be nil")
	}
	if len(r.skills) != 0 {
		t.Errorf("skills map should be empty, got %d", len(r.skills))
	}
}

func TestRegistry_EventBus(t *testing.T) {
	r := NewRegistry()
	eb := r.EventBus()
	if eb == nil {
		t.Fatal("EventBus() returned nil")
	}
	if eb != r.eventBus {
		t.Error("EventBus() should return the internal event bus")
	}
}

// -- RegisterSkillWithMetadata Tests --------------------------

func TestRegistry_RegisterSkillWithMetadata(t *testing.T) {
	r := NewRegistry()

	meta := SkillMetadata{
		ID:       "math_concept_socratic",
		Name:     "苏格拉底数学引导",
		Version:  "1.0.0",
		Category: "socratic",
		Subjects: []string{"math"},
	}

	r.RegisterSkillWithMetadata(meta)

	skill, ok := r.GetSkill("math_concept_socratic")
	if !ok {
		t.Fatal("GetSkill should find the registered skill")
	}
	if skill.Metadata.Name != "苏格拉底数学引导" {
		t.Errorf("Name = %q, want %q", skill.Metadata.Name, "苏格拉底数学引导")
	}
	if skill.State != PluginStateRunning {
		t.Errorf("State = %q, want %q", skill.State, PluginStateRunning)
	}
}

// -- RegisterCustomSkill Tests --------------------------------

func TestRegistry_RegisterCustomSkill(t *testing.T) {
	r := NewRegistry()

	meta := SkillMetadata{
		ID:   "custom_review_quiz",
		Name: "自定义复习测验",
	}
	content := "# Custom Skill\nCustom instructions here."

	r.RegisterCustomSkill(meta, content)

	skill, ok := r.GetSkill("custom_review_quiz")
	if !ok {
		t.Fatal("GetSkill should find the custom skill")
	}
	if !skill.IsCustom {
		t.Error("IsCustom should be true")
	}
	if skill.SkillMDContent != content {
		t.Errorf("SkillMDContent = %q, want %q", skill.SkillMDContent, content)
	}
}

// -- UnregisterSkill Tests ------------------------------------

func TestRegistry_UnregisterSkill(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "to_remove", Name: "Remove Me"})

	_, ok := r.GetSkill("to_remove")
	if !ok {
		t.Fatal("skill should exist before unregister")
	}

	r.UnregisterSkill("to_remove")

	_, ok = r.GetSkill("to_remove")
	if ok {
		t.Error("skill should not exist after unregister")
	}
}

func TestRegistry_UnregisterNonexistent(t *testing.T) {
	r := NewRegistry()
	// Should not panic
	r.UnregisterSkill("nonexistent")
}

// -- GetSkill Tests -------------------------------------------

func TestRegistry_GetSkill_NotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.GetSkill("nonexistent")
	if ok {
		t.Error("GetSkill should return false for nonexistent skill")
	}
}

// -- ListSkills Tests -----------------------------------------

func TestRegistry_ListSkills_All(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A", Category: "cat1", Subjects: []string{"math"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B", Category: "cat2", Subjects: []string{"physics"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "c", Name: "C", Category: "cat1", Subjects: []string{"math", "physics"}})

	all := r.ListSkills("", "")
	if len(all) != 3 {
		t.Errorf("ListSkills('','') = %d skills, want 3", len(all))
	}
}

func TestRegistry_ListSkills_BySubject(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A", Subjects: []string{"math"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B", Subjects: []string{"physics"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "c", Name: "C", Subjects: []string{"math", "physics"}})

	mathSkills := r.ListSkills("math", "")
	if len(mathSkills) != 2 {
		t.Errorf("ListSkills('math','') = %d skills, want 2", len(mathSkills))
	}
}

func TestRegistry_ListSkills_ByCategory(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A", Category: "socratic"})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B", Category: "diagnosis"})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "c", Name: "C", Category: "socratic"})

	socratic := r.ListSkills("", "socratic")
	if len(socratic) != 2 {
		t.Errorf("ListSkills('','socratic') = %d skills, want 2", len(socratic))
	}
}

func TestRegistry_ListSkills_ByCategoryIgnoresCase(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A", Category: "Socratic"})

	skills := r.ListSkills("", "socratic")
	if len(skills) != 1 {
		t.Errorf("ListSkills('','socratic') should match 'Socratic', got %d skills", len(skills))
	}
}

func TestRegistry_ListSkills_BySubjectAndCategory(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A", Category: "socratic", Subjects: []string{"math"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B", Category: "diagnosis", Subjects: []string{"math"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "c", Name: "C", Category: "socratic", Subjects: []string{"physics"}})

	result := r.ListSkills("math", "socratic")
	if len(result) != 1 {
		t.Errorf("ListSkills('math','socratic') = %d skills, want 1", len(result))
	}
	if len(result) == 1 && result[0].Metadata.ID != "a" {
		t.Errorf("expected skill 'a', got %q", result[0].Metadata.ID)
	}
}

// -- LoadConstraints Tests ------------------------------------

func TestRegistry_LoadConstraints_CustomSkill(t *testing.T) {
	r := NewRegistry()

	content := "# Custom Skill MD\nInstructions."
	r.RegisterCustomSkill(SkillMetadata{ID: "custom_test", Name: "Test"}, content)

	constraints, err := r.LoadConstraints("custom_test")
	if err != nil {
		t.Fatalf("LoadConstraints() error: %v", err)
	}
	if constraints.SkillID != "custom_test" {
		t.Errorf("SkillID = %q, want %q", constraints.SkillID, "custom_test")
	}
	if constraints.RawMarkdown != content {
		t.Errorf("RawMarkdown = %q, want %q", constraints.RawMarkdown, content)
	}
}

func TestRegistry_LoadConstraints_FilesystemSkill(t *testing.T) {
	r := NewRegistry()

	// Create temp SKILL.md file
	tmpDir := t.TempDir()
	skillMDPath := filepath.Join(tmpDir, "SKILL.md")
	expectedContent := "# Test Skill\nSome constraints."
	os.WriteFile(skillMDPath, []byte(expectedContent), 0644)

	r.mu.Lock()
	r.skills["fs_skill"] = &RegisteredSkill{
		Metadata:    SkillMetadata{ID: "fs_skill", Name: "FS Skill"},
		SkillMDPath: skillMDPath,
		State:       PluginStateRunning,
	}
	r.mu.Unlock()

	constraints, err := r.LoadConstraints("fs_skill")
	if err != nil {
		t.Fatalf("LoadConstraints() error: %v", err)
	}
	if constraints.RawMarkdown != expectedContent {
		t.Errorf("RawMarkdown = %q, want %q", constraints.RawMarkdown, expectedContent)
	}
}

func TestRegistry_LoadConstraints_NotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.LoadConstraints("nonexistent")
	if err == nil {
		t.Error("LoadConstraints() should return error for nonexistent skill")
	}
}

// -- LoadSkills (filesystem discovery) Tests -------------------

func TestRegistry_LoadSkills_ValidPlugin(t *testing.T) {
	r := NewRegistry()

	// Create temp plugin directory structure
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "math_concept_socratic", "backend")
	os.MkdirAll(skillDir, 0755)

	meta := SkillMetadata{
		ID:      "math_concept_socratic",
		Name:    "苏格拉底数学引导",
		Version: "1.0.0",
	}
	metaJSON, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(skillDir, "metadata.json"), metaJSON, 0644)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill instructions"), 0644)

	err := r.LoadSkills(tmpDir)
	if err != nil {
		t.Fatalf("LoadSkills() error: %v", err)
	}

	skill, ok := r.GetSkill("math_concept_socratic")
	if !ok {
		t.Fatal("Skill should be loaded")
	}
	if skill.State != PluginStateRunning {
		t.Errorf("State = %q, want %q", skill.State, PluginStateRunning)
	}
}

func TestRegistry_LoadSkills_MissingMetadata(t *testing.T) {
	r := NewRegistry()

	// Create dir without metadata.json
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "broken_skill", "backend"), 0755)

	err := r.LoadSkills(tmpDir)
	if err != nil {
		t.Fatalf("LoadSkills() should not return error for skip, got: %v", err)
	}

	// Skill should not be loaded
	_, ok := r.GetSkill("broken_skill")
	if ok {
		t.Error("Skill with missing metadata should not be loaded")
	}
}

func TestRegistry_LoadSkills_NonexistentDir(t *testing.T) {
	r := NewRegistry()

	err := r.LoadSkills("/tmp/nonexistent_dir_" + t.Name())
	if err != nil {
		t.Errorf("LoadSkills() should return nil for nonexistent dir, got: %v", err)
	}
}

func TestRegistry_LoadSkills_SkipFiles(t *testing.T) {
	r := NewRegistry()

	tmpDir := t.TempDir()
	// Create a regular file (not a directory) — should be skipped
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("readme"), 0644)

	err := r.LoadSkills(tmpDir)
	if err != nil {
		t.Fatalf("LoadSkills() error: %v", err)
	}
	if len(r.skills) != 0 {
		t.Errorf("No skills should be loaded, got %d", len(r.skills))
	}
}

// -- ResolveDependencies Tests --------------------------------

func TestRegistry_ResolveDependencies_NoDeps(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A"})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B"})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "c", Name: "C"})

	order, err := r.ResolveDependencies()
	if err != nil {
		t.Fatalf("ResolveDependencies() error: %v", err)
	}
	if len(order) != 3 {
		t.Errorf("order length = %d, want 3", len(order))
	}
}

func TestRegistry_ResolveDependencies_LinearChain(t *testing.T) {
	r := NewRegistry()

	// c depends on b, b depends on a
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A"})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B", Dependencies: []string{"a"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "c", Name: "C", Dependencies: []string{"b"}})

	order, err := r.ResolveDependencies()
	if err != nil {
		t.Fatalf("ResolveDependencies() error: %v", err)
	}

	// a must come before b, b must come before c
	idxA, idxB, idxC := -1, -1, -1
	for i, id := range order {
		switch id {
		case "a":
			idxA = i
		case "b":
			idxB = i
		case "c":
			idxC = i
		}
	}

	if idxA >= idxB {
		t.Errorf("a (idx=%d) should come before b (idx=%d)", idxA, idxB)
	}
	if idxB >= idxC {
		t.Errorf("b (idx=%d) should come before c (idx=%d)", idxB, idxC)
	}
}

func TestRegistry_ResolveDependencies_DiamondShape(t *testing.T) {
	r := NewRegistry()

	// d depends on b and c, both b and c depend on a
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A"})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B", Dependencies: []string{"a"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "c", Name: "C", Dependencies: []string{"a"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "d", Name: "D", Dependencies: []string{"b", "c"}})

	order, err := r.ResolveDependencies()
	if err != nil {
		t.Fatalf("ResolveDependencies() error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("order length = %d, want 4", len(order))
	}

	// a must come before b and c; b and c must come before d
	indexOf := func(id string) int {
		for i, x := range order {
			if x == id {
				return i
			}
		}
		return -1
	}

	if indexOf("a") >= indexOf("b") || indexOf("a") >= indexOf("c") {
		t.Errorf("a should come before b and c; order=%v", order)
	}
	if indexOf("b") >= indexOf("d") || indexOf("c") >= indexOf("d") {
		t.Errorf("b and c should come before d; order=%v", order)
	}
}

func TestRegistry_ResolveDependencies_CircularDependency(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A", Dependencies: []string{"b"}})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B", Dependencies: []string{"a"}})

	_, err := r.ResolveDependencies()
	if err == nil {
		t.Error("ResolveDependencies() should return error for circular dependency")
	}
}

func TestRegistry_ResolveDependencies_MissingDependency(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A", Dependencies: []string{"nonexistent"}})

	_, err := r.ResolveDependencies()
	if err == nil {
		t.Error("ResolveDependencies() should return error for unknown dependency")
	}
}

func TestRegistry_ResolveDependencies_Empty(t *testing.T) {
	r := NewRegistry()

	order, err := r.ResolveDependencies()
	if err != nil {
		t.Fatalf("ResolveDependencies() error: %v", err)
	}
	if len(order) != 0 {
		t.Errorf("order length = %d, want 0", len(order))
	}
}

// -- HealthCheckAll Tests -------------------------------------

func TestRegistry_HealthCheckAll_DeclarativeOnly(t *testing.T) {
	r := NewRegistry()

	r.RegisterSkillWithMetadata(SkillMetadata{ID: "a", Name: "A"})
	r.RegisterSkillWithMetadata(SkillMetadata{ID: "b", Name: "B"})

	results := r.HealthCheckAll(context.Background())
	if len(results) != 2 {
		t.Fatalf("HealthCheckAll() returned %d results, want 2", len(results))
	}

	for id, status := range results {
		if !status.Healthy {
			t.Errorf("Declarative skill %q should be healthy", id)
		}
		if status.Message != "declarative" {
			t.Errorf("Declarative skill %q message = %q, want %q", id, status.Message, "declarative")
		}
	}
}

// -- containsIgnoreCase helper Tests --------------------------

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		target string
		want   bool
	}{
		{"exact match", []string{"math", "physics"}, "math", true},
		{"case insensitive", []string{"Math", "Physics"}, "math", true},
		{"not found", []string{"math"}, "chemistry", false},
		{"empty slice", []string{}, "math", false},
		{"empty target", []string{"math"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsIgnoreCase(tt.slice, tt.target)
			if got != tt.want {
				t.Errorf("containsIgnoreCase(%v, %q) = %v, want %v", tt.slice, tt.target, got, tt.want)
			}
		})
	}
}

// -- LoadTemplates Tests --------------------------------------

func TestRegistry_LoadTemplates_FromFilesystem(t *testing.T) {
	r := NewRegistry()

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)
	os.WriteFile(filepath.Join(templatesDir, "rubric.md"), []byte("# Rubric"), 0644)
	os.WriteFile(filepath.Join(templatesDir, "prompt.md"), []byte("# Prompt"), 0644)

	r.mu.Lock()
	r.skills["test_skill"] = &RegisteredSkill{
		Metadata:      SkillMetadata{ID: "test_skill", Name: "Test"},
		TemplatesPath: templatesDir,
		State:         PluginStateRunning,
	}
	r.mu.Unlock()

	// Load specific template
	templates, err := r.LoadTemplates("test_skill", []string{"rubric.md"})
	if err != nil {
		t.Fatalf("LoadTemplates() error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("LoadTemplates() returned %d templates, want 1", len(templates))
	}
	if templates[0].Content != "# Rubric" {
		t.Errorf("template content = %q, want %q", templates[0].Content, "# Rubric")
	}
}

func TestRegistry_LoadTemplates_AllTemplates(t *testing.T) {
	r := NewRegistry()

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)
	os.WriteFile(filepath.Join(templatesDir, "rubric.md"), []byte("# Rubric"), 0644)
	os.WriteFile(filepath.Join(templatesDir, "prompt.md"), []byte("# Prompt"), 0644)

	r.mu.Lock()
	r.skills["test_skill"] = &RegisteredSkill{
		Metadata:      SkillMetadata{ID: "test_skill", Name: "Test"},
		TemplatesPath: templatesDir,
		State:         PluginStateRunning,
	}
	r.mu.Unlock()

	// Load all templates (empty slice)
	templates, err := r.LoadTemplates("test_skill", nil)
	if err != nil {
		t.Fatalf("LoadTemplates() error: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("LoadTemplates() returned %d templates, want 2", len(templates))
	}
}

func TestRegistry_LoadTemplates_SkillNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.LoadTemplates("nonexistent", nil)
	if err == nil {
		t.Error("LoadTemplates() should return error for nonexistent skill")
	}
}
