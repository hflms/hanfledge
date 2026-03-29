package lms

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
	if a.privateKey == "" {
		return nil, fmt.Errorf("LTI 1.3 private key is required for JWT signing")
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(a.privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse LTI 1.3 private key: %w", err)
	}

	nonce := uuid.New().String()
	state := uuid.New().String()

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   a.platformURL,
		"aud":   a.clientID,
		"exp":   now.Add(5 * time.Minute).Unix(),
		"iat":   now.Unix(),
		"nonce": nonce,
		"sub":   req.UserID,
		"https://purl.imsglobal.org/spec/lti/claim/message_type":    "LtiResourceLinkRequest",
		"https://purl.imsglobal.org/spec/lti/claim/version":         "1.3.0",
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id":   a.deploymentID,
		"https://purl.imsglobal.org/spec/lti/claim/target_link_uri": req.ActivityID,
		"https://purl.imsglobal.org/spec/lti/claim/resource_link": map[string]string{
			"id": req.ActivityID,
		},
		"https://purl.imsglobal.org/spec/lti/claim/context": map[string]string{
			"id": req.CourseID,
		},
	}

	if req.Role == "teacher" {
		claims["https://purl.imsglobal.org/spec/lti/claim/roles"] = []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor",
		}
	} else {
		claims["https://purl.imsglobal.org/spec/lti/claim/roles"] = []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner",
		}
	}

	if len(req.CustomParams) > 0 {
		claims["https://purl.imsglobal.org/spec/lti/claim/custom"] = req.CustomParams
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "lti13-key"

	signedToken, err := token.SignedString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to sign LTI 1.3 JWT: %w", err)
	}

	targetURL := a.authURL
	if targetURL == "" {
		targetURL = req.ActivityID
	}

	return &LaunchResponse{
		URL:    targetURL,
		Method: "POST",
		FormData: map[string]string{
			"id_token": signedToken,
			"state":    state,
		},
		SessionID: state,
	}, nil
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
