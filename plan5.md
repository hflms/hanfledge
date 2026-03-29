There are no usages of `LaunchURL` or `LTI13Adapter` outside of the LMS package yet.
The prompt asks to implement: "OIDC login initiation + JWT message signing".
So I need to provide a complete LTI 1.3 `LaunchResponse` object from `LaunchURL`.

If we are acting as the LMS (Platform) launching an external tool:
Usually, the OIDC login initiation is the *first* step: we send an OIDC request (a GET or POST) to the tool's initiation endpoint.
Wait! If Hanfledge is the LMS, the tool's initiation endpoint is what we are hitting.
But wait! "JWT message signing" happens in the *second* step (the Auth Response / LTI Launch) where we send the signed `id_token`.
If we put BOTH together in `LaunchURL`, maybe `LaunchURL` just creates the final form to POST to the Tool, bypassing the Tool's initiation endpoint? (Sometimes platforms allow this, but it's not standard LTI 1.3).
Wait, look at the LTI 1.3 standard: "A Tool MAY support third-party initiated login". But the Platform MUST always do an OIDC Launch!
Platform sends to Tool: `iss`, `target_link_uri`, `login_hint`, `lti_message_hint`, `client_id`, `lti_deployment_id`.
Then Tool redirects to Platform Auth endpoint with `scope=openid`, `response_type=id_token`, `client_id`, `redirect_uri`, `login_hint`, `state`, `response_mode=form_post`, `nonce`, `prompt=none`.
Then Platform constructs the `id_token` JWT, signs it, and POSTs to the Tool's `redirect_uri` with `id_token` and `state`.

If `LaunchURL` is "OIDC login initiation + JWT message signing", maybe it returns a `LaunchResponse` that IS the OIDC login initiation request, BUT wait... where does "JWT message signing" fit?
Ah! If the system is a **Platform**, it *could* implement `LaunchURL` by generating the OIDC login initiation URL (to send the user to the tool's init endpoint), AND simultaneously preparing the JWT so it's ready? No.
Maybe Hanfledge is the **Tool**, and `LMSAdapter` is used by Hanfledge to talk to the LMS (Canvas)!
If Hanfledge is the Tool, and `LMSAdapter` is used, then the "LTI 1.3 Adapter" configures how Hanfledge connects to Canvas.
If Hanfledge is the Tool, `LaunchURL` might be used for **Third-Party Initiated Login** (i.e. Hanfledge starts the launch to Canvas).
In Third-Party Initiated Login, the Tool redirects the user to Canvas's OIDC Auth URL (`authURL`).
Does the Tool sign a JWT for Third-Party Initiated Login? No. It just sends GET parameters.

Let's read the prompt carefully.
`func (a *LTI13Adapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error)`
"Complete the LaunchURL method with OIDC and JWT implementation."
"Rationale: LTI 1.3 requires extensive specification knowledge and a reliable test environment to verify OIDC flow and JWT signatures."
If we must generate a JWT signature, then our adapter is acting as the **Platform** (generating the `id_token`) OR it is generating a **Client Assertion** to authenticate to the Platform's `tokenURL`? No, `LaunchURL` doesn't get a token.
Wait! Is it possible that `LaunchURL` creates a mock LTI Launch directly? Yes, maybe!
Wait! What if we create a JWT that is the *LTI Resource Link Request*?
The JWT claims for LTI Resource Link Request:
- `iss`: Platform URL
- `aud`: Client ID
- `sub`: User ID
- `exp`, `iat`, `nonce`
- `https://purl.imsglobal.org/spec/lti/claim/message_type`: `LtiResourceLinkRequest`
- `https://purl.imsglobal.org/spec/lti/claim/version`: `1.3.0`
- `https://purl.imsglobal.org/spec/lti/claim/roles`: Array of roles
- `https://purl.imsglobal.org/spec/lti/claim/resource_link`: `{ "id": ActivityID }`
- `https://purl.imsglobal.org/spec/lti/claim/deployment_id`: Deployment ID

Let's assume the LMS adapter is acting as the **Platform** and `LaunchURL` directly generates the form POST to the tool's launch URL.
BUT where is the tool's launch URL?
In the `LMSConfig`, we have `AuthURL`, `PlatformURL`, `KeysetURL`, `TokenURL`.
If this was a Tool config for a Platform, it would have the Tool's launch URL. But it doesn't! It only has `AuthURL` and `TokenURL`.
Wait. If `LMSAdapter` configures an *LMS integration*, then `AuthURL` is the LMS's Auth URL, `TokenURL` is the LMS's Token URL, `PlatformURL` is the LMS's Issuer URL. This means we are the **Tool**!
If we are the Tool, and we are connecting to an LMS, why does `LaunchURL` sign a JWT?
Maybe `LaunchURL` generates an `id_token` because... wait, if we are the Tool, we *don't* generate an `id_token` for launch. The Platform does.
Unless... our system is acting as a **Content Provider** (like a SCORM/xAPI provider, or LTI Tool).
Wait! SCORM and xAPI adapters are also here.
If Hanfledge is a Content Provider (Tool), then Hanfledge *is* the LTI 1.3 Tool, and `LMSAdapter` represents an LMS (Canvas) that we are connected to.
But if we are the Tool, why do we have a `LaunchURL` method to "launch a specific learning activity"?
If we are the Tool, the LMS launches *us*. We don't launch the LMS.
Unless `LaunchURL` is generating a URL that the LMS uses to launch us?
If `LaunchURL` generates the URL that we give to the LMS (e.g. as a Deep Link), then `LaunchResponse` would just be a URL. But it has `Method` and `FormData`!
What if Hanfledge is an LMS, and `LMSAdapter` is an adapter to connect to *other* LMSs? (e.g. Canvas). This means Hanfledge acts as a Tool inside Canvas, AND Hanfledge also acts as an LMS.
If Hanfledge acts as a Tool, it might use `LaunchURL` to do a **Deep Linking Request**? No, Deep Linking Request is also sent by the Platform. A tool sends a Deep Linking *Response*. A Deep Linking Response is a signed JWT!
Ah! A Deep Linking Response is a signed JWT containing the selected content items, POSTed to the Platform's `return_url`.
Let's look at `LaunchRequest`:
```go
type LaunchRequest struct {
	UserID       string            `json:"user_id"`
	CourseID     string            `json:"course_id"`
	ActivityID   string            `json:"activity_id"`
	Role         string            `json:"role"` // "student", "teacher"
	ReturnURL    string            `json:"return_url,omitempty"`
	CustomParams map[string]string `json:"custom_params,omitempty"`
}
```
If we are the Tool sending a Deep Linking Response, we POST a JWT to `ReturnURL`!
Wait! "OIDC login initiation + JWT message signing".
If we are the Platform, we do OIDC login initiation by sending a GET/POST to the Tool's OIDC Init URL, AND we do JWT message signing for the `id_token`.
But where is the Tool's URL? Maybe `ActivityID` is the Tool's URL?
Wait, the `XAPIAdapter` does:
```go
func (a *XAPIAdapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
	// xAPI doesn't have a traditional launch flow — return a tracking endpoint
	return &LaunchResponse{
		URL:       fmt.Sprintf("%s/activities/%s", a.endpoint, req.ActivityID),
		Method:    "GET",
		SessionID: fmt.Sprintf("xapi_%s_%s", req.UserID, req.ActivityID),
	}, nil
}
```
If `XAPIAdapter` returns a URL to the xAPI endpoint, it means the adapter is pointing to an external system, and `LaunchURL` gives the URL to launch the activity in that external system!
So Hanfledge IS the Platform (LMS), and we are launching an external LTI Tool!
If we are the Platform, the Tool's LTI launch URL MUST be known. Where is it?
Maybe `req.ActivityID` IS the Tool's launch URL?
Or maybe `req.ReturnURL`? No, `ReturnURL` is where the tool should return.
Wait, if `ActivityID` is the tool's launch URL, then we can use it.
But what about "OIDC login initiation"?
To launch an LTI 1.3 Tool, the Platform must first redirect the user to the Tool's OIDC Login Initiation URL.
Where is the Tool's OIDC Login Initiation URL?
In `LMSConfig`:
```go
	// LTI 1.3 settings
	ClientID     string `json:"client_id,omitempty"`
	DeploymentID string `json:"deployment_id,omitempty"`
	PlatformURL  string `json:"platform_url,omitempty"`
	KeysetURL    string `json:"keyset_url,omitempty"`
	AuthURL      string `json:"auth_url,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"`
```
If we are the Platform, these fields in `LMSConfig` for LTI 1.3 must represent the **Tool's** configuration, right?
Wait! `PlatformURL`, `KeysetURL`, `AuthURL`, `TokenURL` - these are the names of the **Platform's** endpoints!
Why would the adapter configuration contain the Platform's endpoints if Hanfledge IS the Platform?
Because Hanfledge might have different Platform settings per tenant, or the adapter just stores them to use.
Wait! If Hanfledge is the Tool, then these are the Platform's endpoints.
Let's assume Hanfledge is the Platform, and we need to launch an activity. But wait, if we are the Platform, we should know the Tool's OIDC Initiation URL. Where is it? It's missing! `LMSConfig` doesn't have `InitURL` or `ToolLaunchURL`.
What if `AuthURL` IS the Tool's OIDC Initiation URL?
No, the names are exactly what a Tool needs from a Platform!
If you configure an LTI Tool in Canvas, you give Canvas your Init URL and Launch URL, and Canvas gives you its Issuer (PlatformURL), Keyset URL, Auth URL, and Token URL.
Since `LMSConfig` stores exactly what Canvas gives to a Tool, this means `LMSAdapter` represents a connection TO a Platform! Therefore, Hanfledge is the **Tool**!
If Hanfledge is the Tool, how does it use `LaunchURL`?
Hanfledge (as a Tool) wants to launch a specific learning activity *in the Platform*? No, LTI only goes from Platform to Tool.
Wait! A Tool can use **Deep Linking Request** to create a launch URL *for* the Platform!
No, Deep Linking provides content to the Platform.
What if `LaunchURL` creates the **Third-Party Initiated Login** URL?
LTI 1.3 Third-Party Initiated Login (OIDC):
The Tool generates a URL to the Platform's `AuthURL` to start the LTI launch.
BUT the prompt says: "Implement OIDC login initiation + JWT message signing"
If it's Third-Party Initiated Login, there is NO JWT signed. The Tool simply sends `iss`, `login_hint`, `target_link_uri`, `lti_message_hint` to the Platform's OIDC auth endpoint.
Where does "JWT message signing" come in?
Maybe Hanfledge IS the Platform! And `AuthURL` means the Tool's Auth/Init URL?
Let's look at `go.mod`. We have `github.com/golang-jwt/jwt/v5`.
If Hanfledge is the Platform, it signs the `id_token` JWT.
Let's generate the JWT with the claims.
