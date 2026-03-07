package agent

import (
	"math"
	"testing"
)

// ── DefaultBKTParams Tests ──────────────────────────────────

func TestDefaultBKTParams(t *testing.T) {
	p := DefaultBKTParams()

	if p.PL0 != 0.1 {
		t.Errorf("PL0 = %f, want 0.1", p.PL0)
	}
	if p.PT != 0.3 {
		t.Errorf("PT = %f, want 0.3", p.PT)
	}
	if p.PG != 0.2 {
		t.Errorf("PG = %f, want 0.2", p.PG)
	}
	if p.PS != 0.1 {
		t.Errorf("PS = %f, want 0.1", p.PS)
	}
}

// ── UpdateMastery Tests ─────────────────────────────────────

func TestUpdateMastery_CorrectAnswer(t *testing.T) {
	p := DefaultBKTParams()

	tests := []struct {
		name         string
		priorMastery float64
		wantHigher   bool // mastery should increase after correct answer
	}{
		{"从初始值答对", 0.1, true},
		{"从中等掌握度答对", 0.5, true},
		{"从高掌握度答对", 0.8, true},
		{"从极低掌握度答对", 0.01, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := p.UpdateMastery(tc.priorMastery, true, EvidenceTest)
			if tc.wantHigher && result <= tc.priorMastery {
				t.Errorf("UpdateMastery(%f, true, EvidenceTest) = %f, expected > %f",
					tc.priorMastery, result, tc.priorMastery)
			}
		})
	}
}

func TestUpdateMastery_IncorrectAnswer(t *testing.T) {
	p := DefaultBKTParams()

	// 答错后掌握度仍然不会下降太多（因为有学习转移概率 PT）
	// 但后验概率应该下降
	tests := []struct {
		name         string
		priorMastery float64
	}{
		{"从高掌握度答错", 0.9},
		{"从中等掌握度答错", 0.5},
		{"从低掌握度答错", 0.2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := p.UpdateMastery(tc.priorMastery, false, EvidenceTest)
			// 结果应在 [0, 1] 范围内
			if result < 0.0 || result > 1.0 {
				t.Errorf("UpdateMastery(%f, false, EvidenceTest) = %f, out of [0, 1] range",
					tc.priorMastery, result)
			}
		})
	}
}

func TestUpdateMastery_Clamp(t *testing.T) {
	// 测试极端参数下 clamp 是否生效
	p := BKTParams{PL0: 0.1, PT: 1.0, PG: 0.0, PS: 0.0}

	// PT=1.0 意味着 100% 学习转移，mastery 应为 1.0
	result := p.UpdateMastery(0.5, true, EvidenceTest)
	if result != 1.0 {
		t.Errorf("With PT=1.0, UpdateMastery(0.5, true, EvidenceTest) = %f, want 1.0", result)
	}
}

func TestUpdateMastery_MonotonicallyIncreases_ConsecutiveCorrect(t *testing.T) {
	p := DefaultBKTParams()
	mastery := 0.1

	// 连续 10 次答对，掌握度应单调递增
	for i := 0; i < 10; i++ {
		newMastery := p.UpdateMastery(mastery, true, EvidenceTest)
		if newMastery < mastery {
			t.Errorf("第 %d 次答对后掌握度下降: %f → %f", i+1, mastery, newMastery)
		}
		mastery = newMastery
	}

	// 经过 10 次答对，掌握度应较高
	if mastery < 0.8 {
		t.Errorf("10 次答对后掌握度 = %f, 期望 >= 0.8", mastery)
	}
}

func TestUpdateMastery_CorrectVsIncorrect_Difference(t *testing.T) {
	p := DefaultBKTParams()
	prior := 0.5

	correctResult := p.UpdateMastery(prior, true, EvidenceTest)
	incorrectResult := p.UpdateMastery(prior, false, EvidenceTest)

	// 答对的掌握度应高于答错的
	if correctResult <= incorrectResult {
		t.Errorf("答对后掌握度 (%f) 应高于答错后 (%f)", correctResult, incorrectResult)
	}
}

func TestUpdateMastery_BoundaryValues(t *testing.T) {
	p := DefaultBKTParams()

	tests := []struct {
		name         string
		priorMastery float64
		correct      bool
	}{
		{"先验为0答对", 0.0, true},
		{"先验为0答错", 0.0, false},
		{"先验为1答对", 1.0, true},
		{"先验为1答错", 1.0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := p.UpdateMastery(tc.priorMastery, tc.correct, EvidenceTest)
			if result < 0.0 || result > 1.0 {
				t.Errorf("UpdateMastery(%f, %t, EvidenceTest) = %f, out of [0, 1]",
					tc.priorMastery, tc.correct, result)
			}
			if math.IsNaN(result) || math.IsInf(result, 0) {
				t.Errorf("UpdateMastery(%f, %t, EvidenceTest) = %f, got NaN or Inf",
					tc.priorMastery, tc.correct, result)
			}
		})
	}
}

func TestUpdateMastery_BayesianPosterior_KnownValues(t *testing.T) {
	// 使用 EvidenceTest 参数手工验证贝叶斯后验公式
	// EvidenceTest 下: P(G)=0.05, P(S)=0.05. P(T)依然为默认 0.3
	// prior=0.5, correct=true
	//   pCorrectGivenMastered = 1-0.05 = 0.95
	//   pCorrectGivenNotMastered = 0.05
	//   pCorrect = 0.5*0.95 + 0.5*0.05 = 0.5
	//   posterior = 0.5*0.95/0.5 = 0.95
	//   mastery = 0.95 + (1-0.95)*0.3 = 0.965
	p := DefaultBKTParams()
	result := p.UpdateMastery(0.5, true, EvidenceTest)
	expected := 0.965

	if math.Abs(result-expected) > 0.001 {
		t.Errorf("UpdateMastery(0.5, true, EvidenceTest) = %f, want ≈ %f", result, expected)
	}
}
