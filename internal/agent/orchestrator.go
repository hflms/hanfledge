package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"github.com/hflms/hanfledge/internal/infrastructure/search"
	"github.com/hflms/hanfledge/internal/plugin"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"github.com/hflms/hanfledge/internal/usecase"
	"gorm.io/gorm"
)

// ============================
// 多 Agent 编排引擎
// ============================

// AgentOrchestrator 管理多 Agent 的生命周期与通信。
// 采用同步管线模式: Strategist → Designer → Coach → Critic → Coach (retry loop)
type AgentOrchestrator struct {
	strategist *StrategistAgent
	designer   *DesignerAgent
	coach      *CoachAgent
	critic     *CriticAgent
	bkt        *BKTService

	// 依赖
	db          *gorm.DB
	llm         llm.LLMProvider
	neo4j       *neo4jRepo.Client
	karag       *usecase.KARAGEngine
	registry    *plugin.Registry
	eventBus    *plugin.EventBus         // Plugin event bus (nil-safe)
	cache       *cache.RedisCache        // nil if Redis unavailable
	outputGuard *safety.OutputGuard      // 输出安全审核器
	searchConn  *search.DynamicConnector // Web search fallback (§8.1.2, nil-safe)

	// Actor-Critic 最大重试轮数
	maxCriticRetries int
}

// NewAgentOrchestrator 创建一个新的 Agent 编排引擎。
func NewAgentOrchestrator(
	db *gorm.DB,
	llmClient llm.LLMProvider,
	neo4jClient *neo4jRepo.Client,
	karag *usecase.KARAGEngine,
	registry *plugin.Registry,
	eventBus *plugin.EventBus,
	piiRedactor *safety.PIIRedactor,
	redisCache *cache.RedisCache,
	outputGuard *safety.OutputGuard,
	searchConnector *search.DynamicConnector,
) *AgentOrchestrator {
	o := &AgentOrchestrator{
		db:          db,
		llm:         llmClient,
		neo4j:       neo4jClient,
		karag:       karag,
		registry:    registry,
		eventBus:    eventBus,
		cache:       redisCache,
		outputGuard: outputGuard,
		searchConn:  searchConnector,

		maxCriticRetries: 2,
	}

	// 初始化各 Agent
	o.strategist = NewStrategistAgent(db, neo4jClient, registry)
	o.designer = NewDesignerAgent(db, llmClient, neo4jClient, karag, searchConnector)
	o.coach = NewCoachAgent(db, llmClient, registry, piiRedactor, redisCache)
	o.critic = NewCriticAgent(llmClient)
	o.bkt = NewBKTService(db)

	log.Println("🎯 [Agent] Orchestrator initialized with 4 agents + BKT: Strategist, Designer, Coach, Critic")
	return o
}

// ── Pipeline Execution ──────────────────────────────────────

// HandleTurn 处理一轮学生对话，驱动整个 Agent 管道。
// 这是 WebSocket handler 调用的主入口。
//
// Pipeline (with L2+L3 caching, §8.1.3):
//
//	User Input → L2 Semantic Cache Check → L3 Output Cache Check
//	  → HIT: Return cached response immediately
//	  → MISS: Strategist → Designer → Coach → Critic → Save + Write Cache
func (o *AgentOrchestrator) HandleTurn(tc *TurnContext) error {
	ctx := tc.Ctx
	start := time.Now()

	log.Printf("🎯 [Agent] HandleTurn: session=%d student=%d input=%q",
		tc.SessionID, tc.StudentID, truncate(tc.UserInput, 50))

	// Hook: before student query
	o.publishEvent(ctx, plugin.HookBeforeStudentQuery, map[string]interface{}{
		"session_id":    tc.SessionID,
		"student_input": tc.UserInput,
		"user_id":       tc.StudentID,
	})

	// ── Pre-Stage: L2 Semantic Cache Check (§8.1.3) ─────
	if hit, err := o.checkSemanticCache(tc); err != nil {
		log.Printf("⚠️  [L2-Cache] Check failed: %v", err)
	} else if hit != nil {
		// Cache hit — return cached response directly
		log.Printf("⚡ [L2-Cache] HIT: similarity=%.4f, skipping full pipeline",
			hit.Similarity)
		return o.returnCachedResponse(tc, hit.Entry.Response, hit.Entry.SkillID, start)
	}

	// ── Stage 1: Strategist — 生成学习处方 ──────────────
	if tc.OnThinking != nil {
		tc.OnThinking("Strategist 正在分析学情...")
	}

	prescription, err := o.strategist.Analyze(ctx, tc.SessionID, tc.StudentID, tc.ActivityID)
	if err != nil {
		return fmt.Errorf("strategist failed: %w", err)
	}
	tc.Prescription = &prescription

	log.Printf("   → Strategist: %d KP targets, scaffold=%s, skill=%s",
		len(prescription.TargetKPSequence), prescription.InitialScaffold, prescription.RecommendedSkill)

	// Hook: after skill match
	o.publishEvent(ctx, plugin.HookAfterSkillMatch, map[string]interface{}{
		"session_id": tc.SessionID,
		"skill_id":   prescription.RecommendedSkill,
		"confidence": 1.0, // strategist rule-based, confidence=1
	})

	// ── Stage 2: Designer — 检索 + 组装个性化材料 ───────
	if tc.OnThinking != nil {
		tc.OnThinking("Designer 正在检索知识图谱...")
	}

	material, err := o.designer.Assemble(ctx, prescription, tc.UserInput)
	if err != nil {
		return fmt.Errorf("designer failed: %w", err)
	}
	tc.Material = &material

	log.Printf("   → Designer: %d chunks retrieved, %d graph nodes",
		len(material.RetrievedChunks), len(material.GraphContext))

	// ── Stage 2.5: Build TaskContext for ModelRouter (§8.3.3) ──
	tc.LLMTaskContext = o.buildTaskContext(tc, material)

	// ── Stage 3+4: Coach + Critic Actor-Critic 循环 ─────
	if tc.OnThinking != nil {
		tc.OnThinking("Coach 正在组织回复...")
	}

	// Hook: before LLM call (abortable — plugins may block the call)
	if err := o.publishAbortable(ctx, plugin.HookBeforeLLMCall, map[string]interface{}{
		"session_id":    tc.SessionID,
		"prompt_length": len(material.SystemPrompt) + len(tc.UserInput),
	}); err != nil {
		return fmt.Errorf("aborted by HookBeforeLLMCall: %w", err)
	}

	finalResponse, err := o.actorCriticLoop(tc, material)
	if err != nil {
		return fmt.Errorf("actor-critic loop failed: %w", err)
	}

	// Hook: after LLM response
	o.publishEvent(ctx, plugin.HookAfterLLMResponse, map[string]interface{}{
		"session_id":      tc.SessionID,
		"response_length": len(finalResponse.Content),
		"model":           "coach",
	})

	// ── Stage 4.5: Output Safety Guardrail (§4.1 Layer 3) ──
	finalResponse = o.checkOutputSafety(tc, finalResponse)

	// ── Stage 5: 持久化交互记录 ─────────────────────────
	if err := o.saveInteraction(tc, finalResponse); err != nil {
		log.Printf("⚠️  [Agent] Save interaction failed: %v", err)
	}

	// Hook: after student answer (interaction persisted)
	o.publishEvent(ctx, plugin.HookAfterStudentAnswer, map[string]interface{}{
		"session_id":    tc.SessionID,
		"student_id":    tc.StudentID,
		"student_input": tc.UserInput,
		"coach_output":  finalResponse.Content,
		"skill_id":      finalResponse.SkillID,
	})

	// ── Stage 5.5: Write L2+L3 Cache ────────────────────
	o.writeResponseToCache(tc, material, finalResponse)

	// ── Stage 6: BKT 掌握度更新 + 支架衰减 (skip in sandbox) ──
	if !tc.IsSandbox {
		o.updateMasteryAndFadeScaffold(tc)
	} else {
		log.Printf("🧪 [Sandbox] Skipping mastery update for sandbox session=%d", tc.SessionID)
	}

	// ── Stage 6.5: 谬误侦探阶段推进 (§5.2 Step 2, item 5) ──
	o.advanceFallacyPhaseIfActive(tc, finalResponse)

	// ── Stage 6.6: 角色扮演状态更新 ──
	o.updateRolePlayStateIfActive(tc, finalResponse)

	// ── Stage 6.7: 自动出题阶段推进 (§7.13) ──
	o.advanceQuizPhaseIfActive(tc, finalResponse)

	// ── Stage 7: 错题本自动归档 (§5.2 Step 3, item 3; skip in sandbox) ──
	if !tc.IsSandbox {
		o.archiveErrorIfIncorrect(tc, finalResponse)
	} else {
		log.Printf("🧪 [Sandbox] Skipping error notebook for sandbox session=%d", tc.SessionID)
	}

	elapsed := time.Since(start)
	log.Printf("✅ [Agent] Turn complete: session=%d tokens=%d elapsed=%s",
		tc.SessionID, finalResponse.TokensUsed, elapsed)

	if tc.OnTurnComplete != nil {
		tc.OnTurnComplete(finalResponse.TokensUsed)
	}

	return nil
}

// actorCriticLoop 执行 Coach-Critic 的 Actor-Critic 循环（支持流式输出）。
// 策略：非最终尝试静默缓冲，最终尝试流式输出到前端。
// - 非最终尝试（可能被 Critic 驳回）：onToken=nil，仅累积全文
// - 最终尝试（maxCriticRetries 或 Critic 通过）：通过 OnTokenDelta 实时流式输出
// - 如果非最终尝试被 Critic 通过，将缓冲的全文一次性发送给前端
func (o *AgentOrchestrator) actorCriticLoop(tc *TurnContext, material PersonalizedMaterial) (*DraftResponse, error) {
	var draft DraftResponse
	var err error

	for attempt := 0; attempt <= o.maxCriticRetries; attempt++ {
		// 决定是否流式输出：仅最终尝试（maxCriticRetries）流式输出
		isLastAttempt := attempt == o.maxCriticRetries
		var onToken func(string)
		if isLastAttempt && tc.OnTokenDelta != nil {
			onToken = tc.OnTokenDelta
		}

		// Coach 生成回复
		if attempt == 0 {
			draft, err = o.coach.GenerateResponse(tc, material, onToken)
		} else {
			// 基于 Critic 反馈修订
			review := tc.Review
			if tc.OnThinking != nil {
				tc.OnThinking(fmt.Sprintf("Coach 正在根据审查反馈修订 (第%d次)...", attempt))
			}
			draft, err = o.coach.ReviseResponse(tc, material, review, onToken)
		}
		if err != nil {
			return nil, fmt.Errorf("coach attempt %d failed: %w", attempt, err)
		}

		tc.Draft = &draft

		// 最后一轮不再审查，直接采用（已通过流式输出）
		if isLastAttempt {
			log.Printf("   → Actor-Critic: max retries reached, accepting draft")
			break
		}

		// Critic 审查
		if tc.OnThinking != nil {
			tc.OnThinking("Critic 正在审查回复质量...")
		}

		review, err := o.critic.Review(tc.Ctx, draft, material)
		if err != nil {
			log.Printf("⚠️  [Agent] Critic failed, accepting draft: %v", err)
			// Critic 失败 → 接受当前草稿，需要补发缓冲内容
			if tc.OnTokenDelta != nil {
				tc.OnTokenDelta(draft.Content)
			}
			break
		}

		tc.Review = &review

		log.Printf("   → Critic: approved=%t leakage=%.2f depth=%.2f (attempt %d)",
			review.Approved, review.LeakageScore, review.DepthScore, attempt+1)

		if review.Approved {
			// Critic 通过 → 将缓冲的全文一次性发送给前端
			if tc.OnTokenDelta != nil {
				tc.OnTokenDelta(draft.Content)
			}
			break
		}
	}

	return &draft, nil
}

// saveInteraction 持久化学生输入和 AI 回复到 interactions 表，并更新缓存。
func (o *AgentOrchestrator) saveInteraction(tc *TurnContext, response *DraftResponse) error {
	now := time.Now()

	// 保存学生输入
	studentMsg := model.Interaction{
		SessionID: tc.SessionID,
		Role:      "student",
		Content:   tc.UserInput,
		CreatedAt: now,
	}
	if err := o.db.WithContext(tc.Ctx).Create(&studentMsg).Error; err != nil {
		return fmt.Errorf("save student interaction: %w", err)
	}

	// 保存 Coach 回复
	coachMsg := model.Interaction{
		SessionID:  tc.SessionID,
		Role:       "coach",
		Content:    response.Content,
		SkillID:    response.SkillID,
		TokensUsed: response.TokensUsed,
		CreatedAt:  now,
	}
	if err := o.db.WithContext(tc.Ctx).Create(&coachMsg).Error; err != nil {
		return fmt.Errorf("save coach interaction: %w", err)
	}

	// 更新 Redis 缓存中的会话历史
	if o.cache != nil {
		if err := o.cache.AppendSessionHistory(tc.Ctx, tc.SessionID,
			cache.CachedMessage{Role: "user", Content: tc.UserInput},
			cache.CachedMessage{Role: "assistant", Content: response.Content},
		); err != nil {
			log.Printf("⚠️  [Cache] Append history session=%d failed: %v", tc.SessionID, err)
		}
	}

	return nil
}

// updateMasteryAndFadeScaffold 每轮交互后更新 BKT 掌握度并检查支架衰减。
// 支架衰减规则 (design.md §7.13):
//
//	mastery < 0.6  → high   (分步引导 + 关键词高亮)
//	mastery >= 0.6 → medium (关键词标签)
//	mastery >= 0.8 → low    (仅空白输入框)
//
// 当掌握度跨越阈值时:
//  1. 更新 StudentSession.Scaffold 字段
//  2. 通过 OnScaffold 回调发送 ui_scaffold_change 事件
func (o *AgentOrchestrator) updateMasteryAndFadeScaffold(tc *TurnContext) {
	// 获取当前会话信息
	var session model.StudentSession
	if err := o.db.WithContext(tc.Ctx).First(&session, tc.SessionID).Error; err != nil {
		log.Printf("⚠️  [Scaffold] Load session %d failed: %v", tc.SessionID, err)
		return
	}

	// 获取当前目标 KP（会话的 CurrentKP）
	kpID := session.CurrentKP
	if kpID == 0 && tc.Prescription != nil && len(tc.Prescription.TargetKPSequence) > 0 {
		kpID = tc.Prescription.TargetKPSequence[0].KPID
	}
	if kpID == 0 {
		log.Printf("⚠️  [Scaffold] No current KP for session %d, skipping mastery update", tc.SessionID)
		return
	}

	// 判断学生回答的正确性（基于 Critic 审查结果）
	// 如果 Critic 审核通过且深度分数较高，视为正确（学生理解了引导）
	correct := o.inferCorrectness(tc)

	// Capture old mastery for event delta
	oldMastery := o.bkt.GetMastery(tc.StudentID, kpID)

	// BKT 掌握度更新
	update, err := o.bkt.UpdateStudentMastery(tc.StudentID, kpID, correct)
	if err != nil {
		log.Printf("⚠️  [Scaffold] BKT update failed for student=%d kp=%d: %v",
			tc.StudentID, kpID, err)
		return
	}

	// Hook: after evaluation (correctness inferred + mastery updated)
	o.publishEvent(tc.Ctx, plugin.HookAfterEvaluation, map[string]interface{}{
		"session_id":   tc.SessionID,
		"student_id":   tc.StudentID,
		"knowledge_id": kpID,
		"correct":      correct,
		"old_mastery":  oldMastery,
		"new_mastery":  update.NewMastery,
	})

	// Hook: on mastery change
	if update.NewMastery != oldMastery {
		o.publishEvent(tc.Ctx, plugin.HookOnMasteryChange, map[string]interface{}{
			"session_id":   tc.SessionID,
			"user_id":      tc.StudentID,
			"knowledge_id": kpID,
			"old_mastery":  oldMastery,
			"new_mastery":  update.NewMastery,
		})
	}

	// 计算新的支架等级
	newScaffold := scaffoldForMastery(update.NewMastery)
	oldScaffold := session.Scaffold

	// 检查是否需要衰减
	if newScaffold != oldScaffold {
		log.Printf("🔄 [Scaffold] Fading: student=%d kp=%d mastery=%.3f scaffold %s→%s",
			tc.StudentID, kpID, update.NewMastery, oldScaffold, newScaffold)

		// 更新数据库中的会话支架等级
		if err := o.db.WithContext(tc.Ctx).Model(&model.StudentSession{}).Where("id = ?", tc.SessionID).
			Update("scaffold", newScaffold).Error; err != nil {
			log.Printf("⚠️  [Scaffold] Update session scaffold failed: %v", err)
			return
		}

		// 发送 ui_scaffold_change 事件到前端
		if tc.OnScaffold != nil {
			tc.OnScaffold("scaffold_change", map[string]interface{}{
				"old_level": string(oldScaffold),
				"new_level": string(newScaffold),
				"mastery":   update.NewMastery,
				"kp_id":     kpID,
				"direction": scaffoldDirection(oldScaffold, newScaffold),
			})
		}
	}
}

// inferCorrectness 从当前轮次的上下文推断学生回答的正确性。
// 使用 Critic 审查结果作为代理指标:
//   - depth_score >= 0.6 → 学生有参与深度思考 → 视为"正确"
//   - Critic 未通过或 depth_score 低 → 学生可能需要更多引导 → 视为"不正确"
//
// 这是一个启发式方法，后续可替换为更精确的评估模型。
func (o *AgentOrchestrator) inferCorrectness(tc *TurnContext) bool {
	if tc.Review != nil {
		// 基于 Critic 的深度分数判断
		return tc.Review.DepthScore >= 0.6
	}
	// 如果没有 Critic 审查（例如 Critic 失败），默认视为部分正确
	return true
}

// scaffoldDirection 返回支架变化方向。
func scaffoldDirection(old, new_ model.ScaffoldLevel) string {
	order := map[model.ScaffoldLevel]int{
		ScaffoldHigh:   3,
		ScaffoldMedium: 2,
		ScaffoldLow:    1,
	}
	if order[new_] < order[old] {
		return "fade" // 支架减弱（进步）
	}
	return "strengthen" // 支架增强（退步）
}

// ── Output Safety Guardrail (§4.1 Layer 3) ──────────────────

// checkOutputSafety 在 LLM 输出到达学生之前执行安全审核。
// 如果 outputGuard 为 nil 则跳过（nil-safe）。
//
// 审核结果处理：
//   - Safe:    无操作，原样返回
//   - Warning: 记录审计日志，但允许通过
//   - Blocked: 替换为安全回退消息，记录日志
func (o *AgentOrchestrator) checkOutputSafety(tc *TurnContext, response *DraftResponse) *DraftResponse {
	if o.outputGuard == nil {
		return response
	}

	result := o.outputGuard.Check(tc.Ctx, response.Content)

	switch result.Risk {
	case safety.OutputBlocked:
		log.Printf("🛡️  [Output Guard] BLOCKED: session=%d category=%s reason=%q",
			tc.SessionID, result.Category, result.Reason)
		// 替换为安全回退消息
		response.Content = o.outputGuard.FallbackResponse()
		// 如果已经流式输出了不安全内容，需要通知前端覆盖
		if tc.OnTokenDelta != nil {
			tc.OnTokenDelta("\n\n---\n\n" + response.Content)
		}

	case safety.OutputWarning:
		log.Printf("⚠️  [Output Guard] WARNING: session=%d category=%s reason=%q",
			tc.SessionID, result.Category, result.Reason)
		// Warning 级别允许通过，仅记录日志
	}

	return response
}

// ── L2+L3 Cache Integration (§8.1.3) ───────────────────────

// checkSemanticCache performs L2 semantic cache lookup.
// Embeds the user query and searches for similar cached responses.
// Returns nil if cache is unavailable, empty, or no match found.
func (o *AgentOrchestrator) checkSemanticCache(tc *TurnContext) (*cache.SemanticCacheHit, error) {
	if o.cache == nil || o.llm == nil {
		return nil, nil
	}

	// Get courseID for cache scoping
	courseID, err := o.getCourseIDFromSession(tc.Ctx, tc.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get course_id for cache: %w", err)
	}

	// Embed the query
	embedding, err := o.llm.Embed(tc.Ctx, tc.UserInput)
	if err != nil {
		return nil, fmt.Errorf("embed query for cache: %w", err)
	}

	// Store embedding in TurnContext for later cache write
	tc.queryEmbedding = embedding
	tc.queryCourseID = courseID

	// L2 semantic search
	hit, err := o.cache.FindSemanticMatch(tc.Ctx, courseID, embedding)
	if err != nil {
		return nil, err
	}

	return hit, nil
}

// returnCachedResponse sends a cached response to the client and persists the interaction.
func (o *AgentOrchestrator) returnCachedResponse(tc *TurnContext, response, skillID string, start time.Time) error {
	// Send cached response to frontend
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta(response)
	}

	// Persist interaction
	cached := &DraftResponse{
		SessionID:  tc.SessionID,
		Content:    response,
		SkillID:    skillID,
		TokensUsed: 0, // cached, no LLM tokens used
	}
	if err := o.saveInteraction(tc, cached); err != nil {
		log.Printf("⚠️  [Agent] Save cached interaction failed: %v", err)
	}

	// BKT update still needed
	o.updateMasteryAndFadeScaffold(tc)

	elapsed := time.Since(start)
	log.Printf("⚡ [Agent] Turn complete (cached): session=%d elapsed=%s", tc.SessionID, elapsed)

	if tc.OnTurnComplete != nil {
		tc.OnTurnComplete(0)
	}

	return nil
}

// writeResponseToCache writes the LLM response to both L2 and L3 caches.
func (o *AgentOrchestrator) writeResponseToCache(tc *TurnContext, material PersonalizedMaterial, response *DraftResponse) {
	if o.cache == nil {
		return
	}

	ctx := tc.Ctx

	// L2: Semantic cache (if we have the query embedding from the earlier check)
	if tc.queryEmbedding != nil {
		entry := cache.SemanticCacheEntry{
			QueryText: tc.UserInput,
			Embedding: tc.queryEmbedding,
			Response:  response.Content,
			SkillID:   response.SkillID,
			CourseID:  tc.queryCourseID,
		}
		if err := o.cache.SetSemanticCache(ctx, entry); err != nil {
			log.Printf("⚠️  [L2-Cache] Write failed: %v", err)
		}
	}

	// L3: Output cache (exact prompt hash)
	promptHash := cache.PromptHash(material.SystemPrompt, tc.UserInput, nil, nil)
	outputEntry := cache.OutputCacheEntry{
		Response: response.Content,
		SkillID:  response.SkillID,
		CourseID: tc.queryCourseID,
	}
	if err := o.cache.SetOutputCache(ctx, promptHash, outputEntry); err != nil {
		log.Printf("⚠️  [L3-Cache] Write failed: %v", err)
	}
}

// getCourseIDFromSession queries the course ID from session → activity → course.
func (o *AgentOrchestrator) getCourseIDFromSession(ctx context.Context, sessionID uint) (uint, error) {
	var result struct {
		CourseID uint
	}
	err := o.db.WithContext(ctx).Raw(`
		SELECT la.course_id
		FROM student_sessions ss
		JOIN learning_activities la ON la.id = ss.activity_id
		WHERE ss.id = ?
	`, sessionID).Scan(&result).Error
	if err != nil {
		return 0, err
	}
	return result.CourseID, nil
}

// ── Helpers ─────────────────────────────────────────────────

// -- EventBus Helpers ----------------------------------------

// publishEvent fires an EventBus event if the bus is available.
func (o *AgentOrchestrator) publishEvent(ctx context.Context, hook plugin.HookPoint, payload map[string]interface{}) {
	if o.eventBus == nil {
		return
	}
	o.eventBus.Publish(ctx, plugin.HookEvent{Hook: hook, Payload: payload})
}

// publishAbortable fires an abortable EventBus event. Returns error if any handler aborts.
func (o *AgentOrchestrator) publishAbortable(ctx context.Context, hook plugin.HookPoint, payload map[string]interface{}) error {
	if o.eventBus == nil {
		return nil
	}
	return o.eventBus.PublishAbortable(ctx, plugin.HookEvent{Hook: hook, Payload: payload})
}

// truncate 截断字符串到指定长度。
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// buildTaskContext 构建 LLM 路由的 TaskContext (§8.3.3)。
// 收集当前会话的各种上下文信号用于复杂度评分。
func (o *AgentOrchestrator) buildTaskContext(tc *TurnContext, material PersonalizedMaterial) *llm.TaskContext {
	taskCtx := &llm.TaskContext{
		UserInput:         tc.UserInput,
		ChunkCount:        len(material.RetrievedChunks),
		GraphNodeCount:    len(material.GraphContext),
		HasMisconceptions: len(material.Misconceptions) > 0,
	}

	// 技能 ID
	if tc.Prescription != nil {
		taskCtx.SkillID = tc.Prescription.RecommendedSkill
	}

	// 掌握度 — 取第一个目标 KP 的掌握度
	if tc.Prescription != nil && len(tc.Prescription.TargetKPSequence) > 0 {
		taskCtx.Mastery = tc.Prescription.TargetKPSequence[0].CurrentMastery
	}

	// 对话轮数 — 从 interactions 表统计
	var turnCount int64
	o.db.WithContext(tc.Ctx).Model(&model.Interaction{}).
		Where("session_id = ? AND role = ?", tc.SessionID, "student").
		Count(&turnCount)
	taskCtx.TurnCount = int(turnCount)

	// 跨学科检测 — 检查图谱上下文中是否有不同学科的节点
	if len(material.GraphContext) > 2 {
		relations := make(map[string]bool)
		for _, node := range material.GraphContext {
			relations[node.Relation] = true
		}
		taskCtx.IsCrossDiscipline = len(relations) > 2
	}

	log.Printf("🔀 [Router] TaskContext: skill=%s mastery=%.2f turns=%d chunks=%d graph=%d complexity=%s(%.2f)",
		taskCtx.SkillID, taskCtx.Mastery, taskCtx.TurnCount,
		taskCtx.ChunkCount, taskCtx.GraphNodeCount,
		taskCtx.EstimateComplexity(), taskCtx.ComplexityScore())

	return taskCtx
}

// ── Fallacy Detective Phase Management ─────────────────────

// advanceFallacyPhaseIfActive 当活跃技能为谬误侦探时，推进会话阶段。
// 使用启发式方法从 Critic 审查结果推断学生是否正确识别了谬误:
//   - depth_score >= 0.6 且 Critic 通过 → 视为学生识别成功
//   - 否则 → 视为未识别（需要更多提示）
//
// 当学生成功识别谬误时，通过 OnScaffold 回调发送 fallacy_identified 事件到前端，
// 前端可用此事件展示突破成就通知 (design.md §5.2 Step 4: "谬误猎人" 成就)。
func (o *AgentOrchestrator) advanceFallacyPhaseIfActive(tc *TurnContext, response *DraftResponse) {
	if response == nil || !isFallacyDetectiveActive(response.SkillID) {
		return
	}

	// 推断学生是否正确识别了谬误
	identified := false
	if tc.Review != nil {
		// Critic 审查通过且深度分数较高 → 学生有效参与了辨误
		identified = tc.Review.Approved && tc.Review.DepthScore >= 0.6
	}

	// 加载当前状态用于事件通知
	stateBefore := o.coach.loadFallacyState(tc.SessionID)

	o.coach.AdvanceFallacyPhase(tc.SessionID, identified)

	// 当学生成功识别谬误 → 发送前端通知事件
	if identified && stateBefore.Phase == FallacyPhaseAwaiting && tc.OnScaffold != nil {
		tc.OnScaffold("fallacy_identified", map[string]interface{}{
			"identified_count": stateBefore.IdentifiedCount + 1,
			"embedded_count":   stateBefore.EmbeddedCount,
			"max_per_session":  stateBefore.MaxPerSession,
			"trap_desc":        stateBefore.CurrentTrapDesc,
		})
	}
}

// ── Role-Play State Management ─────────────────────────────

// updateRolePlayStateIfActive 当活跃技能为角色扮演时，更新会话中的角色状态。
// 角色扮演不需要像谬误侦探那样的状态机，但需要在首轮对话后持久化角色身份，
// 以便后续轮次维持角色一致性。
func (o *AgentOrchestrator) updateRolePlayStateIfActive(tc *TurnContext, response *DraftResponse) {
	if response == nil || !isRolePlayActive(response.SkillID) {
		return
	}

	state := o.coach.loadRolePlayState(tc.SessionID)

	// 初始化: 首轮对话后角色已由 LLM 选定，但状态中尚未记录
	// 后续轮次会由前端通过 scaffold 事件传递角色信息
	// 这里仅确保状态已被持久化（即使角色名为空，也保存初始状态）
	if state.CharacterName == "" {
		// 首次保存初始状态，后续 LLM 对话中角色信息会通过 SKILL.md 约束自然维持
		o.coach.saveRolePlayState(tc.SessionID, state)
		log.Printf("🎭 [RolePlay] Session=%d: initial state saved (character TBD by LLM)",
			tc.SessionID)
		return
	}

	// 如果角色已选定，仅记录日志
	log.Printf("🎭 [RolePlay] Session=%d: character=%s, scenario_switches=%d/%d, active=%v",
		tc.SessionID, state.CharacterName, state.ScenarioSwitches, state.MaxSwitches, state.Active)
}

// ── Quiz Phase Management (§7.13) ──────────────────────────

// advanceQuizPhaseIfActive 当活跃技能为自动出题时，推进会话阶段。
// 使用启发式方法从回复内容推断阶段转换:
//   - Coach 回复中包含 <quiz> 标签 → 生成了题目，转入 answering
//   - 学生提交答案后 → 转入 grading
//   - 批改完成后 → 转入 reviewing
func (o *AgentOrchestrator) advanceQuizPhaseIfActive(tc *TurnContext, response *DraftResponse) {
	if response == nil || !isQuizActive(response.SkillID) {
		return
	}

	state := o.coach.loadQuizState(tc.SessionID)

	switch state.Phase {
	case QuizPhaseGenerating:
		// 检查回复中是否包含 <quiz> 标签（题目已生成）
		if strings.Contains(response.Content, "<quiz>") {
			// 计算生成的题目数（简单统计 "id" 出现次数）
			questionCount := strings.Count(response.Content, `"type"`)
			if questionCount == 0 {
				questionCount = 1
			}
			o.coach.AdvanceQuizPhase(tc.SessionID, questionCount, 0)

			// 发送题目数据到前端
			if tc.OnScaffold != nil {
				tc.OnScaffold("quiz_questions", map[string]interface{}{
					"batch":          state.BatchCount + 1,
					"question_count": questionCount,
				})
			}
		}

	case QuizPhaseAnswering:
		// 学生已提交答案 → 推进到批改
		o.coach.AdvanceQuizPhase(tc.SessionID, 0, 0)

	case QuizPhaseGrading:
		// 批改完成 → 推进到查看结果
		// 从 Critic 审查推断正确数（粗略）
		correctCount := 0
		if tc.Review != nil && tc.Review.DepthScore >= 0.5 {
			correctCount = 1 // 简化: 细粒度计数需要解析题目结果 JSON
		}
		o.coach.AdvanceQuizPhase(tc.SessionID, 0, correctCount)

		// 发送批改结果到前端
		if tc.OnScaffold != nil {
			tc.OnScaffold("quiz_result", map[string]interface{}{
				"correct_count":   state.CorrectCount + correctCount,
				"total_questions": state.TotalQuestions,
			})
		}

	case QuizPhaseReviewing:
		// 学生请求继续出题 → 回到生成阶段
		o.coach.AdvanceQuizPhase(tc.SessionID, 0, 0)
	}
}

// ── Error Notebook Auto-Archiving (§5.2 Step 3, item 3) ────

// archiveErrorIfIncorrect 当推断学生回答错误时，自动将错误和 AI 引导归档到错题本。
// 同时检查是否有已归档但尚未解决的错题因掌握度提升而可标记为已解决。
func (o *AgentOrchestrator) archiveErrorIfIncorrect(tc *TurnContext, response *DraftResponse) {
	if response == nil {
		return
	}

	// 获取当前会话的 KP
	var session model.StudentSession
	if err := o.db.WithContext(tc.Ctx).First(&session, tc.SessionID).Error; err != nil {
		return
	}

	kpID := session.CurrentKP
	if kpID == 0 && tc.Prescription != nil && len(tc.Prescription.TargetKPSequence) > 0 {
		kpID = tc.Prescription.TargetKPSequence[0].KPID
	}
	if kpID == 0 {
		return
	}

	correct := o.inferCorrectness(tc)

	if !correct {
		// 获取当前掌握度用于记录
		mastery := o.bkt.GetMastery(tc.StudentID, kpID)

		entry := model.ErrorNotebookEntry{
			StudentID:      tc.StudentID,
			KPID:           kpID,
			SessionID:      tc.SessionID,
			StudentInput:   tc.UserInput,
			CoachGuidance:  response.Content,
			ErrorType:      "unknown", // 默认; 后续可用 LLM 分类
			MasteryAtError: mastery,
			ArchivedAt:     time.Now(),
		}

		if err := o.db.WithContext(tc.Ctx).Create(&entry).Error; err != nil {
			log.Printf("⚠️  [ErrorNotebook] Archive failed: student=%d kp=%d: %v",
				tc.StudentID, kpID, err)
			return
		}

		log.Printf("📝 [ErrorNotebook] Archived: student=%d kp=%d session=%d mastery=%.3f",
			tc.StudentID, kpID, tc.SessionID, mastery)
	}

	// 自动解决：当掌握度达到 0.8 时，标记该 KP 的未解决错题为已解决
	currentMastery := o.bkt.GetMastery(tc.StudentID, kpID)
	if currentMastery >= 0.8 {
		now := time.Now()
		result := o.db.WithContext(tc.Ctx).Model(&model.ErrorNotebookEntry{}).
			Where("student_id = ? AND kp_id = ? AND resolved = ?", tc.StudentID, kpID, false).
			Updates(map[string]interface{}{
				"resolved":    true,
				"resolved_at": now,
			})
		if result.RowsAffected > 0 {
			log.Printf("✅ [ErrorNotebook] Auto-resolved %d entries: student=%d kp=%d mastery=%.3f",
				result.RowsAffected, tc.StudentID, kpID, currentMastery)
		}
	}
}
