Wait! I need to search the internet for "LTI 1.3 OIDC login initiation JWT message signing".
Maybe "JWT message signing" refers to the LTI 1.3 Launch message (the `id_token`), which means Hanfledge is the **Platform**!
If Hanfledge is the Platform, Hanfledge MUST generate the `id_token` and sign it.
If Hanfledge is the Platform, why does `LMSConfig` have `TokenURL`?
Because LTI 1.3 Platforms also have Token URLs to give tokens to the Tools!
Wait, if Hanfledge is the Platform, `LMSConfig` configures a Tool integration in Hanfledge.
So `AuthURL` is the Tool's OIDC Login Initiation URL.
`TokenURL` is the Tool's... wait, Tools don't have Token URLs! They use the Platform's Token URL!
If `LMSConfig` configures an external Tool, why would it store `TokenURL` if the Tool doesn't have one?
Unless the `TokenURL` is the external LMS's Token URL, which proves the external system is the Platform!
Let's look at `lms.go`:
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
If Hanfledge is the Tool, this configuration matches exactly what Canvas gives to a Tool:
- Canvas's Client ID for the Tool (`ClientID`).
- Canvas's Deployment ID for the Tool (`DeploymentID`).
- Canvas's Issuer URL (`PlatformURL`).
- Canvas's JWKS URL (`KeysetURL`).
- Canvas's OIDC Auth URL (`AuthURL`).
- Canvas's OAuth2 Token URL (`TokenURL`).
- The Tool's private key to authenticate to Canvas (`PrivateKey`).

This fits PERFECTLY! This is the standard configuration a Tool needs to connect to an LTI 1.3 Platform!
So Hanfledge is the **Tool**. The external system is the **Platform** (e.g., Canvas).
If Hanfledge is the Tool, what does `LaunchURL` do?
`LaunchURL` generates a launch URL for a specific learning activity IN CANVAS? No, Canvas doesn't have learning activities launched by the Tool.
Wait! Maybe `LaunchURL` generates the URL that Canvas uses to launch Hanfledge!
If `LaunchURL` returns `LaunchResponse`, which has `URL`, `Method`, `FormData`, `SessionID`.
Why would Hanfledge generate a `LaunchResponse` to return to its OWN frontend?
Because Hanfledge's frontend wants to redirect the user to Canvas to START the launch!
This is exactly **Third-Party Initiated Login**!
"The Tool initiates the OIDC flow by redirecting the user to the Platform's OIDC Authorization Endpoint."
So the Tool (Hanfledge) generates a POST/GET to Canvas's `AuthURL` with OIDC parameters!
BUT wait! The comment says: "OIDC login initiation + JWT message signing".
If Hanfledge is doing a Third-Party Initiated Login, it sends an OIDC Authorization Request to `AuthURL`.
Where is the JWT message signing?
LTI 1.3 specifies that for Third-Party Initiated Login, the Tool CAN use the `request` parameter, which is a signed JWT.
Or, wait... What if Hanfledge is the Platform, and it's doing an LTI 1.3 launch to a Tool?
If Hanfledge is the Platform:
- `ClientID`: The Client ID Hanfledge assigned to the Tool.
- `DeploymentID`: The Deployment ID Hanfledge assigned to the Tool.
- `PlatformURL`: Hanfledge's Issuer URL.
- `KeysetURL`: Hanfledge's JWKS URL.
- `AuthURL`: The Tool's OIDC Initiation URL!
- `TokenURL`: Hanfledge's Token URL.
- `PrivateKey`: Hanfledge's private key (to sign the `id_token`).
If Hanfledge is the Platform, this ALSO fits perfectly!
And if Hanfledge is the Platform, it actually SIGNS the LTI 1.3 Launch message (`id_token`)!
This perfectly matches "OIDC login initiation [sending to AuthURL] + JWT message signing [signing the id_token]"!
Actually, the Platform MUST sign the `id_token` for an LTI Launch!
And the Platform MUST do OIDC login initiation with the Tool!
So Hanfledge IS the Platform!
Why would `lms.go` call it "external Learning Management System integrations"?
Because Hanfledge might act as a central hub connecting to Canvas/Moodle, OR it connects to external Tools (like SCORM/xAPI/LTI providers) to provide content for Hanfledge courses!
Wait, if Hanfledge is an LMS, it integrates with external SCORM packages, xAPI content, and LTI 1.3 tools.
So the "LMSAdapter" means "an adapter for an external learning tool/system".
If Hanfledge is the Platform (LMS), then `LaunchURL` creates an LTI 1.3 Launch.
An LTI 1.3 Launch consists of:
1. (Optional) OIDC Login Initiation request to the Tool's initiation URL.
2. The Tool redirects to the Platform's Auth URL.
3. The Platform POSTs the signed `id_token` and `state` to the Tool's launch URL.

If `LaunchURL` only returns one `LaunchResponse`, it must generate the final POST to the Tool's launch URL with the signed `id_token` and `state`!
Let's assume `a.authURL` is the Tool's OIDC Initiation URL. If we bypass the initiation and directly POST the `id_token`, we use `req.ReturnURL` or `req.ActivityID` as the target link URI!
Let's generate the `id_token` JWT, sign it with `a.privateKey`, and return a POST to `req.ActivityID` with `id_token` and `state`.
