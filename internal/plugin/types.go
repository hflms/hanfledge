package plugin

import "context"

// ============================
// Plugin System Type Definitions
// ============================
//
// Reference: design.md section 7.3 - Plugin Interface Contracts

// -- Plugin Lifecycle Interface ----------------------------------

// PluginState represents the lifecycle state of a plugin.
type PluginState string

const (
	PluginStateDiscovered  PluginState = "discovered"
	PluginStateValidated   PluginState = "validated"
	PluginStateInitialized PluginState = "initialized"
	PluginStateRunning     PluginState = "running"
	PluginStateDegraded    PluginState = "degraded"
	PluginStateStopped     PluginState = "stopped"
)

// TrustLevel defines the isolation tier for a plugin.
type TrustLevel string

const (
	TrustCore      TrustLevel = "core"      // In-process, full access
	TrustDomain    TrustLevel = "domain"    // In-process, read-only deps
	TrustCommunity TrustLevel = "community" // Sandboxed / separate process
)

// PluginType categorizes the plugin.
type PluginType string

const (
	PluginTypeSkill        PluginType = "skill"
	PluginTypeLLM          PluginType = "llm"
	PluginTypeStorage      PluginType = "storage"
	PluginTypeAuth         PluginType = "auth"
	PluginTypeLMS          PluginType = "lms"
	PluginTypeNotification PluginType = "notification"
	PluginTypeEditor       PluginType = "editor"
	PluginTypeTheme        PluginType = "theme"
)

// HealthStatus reports plugin health.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// PluginDeps are injected into plugins during Init.
type PluginDeps struct {
	EventBus *EventBus
	// Future: Logger, ConfigStore, KnowledgeAccessor, UserContextReader
}

// Plugin defines the lifecycle contract shared by all plugins.
type Plugin interface {
	// PluginMetadata returns the plugin's identity and configuration.
	PluginMetadata() PluginMeta

	// Init initializes the plugin with injected dependencies.
	Init(ctx context.Context, deps PluginDeps) error

	// HealthCheck returns the plugin's current health status.
	HealthCheck(ctx context.Context) HealthStatus

	// Shutdown gracefully shuts down the plugin and releases resources.
	Shutdown(ctx context.Context) error
}

// PluginMeta is the programmatic metadata returned by Plugin.PluginMetadata().
type PluginMeta struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Version    string      `json:"version"`
	Type       PluginType  `json:"type"`
	TrustLevel TrustLevel  `json:"trust_level"`
	Hooks      []HookPoint `json:"hooks,omitempty"`
}

// -- Skill Plugin Interface --------------------------------------

// StudentIntent represents the parsed intent from a student's query.
type StudentIntent struct {
	RawText     string  `json:"raw_text"`
	Category    string  `json:"category"` // e.g., "concept_confusion", "problem_solving"
	Confidence  float64 `json:"confidence"`
	KnowledgeID uint    `json:"knowledge_id,omitempty"`
}

// SkillEvalResult is the evaluation output from a skill plugin.
type SkillEvalResult struct {
	SkillID    string             `json:"skill_id"`
	Dimensions map[string]float64 `json:"dimensions"` // e.g., {"leakage": 0.1, "depth": 0.8}
	Overall    float64            `json:"overall"`
	Feedback   string             `json:"feedback"`
}

// InteractionData represents a completed student-AI interaction for evaluation.
type InteractionData struct {
	SessionID    uint   `json:"session_id"`
	StudentInput string `json:"student_input"`
	CoachOutput  string `json:"coach_output"`
	SkillID      string `json:"skill_id"`
	Scaffold     string `json:"scaffold"`
}

// SkillPlugin defines the standard contract for teaching skill plugins.
// Extends Plugin with skill-specific methods.
type SkillPlugin interface {
	Plugin

	// Match determines whether the student's query intent matches this skill.
	// Returns a confidence score [0.0, 1.0].
	Match(ctx context.Context, intent StudentIntent) (float64, error)

	// LoadConstraints loads the skill's SKILL.md constraint instructions on demand.
	LoadConstraints(ctx context.Context) (*SkillConstraints, error)

	// LoadTemplates loads scoring rubrics and Prompt templates on demand.
	LoadTemplates(ctx context.Context, templateIDs []string) ([]SkillTemplate, error)

	// Evaluate assesses the quality of an AI interaction along skill-specific dimensions.
	Evaluate(ctx context.Context, interaction InteractionData) (*SkillEvalResult, error)
}

// -- Existing Types (metadata from JSON files) --------------------

// SkillMetadata describes a skill plugin's metadata (from metadata.json).
// Used for Skill Store listing and intent matching routing.
type SkillMetadata struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Category     string   `json:"category"`
	Subjects     []string `json:"subjects"`
	Tags         []string `json:"tags"`
	Dependencies []string `json:"dependencies,omitempty"` // IDs of required plugins

	ScaffoldingLevels []string               `json:"scaffolding_levels"`
	Constraints       map[string]interface{} `json:"constraints"`
	Tools             map[string]ToolConfig  `json:"tools,omitempty"`

	ProgressiveTriggers *ProgressiveTriggers `json:"progressive_triggers,omitempty"`
	EvalDimensions      []string             `json:"evaluation_dimensions,omitempty"`
}

// ToolConfig defines a tool available to the skill.
type ToolConfig struct {
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// ProgressiveTriggers defines conditions for progressive strategy changes.
type ProgressiveTriggers struct {
	ActivateWhen   string `json:"activate_when,omitempty"`
	DeactivateWhen string `json:"deactivate_when,omitempty"`
}

// SkillConstraints holds the SKILL.md content (trigger layer).
type SkillConstraints struct {
	SkillID     string `json:"skill_id"`
	RawMarkdown string `json:"raw_markdown"`
}

// SkillTemplate holds a scoring rubric or Prompt template (reference layer).
type SkillTemplate struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	Content  string `json:"content"`
}

// RegisteredSkill represents a skill registered in the Registry.
// Contains the metadata layer, trigger layer path, and reference layer path.
// For DB-backed custom skills, SkillMDContent is populated instead of SkillMDPath.
type RegisteredSkill struct {
	Metadata       SkillMetadata `json:"metadata"`
	BasePath       string        `json:"-"`
	SkillMDPath    string        `json:"-"`
	SkillMDContent string        `json:"-"` // DB-backed: SKILL.md content stored in memory
	TemplatesPath  string        `json:"-"`
	State          PluginState   `json:"state"`
	IsCustom       bool          `json:"is_custom"` // true for teacher-created skills
	Impl           SkillPlugin   `json:"-"`         // nil for declarative-only skills
}
