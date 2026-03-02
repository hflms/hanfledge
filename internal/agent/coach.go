package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hflms/hanfledge/internal/domain/model"
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
			ModelOverride: tc.ModelOverride,
		}, onToken)
	} else {
		response, err = a.llm.StreamChat(ctx, messages, &llm.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   1024,
		
			ProviderOverride: tc.ProviderOverride,
			ModelOverride: tc.ModelOverride,
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
			ModelOverride: tc.ModelOverride,
		}, onToken)
	} else {
		response, err = a.llm.StreamChat(ctx, messages, &llm.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   1024,
		
			ProviderOverride: tc.ProviderOverride,
			ModelOverride: tc.ModelOverride,
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
func isFallacyDetectiveActive(skillID string) bool {
	return fallacyDetectiveIDs[skillID]
}

// loadFallacyState 从 StudentSession.SkillState 加载谬误侦探会话状态。
// 如果不存在或解析失败，返回初始状态。
func (a *CoachAgent) loadFallacyState(sessionID uint) FallacySessionState {
	var session model.StudentSession
	if err := a.db.Select("skill_state").First(&session, sessionID).Error; err != nil {
		slogCoach.Warn("load fallacy state failed", "session_id", sessionID, "err", err)
		return defaultFallacyState()
	}

	if session.SkillState == nil || *session.SkillState == "" || *session.SkillState == "null" {
		return defaultFallacyState()
	}

	var state FallacySessionState
	if err := json.Unmarshal([]byte(*session.SkillState), &state); err != nil {
		slogCoach.Warn("parse fallacy state failed", "session_id", sessionID, "err", err)
		return defaultFallacyState()
	}

	return state
}

// saveFallacyState 将谬误侦探会话状态保存到 StudentSession.SkillState。
func (a *CoachAgent) saveFallacyState(sessionID uint, state FallacySessionState) {
	data, err := json.Marshal(state)
	if err != nil {
		slogCoach.Warn("marshal fallacy state failed", "err", err)
		return
	}
	stateStr := string(data)
	if err := a.db.Model(&model.StudentSession{}).Where("id = ?", sessionID).
		Update("skill_state", stateStr).Error; err != nil {
		slogCoach.Warn("save fallacy state failed", "session_id", sessionID, "err", err)
	}
}

// defaultFallacyState 返回谬误侦探的初始会话状态。
func defaultFallacyState() FallacySessionState {
	return FallacySessionState{
		EmbeddedCount:   0,
		IdentifiedCount: 0,
		Phase:           FallacyPhasePresentTrap,
		MaxPerSession:   3, // 默认值，来自 metadata.json constraints.max_embedded_fallacies_per_session
	}
}

// buildFallacyContext 构建谬误侦探技能的额外系统上下文。
// 告知 LLM 当前的谬误嵌入进度和学生识别状态，使 LLM 能够正确推进流程。
func buildFallacyContext(state FallacySessionState, misconceptions []MisconceptionItem) string {
	var sb strings.Builder
	sb.WriteString("\n【谬误侦探会话状态】\n")
	sb.WriteString(fmt.Sprintf("- 当前阶段: %s\n", fallacyPhaseLabel(state.Phase)))
	sb.WriteString(fmt.Sprintf("- 已嵌入谬误数: %d / %d\n", state.EmbeddedCount, state.MaxPerSession))
	sb.WriteString(fmt.Sprintf("- 学生已正确识别: %d\n", state.IdentifiedCount))

	if state.CurrentTrapDesc != "" {
		sb.WriteString(fmt.Sprintf("- 当前嵌入的谬误: %s\n", state.CurrentTrapDesc))
	}

	// 阶段指令
	switch state.Phase {
	case FallacyPhasePresentTrap:
		if state.EmbeddedCount >= state.MaxPerSession {
			sb.WriteString("\n注意：本会话已达到最大谬误数，不要再嵌入新的谬误。直接进行总结。\n")
		} else {
			sb.WriteString("\n指令：请在接下来的讲解中巧妙嵌入一个学科常见误区。" +
				"嵌入后，系统将进入等待学生识别阶段。\n")
		}
	case FallacyPhaseAwaiting:
		sb.WriteString("\n指令：学生正在尝试识别谬误。" +
			"评估学生的回答是否准确定位了谬误。" +
			"如果学生正确识别，进入揭示阶段。" +
			"如果学生未能识别，根据支架等级给予适当提示，但不要直接揭露答案。\n")
	case FallacyPhaseRevealed:
		sb.WriteString("\n指令：学生已识别谬误。" +
			"请揭示这个谬误的设计意图，解释为什么它是一个常见误区，" +
			"以及在真实考试中它可能以什么形式出现。" +
			"揭示完成后，如果未达到最大谬误数，准备嵌入下一个谬误。\n")
	}

	return sb.String()
}

// fallacyPhaseLabel 将阶段枚举转换为中文标签。
func fallacyPhaseLabel(phase FallacyPhase) string {
	switch phase {
	case FallacyPhasePresentTrap:
		return "展示陷阱"
	case FallacyPhaseAwaiting:
		return "等待识别"
	case FallacyPhaseRevealed:
		return "已揭示"
	default:
		return string(phase)
	}
}

// advanceFallacyPhase 根据交互结果推进谬误侦探的阶段状态。
// 在 orchestrator 的 HandleTurn 完成后调用。
//
// 状态转换:
//
//	present_trap → awaiting  (Coach 输出含谬误的讲解后)
//	awaiting     → revealed  (学生正确识别后)
//	awaiting     → awaiting  (学生未能识别，保持等待)
//	revealed     → present_trap (准备下一个谬误)
func (a *CoachAgent) AdvanceFallacyPhase(sessionID uint, studentIdentified bool) {
	state := a.loadFallacyState(sessionID)

	switch state.Phase {
	case FallacyPhasePresentTrap:
		// Coach 刚输出了含谬误的讲解 → 进入等待识别
		state.EmbeddedCount++
		state.Phase = FallacyPhaseAwaiting
		slogCoach.Info("trap presented, awaiting identification",
			"session_id", sessionID, "embedded", state.EmbeddedCount, "max", state.MaxPerSession)

	case FallacyPhaseAwaiting:
		if studentIdentified {
			// 学生正确识别 → 进入揭示阶段
			state.IdentifiedCount++
			state.Phase = FallacyPhaseRevealed
			slogCoach.Info("student identified trap",
				"session_id", sessionID, "identified", state.IdentifiedCount, "embedded", state.EmbeddedCount)
		} else {
			slogCoach.Info("student did not identify, staying in awaiting",
				"session_id", sessionID)
		}

	case FallacyPhaseRevealed:
		// 揭示完成 → 回到展示陷阱（如果还有配额）
		state.CurrentTrapDesc = ""
		if state.EmbeddedCount < state.MaxPerSession {
			state.Phase = FallacyPhasePresentTrap
			slogCoach.Info("reveal complete, ready for next trap", "session_id", sessionID)
		} else {
			slogCoach.Info("all traps completed",
				"session_id", sessionID, "identified", state.IdentifiedCount, "embedded", state.EmbeddedCount)
		}
	}

	a.saveFallacyState(sessionID, state)
}

// ── Role-Play Session State ────────────────────────────────

// rolePlayIDs lists all valid skill IDs for the role-play skill.
var rolePlayIDs = map[string]bool{
	"general_review_roleplay": true,
	"role-play":               true, // backward compat
}

// isRolePlayActive 判断当前技能是否为角色扮演。
func isRolePlayActive(skillID string) bool {
	return rolePlayIDs[skillID]
}

// loadRolePlayState 从 StudentSession.SkillState 加载角色扮演会话状态。
// 如果不存在或解析失败，返回初始状态。
func (a *CoachAgent) loadRolePlayState(sessionID uint) RolePlaySessionState {
	var session model.StudentSession
	if err := a.db.Select("skill_state").First(&session, sessionID).Error; err != nil {
		slogCoach.Warn("load role-play state failed", "session_id", sessionID, "err", err)
		return defaultRolePlayState()
	}

	if session.SkillState == nil || *session.SkillState == "" || *session.SkillState == "null" {
		return defaultRolePlayState()
	}

	var state RolePlaySessionState
	if err := json.Unmarshal([]byte(*session.SkillState), &state); err != nil {
		slogCoach.Warn("parse role-play state failed", "session_id", sessionID, "err", err)
		return defaultRolePlayState()
	}

	return state
}

// saveRolePlayState 将角色扮演会话状态保存到 StudentSession.SkillState。
func (a *CoachAgent) saveRolePlayState(sessionID uint, state RolePlaySessionState) {
	data, err := json.Marshal(state)
	if err != nil {
		slogCoach.Warn("marshal role-play state failed", "err", err)
		return
	}
	stateStr := string(data)
	if err := a.db.Model(&model.StudentSession{}).Where("id = ?", sessionID).
		Update("skill_state", stateStr).Error; err != nil {
		slogCoach.Warn("save role-play state failed", "session_id", sessionID, "err", err)
	}
}

// defaultRolePlayState 返回角色扮演的初始会话状态。
func defaultRolePlayState() RolePlaySessionState {
	return RolePlaySessionState{
		ScenarioSwitches: 0,
		MaxSwitches:      3, // 来自 metadata.json constraints.max_scenario_switches_per_session
		Active:           true,
	}
}

// buildRolePlayContext 构建角色扮演技能的额外系统上下文。
// 告知 LLM 当前的角色身份和情境状态，使 LLM 能够维持角色一致性。
func buildRolePlayContext(state RolePlaySessionState) string {
	var sb strings.Builder
	sb.WriteString("\n【角色扮演会话状态】\n")

	if state.CharacterName != "" {
		sb.WriteString(fmt.Sprintf("- 当前角色: %s（%s）\n", state.CharacterName, state.CharacterRole))
	} else {
		sb.WriteString("- 当前角色: 尚未选定（请根据学科和知识点选择一个合适的角色）\n")
	}

	if state.ScenarioDesc != "" {
		sb.WriteString(fmt.Sprintf("- 当前情境: %s\n", state.ScenarioDesc))
	}

	sb.WriteString(fmt.Sprintf("- 已切换情境: %d / %d 次\n", state.ScenarioSwitches, state.MaxSwitches))
	sb.WriteString(fmt.Sprintf("- 角色状态: %s\n", rolePlayActiveLabel(state.Active)))

	// 状态指令
	if !state.Active {
		sb.WriteString("\n指令：学生已请求退出角色扮演。请以角色身份做简短告别，" +
			"然后切换回导师视角，总结本次扮演中涉及的知识点和学生表现亮点。\n")
	} else if state.CharacterName == "" {
		sb.WriteString("\n指令：这是角色扮演的第一轮。请根据当前学科和知识点，" +
			"选择一个合适的角色身份，简要介绍自己并设定情境，然后以角色视角展开对话。\n")
	} else if state.ScenarioSwitches >= state.MaxSwitches {
		sb.WriteString("\n注意：本会话已达到最大情境切换次数，请保持当前角色和情境继续对话。\n")
	} else {
		sb.WriteString("\n指令：请继续以当前角色身份与学生对话，" +
			"在对话中自然融入知识点。保持角色一致性。\n")
	}

	return sb.String()
}

// rolePlayActiveLabel 将活跃状态转换为中文标签。
func rolePlayActiveLabel(active bool) string {
	if active {
		return "沉浸中"
	}
	return "已退出"
}

// ── Quiz Generation Session State (§7.13) ───────────────────

// quizIDs lists all valid skill IDs for the quiz-generation skill.
var quizIDs = map[string]bool{
	"general_assessment_quiz": true,
	"quiz-generation":         true, // backward compat
}

// isQuizActive 判断当前技能是否为自动出题。
func isQuizActive(skillID string) bool {
	return quizIDs[skillID]
}

// loadQuizState 从 StudentSession.SkillState 加载自动出题会话状态。
// 如果不存在或解析失败，返回初始状态。
func (a *CoachAgent) loadQuizState(sessionID uint) QuizSessionState {
	var session model.StudentSession
	if err := a.db.Select("skill_state").First(&session, sessionID).Error; err != nil {
		slogCoach.Warn("load quiz state failed", "session_id", sessionID, "err", err)
		return defaultQuizState()
	}

	if session.SkillState == nil || *session.SkillState == "" || *session.SkillState == "null" {
		return defaultQuizState()
	}

	var state QuizSessionState
	if err := json.Unmarshal([]byte(*session.SkillState), &state); err != nil {
		slogCoach.Warn("parse quiz state failed", "session_id", sessionID, "err", err)
		return defaultQuizState()
	}

	return state
}

// saveQuizState 将自动出题会话状态保存到 StudentSession.SkillState。
func (a *CoachAgent) saveQuizState(sessionID uint, state QuizSessionState) {
	data, err := json.Marshal(state)
	if err != nil {
		slogCoach.Warn("marshal quiz state failed", "err", err)
		return
	}
	stateStr := string(data)
	if err := a.db.Model(&model.StudentSession{}).Where("id = ?", sessionID).
		Update("skill_state", stateStr).Error; err != nil {
		slogCoach.Warn("save quiz state failed", "session_id", sessionID, "err", err)
	}
}

// defaultQuizState 返回自动出题的初始会话状态。
func defaultQuizState() QuizSessionState {
	return QuizSessionState{
		Phase:       QuizPhaseGenerating,
		BatchCount:  0,
		MaxPerBatch: 5, // 来自 metadata.json constraints.max_questions_per_batch
	}
}

// buildQuizContext 构建自动出题技能的额外系统上下文。
// 告知 LLM 当前的出题进度和阶段，使 LLM 能够正确推进流程。
func buildQuizContext(state QuizSessionState) string {
	var sb strings.Builder
	sb.WriteString("\n【自动出题会话状态】\n")
	sb.WriteString(fmt.Sprintf("- 当前阶段: %s\n", quizPhaseLabel(state.Phase)))
	sb.WriteString(fmt.Sprintf("- 已生成批次: %d\n", state.BatchCount))
	sb.WriteString(fmt.Sprintf("- 累计题目数: %d\n", state.TotalQuestions))
	sb.WriteString(fmt.Sprintf("- 累计答对数: %d\n", state.CorrectCount))
	if state.TotalQuestions > 0 {
		accuracy := float64(state.CorrectCount) / float64(state.TotalQuestions) * 100
		sb.WriteString(fmt.Sprintf("- 正确率: %.0f%%\n", accuracy))
	}

	// 阶段指令
	switch state.Phase {
	case QuizPhaseGenerating:
		sb.WriteString(fmt.Sprintf("\n指令：请根据当前知识点和学生掌握度，生成一批题目（最多 %d 道）。\n", state.MaxPerBatch))
		sb.WriteString("题目必须以 <quiz>JSON</quiz> 格式输出，包含 mcq_single、mcq_multiple 或 fill_blank 类型。\n")
		sb.WriteString("在 JSON 之前，可以简短地介绍本次测验的主题。\n")
	case QuizPhaseAnswering:
		sb.WriteString("\n指令：学生正在作答。等待学生提交答案。\n")
		sb.WriteString("如果学生提问或请求提示，根据支架等级给予适当引导，但不要透露答案。\n")
	case QuizPhaseGrading:
		sb.WriteString("\n指令：请根据学生提交的答案逐题批改。\n")
		sb.WriteString("对每道题标注正误，对错误的题目解释原因，对正确的给予肯定。\n")
		sb.WriteString("最后汇总得分并给出学习建议。\n")
	case QuizPhaseReviewing:
		sb.WriteString("\n指令：批改已完成。如果学生要求继续出题，可以生成新一批题目。\n")
		sb.WriteString("如果学生有疑问，详细解答。\n")
	}

	return sb.String()
}

// quizPhaseLabel 将阶段枚举转换为中文标签。
func quizPhaseLabel(phase QuizPhase) string {
	switch phase {
	case QuizPhaseGenerating:
		return "生成题目"
	case QuizPhaseAnswering:
		return "等待作答"
	case QuizPhaseGrading:
		return "批改中"
	case QuizPhaseReviewing:
		return "查看结果"
	default:
		return string(phase)
	}
}

// AdvanceQuizPhase 根据交互结果推进自动出题的阶段状态。
// 在 orchestrator 的 HandleTurn 完成后调用。
//
// 状态转换:
//
//	generating → answering  (Coach 输出含题目的回复后)
//	answering  → grading    (学生提交答案后)
//	grading    → reviewing  (批改完成后)
//	reviewing  → generating (学生请求继续出题)
func (a *CoachAgent) AdvanceQuizPhase(sessionID uint, questionsGenerated, correctAnswers int) {
	state := a.loadQuizState(sessionID)

	switch state.Phase {
	case QuizPhaseGenerating:
		// Coach 输出了题目 → 进入等待作答
		if questionsGenerated > 0 {
			state.BatchCount++
			state.TotalQuestions += questionsGenerated
			state.Phase = QuizPhaseAnswering
			slogCoach.Info("questions generated, awaiting answers",
				"session_id", sessionID, "questions", questionsGenerated, "batch", state.BatchCount)
		}

	case QuizPhaseAnswering:
		// 学生提交答案 → 进入批改
		state.Phase = QuizPhaseGrading
		slogCoach.Info("student submitted answers, grading", "session_id", sessionID)

	case QuizPhaseGrading:
		// 批改完成 → 进入查看结果
		state.CorrectCount += correctAnswers
		state.Phase = QuizPhaseReviewing
		slogCoach.Info("grading complete",
			"session_id", sessionID, "correct", correctAnswers, "total_correct", state.CorrectCount, "total_questions", state.TotalQuestions)

	case QuizPhaseReviewing:
		// 学生请求继续 → 回到生成阶段
		state.Phase = QuizPhaseGenerating
		slogCoach.Info("student requests more questions", "session_id", sessionID)
	}

	a.saveQuizState(sessionID, state)
}

// ── Learning Survey Session State ───────────────────────────

// surveyIDs lists all valid skill IDs for the learning-survey skill.
var surveyIDs = map[string]bool{
	"general_diagnosis_survey": true,
	"learning-survey":          true, // backward compat
}

// isSurveyActive 判断当前技能是否为学情问卷诊断。
func isSurveyActive(skillID string) bool {
	return surveyIDs[skillID]
}

// loadSurveyState 从 StudentSession.SkillState 加载学情问卷会话状态。
// 如果不存在或解析失败，返回初始状态。
func (a *CoachAgent) loadSurveyState(sessionID uint) SurveySessionState {
	var session model.StudentSession
	if err := a.db.Select("skill_state").First(&session, sessionID).Error; err != nil {
		slogCoach.Warn("load survey state failed", "session_id", sessionID, "err", err)
		return defaultSurveyState()
	}

	if session.SkillState == nil || *session.SkillState == "" || *session.SkillState == "null" {
		return defaultSurveyState()
	}

	var state SurveySessionState
	if err := json.Unmarshal([]byte(*session.SkillState), &state); err != nil {
		slogCoach.Warn("parse survey state failed", "session_id", sessionID, "err", err)
		return defaultSurveyState()
	}

	return state
}

// saveSurveyState 将学情问卷会话状态保存到 StudentSession.SkillState。
func (a *CoachAgent) saveSurveyState(sessionID uint, state SurveySessionState) {
	data, err := json.Marshal(state)
	if err != nil {
		slogCoach.Warn("marshal survey state failed", "err", err)
		return
	}
	stateStr := string(data)
	if err := a.db.Model(&model.StudentSession{}).Where("id = ?", sessionID).
		Update("skill_state", stateStr).Error; err != nil {
		slogCoach.Warn("save survey state failed", "session_id", sessionID, "err", err)
	}
}

// defaultSurveyState 返回学情问卷的初始会话状态。
func defaultSurveyState() SurveySessionState {
	return SurveySessionState{
		Phase:           SurveyPhaseWelcome,
		CompletedDims:   []string{},
		TotalDimensions: 6, // learning_style, prior_knowledge, motivation, self_efficacy, study_habits, subject_interest
	}
}

// buildSurveyContext 构建学情问卷诊断技能的额外系统上下文。
// 告知 LLM 当前的问卷进度和阶段，使 LLM 能够正确推进流程。
func buildSurveyContext(state SurveySessionState) string {
	var sb strings.Builder
	sb.WriteString("\n【学情问卷会话状态】\n")
	sb.WriteString(fmt.Sprintf("- 当前阶段: %s\n", surveyPhaseLabel(state.Phase)))
	sb.WriteString(fmt.Sprintf("- 已完成维度: %d / %d\n", len(state.CompletedDims), state.TotalDimensions))
	if len(state.CompletedDims) > 0 {
		sb.WriteString(fmt.Sprintf("- 已完成: %s\n", strings.Join(state.CompletedDims, ", ")))
	}
	if state.CurrentDimension != "" {
		sb.WriteString(fmt.Sprintf("- 当前维度: %s\n", surveyDimensionLabel(state.CurrentDimension)))
	}
	sb.WriteString(fmt.Sprintf("- 累计提问数: %d\n", state.TotalQuestions))
	sb.WriteString(fmt.Sprintf("- 累计回答数: %d\n", state.TotalAnswered))

	// 阶段指令
	switch state.Phase {
	case SurveyPhaseWelcome:
		sb.WriteString("\n指令：这是问卷的开始。请简短自我介绍，说明问卷的目的，" +
			"消除学生的紧张感，强调没有对错之分。\n" +
			"然后推送第一个维度（learning_style）的问卷题目。\n")
	case SurveyPhaseSurveying:
		remaining := allSurveyDimensions(state.CompletedDims)
		if len(remaining) > 0 {
			sb.WriteString(fmt.Sprintf("\n指令：请对学生的上一批回答做简短反馈，"+
				"然后推送下一个维度（%s）的问卷题目。\n", remaining[0]))
			sb.WriteString("问题以 <survey>JSON</survey> 格式输出。\n")
			sb.WriteString(fmt.Sprintf("待完成维度: %s\n", strings.Join(remaining, ", ")))
		} else {
			sb.WriteString("\n指令：所有维度的问卷已完成。请告知学生问卷结束，" +
				"进入分析阶段。\n")
		}
	case SurveyPhaseAnalyzing:
		sb.WriteString("\n指令：所有问卷回答已收集完毕。请汇总分析学生的回答，" +
			"告知学生正在为其生成学习画像。\n")
	case SurveyPhaseReporting:
		sb.WriteString("\n指令：请以 <survey_profile>JSON</survey_profile> 格式输出完整的学习画像。\n" +
			"然后用通俗易懂的语言向学生解释每个维度的诊断结果。\n" +
			"强调优势，对薄弱点提出积极的改进建议。\n")
	case SurveyPhasePlanning:
		sb.WriteString("\n指令：请基于学习画像，以 <learning_plan>JSON</learning_plan> 格式输出学习建议方案。\n" +
			"推荐适合学生的学习策略和技能，给出具体可行的学习建议。\n")
	}

	return sb.String()
}

// surveyPhaseLabel 将阶段枚举转换为中文标签。
func surveyPhaseLabel(phase SurveyPhase) string {
	switch phase {
	case SurveyPhaseWelcome:
		return "欢迎介绍"
	case SurveyPhaseSurveying:
		return "问卷进行中"
	case SurveyPhaseAnalyzing:
		return "分析中"
	case SurveyPhaseReporting:
		return "生成画像"
	case SurveyPhasePlanning:
		return "制定方案"
	default:
		return string(phase)
	}
}

// surveyDimensionLabel 将维度 ID 转换为中文标签。
func surveyDimensionLabel(dim string) string {
	labels := map[string]string{
		"learning_style":   "学习风格",
		"prior_knowledge":  "前置知识",
		"motivation":       "学习动机",
		"self_efficacy":    "自我效能感",
		"study_habits":     "学习习惯",
		"subject_interest": "学科兴趣",
	}
	if label, ok := labels[dim]; ok {
		return label
	}
	return dim
}

// allSurveyDimensions 返回尚未完成的诊断维度列表（按预定顺序）。
func allSurveyDimensions(completed []string) []string {
	allDims := []string{
		"learning_style", "prior_knowledge", "motivation",
		"self_efficacy", "study_habits", "subject_interest",
	}
	done := make(map[string]bool, len(completed))
	for _, d := range completed {
		done[d] = true
	}
	remaining := make([]string, 0)
	for _, d := range allDims {
		if !done[d] {
			remaining = append(remaining, d)
		}
	}
	return remaining
}

// AdvanceSurveyPhase 根据交互结果推进学情问卷的阶段状态。
// 在 orchestrator 的 HandleTurn 完成后调用。
//
// 状态转换:
//
//	welcome    → surveying  (欢迎完成，开始第一个维度)
//	surveying  → surveying  (完成一个维度，继续下一个)
//	surveying  → analyzing  (所有维度完成)
//	analyzing  → reporting  (分析完成，生成画像)
//	reporting  → planning   (画像生成完成，制定方案)
//	planning   → planning   (方案已生成，保持)
func (a *CoachAgent) AdvanceSurveyPhase(sessionID uint, completedDimension string, questionsInBatch int) {
	state := a.loadSurveyState(sessionID)

	switch state.Phase {
	case SurveyPhaseWelcome:
		// 欢迎完成 → 进入问卷阶段
		state.Phase = SurveyPhaseSurveying
		if completedDimension != "" {
			state.CurrentDimension = completedDimension
		}
		if questionsInBatch > 0 {
			state.TotalQuestions += questionsInBatch
		}
		slogCoach.Info("survey welcome complete, starting survey",
			"session_id", sessionID)

	case SurveyPhaseSurveying:
		// 完成当前维度
		if completedDimension != "" {
			state.CompletedDims = appendUnique(state.CompletedDims, completedDimension)
			state.TotalAnswered += questionsInBatch
		}

		remaining := allSurveyDimensions(state.CompletedDims)
		if len(remaining) == 0 {
			// 所有维度完成 → 进入分析阶段
			state.Phase = SurveyPhaseAnalyzing
			state.CurrentDimension = ""
			slogCoach.Info("all survey dimensions complete, analyzing",
				"session_id", sessionID, "completed", len(state.CompletedDims))
		} else {
			// 还有维度未完成 → 继续下一个
			state.CurrentDimension = remaining[0]
			if questionsInBatch > 0 {
				state.TotalQuestions += questionsInBatch
			}
			slogCoach.Info("survey dimension complete, next dimension",
				"session_id", sessionID, "completed_dim", completedDimension, "next", remaining[0])
		}

	case SurveyPhaseAnalyzing:
		// 分析完成 → 进入报告阶段
		state.Phase = SurveyPhaseReporting
		slogCoach.Info("survey analysis complete, generating profile",
			"session_id", sessionID)

	case SurveyPhaseReporting:
		// 画像生成完成 → 进入规划阶段
		state.ProfileGenerated = true
		state.Phase = SurveyPhasePlanning
		slogCoach.Info("survey profile generated, planning",
			"session_id", sessionID)

	case SurveyPhasePlanning:
		// 方案已生成
		state.PlanGenerated = true
		slogCoach.Info("survey plan generated",
			"session_id", sessionID)
	}

	a.saveSurveyState(sessionID, state)
}

// appendUnique 向切片中追加不重复的元素。
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
