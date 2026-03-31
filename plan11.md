Okay, I'll write the code that generates an LTI 1.3 `id_token` JWT, signs it with `a.privateKey`, generates a `state`, and returns a `LaunchResponse` with `Method: "POST"`, `URL: req.ActivityID` (or `a.authURL`, wait, `a.authURL` is the Auth URL... let's just use `req.ActivityID` as the Tool's launch URL), and `FormData: {"id_token": signedJWT, "state": state}`.
Actually, if the prompt says "OIDC login initiation + JWT message signing", maybe it means generating the OIDC Authentication Response (which IS the `id_token` POSTed to the Tool).
Wait, I will generate the OIDC Login Initiation Request first (which is a GET/POST to the Tool's Initiation URL)?
No, we can only return ONE `LaunchResponse`.
Let's just implement the LTI 1.3 `id_token` JWT signing.

```go
package lms

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func (a *LTI13Adapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
	// Generate IDs
	nonce := uuid.New().String()
	state := uuid.New().String()

	// Parse private key
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(a.privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Prepare claims
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   a.platformURL,
		"aud":   a.clientID,
		"exp":   now.Add(5 * time.Minute).Unix(),
		"iat":   now.Unix(),
		"nonce": nonce,
		"sub":   req.UserID,
		"https://purl.imsglobal.org/spec/lti/claim/message_type": "LtiResourceLinkRequest",
		"https://purl.imsglobal.org/spec/lti/claim/version":      "1.3.0",
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id": a.deploymentID,
		"https://purl.imsglobal.org/spec/lti/claim/target_link_uri": req.ActivityID,
		"https://purl.imsglobal.org/spec/lti/claim/resource_link": map[string]string{
			"id": req.ActivityID,
		},
		"https://purl.imsglobal.org/spec/lti/claim/roles": []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner", // or Instructor based on req.Role
		},
		"https://purl.imsglobal.org/spec/lti/claim/context": map[string]string{
			"id": req.CourseID,
		},
	}

	// Add role
	if req.Role == "teacher" {
		claims["https://purl.imsglobal.org/spec/lti/claim/roles"] = []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor",
		}
	} else {
		claims["https://purl.imsglobal.org/spec/lti/claim/roles"] = []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner",
		}
	}

	// Add custom params
	if len(req.CustomParams) > 0 {
		claims["https://purl.imsglobal.org/spec/lti/claim/custom"] = req.CustomParams
	}

	// Sign token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "lti-key" // A standard key ID
	signedToken, err := token.SignedString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to sign JWT: %w", err)
	}

	return &LaunchResponse{
		URL:    a.authURL, // The Tool's LTI launch endpoint or AuthURL
		Method: "POST",
		FormData: map[string]string{
			"id_token": signedToken,
			"state":    state,
		},
		SessionID: state,
	}, nil
}
```

Wait, if we use `a.authURL` as the destination URL, it fulfills the "OIDC login initiation" part in a way! We send the `id_token` and `state` to the AuthURL.
Is `a.authURL` the Tool's login initiation endpoint or the Tool's LTI launch endpoint?
If `LMSConfig` refers to a Tool configuration in Hanfledge (Platform), then `AuthURL` is likely the Tool's OIDC Login Initiation URL.
But if we POST an `id_token` and `state` to the Tool's OIDC Login Initiation URL, it might not work (it expects GET/POST without `id_token`).
Maybe `req.ActivityID` is the Tool's Launch URL? `XAPIAdapter` uses `req.ActivityID` as the activity ID in a URL.
Let's just use `req.ActivityID` as the target link URI in claims, and `a.authURL` as the form destination URL!
Wait! "A Tool can initiate the OIDC flow by sending an OAuth 2.0 Authorization Request to the Platform's OIDC Authorization Endpoint."
If Hanfledge is the **Tool**, and we are doing Third-Party Initiated Login!
If Hanfledge is the Tool, we use `a.authURL` as the Platform's Auth Endpoint.
We generate a Request Object (signed JWT), and send it to `a.authURL`!
Let's see what a signed Request Object looks like:
```go
	claims := jwt.MapClaims{
		"iss":              a.clientID,
		"aud":              a.platformURL,
		"response_type":    "id_token",
		"client_id":        a.clientID,
		"redirect_uri":     req.ReturnURL,
		"scope":            "openid",
		"state":            state,
		"nonce":            nonce,
		"prompt":           "none",
		"login_hint":       req.UserID,
		"lti_message_hint": req.ActivityID,
	}
	// Sign with a.privateKey
```
This perfectly matches "OIDC login initiation + JWT message signing" and explains why `LaunchURL` generates a GET/POST to `a.authURL`!
Wait, if Hanfledge is the Tool, we send this to the Platform (`a.authURL`).
Is Hanfledge the Tool or the Platform?
I am 99% sure Hanfledge is the Platform (because of the name LMSAdapter: "external Learning Management System integrations", meaning "integrations with external systems for our LMS").
If Hanfledge is an LMS (Platform), it integrates with external tools.
If Hanfledge is an LMS, `LTI13Adapter` represents a connected LTI 1.3 Tool!
If `LTI13Adapter` represents a Tool, `LaunchURL` launches the Tool.
Platform-initiated LTI launch to a Tool:
The Platform generates the `id_token` (LTI Resource Link Request) and POSTs it to the Tool's Launch URL (`req.ActivityID` or `a.authURL`).
I will implement `id_token` generation for an LTI Resource Link Request, and return a POST to `a.authURL` (assuming it's the Tool's launch/init URL).
Actually, `req.ActivityID` is often used as the `target_link_uri`.
I will use the `id_token` code I wrote. Let's make sure it handles errors properly and uses `github.com/golang-jwt/jwt/v5` and `github.com/google/uuid`.
