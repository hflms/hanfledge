package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/plugin"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"gorm.io/gorm"
)

var slogStrat = logger.L("Strategist")

// ============================
// Strategist Agent — 策略师
// ============================
//
// 职责：宏观分析学生学情，生成学习处方 (LearningPrescription)。
// 输入：学生 ID、活动 ID → 查询 StudentKPMastery + Neo4j 前置知识
// 输出：LearningPrescription（目标 KP 序列、支架等级、推荐技能、前置差距）

// StrategistAgent 策略师 Agent。
type StrategistAgent struct {
	db       *gorm.DB
	neo4j    *neo4jRepo.Client
	registry *plugin.Registry // 插件注册表，用于读取 ProgressiveTriggers
}

// NewStrategistAgent 创建策略师 Agent。
func NewStrategistAgent(db *gorm.DB, neo4jClient *neo4jRepo.Client, registry *plugin.Registry) *StrategistAgent {
	return &StrategistAgent{db: db, neo4j: neo4jClient, registry: registry}
}

// Name 返回 Agent 名称。
func (a *StrategistAgent) Name() string { return "Strategist" }

// Analyze 分析学生学情，生成学习处方。
// 1. 查询 LearningActivity → 获取目标 KP 列表和教学设计者
// 2. 查询 StudentKPMastery → 获取当前掌握度
// 3. 查询 Neo4j → 获取前置知识差距
// 4. 根据教学设计者策略决定支架等级和技能推荐
func (a *StrategistAgent) Analyze(ctx context.Context, sessionID, studentID, activityID uint) (LearningPrescription, error) {
	slogStrat.Info("analyzing student", "student_id", studentID, "activity_id", activityID)

	// Step 1: 加载学习活动 → 获取目标 KP IDs 和教学设计者
	var activity model.LearningActivity
	if err := a.db.First(&activity, activityID).Error; err != nil {
		return LearningPrescription{}, fmt.Errorf("load activity %d: %w", activityID, err)
	}

	// 加载教学设计者策略
	var designerStrategy *DesignerStrategy
	if activity.DesignerID != "" {
		designerStrategy = a.loadDesignerStrategy(activity.DesignerID, activity.DesignerConfig)
	}

	var session model.StudentSession
	if err := a.db.First(&session, sessionID).Error; err != nil {
		return LearningPrescription{}, fmt.Errorf("load session %d: %w", sessionID, err)
	}

	var targets []KnowledgePointTarget
	var prereqGaps []string
	var recommendedSkill string

	if activity.Type == model.ActivityTypeGuided {
		// Guided Activity Logic
		// Find current step
		var step model.ActivityStep
		if session.CurrentStepID != nil {
			if err := a.db.First(&step, *session.CurrentStepID).Error; err != nil {
				return LearningPrescription{}, fmt.Errorf("load step %d: %w", *session.CurrentStepID, err)
			}
		}

		// map StepType to Skill
		recommendedSkill = mapStepTypeToSkill(string(step.StepType))

		// Target KP can just be the current session KP (if any)
		targets = append(targets, KnowledgePointTarget{
			KPID:           session.CurrentKP,
			CurrentMastery: 0.5,
			TargetMastery:  0.8,
			ScaffoldLevel:  ScaffoldMedium,
			SkillID:        recommendedSkill,
		})

	} else {
		// Autonomous Activity Logic
		kpIDs, err := parseKPIDs(activity.KPIDS)
		if err != nil {
			return LearningPrescription{}, fmt.Errorf("parse kp_ids: %w", err)
		}

		// Step 2: 查询学生对每个 KP 的掌握度
		var masteries []model.StudentKPMastery
		a.db.Where("student_id = ? AND kp_id IN ?", studentID, kpIDs).Find(&masteries)

		masteryMap := make(map[uint]float64)
		for _, m := range masteries {
			masteryMap[m.KPID] = m.MasteryScore
		}

		// Step 3: 构建目标 KP 序列（按掌握度从低到高排列）
		for _, kpID := range kpIDs {
			currentMastery := masteryMap[kpID] // 默认 0.0
			if currentMastery == 0 {
				currentMastery = 0.1 // BKT 初始值 P(L0) = 0.1
			}

			// 决定该 KP 的支架等级（考虑教学设计者偏好）
			scaffold := a.scaffoldForMasteryWithDesigner(currentMastery, designerStrategy)

			// 查询已挂载技能
			skillID := a.getSkillForKP(kpID)

			// 动态技能降级策略 (§5.2 Dynamic Orchestration Fallback)
			if skillID == "" {
				skillID = a.findDynamicSkill(currentMastery)
				if skillID != "" {
					slogStrat.Info("dynamic skill fallback selected", "skill", skillID, "mastery", currentMastery, "kp_id", kpID)
				}
			}

			// Step 3.5: 渐进策略触发 — 检查是否满足技能切换条件 (§5.2 Step 2, item 4)
			// 例如: 苏格拉底引导 mastery >= 0.8 → 自动切换为谬误侦探
			if a.registry != nil && skillID != "" {
				if newSkillID, switched := a.evaluateProgressiveTriggers(skillID, currentMastery); switched {
					slogStrat.Info("progressive trigger fired",
						"from", skillID, "to", newSkillID, "mastery", currentMastery, "kp_id", kpID)
					skillID = newSkillID
				}
			}

			if a.neo4j != nil {
				prereqInserted := make(map[uint]bool)
				_, gapDescs := a.checkPrereqGapsEnriched(ctx, kpID, studentID, masteryMap, prereqInserted)
				prereqGaps = append(prereqGaps, gapDescs...)
			}

			targets = append(targets, KnowledgePointTarget{
				KPID:           kpID,
				CurrentMastery: currentMastery,
				TargetMastery:  0.8, // 默认目标掌握度
				ScaffoldLevel:  scaffold,
				SkillID:        skillID,
			})
		}

		// 排序：掌握度低的优先
		sortTargetsByMastery(targets)

		// 选择推荐技能（取第一个目标 KP 的技能）
		if len(targets) > 0 && targets[0].SkillID != "" {
			recommendedSkill = targets[0].SkillID
		}
	}

	// 日志：输出目标知识点序列
	if len(targets) > 0 {
		var kp model.KnowledgePoint
		if err := a.db.First(&kp, targets[0].KPID).Error; err == nil {
			slogStrat.Info("strategist analysis complete",
				"session_id", sessionID,
				"target_kp_id", targets[0].KPID,
				"target_kp_title", kp.Title,
				"current_mastery", targets[0].CurrentMastery,
				"skill_id", targets[0].SkillID)
		}
	}

	prescription := LearningPrescription{
		SessionID:        sessionID,
		StudentID:        studentID,
		TargetKPSequence: targets,
		InitialScaffold:  scaffoldForMastery(averageMastery(targets)),
		RecommendedSkill: recommendedSkill,
		PrereqGaps:       prereqGaps,
		DesignerStrategy: designerStrategy,
	}

	slogStrat.Debug("prescription generated",
		"targets", len(targets), "scaffold", prescription.InitialScaffold, "gaps", len(prereqGaps), "designer", activity.DesignerID)

	return prescription, nil
}

// ── Internal Helpers ────────────────────────────────────────

// scaffoldForMastery 根据掌握度决定支架等级。
// mastery >= 0.8 → low, >= 0.6 → medium, < 0.6 → high
func scaffoldForMastery(mastery float64) ScaffoldLevel {
	switch {
	case mastery >= 0.8:
		return ScaffoldLow
	case mastery >= 0.6:
		return ScaffoldMedium
	default:
		return ScaffoldHigh
	}
}

// mapStepTypeToSkill 映射环节类型到技能。
func mapStepTypeToSkill(stepType string) string {
	switch model.StepType(stepType) {
	case model.StepTypeLecture:
		return "knowledge_explainer"
	case model.StepTypeDiscussion:
		return "socratic_tutor"
	case model.StepTypeQuiz:
		return "quiz_generator"
	case model.StepTypePractice:
		return "step_by_step_coach"
	case model.StepTypeReading:
		return "reading_guide"
	case model.StepTypeGroupWork:
		return "socratic_tutor"
	case model.StepTypeReflection:
		return "learning_survey"
	case model.StepTypeAITutoring:
		return "socratic_tutor"
	default:
		return "knowledge_explainer"
	}
}

// scaffoldForMasteryWithDesigner 根据掌握度和教学设计者策略决定支架等级。
func (a *StrategistAgent) scaffoldForMasteryWithDesigner(mastery float64, designer *DesignerStrategy) ScaffoldLevel {
	if designer == nil {
		return scaffoldForMastery(mastery)
	}

	// 根据设计者偏好调整支架
	switch designer.ScaffoldPreference {
	case "high":
		return ScaffoldHigh
	case "low":
		return ScaffoldLow
	case "dynamic":
		return scaffoldForMastery(mastery)
	default: // "medium"
		if mastery >= 0.7 {
			return ScaffoldLow
		}
		return ScaffoldMedium
	}
}

// getSkillForKP 查询知识点挂载的技能 ID（取最高优先级的）。
func (a *StrategistAgent) getSkillForKP(kpID uint) string {
	var mount model.KPSkillMount
	err := a.db.Where("kp_id = ?", kpID).
		Order("priority DESC").
		First(&mount).Error
	if err != nil {
		return "" // 没有挂载技能
	}
	return mount.SkillID
}

// checkPrereqGapsEnriched 检查某个 KP 的前置知识点是否存在掌握度差距。
// 当发现差距时，自动生成前置 KP 的复习目标插入到序列中（§5.2 Step 1: 自动插入前置复习环节）。
// 返回: (需要插入的前置 KP 目标列表, 人类可读的差距描述列表)
func (a *StrategistAgent) checkPrereqGapsEnriched(ctx context.Context, kpID, studentID uint, masteryMap map[uint]float64, inserted map[uint]bool) ([]KnowledgePointTarget, []string) {
	prereqs, err := a.neo4j.GetPrerequisites(ctx, kpID)
	if err != nil {
		slogStrat.Warn("get prerequisites failed", "kp_id", kpID, "err", err)
		return nil, nil
	}

	var gapTargets []KnowledgePointTarget
	var gapDescs []string

	for _, p := range prereqs {
		prereqTitle, _ := p["title"].(string)
		prereqID, _ := p["id"].(string)

		// 解析 kp_ID 格式获取数字 ID
		var numID uint
		fmt.Sscanf(prereqID, "kp_%d", &numID)

		mastery := masteryMap[numID]
		if mastery == 0 {
			mastery = 0.1 // BKT 初始值
		}

		if mastery < 0.6 { // 前置知识掌握不足
			gapDescs = append(gapDescs, fmt.Sprintf("%s (mastery=%.2f)", prereqTitle, mastery))

			// 自动插入前置 KP 到复习序列（去重）
			if !inserted[numID] {
				inserted[numID] = true
				scaffold := scaffoldForMastery(mastery)
				skillID := a.getSkillForKP(numID)

				gapTargets = append(gapTargets, KnowledgePointTarget{
					KPID:           numID,
					CurrentMastery: mastery,
					TargetMastery:  0.6, // 前置知识目标: 达到 medium 即可解锁后续
					ScaffoldLevel:  scaffold,
					SkillID:        skillID,
				})

				slogStrat.Info("auto-inserted prereq",
					"kp_id", numID, "title", prereqTitle, "mastery", mastery)
			}
		}
	}
	return gapTargets, gapDescs
}

// averageMastery 计算目标 KP 序列的平均掌握度。
func averageMastery(targets []KnowledgePointTarget) float64 {
	if len(targets) == 0 {
		return 0.1
	}
	sum := 0.0
	for _, t := range targets {
		sum += t.CurrentMastery
	}
	return sum / float64(len(targets))
}

// sortTargetsByMastery 按掌握度从低到高排序（掌握度低的优先练习）。
func sortTargetsByMastery(targets []KnowledgePointTarget) {
	for i := 1; i < len(targets); i++ {
		for j := i; j > 0 && targets[j].CurrentMastery < targets[j-1].CurrentMastery; j-- {
			targets[j], targets[j-1] = targets[j-1], targets[j]
		}
	}
}

// parseKPIDs 从 JSONB 字段解析知识点 ID 列表。
func parseKPIDs(jsonStr string) ([]uint, error) {
	if jsonStr == "" || jsonStr == "null" {
		return nil, fmt.Errorf("empty kp_ids")
	}

	// KPIDS is stored as JSON array, e.g. [1, 2, 3]
	var ids []uint
	// Use simple parsing — json.Unmarshal handles []uint from JSON array
	if err := parseJSONSafe(jsonStr, &ids); err != nil {
		// Try parsing as []float64 first (JSON numbers)
		var floatIDs []float64
		if err2 := parseJSONSafe(jsonStr, &floatIDs); err2 != nil {
			return nil, fmt.Errorf("invalid kp_ids JSON: %w", err)
		}
		ids = make([]uint, len(floatIDs))
		for i, f := range floatIDs {
			ids[i] = uint(f)
		}
	}

	return ids, nil
}

// parseJSONSafe 安全解析 JSON 字符串。
func parseJSONSafe(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// ── Progressive Trigger Evaluation (§5.2 Step 2) ───────────

// evaluateProgressiveTriggers 检查当前技能是否应根据渐进策略触发器切换到另一个技能。
//
// 工作原理：
//  1. 检查当前技能的 deactivate_when 条件是否满足 → 如果满足，说明应该离开当前技能
//  2. 遍历所有注册技能，找到 activate_when 条件满足的候选技能
//  3. 返回新技能 ID 和 true；如果没有触发则返回 ("", false)
//
// 例如：苏格拉底引导 deactivate_when="mastery_score >= 0.8"，
// 谬误侦探 activate_when="mastery_score >= 0.8"，
// 当 mastery >= 0.8 时，从苏格拉底切换到谬误侦探。
func (a *StrategistAgent) evaluateProgressiveTriggers(currentSkillID string, mastery float64) (string, bool) {
	// Step 1: 检查当前技能是否应该 deactivate
	currentSkill, ok := a.registry.GetSkill(currentSkillID)
	if !ok {
		return "", false
	}

	triggers := currentSkill.Metadata.ProgressiveTriggers
	if triggers == nil || triggers.DeactivateWhen == "" {
		return "", false // 当前技能没有 deactivate 条件
	}

	if !parseTriggerCondition(triggers.DeactivateWhen, mastery) {
		return "", false // 条件未满足，保持当前技能
	}

	// Step 2: 当前技能应该 deactivate，寻找应该 activate 的替代技能
	allSkills := a.registry.ListSkills("", "")
	for _, candidate := range allSkills {
		if candidate.Metadata.ID == currentSkillID {
			continue // 跳过当前技能
		}

		ct := candidate.Metadata.ProgressiveTriggers
		if ct == nil || ct.ActivateWhen == "" {
			continue
		}

		if parseTriggerCondition(ct.ActivateWhen, mastery) {
			return candidate.Metadata.ID, true
		}
	}

	return "", false // 没有找到合适的替代技能
}

// parseTriggerCondition 解析并评估触发条件字符串。
// 支持格式: "mastery_score >= 0.8", "mastery_score < 0.6" 等。
// 支持的操作符: >=, <=, >, <, ==, !=
// 当前仅支持 mastery_score 变量。
func parseTriggerCondition(condition string, mastery float64) bool {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return false
	}

	// 解析格式: "<variable> <operator> <value>"
	parts := strings.Fields(condition)
	if len(parts) != 3 {
		slogStrat.Warn("invalid trigger condition format", "condition", condition)
		return false
	}

	variable := parts[0]
	operator := parts[1]
	valueStr := parts[2]

	// 当前仅支持 mastery_score
	if variable != "mastery_score" {
		slogStrat.Warn("unsupported trigger variable", "variable", variable)
		return false
	}

	threshold, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		slogStrat.Warn("invalid trigger threshold", "value", valueStr, "err", err)
		return false
	}

	switch operator {
	case ">=":
		return mastery >= threshold
	case "<=":
		return mastery <= threshold
	case ">":
		return mastery > threshold
	case "<":
		return mastery < threshold
	case "==":
		return mastery == threshold
	case "!=":
		return mastery != threshold
	default:
		slogStrat.Warn("unsupported trigger operator", "operator", operator)
		return false
	}
}

// findDynamicSkill 当没有静态绑定的技能时，基于掌握度动态选择合适的技能。
func (a *StrategistAgent) findDynamicSkill(mastery float64) string {
	if a.registry == nil {
		return "socratic" // fallback 默认
	}
	allSkills := a.registry.ListSkills("", "")
	for _, candidate := range allSkills {
		ct := candidate.Metadata.ProgressiveTriggers
		if ct == nil || ct.ActivateWhen == "" {
			continue
		}
		if parseTriggerCondition(ct.ActivateWhen, mastery) {
			return candidate.Metadata.ID
		}
	}
	return "socratic" // 默认退底为苏格拉底
}

// loadDesignerStrategy 加载教学设计者策略。
func (a *StrategistAgent) loadDesignerStrategy(designerID, configJSON string) *DesignerStrategy {
	var designer model.InstructionalDesigner
	if err := a.db.First(&designer, "id = ?", designerID).Error; err != nil {
		slogStrat.Warn("load designer failed", "id", designerID, "err", err)
		return nil
	}

	var config map[string]interface{}
	if configJSON != "" && configJSON != "{}" {
		json.Unmarshal([]byte(configJSON), &config)
	}

	return &DesignerStrategy{
		ID:                 designer.ID,
		Name:               designer.Name,
		SkillCoordination:  "adaptive",                        // 默认自适应协调
		ScaffoldPreference: "dynamic",                         // 默认动态支架
		InterventionStyle:  string(designer.InterventionStyle),
		Config:             config,
	}
}
