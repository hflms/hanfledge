/**
 * Frontend Plugin System — Type Definitions
 *
 * Defines the plugin taxonomy, slot names, trust levels,
 * sandbox communication protocol, and registration interfaces
 * per design.md §7.10–7.16.
 */

// -- Slot Names -----------------------------------------------

/**
 * All predefined extension-point slot names in the UI.
 * Plugins register themselves to one or more of these slots.
 */
export type SlotName =
  // Student slots
  | 'student.interaction.main'
  | 'student.interaction.sidebar'
  | 'student.interaction.toolbar'
  | 'student.reflection.visualization'
  | 'student.knowledge-map'
  // Teacher slots
  | 'teacher.dashboard.widget'
  | 'teacher.outline.node-action'
  | 'teacher.skill-store.preview'
  | 'teacher.activity.editor'
  // Global slots
  | 'global.navbar.action'
  | 'global.settings.panel'
  | 'admin.school.panel';

// -- Plugin Taxonomy ------------------------------------------

/** Classification of frontend plugins. */
export type PluginType =
  | 'skill_renderer'
  | 'dashboard_widget'
  | 'editor_extension'
  | 'theme'
  | 'page_extension';

/**
 * Trust level determines the sandbox isolation strategy.
 * - core/domain: Shadow DOM (can access React Context)
 * - community: iframe sandbox (postMessage only)
 */
export type TrustLevel = 'core' | 'domain' | 'community';

/** Supported interaction modes for Skill UI Renderers. */
export type InteractionMode = 'text' | 'voice' | 'canvas' | 'formula' | 'code';

// -- Skill UI Renderer ----------------------------------------

/** Student context passed into Skill Renderers. */
export interface StudentContext {
  studentId: number;
  displayName: string;
  courseId: number;
  sessionId: number;
}

/** Knowledge point information passed to renderers. */
export interface KnowledgePointContext {
  id: number;
  title: string;
  difficulty: number;
  chapterTitle: string;
}

/** Interaction event reported by renderers for learning analytics. */
export interface InteractionEvent {
  type: string;
  payload: Record<string, unknown>;
  timestamp: number;
}

/** WebSocket channel abstraction for Agent communication. */
export interface AgentWebSocketChannel {
  send: (message: string) => void;
  onMessage: (handler: (data: string) => void) => void;
  onClose: (handler: () => void) => void;
  close: () => void;
}

/**
 * Props injected into every Skill UI Renderer component.
 * The renderer uses these to interact with the learning session.
 */
export interface SkillRendererProps {
  /** Current student context (identity, course, session). */
  studentContext: StudentContext;
  /** Active knowledge point being learned. */
  knowledgePoint: KnowledgePointContext;
  /** Current scaffolding intensity. */
  scaffoldingLevel: 'high' | 'medium' | 'low';
  /** Real-time communication channel with Coach Agent. */
  agentChannel: AgentWebSocketChannel;
  /** Report interaction events for learning analytics. */
  onInteractionEvent: (event: InteractionEvent) => void;
}

/**
 * A Skill UI Renderer provides a custom interaction experience
 * for a specific backend Skill. When a student session activates
 * a skill, the matching renderer is loaded into
 * `student.interaction.main`.
 */
export interface SkillUIRenderer {
  /** Must match the backend Skill ID (e.g. "general_concept_socratic"). */
  skillId: string;
  /** Renderer metadata for display in Skill Store. */
  metadata: {
    name: string;
    version: string;
    description: string;
    previewImage?: string;
    supportedInteractionModes: InteractionMode[];
  };
  /** The React component to render. */
  Component: React.FC<SkillRendererProps>;
}

// -- Dashboard Widget -----------------------------------------

/** Context data passed to dashboard widget plugins. */
export interface DashboardWidgetContext {
  courseId: number;
  courseTitle: string;
  [key: string]: unknown;
}

/**
 * A Dashboard Widget plugin renders a card/chart in the
 * teacher dashboard via the `teacher.dashboard.widget` slot.
 */
export interface DashboardWidgetPlugin {
  /** Unique widget identifier. */
  id: string;
  /** Display title shown in the widget card header. */
  title: string;
  /** Brief description for the widget catalog. */
  description?: string;
  /** The React component to render inside the widget card. */
  Component: React.FC<{ context: DashboardWidgetContext; data?: unknown }>;
  /**
   * Optional async data loader.
   * Called with the current context before rendering.
   * The resolved value is passed as the `data` prop.
   */
  dataLoader?: (context: DashboardWidgetContext) => Promise<unknown>;
}

// -- Plugin Registration (unified) ----------------------------

/**
 * A single registration entry in the Plugin Registry.
 * Covers all plugin types via a discriminated union on `type`.
 */
export interface PluginRegistration {
  /** Unique plugin identifier. */
  id: string;
  /** Human-readable name. */
  name: string;
  /** Plugin classification. */
  type: PluginType;
  /** Trust level — determines isolation strategy. */
  trustLevel: TrustLevel;
  /** Which slots this plugin renders into. */
  slots: SlotName[];
  /** Sort priority within a slot (lower = first). */
  priority: number;
  /** The React component to render. */
  Component: React.FC<Record<string, unknown>>;
  /** Plugin version string. */
  version?: string;
  /** Optional icon for UI chrome. */
  icon?: string;
}

// -- Sandbox Communication Protocol ---------------------------

/**
 * Standardised message format for host <-> community plugin
 * communication via postMessage (iframe sandbox).
 */
export interface PluginMessage {
  /** Message direction. */
  type: 'request' | 'response' | 'event';
  /** Unique message ID for request-response pairing. */
  id: string;
  /** Host API method being invoked. */
  method?: AllowedHostMethod;
  /** Payload data or return value. */
  payload?: unknown;
  /** Error description (response only). */
  error?: string;
}

/**
 * Whitelisted host API methods that community plugins
 * may call via postMessage.
 */
export type AllowedHostMethod =
  | 'getStudentContext'
  | 'getKnowledgePoint'
  | 'sendMessageToAgent'
  | 'reportInteractionEvent'
  | 'requestUIToast'
  | 'getThemeVariables';

// -- Frontend Plugin Manifest ---------------------------------

/**
 * Schema for a frontend plugin's `manifest.json` file.
 * Used by the plugin loader to discover and validate plugins.
 */
export interface PluginManifest {
  id: string;
  name: string;
  version: string;
  type: PluginType;
  skillId?: string;
  trust_level: TrustLevel;
  author: string;
  description: string;
  entry: string;
  slots: SlotName[];
  supported_interaction_modes?: InteractionMode[];
  dependencies?: Record<string, string>;
  permissions?: AllowedHostMethod[];
  resource_limits?: {
    max_bundle_size_kb?: number;
    max_memory_mb?: number;
  };
  preview?: {
    screenshot?: string;
    demo_video?: string;
  };
}
