# Plugin development guide

Hanfledge uses a plugin-based skill system that lets teachers extend
the AI dialogue with specialized instructional strategies. This guide
explains how to create custom skills, understand the plugin
architecture, and integrate with the skill lifecycle.

## Architecture overview

The skill system consists of three layers:

```
┌─────────────────────────────────┐
│       Plugin Registry           │  Loads skills at startup
│   (internal/plugin/registry.go) │  from plugins/skills/
├─────────────────────────────────┤
│       Skill Metadata            │  metadata.json + SKILL.md
│   (per-skill configuration)     │  define behavior and prompts
├─────────────────────────────────┤
│      Frontend Manifest          │  manifest.json defines
│   (UI components & rendering)   │  skill-specific UI
└─────────────────────────────────┘
```

Skills are loaded from the `plugins/skills/` directory at server
startup. Each skill is a directory containing backend configuration
and an optional frontend manifest.

## Skill directory structure

Each skill follows this directory layout:

```
plugins/skills/<skill-id>/
  backend/
    metadata.json       # Skill metadata and configuration
    SKILL.md            # System prompt for the AI agent
    templates/          # Optional message templates
      greeting.md
      follow-up.md
  frontend/
    manifest.json       # Frontend UI component manifest
```

### `metadata.json`

Defines the skill's identity and configuration:

```json
{
  "id": "my-custom-skill",
  "name": "My Custom Skill",
  "description": "A skill that teaches through analogies",
  "category": "pedagogy",
  "subjects": ["math", "science"],
  "tags": ["analogy", "conceptual"],
  "version": "1.0.0",
  "author": "Teacher Name",
  "tools": {
    "quiz": true,
    "diagram": false,
    "presentation": false
  },
  "scaffold_levels": {
    "high": "Provide detailed analogies with step-by-step mapping",
    "medium": "Suggest the analogy domain, let student map concepts",
    "low": "Ask student to propose their own analogies"
  },
  "progressive_rule": {
    "mastery_thresholds": {
      "high_to_medium": 0.4,
      "medium_to_low": 0.7
    }
  }
}
```

#### Required fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique skill identifier (kebab-case) |
| `name` | `string` | Display name |
| `description` | `string` | Brief description |
| `category` | `string` | Skill category (for example, `pedagogy`, `assessment`, `practice`) |

#### Optional fields

| Field | Type | Description |
|-------|------|-------------|
| `subjects` | `string[]` | Target subjects |
| `tags` | `string[]` | Searchable tags |
| `version` | `string` | Semantic version |
| `author` | `string` | Author name |
| `tools` | `object` | Available tools (quiz, diagram, presentation) |
| `scaffold_levels` | `object` | Behavior at each scaffold level |
| `progressive_rule` | `object` | BKT mastery thresholds for scaffold fading |

### `SKILL.md`

The SKILL.md file is the system prompt injected into the AI agent
when this skill is active. It defines the skill's persona,
constraints, and behavioral rules.

Example (`plugins/skills/socratic-questioning/backend/SKILL.md`):

```markdown
# Socratic Questioning Skill

## Identity
You are a Socratic tutor. Your role is to guide students toward
understanding through carefully crafted questions, never giving
direct answers.

## Core Constraints
1. NEVER provide direct answers to the student's question
2. Always respond with a guiding question
3. Break complex problems into smaller, manageable steps
4. Acknowledge student progress before asking the next question

## Scaffold Levels

### High Scaffold (mastery < 0.4)
- Ask simple yes/no questions
- Provide multiple-choice options
- Offer concrete examples before asking

### Medium Scaffold (0.4 <= mastery < 0.7)
- Ask open-ended questions
- Provide hints only when student is stuck
- Connect to prior knowledge

### Low Scaffold (mastery >= 0.7)
- Ask complex analytical questions
- Expect student to identify connections independently
- Challenge assumptions
```

The system injects the following context into the prompt alongside
SKILL.md:

- Current knowledge point details
- Student mastery level
- Active scaffold level
- Recent interaction history
- Retrieved context from KA-RAG pipeline

### `manifest.json` (Frontend)

Defines the frontend UI components for skill-specific rendering:

```json
{
  "id": "my-custom-skill",
  "components": {
    "QuestionCard": {
      "events": ["quiz_question"],
      "props": ["question", "options", "type"]
    },
    "ProgressBar": {
      "events": ["progress_update"],
      "props": ["current", "total", "label"]
    }
  },
  "styles": {
    "primaryColor": "#4A90D9",
    "icon": "lightbulb"
  }
}
```

The frontend plugin system uses manifest-driven component loading.
When the server sends a `skill_ui` WebSocket event, the frontend
looks up the manifest to determine which component to render.

## Built-in skills

Hanfledge ships with 9 built-in skills:

| Skill ID | Name | Category |
|----------|------|----------|
| `socratic-questioning` | Socratic Questioning | Pedagogy |
| `quiz-generation` | Quiz Generation | Assessment |
| `role-play` | Role Play | Practice |
| `fallacy-detective` | Fallacy Detective | Critical thinking |
| `error-diagnosis` | Error Diagnosis | Remediation |
| `cross-disciplinary` | Cross-Disciplinary | Integration |
| `learning-survey` | Learning Survey | Assessment |
| `presentation-generator` | Presentation Generator | Content creation |
| `stepped-learning` | Stepped Learning | Scaffolded instruction |

Each built-in skill resides in `plugins/skills/<skill-id>/` and
follows the same directory structure described above.

## Creating a custom skill (Web UI)

Teachers can create custom skills through the web interface without
writing code. The custom skill editor is available at
`/teacher/skills/create`.

Follow these steps to create a custom skill:

1. Navigate to **Skills** in the teacher dashboard.
2. Select **Create Custom Skill**.
3. Fill in the skill metadata (name, description, category,
   subjects).
4. Write the SKILL.md content in the editor. This is the system
   prompt that controls AI behavior.
5. Optionally configure tools (quiz, diagram, presentation).
6. Optionally add message templates.
7. Save as draft and test with the preview feature.
8. Publish the skill to make it available for mounting.

### Custom skill lifecycle

```
draft -> published -> shared -> archived
  ^         |
  |         v
  +--- (edit) ---+
```

| Status | Description |
|--------|-------------|
| `draft` | Editable, not available for use |
| `published` | Available for the teacher to mount on KPs |
| `shared` | Available to other teachers (school or platform scope) |
| `archived` | No longer available for new mounts |

### Visibility levels

| Level | Scope |
|-------|-------|
| `private` | Only the creating teacher |
| `school` | All teachers in the same school |
| `platform` | All teachers on the platform |

## Mounting skills to knowledge points

Skills are mounted (attached) to specific knowledge points in a
course. A mounted skill activates when a student's learning session
reaches that knowledge point.

### Mounting via API

```sh
# Mount a skill to a knowledge point
curl -X POST http://localhost:8080/api/v1/knowledge-points/42/skills \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "skill_id": "socratic-questioning",
    "scaffold_level": "high",
    "priority": 1
  }'
```

### Mounting via Web UI

1. Open a course and navigate to the outline.
2. Select a knowledge point.
3. Select **Mount Skill** and choose from the registry.
4. Configure the scaffold level and priority.
5. Optionally set progressive rules for automatic scaffold fading.

### Progressive scaffold fading

Skills support automatic scaffold fading based on BKT mastery scores.
Configure the `progressive_rule` to define when the scaffold level
decreases:

```json
{
  "progressive_rule": {
    "mastery_thresholds": {
      "high_to_medium": 0.4,
      "medium_to_low": 0.7
    }
  }
}
```

When a student's mastery score crosses a threshold, the system
automatically lowers the scaffold level, giving the student more
autonomy.

## Plugin registry

The plugin registry (`internal/plugin/registry.go`) manages skill
registration and lookup at runtime.

### Registry API

```go
// Create an empty registry
registry := plugin.NewRegistry()

// Register a skill with metadata
registry.RegisterSkillWithMetadata(meta)

// Look up a skill
skill, found := registry.Get("skill-id")

// List all skills
skills := registry.List()
```

### Skill loading at startup

On server startup, the registry scans `plugins/skills/` and loads
each skill:

1. Reads `backend/metadata.json` for skill configuration.
2. Reads `backend/SKILL.md` for the system prompt.
3. Reads `frontend/manifest.json` for UI component definitions.
4. Registers the skill in the in-memory registry.

Custom skills created through the web UI are stored in the database
(`custom_skills` table) and registered in the registry at startup.

## EventBus integration

The plugin system includes an EventBus for inter-plugin
communication:

```go
eventBus := plugin.NewEventBus()

// Subscribe to events
eventBus.Subscribe("mastery_updated", func(event plugin.Event) {
    // Handle mastery update
})

// Publish events
eventBus.Publish(plugin.Event{
    Type:    "mastery_updated",
    Payload: map[string]interface{}{"kp_id": 42, "score": 0.8},
})
```

Common events:

| Event | Description |
|-------|-------------|
| `mastery_updated` | Student mastery score changed |
| `session_started` | New learning session started |
| `session_completed` | Session completed |
| `skill_activated` | A skill was activated in a session |
| `achievement_unlocked` | Student unlocked an achievement |

## Marketplace

Teachers can share skills through the plugin marketplace. The
marketplace supports:

- **Submission:** Teachers submit skills for review via
  `POST /api/v1/marketplace/plugins`.
- **Review:** Submitted plugins go through a review process.
- **Installation:** Approved plugins can be installed at the school
  level via `POST /api/v1/marketplace/install`.
- **Trust levels:** `core` (built-in), `domain` (verified), and
  `community` (user-submitted).

## Best practices for skill design

Follow these guidelines when writing SKILL.md prompts:

1. **Define clear boundaries.** State what the skill must and must
   not do. Use "NEVER" and "ALWAYS" for hard constraints.
2. **Differentiate scaffold levels.** Provide distinct behavior for
   high, medium, and low scaffold levels.
3. **Include examples.** Add example interactions showing expected
   AI behavior at each scaffold level.
4. **Keep it focused.** Each skill handles one instructional
   strategy. Do not combine multiple strategies in a single skill.
5. **Use the student's language.** Write prompts that produce
   Chinese-language responses appropriate for K-12 students.
6. **Test with preview.** Use the sandbox preview feature to verify
   skill behavior before publishing.
7. **Set appropriate scaffold thresholds.** Use progressive rules
   to automatically adjust the guidance level as students improve.
