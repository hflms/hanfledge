package embedding

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"gorm.io/gorm"
)

var slogDistill = logger.L("Distillation")

// ============================
// Small Model Distillation Pipeline (§8.3.2)
// ============================
//
// 职责：从高质量 AI 交互日志中构建 SFT 数据集，
// 用于将大模型（Qwen-Max / GPT-4）的技能蒸馏到小模型（Qwen2.5-7B）。
//
// 流程:
//   1. 从 RAGAS 评估后的交互日志中筛选高质量样本 (score > 0.85)
//   2. 构建 SFT (Supervised Fine-Tuning) Input-Output 数据集
//   3. 生成 LoRA/QLoRA 训练配置
//   4. 评估蒸馏效果 (Quality Retention Rate)
//
// Reference: design.md §8.3.2

// -- Distillation Data Types --------------------------------------

// SFTSample 一条监督微调样本 (Input-Output Pair)。
type SFTSample struct {
	ID           uint     `json:"id"`
	SystemPrompt string   `json:"system_prompt"`          // 系统提示词（含技能约束）
	Messages     []SFTMsg `json:"messages"`               // 对话历史
	SkillID      string   `json:"skill_id"`               // 来源技能
	CourseID     uint     `json:"course_id,omitempty"`    // 来源课程
	RAGASScore   float64  `json:"ragas_score"`            // RAGAS 综合评分
	SourceModel  string   `json:"source_model,omitempty"` // 教师模型名
}

// SFTMsg 单条 SFT 对话消息。
type SFTMsg struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"` // 消息内容
}

// SFTDataset 完整的 SFT 数据集。
type SFTDataset struct {
	Samples   []SFTSample `json:"samples"`
	CreatedAt time.Time   `json:"created_at"`
	Stats     SFTStats    `json:"stats"`
	Config    LoRAConfig  `json:"lora_config"`
}

// SFTStats 数据集统计信息。
type SFTStats struct {
	TotalSamples   int            `json:"total_samples"`
	BySkill        map[string]int `json:"by_skill"`         // 各技能样本数
	AvgRAGASScore  float64        `json:"avg_ragas_score"`  // 平均 RAGAS 评分
	AvgTurns       float64        `json:"avg_turns"`        // 平均对话轮次
	TotalTokensEst int            `json:"total_tokens_est"` // 估算总 token 数
}

// LoRAConfig LoRA/QLoRA 微调配置。
type LoRAConfig struct {
	BaseModel      string   `json:"base_model"`       // 基座模型 (e.g., "Qwen/Qwen2.5-7B-Instruct")
	LoRARank       int      `json:"lora_rank"`        // LoRA 秩 (默认 64)
	LoRAAlpha      int      `json:"lora_alpha"`       // LoRA alpha (默认 128)
	LoRADropout    float64  `json:"lora_dropout"`     // Dropout (默认 0.05)
	LearningRate   float64  `json:"learning_rate"`    // 学习率 (默认 2e-4)
	NumEpochs      int      `json:"num_epochs"`       // 训练轮次 (默认 3)
	BatchSize      int      `json:"batch_size"`       // 批大小 (默认 4)
	GradAccumSteps int      `json:"grad_accum_steps"` // 梯度累积 (默认 4)
	MaxSeqLen      int      `json:"max_seq_len"`      // 最大序列长度 (默认 2048)
	Quantization   string   `json:"quantization"`     // "none" | "4bit" (QLoRA)
	TargetModules  []string `json:"target_modules"`   // LoRA 目标模块
}

// -- Distillation Quality Metrics ---------------------------------

// DistillationMetrics 蒸馏效果评估指标。
type DistillationMetrics struct {
	QualityRetentionRate float64            `json:"quality_retention_rate"` // 质量保持率 (蒸馏后/蒸馏前)
	AvgTeacherScore      float64            `json:"avg_teacher_score"`      // 教师模型平均分
	AvgStudentScore      float64            `json:"avg_student_score"`      // 学生模型平均分
	BySkill              map[string]float64 `json:"by_skill"`               // 各技能质量保持率
	SampleCount          int                `json:"sample_count"`           // 评估样本数
}

// -- Distillation Pipeline ----------------------------------------

// DistillationPipeline 小模型蒸馏流水线。
type DistillationPipeline struct {
	DB                 *gorm.DB
	TeacherLLM         llm.LLMProvider // 教师模型 (大模型)
	StudentLLM         llm.LLMProvider // 学生模型 (小模型, 用于评估)
	OutputDir          string          // 输出目录
	MinRAGASScore      float64         // 最低 RAGAS 评分阈值 (默认 0.85)
	MaxSamplesPerSkill int             // 每技能最大样本数 (默认 5000)
}

// NewDistillationPipeline 创建蒸馏流水线。
func NewDistillationPipeline(db *gorm.DB, teacherLLM llm.LLMProvider) *DistillationPipeline {
	return &DistillationPipeline{
		DB:                 db,
		TeacherLLM:         teacherLLM,
		OutputDir:          "data/distillation",
		MinRAGASScore:      0.85,
		MaxSamplesPerSkill: 5000,
	}
}

// -- Step 1: High-Quality Log Filtering ---------------------------

// interactionLogRow maps to an interaction with RAGAS scores.
type interactionLogRow struct {
	ID               uint
	SessionID        uint
	Content          string
	Role             string
	SkillID          string
	CourseID         uint
	Faithfulness     float64
	AnswerRelevance  float64
	AnswerRestraint  float64
	ContextPrecision float64
	ContextRecall    float64
	EvalStatus       string
}

// FilterHighQualityLogs 从交互日志中筛选 RAGAS 评分 > 阈值的高质量样本。
func (p *DistillationPipeline) FilterHighQualityLogs(ctx context.Context, skillIDs []string) ([]interactionLogRow, error) {
	slogDistill.Info("filtering high-quality logs", "min_ragas", p.MinRAGASScore, "skills", len(skillIDs))

	var rows []interactionLogRow

	query := p.DB.WithContext(ctx).Table("interactions i").
		Select(`i.id, i.session_id, i.content, i.role, i.skill_id,
			COALESCE(la.course_id, 0) as course_id,
			COALESCE(i.faithfulness, 0) as faithfulness,
			COALESCE(i.answer_relevance, 0) as answer_relevance,
			COALESCE(i.answer_restraint, 0) as answer_restraint,
			COALESCE(i.context_precision, 0) as context_precision,
			COALESCE(i.context_recall, 0) as context_recall,
			i.eval_status`).
		Joins("JOIN student_sessions ss ON ss.id = i.session_id").
		Joins("LEFT JOIN learning_activities la ON la.id = ss.activity_id").
		Where("i.eval_status = ?", "evaluated").
		Where("i.role = ?", "coach")

	if len(skillIDs) > 0 {
		query = query.Where("i.skill_id IN ?", skillIDs)
	}

	// 筛选 RAGAS 综合分数 > 阈值的交互
	// 综合分数 = 5 维平均值
	avgScoreExpr := fmt.Sprintf(
		"(COALESCE(i.faithfulness,0) + COALESCE(i.answer_relevance,0) + COALESCE(i.answer_restraint,0) + COALESCE(i.context_precision,0) + COALESCE(i.context_recall,0)) / 5.0 > %f",
		p.MinRAGASScore,
	)
	query = query.Where(avgScoreExpr)

	if err := query.Order("i.session_id, i.id").Limit(p.MaxSamplesPerSkill * len(skillIDs)).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query high-quality logs failed: %w", err)
	}

	slogDistill.Info("found high-quality interactions", "count", len(rows))
	return rows, nil
}

// -- Step 2: SFT Dataset Construction ----------------------------

// BuildSFTDataset 从高质量交互日志构建 SFT 数据集。
// 将同一 session 的连续对话组装为 Input-Output 对。
func (p *DistillationPipeline) BuildSFTDataset(ctx context.Context, logs []interactionLogRow) (*SFTDataset, error) {
	if len(logs) == 0 {
		return nil, fmt.Errorf("no logs to build dataset from")
	}

	slogDistill.Info("building sft dataset", "interactions", len(logs))

	// 按 session 分组
	sessionGroups := make(map[uint][]interactionLogRow)
	var sessionIDs []uint
	for _, row := range logs {
		if len(sessionGroups[row.SessionID]) == 0 {
			sessionIDs = append(sessionIDs, row.SessionID)
		}
		sessionGroups[row.SessionID] = append(sessionGroups[row.SessionID], row)
	}

	// 批量加载所有 session 的完整对话上下文
	var allInteractions []interactionLogRow
	if len(sessionIDs) > 0 {
		if err := p.DB.WithContext(ctx).Table("interactions").
			Where("session_id IN ?", sessionIDs).
			Order("session_id ASC, id ASC").
			Find(&allInteractions).Error; err != nil {
			return nil, fmt.Errorf("failed to batch load interactions: %w", err)
		}
	}

	// 按 session 组织上下文
	allMsgsBySession := make(map[uint][]interactionLogRow)
	for _, msg := range allInteractions {
		allMsgsBySession[msg.SessionID] = append(allMsgsBySession[msg.SessionID], msg)
	}

	var samples []SFTSample
	sampleID := uint(1)
	skillCounts := make(map[string]int)
	totalTurns := 0
	totalTokensEst := 0
	totalRAGAS := 0.0

	for sessionID, interactions := range sessionGroups {
		// 获取该 session 的完整对话上下文
		allMsgs := allMsgsBySession[sessionID]

		// 构建 SFT 消息序列
		var msgs []SFTMsg
		for _, m := range allMsgs {
			role := "user"
			if m.Role == "coach" {
				role = "assistant"
			} else if m.Role != "student" {
				continue // 跳过系统消息
			}
			msgs = append(msgs, SFTMsg{Role: role, Content: m.Content})
		}

		if len(msgs) < 2 {
			continue
		}

		// 使用第一条高质量 coach 交互的元信息
		first := interactions[0]
		ragasScore := (first.Faithfulness + first.AnswerRelevance + first.AnswerRestraint +
			first.ContextPrecision + first.ContextRecall) / 5.0

		sample := SFTSample{
			ID:           sampleID,
			SystemPrompt: buildDistillSystemPrompt(first.SkillID),
			Messages:     msgs,
			SkillID:      first.SkillID,
			CourseID:     first.CourseID,
			RAGASScore:   ragasScore,
			SourceModel:  p.TeacherLLM.Name(),
		}

		samples = append(samples, sample)
		sampleID++
		skillCounts[first.SkillID]++
		totalTurns += len(msgs)
		totalRAGAS += ragasScore

		// 粗略 token 估算 (中文约 1.5 字/token)
		for _, m := range msgs {
			totalTokensEst += estimateTokenCount(m.Content)
		}
	}

	if len(samples) == 0 {
		return nil, fmt.Errorf("no valid SFT samples generated")
	}

	avgRAGAS := totalRAGAS / float64(len(samples))
	avgTurns := float64(totalTurns) / float64(len(samples))

	dataset := &SFTDataset{
		Samples:   samples,
		CreatedAt: time.Now(),
		Stats: SFTStats{
			TotalSamples:   len(samples),
			BySkill:        skillCounts,
			AvgRAGASScore:  avgRAGAS,
			AvgTurns:       avgTurns,
			TotalTokensEst: totalTokensEst,
		},
		Config: DefaultLoRAConfig(),
	}

	slogDistill.Info("built sft dataset", "samples", len(samples), "avg_ragas", avgRAGAS, "avg_turns", avgTurns, "tokens_est", totalTokensEst)

	return dataset, nil
}

// -- Step 3: LoRA Config Generation --------------------------------

// DefaultLoRAConfig 返回默认 LoRA/QLoRA 配置。
func DefaultLoRAConfig() LoRAConfig {
	return LoRAConfig{
		BaseModel:      "Qwen/Qwen2.5-7B-Instruct",
		LoRARank:       64,
		LoRAAlpha:      128,
		LoRADropout:    0.05,
		LearningRate:   2e-4,
		NumEpochs:      3,
		BatchSize:      4,
		GradAccumSteps: 4,
		MaxSeqLen:      2048,
		Quantization:   "4bit",
		TargetModules:  []string{"q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"},
	}
}

// -- Step 4: Quality Evaluation -----------------------------------

// EvaluateDistillation 评估蒸馏效果。
// 使用教师模型和学生模型分别对同一组 prompt 生成回答，
// 然后由教师模型评分，计算质量保持率。
func (p *DistillationPipeline) EvaluateDistillation(ctx context.Context, dataset *SFTDataset, maxEvalSamples int) (*DistillationMetrics, error) {
	if p.StudentLLM == nil {
		return nil, fmt.Errorf("student LLM not configured")
	}

	if maxEvalSamples <= 0 || maxEvalSamples > len(dataset.Samples) {
		maxEvalSamples = len(dataset.Samples)
	}
	if maxEvalSamples > 50 {
		maxEvalSamples = 50 // 评估样本上限
	}

	slogDistill.Info("evaluating distillation quality", "samples", maxEvalSamples)

	evalSamples := dataset.Samples[:maxEvalSamples]
	var totalTeacherScore, totalStudentScore float64
	skillScoresTeacher := make(map[string]float64)
	skillScoresStudent := make(map[string]float64)
	skillCounts := make(map[string]int)
	validCount := 0

	for i, sample := range evalSamples {
		if len(sample.Messages) < 2 {
			continue
		}

		// 取最后一条 user 消息作为评估 prompt
		var lastUserMsg string
		var chatHistory []llm.ChatMessage
		for _, m := range sample.Messages {
			if m.Role == "user" {
				lastUserMsg = m.Content
			}
			chatHistory = append(chatHistory, llm.ChatMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}

		if lastUserMsg == "" {
			continue
		}

		// 截取到最后一条 user 消息（不包含 assistant 回复）
		evalMsgs := chatHistory[:len(chatHistory)-1]
		if len(evalMsgs) == 0 {
			continue
		}

		// 教师模型生成
		teacherResp, err := p.TeacherLLM.Chat(ctx, evalMsgs, nil)
		if err != nil {
			slogDistill.Warn("teacher eval failed", "sample", i, "err", err)
			continue
		}

		// 学生模型生成
		studentResp, err := p.StudentLLM.Chat(ctx, evalMsgs, nil)
		if err != nil {
			slogDistill.Warn("student eval failed", "sample", i, "err", err)
			continue
		}

		// 使用教师模型作为评判者打分
		teacherScore := scoreResponse(ctx, p.TeacherLLM, lastUserMsg, teacherResp)
		studentScore := scoreResponse(ctx, p.TeacherLLM, lastUserMsg, studentResp)

		totalTeacherScore += teacherScore
		totalStudentScore += studentScore
		skillScoresTeacher[sample.SkillID] += teacherScore
		skillScoresStudent[sample.SkillID] += studentScore
		skillCounts[sample.SkillID]++
		validCount++

		if (i+1)%10 == 0 {
			slogDistill.Debug("evaluation progress", "evaluated", i+1, "total", maxEvalSamples)
		}
	}

	if validCount == 0 {
		return nil, fmt.Errorf("no valid evaluation samples")
	}

	avgTeacher := totalTeacherScore / float64(validCount)
	avgStudent := totalStudentScore / float64(validCount)

	// 计算各技能质量保持率
	bySkill := make(map[string]float64)
	for skillID, count := range skillCounts {
		if count > 0 {
			t := skillScoresTeacher[skillID] / float64(count)
			s := skillScoresStudent[skillID] / float64(count)
			if t > 0 {
				bySkill[skillID] = s / t
			}
		}
	}

	retentionRate := 0.0
	if avgTeacher > 0 {
		retentionRate = avgStudent / avgTeacher
	}

	metrics := &DistillationMetrics{
		QualityRetentionRate: retentionRate,
		AvgTeacherScore:      avgTeacher,
		AvgStudentScore:      avgStudent,
		BySkill:              bySkill,
		SampleCount:          validCount,
	}

	slogDistill.Info("quality retention rate",
		"retention_pct", retentionRate*100, "teacher_avg", avgTeacher, "student_avg", avgStudent)

	return metrics, nil
}

// -- Export --------------------------------------------------------

// ExportSFTDataset 导出 SFT 数据集为 JSONL 文件 (Alpaca/ShareGPT 格式)。
func (p *DistillationPipeline) ExportSFTDataset(dataset *SFTDataset, format string) (string, error) {
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir failed: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")

	switch format {
	case "sharegpt":
		return p.exportShareGPT(dataset, timestamp)
	default:
		return p.exportAlpaca(dataset, timestamp)
	}
}

// exportAlpaca 导出为 Alpaca SFT 格式 (instruction/input/output)。
func (p *DistillationPipeline) exportAlpaca(dataset *SFTDataset, timestamp string) (string, error) {
	filename := filepath.Join(p.OutputDir, fmt.Sprintf("sft_alpaca_%s.jsonl", timestamp))

	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("create file failed: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, sample := range dataset.Samples {
		// 找到最后一对 user-assistant 消息
		var instruction, output string
		for i := len(sample.Messages) - 1; i >= 0; i-- {
			if sample.Messages[i].Role == "assistant" && output == "" {
				output = sample.Messages[i].Content
			} else if sample.Messages[i].Role == "user" && instruction == "" && output != "" {
				instruction = sample.Messages[i].Content
				break
			}
		}

		record := map[string]string{
			"instruction": sample.SystemPrompt + "\n\n" + instruction,
			"input":       "",
			"output":      output,
		}
		if err := encoder.Encode(record); err != nil {
			return "", fmt.Errorf("encode sample failed: %w", err)
		}
	}

	slogDistill.Info("exported alpaca samples", "count", len(dataset.Samples), "file", filename)
	return filename, nil
}

// exportShareGPT 导出为 ShareGPT 多轮对话格式。
func (p *DistillationPipeline) exportShareGPT(dataset *SFTDataset, timestamp string) (string, error) {
	filename := filepath.Join(p.OutputDir, fmt.Sprintf("sft_sharegpt_%s.jsonl", timestamp))

	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("create file failed: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, sample := range dataset.Samples {
		conversations := make([]map[string]string, 0, len(sample.Messages)+1)

		// 系统消息
		conversations = append(conversations, map[string]string{
			"from":  "system",
			"value": sample.SystemPrompt,
		})

		// 对话消息
		for _, msg := range sample.Messages {
			from := "human"
			if msg.Role == "assistant" {
				from = "gpt"
			}
			conversations = append(conversations, map[string]string{
				"from":  from,
				"value": msg.Content,
			})
		}

		record := map[string]interface{}{
			"conversations": conversations,
		}
		if err := encoder.Encode(record); err != nil {
			return "", fmt.Errorf("encode sample failed: %w", err)
		}
	}

	slogDistill.Info("exported sharegpt samples", "count", len(dataset.Samples), "file", filename)
	return filename, nil
}

// ExportLoRAConfig 导出 LoRA 训练配置文件。
func (p *DistillationPipeline) ExportLoRAConfig(cfg LoRAConfig) (string, error) {
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir failed: %w", err)
	}

	filename := filepath.Join(p.OutputDir, "lora_config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config failed: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("write config failed: %w", err)
	}

	slogDistill.Info("exported lora config", "file", filename)
	return filename, nil
}

// -- Helpers -------------------------------------------------------

// buildDistillSystemPrompt 为 SFT 样本构建系统提示词。
func buildDistillSystemPrompt(skillID string) string {
	base := "你是一位专业的 AI 学习导师。请根据以下教学策略引导学生学习。"
	switch skillID {
	case "general_concept_socratic":
		return base + "\n\n教学策略：苏格拉底式提问 — 通过层层递进的引导性问题帮助学生自主发现知识。"
	case "general_assessment_fallacy":
		return base + "\n\n教学策略：谬误侦探 — 在回答中嵌入常见误区，引导学生识别和纠正。"
	case "general_review_roleplay":
		return base + "\n\n教学策略：角色扮演 — 模拟历史人物或科学家，让学生在沉浸式情境中学习。"
	case "general_practice_quiz":
		return base + "\n\n教学策略：自动出题 — 根据知识点生成练习题，评估学生掌握程度。"
	default:
		return base
	}
}

// scoreResponse 使用 LLM 评分一个回答的质量 (0-1)。
func scoreResponse(ctx context.Context, judge llm.LLMProvider, question, response string) float64 {
	prompt := fmt.Sprintf(
		`请为以下 AI 导师的回答打分 (0-10 分)，评估其教学引导质量、知识准确性和学生友好性。
只输出一个数字分数。

学生提问: %s

AI 回答: %s

分数:`, question, response)

	msgs := []llm.ChatMessage{{Role: "user", Content: prompt}}
	resp, err := judge.Chat(ctx, msgs, nil)
	if err != nil {
		return 0.5 // 默认中等分数
	}

	// 解析分数
	score := parseDistillScore(resp)
	return score / 10.0 // 归一化到 [0, 1]
}

// parseDistillScore 从 LLM 回答中解析分数。
func parseDistillScore(text string) float64 {
	var score float64
	_, err := fmt.Sscanf(text, "%f", &score)
	if err != nil {
		return 5.0 // 解析失败时默认 5 分
	}
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}
	return score
}

// estimateTokenCount 粗略估算文本的 token 数。
// 中文约 1.5 字/token，英文约 4 字符/token。
func estimateTokenCount(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}

	chineseCount := 0
	for _, r := range runes {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
		}
	}

	englishChars := len(runes) - chineseCount
	tokens := int(float64(chineseCount)/1.5) + int(float64(englishChars)/4.0)
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}
