package plugin

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// ============================
// Anti-God Skill Validator
// ============================
//
// Reference: design.md §8.2.1 — Namespacing & Anti-God Skill
//
// 当平台挂载的教学 Skills 从几十个增长到成百上千时，
// 需要工程化治理来防止"上帝技能"和"工具混淆"。

// SkillValidationResult 技能验证结果。
type SkillValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// SkillValidator 技能验证器，强制执行命名空间规范和反"上帝技能"规则。
type SkillValidator struct {
	MaxSkillMDTokens  int // SKILL.md 最大 Token 数（默认 2000）
	MaxEvalDimensions int // 最大评估维度数（默认 5）
	MaxSubjectSpan    int // 触发拆分警告的学科跨度（默认 3）
}

// DefaultSkillValidator 返回使用默认配置的 SkillValidator。
func DefaultSkillValidator() *SkillValidator {
	return &SkillValidator{
		MaxSkillMDTokens:  2000,
		MaxEvalDimensions: 5,
		MaxSubjectSpan:    3,
	}
}

// Validate 验证一个已注册的技能是否符合工程化治理规范。
// 检查规则:
//  1. 命名空间: 三段式命名 {学科}_{场景}_{方法}
//  2. SKILL.md Token 上限: 不超过 MaxSkillMDTokens (默认 2000)
//  3. 评估维度上限: 不超过 MaxEvalDimensions (默认 5)
//  4. 学科跨度警告: 超过 MaxSubjectSpan 个不相关学科时发出拆分警告
func (v *SkillValidator) Validate(skill *RegisteredSkill) SkillValidationResult {
	result := SkillValidationResult{Valid: true}

	// Rule 1: 命名空间检查 — 三段式 {学科}_{场景}_{方法}
	v.validateNamespace(skill.Metadata.ID, &result)

	// Rule 2: SKILL.md Token 上限检查
	v.validateSkillMDSize(skill, &result)

	// Rule 3: 评估维度上限检查
	v.validateEvalDimensions(skill.Metadata.EvalDimensions, &result)

	// Rule 4: 学科跨度检查
	v.validateSubjectSpan(skill.Metadata.ID, skill.Metadata.Subjects, &result)

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// ValidateNamespaceOnly 仅验证技能 ID 命名空间（不需要完整 RegisteredSkill）。
// 用于在创建/注册前快速校验。
func (v *SkillValidator) ValidateNamespaceOnly(skillID string) error {
	result := SkillValidationResult{Valid: true}
	v.validateNamespace(skillID, &result)
	if len(result.Errors) > 0 {
		return fmt.Errorf("invalid skill ID: %s", strings.Join(result.Errors, "; "))
	}
	return nil
}

// -- Validation Rules ------------------------------------------------

// validateNamespace 检查技能 ID 是否遵循三段式命名规范。
// 格式: {学科}_{场景}_{方法}
// 例如: math_concept_socratic, physics_homework_fallacy
func (v *SkillValidator) validateNamespace(skillID string, result *SkillValidationResult) {
	if skillID == "" {
		result.Errors = append(result.Errors, "skill ID 不能为空")
		return
	}

	segments := strings.Split(skillID, "_")

	// 基本检查: 至少 3 段
	if len(segments) < 3 {
		result.Errors = append(result.Errors,
			fmt.Sprintf("skill ID '%s' 不符合三段式命名规范 ({学科}_{场景}_{方法})，当前只有 %d 段", skillID, len(segments)))
		return
	}

	subject := segments[0]
	scenario := segments[1]
	method := strings.Join(segments[2:], "_") // 允许方法部分包含下划线

	// 各段不能为空
	if subject == "" {
		result.Errors = append(result.Errors, "skill ID 的学科段不能为空")
	}
	if scenario == "" {
		result.Errors = append(result.Errors, "skill ID 的场景段不能为空")
	}
	if method == "" {
		result.Errors = append(result.Errors, "skill ID 的方法段不能为空")
	}

	// 只允许小写字母和数字
	for _, seg := range segments {
		for _, r := range seg {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
				result.Errors = append(result.Errors,
					fmt.Sprintf("skill ID 的段 '%s' 包含非法字符 '%c'（只允许小写字母和数字）", seg, r))
				break
			}
		}
	}

	// 已知学科前缀 (可扩展)
	knownSubjects := map[string]bool{
		"math": true, "physics": true, "chemistry": true, "biology": true,
		"english": true, "chinese": true, "history": true, "geography": true,
		"politics": true, "general": true, "cs": true, "art": true, "music": true,
	}

	if !knownSubjects[subject] {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("学科 '%s' 不在已知学科列表中，请确认是否正确", subject))
	}
}

// validateSkillMDSize 检查 SKILL.md 文件的 Token 数是否超出限制。
func (v *SkillValidator) validateSkillMDSize(skill *RegisteredSkill, result *SkillValidationResult) {
	maxTokens := v.maxSkillMDTokens()

	// 如果有文件系统路径，读取文件
	if skill.SkillMDPath != "" {
		data, err := os.ReadFile(skill.SkillMDPath)
		if err != nil {
			// 文件不存在的问题已经在 validateSkill 中检查过
			return
		}

		tokens := estimateSkillTokens(string(data))
		if tokens > maxTokens {
			result.Errors = append(result.Errors,
				fmt.Sprintf("SKILL.md 超出 Token 限制: %d tokens（上限 %d）", tokens, maxTokens))
		}
		return
	}

	// 如果是编程式插件，从 LoadConstraints 获取内容
	if skill.Impl != nil {
		// 编程式插件的约束内容在运行时加载，这里仅记录警告
		log.Printf("[Validator] Programmatic skill %s: SKILL.md size check deferred to runtime", skill.Metadata.ID)
	}
}

// validateEvalDimensions 检查评估维度是否超过上限。
func (v *SkillValidator) validateEvalDimensions(dims []string, result *SkillValidationResult) {
	maxDims := v.maxEvalDimensions()
	if len(dims) > maxDims {
		result.Errors = append(result.Errors,
			fmt.Sprintf("评估维度过多: %d 个（上限 %d 个）", len(dims), maxDims))
	}
}

// validateSubjectSpan 检查技能是否跨越过多不相关学科。
func (v *SkillValidator) validateSubjectSpan(skillID string, subjects []string, result *SkillValidationResult) {
	maxSpan := v.maxSubjectSpan()

	if len(subjects) > maxSpan {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("技能 '%s' 横跨 %d 个学科 (%s)，建议拆分为多个专项技能",
				skillID, len(subjects), strings.Join(subjects, ", ")))
	}
}

// -- Config Defaults -------------------------------------------------

func (v *SkillValidator) maxSkillMDTokens() int {
	if v.MaxSkillMDTokens <= 0 {
		return 2000
	}
	return v.MaxSkillMDTokens
}

func (v *SkillValidator) maxEvalDimensions() int {
	if v.MaxEvalDimensions <= 0 {
		return 5
	}
	return v.MaxEvalDimensions
}

func (v *SkillValidator) maxSubjectSpan() int {
	if v.MaxSubjectSpan <= 0 {
		return 3
	}
	return v.MaxSubjectSpan
}

// -- Token Estimation ------------------------------------------------

// estimateSkillTokens 粗略估算文本的 Token 数。
// 中文约 1.5 字符/Token，英文约 4 字符/Token。
func estimateSkillTokens(text string) int {
	runes := []rune(text)
	chineseCount := 0
	englishCount := 0
	for _, r := range runes {
		if r > 127 {
			chineseCount++
		} else {
			englishCount++
		}
	}
	return int(float64(chineseCount)/1.5) + int(float64(englishCount)/4.0) + 1
}
