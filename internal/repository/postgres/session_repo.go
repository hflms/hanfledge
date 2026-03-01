package postgres

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// SessionRepo is the GORM implementation of repository.SessionRepository.
type SessionRepo struct {
	DB *gorm.DB
}

// NewSessionRepo creates a new SessionRepo.
func NewSessionRepo(db *gorm.DB) *SessionRepo {
	return &SessionRepo{DB: db}
}

func (r *SessionRepo) FindByID(ctx context.Context, id uint) (*model.StudentSession, error) {
	var session model.StudentSession
	err := r.DB.WithContext(ctx).First(&session, id).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *SessionRepo) FindActive(ctx context.Context, studentID, activityID uint, isSandbox bool) (*model.StudentSession, error) {
	var session model.StudentSession
	err := r.DB.WithContext(ctx).
		Where("student_id = ? AND activity_id = ? AND is_sandbox = ? AND status = ?",
			studentID, activityID, isSandbox, model.SessionStatusActive).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *SessionRepo) Create(ctx context.Context, session *model.StudentSession) error {
	return r.DB.WithContext(ctx).Create(session).Error
}

func (r *SessionRepo) ListByActivityID(ctx context.Context, activityID uint, excludeSandbox bool) ([]model.StudentSession, error) {
	query := r.DB.WithContext(ctx).Where("activity_id = ?", activityID)
	if excludeSandbox {
		query = query.Where("is_sandbox = ?", false)
	}
	var sessions []model.StudentSession
	err := query.Order("started_at DESC").Find(&sessions).Error
	return sessions, err
}

func (r *SessionRepo) FindInteractions(ctx context.Context, sessionID uint, limit int) ([]model.Interaction, error) {
	query := r.DB.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var interactions []model.Interaction
	err := query.Find(&interactions).Error
	return interactions, err
}

func (r *SessionRepo) CountStudentInteractions(ctx context.Context, sessionID uint) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&model.Interaction{}).
		Where("session_id = ? AND role = ?", sessionID, "student").
		Count(&count).Error
	return count, err
}
