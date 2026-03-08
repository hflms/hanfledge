package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"gorm.io/gorm"
)

// SoulEvolutionService handles automatic soul.md evolution based on learning data.
type SoulEvolutionService struct {
	db   *gorm.DB
	llm  llm.LLMProvider
	path string
}

func NewSoulEvolutionService(db *gorm.DB, llm llm.LLMProvider, path string) *SoulEvolutionService {
	return &SoulEvolutionService{db: db, llm: llm, path: path}
}

// AnalyzeAndSuggest analyzes recent teaching data and suggests soul.md updates.
func (s *SoulEvolutionService) AnalyzeAndSuggest(ctx context.Context) (string, error) {
	// Collect insights from last 7 days
	since := time.Now().AddDate(0, 0, -7)

	// 1. Average mastery improvement
	var avgMastery float64
	s.db.Model(&model.StudentKPMastery{}).
		Where("updated_at > ?", since).
		Select("AVG(mastery)").
		Scan(&avgMastery)

	// 2. Teacher intervention rate
	var interventionCount int64
	s.db.Model(&model.Interaction{}).
		Where("created_at > ? AND role = 'teacher'", since).
		Count(&interventionCount)

	var totalInteractions int64
	s.db.Model(&model.Interaction{}).
		Where("created_at > ?", since).
		Count(&totalInteractions)

	interventionRate := float64(interventionCount) / float64(totalInteractions) * 100

	// 3. Most used skills
	type skillStat struct {
		SkillID string
		Count   int64
	}
	var skills []skillStat
	s.db.Model(&model.Interaction{}).
		Select("skill_id, COUNT(*) as count").
		Where("created_at > ? AND skill_id != ''", since).
		Group("skill_id").
		Order("count DESC").
		Limit(3).
		Scan(&skills)

	// Build insights
	insights := fmt.Sprintf(`## 教学数据洞察（最近 7 天）

- 平均掌握度：%.2f%%
- 教师干预率：%.2f%%
- 最常用技能：`, avgMastery*100, interventionRate)

	for _, s := range skills {
		insights += fmt.Sprintf("\n  - %s (%d 次)", s.SkillID, s.Count)
	}

	// Read current soul
	content, err := os.ReadFile(s.path)
	if err != nil {
		return "", err
	}

	// Generate suggestion using LLM
	prompt := fmt.Sprintf(`你是 Hanfledge 教学系统的 AI 规则优化专家。

当前 Soul 规则：
%s

%s

请分析数据并提出 3-5 条具体的规则优化建议。格式：
1. 建议内容
2. 预期效果
3. 风险评估

保持简洁，每条建议不超过 100 字。`, string(content), insights)

	messages := []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}

	suggestion, err := s.llm.Chat(ctx, messages, nil)

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s\n\n## AI 优化建议\n\n%s", insights, suggestion), nil
}

// StartAutoEvolution starts a background goroutine that analyzes data weekly.
func (s *SoulEvolutionService) StartAutoEvolution(ctx context.Context) {
	ticker := time.NewTicker(7 * 24 * time.Hour) // Weekly
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			suggestion, err := s.AnalyzeAndSuggest(ctx)
			if err != nil {
				log.Printf("[SoulEvolution] Analysis failed: %v", err)
				continue
			}

			// Store suggestion for admin review
			log.Printf("[SoulEvolution] New suggestion generated:\n%s", suggestion)
			// TODO: Send notification to admin
		}
	}
}
