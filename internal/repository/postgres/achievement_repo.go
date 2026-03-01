package postgres

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// AchievementRepo is the GORM implementation of repository.AchievementRepository.
type AchievementRepo struct {
	DB *gorm.DB
}

// NewAchievementRepo creates a new AchievementRepo.
func NewAchievementRepo(db *gorm.DB) *AchievementRepo {
	return &AchievementRepo{DB: db}
}

func (r *AchievementRepo) ListDefinitions(ctx context.Context) ([]model.AchievementDefinition, error) {
	var defs []model.AchievementDefinition
	err := r.DB.WithContext(ctx).Order("sort_order ASC").Find(&defs).Error
	return defs, err
}

func (r *AchievementRepo) ListDefinitionsByType(ctx context.Context, achievementType model.AchievementType) ([]model.AchievementDefinition, error) {
	var defs []model.AchievementDefinition
	err := r.DB.WithContext(ctx).Where("type = ?", achievementType).
		Order("threshold ASC").Find(&defs).Error
	return defs, err
}

func (r *AchievementRepo) FindStudentAchievements(ctx context.Context, studentID uint) ([]model.StudentAchievement, error) {
	var records []model.StudentAchievement
	err := r.DB.WithContext(ctx).Where("student_id = ?", studentID).Find(&records).Error
	return records, err
}

func (r *AchievementRepo) FindStudentAchievement(ctx context.Context, studentID, achievementID uint) (*model.StudentAchievement, error) {
	var rec model.StudentAchievement
	err := r.DB.WithContext(ctx).
		Where("student_id = ? AND achievement_id = ?", studentID, achievementID).
		First(&rec).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *AchievementRepo) CreateStudentAchievement(ctx context.Context, rec *model.StudentAchievement) error {
	return r.DB.WithContext(ctx).Create(rec).Error
}

func (r *AchievementRepo) SaveStudentAchievement(ctx context.Context, rec *model.StudentAchievement) error {
	return r.DB.WithContext(ctx).Save(rec).Error
}
