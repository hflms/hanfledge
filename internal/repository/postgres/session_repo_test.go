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

func setupSessionTestDB(t *testing.T) (*gorm.DB, *SessionRepo) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.StudentSession{},
		&model.Interaction{},
	)
	require.NoError(t, err)

	repo := NewSessionRepo(db)
	return db, repo
}

func TestNewSessionRepo(t *testing.T) {
	db, repo := setupSessionTestDB(t)
	require.NotNil(t, repo)
	require.Equal(t, db, repo.DB)
}

func TestSessionRepo_FindByID(t *testing.T) {
	_, repo := setupSessionTestDB(t)
	ctx := context.Background()

	session := &model.StudentSession{
		StudentID:  1,
		ActivityID: 2,
		CurrentKP:  3,
	}

	err := repo.Create(ctx, session)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, session.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, session.ID, found.ID)
	require.Equal(t, session.StudentID, found.StudentID)

	notFound, err := repo.FindByID(ctx, 999)
	require.Error(t, err)
	require.Nil(t, notFound)
	require.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestSessionRepo_FindActive(t *testing.T) {
	_, repo := setupSessionTestDB(t)
	ctx := context.Background()

	session1 := &model.StudentSession{
		StudentID:  1,
		ActivityID: 2,
		CurrentKP:  3,
		IsSandbox:  false,
		Status:     model.SessionStatusCompleted,
	}
	require.NoError(t, repo.Create(ctx, session1))

	session2 := &model.StudentSession{
		StudentID:  1,
		ActivityID: 2,
		CurrentKP:  3,
		IsSandbox:  false,
		Status:     model.SessionStatusActive,
	}
	require.NoError(t, repo.Create(ctx, session2))

	session3 := &model.StudentSession{
		StudentID:  1,
		ActivityID: 2,
		CurrentKP:  3,
		IsSandbox:  true,
		Status:     model.SessionStatusActive,
	}
	require.NoError(t, repo.Create(ctx, session3))

	// Find non-sandbox active session
	found, err := repo.FindActive(ctx, 1, 2, false)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, session2.ID, found.ID)

	// Find sandbox active session
	foundSandbox, err := repo.FindActive(ctx, 1, 2, true)
	require.NoError(t, err)
	require.NotNil(t, foundSandbox)
	require.Equal(t, session3.ID, foundSandbox.ID)

	// Find active session for non-existent student/activity
	notFound, err := repo.FindActive(ctx, 999, 2, false)
	require.Error(t, err)
	require.Nil(t, notFound)
	require.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestSessionRepo_Create(t *testing.T) {
	_, repo := setupSessionTestDB(t)
	ctx := context.Background()

	session := &model.StudentSession{
		StudentID:  1,
		ActivityID: 2,
		CurrentKP:  3,
	}

	err := repo.Create(ctx, session)
	require.NoError(t, err)
	require.NotZero(t, session.ID)

	var retrieved model.StudentSession
	err = repo.DB.First(&retrieved, session.ID).Error
	require.NoError(t, err)
	require.Equal(t, uint(1), retrieved.StudentID)
	require.Equal(t, uint(2), retrieved.ActivityID)
}

func TestSessionRepo_ListByActivityID(t *testing.T) {
	_, repo := setupSessionTestDB(t)
	ctx := context.Background()

	s1 := &model.StudentSession{StudentID: 1, ActivityID: 10, CurrentKP: 1, IsSandbox: false}
	s2 := &model.StudentSession{StudentID: 2, ActivityID: 10, CurrentKP: 1, IsSandbox: false}
	s3 := &model.StudentSession{StudentID: 3, ActivityID: 10, CurrentKP: 1, IsSandbox: true}
	s4 := &model.StudentSession{StudentID: 4, ActivityID: 20, CurrentKP: 1, IsSandbox: false}

	require.NoError(t, repo.Create(ctx, s1))
	require.NoError(t, repo.Create(ctx, s2))
	require.NoError(t, repo.Create(ctx, s3))
	require.NoError(t, repo.Create(ctx, s4))

	// Find all for activity 10 (including sandbox)
	sessionsAll, err := repo.ListByActivityID(ctx, 10, false)
	require.NoError(t, err)
	require.Len(t, sessionsAll, 3)

	// Find non-sandbox for activity 10
	sessionsNonSandbox, err := repo.ListByActivityID(ctx, 10, true)
	require.NoError(t, err)
	require.Len(t, sessionsNonSandbox, 2)
	for _, s := range sessionsNonSandbox {
		require.False(t, s.IsSandbox)
	}

	// Find for activity 20
	sessions20, err := repo.ListByActivityID(ctx, 20, false)
	require.NoError(t, err)
	require.Len(t, sessions20, 1)

	// Find for non-existent activity
	sessionsEmpty, err := repo.ListByActivityID(ctx, 999, false)
	require.NoError(t, err)
	require.Len(t, sessionsEmpty, 0)
}

func TestSessionRepo_FindInteractions(t *testing.T) {
	_, repo := setupSessionTestDB(t)
	ctx := context.Background()

	i1 := &model.Interaction{SessionID: 1, Role: "student", Content: "hello 1"}
	i2 := &model.Interaction{SessionID: 1, Role: "coach", Content: "hi 1"}
	i3 := &model.Interaction{SessionID: 1, Role: "student", Content: "hello 2"}
	i4 := &model.Interaction{SessionID: 2, Role: "student", Content: "other"}

	require.NoError(t, repo.DB.Create(i1).Error)
	require.NoError(t, repo.DB.Create(i2).Error)
	require.NoError(t, repo.DB.Create(i3).Error)
	require.NoError(t, repo.DB.Create(i4).Error)

	// Fetch without limit
	interactionsAll, err := repo.FindInteractions(ctx, 1, 0)
	require.NoError(t, err)
	require.Len(t, interactionsAll, 3)

	// Fetch with limit
	interactionsLimited, err := repo.FindInteractions(ctx, 1, 2)
	require.NoError(t, err)
	require.Len(t, interactionsLimited, 2)

	// Fetch for non-existent session
	interactionsEmpty, err := repo.FindInteractions(ctx, 999, 0)
	require.NoError(t, err)
	require.Len(t, interactionsEmpty, 0)
}

func TestSessionRepo_CountStudentInteractions(t *testing.T) {
	_, repo := setupSessionTestDB(t)
	ctx := context.Background()

	i1 := &model.Interaction{SessionID: 1, Role: "student", Content: "q1"}
	i2 := &model.Interaction{SessionID: 1, Role: "coach", Content: "a1"}
	i3 := &model.Interaction{SessionID: 1, Role: "student", Content: "q2"}
	i4 := &model.Interaction{SessionID: 2, Role: "student", Content: "q3"}

	require.NoError(t, repo.DB.Create(i1).Error)
	require.NoError(t, repo.DB.Create(i2).Error)
	require.NoError(t, repo.DB.Create(i3).Error)
	require.NoError(t, repo.DB.Create(i4).Error)

	count, err := repo.CountStudentInteractions(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	count2, err := repo.CountStudentInteractions(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, int64(1), count2)

	countEmpty, err := repo.CountStudentInteractions(ctx, 999)
	require.NoError(t, err)
	require.Equal(t, int64(0), countEmpty)
}
