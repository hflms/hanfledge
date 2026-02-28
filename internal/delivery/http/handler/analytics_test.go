package handler

import (
	"testing"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Phase G: Analytics Handler Tests
// ============================

// -- classifyTurnType Tests -----------------------------------

func TestClassifyTurnType(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		lastRole string
		depth    int
		expected string
	}{
		{"student initial question", "student", "", 0, "question"},
		{"student question after coach at depth 0", "student", "coach", 0, "question"},
		{"student correction after coach probe", "student", "coach", 1, "correction"},
		{"student deep correction", "student", "coach", 3, "correction"},
		{"coach initial response", "coach", "student", 0, "response"},
		{"coach probe at depth", "coach", "student", 1, "probe"},
		{"coach deep probe", "coach", "student", 3, "probe"},
		{"coach after coach", "coach", "coach", 0, "response"},
		{"system message", "system", "", 0, "scaffold_change"},
		{"system after student", "system", "student", 0, "scaffold_change"},
		{"unknown role", "unknown", "", 0, "response"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := classifyTurnType(tc.role, tc.lastRole, tc.depth)
			if result != tc.expected {
				t.Errorf("classifyTurnType(%q, %q, %d) = %q, want %q",
					tc.role, tc.lastRole, tc.depth, result, tc.expected)
			}
		})
	}
}

// -- maskName Tests -------------------------------------------

func TestMaskName(t *testing.T) {
	h := &AnalyticsHandler{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"two-char chinese", "张三", "张*"},
		{"three-char chinese", "李明明", "李**"},
		{"single char", "张", "张"},
		{"empty string", "", ""},
		{"english name", "John", "J***"},
		{"two-char english", "Al", "A*"},
		{"single english", "A", "A"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := h.maskName(tc.input)
			if result != tc.expected {
				t.Errorf("maskName(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// -- buildInquiryTree Tests -----------------------------------

func makeInteractionWithID(id uint, role, content, skillID string, minuteOffset int) model.Interaction {
	inter := model.Interaction{
		Role:    role,
		Content: content,
		SkillID: skillID,
	}
	inter.ID = id
	inter.CreatedAt = time.Date(2026, 1, 1, 0, minuteOffset, 0, 0, time.UTC)
	return inter
}

func TestBuildInquiryTree_Empty(t *testing.T) {
	roots, maxDepth := buildInquiryTree(nil, nil)
	if roots != nil {
		t.Error("empty interactions should return nil roots")
	}
	if maxDepth != 0 {
		t.Errorf("empty interactions maxDepth = %d, want 0", maxDepth)
	}
}

func TestBuildInquiryTree_SingleStudentMessage(t *testing.T) {
	interactions := []model.Interaction{
		makeInteractionWithID(1, "student", "什么是牛顿第二定律？", "general_concept_socratic", 0),
	}

	roots, maxDepth := buildInquiryTree(interactions, nil)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if maxDepth != 0 {
		t.Errorf("single message maxDepth = %d, want 0", maxDepth)
	}
	if roots[0].Role != "student" {
		t.Errorf("root role = %q, want student", roots[0].Role)
	}
	if roots[0].TurnType != "question" {
		t.Errorf("turn type = %q, want question", roots[0].TurnType)
	}
}

func TestBuildInquiryTree_StudentCoachPair(t *testing.T) {
	interactions := []model.Interaction{
		makeInteractionWithID(1, "student", "什么是力？", "general_concept_socratic", 0),
		makeInteractionWithID(2, "coach", "力是一个很有趣的概念...", "general_concept_socratic", 1),
	}

	roots, maxDepth := buildInquiryTree(interactions, nil)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if maxDepth != 0 {
		t.Errorf("simple pair maxDepth = %d, want 0", maxDepth)
	}
	if len(roots[0].Children) != 1 {
		t.Fatalf("root should have 1 child, got %d", len(roots[0].Children))
	}
	if roots[0].Children[0].Role != "coach" {
		t.Errorf("child role = %q, want coach", roots[0].Children[0].Role)
	}
}

func TestBuildInquiryTree_DeepConversation(t *testing.T) {
	// student → coach → student (deepens) → coach (probe) → student (deeper)
	interactions := []model.Interaction{
		makeInteractionWithID(1, "student", "什么是力？", "general_concept_socratic", 0),
		makeInteractionWithID(2, "coach", "你觉得力是什么？", "general_concept_socratic", 1),
		makeInteractionWithID(3, "student", "力是推或拉的作用", "general_concept_socratic", 2),
		makeInteractionWithID(4, "coach", "很好，那力的方向呢？", "general_concept_socratic", 3),
		makeInteractionWithID(5, "student", "力有方向", "general_concept_socratic", 4),
	}

	roots, maxDepth := buildInquiryTree(interactions, nil)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if maxDepth != 2 {
		t.Errorf("deep conversation maxDepth = %d, want 2", maxDepth)
	}
}

func TestBuildInquiryTree_SkillSwitch(t *testing.T) {
	// Skill switch should create a new root
	interactions := []model.Interaction{
		makeInteractionWithID(1, "student", "解释光合作用", "general_concept_socratic", 0),
		makeInteractionWithID(2, "coach", "让我们一起思考...", "general_concept_socratic", 1),
		makeInteractionWithID(3, "student", "识别谬误", "general_assessment_fallacy", 2),
		makeInteractionWithID(4, "coach", "看看这个论点...", "general_assessment_fallacy", 3),
	}

	roots, _ := buildInquiryTree(interactions, nil)
	if len(roots) != 2 {
		t.Errorf("skill switch should create 2 roots, got %d", len(roots))
	}
}

func TestBuildInquiryTree_SystemMessage(t *testing.T) {
	interactions := []model.Interaction{
		makeInteractionWithID(1, "student", "什么是力？", "general_concept_socratic", 0),
		makeInteractionWithID(2, "system", "支架等级调整为 low", "", 1),
		makeInteractionWithID(3, "coach", "力的概念...", "general_concept_socratic", 2),
	}

	roots, _ := buildInquiryTree(interactions, nil)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	// System message is a metadata child; coach should also be a child
	// since system messages don't disrupt the conversation flow
	if len(roots[0].Children) != 2 {
		t.Errorf("root should have 2 children (system + coach), got %d", len(roots[0].Children))
	}
	// Verify the system node has correct turn type
	if roots[0].Children[0].Role != "system" {
		t.Errorf("first child role = %q, want system", roots[0].Children[0].Role)
	}
	if roots[0].Children[0].TurnType != "scaffold_change" {
		t.Errorf("system node turn type = %q, want scaffold_change", roots[0].Children[0].TurnType)
	}
	if roots[0].Children[1].Role != "coach" {
		t.Errorf("second child role = %q, want coach", roots[0].Children[1].Role)
	}
}

func TestBuildInquiryTree_ConsecutiveStudentMessages(t *testing.T) {
	// Two student messages in a row at root level should create separate roots
	interactions := []model.Interaction{
		makeInteractionWithID(1, "student", "第一个问题", "general_concept_socratic", 0),
		makeInteractionWithID(2, "student", "第二个问题", "general_concept_socratic", 1),
	}

	roots, _ := buildInquiryTree(interactions, nil)
	if len(roots) != 2 {
		t.Errorf("consecutive root-level student messages should create 2 roots, got %d", len(roots))
	}
}

func TestBuildInquiryTree_ConsecutiveCoachMessages(t *testing.T) {
	interactions := []model.Interaction{
		makeInteractionWithID(1, "student", "什么是力？", "general_concept_socratic", 0),
		makeInteractionWithID(2, "coach", "第一个回复", "general_concept_socratic", 1),
		makeInteractionWithID(3, "coach", "补充说明", "general_concept_socratic", 2),
	}

	roots, _ := buildInquiryTree(interactions, nil)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	// Both coach messages should be children
	if len(roots[0].Children) != 2 {
		t.Errorf("consecutive coach messages should be siblings, got %d children", len(roots[0].Children))
	}
}

func TestBuildInquiryTree_NilRedactor(t *testing.T) {
	interactions := []model.Interaction{
		makeInteractionWithID(1, "student", "我的手机号是13800138000", "", 0),
	}

	roots, _ := buildInquiryTree(interactions, nil)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	// With nil redactor, content should be unchanged
	if roots[0].Content != "我的手机号是13800138000" {
		t.Error("content should be unchanged with nil redactor")
	}
}

// -- InquiryTreeNode Structure Tests --------------------------

func TestBuildInquiryTree_NodeFields(t *testing.T) {
	interactions := []model.Interaction{
		makeInteractionWithID(42, "student", "测试内容", "general_concept_socratic", 5),
	}

	roots, _ := buildInquiryTree(interactions, nil)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}

	node := roots[0]
	if node.ID != 42 {
		t.Errorf("node.ID = %d, want 42", node.ID)
	}
	if node.Role != "student" {
		t.Errorf("node.Role = %q, want student", node.Role)
	}
	if node.Content != "测试内容" {
		t.Errorf("node.Content = %q, want 测试内容", node.Content)
	}
	if node.SkillID != "general_concept_socratic" {
		t.Errorf("node.SkillID = %q, want general_concept_socratic", node.SkillID)
	}
	if node.Depth != 0 {
		t.Errorf("node.Depth = %d, want 0", node.Depth)
	}
	if node.TurnType != "question" {
		t.Errorf("node.TurnType = %q, want question", node.TurnType)
	}
	if node.Time == "" {
		t.Error("node.Time should not be empty")
	}
}
