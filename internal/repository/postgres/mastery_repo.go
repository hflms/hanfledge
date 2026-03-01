package postgres

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/repository"
	"gorm.io/gorm"
)

// MasteryRepo is the GORM implementation of repository.MasteryRepository.
type MasteryRepo struct {
	DB *gorm.DB
}

// NewMasteryRepo creates a new MasteryRepo.
func NewMasteryRepo(db *gorm.DB) *MasteryRepo {
	return &MasteryRepo{DB: db}
}

func (r *MasteryRepo) FindByStudent(ctx context.Context, studentID uint, kpIDs []uint) ([]model.StudentKPMastery, error) {
	query := r.DB.WithContext(ctx).
		Where("student_id = ?", studentID)
	if len(kpIDs) > 0 {
		query = query.Where("kp_id IN ?", kpIDs)
	}
	var masteries []model.StudentKPMastery
	err := query.Order("updated_at DESC").Find(&masteries).Error
	return masteries, err
}

func (r *MasteryRepo) FindByStudentsAndKPs(ctx context.Context, studentIDs, kpIDs []uint) ([]model.StudentKPMastery, error) {
	if len(studentIDs) == 0 || len(kpIDs) == 0 {
		return nil, nil
	}
	var masteries []model.StudentKPMastery
	err := r.DB.WithContext(ctx).
		Where("student_id IN ? AND kp_id IN ?", studentIDs, kpIDs).
		Find(&masteries).Error
	return masteries, err
}

func (r *MasteryRepo) FindByKPIDs(ctx context.Context, kpIDs []uint) ([]model.StudentKPMastery, error) {
	if len(kpIDs) == 0 {
		return nil, nil
	}
	var masteries []model.StudentKPMastery
	err := r.DB.WithContext(ctx).Where("kp_id IN ?", kpIDs).Find(&masteries).Error
	return masteries, err
}

func (r *MasteryRepo) AggregateAvgByKP(ctx context.Context, kpID uint, studentIDs []uint) (float64, int64, error) {
	var result struct {
		Avg   float64
		Count int64
	}
	query := r.DB.WithContext(ctx).Model(&model.StudentKPMastery{}).
		Select("COALESCE(AVG(mastery_score), 0.0) as avg, COUNT(*) as count").
		Where("kp_id = ?", kpID)
	if len(studentIDs) > 0 {
		query = query.Where("student_id IN ?", studentIDs)
	}
	err := query.Scan(&result).Error
	return result.Avg, result.Count, err
}

func (r *MasteryRepo) CountDistinctStudents(ctx context.Context, kpIDs []uint, studentIDs []uint) (int64, error) {
	if len(kpIDs) == 0 {
		return 0, nil
	}
	var count int64
	query := r.DB.WithContext(ctx).Model(&model.StudentKPMastery{}).
		Where("kp_id IN ?", kpIDs)
	if len(studentIDs) > 0 {
		query = query.Where("student_id IN ?", studentIDs)
	}
	err := query.Distinct("student_id").Count(&count).Error
	return count, err
}

func (r *MasteryRepo) AggregateDailyMastery(ctx context.Context, studentID uint, kpIDs []uint) ([]repository.DailyMasteryAgg, error) {
	query := r.DB.WithContext(ctx).Model(&model.StudentKPMastery{}).
		Select("DATE(updated_at) as date, AVG(mastery_score) as avg_score, SUM(attempt_count) as count").
		Where("student_id = ?", studentID).
		Group("DATE(updated_at)").
		Order("date ASC")
	if len(kpIDs) > 0 {
		query = query.Where("kp_id IN ?", kpIDs)
	}
	var results []repository.DailyMasteryAgg
	err := query.Scan(&results).Error
	return results, err
}

func (r *MasteryRepo) FindMastered(ctx context.Context, studentID uint, threshold float64) ([]model.StudentKPMastery, error) {
	var masteries []model.StudentKPMastery
	err := r.DB.WithContext(ctx).
		Where("student_id = ? AND mastery_score >= ?", studentID, threshold).
		Order("updated_at DESC").
		Find(&masteries).Error
	return masteries, err
}

func (r *MasteryRepo) ListErrorNotebook(ctx context.Context, studentID uint, resolved *bool, kpID uint) ([]model.ErrorNotebookEntry, error) {
	query := r.DB.WithContext(ctx).Where("student_id = ?", studentID)
	if resolved != nil {
		query = query.Where("resolved = ?", *resolved)
	}
	if kpID > 0 {
		query = query.Where("kp_id = ?", kpID)
	}
	var entries []model.ErrorNotebookEntry
	err := query.Order("archived_at DESC").Find(&entries).Error
	return entries, err
}

func (r *MasteryRepo) CountErrorNotebook(ctx context.Context, studentID uint) (int64, int64, error) {
	var total, unresolved int64
	if err := r.DB.WithContext(ctx).Model(&model.ErrorNotebookEntry{}).
		Where("student_id = ?", studentID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	if err := r.DB.WithContext(ctx).Model(&model.ErrorNotebookEntry{}).
		Where("student_id = ? AND resolved = ?", studentID, false).
		Count(&unresolved).Error; err != nil {
		return 0, 0, err
	}
	return total, unresolved, nil
}

func (r *MasteryRepo) FindErrorNotebookByKPIDs(ctx context.Context, kpIDs []uint) ([]model.ErrorNotebookEntry, error) {
	if len(kpIDs) == 0 {
		return nil, nil
	}
	var entries []model.ErrorNotebookEntry
	err := r.DB.WithContext(ctx).Where("kp_id IN ?", kpIDs).
		Order("archived_at DESC").Find(&entries).Error
	return entries, err
}
