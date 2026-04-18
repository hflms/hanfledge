package postgres

import (
	"testing"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database and auto-migrates the
// tables needed for repository tests.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}

	// Migrate only the tables used by handlers under test.
	err = db.AutoMigrate(
		&model.StudentKPMastery{},
		&model.ErrorNotebookEntry{},
	)
	if err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}

	return db
}
