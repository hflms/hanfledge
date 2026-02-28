package plugin

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Registry 是技能插件的注册中心。
// 启动时扫描 /plugins/skills/ 目录，将所有合法技能加载到内存 Map。
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*RegisteredSkill // key: skill ID
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*RegisteredSkill),
	}
}

// LoadSkills scans the given directory for skill plugins and registers them.
// Each subdirectory is expected to contain backend/metadata.json.
func (r *Registry) LoadSkills(skillsDir string) error {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("⚠️  [Plugin] Skills directory not found: %s", skillsDir)
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
		skill, err := r.loadSkill(skillDir)
		if err != nil {
			log.Printf("⚠️  [Plugin] Skip skill %s: %v", entry.Name(), err)
			continue
		}

		r.mu.Lock()
		r.skills[skill.Metadata.ID] = skill
		r.mu.Unlock()
		loaded++

		log.Printf("   ✅ Loaded skill: %s (%s) v%s",
			skill.Metadata.Name, skill.Metadata.ID, skill.Metadata.Version)
	}

	log.Printf("🧩 [Plugin] Loaded %d skill(s) from %s", loaded, skillsDir)
	return nil
}

// loadSkill reads a single skill plugin from a directory.
func (r *Registry) loadSkill(skillDir string) (*RegisteredSkill, error) {
	// Read metadata.json from backend/
	metadataPath := filepath.Join(skillDir, "backend", "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata.json failed: %w", err)
	}

	var meta SkillMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata.json failed: %w", err)
	}

	// Validate required fields
	if meta.ID == "" || meta.Name == "" {
		return nil, fmt.Errorf("metadata.json missing required fields (id, name)")
	}

	skill := &RegisteredSkill{
		Metadata:      meta,
		BasePath:      skillDir,
		SkillMDPath:   filepath.Join(skillDir, "backend", "SKILL.md"),
		TemplatesPath: filepath.Join(skillDir, "backend", "templates"),
	}

	return skill, nil
}

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

// LoadConstraints reads the SKILL.md file for a given skill (lazy loading).
func (r *Registry) LoadConstraints(skillID string) (*SkillConstraints, error) {
	skill, ok := r.GetSkill(skillID)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

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
func (r *Registry) LoadTemplates(skillID string, templateIDs []string) ([]SkillTemplate, error) {
	skill, ok := r.GetSkill(skillID)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	// If no specific templates requested, load all
	if len(templateIDs) == 0 {
		return r.loadAllTemplates(skill)
	}

	templates := make([]SkillTemplate, 0, len(templateIDs))
	for _, tid := range templateIDs {
		tmplPath := filepath.Join(skill.TemplatesPath, tid)
		data, err := os.ReadFile(tmplPath)
		if err != nil {
			log.Printf("⚠️  [Plugin] Template %s not found for skill %s", tid, skillID)
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

// containsIgnoreCase checks if a string slice contains a target (case-insensitive).
func containsIgnoreCase(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}
