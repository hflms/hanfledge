package postgres

import (
	"context"
	"strings"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// MarketplaceRepo is the GORM implementation of repository.MarketplaceRepository.
type MarketplaceRepo struct {
	DB *gorm.DB
}

// NewMarketplaceRepo creates a new MarketplaceRepo.
func NewMarketplaceRepo(db *gorm.DB) *MarketplaceRepo {
	return &MarketplaceRepo{DB: db}
}

func (r *MarketplaceRepo) ListApproved(ctx context.Context, pluginType, category, search string, offset, limit int) ([]model.MarketplacePlugin, int64, error) {
	query := r.DB.WithContext(ctx).Where("status = ?", "approved")

	if pluginType != "" {
		query = query.Where("type = ?", pluginType)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		escaped := escapeLike(search)
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+escaped+"%", "%"+escaped+"%")
	}

	var total int64
	query.Model(&model.MarketplacePlugin{}).Count(&total)

	var plugins []model.MarketplacePlugin
	err := query.Order("downloads DESC").Offset(offset).Limit(limit).Find(&plugins).Error
	return plugins, total, err
}

func (r *MarketplaceRepo) FindByPluginID(ctx context.Context, pluginID string) (*model.MarketplacePlugin, error) {
	var plugin model.MarketplacePlugin
	err := r.DB.WithContext(ctx).Where("plugin_id = ?", pluginID).First(&plugin).Error
	if err != nil {
		return nil, err
	}
	return &plugin, nil
}

func (r *MarketplaceRepo) FindApprovedByPluginID(ctx context.Context, pluginID string) (*model.MarketplacePlugin, error) {
	var plugin model.MarketplacePlugin
	err := r.DB.WithContext(ctx).Where("plugin_id = ? AND status = ?", pluginID, "approved").First(&plugin).Error
	if err != nil {
		return nil, err
	}
	return &plugin, nil
}

func (r *MarketplaceRepo) ListReviewsByPluginID(ctx context.Context, pluginID string, limit int) ([]model.MarketplaceReview, error) {
	var reviews []model.MarketplaceReview
	err := r.DB.WithContext(ctx).Where("plugin_id = ?", pluginID).
		Order("created_at DESC").Limit(limit).Find(&reviews).Error
	return reviews, err
}

func (r *MarketplaceRepo) CreatePlugin(ctx context.Context, plugin *model.MarketplacePlugin) error {
	return r.DB.WithContext(ctx).Create(plugin).Error
}

func (r *MarketplaceRepo) FindInstalledPlugin(ctx context.Context, schoolID uint, pluginID string) (*model.InstalledPlugin, error) {
	var installed model.InstalledPlugin
	err := r.DB.WithContext(ctx).Where("school_id = ? AND plugin_id = ?", schoolID, pluginID).First(&installed).Error
	if err != nil {
		return nil, err
	}
	return &installed, nil
}

func (r *MarketplaceRepo) CreateInstalledPlugin(ctx context.Context, installed *model.InstalledPlugin) error {
	return r.DB.WithContext(ctx).Create(installed).Error
}

func (r *MarketplaceRepo) IncrementDownloads(ctx context.Context, pluginID uint) error {
	return r.DB.WithContext(ctx).Model(&model.MarketplacePlugin{}).
		Where("id = ?", pluginID).
		Update("downloads", gorm.Expr("downloads + 1")).Error
}

func (r *MarketplaceRepo) DeleteInstalledPlugin(ctx context.Context, id uint) error {
	return r.DB.WithContext(ctx).Delete(&model.InstalledPlugin{}, id).Error
}

func (r *MarketplaceRepo) ListInstalledBySchool(ctx context.Context, schoolID uint) ([]model.InstalledPlugin, error) {
	var installed []model.InstalledPlugin
	err := r.DB.WithContext(ctx).Where("school_id = ?", schoolID).Find(&installed).Error
	return installed, err
}

// escapeLike escapes SQL LIKE wildcards in user input.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
