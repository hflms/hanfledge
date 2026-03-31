package lms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SCORMAdapter implements LMSAdapter for SCORM 2004 protocol.
type SCORMAdapter struct {
	endpoint string
	apiKey   string
}

func NewSCORMAdapter(cfg LMSConfig) (*SCORMAdapter, error) {
	if cfg.SCORMEndpoint == "" {
		return nil, fmt.Errorf("SCORM 2004 requires endpoint configuration")
	}
	return &SCORMAdapter{
		endpoint: cfg.SCORMEndpoint,
		apiKey:   cfg.SCORMAPIKey,
	}, nil
}

func (a *SCORMAdapter) Type() AdapterType { return AdapterSCORM }

func (a *SCORMAdapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
	sessionID := fmt.Sprintf("scorm_%s_%s", req.UserID, req.ActivityID)

	formData := map[string]string{
		"user_id":     req.UserID,
		"course_id":   req.CourseID,
		"activity_id": req.ActivityID,
		"session_id":  sessionID,
	}

	if a.apiKey != "" {
		formData["api_key"] = a.apiKey
	}

	return &LaunchResponse{
		URL:       a.endpoint,
		Method:    "POST",
		FormData:  formData,
		SessionID: sessionID,
	}, nil
}

func (a *SCORMAdapter) ReportScore(ctx context.Context, req ScoreReport) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal SCORM score report: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create SCORM score request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send SCORM score report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("SCORM score report failed with status: %s", resp.Status)
	}

	return nil
}

func (a *SCORMAdapter) SyncRoster(ctx context.Context, courseID string) (*Roster, error) {
	// SCORM doesn't have native roster sync — return unsupported
	return nil, fmt.Errorf("SCORM 2004 does not support roster sync")
}

func (a *SCORMAdapter) Validate(ctx context.Context) error {
	if a.endpoint == "" {
		return fmt.Errorf("SCORM 2004: endpoint is required")
	}
	return nil
}
