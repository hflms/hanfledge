package embedding

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"gorm.io/gorm"
)

var slogFinetune = logger.L("Finetune")

// ============================
// Embedding Fine-tuning Pipeline (§8.3.1)
// ============================
//
// 职责：收集"学生真实提问 ↔ 教材段落"配对数据，
// 利用对比学习（InfoNCE Loss）对开源 Embedding 模型进行领域微调。
//
// 流程:
//   1. 从交互日志中提取高质量 Query-Passage 配对
//   2. 计算 InfoNCE 对比学习损失
//   3. 评估微调效果 (Recall@K, NDCG@K, MRR)
//
// Reference: design.md §8.3.1

// -- Training Data Types ------------------------------------------

// TrainingPair 一组"学生提问 ↔ 教材段落"配对数据。
type TrainingPair struct {
	ID       uint    `json:"id"`
	Query    string  `json:"query"`   // 学生真实提问
	Passage  string  `json:"passage"` // 对应教材段落
	CourseID uint    `json:"course_id"`
	KPID     uint    `json:"kp_id,omitempty"`
	Score    float64 `json:"score"` // 相关性分数 [0,1]
}

// TrainingDataset 训练数据集。
type TrainingDataset struct {
	Pairs     []TrainingPair `json:"pairs"`
	CreatedAt time.Time      `json:"created_at"`
	CourseIDs []uint         `json:"course_ids,omitempty"` // 来源课程
	Stats     DatasetStats   `json:"stats"`
}

// DatasetStats 数据集统计信息。
type DatasetStats struct {
	TotalPairs    int     `json:"total_pairs"`
	AvgQueryLen   float64 `json:"avg_query_len"`
	AvgPassageLen float64 `json:"avg_passage_len"`
	CourseCount   int     `json:"course_count"`
}

// -- Internal Row Types -------------------------------------------

// interactionRow maps to a student interaction from the DB query.
type interactionRow struct {
	SessionID uint
	Content   string
	Role      string
	SkillID   string
	CourseID  uint
}

// chunkRow maps to a document chunk from the DB query.
type chunkRow struct {
	ID       uint
	CourseID uint
	Content  string
}

// -- Evaluation Metrics -------------------------------------------

// EvalMetrics Embedding 微调评估指标。
type EvalMetrics struct {
	RecallAt5  float64 `json:"recall_at_5"`
	RecallAt10 float64 `json:"recall_at_10"`
	NDCGAt10   float64 `json:"ndcg_at_10"`
	MRR        float64 `json:"mrr"`
}

// -- Fine-tune Pipeline -------------------------------------------

// FineTunePipeline Embedding 微调流水线。
type FineTunePipeline struct {
	DB        *gorm.DB
	LLM       llm.LLMProvider
	OutputDir string  // 输出目录（训练数据、评估报告）
	MinPairs  int     // 最小配对数据量（默认 100）
	MinScore  float64 // 最低相关性分数阈值（默认 0.6）
	BatchSize int     // Embedding 批量大小
}

// NewFineTunePipeline 创建 Embedding 微调流水线。
func NewFineTunePipeline(db *gorm.DB, llmClient llm.LLMProvider) *FineTunePipeline {
	return &FineTunePipeline{
		DB:        db,
		LLM:       llmClient,
		OutputDir: "data/embedding-finetune",
		MinPairs:  100,
		MinScore:  0.6,
		BatchSize: 32,
	}
}

// -- Step 1: Data Collection --------------------------------------

// CollectTrainingData 从交互日志中收集训练数据。
// 提取"学生提问"与"RAG 检索到的教材段落"的配对关系。
func (p *FineTunePipeline) CollectTrainingData(ctx context.Context, courseIDs []uint) (*TrainingDataset, error) {
	slogFinetune.Info("collecting training data", "courses", len(courseIDs))

	// 查询所有会话交互中学生提问及对应的检索上下文
	var rows []interactionRow
	query := p.DB.Table("interactions i").
		Select("i.session_id, i.content, i.role, i.skill_id, ss.current_kp as course_id").
		Joins("JOIN student_sessions ss ON ss.id = i.session_id").
		Where("i.role = ?", "student").
		Where("LENGTH(i.content) > ?", 10) // 过滤太短的输入

	if len(courseIDs) > 0 {
		// 通过 activity → course 关联
		query = query.Joins("JOIN learning_activities la ON la.id = ss.activity_id").
			Where("la.course_id IN ?", courseIDs)
	}

	if err := query.Limit(10000).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query interactions failed: %w", err)
	}

	// 查询对应课程的文档切片作为 passage
	var chunks []chunkRow
	chunkQuery := p.DB.Table("document_chunks").
		Select("id, course_id, content").
		Where("LENGTH(content) > ?", 50)

	if len(courseIDs) > 0 {
		chunkQuery = chunkQuery.Where("course_id IN ?", courseIDs)
	}

	if err := chunkQuery.Limit(50000).Find(&chunks).Error; err != nil {
		return nil, fmt.Errorf("query chunks failed: %w", err)
	}

	if len(rows) == 0 || len(chunks) == 0 {
		return nil, fmt.Errorf("insufficient data: %d interactions, %d chunks", len(rows), len(chunks))
	}

	// 使用 Embedding 相似度匹配 Query-Passage 配对
	pairs, err := p.matchQueryPassagePairs(ctx, rows, chunks)
	if err != nil {
		return nil, fmt.Errorf("pair matching failed: %w", err)
	}

	// 过滤低质量配对
	var filteredPairs []TrainingPair
	for _, pair := range pairs {
		if pair.Score >= p.MinScore {
			filteredPairs = append(filteredPairs, pair)
		}
	}

	// 统计
	stats := p.computeStats(filteredPairs, courseIDs)

	dataset := &TrainingDataset{
		Pairs:     filteredPairs,
		CreatedAt: time.Now(),
		CourseIDs: courseIDs,
		Stats:     stats,
	}

	slogFinetune.Info("collected training pairs", "filtered", len(filteredPairs), "total", len(pairs), "threshold", p.MinScore)

	return dataset, nil
}

// matchQueryPassagePairs 使用 Embedding 相似度匹配 Query-Passage 配对。
func (p *FineTunePipeline) matchQueryPassagePairs(ctx context.Context, interactions []interactionRow, chunks []chunkRow) ([]TrainingPair, error) {
	var pairs []TrainingPair
	pairID := uint(1)

	// 按批次生成 query embeddings
	for i := 0; i < len(interactions); i += p.BatchSize {
		end := i + p.BatchSize
		if end > len(interactions) {
			end = len(interactions)
		}

		batch := interactions[i:end]
		queryTexts := make([]string, len(batch))
		for j, inter := range batch {
			queryTexts[j] = inter.Content
		}

		queryEmbeddings, err := p.LLM.EmbedBatch(ctx, queryTexts)
		if err != nil {
			slogFinetune.Warn("batch embed failed", "offset", i, "err", err)
			continue
		}

		// 对每个 query，在同课程的 chunks 中找最相关的 passage
		for j, inter := range batch {
			if j >= len(queryEmbeddings) {
				break
			}

			bestScore := 0.0
			bestChunk := ""

			for _, chunk := range chunks {
				// 只匹配同一课程的 chunks
				if chunk.CourseID != inter.CourseID && inter.CourseID != 0 {
					continue
				}

				// 简单使用 chunk 的前缀做快速过滤
				chunkEmb, err := p.LLM.Embed(ctx, chunk.Content)
				if err != nil {
					continue
				}

				sim := cosineSimilarity(queryEmbeddings[j], chunkEmb)
				if sim > bestScore {
					bestScore = sim
					bestChunk = chunk.Content
				}
			}

			if bestChunk != "" && bestScore > 0 {
				pairs = append(pairs, TrainingPair{
					ID:       pairID,
					Query:    inter.Content,
					Passage:  bestChunk,
					CourseID: inter.CourseID,
					Score:    bestScore,
				})
				pairID++
			}
		}
	}

	return pairs, nil
}

// -- Step 2: InfoNCE Contrastive Loss Computation -----------------

// InfoNCEConfig InfoNCE 对比学习配置。
type InfoNCEConfig struct {
	Temperature     float64 `json:"temperature"`      // 温度参数 τ（默认 0.07）
	NegativeSamples int     `json:"negative_samples"` // 每个正样本的负样本数（默认 7）
}

// DefaultInfoNCEConfig 返回默认配置。
func DefaultInfoNCEConfig() InfoNCEConfig {
	return InfoNCEConfig{
		Temperature:     0.07,
		NegativeSamples: 7,
	}
}

// ComputeInfoNCELoss 计算 InfoNCE 对比学习损失。
// 用于评估当前 Embedding 模型在训练数据上的表现。
//
// InfoNCE Loss = -log(exp(sim(q,p+)/τ) / Σ exp(sim(q,pi)/τ))
//
// 其中 q=query, p+=正样本passage, pi=所有候选(正样本+负样本)
func (p *FineTunePipeline) ComputeInfoNCELoss(ctx context.Context, dataset *TrainingDataset, cfg InfoNCEConfig) (float64, error) {
	if len(dataset.Pairs) == 0 {
		return 0, fmt.Errorf("empty dataset")
	}

	slogFinetune.Info("computing infonce loss", "pairs", len(dataset.Pairs), "temperature", cfg.Temperature, "negatives", cfg.NegativeSamples)

	totalLoss := 0.0
	validCount := 0

	for i, pair := range dataset.Pairs {
		// 生成 query 和正样本 passage 的 embeddings
		queryEmb, err := p.LLM.Embed(ctx, pair.Query)
		if err != nil {
			continue
		}

		positiveEmb, err := p.LLM.Embed(ctx, pair.Passage)
		if err != nil {
			continue
		}

		// 从其他样本中随机选取负样本
		negativeEmbs := p.sampleNegativeEmbeddings(ctx, dataset.Pairs, i, cfg.NegativeSamples)

		// 计算 InfoNCE loss
		positiveSim := cosineSimilarity(queryEmb, positiveEmb) / cfg.Temperature

		// 分母：exp(正样本) + Σexp(负样本)
		denominator := math.Exp(positiveSim)
		for _, negEmb := range negativeEmbs {
			negSim := cosineSimilarity(queryEmb, negEmb) / cfg.Temperature
			denominator += math.Exp(negSim)
		}

		loss := -positiveSim + math.Log(denominator)
		totalLoss += loss
		validCount++

		if (i+1)%50 == 0 {
			slogFinetune.Debug("infonce progress", "processed", i+1, "total", len(dataset.Pairs), "avg_loss", totalLoss/float64(validCount))
		}
	}

	if validCount == 0 {
		return 0, fmt.Errorf("no valid pairs processed")
	}

	avgLoss := totalLoss / float64(validCount)
	slogFinetune.Info("infonce loss computed", "loss", avgLoss, "valid_pairs", validCount)

	return avgLoss, nil
}

// sampleNegativeEmbeddings 从数据集中随机选取负样本并生成 embeddings。
func (p *FineTunePipeline) sampleNegativeEmbeddings(ctx context.Context, pairs []TrainingPair, excludeIdx int, n int) [][]float64 {
	var embeddings [][]float64

	// 收集候选索引（排除当前正样本）
	candidates := make([]int, 0, len(pairs)-1)
	for i := range pairs {
		if i != excludeIdx {
			candidates = append(candidates, i)
		}
	}

	// Fisher-Yates 洗牌后取前 n 个
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(candidates) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}

	if n > len(candidates) {
		n = len(candidates)
	}

	for _, idx := range candidates[:n] {
		emb, err := p.LLM.Embed(ctx, pairs[idx].Passage)
		if err != nil {
			continue
		}
		embeddings = append(embeddings, emb)
	}

	return embeddings
}

// -- Step 3: Evaluation -------------------------------------------

// Evaluate 评估当前 Embedding 模型的检索质量。
// 使用 Recall@K, NDCG@K, MRR 指标。
func (p *FineTunePipeline) Evaluate(ctx context.Context, dataset *TrainingDataset, k int) (*EvalMetrics, error) {
	if len(dataset.Pairs) < 10 {
		return nil, fmt.Errorf("need at least 10 pairs for evaluation, got %d", len(dataset.Pairs))
	}

	slogFinetune.Info("evaluating retrieval quality", "k", k, "pairs", len(dataset.Pairs))

	// 将 20% 的数据作为测试集
	splitIdx := len(dataset.Pairs) * 80 / 100
	testPairs := dataset.Pairs[splitIdx:]

	// 所有 passage 组成检索语料库
	allPassages := make([]string, len(dataset.Pairs))
	for i, pair := range dataset.Pairs {
		allPassages[i] = pair.Passage
	}

	// 批量生成语料库 embeddings
	slogFinetune.Debug("generating corpus embeddings", "passages", len(allPassages))
	var corpusEmbeddings [][]float64
	for i := 0; i < len(allPassages); i += p.BatchSize {
		end := i + p.BatchSize
		if end > len(allPassages) {
			end = len(allPassages)
		}
		batch, err := p.LLM.EmbedBatch(ctx, allPassages[i:end])
		if err != nil {
			slogFinetune.Warn("batch embed failed", "err", err)
			continue
		}
		corpusEmbeddings = append(corpusEmbeddings, batch...)
	}

	if len(corpusEmbeddings) == 0 {
		return nil, fmt.Errorf("failed to generate corpus embeddings")
	}

	// 对每个测试 query 计算指标
	var totalRecall5, totalRecall10, totalNDCG10, totalMRR float64
	validCount := 0

	for _, testPair := range testPairs {
		queryEmb, err := p.LLM.Embed(ctx, testPair.Query)
		if err != nil {
			continue
		}

		// 找到正确答案的索引
		targetIdx := -1
		for i, passage := range allPassages {
			if passage == testPair.Passage {
				targetIdx = i
				break
			}
		}
		if targetIdx < 0 {
			continue
		}

		// 计算所有相似度并排序
		type scoredIdx struct {
			Idx   int
			Score float64
		}
		scores := make([]scoredIdx, len(corpusEmbeddings))
		for i, corpusEmb := range corpusEmbeddings {
			scores[i] = scoredIdx{Idx: i, Score: cosineSimilarity(queryEmb, corpusEmb)}
		}

		// 简单冒泡排序前 K 个（足够用）
		for i := 0; i < k && i < len(scores); i++ {
			for j := i + 1; j < len(scores); j++ {
				if scores[j].Score > scores[i].Score {
					scores[i], scores[j] = scores[j], scores[i]
				}
			}
		}

		// Recall@5
		recall5 := 0.0
		for i := 0; i < 5 && i < len(scores); i++ {
			if scores[i].Idx == targetIdx {
				recall5 = 1.0
				break
			}
		}
		totalRecall5 += recall5

		// Recall@10
		recall10 := 0.0
		kVal := k
		if kVal > len(scores) {
			kVal = len(scores)
		}
		for i := 0; i < kVal; i++ {
			if scores[i].Idx == targetIdx {
				recall10 = 1.0
				break
			}
		}
		totalRecall10 += recall10

		// MRR (Mean Reciprocal Rank)
		for i := 0; i < kVal; i++ {
			if scores[i].Idx == targetIdx {
				totalMRR += 1.0 / float64(i+1)
				break
			}
		}

		// NDCG@10
		ndcg := 0.0
		for i := 0; i < kVal; i++ {
			if scores[i].Idx == targetIdx {
				ndcg = 1.0 / math.Log2(float64(i+2)) // log2(rank+1)
				break
			}
		}
		totalNDCG10 += ndcg

		validCount++
	}

	if validCount == 0 {
		return nil, fmt.Errorf("no valid test pairs evaluated")
	}

	metrics := &EvalMetrics{
		RecallAt5:  totalRecall5 / float64(validCount),
		RecallAt10: totalRecall10 / float64(validCount),
		NDCGAt10:   totalNDCG10 / float64(validCount),
		MRR:        totalMRR / float64(validCount),
	}

	slogFinetune.Info("evaluation results",
		"recall_at_5", metrics.RecallAt5,
		"recall_at_10", metrics.RecallAt10,
		"ndcg_at_10", metrics.NDCGAt10,
		"mrr", metrics.MRR)

	return metrics, nil
}

// -- Export --------------------------------------------------------

// ExportDataset 将训练数据集导出为 JSONL 文件（用于外部微调工具）。
func (p *FineTunePipeline) ExportDataset(dataset *TrainingDataset, format string) (string, error) {
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir failed: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(p.OutputDir, fmt.Sprintf("training_pairs_%s.jsonl", timestamp))

	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("create file failed: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, pair := range dataset.Pairs {
		record := map[string]interface{}{
			"query":   pair.Query,
			"passage": pair.Passage,
			"score":   pair.Score,
		}
		if err := encoder.Encode(record); err != nil {
			return "", fmt.Errorf("encode pair failed: %w", err)
		}
	}

	slogFinetune.Info("exported training pairs", "count", len(dataset.Pairs), "file", filename)
	return filename, nil
}

// ExportEvalReport 导出评估报告。
func (p *FineTunePipeline) ExportEvalReport(metrics *EvalMetrics, loss float64) (string, error) {
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir failed: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(p.OutputDir, fmt.Sprintf("eval_report_%s.json", timestamp))

	report := map[string]interface{}{
		"metrics":      metrics,
		"infonce_loss": loss,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report failed: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("write report failed: %w", err)
	}

	slogFinetune.Info("exported eval report", "file", filename)
	return filename, nil
}

// -- Helpers -------------------------------------------------------

// cosineSimilarity 计算两个向量的余弦相似度。
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// computeStats 计算数据集统计信息。
func (p *FineTunePipeline) computeStats(pairs []TrainingPair, courseIDs []uint) DatasetStats {
	if len(pairs) == 0 {
		return DatasetStats{}
	}

	var totalQueryLen, totalPassageLen float64
	courseSet := make(map[uint]struct{})

	for _, pair := range pairs {
		totalQueryLen += float64(len([]rune(pair.Query)))
		totalPassageLen += float64(len([]rune(pair.Passage)))
		courseSet[pair.CourseID] = struct{}{}
	}

	return DatasetStats{
		TotalPairs:    len(pairs),
		AvgQueryLen:   totalQueryLen / float64(len(pairs)),
		AvgPassageLen: totalPassageLen / float64(len(pairs)),
		CourseCount:   len(courseSet),
	}
}
