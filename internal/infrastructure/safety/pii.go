package safety

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"gorm.io/gorm"
)

var slogPII = logger.L("PIIRedactor")

// ============================
// PII 脱敏处理器
// ============================
//
// 在发送消息给 LLM 之前，自动检测并替换以下 PII:
//   1. 学生姓名 → [学生]
//   2. 教师姓名 → [教师]
//   3. 学校名称 → [学校]
//   4. 手机号   → [手机号]
//   5. 邮箱地址 → [邮箱]
//   6. 身份证号 → [证件号]
//
// 脱敏策略:
//   - 基于数据库中实际存在的用户名/学校名进行精确匹配
//   - 基于正则模式匹配手机号、邮箱、身份证号等通用 PII
//   - 定时刷新 PII 词典（首次加载后缓存）

// PIIRedactor PII 脱敏处理器。
type PIIRedactor struct {
	db *gorm.DB

	// 缓存的 PII 词典
	mu           sync.RWMutex
	studentNames []string // 学生姓名列表
	teacherNames []string // 教师姓名列表
	schoolNames  []string // 学校名称列表
	loaded       bool

	// 通用 PII 正则模式
	phonePattern  *regexp.Regexp
	emailPattern  *regexp.Regexp
	idCardPattern *regexp.Regexp
}

// NewPIIRedactor 创建 PII 脱敏处理器。
func NewPIIRedactor(db *gorm.DB) *PIIRedactor {
	r := &PIIRedactor{
		db:            db,
		phonePattern:  regexp.MustCompile(`1[3-9]\d{9}`),
		emailPattern:  regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		idCardPattern: regexp.MustCompile(`[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`),
	}

	// 首次加载 PII 词典
	r.RefreshDictionary()

	return r
}

// RefreshDictionary 从数据库刷新 PII 词典（姓名、学校名）。
func (r *PIIRedactor) RefreshDictionary() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 加载学生姓名
	var studentNames []string
	r.db.Raw(`
		SELECT DISTINCT u.display_name FROM users u
		JOIN user_school_roles usr ON usr.user_id = u.id
		JOIN roles r ON r.id = usr.role_id
		WHERE r.name = 'STUDENT' AND u.deleted_at IS NULL
	`).Scan(&studentNames)

	// 加载教师姓名
	var teacherNames []string
	r.db.Raw(`
		SELECT DISTINCT u.display_name FROM users u
		JOIN user_school_roles usr ON usr.user_id = u.id
		JOIN roles r ON r.id = usr.role_id
		WHERE r.name IN ('TEACHER', 'SCHOOL_ADMIN') AND u.deleted_at IS NULL
	`).Scan(&teacherNames)

	// 加载学校名称
	var schoolNames []string
	r.db.Raw(`
		SELECT name FROM schools WHERE deleted_at IS NULL
	`).Scan(&schoolNames)

	r.studentNames = studentNames
	r.teacherNames = teacherNames
	r.schoolNames = schoolNames
	r.loaded = true

	slogPII.Info("dictionary refreshed", "students", len(studentNames), "teachers", len(teacherNames), "schools", len(schoolNames))
}

// Redact 对文本进行 PII 脱敏处理。
// 返回脱敏后的文本和被替换的 PII 数量。
func (r *PIIRedactor) Redact(text string) (string, int) {
	if text == "" {
		return text, 0
	}

	count := 0
	result := text

	// Step 1: 正则模式替换（注意顺序：先匹配长模式，再匹配短模式）
	// 身份证号 18 位 > 手机号 11 位，必须先替换身份证号
	result, n := r.redactPattern(result, r.idCardPattern, "[证件号]")
	count += n

	result, n = r.redactPattern(result, r.phonePattern, "[手机号]")
	count += n

	result, n = r.redactPattern(result, r.emailPattern, "[邮箱]")
	count += n

	// Step 2: 词典替换 — 学校名（先替换较长的实体，避免部分匹配）
	r.mu.RLock()
	schoolNames := make([]string, len(r.schoolNames))
	copy(schoolNames, r.schoolNames)
	studentNames := make([]string, len(r.studentNames))
	copy(studentNames, r.studentNames)
	teacherNames := make([]string, len(r.teacherNames))
	copy(teacherNames, r.teacherNames)
	r.mu.RUnlock()

	// 按长度降序排列，优先匹配更长的名称
	sortByLengthDesc(schoolNames)
	sortByLengthDesc(studentNames)
	sortByLengthDesc(teacherNames)

	// 替换学校名
	for _, name := range schoolNames {
		if name == "" {
			continue
		}
		if strings.Contains(result, name) {
			result = strings.ReplaceAll(result, name, "[学校]")
			count++
		}
	}

	// 替换学生姓名（只替换 >= 2 字符的名字，避免误匹配单字）
	for _, name := range studentNames {
		if len([]rune(name)) < 2 {
			continue
		}
		if strings.Contains(result, name) {
			result = strings.ReplaceAll(result, name, "[学生]")
			count++
		}
	}

	// 替换教师姓名
	for _, name := range teacherNames {
		if len([]rune(name)) < 2 {
			continue
		}
		if strings.Contains(result, name) {
			result = strings.ReplaceAll(result, name, "[教师]")
			count++
		}
	}

	return result, count
}

// RedactMessages 批量脱敏消息列表（用于 LLM 调用前）。
// 只脱敏 role=user 的消息，system 和 assistant 消息保持不变。
func (r *PIIRedactor) RedactMessages(messages []ChatMessageLike) ([]ChatMessageLike, int) {
	totalCount := 0
	result := make([]ChatMessageLike, len(messages))

	for i, msg := range messages {
		result[i] = msg
		if msg.GetRole() == "user" {
			redacted, count := r.Redact(msg.GetContent())
			if count > 0 {
				result[i] = msg.WithContent(redacted)
				totalCount += count
			}
		}
	}

	return result, totalCount
}

// ChatMessageLike 抽象消息接口，用于解耦 LLM 消息类型。
type ChatMessageLike interface {
	GetRole() string
	GetContent() string
	WithContent(content string) ChatMessageLike
}

// SimpleChatMessage 简单消息实现。
type SimpleChatMessage struct {
	Role    string
	Content string
}

func (m SimpleChatMessage) GetRole() string    { return m.Role }
func (m SimpleChatMessage) GetContent() string { return m.Content }
func (m SimpleChatMessage) WithContent(c string) ChatMessageLike {
	return SimpleChatMessage{Role: m.Role, Content: c}
}

// ── Internal Helpers ────────────────────────────────────────

// redactPattern 使用正则替换文本中的 PII。
func (r *PIIRedactor) redactPattern(text string, pattern *regexp.Regexp, replacement string) (string, int) {
	matches := pattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return text, 0
	}
	return pattern.ReplaceAllString(text, replacement), len(matches)
}

// sortByLengthDesc 按字符串长度降序排列。
func sortByLengthDesc(strs []string) {
	n := len(strs)
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			if len([]rune(strs[j])) > len([]rune(strs[i])) {
				strs[i], strs[j] = strs[j], strs[i]
			}
		}
	}
}

// RedactForLog 脱敏文本用于日志输出（更激进的脱敏）。
func RedactForLog(text string, maxLen int) string {
	if len([]rune(text)) > maxLen {
		text = string([]rune(text)[:maxLen]) + "..."
	}
	// 对日志中的手机号进行部分遮蔽
	phonePattern := regexp.MustCompile(`(1[3-9]\d)\d{4}(\d{4})`)
	text = phonePattern.ReplaceAllString(text, "${1}****${2}")
	return fmt.Sprintf("%q", text)
}
