package neo4j

import (
	"context"
	"fmt"
	"time"

	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

var slogNeo4j = logger.L("Neo4j")

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

	// Verify connectivity with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("Neo4j connectivity check failed: %w", err)
	}

	slogNeo4j.Info("neo4j connected successfully")
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

	slogNeo4j.Info("neo4j schema initialized")
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

// GetKPContext retrieves N-hop neighborhood context around a knowledge point.
// Used for parallel preloading in Designer agent.
func (c *Client) GetKPContext(ctx context.Context, kpID uint, maxDepth int) ([]map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH path = (center:KnowledgePoint {id: $kpId})-[*0..`+fmt.Sprintf("%d", maxDepth)+`]-(neighbor:KnowledgePoint)
		RETURN DISTINCT neighbor.id AS id, neighbor.title AS title, neighbor.difficulty AS difficulty
		LIMIT 50
	`, map[string]interface{}{
		"kpId": fmt.Sprintf("kp_%d", kpID),
	})
	if err != nil {
		return nil, err
	}

	var nodes []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		id, _ := record.Get("id")
		title, _ := record.Get("title")
		difficulty, _ := record.Get("difficulty")
		
		nodes = append(nodes, map[string]interface{}{
			"id":         id,
			"title":      title,
			"difficulty": difficulty,
		})
	}
	return nodes, nil
}

// ── Misconception CRUD ──────────────────────────────────────

// CreateMisconceptionNode creates a Misconception node and links it to a KP via HAS_TRAP.
func (c *Client) CreateMisconceptionNode(ctx context.Context, kpID, misconceptionID uint, description, trapType string) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (kp:KnowledgePoint {id: $kpId})
		MERGE (m:Misconception {id: $mId})
		SET m.description = $description, m.trap_type = $trapType
		MERGE (kp)-[:HAS_TRAP]->(m)
	`, map[string]interface{}{
		"kpId":        fmt.Sprintf("kp_%d", kpID),
		"mId":         fmt.Sprintf("misconception_%d", misconceptionID),
		"description": description,
		"trapType":    trapType,
	})
	return err
}

// GetMisconceptions retrieves all misconceptions linked to a KP via HAS_TRAP.
func (c *Client) GetMisconceptions(ctx context.Context, kpID uint) ([]MisconceptionResult, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH (kp:KnowledgePoint {id: $kpId})-[:HAS_TRAP]->(m:Misconception)
		RETURN m.id AS id, m.description AS description, m.trap_type AS trap_type
	`, map[string]interface{}{
		"kpId": fmt.Sprintf("kp_%d", kpID),
	})
	if err != nil {
		return nil, err
	}

	var misconceptions []MisconceptionResult
	for result.Next(ctx) {
		record := result.Record()
		id, _ := record.Get("id")
		desc, _ := record.Get("description")
		trapType, _ := record.Get("trap_type")

		idStr, _ := id.(string)
		descStr, _ := desc.(string)
		trapTypeStr, _ := trapType.(string)

		misconceptions = append(misconceptions, MisconceptionResult{
			ID:          idStr,
			Description: descStr,
			TrapType:    trapTypeStr,
		})
	}
	return misconceptions, nil
}

// DeleteMisconceptionNode removes a Misconception node and its HAS_TRAP relation.
func (c *Client) DeleteMisconceptionNode(ctx context.Context, misconceptionID uint) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (m:Misconception {id: $mId})
		DETACH DELETE m
	`, map[string]interface{}{
		"mId": fmt.Sprintf("misconception_%d", misconceptionID),
	})
	return err
}

// MisconceptionResult 误区查询结果。
type MisconceptionResult struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	TrapType    string `json:"trap_type"`
}

// ── Cross-Disciplinary Links ────────────────────────────────

// CreateCrossLink creates a RELATES_TO relation between two KPs from different disciplines.
func (c *Client) CreateCrossLink(ctx context.Context, fromKPID, toKPID uint, linkType string, weight float64) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (from:KnowledgePoint {id: $fromId})
		MATCH (to:KnowledgePoint {id: $toId})
		MERGE (from)-[r:RELATES_TO]->(to)
		SET r.link_type = $linkType, r.weight = $weight
	`, map[string]interface{}{
		"fromId":   fmt.Sprintf("kp_%d", fromKPID),
		"toId":     fmt.Sprintf("kp_%d", toKPID),
		"linkType": linkType,
		"weight":   weight,
	})
	return err
}

// GetCrossLinks retrieves all cross-disciplinary links for a KP (bidirectional).
func (c *Client) GetCrossLinks(ctx context.Context, kpID uint) ([]CrossLinkResult, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH (source:KnowledgePoint {id: $kpId})-[r:RELATES_TO]-(linked:KnowledgePoint)
		OPTIONAL MATCH (linked)<-[:HAS_KP]-(ch:Chapter)<-[:HAS_CHAPTER]-(c:Course)
		RETURN linked.id AS id, linked.title AS title, linked.difficulty AS difficulty,
		       r.link_type AS link_type, r.weight AS weight,
		       c.subject AS subject
	`, map[string]interface{}{
		"kpId": fmt.Sprintf("kp_%d", kpID),
	})
	if err != nil {
		return nil, err
	}

	var links []CrossLinkResult
	for result.Next(ctx) {
		record := result.Record()
		id, _ := record.Get("id")
		title, _ := record.Get("title")
		difficulty, _ := record.Get("difficulty")
		linkType, _ := record.Get("link_type")
		weight, _ := record.Get("weight")
		subject, _ := record.Get("subject")

		idStr, _ := id.(string)
		titleStr, _ := title.(string)
		linkTypeStr, _ := linkType.(string)
		subjectStr, _ := subject.(string)

		diffVal := 0.5
		if d, ok := difficulty.(float64); ok {
			diffVal = d
		}
		weightVal := 1.0
		if w, ok := weight.(float64); ok {
			weightVal = w
		}

		links = append(links, CrossLinkResult{
			KPID:       idStr,
			KPTitle:    titleStr,
			Difficulty: diffVal,
			LinkType:   linkTypeStr,
			Weight:     weightVal,
			Subject:    subjectStr,
		})
	}
	return links, nil
}

// DeleteCrossLink removes a RELATES_TO relation between two KPs.
func (c *Client) DeleteCrossLink(ctx context.Context, fromKPID, toKPID uint) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (from:KnowledgePoint {id: $fromId})-[r:RELATES_TO]-(to:KnowledgePoint {id: $toId})
		DELETE r
	`, map[string]interface{}{
		"fromId": fmt.Sprintf("kp_%d", fromKPID),
		"toId":   fmt.Sprintf("kp_%d", toKPID),
	})
	return err
}

// CrossLinkResult 跨学科联结查询结果。
type CrossLinkResult struct {
	KPID       string  `json:"kp_id"`
	KPTitle    string  `json:"kp_title"`
	Difficulty float64 `json:"difficulty"`
	LinkType   string  `json:"link_type"` // "analogy" | "shared_model" | "application"
	Weight     float64 `json:"weight"`    // link strength [0,1]
	Subject    string  `json:"subject"`   // course subject of the linked KP
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

	// Query: find related KPs via REQUIRES, HAS_KP siblings, and RELATES_TO cross-links
	result, err := session.Run(ctx, `
		UNWIND $kpIds AS targetId
		MATCH (target:KnowledgePoint {id: targetId})
		
		// Direct prerequisites and dependents (1-2 hops)
		OPTIONAL MATCH path1 = (target)-[:REQUIRES*1..2]-(related1:KnowledgePoint)
		
		// Sibling KPs in the same chapter
		OPTIONAL MATCH (target)<-[:HAS_KP]-(ch:Chapter)-[:HAS_KP]->(related2:KnowledgePoint)
		WHERE related2.id <> target.id
		
		// Cross-disciplinary links
		OPTIONAL MATCH (target)-[:RELATES_TO]-(related3:KnowledgePoint)
		
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
		     }) AS siblings,
		     collect(DISTINCT {
		       id: related3.id, 
		       title: related3.title, 
		       difficulty: related3.difficulty,
		       relation: 'cross_disciplinary', 
		       depth: 1
		     }) AS crossLinks
		
		UNWIND (prereqs + siblings + crossLinks) AS node
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

// ── Course Graph for Knowledge Map ──────────────────────────

// GraphEdge represents a single directed edge in the knowledge graph.
type GraphEdge struct {
	FromID string `json:"from_id"` // e.g. "kp_1"
	ToID   string `json:"to_id"`   // e.g. "kp_2"
	Type   string `json:"type"`    // "REQUIRES" | "RELATES_TO" | "HAS_TRAP"
}

// GetCourseGraphEdges returns all edges among KPs belonging to a course.
// Traverses: Course → Chapter → KP, then collects REQUIRES and RELATES_TO edges.
func (c *Client) GetCourseGraphEdges(ctx context.Context, courseID uint) ([]GraphEdge, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH (course:Course {id: $courseId})-[:HAS_CHAPTER]->(ch:Chapter)-[:HAS_KP]->(kp:KnowledgePoint)
		WITH collect(kp) AS allKPs
		UNWIND allKPs AS kp
		OPTIONAL MATCH (kp)-[:REQUIRES]->(prereq:KnowledgePoint)
		WHERE prereq IN allKPs
		WITH allKPs, collect({from: kp.id, to: prereq.id, type: 'REQUIRES'}) AS reqEdges
		UNWIND allKPs AS kp2
		OPTIONAL MATCH (kp2)-[:RELATES_TO]-(linked:KnowledgePoint)
		WHERE linked IN allKPs AND kp2.id < linked.id
		WITH reqEdges, collect({from: kp2.id, to: linked.id, type: 'RELATES_TO'}) AS relEdges
		UNWIND (reqEdges + relEdges) AS edge
		WITH edge WHERE edge.from IS NOT NULL AND edge.to IS NOT NULL
		RETURN DISTINCT edge.from AS from_id, edge.to AS to_id, edge.type AS type
	`, map[string]interface{}{
		"courseId": fmt.Sprintf("course_%d", courseID),
	})
	if err != nil {
		return nil, fmt.Errorf("get course graph edges failed: %w", err)
	}

	var edges []GraphEdge
	for result.Next(ctx) {
		record := result.Record()
		fromID, _ := record.Get("from_id")
		toID, _ := record.Get("to_id")
		edgeType, _ := record.Get("type")

		fromStr, _ := fromID.(string)
		toStr, _ := toID.(string)
		typeStr, _ := edgeType.(string)

		edges = append(edges, GraphEdge{
			FromID: fromStr,
			ToID:   toStr,
			Type:   typeStr,
		})
	}
	return edges, nil
}
