package neo4j

import (
	"context"
	"fmt"
	"log"

	"github.com/hflms/hanfledge/internal/config"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Client wraps the Neo4j driver for graph operations.
type Client struct {
	Driver neo4j.DriverWithContext
}

// NewClient creates a new Neo4j connection.
func NewClient(cfg *config.Neo4jConfig) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(
		cfg.URI,
		neo4j.BasicAuth(cfg.User, cfg.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	// Verify connectivity
	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("Neo4j connectivity check failed: %w", err)
	}

	log.Println("✅ Neo4j connected successfully")
	return &Client{Driver: driver}, nil
}

// Close closes the Neo4j driver.
func (c *Client) Close(ctx context.Context) error {
	return c.Driver.Close(ctx)
}

// InitSchema creates constraints and indexes for the knowledge graph.
func (c *Client) InitSchema(ctx context.Context) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	constraints := []string{
		"CREATE CONSTRAINT IF NOT EXISTS FOR (c:Course) REQUIRE c.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (ch:Chapter) REQUIRE ch.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (kp:KnowledgePoint) REQUIRE kp.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (m:Misconception) REQUIRE m.id IS UNIQUE",
	}

	for _, cypher := range constraints {
		if _, err := session.Run(ctx, cypher, nil); err != nil {
			return fmt.Errorf("neo4j schema init failed: %w", err)
		}
	}

	log.Println("✅ Neo4j schema initialized")
	return nil
}

// CreateCourseGraph creates/updates the full knowledge graph for a course.
func (c *Client) CreateCourseGraph(ctx context.Context, courseID uint, title, subject string) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx,
		"MERGE (c:Course {id: $id}) SET c.title = $title, c.subject = $subject",
		map[string]interface{}{"id": fmt.Sprintf("course_%d", courseID), "title": title, "subject": subject},
	)
	return err
}

// CreateChapterNode creates a chapter node linked to a course.
func (c *Client) CreateChapterNode(ctx context.Context, courseID, chapterID uint, title string, order int) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (c:Course {id: $courseId})
		MERGE (ch:Chapter {id: $chapterId})
		SET ch.title = $title, ch.order = $order
		MERGE (c)-[:HAS_CHAPTER]->(ch)
	`, map[string]interface{}{
		"courseId":  fmt.Sprintf("course_%d", courseID),
		"chapterId": fmt.Sprintf("chapter_%d", chapterID),
		"title":     title,
		"order":     order,
	})
	return err
}

// CreateKnowledgePointNode creates a KP node linked to a chapter.
func (c *Client) CreateKnowledgePointNode(ctx context.Context, chapterID, kpID uint, title string, difficulty float64) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (ch:Chapter {id: $chapterId})
		MERGE (kp:KnowledgePoint {id: $kpId})
		SET kp.title = $title, kp.difficulty = $difficulty
		MERGE (ch)-[:HAS_KP]->(kp)
	`, map[string]interface{}{
		"chapterId":  fmt.Sprintf("chapter_%d", chapterID),
		"kpId":       fmt.Sprintf("kp_%d", kpID),
		"title":      title,
		"difficulty": difficulty,
	})
	return err
}

// CreateRequiresRelation creates a REQUIRES dependency between two KPs.
func (c *Client) CreateRequiresRelation(ctx context.Context, fromKPID, toKPID uint) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (from:KnowledgePoint {id: $fromId})
		MATCH (to:KnowledgePoint {id: $toId})
		MERGE (from)-[:REQUIRES]->(to)
	`, map[string]interface{}{
		"fromId": fmt.Sprintf("kp_%d", fromKPID),
		"toId":   fmt.Sprintf("kp_%d", toKPID),
	})
	return err
}

// GetPrerequisites retrieves all prerequisite KPs for a given KP (up to 3 hops).
func (c *Client) GetPrerequisites(ctx context.Context, kpID uint) ([]map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH path = (target:KnowledgePoint {id: $kpId})-[:REQUIRES*1..3]->(prereq:KnowledgePoint)
		RETURN prereq.id AS id, prereq.title AS title, length(path) AS depth
		ORDER BY depth
	`, map[string]interface{}{
		"kpId": fmt.Sprintf("kp_%d", kpID),
	})
	if err != nil {
		return nil, err
	}

	var prereqs []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		id, _ := record.Get("id")
		title, _ := record.Get("title")
		depth, _ := record.Get("depth")
		prereqs = append(prereqs, map[string]interface{}{
			"id": id, "title": title, "depth": depth,
		})
	}
	return prereqs, nil
}

// ── Graph Search for RRF Retrieval ──────────────────────────

// GraphSearchResult 图谱搜索结果。
type GraphSearchResult struct {
	KPID      string  `json:"kp_id"`
	KPTitle   string  `json:"kp_title"`
	ChunkID   uint    `json:"chunk_id,omitempty"`
	Content   string  `json:"content,omitempty"`
	Relation  string  `json:"relation"`  // "target" | "prerequisite" | "sibling"
	Depth     int     `json:"depth"`     // graph hop distance
	Relevance float64 `json:"relevance"` // graph-based relevance score [0,1]
}

// SearchRelatedKPs finds knowledge points related to the given KP IDs
// by traversing REQUIRES and sibling (same chapter) relationships.
// Used as the graph-based retrieval component in RRF hybrid search.
func (c *Client) SearchRelatedKPs(ctx context.Context, kpIDs []uint, limit int) ([]GraphSearchResult, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	// Build Neo4j ID list
	neo4jIDs := make([]string, len(kpIDs))
	for i, id := range kpIDs {
		neo4jIDs[i] = fmt.Sprintf("kp_%d", id)
	}

	// Query: find related KPs via REQUIRES (both directions) and HAS_KP siblings
	result, err := session.Run(ctx, `
		UNWIND $kpIds AS targetId
		MATCH (target:KnowledgePoint {id: targetId})
		
		// Direct prerequisites and dependents (1-2 hops)
		OPTIONAL MATCH path1 = (target)-[:REQUIRES*1..2]-(related1:KnowledgePoint)
		
		// Sibling KPs in the same chapter
		OPTIONAL MATCH (target)<-[:HAS_KP]-(ch:Chapter)-[:HAS_KP]->(related2:KnowledgePoint)
		WHERE related2.id <> target.id
		
		WITH targetId, 
		     collect(DISTINCT {
		       id: related1.id, 
		       title: related1.title, 
		       difficulty: related1.difficulty,
		       relation: 'prerequisite', 
		       depth: length(path1)
		     }) AS prereqs,
		     collect(DISTINCT {
		       id: related2.id, 
		       title: related2.title, 
		       difficulty: related2.difficulty,
		       relation: 'sibling', 
		       depth: 1
		     }) AS siblings
		
		UNWIND (prereqs + siblings) AS node
		WITH DISTINCT node
		WHERE node.id IS NOT NULL
		RETURN node.id AS id, node.title AS title, node.difficulty AS difficulty,
		       node.relation AS relation, node.depth AS depth
		ORDER BY node.depth ASC
		LIMIT $limit
	`, map[string]interface{}{
		"kpIds": neo4jIDs,
		"limit": limit,
	})
	if err != nil {
		return nil, fmt.Errorf("graph search failed: %w", err)
	}

	var results []GraphSearchResult
	for result.Next(ctx) {
		record := result.Record()
		id, _ := record.Get("id")
		title, _ := record.Get("title")
		difficulty, _ := record.Get("difficulty")
		relation, _ := record.Get("relation")
		depth, _ := record.Get("depth")

		// Calculate relevance: closer nodes are more relevant
		depthInt := 1
		if d, ok := depth.(int64); ok {
			depthInt = int(d)
		}
		relevance := 1.0 / float64(1+depthInt) // 1/(1+depth): depth 0→1.0, 1→0.5, 2→0.33

		diffVal := 0.5
		if d, ok := difficulty.(float64); ok {
			diffVal = d
		}
		_ = diffVal // available for future scoring

		idStr, _ := id.(string)
		titleStr, _ := title.(string)
		relationStr, _ := relation.(string)

		results = append(results, GraphSearchResult{
			KPID:      idStr,
			KPTitle:   titleStr,
			Relation:  relationStr,
			Depth:     depthInt,
			Relevance: relevance,
		})
	}

	return results, nil
}
