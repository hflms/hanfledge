package postgres

import (
	"fmt"
	"log"

	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewConnection creates a new GORM database connection to PostgreSQL.
func NewConnection(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	logLevel := logger.Info
	if cfg.Host != "localhost" {
		logLevel = logger.Warn
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	log.Println("✅ PostgreSQL connected successfully")
	return db, nil
}

// AutoMigrate runs GORM auto-migration for all domain models.
// This creates or updates tables to match the Go struct definitions.
func AutoMigrate(db *gorm.DB) error {
	log.Println("🔄 Running database auto-migration...")

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
			log.Printf("⚠️  Timestamp fix query skipped (table may not exist yet): %v", err)
		}
	}

	err := db.AutoMigrate(
		// 用户与权限
		&model.User{},
		&model.School{},
		&model.Class{},
		&model.Role{},
		&model.UserSchoolRole{},
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
	)
	if err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	// Seed default roles if not exist
	seedRoles(db)
	// Seed achievement definitions if not exist
	seedAchievements(db)

	log.Println("✅ Database migration completed")
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
	log.Println("✅ Default roles seeded")
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
	log.Println("✅ Achievement definitions seeded")
}
