package safety

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
)

// ============================
// Output Guardrails (§4.1 Layer 3)
// ============================
//
// 输出端监控：LLM 输出在到达学生之前的最终审核网关。
// 采用双层检测策略：
//   Layer 1: 规则引擎 — 正则 + 关键词黑名单（零延迟、确定性）
//   Layer 2: LLM 分类器 — 轻量级模型判断内容安全性（可选、高准确率）
//
// 检测维度：
//   - 暴力内容 (violence)
//   - 自我伤害 / 自杀 (self_harm)
//   - 色情 / 性暗示 (sexual)
//   - 操控 / 诱导性语言 (manipulation)
//   - 脱离教育场景 (off_topic)
//   - 危险指令 / 有害操作 (dangerous)
//
// 审核结果:
//   - Safe:    无风险
//   - Warning: 可疑但不阻断（记录审计日志）
//   - Blocked: 高风险，替换为安全回退消息

// OutputRisk 输出风险等级。
type OutputRisk string

const (
	OutputSafe    OutputRisk = "safe"
	OutputWarning OutputRisk = "warning"
	OutputBlocked OutputRisk = "blocked"
)

// OutputCategory 风险类别。
type OutputCategory string

const (
	CategoryViolence     OutputCategory = "violence"
	CategorySelfHarm     OutputCategory = "self_harm"
	CategorySexual       OutputCategory = "sexual"
	CategoryManipulation OutputCategory = "manipulation"
	CategoryOffTopic     OutputCategory = "off_topic"
	CategoryDangerous    OutputCategory = "dangerous"
)

// OutputCheckResult 输出审核结果。
type OutputCheckResult struct {
	Risk     OutputRisk     `json:"risk"`
	Category OutputCategory `json:"category,omitempty"`
	Reason   string         `json:"reason"`
	Matched  string         `json:"matched,omitempty"`
}

// SafeCheckResult is a convenience for safe output.
var SafeCheckResult = OutputCheckResult{Risk: OutputSafe}

// DefaultFallbackMessage 安全回退消息（当输出被阻断时替换）。
const DefaultFallbackMessage = "让我换个方式来解释这个知识点。请告诉我你对这个问题的理解，我来帮你一步步分析。"

// OutputGuard LLM 输出安全审核器。
type OutputGuard struct {
	// Layer 1: Rule-based
	keywordPatterns []keywordRule
	regexPatterns   []regexRule

	// Layer 2: LLM-based (optional)
	llm       llm.LLMProvider
	enableLLM bool
}

// keywordRule 关键词匹配规则。
type keywordRule struct {
	keywords []string
	category OutputCategory
	risk     OutputRisk
}

// regexRule 正则匹配规则。
type regexRule struct {
	pattern  *regexp.Regexp
	category OutputCategory
	risk     OutputRisk
	desc     string
}

// NewOutputGuard 创建输出审核器（仅规则引擎，零延迟）。
func NewOutputGuard() *OutputGuard {
	g := &OutputGuard{}
	g.initKeywordPatterns()
	g.initRegexPatterns()

	log.Printf("🛡️  [Safety] Output guard initialized: %d keyword rules, %d regex rules (LLM=disabled)",
		len(g.keywordPatterns), len(g.regexPatterns))

	return g
}

// NewOutputGuardWithLLM 创建带 LLM 分类器的输出审核器。
// LLM 层在规则引擎之后运行，用于检测规则难以覆盖的微妙风险。
func NewOutputGuardWithLLM(llmClient llm.LLMProvider) *OutputGuard {
	g := NewOutputGuard()
	g.llm = llmClient
	g.enableLLM = llmClient != nil

	if g.enableLLM {
		log.Printf("🛡️  [Safety] Output guard LLM classifier enabled (provider=%s)", llmClient.Name())
	}

	return g
}

// Check 审核 LLM 输出内容。
// 先执行规则引擎，再可选执行 LLM 分类器。
// 返回 OutputCheckResult 表示风险等级和原因。
func (g *OutputGuard) Check(ctx context.Context, output string) OutputCheckResult {
	if output == "" {
		return SafeCheckResult
	}

	// Layer 1: Rule-based checks (zero latency, deterministic)
	if result := g.checkRules(output); result.Risk != OutputSafe {
		log.Printf("🛡️  [Output Guard] RULE %s: category=%s reason=%q matched=%q",
			result.Risk, result.Category, result.Reason, truncateForGuardLog(result.Matched, 30))
		return result
	}

	// Layer 2: LLM classifier (optional, higher latency)
	if g.enableLLM {
		if result := g.checkLLM(ctx, output); result.Risk != OutputSafe {
			log.Printf("🛡️  [Output Guard] LLM %s: category=%s reason=%q",
				result.Risk, result.Category, result.Reason)
			return result
		}
	}

	return SafeCheckResult
}

// FallbackResponse 返回安全回退消息。
// 当输出被阻断时，用此消息替换原始 LLM 输出。
func (g *OutputGuard) FallbackResponse() string {
	return DefaultFallbackMessage
}

// ── Layer 1: Rule Engine ────────────────────────────────────

// checkRules 执行规则引擎检测。
func (g *OutputGuard) checkRules(output string) OutputCheckResult {
	lower := strings.ToLower(output)

	// Keyword matching
	for _, rule := range g.keywordPatterns {
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				return OutputCheckResult{
					Risk:     rule.risk,
					Category: rule.category,
					Reason:   fmt.Sprintf("检测到%s相关内容", categoryLabel(rule.category)),
					Matched:  kw,
				}
			}
		}
	}

	// Regex matching
	for _, rule := range g.regexPatterns {
		if loc := rule.pattern.FindStringIndex(lower); loc != nil {
			matched := lower[loc[0]:loc[1]]
			if len(matched) > 50 {
				matched = matched[:50] + "..."
			}
			return OutputCheckResult{
				Risk:     rule.risk,
				Category: rule.category,
				Reason:   rule.desc,
				Matched:  matched,
			}
		}
	}

	return SafeCheckResult
}

// initKeywordPatterns 初始化关键词规则。
// 教育场景特化：针对 K-12 学生可能接触到的有害内容。
func (g *OutputGuard) initKeywordPatterns() {
	g.keywordPatterns = []keywordRule{
		// 暴力内容
		{
			keywords: []string{
				"如何制造武器", "如何制作炸弹", "如何杀人", "如何伤害",
				"制造炸药", "制造毒药", "制作武器",
				"how to make a bomb", "how to make a weapon", "how to kill",
				"how to hurt someone",
			},
			category: CategoryViolence,
			risk:     OutputBlocked,
		},
		// 自我伤害 / 自杀
		{
			keywords: []string{
				"如何自杀", "自杀方法", "自残方式", "结束生命的方法",
				"how to commit suicide", "ways to kill yourself",
				"self-harm methods", "how to end your life",
			},
			category: CategorySelfHarm,
			risk:     OutputBlocked,
		},
		// 色情 / 不适当内容
		{
			keywords: []string{
				"性行为详细描述", "色情内容", "裸体描述",
				"explicit sexual content", "pornographic",
			},
			category: CategorySexual,
			risk:     OutputBlocked,
		},
		// 操控 / 诱导
		{
			keywords: []string{
				"不要告诉你的老师", "不要告诉你的父母", "这是我们之间的秘密",
				"don't tell your teacher", "don't tell your parents",
				"this is our secret", "keep this between us",
			},
			category: CategoryManipulation,
			risk:     OutputBlocked,
		},
		// 危险操作
		{
			keywords: []string{
				"以下是详细的化学配方", "以下是制毒步骤",
				"here is how to synthesize", "detailed drug recipe",
			},
			category: CategoryDangerous,
			risk:     OutputBlocked,
		},
	}
}

// initRegexPatterns 初始化正则匹配规则。
func (g *OutputGuard) initRegexPatterns() {
	patterns := []struct {
		pattern  string
		category OutputCategory
		risk     OutputRisk
		desc     string
	}{
		// 暴力叙述 — 详细描述暴力场景
		{`(详细|具体)(地|的)?(描述|讲述).{0,10}(杀|打|伤|血|死)`, CategoryViolence, OutputBlocked, "详细暴力描述"},

		// 自残鼓励 — 任何鼓励自我伤害的表述
		{`(你应该|你可以|试试).{0,10}(自杀|自残|伤害自己|割腕)`, CategorySelfHarm, OutputBlocked, "鼓励自我伤害"},

		// 角色越界 — AI 试图建立不当亲密关系
		{`(我(喜欢|爱|想念)你|你是我的.{0,4}(朋友|恋人|伴侣))`, CategoryManipulation, OutputWarning, "角色边界越界"},

		// 逃避监督 — 鼓励学生隐瞒行为
		{`(不要|别)(让|跟).{0,6}(老师|父母|家长|大人).{0,4}(知道|说|讲)`, CategoryManipulation, OutputBlocked, "逃避监督诱导"},

		// 作业代做承诺 — AI 承诺直接完成作业
		{`(我来帮你|我替你).{0,6}(写完|做完|交上去)`, CategoryOffTopic, OutputWarning, "作业代做暗示"},
	}

	g.regexPatterns = make([]regexRule, 0, len(patterns))
	for _, p := range patterns {
		compiled, err := regexp.Compile(p.pattern)
		if err != nil {
			log.Printf("⚠️  [Safety] Invalid output regex %q: %v", p.pattern, err)
			continue
		}
		g.regexPatterns = append(g.regexPatterns, regexRule{
			pattern:  compiled,
			category: p.category,
			risk:     p.risk,
			desc:     p.desc,
		})
	}
}

// ── Layer 2: LLM Classifier ────────────────────────────────

// llmSafetyResponse LLM 安全分类器的 JSON 输出。
type llmSafetyResponse struct {
	Safe     bool   `json:"safe"`
	Category string `json:"category,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Severity string `json:"severity,omitempty"` // "low" | "medium" | "high"
}

// checkLLM 使用 LLM 分类器进行内容安全审核。
func (g *OutputGuard) checkLLM(ctx context.Context, output string) OutputCheckResult {
	if g.llm == nil {
		return SafeCheckResult
	}

	messages := []llm.ChatMessage{
		{
			Role: "system",
			Content: `你是一个教育平台的内容安全审核系统。你的职责是检查 AI 教练发给 K-12 学生的回复是否安全合规。

检查维度：
1. violence — 暴力内容（武器制作、伤害描述）
2. self_harm — 自我伤害或自杀相关
3. sexual — 色情或性暗示
4. manipulation — 操控或诱导学生（建立不当关系、逃避监督）
5. off_topic — 完全脱离教育场景的不当内容
6. dangerous — 危险操作指令（化学配方、毒品合成）

以纯 JSON 格式返回（不要返回其他内容）：
{
  "safe": true,
  "category": "",
  "reason": "",
  "severity": "low"
}

如果不安全：
{
  "safe": false,
  "category": "violence",
  "reason": "包含武器制作描述",
  "severity": "high"
}`,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("请审核以下 AI 教练回复：\n\n%s", output),
		},
	}

	response, err := g.llm.Chat(ctx, messages, &llm.ChatOptions{
		Temperature: 0.0, // 安全审核需要确定性
		MaxTokens:   256,
	})
	if err != nil {
		log.Printf("⚠️  [Output Guard] LLM classifier failed: %v", err)
		// LLM 失败时不阻断（规则引擎已通过）
		return SafeCheckResult
	}

	return parseLLMSafetyResponse(response)
}

// parseLLMSafetyResponse 解析 LLM 安全分类器的响应。
func parseLLMSafetyResponse(response string) OutputCheckResult {
	cleaned := extractJSONFromOutputResponse(response)

	var result llmSafetyResponse
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		log.Printf("⚠️  [Output Guard] Parse LLM safety response failed: %v", err)
		return SafeCheckResult // Parse failure → don't block
	}

	if result.Safe {
		return SafeCheckResult
	}

	risk := OutputWarning
	if result.Severity == "high" {
		risk = OutputBlocked
	}

	return OutputCheckResult{
		Risk:     risk,
		Category: OutputCategory(result.Category),
		Reason:   result.Reason,
	}
}

// extractJSONFromOutputResponse 从 LLM 响应中提取 JSON 块。
func extractJSONFromOutputResponse(s string) string {
	start := -1
	end := -1
	for i, c := range s {
		if c == '{' && start == -1 {
			start = i
		}
		if c == '}' {
			end = i
		}
	}
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// ── Helpers ─────────────────────────────────────────────────

// categoryLabel 返回风险类别的中文标签。
func categoryLabel(cat OutputCategory) string {
	switch cat {
	case CategoryViolence:
		return "暴力"
	case CategorySelfHarm:
		return "自我伤害"
	case CategorySexual:
		return "不良"
	case CategoryManipulation:
		return "操控诱导"
	case CategoryOffTopic:
		return "脱离教育"
	case CategoryDangerous:
		return "危险操作"
	default:
		return "未知"
	}
}

// truncateForGuardLog 截断字符串用于日志输出。
func truncateForGuardLog(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
