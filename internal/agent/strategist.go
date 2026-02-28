package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hflms/hanfledge/internal/domain/model"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"gorm.io/gorm"
)

// ============================
// Strategist Agent — 策略师
// ============================
//
// 职责：宏观分析学生学情，生成学习处方 (LearningPrescription)。
// 输入：学生 ID、活动 ID → 查询 StudentKPMastery + Neo4j 前置知识
// 输出：LearningPrescription（目标 KP 序列、支架等级、推荐技能、前置差距）

// StrategistAgent 策略师 Agent。
type StrategistAgent struct {
	db    *gorm.DB
	neo4j *neo4jRepo.Client
}

// NewStrategistAgent 创建策略师 Agent。
func NewStrategistAgent(db *gorm.DB, neo4jClient *neo4jRepo.Client) *StrategistAgent {
	return &StrategistAgent{db: db, neo4j: neo4jClient}
}

// Name 返回 Agent 名称。
func (a *StrategistAgent) Name() string { return "Strategist" }

// Analyze 分析学生学情，生成学习处方。
// 1. 查询 LearningActivity → 获取目标 KP 列表
// 2. 查询 StudentKPMastery → 获取当前掌握度
// 3. 查询 Neo4j → 获取前置知识差距
// 4. 决定支架等级和技能推荐
func (a *StrategistAgent) Analyze(ctx context.Context, sessionID, studentID, activityID uint) (LearningPrescription, error) {
	log.Printf("🧠 [Strategist] Analyzing student=%d activity=%d", studentID, activityID)

	// Step 1: 加载学习活动 → 获取目标 KP IDs
	var activity model.LearningActivity
	if err := a.db.First(&activity, activityID).Error; err != nil {
		return LearningPrescription{}, fmt.Errorf("load activity %d: %w", activityID, err)
	}

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
	targets := make([]KnowledgePointTarget, 0, len(kpIDs))
	var prereqGaps []string

	for _, kpID := range kpIDs {
		currentMastery := masteryMap[kpID] // 默认 0.0
		if currentMastery == 0 {
			currentMastery = 0.1 // BKT 初始值 P(L0) = 0.1
		}

		// 决定该 KP 的支架等级
		scaffold := scaffoldForMastery(currentMastery)

		// 查询已挂载技能
		skillID := a.getSkillForKP(kpID)

		targets = append(targets, KnowledgePointTarget{
			KPID:           kpID,
			CurrentMastery: currentMastery,
			TargetMastery:  0.8, // 默认目标掌握度
			ScaffoldLevel:  scaffold,
			SkillID:        skillID,
		})

		// Step 4: 检查前置知识差距
		if a.neo4j != nil {
			gaps := a.checkPrereqGaps(ctx, kpID, studentID, masteryMap)
			prereqGaps = append(prereqGaps, gaps...)
		}
	}

	// 排序：掌握度低的优先
	sortTargetsByMastery(targets)

	// 选择推荐技能（取第一个目标 KP 的技能）
	recommendedSkill := ""
	if len(targets) > 0 && targets[0].SkillID != "" {
		recommendedSkill = targets[0].SkillID
	}

	prescription := LearningPrescription{
		SessionID:        sessionID,
		StudentID:        studentID,
		TargetKPSequence: targets,
		InitialScaffold:  scaffoldForMastery(averageMastery(targets)),
		RecommendedSkill: recommendedSkill,
		PrereqGaps:       prereqGaps,
	}

	log.Printf("   → Prescription: %d targets, scaffold=%s, gaps=%d",
		len(targets), prescription.InitialScaffold, len(prereqGaps))

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

// checkPrereqGaps 检查某个 KP 的前置知识点是否存在掌握度差距。
func (a *StrategistAgent) checkPrereqGaps(ctx context.Context, kpID, studentID uint, masteryMap map[uint]float64) []string {
	prereqs, err := a.neo4j.GetPrerequisites(ctx, kpID)
	if err != nil {
		log.Printf("⚠️  [Strategist] Get prerequisites for kp=%d failed: %v", kpID, err)
		return nil
	}

	var gaps []string
	for _, p := range prereqs {
		prereqTitle, _ := p["title"].(string)
		prereqID, _ := p["id"].(string)

		// 解析 kp_ID 格式获取数字 ID
		var numID uint
		fmt.Sscanf(prereqID, "kp_%d", &numID)

		mastery := masteryMap[numID]
		if mastery < 0.6 { // 前置知识掌握不足
			gaps = append(gaps, fmt.Sprintf("%s (mastery=%.2f)", prereqTitle, mastery))
		}
	}
	return gaps
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
