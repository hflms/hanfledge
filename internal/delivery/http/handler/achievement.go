package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"gorm.io/gorm"
)

var slogAchieve = logger.L("Achievement")

// ============================
// 成就系统 Handler — design.md §5.2 Step 4
// ============================
//
// 提供成就定义列表、学生成就进度、成就评估触发三个 API。
// 强调纵向自我成长对比，不设学生间排行。

// AchievementHandler handles achievement/gamification APIs.
type AchievementHandler struct {
	DB *gorm.DB
}

// NewAchievementHandler creates a new AchievementHandler.
func NewAchievementHandler(db *gorm.DB) *AchievementHandler {
	return &AchievementHandler{DB: db}
}

// -- Response Types ----------------------------------------

// AchievementProgressResponse 单个成就进度。
type AchievementProgressResponse struct {
	ID          uint    `json:"id"`
	Type        string  `json:"type"`
	Tier        string  `json:"tier"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Icon        string  `json:"icon"`
	Threshold   int     `json:"threshold"`
	Progress    int     `json:"progress"`
	Unlocked    bool    `json:"unlocked"`
	UnlockedAt  *string `json:"unlocked_at,omitempty"`
}

// StudentAchievementsResponse 学生成就总览。
type StudentAchievementsResponse struct {
	TotalUnlocked int                           `json:"total_unlocked"`
	TotalCount    int                           `json:"total_count"`
	Achievements  []AchievementProgressResponse `json:"achievements"`
}

// -- List Achievement Definitions ----------------------------------------

// ListDefinitions returns all achievement definitions.
// GET /api/v1/student/achievements/definitions
func (h *AchievementHandler) ListDefinitions(c *gin.Context) {
	var defs []model.AchievementDefinition
	if err := h.DB.Order("sort_order ASC").Find(&defs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取成就定义失败"})
		return
	}
	c.JSON(http.StatusOK, defs)
}

// -- Get Student Achievements ----------------------------------------

// GetMyAchievements returns the current student's achievement progress.
// GET /api/v1/student/achievements
func (h *AchievementHandler) GetMyAchievements(c *gin.Context) {
	studentID := middleware.GetUserID(c)

	// Load all definitions
	var defs []model.AchievementDefinition
	if err := h.DB.Order("sort_order ASC").Find(&defs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取成就定义失败"})
		return
	}

	// Load student's achievement records
	var records []model.StudentAchievement
	h.DB.Where("student_id = ?", studentID).Find(&records)

	// Build a map for quick lookup
	recordMap := make(map[uint]model.StudentAchievement)
	for _, r := range records {
		recordMap[r.AchievementID] = r
	}

	// Merge definitions with progress
	totalUnlocked := 0
	items := make([]AchievementProgressResponse, 0, len(defs))
	for _, d := range defs {
		item := AchievementProgressResponse{
			ID:          d.ID,
			Type:        string(d.Type),
			Tier:        string(d.Tier),
			Name:        d.Name,
			Description: d.Description,
			Icon:        d.Icon,
			Threshold:   d.Threshold,
		}
		if rec, ok := recordMap[d.ID]; ok {
			item.Progress = rec.Progress
			item.Unlocked = rec.Unlocked
			if rec.UnlockedAt != nil {
				t := rec.UnlockedAt.Format(time.RFC3339)
				item.UnlockedAt = &t
			}
		}
		if item.Unlocked {
			totalUnlocked++
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, StudentAchievementsResponse{
		TotalUnlocked: totalUnlocked,
		TotalCount:    len(defs),
		Achievements:  items,
	})
}

// -- Achievement Evaluation Logic ----------------------------------------

// EvaluateStreakBreaker checks consecutive mastery breakthroughs for a student.
// Called when a mastery score reaches >= 0.8 (from EventBus or handler).
func (h *AchievementHandler) EvaluateStreakBreaker(studentID uint) {
	// Count consecutive mastered KPs (score >= 0.8) ordered by update time desc
	var masteries []model.StudentKPMastery
	h.DB.Where("student_id = ? AND mastery_score >= 0.8", studentID).
		Order("updated_at DESC").
		Find(&masteries)

	streak := len(masteries) // simplified: total mastered count as streak proxy
	h.updateAchievementProgress(studentID, model.AchievementStreakBreaker, streak)
}

// EvaluateDeepInquiry checks deep questioning turns for a session.
// Called after a student answer in a session.
func (h *AchievementHandler) EvaluateDeepInquiry(studentID, sessionID uint) {
	var turnCount int64
	h.DB.Model(&model.Interaction{}).
		Where("session_id = ? AND role = ?", sessionID, "student").
		Count(&turnCount)

	// For deep inquiry, we track the best single-session turn count
	// Update progress to max of current progress and this session's count
	var defs []model.AchievementDefinition
	h.DB.Where("type = ?", model.AchievementDeepInquiry).Order("threshold ASC").Find(&defs)

	if len(defs) == 0 {
		return
	}

	var defIDs []uint
	for _, d := range defs {
		defIDs = append(defIDs, d.ID)
	}

	var existingRecs []model.StudentAchievement
	h.DB.Where("student_id = ? AND achievement_id IN ?", studentID, defIDs).Find(&existingRecs)

	recMap := make(map[uint]model.StudentAchievement)
	for _, r := range existingRecs {
		recMap[r.AchievementID] = r
	}

	for _, d := range defs {
		rec, exists := recMap[d.ID]
		if !exists {
			// Create new record
			rec = model.StudentAchievement{
				StudentID:     studentID,
				AchievementID: d.ID,
				Progress:      int(turnCount),
			}
			if int(turnCount) >= d.Threshold {
				rec.Unlocked = true
				now := time.Now()
				rec.UnlockedAt = &now
				slogAchieve.Info("achievement unlocked", "student_id", studentID, "name", d.Name, "turns", turnCount)
			}
			h.DB.Create(&rec)
		} else {
			// Update only if new count is higher
			if int(turnCount) > rec.Progress {
				rec.Progress = int(turnCount)
				if !rec.Unlocked && int(turnCount) >= d.Threshold {
					rec.Unlocked = true
					now := time.Now()
					rec.UnlockedAt = &now
					slogAchieve.Info("achievement unlocked", "student_id", studentID, "name", d.Name, "turns", turnCount)
				}
				h.DB.Save(&rec)
			}
		}
	}
}

// EvaluateFallacyHunter increments fallacy identification count.
// Called when a fallacy_identified event is received.
func (h *AchievementHandler) EvaluateFallacyHunter(studentID uint, identifiedCount int) {
	h.updateAchievementProgress(studentID, model.AchievementFallacyHunt, identifiedCount)
}

// updateAchievementProgress updates progress for all tiers of a given type.
func (h *AchievementHandler) updateAchievementProgress(studentID uint, achievementType model.AchievementType, value int) {
	var defs []model.AchievementDefinition
	h.DB.Where("type = ?", achievementType).Order("threshold ASC").Find(&defs)

	if len(defs) == 0 {
		return
	}

	var defIDs []uint
	for _, d := range defs {
		defIDs = append(defIDs, d.ID)
	}

	var existingRecs []model.StudentAchievement
	h.DB.Where("student_id = ? AND achievement_id IN ?", studentID, defIDs).Find(&existingRecs)

	recMap := make(map[uint]model.StudentAchievement)
	for _, r := range existingRecs {
		recMap[r.AchievementID] = r
	}

	for _, d := range defs {
		rec, exists := recMap[d.ID]
		if !exists {
			// Create new record
			rec = model.StudentAchievement{
				StudentID:     studentID,
				AchievementID: d.ID,
				Progress:      value,
			}
			if value >= d.Threshold {
				rec.Unlocked = true
				now := time.Now()
				rec.UnlockedAt = &now
				slogAchieve.Info("achievement unlocked", "student_id", studentID, "name", d.Name, "value", value)
			}
			h.DB.Create(&rec)
		} else {
			// Update progress (use max for cumulative achievements)
			if value > rec.Progress {
				rec.Progress = value
			}
			if !rec.Unlocked && rec.Progress >= d.Threshold {
				rec.Unlocked = true
				now := time.Now()
				rec.UnlockedAt = &now
				slogAchieve.Info("achievement unlocked", "student_id", studentID, "name", d.Name, "value", rec.Progress)
			}
			h.DB.Save(&rec)
		}
	}
}
