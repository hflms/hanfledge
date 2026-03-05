package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var slogProfile = logger.L("ProfileService")

// ============================
// 跨会话学习分析服务
// ============================
//
// 负责维护 StudentProfile、LearningPathLog、StudentDimensionRecord。
// 由 AgentOrchestrator 在每轮交互后调用。

// ProfileService 管理学生跨会话画像和学习路径。
type ProfileService struct {
	db *gorm.DB
}

// NewProfileService 创建 ProfileService 实例。
func NewProfileService(db *gorm.DB) *ProfileService {
	return &ProfileService{db: db}
}

// ── StudentProfile 操作 ─────────────────────────────────────

// GetOrCreateProfile 获取或创建学生画像。
// 如果画像不存在，则使用默认值初始化。
func (s *ProfileService) GetOrCreateProfile(studentID uint) (*model.StudentProfile, error) {
	var profile model.StudentProfile
	result := s.db.Where("student_id = ?", studentID).First(&profile)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			now := time.Now()
			profile = model.StudentProfile{
				StudentID:       studentID,
				PreferredSkills: "[]",
				StrengthAreas:   "[]",
				WeaknessAreas:   "[]",
				Dimensions:      "{}",
				LastActiveAt:    now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := s.db.Create(&profile).Error; err != nil {
				return nil, fmt.Errorf("create student profile: %w", err)
			}
			slogProfile.Info("created student profile", "student_id", studentID)
		} else {
			return nil, result.Error
		}
	}
	return &profile, nil
}

// IncrementInteraction 增量更新交互计数和最后活跃时间。
// 每轮对话后调用。
func (s *ProfileService) IncrementInteraction(studentID uint) {
	now := time.Now()
	result := s.db.Model(&model.StudentProfile{}).
		Where("student_id = ?", studentID).
		Updates(map[string]interface{}{
			"total_interactions": gorm.Expr("total_interactions + 1"),
			"last_active_at":     now,
			"updated_at":         now,
		})
	if result.RowsAffected == 0 {
		// Profile 不存在，创建后再更新
		if _, err := s.GetOrCreateProfile(studentID); err != nil {
			slogProfile.Warn("auto-create profile failed", "student_id", studentID, "err", err)
			return
		}
		s.db.Model(&model.StudentProfile{}).
			Where("student_id = ?", studentID).
			Updates(map[string]interface{}{
				"total_interactions": gorm.Expr("total_interactions + 1"),
				"last_active_at":     now,
				"updated_at":         now,
			})
	}
}

// RefreshMasteryStats 重新计算学生的全局掌握度统计。
// 从 StudentKPMastery 表聚合计算，更新 OverallMastery/MasteredKPCount/WeakKPCount。
func (s *ProfileService) RefreshMasteryStats(studentID uint) {
	var stats struct {
		AvgMastery    float64
		MasteredCount int64
		WeakCount     int64
		TotalKPCount  int64
	}

	// 聚合查询
	s.db.Model(&model.StudentKPMastery{}).
		Select(`
			COALESCE(AVG(mastery_score), 0) AS avg_mastery,
			COUNT(CASE WHEN mastery_score >= 0.8 THEN 1 END) AS mastered_count,
			COUNT(CASE WHEN mastery_score < 0.4 THEN 1 END) AS weak_count,
			COUNT(*) AS total_kp_count
		`).
		Where("student_id = ?", studentID).
		Scan(&stats)

	now := time.Now()
	s.db.Model(&model.StudentProfile{}).
		Where("student_id = ?", studentID).
		Updates(map[string]interface{}{
			"overall_mastery":   stats.AvgMastery,
			"mastered_kp_count": stats.MasteredCount,
			"weak_kp_count":     stats.WeakCount,
			"updated_at":        now,
		})

	slogProfile.Debug("mastery stats refreshed",
		"student_id", studentID, "avg", stats.AvgMastery,
		"mastered", stats.MasteredCount, "weak", stats.WeakCount)
}

// RefreshStrengthWeakness 更新学生的优势/薄弱知识领域。
// 取掌握度最高的 5 个 KP 为优势区，最低的 5 个为薄弱区。
func (s *ProfileService) RefreshStrengthWeakness(studentID uint) {
	type KPMasteryItem struct {
		KPID    uint    `json:"kp_id"`
		Title   string  `json:"title"`
		Mastery float64 `json:"mastery"`
	}

	// 优势区: 掌握度最高的 5 个
	var strengths []KPMasteryItem
	s.db.Raw(`
		SELECT m.kp_id, COALESCE(kp.title, '') AS title, m.mastery_score AS mastery
		FROM student_kp_masteries m
		LEFT JOIN knowledge_points kp ON kp.id = m.kp_id
		WHERE m.student_id = ? AND m.mastery_score >= 0.6
		ORDER BY m.mastery_score DESC
		LIMIT 5
	`, studentID).Scan(&strengths)

	// 薄弱区: 掌握度最低的 5 个
	var weaknesses []KPMasteryItem
	s.db.Raw(`
		SELECT m.kp_id, COALESCE(kp.title, '') AS title, m.mastery_score AS mastery
		FROM student_kp_masteries m
		LEFT JOIN knowledge_points kp ON kp.id = m.kp_id
		WHERE m.student_id = ? AND m.mastery_score < 0.6
		ORDER BY m.mastery_score ASC
		LIMIT 5
	`, studentID).Scan(&weaknesses)

	strengthsJSON, _ := json.Marshal(strengths)
	weaknessesJSON, _ := json.Marshal(weaknesses)

	now := time.Now()
	s.db.Model(&model.StudentProfile{}).
		Where("student_id = ?", studentID).
		Updates(map[string]interface{}{
			"strength_areas": string(strengthsJSON),
			"weakness_areas": string(weaknessesJSON),
			"updated_at":     now,
		})
}

// UpdateSkillDimension 更新学生画像中某个 Skill 的维度数据。
// 采用 JSON 合并策略：读取现有 Dimensions → 合并 skill 数据 → 写回。
func (s *ProfileService) UpdateSkillDimension(studentID uint, skillID string, data map[string]interface{}) {
	profile, err := s.GetOrCreateProfile(studentID)
	if err != nil {
		slogProfile.Warn("update skill dimension: get profile failed", "err", err)
		return
	}

	// 解析现有 dimensions
	var dims map[string]interface{}
	if err := json.Unmarshal([]byte(profile.Dimensions), &dims); err != nil {
		dims = make(map[string]interface{})
	}

	// 合并 skill 维度
	dims[skillID] = data

	dimsJSON, _ := json.Marshal(dims)
	now := time.Now()
	s.db.Model(&model.StudentProfile{}).
		Where("student_id = ?", studentID).
		Updates(map[string]interface{}{
			"dimensions": string(dimsJSON),
			"updated_at": now,
		})

	slogProfile.Debug("skill dimension updated",
		"student_id", studentID, "skill_id", skillID)
}

// IncrementSessionCount 增加会话计数。
func (s *ProfileService) IncrementSessionCount(studentID uint) {
	now := time.Now()
	result := s.db.Model(&model.StudentProfile{}).
		Where("student_id = ?", studentID).
		Updates(map[string]interface{}{
			"total_sessions": gorm.Expr("total_sessions + 1"),
			"last_active_at": now,
			"updated_at":     now,
		})
	if result.RowsAffected == 0 {
		if _, err := s.GetOrCreateProfile(studentID); err != nil {
			slogProfile.Warn("auto-create profile for session count failed", "err", err)
			return
		}
		s.db.Model(&model.StudentProfile{}).
			Where("student_id = ?", studentID).
			Updates(map[string]interface{}{
				"total_sessions": gorm.Expr("total_sessions + 1"),
				"last_active_at": now,
				"updated_at":     now,
			})
	}
}

// ── LearningPathLog 操作 ────────────────────────────────────

// LogPathEvent 记录一条学习路径事件。
func (s *ProfileService) LogPathEvent(event model.LearningPathLog) {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}
	if event.Metadata == "" {
		event.Metadata = "{}"
	}

	if err := s.db.Create(&event).Error; err != nil {
		slogProfile.Warn("log path event failed",
			"event_type", event.EventType, "student_id", event.StudentID, "err", err)
		return
	}

	slogProfile.Debug("path event logged",
		"type", event.EventType, "student_id", event.StudentID,
		"from", event.FromState, "to", event.ToState)
}

// LogSkillSwitch 记录技能切换事件（便捷方法）。
func (s *ProfileService) LogSkillSwitch(studentID, sessionID, courseID, kpID uint,
	oldSkill, newSkill, reason string, mastery float64) {
	s.LogPathEvent(model.LearningPathLog{
		StudentID:      studentID,
		SessionID:      sessionID,
		CourseID:       courseID,
		EventType:      model.PathEventSkillSwitch,
		FromState:      oldSkill,
		ToState:        newSkill,
		TriggerReason:  reason,
		KPID:           kpID,
		SkillID:        newSkill,
		MasteryAtEvent: mastery,
	})
}

// LogScaffoldChange 记录支架变化事件（便捷方法）。
func (s *ProfileService) LogScaffoldChange(studentID, sessionID, courseID, kpID uint,
	oldLevel, newLevel string, mastery float64) {
	metadata, _ := json.Marshal(map[string]interface{}{
		"direction": scaffoldChangeDirection(oldLevel, newLevel),
	})
	s.LogPathEvent(model.LearningPathLog{
		StudentID:      studentID,
		SessionID:      sessionID,
		CourseID:       courseID,
		EventType:      model.PathEventScaffoldChange,
		FromState:      oldLevel,
		ToState:        newLevel,
		TriggerReason:  fmt.Sprintf("mastery=%.2f crossed threshold", mastery),
		KPID:           kpID,
		MasteryAtEvent: mastery,
		Metadata:       string(metadata),
	})
}

// LogMilestone 记录里程碑事件（便捷方法）。
func (s *ProfileService) LogMilestone(studentID, sessionID uint, description string, metadata map[string]interface{}) {
	metaJSON, _ := json.Marshal(metadata)
	s.LogPathEvent(model.LearningPathLog{
		StudentID:     studentID,
		SessionID:     sessionID,
		EventType:     model.PathEventMilestone,
		TriggerReason: description,
		Metadata:      string(metaJSON),
	})
}

// LogCustomEvent 记录 Skill 自定义事件（便捷方法）。
// 各 Skill 可通过此方法记录特有的路径事件。
func (s *ProfileService) LogCustomEvent(studentID, sessionID uint, skillID, description string, metadata map[string]interface{}) {
	metaJSON, _ := json.Marshal(metadata)
	s.LogPathEvent(model.LearningPathLog{
		StudentID:     studentID,
		SessionID:     sessionID,
		EventType:     model.PathEventCustom,
		SkillID:       skillID,
		TriggerReason: description,
		Metadata:      string(metaJSON),
	})
}

// GetStudentPath 查询学生的学习路径事件（支持分页和类型过滤）。
func (s *ProfileService) GetStudentPath(studentID uint, eventType *model.PathEventType, limit, offset int) ([]model.LearningPathLog, int64, error) {
	var logs []model.LearningPathLog
	var total int64

	query := s.db.Model(&model.LearningPathLog{}).Where("student_id = ?", studentID)
	if eventType != nil {
		query = query.Where("event_type = ?", *eventType)
	}

	query.Count(&total)

	if limit <= 0 {
		limit = 50
	}
	err := query.Order("occurred_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

// ── StudentDimensionRecord 操作 ─────────────────────────────

// UpsertDimension 写入或更新一条维度记录。
// 采用 PostgreSQL UPSERT (ON CONFLICT DO UPDATE) 保证幂等性。
func (s *ProfileService) UpsertDimension(record model.StudentDimensionRecord) error {
	now := time.Now()
	record.UpdatedAt = now
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.JSONValue == "" {
		record.JSONValue = "{}"
	}

	result := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "student_id"},
			{Name: "dimension_key"},
			{Name: "skill_id"},
			{Name: "course_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"numeric_value": record.NumericValue,
			"text_value":    record.TextValue,
			"json_value":    record.JSONValue,
			"version":       gorm.Expr("student_dimension_records.version + 1"),
			"updated_at":    now,
		}),
	}).Create(&record)

	if result.Error != nil {
		return fmt.Errorf("upsert dimension: %w", result.Error)
	}

	slogProfile.Debug("dimension upserted",
		"student_id", record.StudentID, "key", record.DimensionKey,
		"skill_id", record.SkillID)
	return nil
}

// UpsertNumericDimension 便捷方法：写入数值型维度。
func (s *ProfileService) UpsertNumericDimension(studentID uint, skillID, dimensionKey string, courseID uint, value float64) error {
	return s.UpsertDimension(model.StudentDimensionRecord{
		StudentID:    studentID,
		DimensionKey: dimensionKey,
		SkillID:      skillID,
		CourseID:     courseID,
		NumericValue: &value,
	})
}

// UpsertJSONDimension 便捷方法：写入结构化 JSON 维度。
func (s *ProfileService) UpsertJSONDimension(studentID uint, skillID, dimensionKey string, courseID uint, data interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal json dimension: %w", err)
	}
	return s.UpsertDimension(model.StudentDimensionRecord{
		StudentID:    studentID,
		DimensionKey: dimensionKey,
		SkillID:      skillID,
		CourseID:     courseID,
		JSONValue:    string(jsonBytes),
	})
}

// GetDimensions 查询学生的所有维度记录（支持按 Skill 过滤）。
func (s *ProfileService) GetDimensions(studentID uint, skillID *string) ([]model.StudentDimensionRecord, error) {
	var records []model.StudentDimensionRecord
	query := s.db.Where("student_id = ?", studentID)
	if skillID != nil {
		query = query.Where("skill_id = ?", *skillID)
	}
	err := query.Order("updated_at DESC").Find(&records).Error
	return records, err
}

// GetDimension 查询单条维度记录。
func (s *ProfileService) GetDimension(studentID uint, skillID, dimensionKey string, courseID uint) (*model.StudentDimensionRecord, error) {
	var record model.StudentDimensionRecord
	err := s.db.Where(
		"student_id = ? AND dimension_key = ? AND skill_id = ? AND course_id = ?",
		studentID, dimensionKey, skillID, courseID,
	).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ── Helpers ─────────────────────────────────────────────────

// scaffoldChangeDirection 返回支架变化方向描述。
func scaffoldChangeDirection(oldLevel, newLevel string) string {
	order := map[string]int{"high": 3, "medium": 2, "low": 1}
	if order[newLevel] < order[oldLevel] {
		return "fade"
	}
	return "strengthen"
}
