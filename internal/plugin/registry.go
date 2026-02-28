package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ============================
// Plugin Registry & Discovery
// ============================
//
// Lifecycle state machine (design.md section 7.4):
//   Discovered -> Validated -> Initialized -> Running -> Degraded / Stopped
//
// The Registry manages plugin discovery, validation, initialization,
// and provides an EventBus for hook-based communication.

// Registry is the central plugin registry and lifecycle manager.
type Registry struct {
	mu        sync.RWMutex
	skills    map[string]*RegisteredSkill // key: skill ID
	eventBus  *EventBus
	validator *SkillValidator // Anti-God Skill 验证器 (§8.2.1)
}

// NewRegistry creates an empty plugin registry with an EventBus.
func NewRegistry() *Registry {
	return &Registry{
		skills:    make(map[string]*RegisteredSkill),
		eventBus:  NewEventBus(),
		validator: DefaultSkillValidator(),
	}
}

// EventBus returns the registry's event bus for hook subscriptions.
func (r *Registry) EventBus() *EventBus {
	return r.eventBus
}

// -- Plugin Loading & Lifecycle ----------------------------------

// LoadSkills scans the given directory for skill plugins and registers them.
// Each subdirectory is expected to contain backend/metadata.json.
// Lifecycle: Discovered -> Validated -> Running (for declarative skills).
func (r *Registry) LoadSkills(skillsDir string) error {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Plugin] Skills directory not found: %s", skillsDir)
			return nil
		}
		return fmt.Errorf("read skills directory failed: %w", err)
	}

	loaded := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(skillsDir, entry.Name())

		// Phase 1: Discover
		skill, err := r.discoverSkill(skillDir)
		if err != nil {
			log.Printf("[Plugin] Skip skill %s: %v", entry.Name(), err)
			continue
		}

		// Phase 2: Validate
		if err := r.validateSkill(skill); err != nil {
			log.Printf("[Plugin] Validation failed for %s: %v", skill.Metadata.ID, err)
			continue
		}

		// Phase 3: Register (declarative skills go straight to Running)
		r.mu.Lock()
		skill.State = PluginStateRunning
		r.skills[skill.Metadata.ID] = skill
		r.mu.Unlock()
		loaded++

		log.Printf("   Loaded skill: %s (%s) v%s [%s]",
			skill.Metadata.Name, skill.Metadata.ID, skill.Metadata.Version, skill.State)
	}

	log.Printf("[Plugin] Loaded %d skill(s) from %s", loaded, skillsDir)
	return nil
}

// RegisterSkillPlugin registers a programmatic SkillPlugin implementation.
// This is used for core/domain plugins that implement the SkillPlugin interface.
func (r *Registry) RegisterSkillPlugin(impl SkillPlugin) error {
	meta := impl.PluginMetadata()

	// Initialize the plugin
	deps := PluginDeps{
		EventBus: r.eventBus,
	}
	if err := impl.Init(context.Background(), deps); err != nil {
		return fmt.Errorf("init plugin %s failed: %w", meta.ID, err)
	}

	// Subscribe to declared hooks
	for _, hook := range meta.Hooks {
		// Plugins register their own handlers during Init via EventBus
		log.Printf("[Plugin] %s declares hook: %s", meta.ID, hook)
	}

	r.mu.Lock()
	r.skills[meta.ID] = &RegisteredSkill{
		Metadata: SkillMetadata{
			ID:       meta.ID,
			Name:     meta.Name,
			Version:  meta.Version,
			Category: string(meta.Type),
		},
		State: PluginStateRunning,
		Impl:  impl,
	}
	r.mu.Unlock()

	log.Printf("[Plugin] Registered programmatic skill: %s v%s", meta.Name, meta.Version)
	return nil
}

// RegisterSkillWithMetadata registers a skill with explicit SkillMetadata.
// This allows full control over all metadata fields (including ProgressiveTriggers)
// and is useful for testing and programmatic registration.
func (r *Registry) RegisterSkillWithMetadata(meta SkillMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills[meta.ID] = &RegisteredSkill{
		Metadata: meta,
		State:    PluginStateRunning,
	}
}

// RegisterCustomSkill registers a teacher-created custom skill from database.
// Unlike filesystem skills, custom skills store their SKILL.md content in memory.
// The skill is immediately available for LoadConstraints and GetSkill queries.
func (r *Registry) RegisterCustomSkill(meta SkillMetadata, skillMDContent string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills[meta.ID] = &RegisteredSkill{
		Metadata:       meta,
		SkillMDContent: skillMDContent,
		State:          PluginStateRunning,
		IsCustom:       true,
	}
}

// UnregisterSkill removes a skill from the registry by ID.
// Used when a custom skill is archived or deleted.
func (r *Registry) UnregisterSkill(skillID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, skillID)
}

// ShutdownAll gracefully shuts down all programmatic plugins.
func (r *Registry) ShutdownAll(ctx context.Context) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id, skill := range r.skills {
		if skill.Impl != nil {
			if err := skill.Impl.Shutdown(ctx); err != nil {
				log.Printf("[Plugin] Shutdown %s failed: %v", id, err)
			} else {
				skill.State = PluginStateStopped
			}
		}
	}
}

// HealthCheckAll runs health checks on all programmatic plugins.
// Marks unhealthy plugins as Degraded.
func (r *Registry) HealthCheckAll(ctx context.Context) map[string]HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]HealthStatus)
	for id, skill := range r.skills {
		if skill.Impl != nil {
			status := skill.Impl.HealthCheck(ctx)
			results[id] = status
			if !status.Healthy && skill.State == PluginStateRunning {
				skill.State = PluginStateDegraded
				log.Printf("[Plugin] %s degraded: %s", id, status.Message)
			} else if status.Healthy && skill.State == PluginStateDegraded {
				skill.State = PluginStateRunning
				log.Printf("[Plugin] %s recovered", id)
			}
		} else {
			// Declarative plugins are always healthy
			results[id] = HealthStatus{Healthy: true, Message: "declarative"}
		}
	}
	return results
}

// -- Internal Discovery & Validation -----------------------------

// discoverSkill reads a single skill plugin from a directory (Discovered state).
func (r *Registry) discoverSkill(skillDir string) (*RegisteredSkill, error) {
	metadataPath := filepath.Join(skillDir, "backend", "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata.json failed: %w", err)
	}

	var meta SkillMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata.json failed: %w", err)
	}

	skill := &RegisteredSkill{
		Metadata:      meta,
		BasePath:      skillDir,
		SkillMDPath:   filepath.Join(skillDir, "backend", "SKILL.md"),
		TemplatesPath: filepath.Join(skillDir, "backend", "templates"),
		State:         PluginStateDiscovered,
	}

	return skill, nil
}

// validateSkill checks required fields, file existence, and Anti-God Skill rules (Validated state).
func (r *Registry) validateSkill(skill *RegisteredSkill) error {
	if skill.Metadata.ID == "" {
		return fmt.Errorf("missing required field: id")
	}
	if skill.Metadata.Name == "" {
		return fmt.Errorf("missing required field: name")
	}

	// Check that SKILL.md exists
	if _, err := os.Stat(skill.SkillMDPath); err != nil {
		return fmt.Errorf("SKILL.md not found at %s", skill.SkillMDPath)
	}

	// Anti-God Skill 验证 (§8.2.1)
	if r.validator != nil {
		result := r.validator.Validate(skill)

		// 输出警告（不阻止加载）
		for _, w := range result.Warnings {
			log.Printf("⚠️  [Validator] %s: %s", skill.Metadata.ID, w)
		}

		// 输出错误（阻止加载）
		if !result.Valid {
			return fmt.Errorf("Anti-God Skill validation failed: %s", strings.Join(result.Errors, "; "))
		}
	}

	skill.State = PluginStateValidated
	return nil
}

// -- Query Methods -----------------------------------------------

// GetSkill returns a registered skill by ID.
func (r *Registry) GetSkill(id string) (*RegisteredSkill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[id]
	return skill, ok
}

// ListSkills returns all registered skills, optionally filtered by subject and/or category.
func (r *Registry) ListSkills(subject, category string) []*RegisteredSkill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredSkill, 0, len(r.skills))
	for _, skill := range r.skills {
		if subject != "" && !containsIgnoreCase(skill.Metadata.Subjects, subject) {
			continue
		}
		if category != "" && !strings.EqualFold(skill.Metadata.Category, category) {
			continue
		}
		result = append(result, skill)
	}
	return result
}

// LoadConstraints reads the SKILL.md content for a given skill (lazy loading).
// For programmatic plugins, delegates to the SkillPlugin.LoadConstraints method.
// For DB-backed custom skills, returns the in-memory SkillMDContent.
// For filesystem skills, reads from the SKILL.md file.
func (r *Registry) LoadConstraints(skillID string) (*SkillConstraints, error) {
	skill, ok := r.GetSkill(skillID)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	// If the skill has a programmatic implementation, use it
	if skill.Impl != nil {
		return skill.Impl.LoadConstraints(context.Background())
	}

	// DB-backed custom skill: return in-memory content
	if skill.SkillMDContent != "" {
		return &SkillConstraints{
			SkillID:     skillID,
			RawMarkdown: skill.SkillMDContent,
		}, nil
	}

	// Declarative: read from filesystem
	data, err := os.ReadFile(skill.SkillMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("SKILL.md not found for %s", skillID)
		}
		return nil, fmt.Errorf("read SKILL.md failed: %w", err)
	}

	return &SkillConstraints{
		SkillID:     skillID,
		RawMarkdown: string(data),
	}, nil
}

// LoadTemplates reads template files from the templates/ directory for a given skill.
// For programmatic plugins, delegates to the SkillPlugin.LoadTemplates method.
func (r *Registry) LoadTemplates(skillID string, templateIDs []string) ([]SkillTemplate, error) {
	skill, ok := r.GetSkill(skillID)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	// If the skill has a programmatic implementation, use it
	if skill.Impl != nil {
		return skill.Impl.LoadTemplates(context.Background(), templateIDs)
	}

	// Declarative: read from filesystem
	if len(templateIDs) == 0 {
		return r.loadAllTemplates(skill)
	}

	templates := make([]SkillTemplate, 0, len(templateIDs))
	for _, tid := range templateIDs {
		tmplPath := filepath.Join(skill.TemplatesPath, tid)
		data, err := os.ReadFile(tmplPath)
		if err != nil {
			log.Printf("[Plugin] Template %s not found for skill %s", tid, skillID)
			continue
		}
		templates = append(templates, SkillTemplate{
			ID:       tid,
			FileName: tid,
			Content:  string(data),
		})
	}
	return templates, nil
}

// loadAllTemplates reads all files from the templates directory.
func (r *Registry) loadAllTemplates(skill *RegisteredSkill) ([]SkillTemplate, error) {
	entries, err := os.ReadDir(skill.TemplatesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read templates dir failed: %w", err)
	}

	var templates []SkillTemplate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(skill.TemplatesPath, entry.Name()))
		if err != nil {
			continue
		}
		templates = append(templates, SkillTemplate{
			ID:       entry.Name(),
			FileName: entry.Name(),
			Content:  string(data),
		})
	}
	return templates, nil
}

// -- Helpers ------------------------------------------------------

// containsIgnoreCase checks if a string slice contains a target (case-insensitive).
func containsIgnoreCase(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}
