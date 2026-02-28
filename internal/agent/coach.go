package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"github.com/hflms/hanfledge/internal/plugin"
	"gorm.io/gorm"
)

// ============================
// Coach Agent — 教练
// ============================
//
// 职责：直接面向学生进行多轮对话交互。
// 加载 SKILL.md 约束，生成符合教学规范的回复。
// 支持流式输出（T-4.4 将添加 streaming 支持）。
//
// Actor-Critic 循环中的 "Actor" 角色：
// 1. 根据 Designer 提供的材料生成初稿
// 2. 如果 Critic 驳回，根据反馈修订

// CoachAgent 教练 Agent。
type CoachAgent struct {
	db          *gorm.DB
	llm         *llm.OllamaClient
	registry    *plugin.Registry
	piiRedactor *safety.PIIRedactor
}

// NewCoachAgent 创建教练 Agent。
func NewCoachAgent(db *gorm.DB, llmClient *llm.OllamaClient, registry *plugin.Registry, piiRedactor *safety.PIIRedactor) *CoachAgent {
	return &CoachAgent{
		db:          db,
		llm:         llmClient,
		registry:    registry,
		piiRedactor: piiRedactor,
	}
}

// Name 返回 Agent 名称。
func (a *CoachAgent) Name() string { return "Coach" }

// GenerateResponse 根据个性化材料生成初稿回复。
// 1. 加载 SKILL.md 约束
// 2. 构建对话历史
// 3. 调用 LLM 生成回复
// 4. 流式输出 token（当前为非流式，T-4.4 将升级）
func (a *CoachAgent) GenerateResponse(tc *TurnContext, material PersonalizedMaterial) (DraftResponse, error) {
	log.Printf("🎓 [Coach] Generating response for session=%d", tc.SessionID)

	// Step 1: 加载技能约束
	skillID := material.Prescription.RecommendedSkill
	skillPrompt := a.loadSkillConstraints(skillID)

	// Step 2: 构建消息列表
	messages := a.buildMessages(tc, material, skillPrompt)

	// Step 2.5: PII 脱敏 — 在发送给 LLM 前替换用户消息中的个人信息
	messages = a.redactPII(messages, tc.SessionID)

	// Step 3: 调用 LLM（非流式 — T-4.4 将添加 ChatStream）
	ctx := tc.Ctx
	response, err := a.llm.Chat(ctx, messages, &llm.ChatOptions{
		Temperature: 0.7,
		TopP:        0.9,
		MaxTokens:   1024,
	})
	if err != nil {
		return DraftResponse{}, fmt.Errorf("coach LLM call failed: %w", err)
	}

	// Step 4: 通过回调发送 token（非流式模式下一次性发送）
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta(response)
	}

	// 估算 token 数（粗略：中文约 1 字 = 1 token）
	tokensUsed := estimateTokens(response)

	draft := DraftResponse{
		SessionID:     tc.SessionID,
		Content:       response,
		SkillID:       skillID,
		ScaffoldLevel: material.Prescription.InitialScaffold,
		TokensUsed:    tokensUsed,
	}

	log.Printf("   → Coach draft: %d chars, %d tokens", len(response), tokensUsed)
	return draft, nil
}

// ReviseResponse 根据 Critic 反馈修订回复。
func (a *CoachAgent) ReviseResponse(tc *TurnContext, material PersonalizedMaterial, review *ReviewResult) (DraftResponse, error) {
	log.Printf("🎓 [Coach] Revising response based on Critic feedback (session=%d)", tc.SessionID)

	// 在原始消息后追加 Critic 反馈
	skillID := material.Prescription.RecommendedSkill
	skillPrompt := a.loadSkillConstraints(skillID)
	messages := a.buildMessages(tc, material, skillPrompt)

	// 添加上一轮草稿和 Critic 反馈
	if tc.Draft != nil {
		messages = append(messages, llm.ChatMessage{
			Role:    "assistant",
			Content: tc.Draft.Content,
		})
	}

	messages = append(messages, llm.ChatMessage{
		Role: "user",
		Content: fmt.Sprintf(
			"[内部审查反馈] 你的上一版回复存在以下问题，请修改：\n%s\n\n"+
				"注意：\n- 答案泄露风险评分: %.2f（应低于 0.3）\n- 启发深度评分: %.2f（应高于 0.5）\n"+
				"请重新生成回复，避免直接给出答案，增加引导性提问。",
			review.Feedback, review.LeakageScore, review.DepthScore,
		),
	})

	// PII 脱敏
	messages = a.redactPII(messages, tc.SessionID)

	ctx := tc.Ctx
	response, err := a.llm.Chat(ctx, messages, &llm.ChatOptions{
		Temperature: 0.7,
		TopP:        0.9,
		MaxTokens:   1024,
	})
	if err != nil {
		return DraftResponse{}, fmt.Errorf("coach revision LLM call failed: %w", err)
	}

	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta(response)
	}

	tokensUsed := estimateTokens(response)

	return DraftResponse{
		SessionID:     tc.SessionID,
		Content:       response,
		SkillID:       skillID,
		ScaffoldLevel: material.Prescription.InitialScaffold,
		TokensUsed:    tokensUsed,
	}, nil
}

// ── Internal Helpers ────────────────────────────────────────

// redactPII 对发送给 LLM 的消息进行 PII 脱敏。
// 只脱敏 role=user 的消息内容，system 和 assistant 消息保持不变。
func (a *CoachAgent) redactPII(messages []llm.ChatMessage, sessionID uint) []llm.ChatMessage {
	if a.piiRedactor == nil {
		return messages
	}

	result := make([]llm.ChatMessage, len(messages))
	totalRedacted := 0

	for i, msg := range messages {
		result[i] = msg
		if msg.Role == "user" {
			redacted, count := a.piiRedactor.Redact(msg.Content)
			if count > 0 {
				result[i].Content = redacted
				totalRedacted += count
			}
		}
	}

	if totalRedacted > 0 {
		log.Printf("🛡️  [PII] Redacted %d PII items in session=%d before LLM call",
			totalRedacted, sessionID)
	}

	return result
}

// loadSkillConstraints 加载 SKILL.md 约束（注入到系统 Prompt）。
func (a *CoachAgent) loadSkillConstraints(skillID string) string {
	if skillID == "" || a.registry == nil {
		return ""
	}

	constraints, err := a.registry.LoadConstraints(skillID)
	if err != nil {
		log.Printf("⚠️  [Coach] Load SKILL.md for %s failed: %v", skillID, err)
		return ""
	}

	return fmt.Sprintf("\n【技能约束（%s）】\n%s\n", skillID, constraints.RawMarkdown)
}

// buildMessages 构建 LLM 消息列表。
func (a *CoachAgent) buildMessages(tc *TurnContext, material PersonalizedMaterial, skillPrompt string) []llm.ChatMessage {
	// 系统 Prompt = Designer 组装的 + 技能约束
	systemContent := material.SystemPrompt
	if skillPrompt != "" {
		systemContent += "\n" + skillPrompt
	}

	messages := []llm.ChatMessage{
		{Role: "system", Content: systemContent},
	}

	// 加载历史对话（最近 10 轮）
	history := a.loadHistory(tc.Ctx, tc.SessionID, 10)
	messages = append(messages, history...)

	// 当前用户输入
	messages = append(messages, llm.ChatMessage{
		Role:    "user",
		Content: tc.UserInput,
	})

	return messages
}

// loadHistory 加载会话历史交互记录。
func (a *CoachAgent) loadHistory(_ context.Context, sessionID uint, limit int) []llm.ChatMessage {
	type interaction struct {
		Role    string
		Content string
	}

	var interactions []interaction
	a.db.Raw(`
		SELECT role, content FROM interactions
		WHERE session_id = ?
		ORDER BY created_at ASC
		LIMIT ?
	`, sessionID, limit*2).Scan(&interactions) // *2 for student+coach pairs

	messages := make([]llm.ChatMessage, 0, len(interactions))
	for _, inter := range interactions {
		role := inter.Role
		if role == "student" {
			role = "user"
		} else if role == "coach" {
			role = "assistant"
		}
		messages = append(messages, llm.ChatMessage{
			Role:    role,
			Content: inter.Content,
		})
	}

	return messages
}

// estimateTokens 粗略估算 token 数（中文约 1.5 字 / token）。
func estimateTokens(text string) int {
	runes := []rune(text)
	// 粗略估算：中文字符 ~1.5 char/token, 英文 ~4 char/token
	chineseCount := 0
	englishCount := 0
	for _, r := range runes {
		if r > 127 {
			chineseCount++
		} else {
			englishCount++
		}
	}
	return int(float64(chineseCount)/1.5) + int(float64(englishCount)/4.0) + 1
}
