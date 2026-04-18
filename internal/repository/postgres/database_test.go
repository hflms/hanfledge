package postgres

import (
	"testing"

	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestNewConnection_Error(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "invalid_host_that_does_not_exist",
		Port:     "5432",
		User:     "user",
		Password: "password",
		DBName:   "db",
		SSLMode:  "disable",
	}

	db, err := NewConnection(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "failed to connect to PostgreSQL")
}

func TestAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = AutoMigrate(db)
	assert.NoError(t, err)

	// Verify roles seeded
	var roleCount int64
	db.Model(&model.Role{}).Count(&roleCount)
	assert.Equal(t, int64(4), roleCount)

	// Verify achievements seeded
	var achievementCount int64
	db.Model(&model.AchievementDefinition{}).Count(&achievementCount)
	assert.Equal(t, int64(12), achievementCount)

	// Verify designers seeded
	var designerCount int64
	db.Model(&model.InstructionalDesigner{}).Count(&designerCount)
	assert.Equal(t, int64(4), designerCount)
}
