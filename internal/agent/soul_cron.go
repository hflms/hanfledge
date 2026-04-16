package agent

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogSoulCron = logger.L("SoulCron")

// SoulCronService 定期触发 Soul 进化分析。
type SoulCronService struct {
	db        *gorm.DB
	evolution *SoulEvolutionService
	interval  time.Duration
}

// NewSoulCronService 创建 Soul Cron 服务。
func NewSoulCronService(db *gorm.DB, llm llm.LLMProvider, soulPath string) *SoulCronService {
	return &SoulCronService{
		db:        db,
		evolution: NewSoulEvolutionService(db, llm, soulPath),
		interval:  7 * 24 * time.Hour, // 每周
	}
}

// Start 启动定期分析任务。
func (s *SoulCronService) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	slogSoulCron.Info("🔄 Soul cron started", "interval", s.interval)

	for {
		select {
		case <-ctx.Done():
			slogSoulCron.Info("🛑 Soul cron stopped")
			return
		case <-ticker.C:
			slogSoulCron.Info("⏰ Soul evolution analysis triggered")
			suggestion, err := s.evolution.AnalyzeAndSuggest(ctx)
			if err != nil {
				slogSoulCron.Error("❌ Soul evolution failed", "err", err)
				continue
			}
			slogSoulCron.Info("✅ Soul evolution completed", "suggestionLength", len(suggestion))
			
			// 通知所有管理员
			var admins []model.User
			s.db.Joins("JOIN user_school_roles ON users.id = user_school_roles.user_id").
				Joins("JOIN roles ON user_school_roles.role_id = roles.id").
				Where("roles.name = ?", model.RoleSysAdmin).
				Find(&admins)

			for _, admin := range admins {
				notif := model.Notification{
					UserID:    admin.ID,
					Type:      "soul_evolution",
					Title:     "Soul 规则进化建议已生成",
					Content:   suggestion,
					IsRead:    false,
					CreatedAt: time.Now(),
				}
				s.db.Create(&notif)
			}
			slogSoulCron.Info("📬 Notified admins", "count", len(admins))
		}
	}
}

