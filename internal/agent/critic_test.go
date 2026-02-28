package agent

import (
	"math"
	"testing"
)

// ── extractJSONFromResponse Tests ───────────────────────────

func TestExtractJSONFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"纯JSON",
			`{"approved": true, "leakage_score": 0.1}`,
			`{"approved": true, "leakage_score": 0.1}`,
		},
		{
			"JSON前有文本",
			`审查结果如下：{"approved": true, "feedback": "通过"}`,
			`{"approved": true, "feedback": "通过"}`,
		},
		{
			"JSON后有文本",
			`{"approved": false}以上是审查结果`,
			`{"approved": false}`,
		},
		{
			"JSON前后都有文本",
			`结果：{"score": 0.5}，请查看`,
			`{"score": 0.5}`,
		},
		{
			"多层嵌套JSON",
			`{"outer": {"inner": "value"}}`,
			`{"outer": {"inner": "value"}}`,
		},
		{
			"没有JSON原样返回",
			"没有JSON内容",
			"没有JSON内容",
		},
		{
			"空字符串",
			"",
			"",
		},
		{
			"Markdown代码块包裹",
			"```json\n{\"approved\": true}\n```",
			`{"approved": true}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractJSONFromResponse(tc.input)
			if result != tc.expected {
				t.Errorf("extractJSONFromResponse(%q) = %q, want %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

// ── clamp Tests ─────────────────────────────────────────────

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		v        float64
		min      float64
		max      float64
		expected float64
	}{
		{"值在范围内", 0.5, 0.0, 1.0, 0.5},
		{"值低于最小值", -0.5, 0.0, 1.0, 0.0},
		{"值高于最大值", 1.5, 0.0, 1.0, 1.0},
		{"值等于最小值", 0.0, 0.0, 1.0, 0.0},
		{"值等于最大值", 1.0, 0.0, 1.0, 1.0},
		{"负范围", -5.0, -10.0, -1.0, -5.0},
		{"超出负范围下限", -15.0, -10.0, -1.0, -10.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := clamp(tc.v, tc.min, tc.max)
			if result != tc.expected {
				t.Errorf("clamp(%f, %f, %f) = %f, want %f",
					tc.v, tc.min, tc.max, result, tc.expected)
			}
		})
	}
}

// ── parseCriticResponse Tests ───────────────────────────────

func TestParseCriticResponse(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		sessionID uint
		approved  bool
		leakage   float64
		depth     float64
		feedback  string
		wantError bool
	}{
		{
			"正常审查通过",
			`{"leakage_score": 0.1, "depth_score": 0.8, "approved": true, "feedback": "回复质量好"}`,
			1,
			true, 0.1, 0.8, "回复质量好",
			false,
		},
		{
			"审查未通过",
			`{"leakage_score": 0.7, "depth_score": 0.3, "approved": false, "feedback": "答案泄露严重", "revision": "建议修改"}`,
			2,
			false, 0.7, 0.3, "答案泄露严重",
			false,
		},
		{
			"分数超范围被clamp",
			`{"leakage_score": 1.5, "depth_score": -0.3, "approved": true, "feedback": "测试"}`,
			3,
			true, 1.0, 0.0, "测试",
			false,
		},
		{
			"非法JSON返回错误",
			`不是JSON`,
			4,
			false, 0, 0, "",
			true,
		},
		{
			"带前缀文本的JSON",
			`审查结果：{"leakage_score": 0.2, "depth_score": 0.6, "approved": true, "feedback": "可以"}`,
			5,
			true, 0.2, 0.6, "可以",
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseCriticResponse(tc.response, tc.sessionID)
			if tc.wantError {
				if err == nil {
					t.Errorf("parseCriticResponse() should return error for %q", tc.response)
				}
				return
			}
			if err != nil {
				t.Errorf("parseCriticResponse() returned unexpected error: %v", err)
				return
			}
			if result.SessionID != tc.sessionID {
				t.Errorf("SessionID = %d, want %d", result.SessionID, tc.sessionID)
			}
			if result.Approved != tc.approved {
				t.Errorf("Approved = %t, want %t", result.Approved, tc.approved)
			}
			if math.Abs(result.LeakageScore-tc.leakage) > 0.001 {
				t.Errorf("LeakageScore = %f, want %f", result.LeakageScore, tc.leakage)
			}
			if math.Abs(result.DepthScore-tc.depth) > 0.001 {
				t.Errorf("DepthScore = %f, want %f", result.DepthScore, tc.depth)
			}
			if result.Feedback != tc.feedback {
				t.Errorf("Feedback = %q, want %q", result.Feedback, tc.feedback)
			}
		})
	}
}

// ── buildReviewPrompt Tests ─────────────────────────────────

func TestBuildReviewPrompt(t *testing.T) {
	draft := DraftResponse{
		SessionID:     1,
		Content:       "这道题可以分步思考",
		SkillID:       "socratic_questioning",
		ScaffoldLevel: ScaffoldHigh,
	}
	material := PersonalizedMaterial{
		Prescription: LearningPrescription{
			TargetKPSequence: []KnowledgePointTarget{
				{KPID: 1},
				{KPID: 2},
			},
			PrereqGaps:       []string{"基础概念 (mastery=0.3)"},
			RecommendedSkill: "socratic_questioning",
		},
	}

	result := buildReviewPrompt(draft, material)

	// 验证 prompt 包含关键内容
	checks := []struct {
		keyword string
		desc    string
	}{
		{"这道题可以分步思考", "应包含教练回复内容"},
		{"high", "应包含支架等级"},
		{"socratic_questioning", "应包含教学技能"},
		{"2", "应包含目标知识点数量"},
		{"基础概念", "应包含前置知识差距"},
	}

	for _, check := range checks {
		found := false
		for i := 0; i < len(result)-len(check.keyword)+1; i++ {
			if result[i:i+len(check.keyword)] == check.keyword {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildReviewPrompt: %s，未找到 %q", check.desc, check.keyword)
		}
	}
}
