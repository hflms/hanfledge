package agent

import (
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"gorm.io/gorm"
)

var slogBKT = logger.L("BKT")

// ============================
// BKT — 贝叶斯知识追踪
// ============================
//
// Phase 1 实现：基于 4 参数 BKT 模型更新学生掌握度。
// 参考 design.md §9.2。

// BKTParams 贝叶斯知识追踪的四个核心参数。
type BKTParams struct {
	PL0 float64 `json:"p_l0"` // P(L0): 初始掌握概率 (默认 0.1)
	PT  float64 `json:"p_t"`  // P(T):  学习转移概率 — 从未掌握到掌握 (默认 0.3)
	PG  float64 `json:"p_g"`  // P(G):  猜测概率 — 未掌握但答对 (默认 0.2)
	PS  float64 `json:"p_s"`  // P(S):  失误概率 — 已掌握但答错 (默认 0.1)
}

// DefaultBKTParams 返回 BKT 的默认参数集。
func DefaultBKTParams() BKTParams {
	return BKTParams{
		PL0: 0.1,
		PT:  0.3,
		PG:  0.2,
		PS:  0.1,
	}
}

// EvidenceType 证据类型
type EvidenceType string

const (
	EvidenceChat EvidenceType = "chat" // 弱证据：苏格拉底对话中的表现
	EvidenceTest EvidenceType = "test" // 强证据：独立的技能测试结果
)

// UpdateMastery 根据学生的一次答题结果和证据类型更新掌握度。
// correct: 本次是否答对
// evidence: 证据类型 (chat 或 test)
// 返回: 更新后的 mastery_score [0.0, 1.0]
func (b *BKTParams) UpdateMastery(priorMastery float64, correct bool, evidence EvidenceType) float64 {
	// 动态调整参数
	ps := b.PS
	pg := b.PG

	if evidence == EvidenceTest {
		// 强证据：降低失误率和猜对率（测试题目要求严格）
		ps = 0.05
		pg = 0.05
	} else {
		// 弱证据：提高猜对率（因为有AI提示引导，很容易顺着说）
		pg = 0.4
	}

	var pCorrectGivenMastered, pCorrectGivenNotMastered float64
	pCorrectGivenMastered = 1.0 - ps
	pCorrectGivenNotMastered = pg

	// 贝叶斯后验更新
	var posterior float64
	if correct {
		pCorrect := priorMastery*pCorrectGivenMastered +
			(1-priorMastery)*pCorrectGivenNotMastered
		posterior = (priorMastery * pCorrectGivenMastered) / pCorrect
	} else {
		pIncorrect := priorMastery*ps +
			(1-priorMastery)*(1-pg)
		posterior = (priorMastery * ps) / pIncorrect
	}

	// 学习转移：即使当前未掌握，也有概率通过本次练习学会
	mastery := posterior + (1-posterior)*b.PT

	// Clamp to [0.0, 1.0]
	if mastery < 0.0 {
		mastery = 0.0
	}
	if mastery > 1.0 {
		mastery = 1.0
	}

	// 弱证据上限：如果仅通过对话，掌握度最高只能增加到 0.75，除非本来就更高
	if evidence == EvidenceChat && mastery > 0.75 && priorMastery <= 0.75 {
		mastery = 0.75
	}
	// 如果强证据测错，可能大幅下降
	if evidence == EvidenceTest && !correct && mastery > 0.4 {
		mastery = 0.4 // 打回原型
	}

	return mastery
}

// ── BKT Service ─────────────────────────────────────────────

// BKTService 管理 BKT 参数和持久化操作。
type BKTService struct {
	db     *gorm.DB
	params BKTParams
}

// NewBKTService 创建 BKT 服务（使用默认参数）。
func NewBKTService(db *gorm.DB) *BKTService {
	return &BKTService{
		db:     db,
		params: DefaultBKTParams(),
	}
}

// UpdateStudentMastery 更新学生对某知识点的掌握度。
// 1. 查询或创建 StudentKPMastery 记录
// 2. 使用 BKT 算法计算新掌握度
// 3. 持久化更新
// 返回更新后的 MasteryUpdate 事件。
func (s *BKTService) UpdateStudentMastery(studentID, kpID uint, correct bool, evidence EvidenceType) (MasteryUpdate, error) {
	// Step 1: 查询或创建
	var mastery model.StudentKPMastery
	result := s.db.Where("student_id = ? AND kp_id = ?", studentID, kpID).First(&mastery)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// 首次答题，创建记录
			mastery = model.StudentKPMastery{
				StudentID:    studentID,
				KPID:         kpID,
				MasteryScore: s.params.PL0, // 初始掌握概率
				AttemptCount: 0,
				CorrectCount: 0,
			}
		} else {
			return MasteryUpdate{}, result.Error
		}
	}

	// Step 2: BKT 更新
	oldMastery := mastery.MasteryScore
	newMastery := s.params.UpdateMastery(oldMastery, correct, evidence)

	// Step 3: 更新记录
	mastery.MasteryScore = newMastery
	mastery.AttemptCount++
	if correct {
		mastery.CorrectCount++
	}
	if evidence == EvidenceTest && correct {
		mastery.PassedTest = true
	}

	now := time.Now()
	mastery.LastAttemptAt = &now
	mastery.UpdatedAt = now

	if mastery.ID == 0 {
		if err := s.db.Create(&mastery).Error; err != nil {
			return MasteryUpdate{}, err
		}
	} else {
		if err := s.db.Save(&mastery).Error; err != nil {
			return MasteryUpdate{}, err
		}
	}

	slogBKT.Info("mastery updated",
		"studentID", studentID, "kpID", kpID, "correct", correct, "evidence", evidence,
		"oldMastery", oldMastery, "newMastery", newMastery, "attempts", mastery.AttemptCount)

	return MasteryUpdate{
		StudentID:    studentID,
		KPID:         kpID,
		Correct:      correct,
		NewMastery:   newMastery,
		AttemptCount: mastery.AttemptCount,
	}, nil
}

// GetMastery 查询学生对某知识点的当前掌握度。
func (s *BKTService) GetMastery(studentID, kpID uint) float64 {
	var mastery model.StudentKPMastery
	result := s.db.Where("student_id = ? AND kp_id = ?", studentID, kpID).First(&mastery)
	if result.Error != nil {
		return s.params.PL0 // 返回初始掌握概率
	}
	return mastery.MasteryScore
}

// GetStudentMasteries 批量查询学生对多个知识点的掌握度。
func (s *BKTService) GetStudentMasteries(studentID uint, kpIDs []uint) map[uint]float64 {
	var masteries []model.StudentKPMastery
	s.db.Where("student_id = ? AND kp_id IN ?", studentID, kpIDs).Find(&masteries)

	result := make(map[uint]float64, len(kpIDs))
	for _, m := range masteries {
		result[m.KPID] = m.MasteryScore
	}

	// 对没有记录的 KP 使用初始值
	for _, kpID := range kpIDs {
		if _, ok := result[kpID]; !ok {
			result[kpID] = s.params.PL0
		}
	}

	return result
}
