package agent

import (
	"context"
	"fmt"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
)

type AssessorAgent struct {
	llm llm.LLMProvider
}

func NewAssessorAgent(llmClient llm.LLMProvider) *AssessorAgent {
	return &AssessorAgent{llm: llmClient}
}

func (a *AssessorAgent) GenerateTest(ctx context.Context, kpName string) (string, error) {
	prompt := fmt.Sprintf(`你是一个专业的出题老师。请针对知识点 "%s" 出一道测试题。
这道题需要能够客观、准确地考察学生是否真正掌握了该知识点的实际应用能力。
要求：
1. 题目不要太长，直接给出问题。
2. 不要包含答案。
3. 题目应该是一个具体的应用场景或者计算、分析题，不能是简单的死记硬背。`, kpName)

	msg := llm.ChatMessage{Role: "user", Content: prompt}
	respText, err := a.llm.Chat(ctx, []llm.ChatMessage{msg}, nil)
	if err != nil {
		return "", err
	}
	return respText, nil
}

type EvaluatorAgent struct {
	llm llm.LLMProvider
}

func NewEvaluatorAgent(llmClient llm.LLMProvider) *EvaluatorAgent {
	return &EvaluatorAgent{llm: llmClient}
}

func (e *EvaluatorAgent) Grade(ctx context.Context, question, answer string) (bool, string, error) {
	prompt := fmt.Sprintf(`你是一个严格的阅卷老师。
请根据以下题目和学生的回答，判断学生是否回答正确。

题目：%s
学生回答：%s

请在第一行严格输出 "CORRECT" 或 "INCORRECT"。
从第二行开始，简要给出你的评判理由。`, question, answer)

	msg := llm.ChatMessage{Role: "user", Content: prompt}
	respText, err := e.llm.Chat(ctx, []llm.ChatMessage{msg}, nil)
	if err != nil {
		return false, "", err
	}

	isCorrect := false
	if len(respText) >= 7 && respText[:7] == "CORRECT" {
		isCorrect = true
	}

	return isCorrect, respText, nil
}
