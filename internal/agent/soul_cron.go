package agent

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
)

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

	log.Printf("🔄 Soul cron started, interval: %v", s.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Soul cron stopped")
			return
		case <-ticker.C:
			log.Println("⏰ Soul evolution analysis triggered")
			suggestion, err := s.evolution.AnalyzeAndSuggest(ctx)
			if err != nil {
				log.Printf("❌ Soul evolution failed: %v", err)
				continue
			}
			log.Printf("✅ Soul evolution completed, suggestion length: %d", len(suggestion))
			
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
			log.Printf("📬 Notified %d admins", len(admins))
		}
	}
}

