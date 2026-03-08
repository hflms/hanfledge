package agent

import (
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"gorm.io/gorm"
)

var slogProfileMgr = logger.L("ProfileManager")

// ProfileManager 管理学生档案和错题本。
type ProfileManager struct {
	db      *gorm.DB
	bkt     *BKTService
	profile *ProfileService
}

// NewProfileManager 创建学生档案管理器。
func NewProfileManager(db *gorm.DB, bkt *BKTService, profile *ProfileService) *ProfileManager {
	return &ProfileManager{db: db, bkt: bkt, profile: profile}
}

// ArchiveErrorIfIncorrect 当学生回答不正确时，归档到错题本。
func (p *ProfileManager) ArchiveErrorIfIncorrect(tc *TurnContext, response *DraftResponse, inferCorrect func(*TurnContext) bool) {
	if response == nil {
		return
	}

	var session model.StudentSession
	if err := p.db.WithContext(tc.Ctx).First(&session, tc.SessionID).Error; err != nil {
		return
	}

	kpID := session.CurrentKP
	if kpID == 0 && tc.Prescription != nil && len(tc.Prescription.TargetKPSequence) > 0 {
		kpID = tc.Prescription.TargetKPSequence[0].KPID
	}
	if kpID == 0 {
		return
	}

	correct := inferCorrect(tc)

	if !correct {
		mastery := p.bkt.GetMastery(tc.StudentID, kpID)

		entry := model.ErrorNotebookEntry{
			StudentID:      tc.StudentID,
			KPID:           kpID,
			SessionID:      tc.SessionID,
			StudentInput:   tc.UserInput,
			CoachGuidance:  response.Content,
			ErrorType:      "unknown",
			MasteryAtError: mastery,
			ArchivedAt:     time.Now(),
		}

		if err := p.db.WithContext(tc.Ctx).Create(&entry).Error; err != nil {
			slogProfileMgr.Warn("error notebook archive failed",
				"student_id", tc.StudentID, "kp_id", kpID, "err", err)
			return
		}

		slogProfileMgr.Info("error notebook archived",
			"student_id", tc.StudentID, "kp_id", kpID, "session_id", tc.SessionID, "mastery", mastery)
	}

	currentMastery := p.bkt.GetMastery(tc.StudentID, kpID)
	if currentMastery >= 0.8 {
		now := time.Now()
		result := p.db.WithContext(tc.Ctx).Model(&model.ErrorNotebookEntry{}).
			Where("student_id = ? AND kp_id = ? AND resolved = ?", tc.StudentID, kpID, false).
			Updates(map[string]interface{}{
				"resolved":    true,
				"resolved_at": now,
			})
		if result.RowsAffected > 0 {
			slogProfileMgr.Info("error notebook auto-resolved",
				"entries", result.RowsAffected, "student_id", tc.StudentID, "kp_id", kpID, "mastery", currentMastery)
		}
	}
}

// UpdateProfile 每轮交互后更新学生跨会话画像。
func (p *ProfileManager) UpdateProfile(tc *TurnContext) {
	p.profile.IncrementInteraction(tc.StudentID)
	p.profile.RefreshMasteryStats(tc.StudentID)

	var interactionCount int64
	p.db.WithContext(tc.Ctx).Model(&model.Interaction{}).
		Where("session_id = ? AND role = ?", tc.SessionID, "student").
		Count(&interactionCount)
	if interactionCount%5 == 0 {
		p.profile.RefreshStrengthWeakness(tc.StudentID)
	}

	slogProfileMgr.Debug("student profile updated",
		"student_id", tc.StudentID, "session_id", tc.SessionID)
}
