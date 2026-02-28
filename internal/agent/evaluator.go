package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"gorm.io/gorm"
)

// ============================
// RAGAS + MRBench 异步评估引擎
// ============================
//
// 职责：异步评估 Coach 交互的质量，填充 Interaction 表上的评估分数。
// 评估维度（design.md §4.2）:
//
// 纯技术评估 (RAGAS)：
//   - Faithfulness: 回答是否完全来自检索上下文（图谱/文档依据）
//   - Context Precision: 检索上下文中相关内容的比例
//   - Context Recall: 回答所需知识是否都被检索到
//
// 教学维度评估 (MRBench)：
//   - Answer Restraint: 是否克制直接给出答案
//   - Actionability: 是否提供了有效且可执行的下一步指引
//
// 工作模式：
//   后台 goroutine 定期轮询 eval_status='pending' 的 coach 交互，
//   每批取 N 条进行 LLM 评估，更新分数后标记为 'evaluated'。

// -- Configuration --------------------------------------------

// EvalConfig 评估引擎配置。
type EvalConfig struct {
	// BatchSize 每轮评估的最大交互数量。
	BatchSize int
	// PollInterval 轮询间隔（多久检查一次待评估的交互）。
	PollInterval time.Duration
}

// DefaultEvalConfig 返回默认配置。
func DefaultEvalConfig() EvalConfig {
	return EvalConfig{
		BatchSize:    10,
		PollInterval: 30 * time.Second,
	}
}

// -- Evaluator ------------------------------------------------

// RAGASEvaluator 异步评估引擎，后台运行评估 Coach 交互质量。
type RAGASEvaluator struct {
	db     *gorm.DB
	llm    llm.LLMProvider
	config EvalConfig
}

// NewRAGASEvaluator 创建新的评估引擎。
func NewRAGASEvaluator(db *gorm.DB, llmClient llm.LLMProvider, cfg EvalConfig) *RAGASEvaluator {
	return &RAGASEvaluator{
		db:     db,
		llm:    llmClient,
		config: cfg,
	}
}

// Start 启动后台评估循环。传入的 context 控制生命周期（cancel 即停止）。
func (e *RAGASEvaluator) Start(ctx context.Context) {
	log.Printf("📊 [RAGAS] Evaluator started: batch=%d interval=%s",
		e.config.BatchSize, e.config.PollInterval)

	ticker := time.NewTicker(e.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("📊 [RAGAS] Evaluator shutting down")
			return
		case <-ticker.C:
			evaluated := e.evaluateBatch(ctx)
			if evaluated > 0 {
				log.Printf("📊 [RAGAS] Evaluated %d interactions", evaluated)
			}
		}
	}
}

// evaluateBatch 取一批待评估的 coach 交互，逐条评估。
// 返回成功评估的条目数。
func (e *RAGASEvaluator) evaluateBatch(ctx context.Context) int {
	// 查询 pending coach 交互（仅评估 coach 角色）
	var interactions []model.Interaction
	err := e.db.Where("eval_status = ? AND role = ?", "pending", "coach").
		Order("created_at ASC").
		Limit(e.config.BatchSize).
		Find(&interactions).Error
	if err != nil {
		log.Printf("⚠️  [RAGAS] Fetch pending interactions failed: %v", err)
		return 0
	}

	if len(interactions) == 0 {
		return 0
	}

	evaluated := 0
	for _, inter := range interactions {
		select {
		case <-ctx.Done():
			return evaluated
		default:
		}

		if err := e.evaluateOne(ctx, inter); err != nil {
			log.Printf("⚠️  [RAGAS] Evaluate interaction %d failed: %v", inter.ID, err)
			// 标记为 skipped 避免反复失败重试
			e.markSkipped(inter.ID)
			continue
		}
		evaluated++
	}

	return evaluated
}

// -- Single Interaction Evaluation ----------------------------

// evalScores 评估 LLM 返回的分数结构。
type evalScores struct {
	Faithfulness     float64 `json:"faithfulness"`
	Actionability    float64 `json:"actionability"`
	AnswerRestraint  float64 `json:"answer_restraint"`
	ContextPrecision float64 `json:"context_precision"`
	ContextRecall    float64 `json:"context_recall"`
}

// evaluateOne 对单条 coach 交互进行 RAGAS + MRBench 评估。
func (e *RAGASEvaluator) evaluateOne(ctx context.Context, inter model.Interaction) error {
	// 加载上下文：获取同会话中该 coach 回复前的学生消息
	studentInput, err := e.getPrecedingStudentMessage(inter.SessionID, inter.ID)
	if err != nil {
		return fmt.Errorf("load student context: %w", err)
	}

	// 加载会话信息（活跃技能、支架等级）
	var session model.StudentSession
	if err := e.db.First(&session, inter.SessionID).Error; err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	// 构建评估 Prompt
	evalPrompt := buildEvalPrompt(inter.Content, studentInput, inter.SkillID, string(session.Scaffold))

	messages := []llm.ChatMessage{
		{
			Role:    "system",
			Content: ragasSystemPrompt,
		},
		{
			Role:    "user",
			Content: evalPrompt,
		},
	}

	response, err := e.llm.Chat(ctx, messages, &llm.ChatOptions{
		Temperature: 0.1, // 评估需要确定性
		MaxTokens:   512,
	})
	if err != nil {
		return fmt.Errorf("LLM evaluation call: %w", err)
	}

	// 解析分数
	scores, err := parseEvalScores(response)
	if err != nil {
		return fmt.Errorf("parse eval scores: %w", err)
	}

	// 写回数据库
	return e.updateScores(inter.ID, scores)
}

// -- Prompt Construction --------------------------------------

const ragasSystemPrompt = `你是一位教育 AI 质量评估专家。你的任务是评估 AI 教练回复的质量。

请从以下 5 个维度评分，每个维度 0.0 到 1.0：

1. faithfulness（忠实度）：回答是否基于可验证的教学内容，而非凭空编造？
   - 1.0 = 完全忠实于教学内容，所有表述都有依据
   - 0.5 = 大部分忠实，但有小部分未经证实的表述
   - 0.0 = 大量凭空编造的内容

2. actionability（可执行性）：是否提供了有效且可执行的下一步学习指引？
   - 1.0 = 给出了清晰、具体、学生可以立即执行的下一步
   - 0.5 = 给出了一些指引，但不够具体或缺乏可操作性
   - 0.0 = 没有任何指引，或指引完全不可操作

3. answer_restraint（答案克制度）：是否克制了直接给出答案？
   - 1.0 = 完全没有泄露答案，通过提问引导学生思考
   - 0.5 = 部分泄露了答案方向，但保留了思考空间
   - 0.0 = 直接给出了完整答案

4. context_precision（上下文精度）：回复内容是否精确聚焦于学生的问题？
   - 1.0 = 完全聚焦，没有废话或离题内容
   - 0.5 = 基本聚焦，但有一些无关内容
   - 0.0 = 大量离题或无关内容

5. context_recall（上下文召回）：是否涵盖了回答学生问题所需的关键知识点？
   - 1.0 = 涵盖了所有必要的知识点和关联概念
   - 0.5 = 涵盖了部分关键点，但遗漏了重要内容
   - 0.0 = 几乎没有涵盖学生需要的知识

请以纯 JSON 格式返回（不要返回任何其他内容）：
{
  "faithfulness": 0.0,
  "actionability": 0.0,
  "answer_restraint": 0.0,
  "context_precision": 0.0,
  "context_recall": 0.0
}`

// buildEvalPrompt 构建单条交互的评估 Prompt。
func buildEvalPrompt(coachContent, studentInput, skillID, scaffoldLevel string) string {
	return fmt.Sprintf(`请评估以下 AI 教练的回复质量：

【学生提问/输入】
%s

【AI 教练回复】
%s

【教学技能 ID】%s
【当前支架等级】%s

注意：
- 支架等级 "high" 表示学生水平较低，需要更多引导
- 支架等级 "low" 表示学生水平较高，应给予更多自主空间
- 评估 answer_restraint 时，要结合支架等级判断：high 支架下适当给出更多提示是合理的
- 请客观评估，不要因为回复较长就认为质量高`, studentInput, coachContent, skillID, scaffoldLevel)
}

// -- Response Parsing -----------------------------------------

// parseEvalScores 解析 LLM 评估响应为分数结构。
func parseEvalScores(response string) (evalScores, error) {
	// 尝试提取 JSON（LLM 可能附带 markdown 代码块）
	jsonStr := extractJSON(response)

	var scores evalScores
	if err := json.Unmarshal([]byte(jsonStr), &scores); err != nil {
		return evalScores{}, fmt.Errorf("JSON parse failed: %w (raw: %s)", err, truncate(response, 200))
	}

	// 校验分数范围
	scores.Faithfulness = clampScore(scores.Faithfulness)
	scores.Actionability = clampScore(scores.Actionability)
	scores.AnswerRestraint = clampScore(scores.AnswerRestraint)
	scores.ContextPrecision = clampScore(scores.ContextPrecision)
	scores.ContextRecall = clampScore(scores.ContextRecall)

	return scores, nil
}

// extractJSON 从 LLM 响应中提取 JSON 内容。
// 支持从 markdown 代码块（```json ... ```）中提取。
func extractJSON(response string) string {
	// 尝试找 ``` 代码块
	start := -1
	end := -1
	for i := 0; i < len(response)-2; i++ {
		if response[i] == '{' && start == -1 {
			start = i
		}
	}
	if start == -1 {
		return response
	}
	// 找到最后一个 }
	for i := len(response) - 1; i >= start; i-- {
		if response[i] == '}' {
			end = i + 1
			break
		}
	}
	if end == -1 {
		return response
	}
	return response[start:end]
}

// clampScore 将分数限制在 [0.0, 1.0] 范围内。
func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// -- Database Operations --------------------------------------

// getPrecedingStudentMessage 获取同会话中在指定交互之前的最近一条学生消息。
func (e *RAGASEvaluator) getPrecedingStudentMessage(sessionID uint, interactionID uint) (string, error) {
	var studentMsg model.Interaction
	err := e.db.Where("session_id = ? AND id < ? AND role = ?", sessionID, interactionID, "student").
		Order("created_at DESC").
		First(&studentMsg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "(无学生输入)", nil
		}
		return "", err
	}
	return studentMsg.Content, nil
}

// updateScores 将评估分数写回 Interaction 表。
func (e *RAGASEvaluator) updateScores(interactionID uint, scores evalScores) error {
	return e.db.Model(&model.Interaction{}).Where("id = ?", interactionID).
		Updates(map[string]interface{}{
			"faithfulness_score":     scores.Faithfulness,
			"actionability_score":    scores.Actionability,
			"answer_restraint_score": scores.AnswerRestraint,
			"context_precision":      scores.ContextPrecision,
			"context_recall":         scores.ContextRecall,
			"eval_status":            "evaluated",
		}).Error
}

// markSkipped 将交互标记为跳过评估（评估失败时使用，避免反复重试）。
func (e *RAGASEvaluator) markSkipped(interactionID uint) {
	if err := e.db.Model(&model.Interaction{}).Where("id = ?", interactionID).
		Update("eval_status", "skipped").Error; err != nil {
		log.Printf("⚠️  [RAGAS] Mark interaction %d as skipped failed: %v", interactionID, err)
	}
}
