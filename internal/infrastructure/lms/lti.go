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

// NRPSResponse represents the response from the LTI 1.3 Names and Role Provisioning Services.
type NRPSResponse struct {
	ID      string       `json:"id"`
	Context NRPSContext  `json:"context"`
	Members []NRPSMember `json:"members"`
}

// NRPSContext represents the context (course) in an NRPS response.
type NRPSContext struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Title string `json:"title"`
}

// NRPSMember represents a member (user) in an NRPS response.
type NRPSMember struct {
	Status             string     `json:"status"`
	Name               string     `json:"name"`
	Picture            string     `json:"picture"`
	GivenName          string     `json:"given_name"`
	FamilyName         string     `json:"family_name"`
	Email              string     `json:"email"`
	UserID             string     `json:"user_id"`
	Roles              []string   `json:"roles"`
	Message            []struct{} `json:"message"`
	LISPersonSourcedID string     `json:"lis_person_sourcedid"`
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

func (a *LTI13Adapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
	// TODO: Implement OIDC login initiation + JWT message signing
	return nil, fmt.Errorf("LTI 1.3 LaunchURL not yet implemented")
}

func (a *LTI13Adapter) ReportScore(ctx context.Context, req ScoreReport) error {
	// TODO: Implement AGS (Assignment and Grade Services) score passback
	return fmt.Errorf("LTI 1.3 ReportScore not yet implemented")
}

func (a *LTI13Adapter) getAccessToken(ctx context.Context, scopes []string) (string, error) {
	if a.privateKey == "" {
		return "", fmt.Errorf("LTI 1.3: private_key is required for access token generation")
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(a.privateKey))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    a.clientID,
		Subject:   a.clientID,
		Audience:  jwt.ClaimStrings{a.tokenURL},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		ID:        uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("failed to sign client assertion: %w", err)
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", signedToken)
	data.Set("scope", strings.Join(scopes, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get access token, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode access token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

func (a *LTI13Adapter) SyncRoster(ctx context.Context, courseID string) (*Roster, error) {
	// For LTI 1.3 NRPS, the courseID argument is treated as the context_memberships_url
	// provided by the platform during launch or configuration.
	membershipsURL := courseID
	if membershipsURL == "" {
		return nil, fmt.Errorf("LTI 1.3: context_memberships_url is required for roster sync")
	}

	scopes := []string{"https://purl.imsglobal.org/spec/lti-nrps/scope/contextmembership.readonly"}
	token, err := a.getAccessToken(ctx, scopes)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token for NRPS: %w", err)
	}

	var allMembers []NRPSMember
	var courseIDFromNRPS string

	nextURL := membershipsURL
	client := &http.Client{Timeout: 30 * time.Second}

	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create NRPS request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.ims.lti-nrps.v2.membershipcontainer+json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch NRPS data: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to fetch NRPS data, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
		}

		var nrpsResp NRPSResponse
		if err := json.NewDecoder(resp.Body).Decode(&nrpsResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode NRPS response: %w", err)
		}

		if courseIDFromNRPS == "" {
			courseIDFromNRPS = nrpsResp.Context.ID
		}
		allMembers = append(allMembers, nrpsResp.Members...)

		// Handle pagination via Link header
		// Format: <url>; rel="next"
		linkHeader := resp.Header.Get("Link")
		nextURL = ""
		if linkHeader != "" {
			links := strings.Split(linkHeader, ",")
			for _, l := range links {
				parts := strings.Split(l, ";")
				if len(parts) >= 2 && strings.Contains(parts[1], `rel="next"`) {
					nextURL = strings.Trim(strings.TrimSpace(parts[0]), "<>")
					break
				}
			}
		}
		resp.Body.Close()
	}

	roster := &Roster{
		CourseID: courseIDFromNRPS,
		Members:  make([]RosterEntry, 0, len(allMembers)),
	}

	for _, m := range allMembers {
		role := "student"
		for _, r := range m.Roles {
			lowerRole := strings.ToLower(r)
			if strings.Contains(lowerRole, "instructor") || strings.Contains(lowerRole, "teacher") {
				role = "teacher"
				break
			}
			if strings.Contains(lowerRole, "administrator") {
				role = "admin"
				break
			}
		}

		status := "active"
		if strings.EqualFold(m.Status, "Deleted") || strings.EqualFold(m.Status, "Inactive") {
			status = "inactive"
		}

		roster.Members = append(roster.Members, RosterEntry{
			UserID: m.UserID,
			Name:   m.Name,
			Email:  m.Email,
			Role:   role,
			Status: status,
		})
	}

	return roster, nil
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
