package postgres

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// ActivityRepo is the GORM implementation of repository.ActivityRepository.
type ActivityRepo struct {
	DB *gorm.DB
}

// NewActivityRepo creates a new ActivityRepo.
func NewActivityRepo(db *gorm.DB) *ActivityRepo {
	return &ActivityRepo{DB: db}
}

func (r *ActivityRepo) FindByID(ctx context.Context, id uint) (*model.LearningActivity, error) {
	var activity model.LearningActivity
	err := r.DB.WithContext(ctx).First(&activity, id).Error
	if err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *ActivityRepo) Create(ctx context.Context, activity *model.LearningActivity) error {
	return r.DB.WithContext(ctx).Create(activity).Error
}

func (r *ActivityRepo) CreateClassAssignment(ctx context.Context, assignment *model.ActivityClassAssignment) error {
	return r.DB.WithContext(ctx).Create(assignment).Error
}

func (r *ActivityRepo) ListByTeacher(ctx context.Context, teacherID uint, courseID uint, status string, offset, limit int) ([]model.LearningActivity, int64, error) {
	query := r.DB.WithContext(ctx).Where("teacher_id = ?", teacherID)

	if courseID > 0 {
		query = query.Where("course_id = ?", courseID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Model(&model.LearningActivity{}).Count(&total)

	var activities []model.LearningActivity
	err := query.Preload("AssignedClasses").
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&activities).Error
	return activities, total, err
}

func (r *ActivityRepo) UpdateFields(ctx context.Context, activityID uint, fields map[string]interface{}) error {
	return r.DB.WithContext(ctx).Model(&model.LearningActivity{}).
		Where("id = ?", activityID).
		Updates(fields).Error
}

func (r *ActivityRepo) ListPublishedForClasses(ctx context.Context, classIDs []uint) ([]model.LearningActivity, error) {
	if len(classIDs) == 0 {
		return nil, nil
	}
	var activities []model.LearningActivity
	err := r.DB.WithContext(ctx).
		Where("status = ?", model.ActivityStatusPublished).
		Where("id IN (SELECT activity_id FROM activity_class_assignments WHERE class_id IN ?)", classIDs).
		Order("created_at DESC").
		Find(&activities).Error
	return activities, err
}
