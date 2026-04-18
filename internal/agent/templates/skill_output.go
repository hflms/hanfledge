package templates

import (
	"encoding/json"
	"fmt"
)

// SkillOutput 统一的技能输出结构。
type SkillOutput struct {
	SkillID  string               `json:"skill_id"`
	Phase    string               `json:"phase,omitempty"`
	Data     interface{}          `json:"data"`
	Metadata *SkillOutputMetadata `json:"metadata,omitempty"`
}

// SkillOutputMetadata 输出元数据。
type SkillOutputMetadata struct {
	Confidence   float64       `json:"confidence,omitempty"`
	Reasoning    string        `json:"reasoning,omitempty"`
	Alternatives []interface{} `json:"alternatives,omitempty"`
}

// WrapSkillOutput 将数据包装为统一的技能输出格式。
func WrapSkillOutput(skillID, phase string, data interface{}, metadata *SkillOutputMetadata) string {
	output := SkillOutput{
		SkillID:  skillID,
		Phase:    phase,
		Data:     data,
		Metadata: metadata,
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Sprintf("<skill_output type=\"%s\">%v</skill_output>", skillID, data)
	}

	return fmt.Sprintf("<skill_output type=\"%s\">\n%s\n</skill_output>", skillID, string(jsonBytes))
}

// QuizTemplate 测验生成模板。
type QuizTemplate struct {
	Difficulty    string   // easy, medium, hard
	QuestionTypes []string // mcq_single, mcq_multiple, fill_blank
	MaxQuestions  int
}

// Generate 生成测验 prompt。
func (t *QuizTemplate) Generate(kpTitle string, mastery float64) string {
	return fmt.Sprintf(`请根据以下知识点生成 %d 道 %s 难度的测验题目：

知识点：%s
学生掌握度：%.2f

要求：
- 题型：%s
- 每题必须包含详细解析
- 选择题的干扰项必须具有迷惑性
- 填空题答案要明确唯一
- 输出格式必须为 JSON，包裹在 <skill_output type="quiz"> 标签中

JSON 格式示例：
<skill_output type="quiz">
{
  "skill_id": "general_assessment_quiz",
  "phase": "generating",
  "data": {
    "questions": [
      {
        "id": 1,
        "type": "mcq_single",
        "stem": "题目内容",
        "options": [
          {"key": "A", "text": "选项A"},
          {"key": "B", "text": "选项B"}
        ],
        "answer": ["A"],
        "explanation": "详细解析"
      }
    ]
  },
  "metadata": {
    "confidence": 0.92,
    "reasoning": "基于学生掌握度生成中等难度题目"
  }
}
</skill_output>`,
		t.MaxQuestions,
		t.Difficulty,
		kpTitle,
		mastery,
		joinStrings(t.QuestionTypes, "、"),
	)
}

// PresentationTemplate 演示文稿生成模板。
type PresentationTemplate struct {
	MaxSlides int
	Style     string // formal, casual, interactive
}

// Generate 生成演示文稿 prompt。
func (p *PresentationTemplate) Generate(kpTitle, kpDescription string) string {
	return fmt.Sprintf(`请为以下知识点生成一份演示文稿（最多 %d 张幻灯片）：

知识点：%s
描述：%s
风格：%s

要求：
- 使用 Reveal.js Markdown 格式
- 每张幻灯片用 --- 分隔
- 包含标题、要点、示例、总结
- 适当使用图表和可视化描述
- 输出格式包裹在 <skill_output type="presentation"> 标签中

示例格式：
<skill_output type="presentation">
{
  "skill_id": "presentation_generator",
  "phase": "generating",
  "data": {
    "slides": "# 标题\n\n要点1\n要点2\n\n---\n\n# 第二张\n\n内容..."
  }
}
</skill_output>`,
		p.MaxSlides,
		kpTitle,
		kpDescription,
		p.Style,
	)
}

// joinStrings 连接字符串数组。
func joinStrings(arr []string, sep string) string {
	if len(arr) == 0 {
		return ""
	}
	result := arr[0]
	for i := 1; i < len(arr); i++ {
		result += sep + arr[i]
	}
	return result
}
