package safety

import (
	"regexp"
	"testing"
)

// ── PII Redactor Tests (without DB) ─────────────────────────

func TestPIIRedactor_PhoneNumbers(t *testing.T) {
	r := &PIIRedactor{
		phonePattern:  newPhonePattern(),
		emailPattern:  newEmailPattern(),
		idCardPattern: newIDCardPattern(),
		loaded:        true,
	}

	tests := []struct {
		input    string
		expected string
		count    int
	}{
		{"我的手机号是13812345678", "我的手机号是[手机号]", 1},
		{"联系电话：13912345678 和 15012345678", "联系电话：[手机号] 和 [手机号]", 2},
		{"没有手机号的文本", "没有手机号的文本", 0},
		{"1234567890不是手机号", "1234567890不是手机号", 0},
	}

	for _, tc := range tests {
		result, count := r.Redact(tc.input)
		if result != tc.expected {
			t.Errorf("Redact(%q) = %q, want %q", tc.input, result, tc.expected)
		}
		if count != tc.count {
			t.Errorf("Redact(%q) count = %d, want %d", tc.input, count, tc.count)
		}
	}
}

func TestPIIRedactor_Email(t *testing.T) {
	r := &PIIRedactor{
		phonePattern:  newPhonePattern(),
		emailPattern:  newEmailPattern(),
		idCardPattern: newIDCardPattern(),
		loaded:        true,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"邮箱是test@example.com", "邮箱是[邮箱]"},
		{"student123@school.edu.cn 是学校邮箱", "[邮箱] 是学校邮箱"},
	}

	for _, tc := range tests {
		result, _ := r.Redact(tc.input)
		if result != tc.expected {
			t.Errorf("Redact(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestPIIRedactor_IDCard(t *testing.T) {
	r := &PIIRedactor{
		phonePattern:  newPhonePattern(),
		emailPattern:  newEmailPattern(),
		idCardPattern: newIDCardPattern(),
		loaded:        true,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"身份证号110101199001011234", "身份证号[证件号]"},
		{"证件：32010119900101123X", "证件：[证件号]"},
	}

	for _, tc := range tests {
		result, _ := r.Redact(tc.input)
		if result != tc.expected {
			t.Errorf("Redact(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestPIIRedactor_DictionaryRedaction(t *testing.T) {
	r := &PIIRedactor{
		phonePattern:  newPhonePattern(),
		emailPattern:  newEmailPattern(),
		idCardPattern: newIDCardPattern(),
		loaded:        true,
		studentNames:  []string{"张三", "李明明"},
		teacherNames:  []string{"王老师"},
		schoolNames:   []string{"北京市第一中学"},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"我是张三同学", "我是[学生]同学"},
		{"李明明说过", "[学生]说过"},
		{"北京市第一中学的学生", "[学校]的学生"},
		{"王老师教得好", "[教师]教得好"},
	}

	for _, tc := range tests {
		result, count := r.Redact(tc.input)
		if result != tc.expected {
			t.Errorf("Redact(%q) = %q, want %q", tc.input, result, tc.expected)
		}
		if count == 0 {
			t.Errorf("Redact(%q) expected count > 0", tc.input)
		}
	}
}

func TestPIIRedactor_SkipShortNames(t *testing.T) {
	r := &PIIRedactor{
		phonePattern:  newPhonePattern(),
		emailPattern:  newEmailPattern(),
		idCardPattern: newIDCardPattern(),
		loaded:        true,
		studentNames:  []string{"张"},
	}

	// 单字姓名不应被替换（防止误匹配）
	result, count := r.Redact("我姓张")
	if count != 0 {
		t.Errorf("Expected 0 replacements for single-char name, got %d (result: %q)", count, result)
	}
}

func TestRedactForLog(t *testing.T) {
	result := RedactForLog("我的电话是13812345678", 50)
	if result != `"我的电话是138****5678"` {
		t.Errorf("RedactForLog unexpected result: %s", result)
	}
}

// ── Helpers ─────────────────────────────────────────────────

func newPhonePattern() *regexp.Regexp {
	return regexp.MustCompile(`1[3-9]\d{9}`)
}

func newEmailPattern() *regexp.Regexp {
	return regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
}

func newIDCardPattern() *regexp.Regexp {
	return regexp.MustCompile(`[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`)
}

func TestPIIRedactor_EmptyAndEdgeCases(t *testing.T) {
	r := &PIIRedactor{
		phonePattern:  newPhonePattern(),
		emailPattern:  newEmailPattern(),
		idCardPattern: newIDCardPattern(),
		loaded:        true,
		schoolNames:   []string{"", "正常学校"},
		studentNames:  []string{"A", "小明"},
		teacherNames:  []string{"B", "王老师"},
	}

	// 测试空文本
	result, count := r.Redact("")
	if result != "" || count != 0 {
		t.Errorf("Expected empty result and 0 count, got %q and %d", result, count)
	}

	// 测试空词典项和短姓名是否被跳过
	input := "A和B在正常学校遇到了小明和王老师"
	expected := "A和B在[学校]遇到了[学生]和[教师]"
	result, count = r.Redact(input)
	if result != expected {
		t.Errorf("Redact(%q) = %q, want %q", input, result, expected)
	}
	if count != 3 {
		t.Errorf("Redact(%q) count = %d, want 3", input, count)
	}
}

func TestPIIRedactor_RedactMessages(t *testing.T) {
	r := &PIIRedactor{
		phonePattern:  newPhonePattern(),
		emailPattern:  newEmailPattern(),
		idCardPattern: newIDCardPattern(),
		loaded:        true,
		studentNames:  []string{"小明"},
	}

	messages := []ChatMessageLike{
		SimpleChatMessage{Role: "system", Content: "你是老师，小明是你的学生"},
		SimpleChatMessage{Role: "user", Content: "小明的手机号是13812345678"},
		SimpleChatMessage{Role: "assistant", Content: "好的，我已经知道小明的手机号了。"},
		SimpleChatMessage{Role: "user", Content: "没有PII的信息"},
	}

	redacted, count := r.RedactMessages(messages)
	if count != 2 {
		t.Errorf("RedactMessages count = %d, want 2", count)
	}

	// 检查系统消息和助手消息是否保持不变
	if redacted[0].GetContent() != "你是老师，小明是你的学生" {
		t.Errorf("Expected system message to remain unchanged, got %q", redacted[0].GetContent())
	}
	if redacted[2].GetContent() != "好的，我已经知道小明的手机号了。" {
		t.Errorf("Expected assistant message to remain unchanged, got %q", redacted[2].GetContent())
	}

	// 检查用户消息是否被脱敏
	if redacted[1].GetContent() != "[学生]的手机号是[手机号]" {
		t.Errorf("Expected user message to be redacted, got %q", redacted[1].GetContent())
	}
	if redacted[3].GetContent() != "没有PII的信息" {
		t.Errorf("Expected user message without PII to remain unchanged, got %q", redacted[3].GetContent())
	}
}

func TestRedactForLog_LongText(t *testing.T) {
	input := "这是一段非常长的文本，用来测试超过最大长度限制时的截断功能。包含手机号13812345678也会被处理。"
	result := RedactForLog(input, 15)

	// 期望截断到15个字符并添加...
	expected := `"这是一段非常长的文本，用来测试..."`
	if result != expected {
		t.Errorf("RedactForLog(%q, 15) = %q, want %q", input, result, expected)
	}
}

func TestSimpleChatMessage(t *testing.T) {
	msg := SimpleChatMessage{Role: "user", Content: "hello"}

	if msg.GetRole() != "user" {
		t.Errorf("GetRole() = %q, want user", msg.GetRole())
	}
	if msg.GetContent() != "hello" {
		t.Errorf("GetContent() = %q, want hello", msg.GetContent())
	}

	newMsg := msg.WithContent("world")
	if newMsg.GetContent() != "world" {
		t.Errorf("WithContent() = %q, want world", newMsg.GetContent())
	}
	if newMsg.GetRole() != "user" {
		t.Errorf("WithContent().GetRole() = %q, want user", newMsg.GetRole())
	}
}
