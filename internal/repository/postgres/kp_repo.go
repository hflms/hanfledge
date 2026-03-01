package postgres

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/repository"
	"gorm.io/gorm"
)

// KnowledgePointRepo is the GORM implementation of repository.KnowledgePointRepository.
type KnowledgePointRepo struct {
	DB *gorm.DB
}

// NewKnowledgePointRepo creates a new KnowledgePointRepo.
func NewKnowledgePointRepo(db *gorm.DB) *KnowledgePointRepo {
	return &KnowledgePointRepo{DB: db}
}

func (r *KnowledgePointRepo) FindByID(ctx context.Context, id uint) (*model.KnowledgePoint, error) {
	var kp model.KnowledgePoint
	err := r.DB.WithContext(ctx).First(&kp, id).Error
	if err != nil {
		return nil, err
	}
	return &kp, nil
}

func (r *KnowledgePointRepo) FindByCourseID(ctx context.Context, courseID uint) ([]model.KnowledgePoint, error) {
	var kps []model.KnowledgePoint
	err := r.DB.WithContext(ctx).
		Joins("JOIN chapters ON chapters.id = knowledge_points.chapter_id").
		Where("chapters.course_id = ?", courseID).
		Order("chapters.sort_order ASC, knowledge_points.id ASC").
		Find(&kps).Error
	return kps, err
}

func (r *KnowledgePointRepo) FindIDsByCourseID(ctx context.Context, courseID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.WithContext(ctx).
		Model(&model.KnowledgePoint{}).
		Joins("JOIN chapters ON chapters.id = knowledge_points.chapter_id").
		Where("chapters.course_id = ?", courseID).
		Pluck("knowledge_points.id", &ids).Error
	return ids, err
}

func (r *KnowledgePointRepo) FindByIDs(ctx context.Context, ids []uint) ([]model.KnowledgePoint, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var kps []model.KnowledgePoint
	err := r.DB.WithContext(ctx).Where("id IN ?", ids).Find(&kps).Error
	return kps, err
}

func (r *KnowledgePointRepo) FindByIDsWithChapter(ctx context.Context, ids []uint) ([]model.KnowledgePoint, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var kps []model.KnowledgePoint
	err := r.DB.WithContext(ctx).Preload("Chapter").Where("id IN ?", ids).Find(&kps).Error
	return kps, err
}

func (r *KnowledgePointRepo) FindWithChapterTitles(ctx context.Context, ids []uint) ([]repository.KPWithChapterTitle, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var results []repository.KPWithChapterTitle
	err := r.DB.WithContext(ctx).Raw(`
		SELECT kp.id, kp.title, c.title AS chapter_title
		FROM knowledge_points kp
		JOIN chapters c ON c.id = kp.chapter_id
		WHERE kp.id IN ?
	`, ids).Scan(&results).Error
	return results, err
}

// -- Misconception operations --

func (r *KnowledgePointRepo) CreateMisconception(ctx context.Context, m *model.Misconception) error {
	return r.DB.WithContext(ctx).Create(m).Error
}

func (r *KnowledgePointRepo) UpdateMisconceptionNeo4jID(ctx context.Context, id uint, nodeID string) error {
	return r.DB.WithContext(ctx).Model(&model.Misconception{}).
		Where("id = ?", id).
		Update("neo4j_node_id", nodeID).Error
}

func (r *KnowledgePointRepo) ListMisconceptionsByKPID(ctx context.Context, kpID uint) ([]model.Misconception, error) {
	var misconceptions []model.Misconception
	err := r.DB.WithContext(ctx).Where("kp_id = ?", kpID).
		Order("severity DESC").Find(&misconceptions).Error
	return misconceptions, err
}

func (r *KnowledgePointRepo) FindMisconceptionByIDAndKPID(ctx context.Context, id, kpID uint) (*model.Misconception, error) {
	var m model.Misconception
	err := r.DB.WithContext(ctx).Where("id = ? AND kp_id = ?", id, kpID).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *KnowledgePointRepo) DeleteMisconception(ctx context.Context, m *model.Misconception) error {
	return r.DB.WithContext(ctx).Delete(m).Error
}

// -- Cross-link operations --

func (r *KnowledgePointRepo) CreateCrossLink(ctx context.Context, link *model.CrossLink) error {
	return r.DB.WithContext(ctx).Create(link).Error
}

func (r *KnowledgePointRepo) ListCrossLinksByKPID(ctx context.Context, kpID uint) ([]model.CrossLink, error) {
	var links []model.CrossLink
	err := r.DB.WithContext(ctx).
		Where("from_kp_id = ? OR to_kp_id = ?", kpID, kpID).
		Find(&links).Error
	return links, err
}

func (r *KnowledgePointRepo) FindCrossLinkByIDAndKPID(ctx context.Context, linkID, kpID uint) (*model.CrossLink, error) {
	var link model.CrossLink
	err := r.DB.WithContext(ctx).
		Where("id = ? AND (from_kp_id = ? OR to_kp_id = ?)", linkID, kpID, kpID).
		First(&link).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *KnowledgePointRepo) DeleteCrossLink(ctx context.Context, link *model.CrossLink) error {
	return r.DB.WithContext(ctx).Delete(link).Error
}
