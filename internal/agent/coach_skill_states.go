package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogCoachSkill = logger.L("CoachSkill")

// ── Skill State Management (moved from coach.go) ──────────

func isFallacyDetectiveActive(skillID string) bool {
	return fallacyDetectiveIDs[skillID]
}

// loadFallacyState 从 StudentSession.SkillState 加载谬误侦探会话状态。
// 如果不存在或解析失败，返回初始状态。
func (a *CoachAgent) loadFallacyState(sessionID uint) FallacySessionState {
	var session model.StudentSession
	if err := a.db.Select("skill_state").First(&session, sessionID).Error; err != nil {
		slogCoach.Warn("load fallacy state failed", "session_id", sessionID, "err", err)
		return defaultFallacyState()
	}

	if session.SkillState == nil || *session.SkillState == "" || *session.SkillState == "null" {
		return defaultFallacyState()
	}

	var state FallacySessionState
	if err := json.Unmarshal([]byte(*session.SkillState), &state); err != nil {
		slogCoach.Warn("parse fallacy state failed", "session_id", sessionID, "err", err)
		return defaultFallacyState()
	}

	return state
}

// saveFallacyState 将谬误侦探会话状态保存到 StudentSession.SkillState。
func (a *CoachAgent) saveFallacyState(sessionID uint, state FallacySessionState) {
	data, err := json.Marshal(state)
	if err != nil {
		slogCoach.Warn("marshal fallacy state failed", "err", err)
		return
	}
	stateStr := string(data)
	if err := a.db.Model(&model.StudentSession{}).Where("id = ?", sessionID).
		Update("skill_state", stateStr).Error; err != nil {
		slogCoach.Warn("save fallacy state failed", "session_id", sessionID, "err", err)
	}
}

// defaultFallacyState 返回谬误侦探的初始会话状态。
func defaultFallacyState() FallacySessionState {
	return FallacySessionState{
		EmbeddedCount:   0,
		IdentifiedCount: 0,
		Phase:           FallacyPhasePresentTrap,
		MaxPerSession:   3, // 默认值，来自 metadata.json constraints.max_embedded_fallacies_per_session
	}
}

// buildFallacyContext 构建谬误侦探技能的额外系统上下文。
// 告知 LLM 当前的谬误嵌入进度和学生识别状态，使 LLM 能够正确推进流程。
func buildFallacyContext(state FallacySessionState, misconceptions []MisconceptionItem) string {
	var sb strings.Builder
	sb.WriteString("\n【谬误侦探会话状态】\n")
	sb.WriteString(fmt.Sprintf("- 当前阶段: %s\n", fallacyPhaseLabel(state.Phase)))
	sb.WriteString(fmt.Sprintf("- 已嵌入谬误数: %d / %d\n", state.EmbeddedCount, state.MaxPerSession))
	sb.WriteString(fmt.Sprintf("- 学生已正确识别: %d\n", state.IdentifiedCount))

	if state.CurrentTrapDesc != "" {
		sb.WriteString(fmt.Sprintf("- 当前嵌入的谬误: %s\n", state.CurrentTrapDesc))
	}

	// 阶段指令
	switch state.Phase {
	case FallacyPhasePresentTrap:
		if state.EmbeddedCount >= state.MaxPerSession {
			sb.WriteString("\n注意：本会话已达到最大谬误数，不要再嵌入新的谬误。直接进行总结。\n")
		} else {
			sb.WriteString("\n指令：请在接下来的讲解中巧妙嵌入一个学科常见误区。" +
				"嵌入后，系统将进入等待学生识别阶段。\n")
		}
	case FallacyPhaseAwaiting:
		sb.WriteString("\n指令：学生正在尝试识别谬误。" +
			"评估学生的回答是否准确定位了谬误。" +
			"如果学生正确识别，进入揭示阶段。" +
			"如果学生未能识别，根据支架等级给予适当提示，但不要直接揭露答案。\n")
	case FallacyPhaseRevealed:
		sb.WriteString("\n指令：学生已识别谬误。" +
			"请揭示这个谬误的设计意图，解释为什么它是一个常见误区，" +
			"以及在真实考试中它可能以什么形式出现。" +
			"揭示完成后，如果未达到最大谬误数，准备嵌入下一个谬误。\n")
	}

	return sb.String()
}

// fallacyPhaseLabel 将阶段枚举转换为中文标签。
func fallacyPhaseLabel(phase FallacyPhase) string {
	switch phase {
	case FallacyPhasePresentTrap:
		return "展示陷阱"
	case FallacyPhaseAwaiting:
		return "等待识别"
	case FallacyPhaseRevealed:
		return "已揭示"
	default:
		return string(phase)
	}
}

// advanceFallacyPhase 根据交互结果推进谬误侦探的阶段状态。
// 在 orchestrator 的 HandleTurn 完成后调用。
//
// 状态转换:
//
//	present_trap → awaiting  (Coach 输出含谬误的讲解后)
//	awaiting     → revealed  (学生正确识别后)
//	awaiting     → awaiting  (学生未能识别，保持等待)
//	revealed     → present_trap (准备下一个谬误)
func (a *CoachAgent) AdvanceFallacyPhase(sessionID uint, studentIdentified bool) {
	state := a.loadFallacyState(sessionID)

	switch state.Phase {
	case FallacyPhasePresentTrap:
		// Coach 刚输出了含谬误的讲解 → 进入等待识别
		state.EmbeddedCount++
		state.Phase = FallacyPhaseAwaiting
		slogCoach.Info("trap presented, awaiting identification",
			"session_id", sessionID, "embedded", state.EmbeddedCount, "max", state.MaxPerSession)

	case FallacyPhaseAwaiting:
		if studentIdentified {
			// 学生正确识别 → 进入揭示阶段
			state.IdentifiedCount++
			state.Phase = FallacyPhaseRevealed
			slogCoach.Info("student identified trap",
				"session_id", sessionID, "identified", state.IdentifiedCount, "embedded", state.EmbeddedCount)
		} else {
			slogCoach.Info("student did not identify, staying in awaiting",
				"session_id", sessionID)
		}

	case FallacyPhaseRevealed:
		// 揭示完成 → 回到展示陷阱（如果还有配额）
		state.CurrentTrapDesc = ""
		if state.EmbeddedCount < state.MaxPerSession {
			state.Phase = FallacyPhasePresentTrap
			slogCoach.Info("reveal complete, ready for next trap", "session_id", sessionID)
		} else {
			slogCoach.Info("all traps completed",
				"session_id", sessionID, "identified", state.IdentifiedCount, "embedded", state.EmbeddedCount)
		}
	}

	a.saveFallacyState(sessionID, state)
}

// ── Role-Play Session State ────────────────────────────────

// rolePlayIDs lists all valid skill IDs for the role-play skill.
var rolePlayIDs = map[string]bool{
	"general_review_roleplay": true,
	"role-play":               true, // backward compat
}

// isRolePlayActive 判断当前技能是否为角色扮演。
func isRolePlayActive(skillID string) bool {
	return rolePlayIDs[skillID]
}

// loadRolePlayState 从 StudentSession.SkillState 加载角色扮演会话状态。
// 如果不存在或解析失败，返回初始状态。
func (a *CoachAgent) loadRolePlayState(sessionID uint) RolePlaySessionState {
	var session model.StudentSession
	if err := a.db.Select("skill_state").First(&session, sessionID).Error; err != nil {
		slogCoach.Warn("load role-play state failed", "session_id", sessionID, "err", err)
		return defaultRolePlayState()
	}

	if session.SkillState == nil || *session.SkillState == "" || *session.SkillState == "null" {
		return defaultRolePlayState()
	}

	var state RolePlaySessionState
	if err := json.Unmarshal([]byte(*session.SkillState), &state); err != nil {
		slogCoach.Warn("parse role-play state failed", "session_id", sessionID, "err", err)
		return defaultRolePlayState()
	}

	return state
}

// saveRolePlayState 将角色扮演会话状态保存到 StudentSession.SkillState。
func (a *CoachAgent) saveRolePlayState(sessionID uint, state RolePlaySessionState) {
	data, err := json.Marshal(state)
	if err != nil {
		slogCoach.Warn("marshal role-play state failed", "err", err)
		return
	}
	stateStr := string(data)
	if err := a.db.Model(&model.StudentSession{}).Where("id = ?", sessionID).
		Update("skill_state", stateStr).Error; err != nil {
		slogCoach.Warn("save role-play state failed", "session_id", sessionID, "err", err)
	}
}

// defaultRolePlayState 返回角色扮演的初始会话状态。
func defaultRolePlayState() RolePlaySessionState {
	return RolePlaySessionState{
		ScenarioSwitches: 0,
		MaxSwitches:      3, // 来自 metadata.json constraints.max_scenario_switches_per_session
		Active:           true,
	}
}

// buildRolePlayContext 构建角色扮演技能的额外系统上下文。
// 告知 LLM 当前的角色身份和情境状态，使 LLM 能够维持角色一致性。
func buildRolePlayContext(state RolePlaySessionState) string {
	var sb strings.Builder
	sb.WriteString("\n【角色扮演会话状态】\n")

	if state.CharacterName != "" {
		sb.WriteString(fmt.Sprintf("- 当前角色: %s（%s）\n", state.CharacterName, state.CharacterRole))
	} else {
		sb.WriteString("- 当前角色: 尚未选定（请根据学科和知识点选择一个合适的角色）\n")
	}

	if state.ScenarioDesc != "" {
		sb.WriteString(fmt.Sprintf("- 当前情境: %s\n", state.ScenarioDesc))
	}

	sb.WriteString(fmt.Sprintf("- 已切换情境: %d / %d 次\n", state.ScenarioSwitches, state.MaxSwitches))
	sb.WriteString(fmt.Sprintf("- 角色状态: %s\n", rolePlayActiveLabel(state.Active)))

	// 状态指令
	if !state.Active {
		sb.WriteString("\n指令：学生已请求退出角色扮演。请以角色身份做简短告别，" +
			"然后切换回导师视角，总结本次扮演中涉及的知识点和学生表现亮点。\n")
	} else if state.CharacterName == "" {
		sb.WriteString("\n指令：这是角色扮演的第一轮。请根据当前学科和知识点，" +
			"选择一个合适的角色身份，简要介绍自己并设定情境，然后以角色视角展开对话。\n")
	} else if state.ScenarioSwitches >= state.MaxSwitches {
		sb.WriteString("\n注意：本会话已达到最大情境切换次数，请保持当前角色和情境继续对话。\n")
	} else {
		sb.WriteString("\n指令：请继续以当前角色身份与学生对话，" +
			"在对话中自然融入知识点。保持角色一致性。\n")
	}

	return sb.String()
}

// rolePlayActiveLabel 将活跃状态转换为中文标签。
func rolePlayActiveLabel(active bool) string {
	if active {
		return "沉浸中"
	}
	return "已退出"
}

// ── Quiz Generation Session State (§7.13) ───────────────────

// quizIDs lists all valid skill IDs for the quiz-generation skill.
var quizIDs = map[string]bool{
	"general_assessment_quiz": true,
	"quiz-generation":         true, // backward compat
}

// isQuizActive 判断当前技能是否为自动出题。
func isQuizActive(skillID string) bool {
	return quizIDs[skillID]
}

// loadQuizState 从 StudentSession.SkillState 加载自动出题会话状态。
// 如果不存在或解析失败，返回初始状态。
func (a *CoachAgent) loadQuizState(sessionID uint) QuizSessionState {
	var session model.StudentSession
	if err := a.db.Select("skill_state").First(&session, sessionID).Error; err != nil {
		slogCoach.Warn("load quiz state failed", "session_id", sessionID, "err", err)
		return defaultQuizState()
	}

	if session.SkillState == nil || *session.SkillState == "" || *session.SkillState == "null" {
		return defaultQuizState()
	}

	var state QuizSessionState
	if err := json.Unmarshal([]byte(*session.SkillState), &state); err != nil {
		slogCoach.Warn("parse quiz state failed", "session_id", sessionID, "err", err)
		return defaultQuizState()
	}

	return state
}

// saveQuizState 将自动出题会话状态保存到 StudentSession.SkillState。
func (a *CoachAgent) saveQuizState(sessionID uint, state QuizSessionState) {
	data, err := json.Marshal(state)
	if err != nil {
		slogCoach.Warn("marshal quiz state failed", "err", err)
		return
	}
	stateStr := string(data)
	if err := a.db.Model(&model.StudentSession{}).Where("id = ?", sessionID).
		Update("skill_state", stateStr).Error; err != nil {
		slogCoach.Warn("save quiz state failed", "session_id", sessionID, "err", err)
	}
}

// defaultQuizState 返回自动出题的初始会话状态。
func defaultQuizState() QuizSessionState {
	return QuizSessionState{
		Phase:       QuizPhaseGenerating,
		BatchCount:  0,
		MaxPerBatch: 5, // 来自 metadata.json constraints.max_questions_per_batch
	}
}

// buildQuizContext 构建自动出题技能的额外系统上下文。
// 告知 LLM 当前的出题进度和阶段，使 LLM 能够正确推进流程。
func buildQuizContext(state QuizSessionState) string {
	var sb strings.Builder
	sb.WriteString("\n【自动出题会话状态】\n")
	sb.WriteString(fmt.Sprintf("- 当前阶段: %s\n", quizPhaseLabel(state.Phase)))
	sb.WriteString(fmt.Sprintf("- 已生成批次: %d\n", state.BatchCount))
	sb.WriteString(fmt.Sprintf("- 累计题目数: %d\n", state.TotalQuestions))
	sb.WriteString(fmt.Sprintf("- 累计答对数: %d\n", state.CorrectCount))
	if state.TotalQuestions > 0 {
		accuracy := float64(state.CorrectCount) / float64(state.TotalQuestions) * 100
		sb.WriteString(fmt.Sprintf("- 正确率: %.0f%%\n", accuracy))
	}

	// 阶段指令
	switch state.Phase {
	case QuizPhaseGenerating:
		sb.WriteString(fmt.Sprintf("\n指令：请根据当前知识点和学生掌握度，生成一批题目（最多 %d 道）。\n", state.MaxPerBatch))
		sb.WriteString("题目必须以 <quiz>JSON</quiz> 格式输出，包含 mcq_single、mcq_multiple 或 fill_blank 类型。\n")
		sb.WriteString("在 JSON 之前，可以简短地介绍本次测验的主题。\n")
	case QuizPhaseAnswering:
		sb.WriteString("\n指令：学生正在作答。等待学生提交答案。\n")
		sb.WriteString("如果学生提问或请求提示，根据支架等级给予适当引导，但不要透露答案。\n")
	case QuizPhaseGrading:
		sb.WriteString("\n指令：请根据学生提交的答案逐题批改。\n")
		sb.WriteString("对每道题标注正误，对错误的题目解释原因，对正确的给予肯定。\n")
		sb.WriteString("最后汇总得分并给出学习建议。\n")
	case QuizPhaseReviewing:
		sb.WriteString("\n指令：批改已完成。如果学生要求继续出题，可以生成新一批题目。\n")
		sb.WriteString("如果学生有疑问，详细解答。\n")
	}

	return sb.String()
}

// quizPhaseLabel 将阶段枚举转换为中文标签。
func quizPhaseLabel(phase QuizPhase) string {
	switch phase {
	case QuizPhaseGenerating:
		return "生成题目"
	case QuizPhaseAnswering:
		return "等待作答"
	case QuizPhaseGrading:
		return "批改中"
	case QuizPhaseReviewing:
		return "查看结果"
	default:
		return string(phase)
	}
}

// AdvanceQuizPhase 根据交互结果推进自动出题的阶段状态。
// 在 orchestrator 的 HandleTurn 完成后调用。
//
// 状态转换:
//
//	generating → answering  (Coach 输出含题目的回复后)
//	answering  → grading    (学生提交答案后)
//	grading    → reviewing  (批改完成后)
//	reviewing  → generating (学生请求继续出题)
func (a *CoachAgent) AdvanceQuizPhase(sessionID uint, questionsGenerated, correctAnswers int) {
	state := a.loadQuizState(sessionID)

	switch state.Phase {
	case QuizPhaseGenerating:
		// Coach 输出了题目 → 进入等待作答
		if questionsGenerated > 0 {
			state.BatchCount++
			state.TotalQuestions += questionsGenerated
			state.Phase = QuizPhaseAnswering
			slogCoach.Info("questions generated, awaiting answers",
				"session_id", sessionID, "questions", questionsGenerated, "batch", state.BatchCount)
		}

	case QuizPhaseAnswering:
		// 学生提交答案 → 进入批改
		state.Phase = QuizPhaseGrading
		slogCoach.Info("student submitted answers, grading", "session_id", sessionID)

	case QuizPhaseGrading:
		// 批改完成 → 进入查看结果
		state.CorrectCount += correctAnswers
		state.Phase = QuizPhaseReviewing
		slogCoach.Info("grading complete",
			"session_id", sessionID, "correct", correctAnswers, "total_correct", state.CorrectCount, "total_questions", state.TotalQuestions)

	case QuizPhaseReviewing:
		// 学生请求继续 → 回到生成阶段
		state.Phase = QuizPhaseGenerating
		slogCoach.Info("student requests more questions", "session_id", sessionID)
	}

	a.saveQuizState(sessionID, state)
}

// ── Learning Survey Session State ───────────────────────────

// surveyIDs lists all valid skill IDs for the learning-survey skill.
var surveyIDs = map[string]bool{
	"general_diagnosis_survey": true,
	"learning-survey":          true, // backward compat
}

// isSurveyActive 判断当前技能是否为学情问卷诊断。
func isSurveyActive(skillID string) bool {
	return surveyIDs[skillID]
}

// loadSurveyState 从 StudentSession.SkillState 加载学情问卷会话状态。
// 如果不存在或解析失败，返回初始状态。
func (a *CoachAgent) loadSurveyState(sessionID uint) SurveySessionState {
	var session model.StudentSession
	if err := a.db.Select("skill_state").First(&session, sessionID).Error; err != nil {
		slogCoach.Warn("load survey state failed", "session_id", sessionID, "err", err)
		return defaultSurveyState()
	}

	if session.SkillState == nil || *session.SkillState == "" || *session.SkillState == "null" {
		return defaultSurveyState()
	}

	var state SurveySessionState
	if err := json.Unmarshal([]byte(*session.SkillState), &state); err != nil {
		slogCoach.Warn("parse survey state failed", "session_id", sessionID, "err", err)
		return defaultSurveyState()
	}

	return state
}

// saveSurveyState 将学情问卷会话状态保存到 StudentSession.SkillState。
func (a *CoachAgent) saveSurveyState(sessionID uint, state SurveySessionState) {
	data, err := json.Marshal(state)
	if err != nil {
		slogCoach.Warn("marshal survey state failed", "err", err)
		return
	}
	stateStr := string(data)
	if err := a.db.Model(&model.StudentSession{}).Where("id = ?", sessionID).
		Update("skill_state", stateStr).Error; err != nil {
		slogCoach.Warn("save survey state failed", "session_id", sessionID, "err", err)
	}
}

// defaultSurveyState 返回学情问卷的初始会话状态。
func defaultSurveyState() SurveySessionState {
	return SurveySessionState{
		Phase:           SurveyPhaseWelcome,
		CompletedDims:   []string{},
		TotalDimensions: 6, // learning_style, prior_knowledge, motivation, self_efficacy, study_habits, subject_interest
	}
}

// buildSurveyContext 构建学情问卷诊断技能的额外系统上下文。
// 告知 LLM 当前的问卷进度和阶段，使 LLM 能够正确推进流程。
func buildSurveyContext(state SurveySessionState) string {
	var sb strings.Builder
	sb.WriteString("\n【学情问卷会话状态】\n")
	sb.WriteString(fmt.Sprintf("- 当前阶段: %s\n", surveyPhaseLabel(state.Phase)))
	sb.WriteString(fmt.Sprintf("- 已完成维度: %d / %d\n", len(state.CompletedDims), state.TotalDimensions))
	if len(state.CompletedDims) > 0 {
		sb.WriteString(fmt.Sprintf("- 已完成: %s\n", strings.Join(state.CompletedDims, ", ")))
	}
	if state.CurrentDimension != "" {
		sb.WriteString(fmt.Sprintf("- 当前维度: %s\n", surveyDimensionLabel(state.CurrentDimension)))
	}
	sb.WriteString(fmt.Sprintf("- 累计提问数: %d\n", state.TotalQuestions))
	sb.WriteString(fmt.Sprintf("- 累计回答数: %d\n", state.TotalAnswered))

	// 阶段指令
	switch state.Phase {
	case SurveyPhaseWelcome:
		sb.WriteString("\n指令：这是问卷的开始。请简短自我介绍，说明问卷的目的，" +
			"消除学生的紧张感，强调没有对错之分。\n" +
			"然后推送第一个维度（learning_style）的问卷题目。\n")
	case SurveyPhaseSurveying:
		remaining := allSurveyDimensions(state.CompletedDims)
		if len(remaining) > 0 {
			sb.WriteString(fmt.Sprintf("\n指令：请对学生的上一批回答做简短反馈，"+
				"然后推送下一个维度（%s）的问卷题目。\n", remaining[0]))
			sb.WriteString("问题以 <survey>JSON</survey> 格式输出。\n")
			sb.WriteString(fmt.Sprintf("待完成维度: %s\n", strings.Join(remaining, ", ")))
		} else {
			sb.WriteString("\n指令：所有维度的问卷已完成。请告知学生问卷结束，" +
				"进入分析阶段。\n")
		}
	case SurveyPhaseAnalyzing:
		sb.WriteString("\n指令：所有问卷回答已收集完毕。请汇总分析学生的回答，" +
			"告知学生正在为其生成学习画像。\n")
	case SurveyPhaseReporting:
		sb.WriteString("\n指令：请以 <survey_profile>JSON</survey_profile> 格式输出完整的学习画像。\n" +
			"然后用通俗易懂的语言向学生解释每个维度的诊断结果。\n" +
			"强调优势，对薄弱点提出积极的改进建议。\n")
	case SurveyPhasePlanning:
		sb.WriteString("\n指令：请基于学习画像，以 <learning_plan>JSON</learning_plan> 格式输出学习建议方案。\n" +
			"推荐适合学生的学习策略和技能，给出具体可行的学习建议。\n")
	}

	return sb.String()
}

// surveyPhaseLabel 将阶段枚举转换为中文标签。
func surveyPhaseLabel(phase SurveyPhase) string {
	switch phase {
	case SurveyPhaseWelcome:
		return "欢迎介绍"
	case SurveyPhaseSurveying:
		return "问卷进行中"
	case SurveyPhaseAnalyzing:
		return "分析中"
	case SurveyPhaseReporting:
		return "生成画像"
	case SurveyPhasePlanning:
		return "制定方案"
	default:
		return string(phase)
	}
}

// surveyDimensionLabel 将维度 ID 转换为中文标签。
func surveyDimensionLabel(dim string) string {
	labels := map[string]string{
		"learning_style":   "学习风格",
		"prior_knowledge":  "前置知识",
		"motivation":       "学习动机",
		"self_efficacy":    "自我效能感",
		"study_habits":     "学习习惯",
		"subject_interest": "学科兴趣",
	}
	if label, ok := labels[dim]; ok {
		return label
	}
	return dim
}

// allSurveyDimensions 返回尚未完成的诊断维度列表（按预定顺序）。
func allSurveyDimensions(completed []string) []string {
	allDims := []string{
		"learning_style", "prior_knowledge", "motivation",
		"self_efficacy", "study_habits", "subject_interest",
	}
	done := make(map[string]bool, len(completed))
	for _, d := range completed {
		done[d] = true
	}
	remaining := make([]string, 0)
	for _, d := range allDims {
		if !done[d] {
			remaining = append(remaining, d)
		}
	}
	return remaining
}

// AdvanceSurveyPhase 根据交互结果推进学情问卷的阶段状态。
// 在 orchestrator 的 HandleTurn 完成后调用。
//
// 状态转换:
//
//	welcome    → surveying  (欢迎完成，开始第一个维度)
//	surveying  → surveying  (完成一个维度，继续下一个)
//	surveying  → analyzing  (所有维度完成)
//	analyzing  → reporting  (分析完成，生成画像)
//	reporting  → planning   (画像生成完成，制定方案)
//	planning   → planning   (方案已生成，保持)
func (a *CoachAgent) AdvanceSurveyPhase(sessionID uint, completedDimension string, questionsInBatch int) {
	state := a.loadSurveyState(sessionID)

	switch state.Phase {
	case SurveyPhaseWelcome:
		// 欢迎完成 → 进入问卷阶段
		state.Phase = SurveyPhaseSurveying
		if completedDimension != "" {
			state.CurrentDimension = completedDimension
		}
		if questionsInBatch > 0 {
			state.TotalQuestions += questionsInBatch
		}
		slogCoach.Info("survey welcome complete, starting survey",
			"session_id", sessionID)

	case SurveyPhaseSurveying:
		// 完成当前维度
		if completedDimension != "" {
			state.CompletedDims = appendUnique(state.CompletedDims, completedDimension)
			state.TotalAnswered += questionsInBatch
		}

		remaining := allSurveyDimensions(state.CompletedDims)
		if len(remaining) == 0 {
			// 所有维度完成 → 进入分析阶段
			state.Phase = SurveyPhaseAnalyzing
			state.CurrentDimension = ""
			slogCoach.Info("all survey dimensions complete, analyzing",
				"session_id", sessionID, "completed", len(state.CompletedDims))
		} else {
			// 还有维度未完成 → 继续下一个
			state.CurrentDimension = remaining[0]
			if questionsInBatch > 0 {
				state.TotalQuestions += questionsInBatch
			}
			slogCoach.Info("survey dimension complete, next dimension",
				"session_id", sessionID, "completed_dim", completedDimension, "next", remaining[0])
		}

	case SurveyPhaseAnalyzing:
		// 分析完成 → 进入报告阶段
		state.Phase = SurveyPhaseReporting
		slogCoach.Info("survey analysis complete, generating profile",
			"session_id", sessionID)

	case SurveyPhaseReporting:
		// 画像生成完成 → 进入规划阶段
		state.ProfileGenerated = true
		state.Phase = SurveyPhasePlanning
		slogCoach.Info("survey profile generated, planning",
			"session_id", sessionID)

	case SurveyPhasePlanning:
		// 方案已生成
		state.PlanGenerated = true
		slogCoach.Info("survey plan generated",
			"session_id", sessionID)
	}

	a.saveSurveyState(sessionID, state)
}

// appendUnique 向切片中追加不重复的元素。
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
