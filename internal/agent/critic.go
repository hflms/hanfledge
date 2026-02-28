package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
)

// ============================
// Critic Agent — 审查者
// ============================
//
// 职责：Actor-Critic 苏格拉底循环中的 "Critic" 角色。
// 审查 Coach 的回复草稿，评估：
// 1. 答案泄露风险（是否直接给出了答案）
// 2. 启发深度（是否引导学生深入思考）
// 3. 内容安全性（是否包含暴力、自伤、不当内容）
//
// 输入：DraftResponse + PersonalizedMaterial
// 输出：ReviewResult（通过/驳回 + 反馈 + 分数）
//
// 安全策略：Fail-Closed
//   - LLM 调用失败 → 驳回（而非自动通过）
//   - JSON 解析失败 → 驳回（而非自动通过）
//   - 保护学生安全优先于服务可用性

// CriticAgent 审查者 Agent。
type CriticAgent struct {
	llm llm.LLMProvider
}

// NewCriticAgent 创建审查者 Agent。
func NewCriticAgent(llmClient llm.LLMProvider) *CriticAgent {
	return &CriticAgent{llm: llmClient}
}

// Name 返回 Agent 名称。
func (a *CriticAgent) Name() string { return "Critic" }

// Review 审查 Coach 的回复草稿。
// 使用 LLM 作为审查者，根据预设的苏格拉底评估模板打分。
func (a *CriticAgent) Review(ctx context.Context, draft DraftResponse, material PersonalizedMaterial) (ReviewResult, error) {
	log.Printf("🔍 [Critic] Reviewing draft for session=%d (%d chars)",
		draft.SessionID, len(draft.Content))

	// 构建审查 Prompt
	reviewPrompt := buildReviewPrompt(draft, material)

	messages := []llm.ChatMessage{
		{
			Role: "system",
			Content: `你是一位教学质量审查专家。你的职责是评估 AI 教练的回复质量和安全性。

你需要从以下四个维度评分（0.0 到 1.0）：

1. leakage_score（答案泄露分数）：0.0=完全没有泄露答案, 1.0=直接给出了完整答案
   - 检查回复中是否直接给出了问题的答案
   - 好的苏格拉底式教学应该引导学生自己得出答案

2. depth_score（启发深度分数）：0.0=完全没有引导, 1.0=深度引导思考
   - 是否使用了引导性提问
   - 是否鼓励学生多角度思考
   - 是否建立了知识之间的联系

3. safety_score（内容安全分数）：0.0=严重不安全, 1.0=完全安全
   - 是否包含暴力、自残、自杀相关内容
   - 是否包含色情或性暗示内容
   - 是否包含操控性或诱导性语言（如建立不当关系、鼓励逃避监督）
   - 是否包含危险操作指令（如化学配方、武器制作）
   - 是否完全脱离教育场景
   - 教育平台面向 K-12 学生，安全标准应极其严格

4. approved（是否通过审查）：
   - 通过条件：leakage_score < 0.3 且 depth_score > 0.5 且 safety_score > 0.9
   - 任何安全问题（safety_score <= 0.9）必须驳回
   - 未通过时，提供具体修改建议

请以纯 JSON 格式返回（不要返回其他内容）：
{
  "leakage_score": 0.0,
  "depth_score": 0.0,
  "safety_score": 1.0,
  "approved": true,
  "feedback": "审查意见...",
  "revision": "如果未通过，给出修改后的版本（可选）"
}`,
		},
		{
			Role:    "user",
			Content: reviewPrompt,
		},
	}

	response, err := a.llm.Chat(ctx, messages, &llm.ChatOptions{
		Temperature: 0.1, // 审查需要确定性
		MaxTokens:   512,
	})
	if err != nil {
		return ReviewResult{}, fmt.Errorf("critic LLM call failed: %w", err)
	}

	// 解析审查结果 (Fail-Closed: 解析失败 → 驳回)
	result, err := parseCriticResponse(response, draft.SessionID)
	if err != nil {
		log.Printf("🛡️  [Critic] Parse response failed, REJECTING draft (fail-closed): %v", err)
		return ReviewResult{
			SessionID:    draft.SessionID,
			Approved:     false,
			Feedback:     "审查解析失败，出于安全考虑驳回。请重新生成回复。",
			LeakageScore: 0.5,
			DepthScore:   0.0,
			SafetyScore:  0.0,
		}, nil
	}

	log.Printf("   → Critic: approved=%t leakage=%.2f depth=%.2f safety=%.2f",
		result.Approved, result.LeakageScore, result.DepthScore, result.SafetyScore)

	return result, nil
}

// ── Internal Helpers ────────────────────────────────────────

// buildReviewPrompt 构建审查 Prompt。
func buildReviewPrompt(draft DraftResponse, material PersonalizedMaterial) string {
	return fmt.Sprintf(`请审查以下 AI 教练的回复：

【教练回复】
%s

【当前支架等级】%s

【教学技能】%s

【学习处方摘要】
- 目标知识点数量: %d
- 前置知识差距: %v
- 推荐技能: %s

请根据苏格拉底式教学标准进行评审。`,
		draft.Content,
		draft.ScaffoldLevel,
		draft.SkillID,
		len(material.Prescription.TargetKPSequence),
		material.Prescription.PrereqGaps,
		material.Prescription.RecommendedSkill,
	)
}

// criticJSON 审查 LLM 返回的 JSON 结构。
type criticJSON struct {
	LeakageScore float64 `json:"leakage_score"`
	DepthScore   float64 `json:"depth_score"`
	SafetyScore  float64 `json:"safety_score"`
	Approved     bool    `json:"approved"`
	Feedback     string  `json:"feedback"`
	Revision     string  `json:"revision"`
}

// parseCriticResponse 解析 Critic LLM 的 JSON 响应。
func parseCriticResponse(response string, sessionID uint) (ReviewResult, error) {
	// 清理可能的 markdown 代码块
	cleaned := extractJSONFromResponse(response)

	var result criticJSON
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return ReviewResult{}, fmt.Errorf("parse critic JSON: %w", err)
	}

	// 校验分数范围
	result.LeakageScore = clamp(result.LeakageScore, 0.0, 1.0)
	result.DepthScore = clamp(result.DepthScore, 0.0, 1.0)
	result.SafetyScore = clamp(result.SafetyScore, 0.0, 1.0)

	return ReviewResult{
		SessionID:    sessionID,
		Approved:     result.Approved,
		Feedback:     result.Feedback,
		LeakageScore: result.LeakageScore,
		DepthScore:   result.DepthScore,
		SafetyScore:  result.SafetyScore,
		Revision:     result.Revision,
	}, nil
}

// extractJSONFromResponse 从 LLM 响应中提取 JSON。
func extractJSONFromResponse(s string) string {
	// 尝试找到 JSON 起始
	start := -1
	end := -1
	for i, c := range s {
		if c == '{' && start == -1 {
			start = i
		}
		if c == '}' {
			end = i
		}
	}
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// clamp 将值限制在 [min, max] 范围内。
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
