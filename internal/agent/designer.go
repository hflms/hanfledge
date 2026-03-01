package agent

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/search"
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
	db         *gorm.DB
	llm        llm.LLMProvider
	neo4j      *neo4jRepo.Client
	karag      *usecase.KARAGEngine
	truncator  *TokenTruncator          // Token 截断中间件 (§8.2.2)
	expander   *QueryExpander           // RAG-Fusion 查询扩展 (§8.1.2)
	gateway    *QualityGateway          // CRAG 质量网关 (§8.1.2)
	reranker   *CrossEncoderReranker    // Cross-Encoder 精重排 (§8.1.1 Stage 2)
	searchConn *search.DynamicConnector // Web search fallback (§8.1.2, nil-safe)
}

// NewDesignerAgent 创建设计师 Agent。
func NewDesignerAgent(db *gorm.DB, llmClient llm.LLMProvider, neo4jClient *neo4jRepo.Client, karag *usecase.KARAGEngine, searchConnector *search.DynamicConnector) *DesignerAgent {
	return &DesignerAgent{
		db:         db,
		llm:        llmClient,
		neo4j:      neo4jClient,
		karag:      karag,
		truncator:  DefaultTokenTruncator(),
		expander:   NewQueryExpander(llmClient),
		gateway:    NewQualityGateway(),
		reranker:   NewCrossEncoderReranker(llmClient),
		searchConn: searchConnector,
	}
}

// Name 返回 Agent 名称。
func (a *DesignerAgent) Name() string { return "Designer" }

// Assemble 根据学习处方检索并组装个性化学习材料。
// Pipeline (§8.1.1 + §8.1.2):
// 1. RAG-Fusion 查询扩展 — 原始查询 → N 个变体
// 2. 多路语义检索 — 每个变体独立检索 Top-50
// 3. Neo4j 图谱引导检索 Top-50
// 4. RRF 多路融合排序 → Top-20 (粗排候选池)
// 4.5. Cross-Encoder 精重排 → Top-5 (§8.1.1 Stage 2)
// 5. CRAG 质量网关 — 评估检索质量，低质量时触发回退
// 6. Token 截断 + 图谱上下文 + 系统 Prompt 组装
func (a *DesignerAgent) Assemble(ctx context.Context, prescription LearningPrescription, userInput string) (PersonalizedMaterial, error) {
	log.Printf("🎨 [Designer] Assembling material for student=%d, %d KP targets",
		prescription.StudentID, len(prescription.TargetKPSequence))

	// Step 1: 获取课程 ID（从活动关联）
	courseID, err := a.getCourseIDFromSession(prescription.SessionID)
	if err != nil {
		return PersonalizedMaterial{}, fmt.Errorf("get course_id: %w", err)
	}

	// Step 2: RAG-Fusion 查询扩展 (§8.1.2)
	// 将原始查询扩展为多个学术化变体，提升召回覆盖面
	expandedQueries := a.expander.ExpandQuery(ctx, userInput)
	log.Printf("   → RAG-Fusion: %d query variants", len(expandedQueries))

	// Step 3: 多路语义检索
	// 3a. 对每个变体独立执行 pgvector 语义检索 Top-50
	var allSemanticChunks []RetrievedChunk
	for i, query := range expandedQueries {
		chunks, err := a.semanticSearch(ctx, courseID, query)
		if err != nil {
			log.Printf("⚠️  [Designer] Semantic search failed for variant %d: %v", i, err)
			continue
		}
		allSemanticChunks = append(allSemanticChunks, chunks...)
	}

	// 3b. 图谱引导检索 — Neo4j → KP titles → pgvector Top-50
	graphChunks, err := a.graphContentSearch(ctx, courseID, prescription.TargetKPSequence)
	if err != nil {
		log.Printf("⚠️  [Designer] Graph content search failed: %v", err)
	}

	// Step 4: RRF 多路融合排序 → Top-20 (粗排候选池)
	// 合并所有语义检索变体结果 + 图谱检索结果
	mergedChunks := rrfMerge(allSemanticChunks, graphChunks, 20)

	log.Printf("   → RRF merge: semantic=%d (from %d variants) + graph=%d → merged=%d",
		len(allSemanticChunks), len(expandedQueries), len(graphChunks), len(mergedChunks))

	// Step 4.5: Cross-Encoder 精重排 (§8.1.1 Stage 2)
	// 对 RRF 粗排候选池中的每个 chunk 与原始查询做深度语义评分，精选 Top-5
	mergedChunks = a.reranker.Rerank(ctx, userInput, mergedChunks)

	// Step 5: CRAG 质量网关 (§8.1.2)
	// 评估检索结果与查询的相关性，低质量时触发回退
	relevance := a.gateway.EvaluateRelevance(mergedChunks, userInput)

	// Step 5.5: Token 截断 (§8.2.2) — 防止 "Lost in the Middle"
	truncResult := a.truncator.TruncateChunks(mergedChunks)
	mergedChunks = truncResult.Data
	if truncResult.Truncated {
		log.Printf("   → Truncator: %d→%d chunks (%d pages total)",
			truncResult.TotalItems, len(mergedChunks), truncResult.TotalPages)
	}

	// Step 6: 图谱上下文 — 知识点关系（用于 Prompt 和前端展示）
	graphNodes := a.graphSearch(ctx, prescription.TargetKPSequence)

	// Step 6.5: 误区加载 — 谬误侦探技能激活时加载目标 KP 的误区 (§5.2 Step 5)
	var misconceptions []MisconceptionItem
	if isFallacyDetectiveSkill(prescription.RecommendedSkill) {
		misconceptions = a.loadMisconceptions(prescription.TargetKPSequence)
		log.Printf("   → Misconceptions: %d items loaded for fallacy-detective", len(misconceptions))
	}

	// Step 7: 组装系统 Prompt
	systemPrompt := a.buildSystemPrompt(prescription, mergedChunks, graphNodes, misconceptions)

	// Step 7.5: CRAG 回退处理 — 如果检索质量不达标，触发回退策略
	if !relevance.Passed {
		if a.searchConn != nil {
			// §8.1.2: CRAG → fail → Dynamic Connector → Web Search Enrichment
			systemPrompt = a.gateway.HandleFallbackWithSearch(ctx, systemPrompt, userInput, a.searchConn)
		} else {
			systemPrompt = a.gateway.HandleFallback(systemPrompt)
		}
	}

	// Step 8: System Prompt Token 截断 — 防止超长上下文
	systemPrompt, promptTruncated := a.truncator.TruncateSystemPrompt(systemPrompt, 2048)
	if promptTruncated {
		log.Printf("   → Truncator: system prompt truncated to 2048 tokens")
	}

	material := PersonalizedMaterial{
		SessionID:       prescription.SessionID,
		Prescription:    prescription,
		RetrievedChunks: mergedChunks,
		GraphContext:    graphNodes,
		Misconceptions:  misconceptions,
		SystemPrompt:    systemPrompt,
	}

	log.Printf("   → Material: %d chunks, %d graph nodes, prompt=%d chars, CRAG=%v (avg=%.4f)",
		len(mergedChunks), len(graphNodes), len(systemPrompt), relevance.Passed, relevance.AvgScore)

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
func (a *DesignerAgent) buildSystemPrompt(prescription LearningPrescription, chunks []RetrievedChunk, nodes []GraphNode, misconceptions []MisconceptionItem) string {
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

	// 误区材料（谬误侦探技能专用 §5.2 Step 5）
	if len(misconceptions) > 0 {
		sb.WriteString("【已知误区库 — 仅供你生成谬误挑战时参考，不可直接展示给学生】\n")
		sb.WriteString("以下是该知识点的常见认知陷阱，你应从中选取或改编来设计谬误挑战：\n")
		for i, m := range misconceptions {
			trapLabel := trapTypeLabel(m.TrapType)
			sb.WriteString(fmt.Sprintf("%d. [%s|严重度:%.1f] %s\n", i+1, trapLabel, m.Severity, m.Description))
		}
		sb.WriteString("\n")
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

// ── Fallacy Detective Support (§5.2 Step 5) ────────────────

// fallacyDetectiveSkillID 是谬误侦探技能的标准 ID。
const fallacyDetectiveSkillID = "general_assessment_fallacy"

// isFallacyDetectiveSkill 判断技能 ID 是否为谬误侦探。
// 兼容旧 ID (fallacy-detective) 和新 ID (general_assessment_fallacy)。
func isFallacyDetectiveSkill(skillID string) bool {
	return skillID == fallacyDetectiveSkillID || skillID == "fallacy-detective"
}

// loadMisconceptions 从 PostgreSQL 加载目标 KP 的误区列表。
// 按严重度降序排列，最多返回 5 个（防止 Prompt 过长）。
func (a *DesignerAgent) loadMisconceptions(targets []KnowledgePointTarget) []MisconceptionItem {
	if a.db == nil || len(targets) == 0 {
		return nil
	}

	kpIDs := make([]uint, len(targets))
	for i, t := range targets {
		kpIDs[i] = t.KPID
	}

	var misconceptions []model.Misconception
	a.db.Where("kp_id IN ?", kpIDs).
		Order("severity DESC").
		Limit(5).
		Find(&misconceptions)

	if len(misconceptions) == 0 {
		return nil
	}

	// 预加载 KP 标题
	kpTitles := make(map[uint]string)
	for _, t := range targets {
		if _, ok := kpTitles[t.KPID]; !ok {
			kpTitles[t.KPID] = a.getKPTitle(t.KPID)
		}
	}

	items := make([]MisconceptionItem, len(misconceptions))
	for i, m := range misconceptions {
		items[i] = MisconceptionItem{
			KPID:        m.KPID,
			KPTitle:     kpTitles[m.KPID],
			Description: m.Description,
			TrapType:    string(m.TrapType),
			Severity:    m.Severity,
		}
	}

	return items
}

// trapTypeLabel 返回误区类型的中文标签。
func trapTypeLabel(trapType string) string {
	switch trapType {
	case "conceptual":
		return "概念性"
	case "procedural":
		return "操作性"
	case "intuitive":
		return "直觉性"
	case "transfer":
		return "迁移性"
	default:
		return trapType
	}
}
