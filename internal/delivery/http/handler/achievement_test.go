package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// ============================
// Achievement Handler Tests
// ============================

// seedAchievementDefs creates the 12 default achievement definitions.
func seedAchievementDefs(t *testing.T, db *gorm.DB) {
	t.Helper()
	defs := []model.AchievementDefinition{
		{ID: 1, Type: model.AchievementStreakBreaker, Tier: model.TierBronze, Name: "初露锋芒", Description: "连续掌握 3 个知识点", Icon: "🔥", Threshold: 3, SortOrder: 1},
		{ID: 2, Type: model.AchievementStreakBreaker, Tier: model.TierSilver, Name: "势如破竹", Description: "连续掌握 5 个知识点", Icon: "⚡", Threshold: 5, SortOrder: 2},
		{ID: 3, Type: model.AchievementStreakBreaker, Tier: model.TierGold, Name: "一往无前", Description: "连续掌握 10 个知识点", Icon: "🌟", Threshold: 10, SortOrder: 3},
		{ID: 4, Type: model.AchievementStreakBreaker, Tier: model.TierDiamond, Name: "学霸无双", Description: "连续掌握 20 个知识点", Icon: "💎", Threshold: 20, SortOrder: 4},
		{ID: 5, Type: model.AchievementDeepInquiry, Tier: model.TierBronze, Name: "好奇宝宝", Description: "单次会话中追问 5 轮", Icon: "🔍", Threshold: 5, SortOrder: 5},
		{ID: 6, Type: model.AchievementDeepInquiry, Tier: model.TierSilver, Name: "刨根问底", Description: "单次会话中追问 10 轮", Icon: "🧐", Threshold: 10, SortOrder: 6},
		{ID: 7, Type: model.AchievementDeepInquiry, Tier: model.TierGold, Name: "思维深潜", Description: "单次会话中追问 15 轮", Icon: "🧠", Threshold: 15, SortOrder: 7},
		{ID: 8, Type: model.AchievementDeepInquiry, Tier: model.TierDiamond, Name: "追问大师", Description: "单次会话中追问 20 轮", Icon: "💡", Threshold: 20, SortOrder: 8},
		{ID: 9, Type: model.AchievementFallacyHunt, Tier: model.TierBronze, Name: "火眼金睛", Description: "累计识别 3 个谬误", Icon: "🎯", Threshold: 3, SortOrder: 9},
		{ID: 10, Type: model.AchievementFallacyHunt, Tier: model.TierSilver, Name: "明察秋毫", Description: "累计识别 10 个谬误", Icon: "🔎", Threshold: 10, SortOrder: 10},
		{ID: 11, Type: model.AchievementFallacyHunt, Tier: model.TierGold, Name: "谬误克星", Description: "累计识别 20 个谬误", Icon: "🛡️", Threshold: 20, SortOrder: 11},
		{ID: 12, Type: model.AchievementFallacyHunt, Tier: model.TierDiamond, Name: "真理守护者", Description: "累计识别 50 个谬误", Icon: "👑", Threshold: 50, SortOrder: 12},
	}
	for _, d := range defs {
		if err := db.Create(&d).Error; err != nil {
			t.Fatalf("seedAchievementDefs failed: %v", err)
		}
	}
}

// -- ListDefinitions Tests ----------------------------------------

func TestAchievementHandler_ListDefinitions(t *testing.T) {
	db := setupTestDB(t)
	h := NewAchievementHandler(db)

	t.Run("returns empty when no definitions", func(t *testing.T) {
		w, c := newTestContext("GET", "/api/v1/student/achievements/definitions", "", 1)
		h.ListDefinitions(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "[]")
	})

	t.Run("returns all definitions sorted by sort_order", func(t *testing.T) {
		seedAchievementDefs(t, db)
		w, c := newTestContext("GET", "/api/v1/student/achievements/definitions", "", 1)
		h.ListDefinitions(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "初露锋芒")
		assertBodyContains(t, w, "streak_breaker")
		assertBodyContains(t, w, "deep_inquiry")
		assertBodyContains(t, w, "fallacy_hunter")
	})
}

// -- GetMyAchievements Tests ----------------------------------------

func TestAchievementHandler_GetMyAchievements(t *testing.T) {
	db := setupTestDB(t)
	seedAchievementDefs(t, db)
	student := seedUser(t, db, "13800001111", "pass123", "测试学生", model.UserStatusActive)
	h := NewAchievementHandler(db)

	t.Run("returns all defs with zero progress for new student", func(t *testing.T) {
		w, c := newTestContext("GET", "/api/v1/student/achievements", "", student.ID)
		h.GetMyAchievements(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, `"total_unlocked":0`)
		assertBodyContains(t, w, `"total_count":12`)
		assertBodyContains(t, w, `"progress":0`)
	})

	t.Run("reflects progress after partial achievement", func(t *testing.T) {
		// Give student some progress
		now := time.Now()
		db.Create(&model.StudentAchievement{
			StudentID:     student.ID,
			AchievementID: 1,
			Progress:      2,
			Unlocked:      false,
		})
		db.Create(&model.StudentAchievement{
			StudentID:     student.ID,
			AchievementID: 9,
			Progress:      3,
			Unlocked:      true,
			UnlockedAt:    &now,
		})

		w, c := newTestContext("GET", "/api/v1/student/achievements", "", student.ID)
		h.GetMyAchievements(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, `"total_unlocked":1`)
		assertBodyContains(t, w, `"progress":2`)
		assertBodyContains(t, w, `"unlocked":true`)
	})
}

// -- EvaluateStreakBreaker Tests ----------------------------------------

func TestAchievementHandler_EvaluateStreakBreaker(t *testing.T) {
	db := setupTestDB(t)
	seedAchievementDefs(t, db)
	student := seedUser(t, db, "13800002222", "pass123", "突破学生", model.UserStatusActive)
	h := NewAchievementHandler(db)

	// Create chapter/KP for mastery records
	course := seedCourse(t, db, 1, "物理")
	ch := seedChapter(t, db, course.ID, "力学", 1)

	t.Run("no unlock with fewer than 3 mastered KPs", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			kp := seedKP(t, db, ch.ID, "知识点")
			db.Create(&model.StudentKPMastery{
				StudentID: student.ID, KPID: kp.ID,
				MasteryScore: 0.85, AttemptCount: 3,
			})
		}
		h.EvaluateStreakBreaker(student.ID)

		var rec model.StudentAchievement
		db.Where("student_id = ? AND achievement_id = 1", student.ID).First(&rec)
		if rec.Unlocked {
			t.Error("should not unlock bronze with only 2 mastered KPs")
		}
	})

	t.Run("unlocks bronze with 3+ mastered KPs", func(t *testing.T) {
		kp := seedKP(t, db, ch.ID, "第三知识点")
		db.Create(&model.StudentKPMastery{
			StudentID: student.ID, KPID: kp.ID,
			MasteryScore: 0.9, AttemptCount: 5,
		})
		h.EvaluateStreakBreaker(student.ID)

		var rec model.StudentAchievement
		db.Where("student_id = ? AND achievement_id = 1", student.ID).First(&rec)
		if !rec.Unlocked {
			t.Error("should unlock bronze streak breaker with 3 mastered KPs")
		}
		if rec.Progress != 3 {
			t.Errorf("progress = %d, want 3", rec.Progress)
		}
	})
}

// -- EvaluateDeepInquiry Tests ----------------------------------------

func TestAchievementHandler_EvaluateDeepInquiry(t *testing.T) {
	db := setupTestDB(t)
	seedAchievementDefs(t, db)
	student := seedUser(t, db, "13800003333", "pass123", "追问学生", model.UserStatusActive)
	course := seedCourse(t, db, 1, "数学")
	ch := seedChapter(t, db, course.ID, "代数", 1)
	kp := seedKP(t, db, ch.ID, "方程")
	act := seedActivity(t, db, 1, course.ID, "方程练习")
	sess := seedSession(t, db, student.ID, act.ID, kp.ID, model.SessionStatusActive)
	h := NewAchievementHandler(db)

	t.Run("no unlock with fewer than 5 turns", func(t *testing.T) {
		for i := 0; i < 4; i++ {
			db.Create(&model.Interaction{
				SessionID: sess.ID, Role: "student",
				Content: "问题", CreatedAt: time.Now(),
			})
		}
		h.EvaluateDeepInquiry(student.ID, sess.ID)

		var rec model.StudentAchievement
		db.Where("student_id = ? AND achievement_id = 5", student.ID).First(&rec)
		if rec.Unlocked {
			t.Error("should not unlock with 4 turns")
		}
	})

	t.Run("unlocks bronze with 5+ turns", func(t *testing.T) {
		db.Create(&model.Interaction{
			SessionID: sess.ID, Role: "student",
			Content: "第五个问题", CreatedAt: time.Now(),
		})
		h.EvaluateDeepInquiry(student.ID, sess.ID)

		var rec model.StudentAchievement
		db.Where("student_id = ? AND achievement_id = 5", student.ID).First(&rec)
		if !rec.Unlocked {
			t.Error("should unlock bronze deep inquiry with 5 turns")
		}
	})
}

// -- EvaluateFallacyHunter Tests ----------------------------------------

func TestAchievementHandler_EvaluateFallacyHunter(t *testing.T) {
	db := setupTestDB(t)
	seedAchievementDefs(t, db)
	student := seedUser(t, db, "13800004444", "pass123", "猎人学生", model.UserStatusActive)
	h := NewAchievementHandler(db)

	t.Run("increments progress cumulatively", func(t *testing.T) {
		h.EvaluateFallacyHunter(student.ID, 1)

		var rec model.StudentAchievement
		db.Where("student_id = ? AND achievement_id = 9", student.ID).First(&rec)
		if rec.Progress != 1 {
			t.Errorf("progress = %d, want 1", rec.Progress)
		}
		if rec.Unlocked {
			t.Error("should not unlock with 1 fallacy")
		}
	})

	t.Run("unlocks bronze at threshold 3", func(t *testing.T) {
		h.EvaluateFallacyHunter(student.ID, 3)

		var rec model.StudentAchievement
		db.Where("student_id = ? AND achievement_id = 9", student.ID).First(&rec)
		if !rec.Unlocked {
			t.Error("should unlock bronze fallacy hunter at 3")
		}
		if rec.Progress != 3 {
			t.Errorf("progress = %d, want 3", rec.Progress)
		}
	})
}
