package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/plugin"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"gorm.io/gorm"
)

var slogKARAG = logger.L("KA-RAG")

// KARAGEngine implements the Knowledge-Augmented RAG pipeline.
// It handles document slicing, embedding, graph building, and outline generation.
type KARAGEngine struct {
	DB       *gorm.DB
	Neo4j    *neo4jRepo.Client
	LLM      llm.LLMProvider
	EventBus *plugin.EventBus // Plugin event bus (nil-safe)
}

// NewKARAGEngine creates a new KA-RAG engine.
func NewKARAGEngine(db *gorm.DB, neo4j *neo4jRepo.Client, llmClient llm.LLMProvider, eventBus *plugin.EventBus) *KARAGEngine {
	return &KARAGEngine{DB: db, Neo4j: neo4j, LLM: llmClient, EventBus: eventBus}
}

// ProcessDocument runs the full KA-RAG pipeline for an uploaded document.
// 1. Extract text → 2. Hybrid Slicing → 3. Store chunks → 4. Embed → 5. Build graph
func (e *KARAGEngine) ProcessDocument(ctx context.Context, doc *model.Document, rawText string) error {
	// Update status to processing
	e.DB.Model(doc).Update("status", model.DocStatusProcessing)

	// Step 1: Hybrid Slicing
	slogKARAG.Info("slicing document", "file", doc.FileName)

	// Hook: before slicing (abortable — plugins may reject the document)
	if err := e.publishAbortable(ctx, plugin.HookBeforeSlicing, map[string]interface{}{
		"document_id": doc.ID,
		"file_name":   doc.FileName,
	}); err != nil {
		return fmt.Errorf("aborted by HookBeforeSlicing: %w", err)
	}

	chunks := e.hybridSlice(rawText)
	slogKARAG.Info("generated chunks", "count", len(chunks))

	// Hook: after slicing
	e.publishEvent(ctx, plugin.HookAfterSlicing, map[string]interface{}{
		"document_id": doc.ID,
		"chunk_count": len(chunks),
	})

	// Step 2: Store chunks in PostgreSQL
	var chunkIDs []uint
	for i, content := range chunks {
		chunk := model.DocumentChunk{
			DocumentID: doc.ID,
			CourseID:   doc.CourseID,
			ChunkIndex: i,
			Content:    content,
			TokenCount: utf8.RuneCountInString(content),
			// Initialize with an empty valid vector. Since pgvector requires valid format like "[0,0,0...]"
			// if the field is omitted but gorm tries to insert "" for a string field.
			// It is better to use `gorm:"default:NULL"` or let it be if GORM allows omit,
			// but we will pass a dummy zero array for now, or just let DB default handle it
			// if we change the struct tag. Actually, it's safer to not insert it if we change gorm mapping.
		}
		if err := e.DB.Omit("Embedding").Create(&chunk).Error; err != nil {
			return fmt.Errorf("store chunk %d failed: %w", i, err)
		}
		chunkIDs = append(chunkIDs, chunk.ID)
	}

	// Step 3: Generate embeddings and store in pgvector
	slogKARAG.Info("generating embeddings", "chunks", len(chunks))

	// Hook: before embedding (abortable — plugins may skip embedding)
	if err := e.publishAbortable(ctx, plugin.HookBeforeEmbedding, map[string]interface{}{
		"document_id": doc.ID,
		"chunk_count": len(chunks),
	}); err != nil {
		return fmt.Errorf("aborted by HookBeforeEmbedding: %w", err)
	}

	e.generateEmbeddings(ctx, chunks, chunkIDs)

	// Step 4: Use LLM to extract knowledge structure and build graph
	slogKARAG.Info("extracting knowledge structure via llm")
	if err := e.buildKnowledgeGraph(ctx, doc.CourseID, chunks); err != nil {
		return fmt.Errorf("build knowledge graph failed: %w", err)
	}

	// Hook: after graph build (only on success)
	e.publishEvent(ctx, plugin.HookAfterGraphBuild, map[string]interface{}{
		"course_id":   doc.CourseID,
		"document_id": doc.ID,
	})

	// Mark as completed
	e.DB.Model(doc).Update("status", model.DocStatusCompleted)
	slogKARAG.Info("document processing complete", "file", doc.FileName)
	return nil
}

// generateEmbeddings calls the embedding model for each chunk and stores vectors.
func (e *KARAGEngine) generateEmbeddings(ctx context.Context, chunks []string, chunkIDs []uint) {
	for i, content := range chunks {
		vec, err := e.LLM.Embed(ctx, content)
		if err != nil {
			slogKARAG.Warn("embedding chunk failed", "chunk", i, "err", err)
			continue
		}

		// Format as pgvector string: [0.1,0.2,0.3,...]
		vecStr := formatVector(vec)
		e.DB.Exec("UPDATE document_chunks SET embedding = ? WHERE id = ?", vecStr, chunkIDs[i])
	}
	slogKARAG.Info("embedded chunks", "count", len(chunks))
}

// formatVector converts a float64 slice to pgvector format string.
func formatVector(vec []float64) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// SemanticSearch performs cosine similarity search on document chunks.
// Returns the top-K most relevant chunks for the given query.
func (e *KARAGEngine) SemanticSearch(ctx context.Context, courseID uint, query string, topK int) ([]SearchResult, error) {
	// Generate query embedding
	queryVec, err := e.LLM.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query embedding failed: %w", err)
	}
	vecStr := formatVector(queryVec)

	// Cosine similarity search via pgvector
	var results []SearchResult
	err = e.DB.Raw(`
		SELECT id, content, chunk_index, 
		       1 - (embedding <=> ?::vector) AS similarity
		FROM document_chunks
		WHERE course_id = ? AND embedding IS NOT NULL
		ORDER BY embedding <=> ?::vector
		LIMIT ?
	`, vecStr, courseID, vecStr, topK).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}
	return results, nil
}

// SearchResult represents a single search result with similarity score.
type SearchResult struct {
	ID         uint    `json:"id"`
	Content    string  `json:"content"`
	ChunkIndex int     `json:"chunk_index"`
	Similarity float64 `json:"similarity"`
}

// hybridSlice splits text into logical chunks using paragraph-based splitting.
// Adjacent chunks with high similarity are merged to preserve educational coherence.
func (e *KARAGEngine) hybridSlice(text string) []string {
	// Split by double newlines (paragraph boundaries)
	paragraphs := strings.Split(text, "\n\n")

	var chunks []string
	var currentChunk strings.Builder
	maxChunkSize := 500 // ~500 characters per chunk

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// If adding this paragraph would exceed max size, save current chunk
		if currentChunk.Len() > 0 && currentChunk.Len()+len(para) > maxChunkSize {
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
			currentChunk.Reset()
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)
	}

	// Don't forget the last chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	// Filter out very short chunks (likely noise)
	var filtered []string
	for _, c := range chunks {
		if utf8.RuneCountInString(c) >= 20 {
			filtered = append(filtered, c)
		}
	}

	if len(filtered) == 0 && len(chunks) > 0 {
		return chunks // fallback: return all if filtering removed everything
	}
	return filtered
}

// buildKnowledgeGraph uses LLM to extract chapters, knowledge points,
// and their relationships, then writes them to Neo4j.
func (e *KARAGEngine) buildKnowledgeGraph(ctx context.Context, courseID uint, chunks []string) error {
	// Prepare a summary of the document for LLM analysis
	// Use first 10 chunks as representative sample
	sampleSize := 10
	if len(chunks) < sampleSize {
		sampleSize = len(chunks)
	}
	sample := strings.Join(chunks[:sampleSize], "\n---\n")

	prompt := fmt.Sprintf(`你是一位教学大纲分析专家。请分析以下教材内容，提取出章节和知识点结构。

请以如下 JSON 格式返回（不要返回其他内容）：
{
  "chapters": [
    {
      "title": "章节名称",
      "knowledge_points": [
        {
          "title": "知识点名称",
          "difficulty": 0.5,
          "is_key_point": true,
          "prerequisites": ["前置知识点名称"]
        }
      ]
    }
  ]
}

教材内容节选：
%s`, sample)

	response, err := e.LLM.Chat(ctx, []llm.ChatMessage{
		{Role: "system", Content: "你是教育领域的结构化数据提取专家。只返回纯 JSON，不要返回其他内容。"},
		{Role: "user", Content: prompt},
	}, &llm.ChatOptions{Temperature: 0.1})

	if err != nil {
		return fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse LLM response and create graph nodes
	return e.parseAndStoreOutline(ctx, courseID, response)
}

// OutlineJSON represents the LLM-extracted outline structure.
type OutlineJSON struct {
	Chapters []OutlineChapter `json:"chapters"`
}

// OutlineChapter represents a chapter in the extracted outline.
type OutlineChapter struct {
	Title           string      `json:"title"`
	KnowledgePoints []OutlineKP `json:"knowledge_points"`
}

// OutlineKP represents a knowledge point in the extracted outline.
type OutlineKP struct {
	Title         string   `json:"title"`
	Difficulty    float64  `json:"difficulty"`
	IsKeyPoint    bool     `json:"is_key_point"`
	Prerequisites []string `json:"prerequisites"`
}

// parseAndStoreOutline parses the LLM JSON response and stores
// chapters/KPs in both PostgreSQL and Neo4j.
func (e *KARAGEngine) parseAndStoreOutline(ctx context.Context, courseID uint, llmResponse string) error {
	// Extract JSON from response (LLM may wrap it in markdown code blocks)
	jsonStr := extractJSON(llmResponse)

	var outline OutlineJSON
	if err := parseJSONSafe(jsonStr, &outline); err != nil {
		return fmt.Errorf("invalid outline json: %w", err)
	}

	if len(outline.Chapters) == 0 {
		return fmt.Errorf("outline is empty")
	}

	// Create course node in Neo4j
	var course model.Course
	e.DB.First(&course, courseID)
	if e.Neo4j != nil {
		e.Neo4j.CreateCourseGraph(ctx, courseID, course.Title, course.Subject)
	}

	// Store chapters and KPs
	kpTitleToID := make(map[string]uint) // for prerequisite linking

	for i, ch := range outline.Chapters {
		// PostgreSQL
		chapter := model.Chapter{
			CourseID:  courseID,
			Title:     ch.Title,
			SortOrder: i + 1,
		}
		e.DB.Create(&chapter)

		// Neo4j
		if e.Neo4j != nil {
			e.Neo4j.CreateChapterNode(ctx, courseID, chapter.ID, ch.Title, i+1)
		}

		for _, kpData := range ch.KnowledgePoints {
			kp := model.KnowledgePoint{
				ChapterID:  chapter.ID,
				Title:      kpData.Title,
				Difficulty: kpData.Difficulty,
				IsKeyPoint: kpData.IsKeyPoint,
			}
			e.DB.Create(&kp)

			// Set Neo4j node ID
			neo4jID := fmt.Sprintf("kp_%d", kp.ID)
			e.DB.Model(&kp).Update("neo4j_node_id", neo4jID)

			// Neo4j
			if e.Neo4j != nil {
				e.Neo4j.CreateKnowledgePointNode(ctx, chapter.ID, kp.ID, kpData.Title, kpData.Difficulty)
			}

			kpTitleToID[kpData.Title] = kp.ID
		}
	}

	// Create prerequisite relationships
	if e.Neo4j != nil {
		for _, ch := range outline.Chapters {
			for _, kpData := range ch.KnowledgePoints {
				fromID, ok := kpTitleToID[kpData.Title]
				if !ok {
					continue
				}
				for _, prereqTitle := range kpData.Prerequisites {
					if toID, ok := kpTitleToID[prereqTitle]; ok {
						e.Neo4j.CreateRequiresRelation(ctx, fromID, toID)
					}
				}
			}
		}
	}

	slogKARAG.Info("stored chapters", "count", len(outline.Chapters))
	return nil
}

// extractJSON extracts JSON content from a string that may contain markdown fences.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Remove markdown code fences
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.LastIndex(s, "```"); idx > 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx > 0 {
			s = s[:idx]
		}
	}

	// Fallback to find the first { and last } if there is extra text
	s = strings.TrimSpace(s)
	startIdx := strings.Index(s, "{")
	endIdx := strings.LastIndex(s, "}")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		s = s[startIdx : endIdx+1]
	}

	return strings.TrimSpace(s)
}

// parseJSONSafe parses JSON with error handling.
func parseJSONSafe(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// -- EventBus Helpers ----------------------------------------

// publishEvent fires an EventBus event if the bus is available.
func (e *KARAGEngine) publishEvent(ctx context.Context, hook plugin.HookPoint, payload map[string]interface{}) {
	plugin.PublishEvent(e.EventBus, ctx, hook, payload)
}

// publishAbortable fires an abortable EventBus event. Returns error if any handler aborts.
func (e *KARAGEngine) publishAbortable(ctx context.Context, hook plugin.HookPoint, payload map[string]interface{}) error {
	return plugin.PublishAbortableEvent(e.EventBus, ctx, hook, payload)
}
