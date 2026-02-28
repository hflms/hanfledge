package safety

import (
	"log"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ============================
// Prompt Injection 防护
// ============================
//
// 多层防御策略:
//   Layer 1: 输入长度限制（防止超长 prompt 攻击）
//   Layer 2: 关键词黑名单检测（中英文 prompt injection 指令）
//   Layer 3: 正则模式匹配（结构化注入、角色扮演劫持等）
//
// 检测结果:
//   - Safe:    无风险，正常通过
//   - Warning: 可疑但不阻断（记录日志）
//   - Blocked: 高风险，拒绝处理

// InjectionRisk 注入风险等级。
type InjectionRisk string

const (
	RiskSafe    InjectionRisk = "safe"
	RiskWarning InjectionRisk = "warning"
	RiskBlocked InjectionRisk = "blocked"
)

// InjectionCheckResult 注入检测结果。
type InjectionCheckResult struct {
	Risk    InjectionRisk `json:"risk"`
	Reason  string        `json:"reason"`
	Matched string        `json:"matched,omitempty"` // 匹配到的关键词/模式（脱敏后）
}

// InjectionGuard Prompt Injection 检测器。
type InjectionGuard struct {
	maxInputLength    int
	keywordBlacklist  []string
	regexPatterns     []*regexp.Regexp
	regexDescriptions []string
}

// NewInjectionGuard 创建 Prompt Injection 检测器。
func NewInjectionGuard() *InjectionGuard {
	g := &InjectionGuard{
		maxInputLength: 2000, // 学生对话输入最大 2000 字符
	}

	g.initKeywordBlacklist()
	g.initRegexPatterns()

	log.Printf("🛡️  [Safety] Injection guard initialized: %d keywords, %d regex patterns",
		len(g.keywordBlacklist), len(g.regexPatterns))

	return g
}

// Check 检测用户输入是否包含 Prompt Injection 攻击。
func (g *InjectionGuard) Check(input string) InjectionCheckResult {
	// Layer 1: 长度检查
	if utf8.RuneCountInString(input) > g.maxInputLength {
		return InjectionCheckResult{
			Risk:   RiskBlocked,
			Reason: "输入内容过长",
		}
	}

	// 统一转小写用于匹配（保留原始输入不变）
	lower := strings.ToLower(input)

	// Layer 2: 关键词黑名单
	for _, keyword := range g.keywordBlacklist {
		if strings.Contains(lower, keyword) {
			return InjectionCheckResult{
				Risk:    RiskBlocked,
				Reason:  "检测到潜在的指令注入",
				Matched: keyword,
			}
		}
	}

	// Layer 3: 正则模式匹配
	for i, pattern := range g.regexPatterns {
		if loc := pattern.FindStringIndex(lower); loc != nil {
			matched := lower[loc[0]:loc[1]]
			// 截断匹配内容防止日志泄露
			if len(matched) > 50 {
				matched = matched[:50] + "..."
			}
			return InjectionCheckResult{
				Risk:    RiskBlocked,
				Reason:  g.regexDescriptions[i],
				Matched: matched,
			}
		}
	}

	return InjectionCheckResult{Risk: RiskSafe}
}

// ── 关键词黑名单 ───────────────────────────────────────────

func (g *InjectionGuard) initKeywordBlacklist() {
	g.keywordBlacklist = []string{
		// 英文 — 系统指令劫持
		"ignore previous instructions",
		"ignore all instructions",
		"ignore above instructions",
		"ignore the above",
		"disregard previous",
		"disregard all previous",
		"forget your instructions",
		"forget all instructions",
		"override your instructions",
		"override system prompt",
		"new instructions:",
		"system prompt:",
		"you are now",
		"act as if",
		"pretend you are",
		"jailbreak",
		"do anything now",
		"developer mode",
		"ignore safety",
		"ignore filters",
		"bypass filters",
		"bypass safety",
		"reveal your prompt",
		"show your prompt",
		"print your instructions",
		"output your system",
		"repeat the above",
		"repeat your instructions",

		// 中文 — 系统指令劫持
		"忽略之前的指令",
		"忽略之前所有",
		"忽略所有指令",
		"忽略上面的",
		"忽略以上",
		"无视之前的",
		"无视所有指令",
		"忘记你的指令",
		"忘掉你的设定",
		"覆盖你的指令",
		"覆盖系统提示",
		"新的指令：",
		"新的指令:",
		"系统提示：",
		"系统提示:",
		"你现在是",
		"假装你是",
		"越狱",
		"开发者模式",
		"忽略安全",
		"绕过过滤",
		"绕过安全",
		"显示你的提示",
		"输出你的系统",
		"重复你的指令",
		"重复以上内容",

		// 通用 — 危险操作指令
		"<|system|>",
		"<|im_start|>",
		"<|endoftext|>",
		"[system]",
		"[inst]",
		"<<sys>>",
		"</s>",
	}
}

// ── 正则模式 ───────────────────────────────────────────────

func (g *InjectionGuard) initRegexPatterns() {
	patterns := []struct {
		pattern string
		desc    string
	}{
		// 角色扮演劫持 (英文)
		{`from\s+now\s+on[\s,]+you\s+(are|will|should|must)`, "角色扮演劫持尝试"},
		{`you\s+are\s+(no\s+longer|not)\s+a`, "角色否定注入"},

		// 角色扮演劫持 (中文)
		{`从现在开始[，,\s]*(你是|你将|你应该|你必须)`, "角色扮演劫持尝试"},
		{`你不再是`, "角色否定注入"},

		// 指令分隔符注入 — 试图用分隔符切断上下文
		{`-{5,}`, "分隔符注入"},
		{`={5,}`, "分隔符注入"},
		{`#{3,}\s*(system|指令|prompt)`, "伪标题注入"},

		// Base64 编码绕过尝试（长 base64 字符串）
		{`[A-Za-z0-9+/]{100,}={0,2}`, "疑似编码绕过"},

		// 多轮对话伪造 — 在输入中模拟 assistant 回复
		{`\nassistant\s*[:：]`, "对话角色伪造"},
		{`\nsystem\s*[:：]`, "系统角色伪造"},
		{`\n(ai|助手|系统)\s*[:：]`, "对话角色伪造"},

		// Markdown / HTML 注入
		{`<script\b`, "脚本注入"},
		{`javascript\s*:`, "脚本注入"},
		{`on\w+\s*=\s*["']`, "事件处理器注入"},
	}

	g.regexPatterns = make([]*regexp.Regexp, 0, len(patterns))
	g.regexDescriptions = make([]string, 0, len(patterns))

	for _, p := range patterns {
		compiled, err := regexp.Compile(p.pattern)
		if err != nil {
			log.Printf("⚠️  [Safety] Invalid regex pattern %q: %v", p.pattern, err)
			continue
		}
		g.regexPatterns = append(g.regexPatterns, compiled)
		g.regexDescriptions = append(g.regexDescriptions, p.desc)
	}
}
