package lms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type agsScore struct {
	Timestamp        string  `json:"timestamp"`
	ScoreGiven       float64 `json:"scoreGiven"`
	ScoreMaximum     float64 `json:"scoreMaximum"`
	Comment          string  `json:"comment,omitempty"`
	ActivityProgress string  `json:"activityProgress"`
	GradingProgress  string  `json:"gradingProgress"`
	UserId           string  `json:"userId"`
}

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

func (a *LTI13Adapter) getAccessToken(ctx context.Context) (string, error) {
	parsedKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(a.privateKey))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	claims := jwt.MapClaims{
		"iss": a.clientID,
		"sub": a.clientID,
		"aud": a.tokenURL,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"jti": uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(parsedKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", signedToken)
	data.Set("scope", "https://purl.imsglobal.org/spec/lti-ags/scope/score")

	req, err := http.NewRequestWithContext(ctx, "POST", a.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

func (a *LTI13Adapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
	// TODO: Implement OIDC login initiation + JWT message signing
	return nil, fmt.Errorf("LTI 1.3 LaunchURL not yet implemented")
}

func (a *LTI13Adapter) ReportScore(ctx context.Context, req ScoreReport) error {
	token, err := a.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get AGS access token: %w", err)
	}

	gradingProgress := "FullyGraded"
	if req.Status == "in_progress" {
		gradingProgress = "PendingManual"
	}

	activityProgress := "Completed"
	if req.Status == "in_progress" {
		activityProgress = "InProgress"
	}

	scorePayload := agsScore{
		Timestamp:        req.Timestamp.Format(time.RFC3339),
		ScoreGiven:       req.Score,
		ScoreMaximum:     req.MaxScore,
		Comment:          req.Comment,
		ActivityProgress: activityProgress,
		GradingProgress:  gradingProgress,
		UserId:           req.UserID,
	}

	body, err := json.Marshal(scorePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal AGS score payload: %w", err)
	}

	endpoint := req.ActivityID + "/scores"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create AGS score request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/vnd.ims.lis.v1.score+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send AGS score request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AGS score request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
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
