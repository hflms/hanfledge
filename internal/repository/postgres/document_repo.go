package postgres

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// DocumentRepo is the GORM implementation of repository.DocumentRepository.
type DocumentRepo struct {
	DB *gorm.DB
}

// NewDocumentRepo creates a new DocumentRepo.
func NewDocumentRepo(db *gorm.DB) *DocumentRepo {
	return &DocumentRepo{DB: db}
}

func (r *DocumentRepo) Create(ctx context.Context, doc *model.Document) error {
	return r.DB.WithContext(ctx).Create(doc).Error
}

func (r *DocumentRepo) UpdateStatus(ctx context.Context, docID uint, status model.DocStatus) error {
	return r.DB.WithContext(ctx).Model(&model.Document{}).
		Where("id = ?", docID).
		Update("status", status).Error
}

func (r *DocumentRepo) UpdateFields(ctx context.Context, docID uint, fields map[string]interface{}) error {
	return r.DB.WithContext(ctx).Model(&model.Document{}).
		Where("id = ?", docID).
		Updates(fields).Error
}

func (r *DocumentRepo) FindByCourseID(ctx context.Context, courseID uint) ([]model.Document, error) {
	var docs []model.Document
	err := r.DB.WithContext(ctx).Where("course_id = ?", courseID).Find(&docs).Error
	return docs, err
}

func (r *DocumentRepo) FindByCourseIDOrdered(ctx context.Context, courseID uint) ([]model.Document, error) {
	var docs []model.Document
	err := r.DB.WithContext(ctx).Where("course_id = ?", courseID).
		Order("created_at DESC").Find(&docs).Error
	return docs, err
}

func (r *DocumentRepo) FindByIDAndCourseID(ctx context.Context, docID, courseID uint) (*model.Document, error) {
	var doc model.Document
	err := r.DB.WithContext(ctx).Where("id = ? AND course_id = ?", docID, courseID).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *DocumentRepo) DeleteChunksByDocumentID(ctx context.Context, docID uint) error {
	return r.DB.WithContext(ctx).Where("document_id = ?", docID).Delete(&model.DocumentChunk{}).Error
}

func (r *DocumentRepo) Delete(ctx context.Context, doc *model.Document) error {
	return r.DB.WithContext(ctx).Delete(doc).Error
}
