package lms

import (
	"context"
	"fmt"
)

// LTI13Adapter implements LMSAdapter for LTI 1.3 protocol.
// Reference: IMS Global LTI 1.3 specification.
type LTI13Adapter struct {
	clientID     string
	deploymentID string
	platformURL  string
	keysetURL    string
	authURL      string
	tokenURL     string
	privateKey   string
}

func NewLTI13Adapter(cfg LMSConfig) (*LTI13Adapter, error) {
	if cfg.ClientID == "" || cfg.PlatformURL == "" {
		return nil, fmt.Errorf("LTI 1.3 requires client_id and platform_url")
	}
	return &LTI13Adapter{
		clientID:     cfg.ClientID,
		deploymentID: cfg.DeploymentID,
		platformURL:  cfg.PlatformURL,
		keysetURL:    cfg.KeysetURL,
		authURL:      cfg.AuthURL,
		tokenURL:     cfg.TokenURL,
		privateKey:   cfg.PrivateKey,
	}, nil
}

func (a *LTI13Adapter) Type() AdapterType { return AdapterLTI13 }

func (a *LTI13Adapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
	// TODO: Implement OIDC login initiation + JWT message signing
	return nil, fmt.Errorf("LTI 1.3 LaunchURL not yet implemented")
}

func (a *LTI13Adapter) ReportScore(ctx context.Context, req ScoreReport) error {
	// TODO: Implement AGS (Assignment and Grade Services) score passback
	return fmt.Errorf("LTI 1.3 ReportScore not yet implemented")
}

func (a *LTI13Adapter) SyncRoster(ctx context.Context, courseID string) (*Roster, error) {
	// TODO: Implement NRPS (Names and Role Provisioning Services)
	return nil, fmt.Errorf("LTI 1.3 SyncRoster not yet implemented")
}

func (a *LTI13Adapter) Validate(ctx context.Context) error {
	if a.clientID == "" {
		return fmt.Errorf("LTI 1.3: client_id is required")
	}
	if a.platformURL == "" {
		return fmt.Errorf("LTI 1.3: platform_url is required")
	}
	return nil
}
