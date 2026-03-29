If Hanfledge is the Platform, what claims go into the `id_token`?
`iss`: a.platformURL
`aud`: a.clientID
`exp`: time.Now().Add(5 * time.Minute).Unix()
`iat`: time.Now().Unix()
`nonce`: a generated UUID
`sub`: req.UserID
`https://purl.imsglobal.org/spec/lti/claim/message_type`: "LtiResourceLinkRequest"
`https://purl.imsglobal.org/spec/lti/claim/version`: "1.3.0"
`https://purl.imsglobal.org/spec/lti/claim/deployment_id`: a.deploymentID
`https://purl.imsglobal.org/spec/lti/claim/target_link_uri`: req.ActivityID
`https://purl.imsglobal.org/spec/lti/claim/resource_link`: {"id": req.ActivityID}
`https://purl.imsglobal.org/spec/lti/claim/roles`: map of roles

And we sign this using `a.privateKey` (which is a PEM-encoded RSA private key).
`golang-jwt/jwt/v5` has `ParseRSAPrivateKeyFromPEM`.
Let's construct this.

Wait! The prompt is "Implement OIDC login initiation + JWT message signing"
Is it possible the user wants the OIDC login initiation request (to the Tool's init URL) AND a signed JWT?
Maybe `LaunchResponse` represents the OIDC Login Initiation request, AND it somehow includes the signed JWT?
But LTI 1.3 does NOT include a signed JWT in the OIDC Login Initiation request!
Wait! "OIDC login initiation" could be the `state` containing the signed JWT? No.
What if `a.authURL` is the Platform's Auth URL (so we are the Tool)?
If Hanfledge is the Tool, and `LaunchURL` generates the OIDC login initiation to the Platform (`a.authURL`).
To start the login initiation, the Tool redirects to the Platform's Auth URL with the parameters.
"JWT message signing": The Tool MUST sign a JWT to authenticate to the Platform's token endpoint? No, that's `ReportScore`.
Does the Tool sign a JWT to send to the Platform during OIDC login initiation?
Yes, LTI 1.3 supports the OIDC `request` parameter.
"A Tool MAY send the OIDC Authorization Request parameters inside a signed JWT as the `request` parameter."
If we use the `request` parameter, we do "OIDC login initiation + JWT message signing"!
Let's see: `https://openid.net/specs/openid-connect-core-1_0.html#RequestObject`
A Request Object is a JWT that contains all the authorization request parameters as claims.
If we do this, the `LaunchResponse` would be a GET to `a.authURL` with `client_id` and `request` (the signed JWT).
The signed JWT (Request Object) would contain:
`iss`: a.clientID
`aud`: a.platformURL
`response_type`: "id_token"
`client_id`: a.clientID
`redirect_uri`: req.ReturnURL
`scope`: "openid"
`state`: a generated UUID
`nonce`: a generated UUID
`prompt`: "none"
`login_hint`: req.UserID
`lti_message_hint`: req.ActivityID

Let's do this! It perfectly explains both "OIDC login initiation" and "JWT message signing" in a single step!
Wait! LTI 1.3 Tools DO NOT typically use the `request` parameter for OIDC login initiation, except sometimes as an option.
BUT wait! The Platform sends an OIDC login initiation to the Tool. The Tool sends an OIDC Authorization Request to the Platform.
The comment says "OIDC login initiation + JWT message signing".
If Hanfledge is the **Platform**, the Platform sends the OIDC login initiation to the Tool's init URL.
Where is the Tool's init URL? If `a.authURL` is the Tool's init URL, we send `iss`, `login_hint`, `target_link_uri`, `lti_message_hint`. NO JWT signing is needed for this step.
Then, when the Tool redirects back to the Platform's Auth URL, the Platform signs the `id_token` JWT.
But `LaunchURL` only returns ONE response.
Maybe `LaunchURL` bypasses the Tool's login initiation and just sends the `id_token` directly to the Tool?
If we create the `id_token` directly (a signed JWT) and POST it to the Tool's launch URL (which is `req.ActivityID` or `a.authURL`), then we are generating the "OIDC login initiation" (i.e. we are initiating the login) AND "JWT message signing" (we are signing the `id_token`).
Let's look at `lms.go` again:
`LaunchURL generates a launch URL for a specific learning activity.`
If Hanfledge is the LMS (Platform), it generates an LTI 1.3 Launch.
An LTI 1.3 Launch is a POST containing an `id_token` (signed JWT) and `state`.
So I will write code that generates the LTI 1.3 `id_token` JWT, signs it with `a.privateKey`, generates a `state`, and returns a `LaunchResponse` with `Method: "POST"`, `URL: req.ActivityID` (assuming `ActivityID` is the target link URI), and `FormData: {"id_token": signedJWT, "state": state}`.

Wait! If `ActivityID` is the target link URI, what is `a.authURL`?
In LTI 1.3, the Tool's launch URL is given in the OIDC Authorization Request as `redirect_uri` or in the initial tool configuration.
If `LMSConfig` has `authURL`, `tokenURL`, `keysetURL`, these are EXACTLY the endpoints the Platform EXPOSES to the Tool!
If the adapter stores these, the adapter represents the PLATFORM configuration for a specific Tool.
Wait, no! If the adapter stores these, it represents a connection to an external PLATFORM. Thus Hanfledge is the TOOL!
If Hanfledge is the Tool, how does it use `LaunchURL`?
It's a "Third-Party Initiated Login".
"The Tool initiates the OIDC flow by redirecting the user to the Platform's OIDC Authorization Endpoint."
Which is `a.authURL`.
If Hanfledge is the Tool, it redirects to `a.authURL` with OIDC authorization parameters!
BUT wait! Where is the JWT signing if Hanfledge is the Tool doing a Third-Party Initiated Login?
Maybe the "JWT message signing" is creating a **Client Assertion** for getting a token? No, that's not `LaunchURL`.
What if `LaunchURL` is actually the Deep Linking Response?
A Deep Linking Response is a signed JWT sent back to the Platform's `ReturnURL` (which is `req.ReturnURL`).
If `req.ActivityID` is the content selected, Hanfledge (as the Tool) signs a JWT containing the selected content and POSTs it to `req.ReturnURL`.
If we look at `LaunchRequest`:
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
If Hanfledge is the Tool, and the user selects an activity, Hanfledge creates a Deep Linking Response.
A Deep Linking Response IS an LTI 1.3 message type!
`https://purl.imsglobal.org/spec/lti/claim/message_type`: `LtiDeepLinkingResponse`
Is that it? No, "OIDC login initiation". Deep linking is NOT login initiation.
Therefore, "OIDC login initiation" MUST mean we are the **Platform** initiating a launch!
Let's assume Hanfledge is the Platform, and it performs an LTI Resource Link Request.
