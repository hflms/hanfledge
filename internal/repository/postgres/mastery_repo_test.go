package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
)

func TestMasteryRepo_FindByStudent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	now := time.Now()
	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 10, MasteryScore: 0.8, UpdatedAt: now})
	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 20, MasteryScore: 0.9, UpdatedAt: now.Add(-1 * time.Hour)})
	db.Create(&model.StudentKPMastery{StudentID: 2, KPID: 10, MasteryScore: 0.5, UpdatedAt: now})

	t.Run("Find without KP filtering", func(t *testing.T) {
		res, err := repo.FindByStudent(ctx, 1, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res))
		}
		if res[0].KPID != 10 {
			t.Errorf("expected KPID 10 first, got %d", res[0].KPID)
		}
	})

	t.Run("Find with KP filtering", func(t *testing.T) {
		res, err := repo.FindByStudent(ctx, 1, []uint{20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].KPID != 20 {
			t.Errorf("expected KPID 20, got %d", res[0].KPID)
		}
	})
}

func TestMasteryRepo_FindByStudentsAndKPs(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 10})
	db.Create(&model.StudentKPMastery{StudentID: 2, KPID: 10})

	t.Run("Find valid", func(t *testing.T) {
		res, err := repo.FindByStudentsAndKPs(ctx, []uint{1, 2}, []uint{10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res))
		}
	})

	t.Run("Empty student list", func(t *testing.T) {
		res, err := repo.FindByStudentsAndKPs(ctx, nil, []uint{10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != nil {
			t.Errorf("expected nil result")
		}
	})

	t.Run("Empty kp list", func(t *testing.T) {
		res, err := repo.FindByStudentsAndKPs(ctx, []uint{1}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != nil {
			t.Errorf("expected nil result")
		}
	})
}

func TestMasteryRepo_FindByKPIDs(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 10})
	db.Create(&model.StudentKPMastery{StudentID: 2, KPID: 20})

	t.Run("Find valid", func(t *testing.T) {
		res, err := repo.FindByKPIDs(ctx, []uint{10, 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res))
		}
	})

	t.Run("Empty kp list", func(t *testing.T) {
		res, err := repo.FindByKPIDs(ctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != nil {
			t.Errorf("expected nil result")
		}
	})
}

func TestMasteryRepo_AggregateAvgByKP(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 10, MasteryScore: 0.8})
	db.Create(&model.StudentKPMastery{StudentID: 2, KPID: 10, MasteryScore: 0.6})
	db.Create(&model.StudentKPMastery{StudentID: 3, KPID: 10, MasteryScore: 0.4})

	t.Run("All students", func(t *testing.T) {
		avg, count, err := repo.AggregateAvgByKP(ctx, 10, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 3 {
			t.Errorf("expected count 3, got %d", count)
		}
		// (0.8 + 0.6 + 0.4) / 3 = 0.6
		if avg < 0.59 || avg > 0.61 {
			t.Errorf("expected avg ~0.6, got %f", avg)
		}
	})

	t.Run("Filtered students", func(t *testing.T) {
		avg, count, err := repo.AggregateAvgByKP(ctx, 10, []uint{1, 2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}
		// (0.8 + 0.6) / 2 = 0.7
		if avg < 0.69 || avg > 0.71 {
			t.Errorf("expected avg ~0.7, got %f", avg)
		}
	})
}

func TestMasteryRepo_CountDistinctStudents(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 10})
	db.Create(&model.StudentKPMastery{StudentID: 2, KPID: 10})
	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 20})

	t.Run("Count all", func(t *testing.T) {
		count, err := repo.CountDistinctStudents(ctx, []uint{10, 20}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}
	})

	t.Run("Filtered students", func(t *testing.T) {
		count, err := repo.CountDistinctStudents(ctx, []uint{10, 20}, []uint{1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("expected count 1, got %d", count)
		}
	})

	t.Run("Empty KP list", func(t *testing.T) {
		count, err := repo.CountDistinctStudents(ctx, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0, got %d", count)
		}
	})
}

func TestMasteryRepo_AggregateDailyMastery(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	// SQLite grouping with DATE() can be tricky because time is stored as string in memory sqlite without driver tweaking
	// StudentKPMastery has a unique constraint on (StudentID, KPID). We can't insert the same KPID twice for the same student.
	// So we use different KPIDs.
	db.Exec("INSERT INTO student_kp_masteries (student_id, kp_id, mastery_score, attempt_count, updated_at) VALUES (?, ?, ?, ?, ?)", 1, 10, 0.8, 2, "2023-10-01 10:00:00")
	db.Exec("INSERT INTO student_kp_masteries (student_id, kp_id, mastery_score, attempt_count, updated_at) VALUES (?, ?, ?, ?, ?)", 1, 20, 0.6, 1, "2023-10-01 12:00:00")
	db.Exec("INSERT INTO student_kp_masteries (student_id, kp_id, mastery_score, attempt_count, updated_at) VALUES (?, ?, ?, ?, ?)", 1, 30, 0.9, 3, "2023-10-02 10:00:00")

	t.Run("All KPs", func(t *testing.T) {
		res, err := repo.AggregateDailyMastery(ctx, 1, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 days, got %d. Result: %+v", len(res), res)
		}
		// SQLite date formatting might be different than expected or it might group strangely.
		// For SQLite DATE() returns 'YYYY-MM-DD' or similar strings.
		if res[0].Date != "2023-10-01" && res[0].Date != "2023-10-01T00:00:00Z" {
			t.Errorf("unexpected date string: %s", res[0].Date)
		}
		if res[0].AvgScore < 0.69 || res[0].AvgScore > 0.71 {
			t.Errorf("expected avg score 0.7, got %f", res[0].AvgScore)
		}
		if res[0].Count != 3 {
			t.Errorf("expected count 3, got %d", res[0].Count)
		}
	})

	t.Run("Filtered KPs", func(t *testing.T) {
		res, err := repo.AggregateDailyMastery(ctx, 1, []uint{10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 day, got %d", len(res))
		}
		if res[0].AvgScore < 0.79 || res[0].AvgScore > 0.81 {
			t.Errorf("expected avg score 0.8, got %f", res[0].AvgScore)
		}
	})
}

func TestMasteryRepo_FindMastered(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 10, MasteryScore: 0.8})
	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 20, MasteryScore: 0.9})
	db.Create(&model.StudentKPMastery{StudentID: 1, KPID: 30, MasteryScore: 0.5})

	res, err := repo.FindMastered(ctx, 1, 0.8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
}

func TestMasteryRepo_ListErrorNotebook(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	resolvedTrue := true
	resolvedFalse := false

	now := time.Now()

	db.Create(&model.ErrorNotebookEntry{StudentID: 1, KPID: 10, Resolved: true, ArchivedAt: now})
	db.Create(&model.ErrorNotebookEntry{StudentID: 1, KPID: 20, Resolved: false, ArchivedAt: now.Add(-time.Hour)})
	db.Create(&model.ErrorNotebookEntry{StudentID: 1, KPID: 10, Resolved: false, ArchivedAt: now.Add(-2 * time.Hour)})
	db.Create(&model.ErrorNotebookEntry{StudentID: 2, KPID: 10, Resolved: false, ArchivedAt: now})

	t.Run("All entries for student", func(t *testing.T) {
		res, err := repo.ListErrorNotebook(ctx, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 3 {
			t.Fatalf("expected 3 results, got %d", len(res))
		}
	})

	t.Run("Resolved entries", func(t *testing.T) {
		res, err := repo.ListErrorNotebook(ctx, 1, &resolvedTrue, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
	})

	t.Run("Unresolved entries", func(t *testing.T) {
		res, err := repo.ListErrorNotebook(ctx, 1, &resolvedFalse, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res))
		}
	})

	t.Run("Filter by KPID", func(t *testing.T) {
		res, err := repo.ListErrorNotebook(ctx, 1, nil, 20)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
	})
}

func TestMasteryRepo_CountErrorNotebook(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	db.Create(&model.ErrorNotebookEntry{StudentID: 1, Resolved: true})
	db.Create(&model.ErrorNotebookEntry{StudentID: 1, Resolved: false})
	db.Create(&model.ErrorNotebookEntry{StudentID: 1, Resolved: false})

	total, unresolved, err := repo.CountErrorNotebook(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if unresolved != 2 {
		t.Errorf("expected unresolved 2, got %d", unresolved)
	}
}

func TestMasteryRepo_FindErrorNotebookByKPIDs(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMasteryRepo(db)
	ctx := context.Background()

	db.Create(&model.ErrorNotebookEntry{StudentID: 1, KPID: 10})
	db.Create(&model.ErrorNotebookEntry{StudentID: 2, KPID: 20})
	db.Create(&model.ErrorNotebookEntry{StudentID: 3, KPID: 30})

	t.Run("Valid KPIDs", func(t *testing.T) {
		res, err := repo.FindErrorNotebookByKPIDs(ctx, []uint{10, 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res))
		}
	})

	t.Run("Empty KPIDs", func(t *testing.T) {
		res, err := repo.FindErrorNotebookByKPIDs(ctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != nil {
			t.Errorf("expected nil result")
		}
	})
}
