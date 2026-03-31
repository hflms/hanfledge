package lms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// XAPIAdapter implements LMSAdapter for the xAPI (Experience API) protocol.
// Also known as Tin Can API.
type XAPIAdapter struct {
	endpoint string
	key      string
	secret   string
}

func NewXAPIAdapter(cfg LMSConfig) (*XAPIAdapter, error) {
	if cfg.XAPIEndpoint == "" {
		return nil, fmt.Errorf("xAPI requires endpoint configuration")
	}
	return &XAPIAdapter{
		endpoint: cfg.XAPIEndpoint,
		key:      cfg.XAPIKey,
		secret:   cfg.XAPISecret,
	}, nil
}

func (a *XAPIAdapter) Type() AdapterType { return AdapterXAPI }

func (a *XAPIAdapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
	// xAPI doesn't have a traditional launch flow — return a tracking endpoint
	return &LaunchResponse{
		URL:       fmt.Sprintf("%s/activities/%s", a.endpoint, req.ActivityID),
		Method:    "GET",
		SessionID: fmt.Sprintf("xapi_%s_%s", req.UserID, req.ActivityID),
	}, nil
}

func (a *XAPIAdapter) ReportScore(ctx context.Context, req ScoreReport) error {
	verbID := "http://adlnet.gov/expapi/verbs/progressed"
	if req.Status == "completed" {
		verbID = "http://adlnet.gov/expapi/verbs/completed"
	} else if req.Status == "not_attempted" {
		verbID = "http://adlnet.gov/expapi/verbs/abandoned"
	}

	statement := map[string]interface{}{
		"actor": map[string]interface{}{
			"objectType": "Agent",
			"account": map[string]interface{}{
				"homePage": "http://hanfledge.internal/",
				"name":     req.UserID,
			},
		},
		"verb": map[string]interface{}{
			"id": verbID,
			"display": map[string]string{
				"en-US": req.Status,
			},
		},
		"object": map[string]interface{}{
			"objectType": "Activity",
			"id":         req.ActivityID,
		},
		"result": map[string]interface{}{
			"score": map[string]interface{}{
				"scaled": req.Score,
				"raw":    req.Score * req.MaxScore,
				"min":    0.0,
				"max":    req.MaxScore,
			},
			"success":    req.Score >= 0.8,
			"completion": req.Status == "completed",
		},
		"timestamp": req.Timestamp.Format("2006-01-02T15:04:05.999Z"),
	}

	body, err := json.Marshal(statement)
	if err != nil {
		return fmt.Errorf("failed to marshal xAPI statement: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+"/statements", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create xAPI request: %w", err)
	}

	httpReq.Header.Set("X-Experience-API-Version", "1.0.3")
	httpReq.Header.Set("Content-Type", "application/json")
	if a.key != "" || a.secret != "" {
		httpReq.SetBasicAuth(a.key, a.secret)
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send xAPI statement: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected xAPI response status: %d", resp.StatusCode)
	}

	return nil
}

func (a *XAPIAdapter) SyncRoster(ctx context.Context, courseID string) (*Roster, error) {
	// xAPI doesn't have native roster sync
	return nil, fmt.Errorf("xAPI does not support roster sync")
}

func (a *XAPIAdapter) Validate(ctx context.Context) error {
	if a.endpoint == "" {
		return fmt.Errorf("xAPI: endpoint is required")
	}
	return nil
}
