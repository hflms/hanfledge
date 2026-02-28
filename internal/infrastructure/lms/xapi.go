package lms

import (
	"context"
	"fmt"
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
	// TODO: Implement xAPI statement submission
	// POST to LRS endpoint with Statement JSON
	return fmt.Errorf("xAPI ReportScore not yet implemented")
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
