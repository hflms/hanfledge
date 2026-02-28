package agent

import (
	"fmt"
	"log"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/plugin"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"github.com/hflms/hanfledge/internal/usecase"
	"gorm.io/gorm"
)

// ============================
// 多 Agent 编排引擎
// ============================

// AgentOrchestrator 管理多 Agent 的生命周期与通信。
// 采用 goroutine + channel 管道模式:
//
//	Strategist → Designer → Coach → Critic → Coach (retry loop)
//	                                  ↓
//	                          MasteryUpdate → Strategist
type AgentOrchestrator struct {
	strategist *StrategistAgent
	designer   *DesignerAgent
	coach      *CoachAgent
	critic     *CriticAgent
	bkt        *BKTService

	// Agent 间通信通道
	prescriptionCh chan LearningPrescription // Strategist → Designer
	materialCh     chan PersonalizedMaterial // Designer → Coach
	draftCh        chan DraftResponse        // Coach → Critic
	reviewCh       chan ReviewResult         // Critic → Coach
	masteryCh      chan MasteryUpdate        // Coach → Strategist

	// 依赖
	db       *gorm.DB
	llm      *llm.OllamaClient
	neo4j    *neo4jRepo.Client
	karag    *usecase.KARAGEngine
	registry *plugin.Registry

	// Actor-Critic 最大重试轮数
	maxCriticRetries int
}

// NewAgentOrchestrator 创建一个新的 Agent 编排引擎。
func NewAgentOrchestrator(
	db *gorm.DB,
	llmClient *llm.OllamaClient,
	neo4jClient *neo4jRepo.Client,
	karag *usecase.KARAGEngine,
	registry *plugin.Registry,
) *AgentOrchestrator {
	o := &AgentOrchestrator{
		// 通道初始化（缓冲为 1，防止阻塞）
		prescriptionCh: make(chan LearningPrescription, 1),
		materialCh:     make(chan PersonalizedMaterial, 1),
		draftCh:        make(chan DraftResponse, 1),
		reviewCh:       make(chan ReviewResult, 1),
		masteryCh:      make(chan MasteryUpdate, 1),

		db:       db,
		llm:      llmClient,
		neo4j:    neo4jClient,
		karag:    karag,
		registry: registry,

		maxCriticRetries: 2,
	}

	// 初始化各 Agent
	o.strategist = NewStrategistAgent(db, neo4jClient)
	o.designer = NewDesignerAgent(db, llmClient, neo4jClient, karag)
	o.coach = NewCoachAgent(db, llmClient, registry)
	o.critic = NewCriticAgent(llmClient)
	o.bkt = NewBKTService(db)

	log.Println("🎯 [Agent] Orchestrator initialized with 4 agents + BKT: Strategist, Designer, Coach, Critic")
	return o
}

// ── Pipeline Execution ──────────────────────────────────────

// HandleTurn 处理一轮学生对话，驱动整个 Agent 管道。
// 这是 WebSocket handler 调用的主入口。
func (o *AgentOrchestrator) HandleTurn(tc *TurnContext) error {
	ctx := tc.Ctx
	start := time.Now()

	log.Printf("🎯 [Agent] HandleTurn: session=%d student=%d input=%q",
		tc.SessionID, tc.StudentID, truncate(tc.UserInput, 50))

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

	// ── Stage 3+4: Coach + Critic Actor-Critic 循环 ─────
	if tc.OnThinking != nil {
		tc.OnThinking("Coach 正在组织回复...")
	}

	finalResponse, err := o.actorCriticLoop(tc, material)
	if err != nil {
		return fmt.Errorf("actor-critic loop failed: %w", err)
	}

	// ── Stage 5: 持久化交互记录 ─────────────────────────
	if err := o.saveInteraction(tc, finalResponse); err != nil {
		log.Printf("⚠️  [Agent] Save interaction failed: %v", err)
	}

	// ── Stage 6: BKT 掌握度更新 + 支架衰减 ──────────────
	o.updateMasteryAndFadeScaffold(tc)

	elapsed := time.Since(start)
	log.Printf("✅ [Agent] Turn complete: session=%d tokens=%d elapsed=%s",
		tc.SessionID, finalResponse.TokensUsed, elapsed)

	if tc.OnTurnComplete != nil {
		tc.OnTurnComplete(finalResponse.TokensUsed)
	}

	return nil
}

// actorCriticLoop 执行 Coach-Critic 的 Actor-Critic 循环。
// Coach 生成初稿 → Critic 审查 → 不通过则 Coach 修订 → 最多重试 maxCriticRetries 次。
func (o *AgentOrchestrator) actorCriticLoop(tc *TurnContext, material PersonalizedMaterial) (*DraftResponse, error) {
	var draft DraftResponse
	var err error

	for attempt := 0; attempt <= o.maxCriticRetries; attempt++ {
		// Coach 生成回复
		if attempt == 0 {
			draft, err = o.coach.GenerateResponse(tc, material)
		} else {
			// 基于 Critic 反馈修订
			review := tc.Review
			if tc.OnThinking != nil {
				tc.OnThinking(fmt.Sprintf("Coach 正在根据审查反馈修订 (第%d次)...", attempt))
			}
			draft, err = o.coach.ReviseResponse(tc, material, review)
		}
		if err != nil {
			return nil, fmt.Errorf("coach attempt %d failed: %w", attempt, err)
		}

		tc.Draft = &draft

		// 最后一轮不再审查，直接采用
		if attempt == o.maxCriticRetries {
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
			break // Critic 失败时 fallback 到当前草稿
		}

		tc.Review = &review

		log.Printf("   → Critic: approved=%t leakage=%.2f depth=%.2f (attempt %d)",
			review.Approved, review.LeakageScore, review.DepthScore, attempt+1)

		if review.Approved {
			break
		}
	}

	return &draft, nil
}

// saveInteraction 持久化学生输入和 AI 回复到 interactions 表。
func (o *AgentOrchestrator) saveInteraction(tc *TurnContext, response *DraftResponse) error {
	now := time.Now()

	// 保存学生输入
	studentMsg := model.Interaction{
		SessionID: tc.SessionID,
		Role:      "student",
		Content:   tc.UserInput,
		CreatedAt: now,
	}
	if err := o.db.Create(&studentMsg).Error; err != nil {
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
	if err := o.db.Create(&coachMsg).Error; err != nil {
		return fmt.Errorf("save coach interaction: %w", err)
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
	if err := o.db.First(&session, tc.SessionID).Error; err != nil {
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

	// BKT 掌握度更新
	update, err := o.bkt.UpdateStudentMastery(tc.StudentID, kpID, correct)
	if err != nil {
		log.Printf("⚠️  [Scaffold] BKT update failed for student=%d kp=%d: %v",
			tc.StudentID, kpID, err)
		return
	}

	// 计算新的支架等级
	newScaffold := scaffoldForMastery(update.NewMastery)
	oldScaffold := session.Scaffold

	// 检查是否需要衰减
	if newScaffold != oldScaffold {
		log.Printf("🔄 [Scaffold] Fading: student=%d kp=%d mastery=%.3f scaffold %s→%s",
			tc.StudentID, kpID, update.NewMastery, oldScaffold, newScaffold)

		// 更新数据库中的会话支架等级
		if err := o.db.Model(&model.StudentSession{}).Where("id = ?", tc.SessionID).
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

// ── Helpers ─────────────────────────────────────────────────

// truncate 截断字符串到指定长度。
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
