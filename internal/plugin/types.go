package plugin

// ============================
// 技能插件类型定义
// ============================

// SkillMetadata 描述一个技能插件的元数据（对应 metadata.json）。
// 常驻内存，用于 Skill Store 列表展示和意图匹配路由。
type SkillMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Category    string   `json:"category"`
	Subjects    []string `json:"subjects"`
	Tags        []string `json:"tags"`

	ScaffoldingLevels []string               `json:"scaffolding_levels"`
	Constraints       map[string]interface{} `json:"constraints"`
	Tools             map[string]ToolConfig  `json:"tools,omitempty"`

	ProgressiveTriggers *ProgressiveTriggers `json:"progressive_triggers,omitempty"`
	EvalDimensions      []string             `json:"evaluation_dimensions,omitempty"`
}

// ToolConfig 技能可用工具配置。
type ToolConfig struct {
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// ProgressiveTriggers 渐进式策略触发条件。
type ProgressiveTriggers struct {
	ActivateWhen   string `json:"activate_when,omitempty"`
	DeactivateWhen string `json:"deactivate_when,omitempty"`
}

// SkillConstraints 技能约束指令（对应 SKILL.md 内容）。
// 渐进式披露的"触发层"。
type SkillConstraints struct {
	SkillID     string `json:"skill_id"`
	RawMarkdown string `json:"raw_markdown"`
}

// SkillTemplate 技能评分量规或 Prompt 模板。
// 渐进式披露的"参考层"。
type SkillTemplate struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	Content  string `json:"content"`
}

// RegisteredSkill 表示一个已注册到 Registry 中的完整技能。
// 包含元数据层、触发层路径、参考层路径。
type RegisteredSkill struct {
	Metadata      SkillMetadata `json:"metadata"`
	BasePath      string        `json:"-"` // 文件系统中的技能根目录
	SkillMDPath   string        `json:"-"` // SKILL.md 文件路径
	TemplatesPath string        `json:"-"` // templates/ 目录路径
}
