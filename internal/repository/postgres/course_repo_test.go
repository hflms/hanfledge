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

func setupCourseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.Course{},
		&model.Chapter{},
		&model.KnowledgePoint{},
		&model.KPSkillMount{},
		&model.User{},
		&model.School{},
	)
	require.NoError(t, err)

	return db
}

func TestCourseRepo_FindByID(t *testing.T) {
	db := setupCourseTestDB(t)
	repo := NewCourseRepo(db)

	course := &model.Course{
		Title:   "Test Course",
		Subject: "Math",
	}
	err := db.Create(course).Error
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByID(ctx, course.ID)
		assert.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, course.Title, found.Title)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999)
		assert.Error(t, err)
		assert.Nil(t, found)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})
}

func TestCourseRepo_FindWithOutline(t *testing.T) {
	db := setupCourseTestDB(t)
	repo := NewCourseRepo(db)

	course := &model.Course{
		Title:   "Course With Outline",
		Subject: "Science",
		Chapters: []model.Chapter{
			{
				Title:     "Chapter 2",
				SortOrder: 2,
				KnowledgePoints: []model.KnowledgePoint{
					{
						Title:       "KP 2.1",
						Neo4jNodeID: "kp21",
						MountedSkills: []model.KPSkillMount{
							{SkillID: "skill-c"},
						},
					},
				},
			},
			{
				Title:     "Chapter 1",
				SortOrder: 1,
				KnowledgePoints: []model.KnowledgePoint{
					{
						Title:       "KP 1.1",
						Neo4jNodeID: "kp11",
						MountedSkills: []model.KPSkillMount{
							{SkillID: "skill-a"},
							{SkillID: "skill-b"},
						},
					},
				},
			},
		},
	}
	err := db.Create(course).Error
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("found with outline preloaded", func(t *testing.T) {
		found, err := repo.FindWithOutline(ctx, course.ID)
		assert.NoError(t, err)
		assert.NotNil(t, found)

		require.Len(t, found.Chapters, 2)
		// Should be ordered by SortOrder ASC
		assert.Equal(t, "Chapter 1", found.Chapters[0].Title)
		assert.Equal(t, "Chapter 2", found.Chapters[1].Title)

		require.Len(t, found.Chapters[0].KnowledgePoints, 1)
		assert.Equal(t, "KP 1.1", found.Chapters[0].KnowledgePoints[0].Title)

		require.Len(t, found.Chapters[0].KnowledgePoints[0].MountedSkills, 2)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.FindWithOutline(ctx, 999)
		assert.Error(t, err)
		assert.Nil(t, found)
	})
}

func TestCourseRepo_FindWithChaptersAndKPs(t *testing.T) {
	db := setupCourseTestDB(t)
	repo := NewCourseRepo(db)

	course := &model.Course{
		Title:   "Course Without Skills",
		Subject: "Physics",
		Chapters: []model.Chapter{
			{
				Title:     "Chapter B",
				SortOrder: 2,
				KnowledgePoints: []model.KnowledgePoint{
					{Title: "KP B.1", Neo4jNodeID: "kpb1"},
				},
			},
			{
				Title:     "Chapter A",
				SortOrder: 1,
				KnowledgePoints: []model.KnowledgePoint{
					{Title: "KP A.1", Neo4jNodeID: "kpa1"},
				},
			},
		},
	}
	err := db.Create(course).Error
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("found with chapters and kps", func(t *testing.T) {
		found, err := repo.FindWithChaptersAndKPs(ctx, course.ID)
		assert.NoError(t, err)
		assert.NotNil(t, found)

		require.Len(t, found.Chapters, 2)
		assert.Equal(t, "Chapter A", found.Chapters[0].Title)
		assert.Equal(t, "Chapter B", found.Chapters[1].Title)

		require.Len(t, found.Chapters[0].KnowledgePoints, 1)
		assert.Equal(t, "KP A.1", found.Chapters[0].KnowledgePoints[0].Title)

		// Ensure MountedSkills are not loaded by default (nil or empty)
		assert.Len(t, found.Chapters[0].KnowledgePoints[0].MountedSkills, 0)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.FindWithChaptersAndKPs(ctx, 999)
		assert.Error(t, err)
		assert.Nil(t, found)
	})
}

func TestCourseRepo_ListByTeacher(t *testing.T) {
	db := setupCourseTestDB(t)
	repo := NewCourseRepo(db)

	teacher1ID := uint(100)
	teacher2ID := uint(200)
	school1ID := uint(10)
	school2ID := uint(20)

	courses := []model.Course{
		{TeacherID: teacher1ID, SchoolID: school1ID, Title: "T1 S1 C1", Subject: "Math"},
		{TeacherID: teacher1ID, SchoolID: school1ID, Title: "T1 S1 C2", Subject: "Math"},
		{TeacherID: teacher1ID, SchoolID: school2ID, Title: "T1 S2 C1", Subject: "Math"},
		{TeacherID: teacher2ID, SchoolID: school1ID, Title: "T2 S1 C1", Subject: "Math"},
	}
	for _, c := range courses {
		err := db.Create(&c).Error
		require.NoError(t, err)
	}

	ctx := context.Background()

	t.Run("list all for teacher 1, specific school", func(t *testing.T) {
		result, total, err := repo.ListByTeacher(ctx, teacher1ID, school1ID, 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		require.Len(t, result, 2)
		assert.Equal(t, "T1 S1 C1", result[0].Title)
	})

	t.Run("list all for teacher 1, no school filter", func(t *testing.T) {
		// schoolID = 0 means no school filter
		result, total, err := repo.ListByTeacher(ctx, teacher1ID, 0, 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), total)
		require.Len(t, result, 3)
	})

	t.Run("list for teacher 1 with pagination", func(t *testing.T) {
		result, total, err := repo.ListByTeacher(ctx, teacher1ID, 0, 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), total)
		require.Len(t, result, 1)
		assert.Equal(t, "T1 S1 C2", result[0].Title)
	})

	t.Run("list for teacher 2", func(t *testing.T) {
		result, total, err := repo.ListByTeacher(ctx, teacher2ID, 0, 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
		require.Len(t, result, 1)
		assert.Equal(t, "T2 S1 C1", result[0].Title)
	})

	t.Run("list for non-existent teacher", func(t *testing.T) {
		result, total, err := repo.ListByTeacher(ctx, 999, 0, 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
		require.Len(t, result, 0)
	})
}

func TestCourseRepo_Create(t *testing.T) {
	db := setupCourseTestDB(t)
	repo := NewCourseRepo(db)

	ctx := context.Background()

	t.Run("create success", func(t *testing.T) {
		course := &model.Course{
			Title:      "New Course",
			Subject:    "English",
			TeacherID:  1,
			SchoolID:   1,
			GradeLevel: 10,
		}

		err := repo.Create(ctx, course)
		assert.NoError(t, err)
		assert.NotZero(t, course.ID)

		// Verify in DB
		var found model.Course
		err = db.First(&found, course.ID).Error
		require.NoError(t, err)
		assert.Equal(t, "New Course", found.Title)
		assert.Equal(t, "English", found.Subject)
	})

	t.Run("create with constraint violation", func(t *testing.T) {
		// SQLite testing "NOT NULL" using Gorm. SQLite default behaviors for nulls vary.
		// A known error is primary key uniqueness.
		course1 := &model.Course{
			ID:         100,
			Title:      "Duplicate",
			Subject:    "Art",
			TeacherID:  1,
			SchoolID:   1,
			GradeLevel: 5,
		}
		err := repo.Create(ctx, course1)
		assert.NoError(t, err)

		course2 := &model.Course{
			ID:         100, // same ID
			Title:      "Duplicate 2",
			Subject:    "Art",
			TeacherID:  1,
			SchoolID:   1,
			GradeLevel: 5,
		}
		err = repo.Create(ctx, course2)
		assert.Error(t, err)
	})
}
