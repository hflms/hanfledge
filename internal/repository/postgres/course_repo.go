package postgres

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// CourseRepo is the GORM implementation of repository.CourseRepository.
type CourseRepo struct {
	DB *gorm.DB
}

// NewCourseRepo creates a new CourseRepo.
func NewCourseRepo(db *gorm.DB) *CourseRepo {
	return &CourseRepo{DB: db}
}

func (r *CourseRepo) FindByID(ctx context.Context, id uint) (*model.Course, error) {
	var course model.Course
	err := r.DB.WithContext(ctx).First(&course, id).Error
	if err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *CourseRepo) FindWithOutline(ctx context.Context, id uint) (*model.Course, error) {
	var course model.Course
	err := r.DB.WithContext(ctx).
		Preload("Chapters", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Preload("Chapters.KnowledgePoints.MountedSkills").
		First(&course, id).Error
	if err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *CourseRepo) FindWithChaptersAndKPs(ctx context.Context, id uint) (*model.Course, error) {
	var course model.Course
	err := r.DB.WithContext(ctx).
		Preload("Chapters", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Preload("Chapters.KnowledgePoints").
		First(&course, id).Error
	if err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *CourseRepo) ListByTeacher(ctx context.Context, teacherID uint, schoolID uint, offset, limit int) ([]model.Course, int64, error) {
	query := r.DB.WithContext(ctx).Preload("Chapters.KnowledgePoints")

	if schoolID > 0 {
		query = query.Where("school_id = ?", schoolID)
	}
	query = query.Where("teacher_id = ?", teacherID)

	var total int64
	query.Model(&model.Course{}).Count(&total)

	var courses []model.Course
	err := query.Offset(offset).Limit(limit).Find(&courses).Error
	return courses, total, err
}

func (r *CourseRepo) Create(ctx context.Context, course *model.Course) error {
	return r.DB.WithContext(ctx).Create(course).Error
}
