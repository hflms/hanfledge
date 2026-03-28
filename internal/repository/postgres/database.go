package postgres

import (
	"fmt"

	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var slogDB = logger.L("Database")

// NewConnection creates a new GORM database connection to PostgreSQL.
func NewConnection(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	logLevel := gormlogger.Info
	if cfg.Host != "localhost" {
		logLevel = gormlogger.Warn
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	slogDB.Info("postgresql connected successfully")
	return db, nil
}

// AutoMigrate runs GORM auto-migration for all domain models.
// This creates or updates tables to match the Go struct definitions.
func AutoMigrate(db *gorm.DB) error {
	slogDB.Info("running database auto-migration")

	// Enable pgvector extension
	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")

	// Fix empty string values in timestamp columns before migration.
	// If columns were previously text/varchar, empty strings cannot be cast to timestamptz.
	fixTimestampSQL := []string{
		`UPDATE courses SET created_at = NULL WHERE created_at = '' OR created_at IS NOT NULL AND created_at::text = ''`,
		`UPDATE courses SET updated_at = NULL WHERE updated_at = '' OR updated_at IS NOT NULL AND updated_at::text = ''`,
	}
	for _, sql := range fixTimestampSQL {
		if err := db.Exec(sql).Error; err != nil {
			slogDB.Warn("timestamp fix query skipped (table may not exist yet)", "err", err)
		}
	}

	err := db.AutoMigrate(
		// 用户与权限
		&model.User{},
		&model.School{},
		&model.Class{},
		&model.Role{},
		&model.UserSchoolRole{}, model.UserSchoolRole{}, model.UserSchoolRole{},
		&model.SystemConfig{},
		&model.ClassStudent{},
		// 课程与知识
		&model.Course{},
		&model.Chapter{},
		&model.KnowledgePoint{},
		// 技能挂载
		&model.KPSkillMount{},
		// 学习活动
		&model.LearningActivity{},
		&model.ActivityClassAssignment{},
		&model.ActivityStep{},
		// 教学设计者
		&model.InstructionalDesigner{},
		// 交互与学情
		&model.StudentSession{},
		&model.Interaction{},
		&model.StudentKPMastery{},
		&model.ErrorNotebookEntry{},
		// 文档与向量
		&model.Document{},
		&model.DocumentChunk{},
		// 知识图谱扩展
		&model.Misconception{},
		&model.CrossLink{},
		// 成就系统
		&model.AchievementDefinition{},
		&model.StudentAchievement{},
		// 自定义技能
		&model.CustomSkill{},
		&model.CustomSkillVersion{},
		// 插件市场
		&model.MarketplacePlugin{},
		&model.MarketplaceReview{},
		&model.InstalledPlugin{},
		// WeKnora 知识库引用
		&model.WeKnoraKBRef{},
		// WeKnora 用户 Token 映射
		&model.WeKnoraToken{},
		// 性能监控
		&model.AnalyticsEvent{},
		// Soul 版本管理
		&model.SoulVersion{},
		// 系统通知
		&model.Notification{},
	)
	if err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	// Seed default roles if not exist
	seedRoles(db)
	// Seed achievement definitions if not exist
	seedAchievements(db)
	// Seed instructional designers if not exist
	seedDesigners(db)
	// Create performance indexes
	createPerformanceIndexes(db)

	slogDB.Info("database migration completed")
	return nil
}

// seedRoles inserts the four default roles if they don't exist.
func seedRoles(db *gorm.DB) {
	roles := []model.Role{
		{ID: 1, Name: model.RoleSysAdmin},
		{ID: 2, Name: model.RoleSchoolAdmin},
		{ID: 3, Name: model.RoleTeacher},
		{ID: 4, Name: model.RoleStudent},
	}
	for _, r := range roles {
		db.FirstOrCreate(&r, model.Role{Name: r.Name})
	}
	slogDB.Info("default roles seeded")
}

// seedAchievements inserts the predefined achievement definitions (3 types × 4 tiers).
// design.md §5.2 Step 4: 连续突破徽章、深度追问勋章、谬误猎人。
func seedAchievements(db *gorm.DB) {
	defs := []model.AchievementDefinition{
		// 连续突破徽章 — 连续掌握知识点数
		{ID: 1, Type: model.AchievementStreakBreaker, Tier: model.TierBronze, Name: "初露锋芒", Description: "连续掌握 3 个知识点", Icon: "🔥", Threshold: 3, SortOrder: 1},
		{ID: 2, Type: model.AchievementStreakBreaker, Tier: model.TierSilver, Name: "势如破竹", Description: "连续掌握 5 个知识点", Icon: "⚡", Threshold: 5, SortOrder: 2},
		{ID: 3, Type: model.AchievementStreakBreaker, Tier: model.TierGold, Name: "一往无前", Description: "连续掌握 10 个知识点", Icon: "🌟", Threshold: 10, SortOrder: 3},
		{ID: 4, Type: model.AchievementStreakBreaker, Tier: model.TierDiamond, Name: "学霸无双", Description: "连续掌握 20 个知识点", Icon: "💎", Threshold: 20, SortOrder: 4},
		// 深度追问勋章 — 单次会话中学生提问轮次
		{ID: 5, Type: model.AchievementDeepInquiry, Tier: model.TierBronze, Name: "好奇宝宝", Description: "单次会话中追问 5 轮", Icon: "🔍", Threshold: 5, SortOrder: 5},
		{ID: 6, Type: model.AchievementDeepInquiry, Tier: model.TierSilver, Name: "刨根问底", Description: "单次会话中追问 10 轮", Icon: "🧐", Threshold: 10, SortOrder: 6},
		{ID: 7, Type: model.AchievementDeepInquiry, Tier: model.TierGold, Name: "思维深潜", Description: "单次会话中追问 15 轮", Icon: "🧠", Threshold: 15, SortOrder: 7},
		{ID: 8, Type: model.AchievementDeepInquiry, Tier: model.TierDiamond, Name: "追问大师", Description: "单次会话中追问 20 轮", Icon: "💡", Threshold: 20, SortOrder: 8},
		// 谬误猎人 — 累计识别谬误次数
		{ID: 9, Type: model.AchievementFallacyHunt, Tier: model.TierBronze, Name: "火眼金睛", Description: "累计识别 3 个谬误", Icon: "🎯", Threshold: 3, SortOrder: 9},
		{ID: 10, Type: model.AchievementFallacyHunt, Tier: model.TierSilver, Name: "明察秋毫", Description: "累计识别 10 个谬误", Icon: "🔎", Threshold: 10, SortOrder: 10},
		{ID: 11, Type: model.AchievementFallacyHunt, Tier: model.TierGold, Name: "谬误克星", Description: "累计识别 20 个谬误", Icon: "🛡️", Threshold: 20, SortOrder: 11},
		{ID: 12, Type: model.AchievementFallacyHunt, Tier: model.TierDiamond, Name: "真理守护者", Description: "累计识别 50 个谬误", Icon: "👑", Threshold: 50, SortOrder: 12},
	}
	for _, d := range defs {
		db.FirstOrCreate(&d, model.AchievementDefinition{Type: d.Type, Tier: d.Tier})
	}
	slogDB.Info("achievement definitions seeded")
}

// seedDesigners inserts the predefined instructional designers.
func seedDesigners(db *gorm.DB) {
	designers := []model.InstructionalDesigner{
		{
			ID:                "socratic-master",
			Name:              "苏格拉底大师",
			Description:       "通过连续追问引导学生自主发现答案，避免直接给出结论",
			InterventionStyle: model.StyleQuestioning,
			IsBuiltIn:         true,
		},
		{
			ID:                "practical-coach",
			Name:              "实践教练",
			Description:       "提供实践建议和反馈，鼓励学生动手尝试和验证",
			InterventionStyle: model.StyleCoaching,
			IsBuiltIn:         true,
		},
		{
			ID:                "diagnostic-tutor",
			Name:              "诊断导师",
			Description:       "先评估学生当前水平，再针对性补足薄弱环节",
			InterventionStyle: model.StyleDiagnostic,
			IsBuiltIn:         true,
		},
		{
			ID:                "facilitator",
			Name:              "促进者",
			Description:       "鼓励学生提出假设和验证，教师作为学习促进者",
			InterventionStyle: model.StyleFacilitation,
			IsBuiltIn:         true,
		},
	}
	for _, d := range designers {
		db.FirstOrCreate(&d, model.InstructionalDesigner{ID: d.ID})
	}
	slogDB.Info("instructional designers seeded")
}

// createPerformanceIndexes creates indexes for high-frequency queries.
// These indexes are idempotent (IF NOT EXISTS).
func createPerformanceIndexes(db *gorm.DB) {
	indexes := []string{
		// student_sessions: 高频查询 activity_id + status
		`CREATE INDEX IF NOT EXISTS idx_sessions_activity_status ON student_sessions(activity_id, status)`,
		// student_sessions: 学生历史会话查询
		`CREATE INDEX IF NOT EXISTS idx_sessions_student_created ON student_sessions(student_id, created_at DESC)`,
		// interactions: 会话消息时间序列查询
		`CREATE INDEX IF NOT EXISTS idx_interactions_session_created ON interactions(session_id, created_at)`,
		// interactions: 知识点正确率分析
		`CREATE INDEX IF NOT EXISTS idx_interactions_kp_correct ON interactions(kp_id, is_correct) WHERE kp_id IS NOT NULL`,
		// student_kp_masteries: 更新时间排序（已有 idx_student_kp）
		`CREATE INDEX IF NOT EXISTS idx_mastery_updated ON student_kp_masteries(updated_at DESC)`,
		// error_notebook_entries: 学生错题查询
		`CREATE INDEX IF NOT EXISTS idx_error_student_resolved ON error_notebook_entries(student_id, resolved, archived_at DESC)`,
		// learning_activities: 课程活动列表
		`CREATE INDEX IF NOT EXISTS idx_activities_course_created ON learning_activities(course_id, created_at DESC)`,
	}

	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			slogDB.Warn("index creation skipped", "err", err)
		}
	}
	slogDB.Info("performance indexes created")
}
