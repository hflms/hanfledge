package agent

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"github.com/hflms/hanfledge/internal/usecase"
	"gorm.io/gorm"
)

// ============================
// Designer Agent — 设计师
// ============================
//
// 职责：根据策略师的学习处方，执行 RRF 混合检索，组装个性化学习材料。
// 输入：LearningPrescription + 用户输入
// 输出：PersonalizedMaterial（检索片段 + 图谱上下文 + 系统 Prompt）

// DesignerAgent 设计师 Agent。
type DesignerAgent struct {
	db    *gorm.DB
	llm   *llm.OllamaClient
	neo4j *neo4jRepo.Client
	karag *usecase.KARAGEngine
}

// NewDesignerAgent 创建设计师 Agent。
func NewDesignerAgent(db *gorm.DB, llmClient *llm.OllamaClient, neo4jClient *neo4jRepo.Client, karag *usecase.KARAGEngine) *DesignerAgent {
	return &DesignerAgent{
		db:    db,
		llm:   llmClient,
		neo4j: neo4jClient,
		karag: karag,
	}
}

// Name 返回 Agent 名称。
func (a *DesignerAgent) Name() string { return "Designer" }

// Assemble 根据学习处方检索并组装个性化学习材料。
// 1. pgvector 语义检索 Top-50
// 2. Neo4j 图谱引导检索 Top-50
// 3. RRF 融合排序 → Top-10
// 4. 图谱上下文（知识点关系）
// 5. 组装系统 Prompt
func (a *DesignerAgent) Assemble(ctx context.Context, prescription LearningPrescription, userInput string) (PersonalizedMaterial, error) {
	log.Printf("🎨 [Designer] Assembling material for student=%d, %d KP targets",
		prescription.StudentID, len(prescription.TargetKPSequence))

	// Step 1: 获取课程 ID（从活动关联）
	courseID, err := a.getCourseIDFromSession(prescription.SessionID)
	if err != nil {
		return PersonalizedMaterial{}, fmt.Errorf("get course_id: %w", err)
	}

	// Step 2: 双路检索
	// 2a. 语义检索 — pgvector cosine similarity Top-50
	semanticChunks, err := a.semanticSearch(ctx, courseID, userInput)
	if err != nil {
		log.Printf("⚠️  [Designer] Semantic search failed: %v", err)
	}

	// 2b. 图谱引导检索 — Neo4j → KP titles → pgvector Top-50
	graphChunks, err := a.graphContentSearch(ctx, courseID, prescription.TargetKPSequence)
	if err != nil {
		log.Printf("⚠️  [Designer] Graph content search failed: %v", err)
	}

	// Step 3: RRF 融合排序 → Top-10
	mergedChunks := rrfMerge(semanticChunks, graphChunks, 10)

	log.Printf("   → RRF merge: semantic=%d + graph=%d → merged=%d",
		len(semanticChunks), len(graphChunks), len(mergedChunks))

	// Step 4: 图谱上下文 — 知识点关系（用于 Prompt 和前端展示）
	graphNodes := a.graphSearch(ctx, prescription.TargetKPSequence)

	// Step 5: 组装系统 Prompt
	systemPrompt := a.buildSystemPrompt(prescription, mergedChunks, graphNodes)

	material := PersonalizedMaterial{
		SessionID:       prescription.SessionID,
		Prescription:    prescription,
		RetrievedChunks: mergedChunks,
		GraphContext:    graphNodes,
		SystemPrompt:    systemPrompt,
	}

	log.Printf("   → Material: %d chunks, %d graph nodes, prompt=%d chars",
		len(mergedChunks), len(graphNodes), len(systemPrompt))

	return material, nil
}

// ── Retrieval Methods ───────────────────────────────────────

// rrfK is the constant for Reciprocal Rank Fusion: RRF(d) = 1/(k+rank).
// Standard value k=60 as per the original RRF paper.
const rrfK = 60

// semanticSearch 使用 KA-RAG 引擎执行语义检索 Top-50。
func (a *DesignerAgent) semanticSearch(ctx context.Context, courseID uint, query string) ([]RetrievedChunk, error) {
	if a.karag == nil {
		return nil, nil
	}

	results, err := a.karag.SemanticSearch(ctx, courseID, query, 50)
	if err != nil {
		return nil, err
	}

	chunks := make([]RetrievedChunk, len(results))
	for i, r := range results {
		chunks[i] = RetrievedChunk{
			Content:    r.Content,
			Source:     "semantic",
			Score:      r.Similarity,
			ChunkIndex: r.ChunkIndex,
		}
	}
	return chunks, nil
}

// graphContentSearch 使用 Neo4j 图谱检索相关 KP，再用 KP 标题做语义检索。
// 这是 RRF 的第二路检索：graph-guided content retrieval。
func (a *DesignerAgent) graphContentSearch(ctx context.Context, courseID uint, targets []KnowledgePointTarget) ([]RetrievedChunk, error) {
	if a.neo4j == nil || a.karag == nil {
		return nil, nil
	}

	// Step 1: 从图谱获取相关 KP IDs
	kpIDs := make([]uint, len(targets))
	for i, t := range targets {
		kpIDs[i] = t.KPID
	}

	graphResults, err := a.neo4j.SearchRelatedKPs(ctx, kpIDs, 50)
	if err != nil {
		return nil, fmt.Errorf("graph search failed: %w", err)
	}

	if len(graphResults) == 0 {
		return nil, nil
	}

	// Step 2: 使用图谱节点的标题作为查询，在 pgvector 中检索相关文档片段
	// 合并所有 KP 标题为一个查询
	var queryParts []string
	for _, r := range graphResults {
		if r.KPTitle != "" {
			queryParts = append(queryParts, r.KPTitle)
		}
	}
	if len(queryParts) == 0 {
		return nil, nil
	}

	// 最多取前 10 个 KP 标题组合查询（防止查询过长）
	if len(queryParts) > 10 {
		queryParts = queryParts[:10]
	}
	combinedQuery := strings.Join(queryParts, " ")

	results, err := a.karag.SemanticSearch(ctx, courseID, combinedQuery, 50)
	if err != nil {
		return nil, fmt.Errorf("graph-guided semantic search failed: %w", err)
	}

	chunks := make([]RetrievedChunk, len(results))
	for i, r := range results {
		chunks[i] = RetrievedChunk{
			Content:    r.Content,
			Source:     "graph",
			Score:      r.Similarity,
			ChunkIndex: r.ChunkIndex,
		}
	}
	return chunks, nil
}

// graphSearch 使用 Neo4j 检索目标 KP 的关联知识点（用于图谱上下文展示）。
func (a *DesignerAgent) graphSearch(ctx context.Context, targets []KnowledgePointTarget) []GraphNode {
	if a.neo4j == nil {
		return nil
	}

	var nodes []GraphNode
	seen := make(map[string]bool)

	for _, t := range targets {
		// 添加目标节点本身
		kpID := fmt.Sprintf("kp_%d", t.KPID)
		if !seen[kpID] {
			// 查询 KP 标题
			title := a.getKPTitle(t.KPID)
			nodes = append(nodes, GraphNode{
				ID:       kpID,
				Title:    title,
				Relation: "target",
				Depth:    0,
			})
			seen[kpID] = true
		}

		// 查询前置知识
		prereqs, err := a.neo4j.GetPrerequisites(ctx, t.KPID)
		if err != nil {
			log.Printf("⚠️  [Designer] Get prereqs for kp=%d failed: %v", t.KPID, err)
			continue
		}

		for _, p := range prereqs {
			id, _ := p["id"].(string)
			title, _ := p["title"].(string)
			depth, _ := p["depth"].(int64)

			if !seen[id] {
				nodes = append(nodes, GraphNode{
					ID:       id,
					Title:    title,
					Relation: "prerequisite",
					Depth:    int(depth),
				})
				seen[id] = true
			}
		}
	}

	return nodes
}

// ── RRF Merge ───────────────────────────────────────────────

// rrfMerge implements Reciprocal Rank Fusion to merge two ranked result lists.
// RRF score for document d: sum over all lists of 1/(k + rank_in_list).
// Documents are identified by ChunkIndex for deduplication.
// Returns the top-N results sorted by fused score.
func rrfMerge(semantic, graph []RetrievedChunk, topN int) []RetrievedChunk {
	type fusedDoc struct {
		chunk    RetrievedChunk
		rrfScore float64
	}

	// Map: ChunkIndex → fusedDoc
	docMap := make(map[int]*fusedDoc)

	// Add semantic retrieval scores
	for rank, chunk := range semantic {
		key := chunk.ChunkIndex
		if _, exists := docMap[key]; !exists {
			docMap[key] = &fusedDoc{chunk: chunk}
		}
		docMap[key].rrfScore += 1.0 / float64(rrfK+rank+1)
	}

	// Add graph retrieval scores
	for rank, chunk := range graph {
		key := chunk.ChunkIndex
		if _, exists := docMap[key]; !exists {
			docMap[key] = &fusedDoc{chunk: chunk}
		}
		docMap[key].rrfScore += 1.0 / float64(rrfK+rank+1)
		// Mark source as "hybrid" if it appeared in both lists
		if docMap[key].chunk.Source == "semantic" {
			docMap[key].chunk.Source = "hybrid"
		}
	}

	// Collect and sort by RRF score descending
	docs := make([]fusedDoc, 0, len(docMap))
	for _, d := range docMap {
		docs = append(docs, *d)
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].rrfScore > docs[j].rrfScore
	})

	// Take top-N
	if topN > len(docs) {
		topN = len(docs)
	}

	result := make([]RetrievedChunk, topN)
	for i := 0; i < topN; i++ {
		result[i] = docs[i].chunk
		result[i].Score = docs[i].rrfScore // Replace original score with RRF score
	}

	return result
}

// ── Prompt Assembly ─────────────────────────────────────────

// buildSystemPrompt 根据检索结果和处方组装系统 Prompt。
func (a *DesignerAgent) buildSystemPrompt(prescription LearningPrescription, chunks []RetrievedChunk, nodes []GraphNode) string {
	var sb strings.Builder

	sb.WriteString("你是一位 AI 学习教练，正在帮助学生学习。\n\n")

	// 支架说明
	switch prescription.InitialScaffold {
	case ScaffoldHigh:
		sb.WriteString("【教学策略：高支架模式】\n")
		sb.WriteString("- 提供分步引导和关键词提示\n")
		sb.WriteString("- 使用苏格拉底式提问，引导学生逐步思考\n")
		sb.WriteString("- 不直接给出答案\n\n")
	case ScaffoldMedium:
		sb.WriteString("【教学策略：中等支架模式】\n")
		sb.WriteString("- 提供关键概念标签和方向性提示\n")
		sb.WriteString("- 鼓励学生独立推理，适当辅助\n\n")
	case ScaffoldLow:
		sb.WriteString("【教学策略：低支架模式】\n")
		sb.WriteString("- 学生已有较好基础，仅做开放式引导\n")
		sb.WriteString("- 鼓励深度思考和创新性回答\n\n")
	}

	// 前置知识差距提醒
	if len(prescription.PrereqGaps) > 0 {
		sb.WriteString("【注意：学生在以下前置知识存在薄弱环节】\n")
		for _, gap := range prescription.PrereqGaps {
			sb.WriteString("- " + gap + "\n")
		}
		sb.WriteString("\n")
	}

	// 知识图谱上下文
	if len(nodes) > 0 {
		sb.WriteString("【相关知识点图谱】\n")
		for _, n := range nodes {
			prefix := "📍"
			if n.Relation == "prerequisite" {
				prefix = "🔗"
			}
			sb.WriteString(fmt.Sprintf("%s %s (关系: %s)\n", prefix, n.Title, n.Relation))
		}
		sb.WriteString("\n")
	}

	// 检索到的参考材料
	if len(chunks) > 0 {
		sb.WriteString("【参考材料】\n")
		for i, c := range chunks {
			if i >= 5 { // 最多放 5 个片段到 Prompt
				break
			}
			sb.WriteString(fmt.Sprintf("--- 片段 %d (相关度: %.2f) ---\n%s\n\n", i+1, c.Score, c.Content))
		}
	}

	return sb.String()
}

// ── Database Helpers ────────────────────────────────────────

// getCourseIDFromSession 通过 session → activity → course 获取课程 ID。
func (a *DesignerAgent) getCourseIDFromSession(sessionID uint) (uint, error) {
	var result struct {
		CourseID uint
	}
	err := a.db.Raw(`
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

// getKPTitle 查询知识点标题。
func (a *DesignerAgent) getKPTitle(kpID uint) string {
	var kp struct{ Title string }
	a.db.Raw("SELECT title FROM knowledge_points WHERE id = ?", kpID).Scan(&kp)
	return kp.Title
}
