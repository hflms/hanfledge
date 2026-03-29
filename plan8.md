If Hanfledge is the Platform, what is `a.authURL`?
If Hanfledge is the Platform, it configures a Tool using an LMSAdapter.
In `LMSConfig`:
`ClientID`: Tool's client ID.
`DeploymentID`: Tool's deployment ID.
`PlatformURL`: The issuer URL of our platform.
`KeysetURL`: Our JWKS endpoint.
`AuthURL`: The Tool's OIDC Login Initiation URL! (Ah! This makes sense!)
`TokenURL`: The Tool's OAuth2 Token URL? (Wait, does a Tool have a token URL? No, only the Platform does.)
Wait! If `TokenURL` is the Platform's token URL, then `LMSConfig` holds the Platform's configuration!
If `LMSConfig` holds the Platform's configuration, it means Hanfledge IS the Tool!
Why? Because the Platform gives the Tool: Client ID, Deployment ID, Platform URL, Keyset URL, Auth URL, Token URL.
These are exactly the fields in `LMSConfig`!
So Hanfledge is definitely an LTI 1.3 **Tool**!

If Hanfledge is the Tool, what does `LaunchURL` do?
"LaunchURL generates a launch URL for a specific learning activity."
If Hanfledge is the Tool, how does it launch a specific activity?
Third-Party Initiated Login!
The Tool wants to initiate an LTI launch into the Platform (for a specific activity).
The Tool redirects the user to the Platform's `AuthURL`.
Wait, OIDC login initiation MUST redirect to the Platform's `AuthURL`? No, the Tool redirects to its OWN login initiation, or to the Platform's Auth URL?
"The Tool initiates the OIDC flow by sending an OAuth 2.0 Authorization Request to the Platform's OIDC Authorization Endpoint."
So the Tool redirects the user to `a.authURL` (which is the Platform's OIDC Auth Endpoint).
Parameters for the Authorization Request:
- `response_type`: "id_token"
- `client_id`: `a.clientID`
- `scope`: "openid"
- `redirect_uri`: `req.ReturnURL` (or the tool's launch URL)
- `state`: a generated UUID
- `nonce`: a generated UUID
- `response_mode`: "form_post"
- `prompt`: "none"
- `login_hint`: `req.UserID`
- `lti_message_hint`: `req.ActivityID` (or CourseID)
But where is "JWT message signing"?
"JWT message signing" might mean the **Request Object** as the `request` parameter!
Or perhaps "JWT message signing" means creating the **Client Assertion** for getting an access token? But that's for API calls (like `ReportScore`), not `LaunchURL`.
Wait! Is there ANY other JWT signed during an LTI 1.3 Tool launch?
Yes, the Deep Linking Response (which is a signed JWT). But that's not "login initiation".

What if `LaunchURL` creates an `id_token` and acts as a **mock Platform**?
If this `LTI13Adapter` is used to mock an LMS for testing purposes?
No, "LMSAdapter defines the contract for external Learning Management System integrations."
Let's assume the user wants `LaunchURL` to do what an LTI 1.3 Platform does:
1. Generate an `id_token` JWT.
2. Sign it with `a.privateKey`.
3. Create a POST to `req.ActivityID` (assuming it's the Tool's launch URL).
4. `FormData` contains `id_token` and `state`.

Let's look at `XAPIAdapter`:
`URL: fmt.Sprintf("%s/activities/%s", a.endpoint, req.ActivityID)`
This clearly shows `req.ActivityID` is an identifier (like `1234`), not a full URL.
If `req.ActivityID` is just an ID, how can we POST an `id_token` to it? We need the Tool's launch URL!
Where is the Tool's launch URL?
If Hanfledge is the Platform, the Tool's launch URL would be in the config (e.g. `ToolLaunchURL`). But it's not.
Therefore, Hanfledge MUST be the Tool, and `req.ActivityID` is the ID of an activity in Hanfledge.
Wait, if Hanfledge is the Tool, and we use `XAPIAdapter`, we launch `fmt.Sprintf("%s/activities/%s", a.endpoint, req.ActivityID)`.
This means `a.endpoint` is the external LMS (LRS in xAPI), and we launch an activity *in that LMS*.
If we use `LTI13Adapter`, we want to launch an activity *in that external LMS* (which means the external LMS is the Tool, and we are the Platform launching it)!
Wait, if `a.endpoint` is the external system's endpoint, then Hanfledge is the Platform (or LMS), and it connects to external Tools via `LMSAdapter`!
If Hanfledge is the LMS (Platform), then the `LMSConfig` fields must mean:
`PlatformURL`: Our own URL (or the Tool's URL?).
Wait, if it's the Tool's URL, why is it called `PlatformURL`?
Maybe `LMSConfig` is generic, and `PlatformURL` means the external system's URL!
If the external system is an LMS (Canvas), and Hanfledge is a Tool connecting to Canvas, then `PlatformURL` is Canvas's URL.
Wait! "Hanfledge is an AI-Native EdTech platform... Backend stack: Go (Gin, GORM)..."
Usually, EdTech platforms can act as EITHER the Tool (LTI Provider) or the Platform (LTI Consumer).
Since it has an `LMSAdapter` to connect to external LMSs, it means Hanfledge integrates WITH Canvas/Moodle, so Hanfledge IS the Tool!
When Hanfledge (the Tool) wants to give a user a link to an activity in Canvas (the LMS)... wait. Why would a Tool link back to Canvas?
A Tool doesn't launch Canvas activities. Canvas launches Tool activities.
Unless... Hanfledge acts as the Platform (LMS) AND it launches external SCORM packages, xAPI courses, and LTI 1.3 Tools!
If Hanfledge IS the LMS, then `LMSAdapter` connects to external content providers (SCORM, xAPI, LTI Tools).
In this case, Hanfledge is the LTI 1.3 Platform, and the external system is the LTI 1.3 Tool.
If Hanfledge is the LTI 1.3 Platform:
- `ClientID`: The ID we gave the Tool.
- `DeploymentID`: The ID we gave the Tool.
- `PlatformURL`: The Tool's base URL? No, `PlatformURL` usually means the Platform's URL. But why put it in the adapter config? Maybe because different adapters use different subdomains?
- `KeysetURL`: The Tool's JWKS endpoint.
- `AuthURL`: The Tool's OIDC Login Initiation endpoint.
- `TokenURL`: The Tool's Token endpoint (wait, Tools don't have token endpoints. Oh wait, LTI 1.3 Tools don't have token endpoints, platforms do).
Wait, if `LMSConfig` has `TokenURL`, that MUST mean the external system is the Platform!
Because in LTI 1.3, ONLY the Platform has a Token URL (for the Tool to get tokens for AGS/NRPS).
If the external system has a Token URL, the external system is the Platform (Canvas).
Therefore, Hanfledge is the Tool!
This is conclusive. The `TokenURL` proves the external system is the Platform.
So Hanfledge is an LTI 1.3 Tool.
