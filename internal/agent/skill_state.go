package agent

import (
	"strings"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogSkillState = logger.L("SkillState")

// SkillStateManager 管理技能会话状态（Quiz/Survey/RolePlay/Fallacy）。
type SkillStateManager struct {
	coach *CoachAgent
}

// NewSkillStateManager 创建技能状态管理器。
func NewSkillStateManager(coach *CoachAgent) *SkillStateManager {
	return &SkillStateManager{coach: coach}
}

// ── Fallacy Detective ──────────────────────────────────────

// AdvanceFallacyIfActive 当活跃技能为谬误侦探时，推进阶段。
func (s *SkillStateManager) AdvanceFallacyIfActive(tc *TurnContext, response *DraftResponse) {
	if response == nil || !isFallacyDetectiveActive(response.SkillID) {
		return
	}

	identified := false
	if tc.Review != nil {
		identified = tc.Review.Approved && tc.Review.DepthScore >= 0.6
	}

	stateBefore := s.coach.loadFallacyState(tc.SessionID)
	s.coach.AdvanceFallacyPhase(tc.SessionID, identified)

	if identified && stateBefore.Phase == FallacyPhaseAwaiting && tc.OnScaffold != nil {
		tc.OnScaffold("fallacy_identified", map[string]interface{}{
			"identified_count": stateBefore.IdentifiedCount + 1,
			"embedded_count":   stateBefore.EmbeddedCount,
			"max_per_session":  stateBefore.MaxPerSession,
			"trap_desc":        stateBefore.CurrentTrapDesc,
		})
	}
}

// ── Role-Play ──────────────────────────────────────────────

// UpdateRolePlayIfActive 当活跃技能为角色扮演时，更新角色状态。
func (s *SkillStateManager) UpdateRolePlayIfActive(tc *TurnContext, response *DraftResponse) {
	if response == nil || !isRolePlayActive(response.SkillID) {
		return
	}

	state := s.coach.loadRolePlayState(tc.SessionID)

	if state.CharacterName == "" {
		s.coach.saveRolePlayState(tc.SessionID, state)
		slogSkillState.Info("roleplay initial state saved", "session_id", tc.SessionID)
		return
	}

	slogSkillState.Info("roleplay state",
		"session_id", tc.SessionID, "character", state.CharacterName,
		"scenario_switches", state.ScenarioSwitches, "max_switches", state.MaxSwitches, "active", state.Active)
}

// ── Quiz ───────────────────────────────────────────────────

// AdvanceQuizIfActive 当活跃技能为自动出题时，推进阶段。
func (s *SkillStateManager) AdvanceQuizIfActive(tc *TurnContext, response *DraftResponse) {
	if response == nil || !isQuizActive(response.SkillID) {
		return
	}

	state := s.coach.loadQuizState(tc.SessionID)

	switch state.Phase {
	case QuizPhaseGenerating:
		if strings.Contains(response.Content, "<quiz>") {
			questionCount := strings.Count(response.Content, `"type"`)
			if questionCount == 0 {
				questionCount = 1
			}
			s.coach.AdvanceQuizPhase(tc.SessionID, questionCount, 0)

			if tc.OnScaffold != nil {
				tc.OnScaffold("quiz_questions", map[string]interface{}{
					"batch":          state.BatchCount + 1,
					"question_count": questionCount,
				})
			}
		}

	case QuizPhaseAnswering:
		s.coach.AdvanceQuizPhase(tc.SessionID, 0, 0)

	case QuizPhaseGrading:
		correctCount := 0
		if tc.Review != nil && tc.Review.DepthScore >= 0.5 {
			correctCount = 1
		}
		s.coach.AdvanceQuizPhase(tc.SessionID, 0, correctCount)

		if tc.OnScaffold != nil {
			tc.OnScaffold("quiz_result", map[string]interface{}{
				"correct_count":   state.CorrectCount + correctCount,
				"total_questions": state.TotalQuestions,
			})
		}

	case QuizPhaseReviewing:
		s.coach.AdvanceQuizPhase(tc.SessionID, 0, 0)
	}
}

// ── Learning Survey ────────────────────────────────────────

// AdvanceSurveyIfActive 当活跃技能为学情问卷时，推进阶段。
func (s *SkillStateManager) AdvanceSurveyIfActive(tc *TurnContext, response *DraftResponse) {
	if response == nil || !isSurveyActive(response.SkillID) {
		return
	}

	state := s.coach.loadSurveyState(tc.SessionID)

	switch state.Phase {
	case SurveyPhaseWelcome:
		if strings.Contains(response.Content, "<survey>") {
			dim := extractSurveyDimensionFromContent(response.Content)
			questionCount := strings.Count(response.Content, `"id"`)
			if questionCount == 0 {
				questionCount = 1
			}
			s.coach.AdvanceSurveyPhase(tc.SessionID, dim, questionCount)

			if tc.OnScaffold != nil {
				tc.OnScaffold("survey_questions", map[string]interface{}{
					"dimension":      dim,
					"question_count": questionCount,
					"phase":          "welcome",
				})
			}
		} else {
			s.coach.AdvanceSurveyPhase(tc.SessionID, "", 0)
		}

	case SurveyPhaseSurveying:
		if strings.Contains(response.Content, "<survey>") {
			dim := extractSurveyDimensionFromContent(response.Content)
			questionCount := strings.Count(response.Content, `"id"`)
			if questionCount == 0 {
				questionCount = 1
			}

			if state.CurrentDimension != "" && state.CurrentDimension != dim {
				s.coach.AdvanceSurveyPhase(tc.SessionID, state.CurrentDimension, questionCount)
			} else {
				s.coach.AdvanceSurveyPhase(tc.SessionID, dim, questionCount)
			}

			if tc.OnScaffold != nil {
				tc.OnScaffold("survey_questions", map[string]interface{}{
					"dimension":      dim,
					"question_count": questionCount,
					"completed_dims": len(state.CompletedDims),
					"total_dims":     state.TotalDimensions,
				})
			}
		} else {
			if state.CurrentDimension != "" {
				s.coach.AdvanceSurveyPhase(tc.SessionID, state.CurrentDimension, 0)
			}
		}

	case SurveyPhaseAnalyzing:
		if strings.Contains(response.Content, "<survey_profile>") {
			s.coach.AdvanceSurveyPhase(tc.SessionID, "", 0)
			s.coach.AdvanceSurveyPhase(tc.SessionID, "", 0)

			if tc.OnScaffold != nil {
				tc.OnScaffold("learning_profile", map[string]interface{}{
					"status": "generated",
				})
			}
		} else {
			s.coach.AdvanceSurveyPhase(tc.SessionID, "", 0)
		}

	case SurveyPhaseReporting:
		if strings.Contains(response.Content, "<survey_profile>") {
			s.coach.AdvanceSurveyPhase(tc.SessionID, "", 0)

			if tc.OnScaffold != nil {
				tc.OnScaffold("learning_profile", map[string]interface{}{
					"status": "generated",
				})
			}
		}

	case SurveyPhasePlanning:
		if strings.Contains(response.Content, "<learning_plan>") {
			s.coach.AdvanceSurveyPhase(tc.SessionID, "", 0)

			if tc.OnScaffold != nil {
				tc.OnScaffold("learning_plan", map[string]interface{}{
					"status": "generated",
				})
			}
		}
	}
}

// extractSurveyDimensionFromContent 从 <survey> JSON 中提取 dimension 字段。
func extractSurveyDimensionFromContent(content string) string {
	start := strings.Index(content, `"dimension"`)
	if start == -1 {
		return ""
	}
	start = strings.Index(content[start:], `"`) + start + 1
	start = strings.Index(content[start:], `"`) + start + 1
	end := strings.Index(content[start:], `"`)
	if end == -1 {
		return ""
	}
	return content[start : start+end]
}
