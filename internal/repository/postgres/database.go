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
	)
	if err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	// Seed default roles if not exist
	seedRoles(db)

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
