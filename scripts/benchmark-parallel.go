//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"github.com/hflms/hanfledge/internal/repository/postgres"
	"github.com/joho/godotenv"
)

// 性能基准测试: 对比并行化前后的 Strategist + Designer 耗时

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  未找到 .env 文件,使用默认配置")
	}

	cfg := config.Load()

	// 初始化数据库
	db, err := postgres.NewConnection(&cfg.Database)
	if err != nil {
		log.Fatalf("❌ 数据库连接失败: %v", err)
	}

	// 初始化 Neo4j
	neo4jClient, err := neo4jRepo.NewClient(&cfg.Neo4j)
	if err != nil {
		log.Fatalf("❌ Neo4j 连接失败: %v", err)
	}
	defer neo4jClient.Close(context.Background())

	// 初始化 LLM (用于 Designer)
	var llmProvider llm.LLMProvider
	switch cfg.LLM.Provider {
	case "dashscope":
		embModel := cfg.LLM.EmbeddingModel
		if embModel == "" {
			embModel = "text-embedding-v3"
		}
		llmProvider = llm.NewDashScopeClient(llm.DashScopeConfig{
			APIKey:         cfg.LLM.DashScopeKey,
			ChatModel:      cfg.LLM.DashScopeModel,
			EmbeddingModel: embModel,
			CompatBaseURL:  cfg.LLM.DashScopeCompatURL,
		})
	default:
		llmProvider = llm.NewOllamaClient(
			cfg.LLM.OllamaHost,
			cfg.LLM.OllamaModel,
			cfg.LLM.EmbeddingModel,
		)
	}
	_ = llmProvider // reserved for future Designer benchmark

	fmt.Println("🚀 Hanfledge 2.0 性能基准测试")
	fmt.Println("================================")
	fmt.Println()

	// 查找一个测试会话
	var sessionID, studentID, activityID uint
	err = db.Raw(`
		SELECT id, student_id, activity_id 
		FROM student_sessions 
		WHERE status = 'active' 
		LIMIT 1
	`).Scan(&map[string]interface{}{
		"id":          &sessionID,
		"student_id":  &studentID,
		"activity_id": &activityID,
	}).Error

	if err != nil || sessionID == 0 {
		fmt.Println("⚠️  未找到活跃会话,请先创建测试数据:")
		fmt.Println("   go run scripts/seed.go")
		os.Exit(1)
	}

	fmt.Printf("📊 测试会话: ID=%d, Student=%d, Activity=%d\n", sessionID, studentID, activityID)
	fmt.Println()

	// 创建 Strategist
	strategist := agent.NewStrategistAgent(db, neo4jClient, nil)

	// 测试 1: Strategist 单独耗时
	fmt.Println("⏱️  [测试 1] Strategist 单独耗时")
	start := time.Now()
	_, err = strategist.Analyze(context.Background(), sessionID, studentID, activityID)
	strategistTime := time.Since(start)
	if err != nil {
		log.Printf("❌ Strategist 失败: %v", err)
	} else {
		fmt.Printf("   ✓ Strategist: %v\n", strategistTime)
	}
	fmt.Println()

	// 测试 2: Designer 预加载耗时
	fmt.Println("⏱️  [测试 2] Designer 预加载 (Neo4j GetKPContext)")
	var currentKP uint
	var designerTime time.Duration
	db.Raw("SELECT current_kp FROM student_sessions WHERE id = ?", sessionID).Scan(&currentKP)

	if currentKP > 0 {
		start = time.Now()
		_, err = neo4jClient.GetKPContext(context.Background(), currentKP, 2)
		designerTime = time.Since(start)
		if err != nil {
			log.Printf("❌ Designer 预加载失败: %v", err)
		} else {
			fmt.Printf("   ✓ Designer 预加载: %v\n", designerTime)
		}
	} else {
		fmt.Println("   ⚠️  当前会话无 CurrentKP,跳过测试")
	}
	fmt.Println()

	// 测试 3: 并行执行模拟
	fmt.Println("⏱️  [测试 3] 并行执行模拟")
	type strategistResult struct {
		err error
		dur time.Duration
	}
	type designerResult struct {
		err error
		dur time.Duration
	}

	strategistCh := make(chan strategistResult, 1)
	designerCh := make(chan designerResult, 1)

	parallelStart := time.Now()

	// 并行分支 A
	go func() {
		start := time.Now()
		_, err := strategist.Analyze(context.Background(), sessionID, studentID, activityID)
		strategistCh <- strategistResult{err, time.Since(start)}
	}()

	// 并行分支 B
	go func() {
		start := time.Now()
		if currentKP > 0 {
			_, err = neo4jClient.GetKPContext(context.Background(), currentKP, 2)
			designerCh <- designerResult{err, time.Since(start)}
		} else {
			designerCh <- designerResult{nil, 0}
		}
	}()

	// 等待汇聚
	stratRes := <-strategistCh
	designRes := <-designerCh
	parallelTotal := time.Since(parallelStart)

	fmt.Printf("   ✓ Strategist (并行): %v\n", stratRes.dur)
	if currentKP > 0 {
		fmt.Printf("   ✓ Designer (并行): %v\n", designRes.dur)
	}
	fmt.Printf("   ✓ 总耗时 (并行): %v\n", parallelTotal)
	fmt.Println()

	// 计算性能提升
	serialTotal := strategistTime
	if currentKP > 0 {
		serialTotal += designerTime
	}

	improvement := float64(serialTotal-parallelTotal) / float64(serialTotal) * 100

	fmt.Println("================================")
	fmt.Println("📈 性能对比")
	fmt.Println("================================")
	fmt.Printf("串行总耗时: %v\n", serialTotal)
	fmt.Printf("并行总耗时: %v\n", parallelTotal)
	fmt.Printf("性能提升:   %.1f%%\n", improvement)
	fmt.Println()

	if improvement > 30 {
		fmt.Println("✅ 并行化优化效果显著!")
	} else if improvement > 10 {
		fmt.Println("⚠️  并行化有一定效果,但提升有限")
	} else {
		fmt.Println("❌ 并行化效果不明显,可能需要进一步优化")
	}
}
