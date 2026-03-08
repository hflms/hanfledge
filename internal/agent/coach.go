package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"github.com/hflms/hanfledge/internal/plugin"
	"gorm.io/gorm"
)

var slogCoach = logger.L("Coach")

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

// cotReasoningDirective 是注入到系统 Prompt 的交错思考指令 (design.md §8.2.3)。
// 强制 Agent 在调用技能或生成回复前，先在 <reasoning> 标签内完成自检。
const cotReasoningDirective = `
【交错思考指令】
在你生成最终回复之前，你 **必须** 先在 <reasoning> 标签内完成以下自检：
1. 学生要解决的核心问题是什么？
2. 我应该使用哪种教学策略？为什么选它而非其他？
3. 当前的参考材料是否足够回答学生的问题？
4. 我的回复是否符合当前技能约束中的所有规则（不泄露答案、保持启发性）？
只有在推理完成后，才可以生成面向学生的回复。

格式要求：
<reasoning>
你的推理过程...
</reasoning>

然后是面向学生的正式回复。
`

// quickRepliesDirective 是减少学生输入负担的指令。
const quickRepliesDirective = `
【交互体验要求 (Quick Replies)】
为了减少学生的输入负担，如果你在当前回复的结尾提出了问题、给出了选项或期待学生的特定回复，
你 **必须** 在回复的最末尾提供 2-3 个可选的简短回复建议（每条建议不超过 15 个字）。
这些建议必须包裹在 <suggestions> 和 </suggestions> 标签之间，格式必须是合法的 JSON 字符串数组。
例如：
<suggestions>["选A，我觉得是光合作用", "还是不太懂，能再讲讲吗？", "我想跳过这部分"]</suggestions>
`

// CoachAgent 教练 Agent。
type CoachAgent struct {
	db          *gorm.DB
	llm         llm.LLMProvider
	registry    *plugin.Registry
	piiRedactor *safety.PIIRedactor
	cache       *cache.RedisCache // nil if Redis unavailable
}

// NewCoachAgent 创建教练 Agent。
func NewCoachAgent(db *gorm.DB, llmClient llm.LLMProvider, registry *plugin.Registry, piiRedactor *safety.PIIRedactor, redisCache *cache.RedisCache) *CoachAgent {
	return &CoachAgent{
		db:          db,
		llm:         llmClient,
		registry:    registry,
		piiRedactor: piiRedactor,
		cache:       redisCache,
	}
}

// Name 返回 Agent 名称。
func (a *CoachAgent) Name() string { return "Coach" }

// GenerateResponse 根据个性化材料生成初稿回复（流式）。
// onToken 回调用于逐 token 发送给前端。如果 onToken 为 nil，则仅静默累积。
// 这使得编排器可以控制何时流式输出（仅在最终被采纳的草稿时）。
func (a *CoachAgent) GenerateResponse(tc *TurnContext, material PersonalizedMaterial, onToken func(string)) (DraftResponse, error) {
	slogCoach.Info("generating response", "session_id", tc.SessionID)

	// Step 1: 加载技能约束
	skillID := material.Prescription.RecommendedSkill
	skillPrompt := a.loadSkillConstraints(skillID)

	// Step 2: 构建消息列表
	messages := a.buildMessages(tc, material, skillPrompt)

	// Step 2.5: PII 脱敏 — 在发送给 LLM 前替换用户消息中的个人信息
	messages = a.redactPII(messages, tc.SessionID)

	// Step 3: 调用 LLM（流式），使用 ModelRouter 路由 (§8.3.3)
	ctx := tc.Ctx
	var response string
	var err error

	if router, ok := a.llm.(*llm.ModelRouter); ok && tc.LLMTaskContext != nil {
		response, err = router.StreamChatWithContext(ctx, tc.LLMTaskContext, messages, &llm.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   1024,

			ProviderOverride: tc.ProviderOverride,
			ModelOverride:    tc.ModelOverride,
		}, onToken)
	} else {
		response, err = a.llm.StreamChat(ctx, messages, &llm.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   1024,

			ProviderOverride: tc.ProviderOverride,
			ModelOverride:    tc.ModelOverride,
		}, onToken)
	}
	if err != nil {
		return DraftResponse{}, fmt.Errorf("coach LLM call failed: %w", err)
	}

	// 估算 token 数（粗略：中文约 1 字 = 1 token）
	tokensUsed := estimateTokens(response)

	// 剥离 <reasoning> 块 — 不发送给学生 (§8.2.3)
	cleanResponse, reasoning := stripReasoningBlock(response)
	if reasoning != "" {
		slogCoach.Debug("CoT reasoning trace", "reasoning", truncate(reasoning, 100))
	}

	draft := DraftResponse{
		SessionID:     tc.SessionID,
		Content:       cleanResponse,
		SkillID:       skillID,
		ScaffoldLevel: material.Prescription.InitialScaffold,
		TokensUsed:    tokensUsed,
	}

	slogCoach.Debug("draft stats", "chars", len(cleanResponse), "tokens", tokensUsed)
	return draft, nil
}

// ReviseResponse 根据 Critic 反馈修订回复（流式）。
// onToken 回调用于逐 token 发送给前端。如果 onToken 为 nil，则仅静默累积。
func (a *CoachAgent) ReviseResponse(tc *TurnContext, material PersonalizedMaterial, review *ReviewResult, onToken func(string)) (DraftResponse, error) {
	slogCoach.Info("revising response based on critic feedback", "session_id", tc.SessionID)

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
	var response string
	var err error

	if router, ok := a.llm.(*llm.ModelRouter); ok && tc.LLMTaskContext != nil {
		response, err = router.StreamChatWithContext(ctx, tc.LLMTaskContext, messages, &llm.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   1024,

			ProviderOverride: tc.ProviderOverride,
			ModelOverride:    tc.ModelOverride,
		}, onToken)
	} else {
		response, err = a.llm.StreamChat(ctx, messages, &llm.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   1024,

			ProviderOverride: tc.ProviderOverride,
			ModelOverride:    tc.ModelOverride,
		}, onToken)
	}
	if err != nil {
		return DraftResponse{}, fmt.Errorf("coach revision LLM call failed: %w", err)
	}

	tokensUsed := estimateTokens(response)

	// 剥离 <reasoning> 块 — 不发送给学生 (§8.2.3)
	cleanResponse, _ := stripReasoningBlock(response)

	return DraftResponse{
		SessionID:     tc.SessionID,
		Content:       cleanResponse,
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
		slogCoach.Info("PII redacted before LLM call",
			"redacted_count", totalRedacted, "session_id", sessionID)
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
		slogCoach.Warn("load skill constraints failed", "skill_id", skillID, "err", err)
		return ""
	}

	return fmt.Sprintf("\n【技能约束（%s）】\n%s\n", skillID, constraints.RawMarkdown)
}

// buildMessages 构建 LLM 消息列表。
func (a *CoachAgent) buildMessages(tc *TurnContext, material PersonalizedMaterial, skillPrompt string) []llm.ChatMessage {
	// 系统 Prompt = Designer 组装的 + 技能约束 + CoT 推理指令 (§8.2.3)
	systemContent := material.SystemPrompt
	if skillPrompt != "" {
		systemContent += "\n" + skillPrompt
	}

	// 技能会话状态注入
	skillID := material.Prescription.RecommendedSkill

	// 谬误侦探技能: 注入会话状态上下文 (§5.2 Step 2, item 5)
	if isFallacyDetectiveActive(skillID) {
		state := a.loadFallacyState(tc.SessionID)
		fallacyCtx := buildFallacyContext(state, material.Misconceptions)
		systemContent += fallacyCtx
	}

	// 角色扮演技能: 注入角色状态上下文
	if isRolePlayActive(skillID) {
		state := a.loadRolePlayState(tc.SessionID)
		rolePlayCtx := buildRolePlayContext(state)
		systemContent += rolePlayCtx
	}

	// 自动出题技能: 注入出题状态上下文 (§7.13)
	if isQuizActive(skillID) {
		state := a.loadQuizState(tc.SessionID)
		quizCtx := buildQuizContext(state)
		systemContent += quizCtx
	}

	// 学情问卷诊断技能: 注入问卷状态上下文
	if isSurveyActive(skillID) {
		state := a.loadSurveyState(tc.SessionID)
		surveyCtx := buildSurveyContext(state)
		systemContent += surveyCtx
	}

	// 交错思考 (Interleaved Thinking) — 强制 <reasoning> 块 (§8.2.3)
	systemContent += "\n" + cotReasoningDirective

	// 动态快捷回复 (Quick Replies) — 注入建议标签指令
	systemContent += "\n" + quickRepliesDirective

	messages := []llm.ChatMessage{
		{Role: "system", Content: systemContent},
	}

	// 加载历史对话（最近 10 轮）
	history := a.loadHistory(tc.Ctx, tc.SessionID, 10)
	messages = append(messages, history...)

	// 如果有教师干预指令（Whisper），将其作为最高优先级的系统提示插入到历史之后
	if tc.TeacherWhisper != "" {
		messages = append(messages, llm.ChatMessage{
			Role:    "system",
			Content: "[来自教师的强制干预指令，必须优先遵守] " + tc.TeacherWhisper,
		})
	}

	// 当前用户输入 (只有非空才追加，因为如果是 whisper 触发，可能没有用户输入)
	if tc.UserInput != "" {
		messages = append(messages, llm.ChatMessage{
			Role:    "user",
			Content: tc.UserInput,
		})
	}

	return messages
}

// loadHistory 加载会话历史交互记录（cache-first, DB fallback）。
func (a *CoachAgent) loadHistory(ctx context.Context, sessionID uint, limit int) []llm.ChatMessage {
	// 尝试从 Redis 缓存读取
	if a.cache != nil {
		cached, err := a.cache.GetSessionHistory(ctx, sessionID)
		if err != nil {
			slogCoach.Warn("cache get history failed", "session_id", sessionID, "err", err)
		} else if cached != nil && len(cached) > 0 {
			messages := make([]llm.ChatMessage, 0, len(cached))
			for _, cm := range cached {
				messages = append(messages, llm.ChatMessage{
					Role:    cm.Role,
					Content: cm.Content,
				})
			}
			slogCoach.Debug("cache hit session history", "session_id", sessionID, "messages", len(messages))
			return messages
		}
	}

	// Cache miss → 从数据库加载
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

	// 回填缓存
	if a.cache != nil && len(messages) > 0 {
		cached := make([]cache.CachedMessage, len(messages))
		for i, m := range messages {
			cached[i] = cache.CachedMessage{Role: m.Role, Content: m.Content}
		}
		if err := a.cache.AppendSessionHistory(ctx, sessionID, cached...); err != nil {
			slogCoach.Warn("cache backfill history failed", "session_id", sessionID, "err", err)
		}
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

// ── CoT Reasoning Support (§8.2.3) ─────────────────────────

// reasoningBlockRe 匹配 <reasoning>...</reasoning> 块（含换行符）。
var reasoningBlockRe = regexp.MustCompile(`(?s)<reasoning>\s*(.*?)\s*</reasoning>`)

// stripReasoningBlock 从 LLM 输出中剥离 <reasoning> 推理块。
// 返回 (面向学生的干净内容, 推理部分内容)。
// 推理内容仅用于日志和内部审查，不应发送给学生。
func stripReasoningBlock(response string) (string, string) {
	matches := reasoningBlockRe.FindStringSubmatch(response)
	if len(matches) < 2 {
		return response, ""
	}

	reasoning := matches[1]
	cleaned := reasoningBlockRe.ReplaceAllString(response, "")
	cleaned = strings.TrimSpace(cleaned)

	return cleaned, reasoning
}

// ── Fallacy Detective Session State (§5.2 Step 2, item 5) ───

// fallacyDetectiveIDs lists all valid skill IDs for the fallacy-detective skill.
var fallacyDetectiveIDs = map[string]bool{
	"general_assessment_fallacy": true,
	"fallacy-detective":          true, // backward compat
}

// isFallacyDetectiveActive 判断当前技能是否为谬误侦探。
