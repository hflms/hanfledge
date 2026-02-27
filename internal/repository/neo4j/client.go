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
