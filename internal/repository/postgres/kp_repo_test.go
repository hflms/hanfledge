package postgres

import (
	"context"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database and auto-migrates the
// tables needed for tests.
func setupTestDB_1(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}

	err = db.AutoMigrate(
		&model.Course{},
		&model.Chapter{},
		&model.KnowledgePoint{},
		&model.Misconception{},
		&model.CrossLink{},
		&model.StudentKPMastery{},
		&model.ErrorNotebookEntry{},
	)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	return db
}

func TestKnowledgePointRepo_FindByID(t *testing.T) {
	db := setupTestDB_1(t)
	repo := NewKnowledgePointRepo(db)
	ctx := context.Background()

	kp := model.KnowledgePoint{
		ChapterID: 1,
		Title:     "Test KP",
	}
	require.NoError(t, db.Create(&kp).Error)

	t.Run("success", func(t *testing.T) {
		found, err := repo.FindByID(ctx, kp.ID)
		require.NoError(t, err)
		assert.Equal(t, kp.ID, found.ID)
		assert.Equal(t, "Test KP", found.Title)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999)
		assert.Error(t, err)
		assert.Nil(t, found)
	})
}

func TestKnowledgePointRepo_FindByCourseID(t *testing.T) {
	db := setupTestDB_1(t)
	repo := NewKnowledgePointRepo(db)
	ctx := context.Background()

	// Add course and chapters
	course := model.Course{Title: "Physics 101"}
	require.NoError(t, db.Create(&course).Error)

	chapter1 := model.Chapter{CourseID: course.ID, Title: "Chapter 1", SortOrder: 1}
	chapter2 := model.Chapter{CourseID: course.ID, Title: "Chapter 2", SortOrder: 2}
	require.NoError(t, db.Create(&chapter1).Error)
	require.NoError(t, db.Create(&chapter2).Error)

	// Add KPs
	kp1 := model.KnowledgePoint{ChapterID: chapter1.ID, Title: "KP 1"}
	kp2 := model.KnowledgePoint{ChapterID: chapter2.ID, Title: "KP 2"}
	kp3 := model.KnowledgePoint{ChapterID: chapter1.ID, Title: "KP 3"}
	require.NoError(t, db.Create(&kp1).Error)
	require.NoError(t, db.Create(&kp2).Error)
	require.NoError(t, db.Create(&kp3).Error)

	t.Run("success", func(t *testing.T) {
		kps, err := repo.FindByCourseID(ctx, course.ID)
		require.NoError(t, err)
		assert.Len(t, kps, 3)
		// Should be ordered by chapter sort_order, then kp id
		assert.Equal(t, kp1.ID, kps[0].ID)
		assert.Equal(t, kp3.ID, kps[1].ID)
		assert.Equal(t, kp2.ID, kps[2].ID)
	})

	t.Run("not found returns empty", func(t *testing.T) {
		kps, err := repo.FindByCourseID(ctx, 999)
		require.NoError(t, err)
		assert.Empty(t, kps)
	})
}

func TestKnowledgePointRepo_FindIDsByCourseID(t *testing.T) {
	db := setupTestDB_1(t)
	repo := NewKnowledgePointRepo(db)
	ctx := context.Background()

	course := model.Course{Title: "Math 101"}
	require.NoError(t, db.Create(&course).Error)

	chapter := model.Chapter{CourseID: course.ID, Title: "Ch 1"}
	require.NoError(t, db.Create(&chapter).Error)

	kp1 := model.KnowledgePoint{ChapterID: chapter.ID, Title: "KP 1"}
	kp2 := model.KnowledgePoint{ChapterID: chapter.ID, Title: "KP 2"}
	require.NoError(t, db.Create(&kp1).Error)
	require.NoError(t, db.Create(&kp2).Error)

	t.Run("success", func(t *testing.T) {
		ids, err := repo.FindIDsByCourseID(ctx, course.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uint{kp1.ID, kp2.ID}, ids)
	})
}

func TestKnowledgePointRepo_FindByIDs(t *testing.T) {
	db := setupTestDB_1(t)
	repo := NewKnowledgePointRepo(db)
	ctx := context.Background()

	kp1 := model.KnowledgePoint{ChapterID: 1, Title: "KP 1"}
	kp2 := model.KnowledgePoint{ChapterID: 1, Title: "KP 2"}
	require.NoError(t, db.Create(&kp1).Error)
	require.NoError(t, db.Create(&kp2).Error)

	t.Run("success", func(t *testing.T) {
		kps, err := repo.FindByIDs(ctx, []uint{kp1.ID, kp2.ID})
		require.NoError(t, err)
		assert.Len(t, kps, 2)
	})

	t.Run("empty ids list", func(t *testing.T) {
		kps, err := repo.FindByIDs(ctx, []uint{})
		require.NoError(t, err)
		assert.Empty(t, kps)
	})
}

func TestKnowledgePointRepo_FindByIDsWithChapter(t *testing.T) {
	db := setupTestDB_1(t)
	repo := NewKnowledgePointRepo(db)
	ctx := context.Background()

	course := model.Course{Title: "History"}
	require.NoError(t, db.Create(&course).Error)

	chapter := model.Chapter{CourseID: course.ID, Title: "Ch 1"}
	require.NoError(t, db.Create(&chapter).Error)

	kp := model.KnowledgePoint{ChapterID: chapter.ID, Title: "KP 1"}
	require.NoError(t, db.Create(&kp).Error)

	t.Run("success", func(t *testing.T) {
		kps, err := repo.FindByIDsWithChapter(ctx, []uint{kp.ID})
		require.NoError(t, err)
		require.Len(t, kps, 1)
		assert.Equal(t, chapter.Title, kps[0].Chapter.Title)
	})

	t.Run("empty list", func(t *testing.T) {
		kps, err := repo.FindByIDsWithChapter(ctx, []uint{})
		require.NoError(t, err)
		assert.Nil(t, kps)
	})
}

func TestKnowledgePointRepo_FindWithChapterTitles(t *testing.T) {
	db := setupTestDB_1(t)
	repo := NewKnowledgePointRepo(db)
	ctx := context.Background()

	course := model.Course{Title: "Biology"}
	require.NoError(t, db.Create(&course).Error)

	chapter := model.Chapter{CourseID: course.ID, Title: "Cells"}
	require.NoError(t, db.Create(&chapter).Error)

	kp := model.KnowledgePoint{ChapterID: chapter.ID, Title: "Mitochondria"}
	require.NoError(t, db.Create(&kp).Error)

	t.Run("success", func(t *testing.T) {
		results, err := repo.FindWithChapterTitles(ctx, []uint{kp.ID})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, kp.ID, results[0].ID)
		assert.Equal(t, "Mitochondria", results[0].Title)
		assert.Equal(t, "Cells", results[0].ChapterTitle)
	})

	t.Run("empty list", func(t *testing.T) {
		results, err := repo.FindWithChapterTitles(ctx, []uint{})
		require.NoError(t, err)
		assert.Nil(t, results)
	})
}

func TestKnowledgePointRepo_Misconceptions(t *testing.T) {
	db := setupTestDB_1(t)
	repo := NewKnowledgePointRepo(db)
	ctx := context.Background()

	kp := model.KnowledgePoint{ChapterID: 1, Title: "Gravity"}
	require.NoError(t, db.Create(&kp).Error)

	m := model.Misconception{
		KPID:        kp.ID,
		Description: "Heavy objects fall faster",
		Severity:    0.9,
	}

	t.Run("CreateMisconception", func(t *testing.T) {
		err := repo.CreateMisconception(ctx, &m)
		require.NoError(t, err)
		assert.NotZero(t, m.ID)
	})

	t.Run("UpdateMisconceptionNeo4jID", func(t *testing.T) {
		err := repo.UpdateMisconceptionNeo4jID(ctx, m.ID, "neo123")
		require.NoError(t, err)

		var updated model.Misconception
		db.First(&updated, m.ID)
		assert.Equal(t, "neo123", updated.Neo4jNodeID)
	})

	t.Run("ListMisconceptionsByKPID", func(t *testing.T) {
		m2 := model.Misconception{
			KPID:        kp.ID,
			Description: "No gravity in space",
			Severity:    0.5,
		}
		require.NoError(t, db.Create(&m2).Error)

		list, err := repo.ListMisconceptionsByKPID(ctx, kp.ID)
		require.NoError(t, err)
		require.Len(t, list, 2)
		// Should be ordered by severity DESC
		assert.Equal(t, m.ID, list[0].ID)
		assert.Equal(t, m2.ID, list[1].ID)
	})

	t.Run("FindMisconceptionByIDAndKPID", func(t *testing.T) {
		found, err := repo.FindMisconceptionByIDAndKPID(ctx, m.ID, kp.ID)
		require.NoError(t, err)
		assert.Equal(t, m.ID, found.ID)

		// Not found
		_, err = repo.FindMisconceptionByIDAndKPID(ctx, 999, kp.ID)
		assert.Error(t, err)
	})

	t.Run("DeleteMisconception", func(t *testing.T) {
		err := repo.DeleteMisconception(ctx, &m)
		require.NoError(t, err)

		_, err = repo.FindMisconceptionByIDAndKPID(ctx, m.ID, kp.ID)
		assert.Error(t, err) // Should be not found now
	})
}

func TestKnowledgePointRepo_CrossLinks(t *testing.T) {
	db := setupTestDB_1(t)
	repo := NewKnowledgePointRepo(db)
	ctx := context.Background()

	kp1 := model.KnowledgePoint{ChapterID: 1, Title: "Electricity"}
	kp2 := model.KnowledgePoint{ChapterID: 1, Title: "Water flow"}
	require.NoError(t, db.Create(&kp1).Error)
	require.NoError(t, db.Create(&kp2).Error)

	link := model.CrossLink{
		FromKPID: kp1.ID,
		ToKPID:   kp2.ID,
		LinkType: "analogy",
	}

	t.Run("CreateCrossLink", func(t *testing.T) {
		err := repo.CreateCrossLink(ctx, &link)
		require.NoError(t, err)
		assert.NotZero(t, link.ID)
	})

	t.Run("ListCrossLinksByKPID", func(t *testing.T) {
		links, err := repo.ListCrossLinksByKPID(ctx, kp1.ID)
		require.NoError(t, err)
		require.Len(t, links, 1)
		assert.Equal(t, link.ID, links[0].ID)

		// Also findable by ToKPID
		links2, err := repo.ListCrossLinksByKPID(ctx, kp2.ID)
		require.NoError(t, err)
		require.Len(t, links2, 1)
		assert.Equal(t, link.ID, links2[0].ID)
	})

	t.Run("FindCrossLinkByIDAndKPID", func(t *testing.T) {
		found, err := repo.FindCrossLinkByIDAndKPID(ctx, link.ID, kp1.ID)
		require.NoError(t, err)
		assert.Equal(t, link.ID, found.ID)

		// Not found
		_, err = repo.FindCrossLinkByIDAndKPID(ctx, 999, kp1.ID)
		assert.Error(t, err)
	})

	t.Run("DeleteCrossLink", func(t *testing.T) {
		err := repo.DeleteCrossLink(ctx, &link)
		require.NoError(t, err)

		_, err = repo.FindCrossLinkByIDAndKPID(ctx, link.ID, kp1.ID)
		assert.Error(t, err) // Should be not found now
	})
}
