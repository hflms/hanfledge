# Database schema

Hanfledge uses PostgreSQL as its primary database with GORM as the ORM
layer. The database also integrates pgvector for embedding storage and
Neo4j for knowledge graph relationships. This document describes all
14 domain models, their fields, relationships, and the overall schema
design.

## Entity-relationship overview

The following diagram shows the relationships between all major
entities:

```
┌──────────┐     ┌──────────────┐     ┌──────────┐
│  School   │────<│UserSchoolRole│>────│   User   │
└──────────┘     └──────────────┘     └──────────┘
     │                                  │    │
     │                                  │    │
     ▼                                  │    │
┌──────────┐     ┌──────────────┐      │    │
│  Class    │────<│ ClassStudent │>─────┘    │
└──────────┘     └──────────────┘           │
                                            │
┌──────────┐     ┌──────────┐     ┌─────────┴───────┐
│  Course   │────<│ Chapter  │────<│ KnowledgePoint  │
└──────────┘     └──────────┘     └─────────────────┘
     │                                │    │    │
     │                                │    │    │
     ▼                                ▼    │    ▼
┌──────────┐              ┌──────────┐│ ┌──────────────┐
│ Document │              │Misconcep.││ │ KPSkillMount │
└──────────┘              └──────────┘│ └──────────────┘
     │                                │
     ▼                                ▼
┌──────────────┐              ┌──────────┐
│DocumentChunk │              │CrossLink │
│  (pgvector)  │              └──────────┘
└──────────────┘

┌──────────────────┐     ┌──────────────┐
│LearningActivity  │────<│ ActivityStep │
└──────────────────┘     └──────────────┘
     │    │
     │    ▼
     │ ┌──────────────────────┐
     │ │ActivityClassAssignment│
     │ └──────────────────────┘
     │
     ▼
┌──────────────────┐     ┌─────────────┐
│ StudentSession   │────<│ Interaction │
└──────────────────┘     └─────────────┘

┌──────────────────┐     ┌───────────────────┐
│StudentKPMastery  │     │ErrorNotebookEntry │
└──────────────────┘     └───────────────────┘

┌──────────────────────┐  ┌───────────────────┐
│AchievementDefinition │─<│StudentAchievement │
└──────────────────────┘  └───────────────────┘

┌──────────────────┐  ┌──────────────────┐
│MarketplacePlugin │  │MarketplaceReview │
└──────────────────┘  └──────────────────┘
        │
        ▼
┌──────────────────┐
│ InstalledPlugin  │
└──────────────────┘

┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Notification │  │ SoulVersion  │  │ SystemConfig │
└──────────────┘  └──────────────┘  └──────────────┘

┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐
│WeKnoraKBRef  │  │WeKnoraToken  │  │InstructionalDesigner │
└──────────────┘  └──────────────┘  └──────────────────────┘

┌──────────────────┐  ┌──────────────────────┐
│ AnalyticsEvent   │  │ CustomSkill          │
└──────────────────┘  │   └─CustomSkillVersion│
                      └──────────────────────┘
```

## User and access control

### User

Represents a platform user. Users are identified by phone number and
can hold multiple roles across different schools.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `phone` | `varchar(20)` | Unique index, not null | Phone number (login identifier) |
| `email` | `varchar(100)` | Unique index | Optional email address |
| `password_hash` | `varchar(255)` | Not null | bcrypt password hash (hidden from JSON) |
| `display_name` | `varchar(50)` | Not null | Display name |
| `avatar_url` | `varchar(500)` | | Avatar image URL |
| `status` | `varchar(20)` | Default: `active` | `active`, `inactive`, or `banned` |
| `created_at` | `timestamp` | | Creation timestamp |
| `updated_at` | `timestamp` | | Last update timestamp |
| `deleted_at` | `timestamp` | Index (soft delete) | Soft delete timestamp |

### School

Represents a school in the platform.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `name` | `varchar(100)` | Not null | School name |
| `code` | `varchar(20)` | Unique index | School code identifier |
| `region` | `varchar(100)` | | Geographic region |
| `status` | `varchar(20)` | Default: `active` | `active` or `inactive` |
| `created_at` | `timestamp` | | Creation timestamp |
| `updated_at` | `timestamp` | | Last update timestamp |
| `deleted_at` | `timestamp` | Index (soft delete) | Soft delete timestamp |

### Class

Represents a class within a school.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `school_id` | `uint` | Not null, index, FK | Reference to School |
| `name` | `varchar(50)` | Not null | Class name |
| `grade_level` | `int` | Not null | Grade level (1-12) |
| `academic_year` | `varchar(10)` | | Academic year (for example, `2025-2026`) |
| `created_at` | `timestamp` | | Creation timestamp |
| `deleted_at` | `timestamp` | Index (soft delete) | Soft delete timestamp |

### Role

Defines the four system roles.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `name` | `varchar(20)` | Unique index, not null | `SYS_ADMIN`, `SCHOOL_ADMIN`, `TEACHER`, `STUDENT` |

### UserSchoolRole

Maps users to roles at specific schools (many-to-many).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `user_id` | `uint` | Not null, index, FK | Reference to User |
| `school_id` | `uint` | Index, FK | Reference to School (null for SYS_ADMIN) |
| `role_id` | `uint` | Not null, FK | Reference to Role |

### ClassStudent

Maps students to classes (many-to-many).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `class_id` | `uint` | Not null, index, FK | Reference to Class |
| `student_id` | `uint` | Not null, index, FK | Reference to User (student) |

## Course and knowledge graph

### Course

Represents a course created by a teacher.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `school_id` | `uint` | Not null, index, FK | Reference to School |
| `teacher_id` | `uint` | Not null, index, FK | Reference to User (teacher) |
| `title` | `varchar(200)` | Not null | Course title |
| `subject` | `varchar(50)` | Not null | Subject area |
| `grade_level` | `int` | Not null | Target grade level |
| `description` | `text` | | Course description |
| `status` | `varchar(20)` | Default: `draft` | `draft`, `published`, or `archived` |
| `created_at` | `timestamp` | | Creation timestamp |
| `updated_at` | `timestamp` | | Last update timestamp |

### Chapter

Represents a chapter within a course. Supports tree structure via
`parent_id` for nested chapters.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `course_id` | `uint` | Not null, index, FK | Reference to Course |
| `parent_id` | `uint` | Index | Self-referencing parent (null for root) |
| `title` | `varchar(200)` | Not null | Chapter title |
| `sort_order` | `int` | Default: 0 | Display order within parent |

### KnowledgePoint

Represents a knowledge point (concept) within a chapter.
Synchronized with Neo4j for graph queries.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `chapter_id` | `uint` | Not null, index, FK | Reference to Chapter |
| `neo4j_node_id` | `varchar(50)` | Index | Corresponding Neo4j node ID |
| `title` | `varchar(200)` | Not null | Knowledge point title |
| `description` | `text` | | Detailed description |
| `difficulty` | `float64` | Default: 0.5 | Difficulty level (0.0-1.0) |
| `is_key_point` | `bool` | Default: false | Whether this is a key concept |

### Misconception

Common student misconceptions tied to a knowledge point.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `kp_id` | `uint` | Not null, index, FK | Reference to KnowledgePoint |
| `neo4j_node_id` | `varchar(50)` | Index | Corresponding Neo4j node ID |
| `description` | `text` | Not null | Misconception description |
| `trap_type` | `varchar(20)` | Default: `conceptual` | `conceptual`, `procedural`, `intuitive`, `transfer` |
| `severity` | `float64` | Default: 0.5 | Severity level (0.0-1.0) |
| `created_at` | `timestamp` | | Creation timestamp |

### CrossLink

Represents cross-disciplinary links between knowledge points.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `from_kp_id` | `uint` | Not null, index, FK | Source KnowledgePoint |
| `to_kp_id` | `uint` | Not null, index, FK | Target KnowledgePoint |
| `link_type` | `varchar(50)` | Default: `analogy` | `analogy`, `shared_model`, or `application` |
| `weight` | `float64` | Default: 1.0 | Link strength |
| `created_at` | `timestamp` | | Creation timestamp |

## Document processing

### Document

Represents an uploaded document (PDF) for a course.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `course_id` | `uint` | Not null, index, FK | Reference to Course |
| `file_name` | `varchar(500)` | Not null | Original filename |
| `file_path` | `varchar(1000)` | Not null | Storage path |
| `file_size` | `int64` | | File size in bytes |
| `mime_type` | `varchar(100)` | | MIME type |
| `status` | `varchar(20)` | Default: `uploaded` | `uploaded`, `processing`, `completed`, `failed` |
| `page_count` | `int` | | Number of pages |
| `error_message` | `text` | | Error message if processing failed |
| `created_at` | `timestamp` | | Creation timestamp |
| `updated_at` | `timestamp` | | Last update timestamp |

### DocumentChunk

Represents a text chunk from a processed document with its embedding
vector.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `document_id` | `uint` | Not null, index, FK | Reference to Document |
| `course_id` | `uint` | Not null, index | Reference to Course (denormalized) |
| `chunk_index` | `int` | Not null | Chunk position within document |
| `content` | `text` | Not null | Chunk text content |
| `token_count` | `int` | | Approximate token count |
| `page_number` | `int` | | Source page number |
| `embedding` | `vector(1024)` | | bge-m3 embedding (pgvector) |
| `created_at` | `timestamp` | | Creation timestamp |

> **Note:** The `embedding` column uses the pgvector extension for
> 1024-dimensional vector storage. The `bge-m3` model generates these
> embeddings during document processing.

## Learning activities

### LearningActivity

Represents a learning activity designed by a teacher.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `course_id` | `uint` | Not null, index, FK | Reference to Course |
| `teacher_id` | `uint` | Not null, index, FK | Reference to User (teacher) |
| `title` | `varchar(200)` | Not null | Activity title |
| `description` | `varchar(2000)` | | Activity description |
| `type` | `varchar(50)` | Default: `autonomous` | `autonomous` or `guided` |
| `designer_id` | `varchar(100)` | | Instructional designer ID |
| `designer_config` | `jsonb` | | Designer-specific config |
| `steps_config` | `jsonb` | | Steps configuration |
| `kp_id_s` | `jsonb` | | Target knowledge point IDs |
| `skill_config` | `jsonb` | | Skill configuration |
| `deadline` | `varchar` | | Activity deadline |
| `allow_retry` | `bool` | Default: true | Whether retries are allowed |
| `max_attempts` | `int` | Default: 3 | Maximum attempt count |
| `status` | `varchar(20)` | Default: `draft` | `draft`, `published`, `closed` |
| `created_at` | `varchar` | | Creation timestamp |
| `updated_at` | `varchar` | | Last update timestamp |
| `published_at` | `varchar` | | Publication timestamp |

> **Note:** The GORM field `KPIDS` maps to column `kp_id_s` via
> GORM naming strategy, not `kpids`.

### ActivityStep

Represents a step within a guided learning activity.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `activity_id` | `uint` | Not null, index, FK | Reference to LearningActivity |
| `title` | `varchar(200)` | Not null | Step title |
| `description` | `varchar(2000)` | | Step description |
| `step_type` | `varchar(50)` | Default: `lecture` | `lecture`, `discussion`, `quiz`, `practice`, `reading`, `group_work`, `reflection`, `ai_tutoring` |
| `sort_order` | `int` | Default: 0 | Display order |
| `content_blocks` | `jsonb` | Default: `[]` | Content blocks (markdown, file, video, image) |
| `duration` | `int` | Default: 0 | Estimated duration in minutes |

### ActivityClassAssignment

Maps activities to target classes (many-to-many).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `activity_id` | `uint` | Not null, index, FK | Reference to LearningActivity |
| `class_id` | `uint` | Not null, index, FK | Reference to Class |

## Student sessions and mastery

### StudentSession

Represents a student's learning session within an activity.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `student_id` | `uint` | Not null, index, FK | Reference to User (student) |
| `activity_id` | `uint` | Not null, index, FK | Reference to LearningActivity |
| `current_kp_id` | `uint` | Not null | Current knowledge point |
| `active_skill` | `varchar(100)` | | Currently active skill ID |
| `scaffold_level` | `varchar(20)` | | `high`, `medium`, or `low` |
| `skill_state` | `jsonb` | | Skill-specific state data |
| `is_sandbox` | `bool` | Default: false | Whether this is a preview session |
| `status` | `varchar(20)` | Default: `active` | `active`, `completed`, `abandoned` |
| `mode` | `varchar(20)` | Default: `socratic` | `socratic` or `testing` |
| `started_at` | `timestamp` | | Session start time |
| `ended_at` | `timestamp` | | Session end time |

### Interaction

Represents a single message in a dialogue session.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `session_id` | `uint` | Not null, index, FK | Reference to StudentSession |
| `role` | `varchar(20)` | Not null | `student`, `coach`, `system`, `teacher` |
| `content` | `text` | Not null | Message content |
| `skill_id` | `varchar(100)` | | Active skill when message was sent |
| `tokens_used` | `int` | Default: 0 | LLM tokens consumed |
| `created_at` | `timestamp` | | Message timestamp |
| `faithfulness_score` | `float64` | | RAGAS faithfulness score |
| `actionability_score` | `float64` | | RAGAS actionability score |
| `answer_restraint_score` | `float64` | | Socratic restraint score |
| `context_precision` | `float64` | | RAG context precision |
| `context_recall` | `float64` | | RAG context recall |
| `eval_status` | `varchar(20)` | Default: `pending` | `pending`, `evaluated`, `skipped` |

### StudentKPMastery

Tracks a student's mastery of a specific knowledge point using
Bayesian Knowledge Tracing (BKT).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `student_id` | `uint` | Not null, unique index | Reference to User (student) |
| `kp_id` | `uint` | Not null, unique index | Reference to KnowledgePoint |
| `mastery_score` | `float64` | Default: 0.1 | Current mastery level (0.0-1.0) |
| `attempt_count` | `int` | Default: 0 | Number of attempts |
| `correct_count` | `int` | Default: 0 | Number of correct responses |
| `passed_test` | `bool` | Default: false | Whether mastery test is passed |
| `last_attempt_at` | `timestamp` | | Last attempt timestamp |
| `updated_at` | `timestamp` | | Last update timestamp |

> **Note:** The `(student_id, kp_id)` pair has a unique composite
> index (`idx_student_kp`).

### ErrorNotebookEntry

Records student errors for review and reinforcement.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `student_id` | `uint` | Not null, index | Reference to User (student) |
| `kp_id` | `uint` | Not null, index | Reference to KnowledgePoint |
| `session_id` | `uint` | Not null, index | Reference to StudentSession |
| `student_input` | `text` | Not null | Student's incorrect response |
| `coach_guidance` | `text` | Not null | AI coach's corrective guidance |
| `error_type` | `varchar(30)` | Default: `unknown` | `conceptual`, `procedural`, `intuitive`, `unknown` |
| `mastery_at_error` | `float64` | Default: 0 | Mastery score when error occurred |
| `resolved` | `bool` | Default: false | Whether the error has been resolved |
| `resolved_at` | `timestamp` | | Resolution timestamp |
| `archived_at` | `timestamp` | | When the error was recorded |

## Skill system

### KPSkillMount

Maps skills to knowledge points with configuration.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `kp_id` | `uint` | Not null, index, FK | Reference to KnowledgePoint |
| `skill_id` | `varchar(100)` | Not null | Skill registry ID |
| `scaffold_level` | `varchar(20)` | Default: `high` | `high`, `medium`, `low` |
| `constraints_json` | `jsonb` | | Custom constraints |
| `priority` | `int` | Default: 0 | Priority order |
| `progressive_rule` | `jsonb` | | BKT-based scaffold fading rule |

### CustomSkill

Teacher-created custom skills.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `skill_id` | `varchar(100)` | Unique index, not null | Unique skill identifier |
| `teacher_id` | `uint` | Not null, index, FK | Reference to User (teacher) |
| `school_id` | `uint` | Index | Reference to School |
| `name` | `varchar(200)` | Not null | Skill name |
| `description` | `varchar(1000)` | | Skill description |
| `category` | `varchar(50)` | | Skill category |
| `subjects` | `jsonb` | Default: `[]` | Target subjects |
| `tags` | `jsonb` | Default: `[]` | Searchable tags |
| `skill_md` | `text` | Not null | Skill prompt (SKILL.md content) |
| `tools_config` | `jsonb` | Default: `{}` | Tool configuration |
| `templates` | `jsonb` | Default: `[]` | Message templates |
| `status` | `varchar(20)` | Default: `draft` | `draft`, `published`, `shared`, `archived` |
| `visibility` | `varchar(20)` | Default: `private` | `private`, `school`, `platform` |
| `version` | `int` | Default: 1 | Current version number |
| `usage_count` | `int` | Default: 0 | Total usage count |

### CustomSkillVersion

Version history for custom skills.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `custom_skill_id` | `uint` | Not null, index, FK | Reference to CustomSkill |
| `version` | `int` | Not null | Version number |
| `skill_md` | `text` | Not null | Skill prompt content |
| `tools_config` | `jsonb` | | Tool configuration |
| `templates` | `jsonb` | | Message templates |
| `change_log` | `varchar(500)` | | Change description |

## Gamification

### AchievementDefinition

Defines available achievements with tier thresholds.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `type` | `varchar(30)` | Not null, unique index | `streak_breaker`, `deep_inquiry`, `fallacy_hunter` |
| `tier` | `varchar(20)` | Not null, unique index | `bronze`, `silver`, `gold`, `diamond` |
| `name` | `varchar(50)` | Not null | Achievement name |
| `description` | `varchar(200)` | Not null | Achievement description |
| `icon` | `varchar(10)` | Not null | Emoji icon |
| `threshold` | `int` | Not null | Progress required to unlock |
| `sort_order` | `int` | Default: 0 | Display order |

> **Note:** The `(type, tier)` pair has a unique composite index
> (`idx_achievement_def`).

### StudentAchievement

Tracks a student's progress toward achievements.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `student_id` | `uint` | Not null, unique index | Reference to User (student) |
| `achievement_id` | `uint` | Not null, unique index | Reference to AchievementDefinition |
| `progress` | `int` | Default: 0 | Current progress value |
| `unlocked` | `bool` | Default: false | Whether achievement is unlocked |
| `unlocked_at` | `timestamp` | | Unlock timestamp |

## Marketplace

### MarketplacePlugin

Represents a plugin available in the marketplace.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `plugin_id` | `varchar(100)` | Unique index, not null | Unique plugin identifier |
| `name` | `varchar(200)` | Not null | Plugin name |
| `description` | `text` | | Plugin description |
| `version` | `varchar(50)` | Not null | Semantic version |
| `author` | `varchar(100)` | | Author name |
| `author_id` | `uint` | Index | Reference to User |
| `type` | `varchar(50)` | Not null | Plugin type |
| `trust_level` | `varchar(50)` | Default: `community` | `core`, `domain`, `community` |
| `category` | `varchar(100)` | | Plugin category |
| `tags` | `text` | | Comma-separated tags |
| `downloads` | `int` | Default: 0 | Download count |
| `rating` | `float64` | Default: 0 | Average rating |
| `rating_count` | `int` | Default: 0 | Number of ratings |
| `status` | `varchar(50)` | Default: `pending` | `pending`, `approved`, `rejected`, `deprecated` |

### InstalledPlugin

Tracks plugins installed at a school.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `school_id` | `uint` | Index, not null, FK | Reference to School |
| `plugin_id` | `varchar(100)` | Index, not null | Reference to plugin |
| `version` | `varchar(50)` | | Installed version |
| `config` | `text` | | School-specific config |
| `enabled` | `bool` | Default: true | Whether plugin is active |

## System

### Notification

System notifications delivered to users.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `user_id` | `uint` | Not null, index | Reference to User |
| `type` | `varchar(50)` | Not null | `soul_evolution`, `system_alert` |
| `title` | `varchar(200)` | Not null | Notification title |
| `content` | `text` | | Notification body |
| `is_read` | `bool` | Default: false | Read status |
| `created_at` | `timestamp` | | Creation timestamp |

### SoulVersion

Tracks versions of the AI behavior rules (Soul system).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `version` | `varchar(20)` | Not null | Version identifier |
| `content` | `text` | Not null | Soul rules content |
| `updated_by` | `uint` | Not null | User who made the change |
| `reason` | `text` | | Change reason |
| `is_active` | `bool` | Default: false, index | Whether this is the active version |
| `created_at` | `timestamp` | | Creation timestamp |

Table name: `soul_versions`

### SystemConfig

Key-value configuration store for runtime settings.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `key` | `varchar(100)` | Primary key | Configuration key |
| `value` | `text` | | Configuration value |

### AnalyticsEvent

Stores frontend analytics events for performance monitoring.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `type` | `varchar(50)` | Not null, index | `render`, `interaction`, `error`, `performance` |
| `component` | `varchar(100)` | Not null, index | Component name |
| `data` | `jsonb` | | Event data payload |
| `user_id` | `uint` | Index | Reference to User |
| `session_id` | `uint` | Index | Reference to StudentSession |
| `timestamp` | `int64` | Not null, index | Unix timestamp |

Table name: `analytics_events`

## External integrations

### InstructionalDesigner

Defines instructional designer templates for guided activities.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `varchar(100)` | Primary key | Designer identifier |
| `name` | `varchar(100)` | Not null | Designer name |
| `description` | `text` | | Designer description |
| `intervention_style` | `varchar(20)` | Not null | `questioning`, `coaching`, `diagnostic`, `facilitation` |
| `is_built_in` | `bool` | Default: false | Whether this is a built-in designer |

### WeKnoraKBRef

Maps courses to WeKnora knowledge bases.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `course_id` | `uint` | Not null, index, FK | Reference to Course |
| `kb_id` | `varchar(100)` | Not null | WeKnora knowledge base ID |
| `kb_name` | `varchar(200)` | Not null | Knowledge base name |
| `added_by_id` | `uint` | Not null, FK | User who added the binding |

### WeKnoraToken

Stores WeKnora SSO tokens for user mapping.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary key | Auto-increment ID |
| `user_id` | `uint` | Unique index, not null, FK | Reference to User |
| `wk_user_id` | `varchar(100)` | | WeKnora user ID |
| `wk_email` | `varchar(200)` | Not null | WeKnora email |
| `token` | `text` | | Encrypted access token (hidden from JSON) |
| `refresh_token` | `text` | | Encrypted refresh token (hidden from JSON) |
| `expires_at` | `timestamp` | | Token expiry time |

## Neo4j graph schema

In addition to PostgreSQL, Hanfledge maintains a knowledge graph in
Neo4j with the following node and relationship types:

### Node types

| Label | Properties | Source |
|-------|------------|--------|
| `KnowledgePoint` | `id`, `title`, `description`, `difficulty`, `is_key_point` | Synced from PostgreSQL |
| `Misconception` | `id`, `description`, `trap_type`, `severity` | Synced from PostgreSQL |
| `Chapter` | `id`, `title` | Synced from PostgreSQL |

### Relationship types

| Type | From | To | Properties |
|------|------|----|-----------|
| `BELONGS_TO` | KnowledgePoint | Chapter | -- |
| `HAS_MISCONCEPTION` | KnowledgePoint | Misconception | -- |
| `PREREQUISITE` | KnowledgePoint | KnowledgePoint | -- |
| `CROSS_LINK` | KnowledgePoint | KnowledgePoint | `type`, `weight` |

## Database migrations

GORM automatically runs migrations on server startup. The migration
list is defined in `cmd/server/main.go` and includes all model types
documented above. New models must be added to the `AutoMigrate` call
to be included in the migration.
