package postgres

import (
	"context"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAchievementTestDB(t *testing.T) (*gorm.DB, *AchievementRepo) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.AchievementDefinition{},
		&model.StudentAchievement{},
	)
	require.NoError(t, err)

	repo := NewAchievementRepo(db)
	return db, repo
}

func TestNewAchievementRepo(t *testing.T) {
	db, repo := setupAchievementTestDB(t)
	require.NotNil(t, repo)
	require.Equal(t, db, repo.DB)
}

func TestAchievementRepo_ListDefinitions(t *testing.T) {
	db, repo := setupAchievementTestDB(t)

	// Insert definitions with different orders to verify sorting
	defs := []model.AchievementDefinition{
		{ID: 1, Type: model.AchievementStreakBreaker, Tier: model.TierBronze, Name: "A", Description: "A", Icon: "A", Threshold: 10, SortOrder: 2},
		{ID: 2, Type: model.AchievementDeepInquiry, Tier: model.TierSilver, Name: "B", Description: "B", Icon: "B", Threshold: 20, SortOrder: 1},
		{ID: 3, Type: model.AchievementFallacyHunt, Tier: model.TierGold, Name: "C", Description: "C", Icon: "C", Threshold: 30, SortOrder: 3},
	}
	for i := range defs {
		err := db.Create(&defs[i]).Error
		require.NoError(t, err)
	}

	res, err := repo.ListDefinitions(context.Background())
	require.NoError(t, err)
	require.Len(t, res, 3)
	require.Equal(t, 2, int(res[0].ID)) // SortOrder 1
	require.Equal(t, 1, int(res[1].ID)) // SortOrder 2
	require.Equal(t, 3, int(res[2].ID)) // SortOrder 3
}

func TestAchievementRepo_ListDefinitionsByType(t *testing.T) {
	db, repo := setupAchievementTestDB(t)

	defs := []model.AchievementDefinition{
		{ID: 1, Type: model.AchievementStreakBreaker, Tier: model.TierBronze, Name: "A", Description: "A", Icon: "A", Threshold: 30, SortOrder: 1},
		{ID: 2, Type: model.AchievementDeepInquiry, Tier: model.TierBronze, Name: "B", Description: "B", Icon: "B", Threshold: 10, SortOrder: 2},
		{ID: 3, Type: model.AchievementStreakBreaker, Tier: model.TierSilver, Name: "C", Description: "C", Icon: "C", Threshold: 10, SortOrder: 3},
		{ID: 4, Type: model.AchievementStreakBreaker, Tier: model.TierGold, Name: "D", Description: "D", Icon: "D", Threshold: 20, SortOrder: 4},
	}
	for i := range defs {
		err := db.Create(&defs[i]).Error
		require.NoError(t, err)
	}

	res, err := repo.ListDefinitionsByType(context.Background(), model.AchievementStreakBreaker)
	require.NoError(t, err)
	require.Len(t, res, 3)
	require.Equal(t, 3, int(res[0].ID)) // Threshold 10
	require.Equal(t, 4, int(res[1].ID)) // Threshold 20
	require.Equal(t, 1, int(res[2].ID)) // Threshold 30
}

func TestAchievementRepo_FindStudentAchievements(t *testing.T) {
	db, repo := setupAchievementTestDB(t)

	records := []model.StudentAchievement{
		{ID: 1, StudentID: 100, AchievementID: 1, Progress: 10, Unlocked: false},
		{ID: 2, StudentID: 100, AchievementID: 2, Progress: 20, Unlocked: true},
		{ID: 3, StudentID: 101, AchievementID: 1, Progress: 5, Unlocked: false},
	}
	for i := range records {
		err := db.Create(&records[i]).Error
		require.NoError(t, err)
	}

	res, err := repo.FindStudentAchievements(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, res, 2)
	require.Equal(t, 1, int(res[0].ID))
	require.Equal(t, 2, int(res[1].ID))

	resEmpty, err := repo.FindStudentAchievements(context.Background(), 999)
	require.NoError(t, err)
	require.Len(t, resEmpty, 0)
}

func TestAchievementRepo_FindStudentAchievement(t *testing.T) {
	db, repo := setupAchievementTestDB(t)

	err := db.Create(&model.StudentAchievement{
		ID:            1,
		StudentID:     100,
		AchievementID: 5,
		Progress:      50,
		Unlocked:      false,
	}).Error
	require.NoError(t, err)

	res, err := repo.FindStudentAchievement(context.Background(), 100, 5)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, 1, int(res.ID))
	require.Equal(t, 50, res.Progress)

	resNotFound, err := repo.FindStudentAchievement(context.Background(), 100, 99)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.Nil(t, resNotFound)
}

func TestAchievementRepo_CreateStudentAchievement(t *testing.T) {
	_, repo := setupAchievementTestDB(t)

	rec := &model.StudentAchievement{
		StudentID:     200,
		AchievementID: 10,
		Progress:      5,
		Unlocked:      false,
	}
	err := repo.CreateStudentAchievement(context.Background(), rec)
	require.NoError(t, err)
	require.NotZero(t, rec.ID)

	// Verify it was actually created
	fetched, err := repo.FindStudentAchievement(context.Background(), 200, 10)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, 5, fetched.Progress)
}

func TestAchievementRepo_SaveStudentAchievement(t *testing.T) {
	_, repo := setupAchievementTestDB(t)

	rec := &model.StudentAchievement{
		StudentID:     300,
		AchievementID: 20,
		Progress:      10,
		Unlocked:      false,
	}
	err := repo.CreateStudentAchievement(context.Background(), rec)
	require.NoError(t, err)

	rec.Progress = 100
	rec.Unlocked = true
	err = repo.SaveStudentAchievement(context.Background(), rec)
	require.NoError(t, err)

	fetched, err := repo.FindStudentAchievement(context.Background(), 300, 20)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, 100, fetched.Progress)
	require.True(t, fetched.Unlocked)
}
