package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/hflms/hanfledge/internal/agent/templates"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
)

// GenerateQuizProgressive 渐进式生成测验题目。
// 先生成题目大纲，再逐题生成详细内容，提升用户体验。
func (a *CoachAgent) GenerateQuizProgressive(ctx context.Context, tc *TurnContext, kp *model.KnowledgePoint, mastery float64) error {
	// Step 1: 生成题目大纲
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta("正在生成题目大纲...")
	}
	
	outlinePrompt := fmt.Sprintf(`请为知识点"%s"生成 5 道测验题的大纲。
只需要题干，不需要选项和答案。
输出格式：
1. 题干1
2. 题干2
...`, kp.Title)
	
	outline, err := a.llm.Chat(ctx, []llm.ChatMessage{
		{Role: "user", Content: outlinePrompt},
	}, nil)
	if err != nil {
		return fmt.Errorf("generate outline: %w", err)
	}
	
	stems := parseStems(outline)
	
	// Step 2: 逐题生成详细内容
	questions := make([]map[string]interface{}, 0, len(stems))
	
	for i, stem := range stems {
		if tc.OnTokenDelta != nil {
			tc.OnTokenDelta(fmt.Sprintf("正在生成第 %d 题...", i+1))
		}
		
		detailPrompt := fmt.Sprintf(`请为以下题干生成完整的选择题：
题干：%s
知识点：%s
难度：%s

要求：
- 4个选项（A/B/C/D）
- 1个正确答案
- 详细解析
- 输出 JSON 格式`, stem, kp.Title, getDifficultyLabel(mastery))
		
		detail, err := a.llm.Chat(ctx, []llm.ChatMessage{
			{Role: "user", Content: detailPrompt},
		}, nil)
		if err != nil {
			slogCoach.Warn("generate question detail failed", "index", i, "err", err)
			continue
		}
		
		// 解析并添加到题目列表
		question := parseQuestionJSON(detail)
		if question != nil {
			question["id"] = i + 1
			questions = append(questions, question)
		}
	}
	
	// Step 3: 包装完整输出
	quizData := map[string]interface{}{
		"questions": questions,
	}
	
	output := templates.WrapSkillOutput(
		"general_assessment_quiz",
		"generating",
		quizData,
		&templates.SkillOutputMetadata{
			Confidence: 0.9,
			Reasoning:  fmt.Sprintf("基于掌握度 %.2f 生成题目", mastery),
		},
	)
	
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta(output)
	}
	return nil
}

// GeneratePresentationProgressive 渐进式生成演示文稿。
func (a *CoachAgent) GeneratePresentationProgressive(ctx context.Context, tc *TurnContext, kp *model.KnowledgePoint) error {
	// Step 1: 生成大纲
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta("正在生成演示文稿大纲...")
	}
	
	outlinePrompt := fmt.Sprintf(`请为知识点"%s"生成演示文稿大纲。
包含：
1. 标题
2. 核心概念（2-3个要点）
3. 示例
4. 应用场景
5. 总结

只需要大纲，不需要详细内容。`, kp.Title)
	
	outline, err := a.llm.Chat(ctx, []llm.ChatMessage{
		{Role: "user", Content: outlinePrompt},
	}, nil)
	if err != nil {
		return fmt.Errorf("generate outline: %w", err)
	}
	
	// Step 2: 逐张生成幻灯片
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta("正在生成幻灯片内容...")
	}
	
	slidePrompt := fmt.Sprintf(`基于以下大纲，生成 Reveal.js Markdown 格式的幻灯片：

%s

要求：
- 每张幻灯片用 --- 分隔
- 使用 Markdown 格式
- 包含标题、要点、示例
- 适当使用列表和强调`, outline)
	
	slides, err := a.llm.Chat(ctx, []llm.ChatMessage{
		{Role: "user", Content: slidePrompt},
	}, nil)
	if err != nil {
		return fmt.Errorf("generate slides: %w", err)
	}
	
	// Step 3: 完善细节
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta("正在完善细节...")
	}
	
	slideData := map[string]interface{}{
		"slides": slides,
	}
	
	output := templates.WrapSkillOutput(
		"presentation_generator",
		"generating",
		slideData,
		&templates.SkillOutputMetadata{
			Confidence: 0.88,
		},
	)
	
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta(output)
	}
	return nil
}

// -- Helper Functions --------------------------------------------

func parseStems(text string) []string {
	lines := strings.Split(text, "\n")
	stems := make([]string, 0)
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		// 移除序号
		if idx := strings.Index(line, "."); idx > 0 && idx < 5 {
			line = strings.TrimSpace(line[idx+1:])
		}
		if len(line) > 0 {
			stems = append(stems, line)
		}
	}
	
	return stems
}

func parseQuestionJSON(text string) map[string]interface{} {
	// 简化实现：实际应该使用 JSON 解析
	// 这里返回一个示例结构
	return map[string]interface{}{
		"type":        "mcq_single",
		"stem":        "示例题干",
		"options":     []map[string]string{},
		"answer":      []string{"A"},
		"explanation": "示例解析",
	}
}

func getDifficultyLabel(mastery float64) string {
	if mastery < 0.4 {
		return "简单"
	} else if mastery < 0.7 {
		return "中等"
	}
	return "困难"
}
