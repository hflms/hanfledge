package lms

import (
	"context"
	"fmt"
	"time"
)

// -- LMS Adapter Interface (§7.2) --------------------------------

// LMSAdapter defines the contract for external Learning Management System integrations.
// Implementations: LTI 1.3, SCORM 2004, xAPI.
type LMSAdapter interface {
	// Type returns the adapter type identifier.
	Type() AdapterType

	// LaunchURL generates a launch URL for a specific learning activity.
	LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error)

	// ReportScore sends a student's score back to the LMS.
	ReportScore(ctx context.Context, req ScoreReport) error

	// SyncRoster fetches the course roster (students/teachers) from the LMS.
	SyncRoster(ctx context.Context, courseID string) (*Roster, error)

	// Validate checks the adapter configuration and connectivity.
	Validate(ctx context.Context) error
}

// AdapterType identifies the LMS protocol.
type AdapterType string

const (
	AdapterLTI13 AdapterType = "lti_1.3"
	AdapterSCORM AdapterType = "scorm_2004"
	AdapterXAPI  AdapterType = "xapi"
)

// LaunchRequest contains parameters for launching a learning activity.
type LaunchRequest struct {
	UserID       string            `json:"user_id"`
	CourseID     string            `json:"course_id"`
	ActivityID   string            `json:"activity_id"`
	Role         string            `json:"role"` // "student", "teacher"
	ReturnURL    string            `json:"return_url,omitempty"`
	CustomParams map[string]string `json:"custom_params,omitempty"`
}

// LaunchResponse contains the result of a launch request.
type LaunchResponse struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"` // "GET" or "POST"
	FormData  map[string]string `json:"form_data,omitempty"`
	SessionID string            `json:"session_id"`
}

// ScoreReport contains a student's score to be reported back to the LMS.
type ScoreReport struct {
	UserID     string    `json:"user_id"`
	CourseID   string    `json:"course_id"`
	ActivityID string    `json:"activity_id"`
	Score      float64   `json:"score"` // 0.0 - 1.0
	MaxScore   float64   `json:"max_score"`
	Comment    string    `json:"comment,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Status     string    `json:"status"` // "completed", "in_progress", "not_attempted"
}

// Roster contains the course roster synced from the LMS.
type Roster struct {
	CourseID string        `json:"course_id"`
	Members  []RosterEntry `json:"members"`
}

// RosterEntry is a single member in a course roster.
type RosterEntry struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`   // "student", "teacher", "admin"
	Status string `json:"status"` // "active", "inactive"
}

// LMSConfig holds configuration for an LMS adapter.
type LMSConfig struct {
	Type AdapterType `json:"type"`

	// LTI 1.3 settings
	ClientID     string `json:"client_id,omitempty"`
	DeploymentID string `json:"deployment_id,omitempty"`
	PlatformURL  string `json:"platform_url,omitempty"`
	KeysetURL    string `json:"keyset_url,omitempty"`
	AuthURL      string `json:"auth_url,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"`

	// SCORM 2004 settings
	SCORMEndpoint string `json:"scorm_endpoint,omitempty"`
	SCORMAPIKey   string `json:"scorm_api_key,omitempty"`

	// xAPI settings
	XAPIEndpoint string `json:"xapi_endpoint,omitempty"`
	XAPIKey      string `json:"xapi_key,omitempty"`
	XAPISecret   string `json:"xapi_secret,omitempty"`
}

// NewAdapter creates an LMS adapter based on configuration.
func NewAdapter(cfg LMSConfig) (LMSAdapter, error) {
	switch cfg.Type {
	case AdapterLTI13:
		return NewLTI13Adapter(cfg)
	case AdapterSCORM:
		return NewSCORMAdapter(cfg)
	case AdapterXAPI:
		return NewXAPIAdapter(cfg)
	default:
		return nil, fmt.Errorf("unsupported LMS adapter type: %s", cfg.Type)
	}
}
