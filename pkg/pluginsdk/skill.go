package pluginsdk

import "context"

// -- Skill Plugin SDK -------------------------------------------

// StudentIntent represents parsed student query intent.
type StudentIntent struct {
	RawText     string  `json:"raw_text"`
	Category    string  `json:"category"`
	Confidence  float64 `json:"confidence"`
	KnowledgeID uint    `json:"knowledge_id,omitempty"`
}

// SkillEvalResult is the evaluation output from a skill plugin.
type SkillEvalResult struct {
	SkillID    string             `json:"skill_id"`
	Dimensions map[string]float64 `json:"dimensions"`
	Overall    float64            `json:"overall"`
	Feedback   string             `json:"feedback"`
}

// InteractionData represents a completed interaction for evaluation.
type InteractionData struct {
	SessionID    uint   `json:"session_id"`
	StudentInput string `json:"student_input"`
	CoachOutput  string `json:"coach_output"`
	SkillID      string `json:"skill_id"`
	Scaffold     string `json:"scaffold"`
}

// SkillConstraints holds the SKILL.md content.
type SkillConstraints struct {
	SkillID     string `json:"skill_id"`
	RawMarkdown string `json:"raw_markdown"`
}

// SkillTemplate holds a scoring rubric or prompt template.
type SkillTemplate struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	Content  string `json:"content"`
}

// SkillPlugin is the interface for teaching skill plugins.
type SkillPlugin interface {
	Plugin

	// Match returns a confidence score [0.0, 1.0] for intent matching.
	Match(ctx context.Context, intent StudentIntent) (float64, error)

	// LoadConstraints loads the skill's constraint instructions.
	LoadConstraints(ctx context.Context) (*SkillConstraints, error)

	// LoadTemplates loads scoring rubrics and prompt templates.
	LoadTemplates(ctx context.Context, templateIDs []string) ([]SkillTemplate, error)

	// Evaluate assesses the quality of an interaction.
	Evaluate(ctx context.Context, interaction InteractionData) (*SkillEvalResult, error)
}
