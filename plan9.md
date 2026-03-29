If Hanfledge is the Tool, and `LMSAdapter` represents the connection to Canvas (the Platform):
- `LaunchURL`: Generates a launch URL for a specific learning activity IN CANVAS? No, the user is IN Hanfledge, and clicks a button to go to a Canvas activity?
Yes! Sometimes a tool provides a "Launch to LMS" button to go to a specific assignment.
BUT Canvas doesn't have an LTI launch endpoint for its own activities. It has Deep Linking.
Wait, if Hanfledge is the Tool, and the user is IN Hanfledge, and clicks a button to launch an LTI activity IN HANFLEDGE...
Wait. "LMSAdapter defines the contract for external Learning Management System integrations."
If Hanfledge is the Tool, `LaunchURL` might be the Tool's endpoint that Canvas calls to start the launch!
Wait! The Tool's OIDC login initiation endpoint!
When Canvas wants to launch Hanfledge, it sends a GET to Hanfledge's OIDC Init URL.
Then Hanfledge MUST redirect the user to Canvas's `AuthURL`.
BUT `LaunchURL` method is called internally by Hanfledge!
`func (a *LTI13Adapter) LaunchURL(ctx context.Context, req LaunchRequest) (*LaunchResponse, error)`
This looks like Hanfledge generates the URL *to return to the user* (e.g., to redirect them).
If it returns `Method: "POST", URL: a.authURL, FormData: {...}`, it redirects the user to Canvas's AuthURL!
Wait, if this is OIDC Login Initiation, Hanfledge (as the Tool) receives an OIDC Init request from Canvas.
Canvas provides: `iss`, `login_hint`, `target_link_uri`, `lti_message_hint`, `client_id`, `lti_deployment_id`.
Then Hanfledge (Tool) generates the Auth Request to `a.authURL` with these parameters.
BUT `req LaunchRequest` only has:
`UserID`, `CourseID`, `ActivityID`, `Role`, `ReturnURL`, `CustomParams`.
It DOES NOT have `iss`, `login_hint`, `lti_message_hint`, `client_id`, `lti_deployment_id`!
This means Hanfledge is NOT responding to Canvas's init request! Hanfledge is INITIATING the launch!
This is Third-Party Initiated Login!
Hanfledge wants to start a launch with Canvas for a specific user and activity.
So Hanfledge generates an OIDC Auth Request to `a.authURL` (Canvas's Auth URL).
What does Hanfledge send?
- `response_type`: "id_token"
- `client_id`: `a.clientID`
- `scope`: "openid"
- `redirect_uri`: `req.ReturnURL` (or Hanfledge's launch URL)
- `state`: a generated UUID
- `nonce`: a generated UUID
- `response_mode`: "form_post"
- `prompt`: "none"
- `login_hint`: `req.UserID`
- `lti_message_hint`: `req.ActivityID` (or `req.CourseID`)

Wait! "JWT message signing"
If it is Third-Party Initiated Login, does the Tool sign a JWT?
Yes! It can send the `request` parameter (a signed Request Object JWT)!
Or, wait... what if Hanfledge IS the Platform?
If Hanfledge is the Platform, it HAS TO SEND AN LTI LAUNCH to the external Tool!
If Hanfledge is the Platform, the Tool gives Hanfledge:
`ClientID` (wait, the Platform generates the ClientID for the Tool).
`DeploymentID` (Platform generates it).
`PlatformURL` (Hanfledge's Issuer).
`KeysetURL` (Hanfledge's JWKS).
`AuthURL` (Tool's OIDC Init URL? No, `AuthURL` is usually the Auth endpoint).
Wait, if Hanfledge is the Platform, why does `LMSConfig` have `TokenURL`?
Because the Tool might have a Token endpoint? No, LTI Tools don't have token endpoints.
BUT wait! The LTI 1.3 spec says a Platform might use OAuth 2.0 to get tokens for the Tool's APIs? No, the Tool uses OAuth to get tokens for the Platform's APIs (AGS/NRPS).
If `LMSAdapter` implements `ReportScore` and `SyncRoster`, it means Hanfledge (using this adapter) CALLS the external system to report a score, or CALLS the external system to sync a roster!
If Hanfledge calls the external system to report a score, Hanfledge is the Tool!
Because ONLY the Tool reports scores to the Platform (via AGS)!
ONLY the Tool syncs rosters from the Platform (via NRPS)!
This absolutely confirms: **Hanfledge is the LTI 1.3 Tool.**
The `LMSAdapter` represents the connection to the external LMS (Canvas, Moodle, etc.).
So Hanfledge (Tool) calls `LaunchURL` to do what?
To initiate an LTI launch for a user who is currently in Hanfledge!
This is Third-Party Initiated Login.
Wait. Does Third-Party Initiated Login sign a JWT?
Yes, if it uses a signed OIDC Request Object (`request` parameter).
Some Platforms (like Canvas) require or support it.
Or maybe "JWT message signing" refers to generating a `client_assertion` JWT for getting an access token?
But `LaunchURL` doesn't get an access token!
Wait! What if "JWT message signing" is actually a mock implementation of the *Platform's* LTI Launch, just for local testing?
No, the prompt says:
"Rationale: LTI 1.3 requires extensive specification knowledge and a reliable test environment to verify OIDC flow and JWT signatures."
This implies the implementation of `LaunchURL` is exactly the LTI 1.3 standard for the OIDC flow and JWT signatures.

If Hanfledge is the Tool, the standard way to initiate a launch from the Tool is to generate an OIDC Auth Request.
If we include a signed JWT as the `client_assertion` for the OIDC Auth Request?
No, `client_assertion` is for the Token endpoint, not the Auth endpoint.
What about the `request` parameter? It is a signed JWT!
Let's create the Request Object (signed JWT).
Claims for the Request Object:
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

Let's do this! This fulfills "OIDC login initiation" AND "JWT message signing"!
Wait, what if the adapter is actually a Platform adapter, and `ReportScore` means Hanfledge (Platform) *receives* the score?
`ReportScore(ctx context.Context, req ScoreReport) error`
If Hanfledge is the Platform, the adapter provides an interface for the internal logic to call out to the tool? No, the tool calls the platform.
Wait. "LMSAdapter defines the contract for external Learning Management System integrations."
"ReportScore sends a student's score back to the LMS."
This means Hanfledge sends the score TO the LMS. So Hanfledge is the Tool!

Okay, so Hanfledge is the Tool. The external system is the LMS (Platform).
Hanfledge wants to launch an activity in the LMS? No!
"LaunchURL generates a launch URL for a specific learning activity."
Usually, a Tool is launched BY the LMS.
But sometimes, a user logs into the Tool directly, and the Tool provides a "Go to LMS" button, which initiates a Third-Party Initiated Login to launch the Tool *within* the LMS frame, or just logs them into the LMS.
Wait! What if Hanfledge acts as a proxy, and the "learning activity" is actually hosted on Hanfledge, but we need to authenticate the user via the LMS using LTI 1.3?
If we need to authenticate the user via the LMS using LTI 1.3, we start the OIDC Login Initiation flow by redirecting the user to the LMS's `AuthURL`.
To do this, we generate a URL to `a.authURL` with the required parameters:
- `client_id`: `a.clientID`
- `login_hint`: `req.UserID`
- `lti_message_hint`: `req.ActivityID`
- `target_link_uri`: `req.ReturnURL` (or a known tool launch URL)

Wait! Could "JWT message signing" just mean we sign the JWT for the OIDC Auth Request (`request` parameter)?
Let's see the OIDC core spec for `request` parameter:
"The request parameter enables OpenID Connect requests to be passed in a single, self-contained parameter... It is a JWT whose Claims are the request parameters."
This perfectly fits "OIDC login initiation + JWT message signing" in one function!
Let's implement it.

Wait, I should check the `jwt` package.
`github.com/golang-jwt/jwt/v5` is what we use.
I will create a JWT, sign it with `a.privateKey` (parsing the PEM string), and return a `LaunchResponse` with `Method: "POST"` and `FormData` containing the OIDC parameters, including `request`.
Or maybe just `Method: "POST"`, `URL: a.authURL`, and `FormData` containing `id_token` and `state`?
Wait! "OIDC login initiation + JWT message signing".
If we generate an `id_token` and `state`, and POST it to `req.ReturnURL`, we are acting as the Platform!
Let's look at `XAPIAdapter.LaunchURL`:
`URL: fmt.Sprintf("%s/activities/%s", a.endpoint, req.ActivityID)`
If we are the Tool, why does XAPI launch `a.endpoint/activities/xxx`?
Because XAPI is an LRS (Learning Record Store). We just point the user to the activity URL, and the activity sends xAPI statements to our LRS? Or the activity is in the LRS?
If Hanfledge is an LMS, it launches XAPI activities, SCORM activities, and LTI 1.3 tools!
If Hanfledge IS an LMS (Platform), then `ReportScore` means Hanfledge's LMSAdapter interface allows the internal system to pretend to be an LMS but actually it delegates to an external system?
No! "ReportScore sends a student's score back to the LMS."
This means Hanfledge is a Tool!
If Hanfledge is a Tool, why does it have an `XAPIAdapter`?
"XAPIAdapter implements LMSAdapter for the xAPI protocol."
"LaunchURL: xAPI doesn't have a traditional launch flow — return a tracking endpoint"
"ReportScore: Implement xAPI statement submission. POST to LRS endpoint with Statement JSON."
This means Hanfledge sends xAPI statements TO the LRS! So Hanfledge is an xAPI Provider (a Tool)!
Hanfledge sends SCORM scores TO the LMS! So Hanfledge is a SCORM package (or acts like one)!
Hanfledge sends LTI 1.3 scores TO the LMS! So Hanfledge is an LTI 1.3 Tool!

Conclusion: **Hanfledge is the Tool.** The external system is the LMS (Canvas/Moodle/LRS).

Why does a Tool have `LaunchURL`?
If Hanfledge is the Tool, and a user is in Hanfledge, and they want to start an activity, maybe Hanfledge redirects them to the LMS, which then launches Hanfledge back?
Or maybe `LaunchURL` just returns the URL that the LMS should use to launch Hanfledge?
If it just returns a URL, why would `LaunchResponse` have `FormData` and `Method: "POST"`?
Ah! If Hanfledge is the Tool, and it needs to generate a launch URL for a specific learning activity...
What if `LaunchURL` is generating a **Deep Linking Response**?
When an instructor is in the LMS (Canvas) and clicks "Select Content", Canvas launches Hanfledge's Deep Linking endpoint.
Hanfledge shows a UI to pick an activity. The instructor picks `req.ActivityID`.
Then Hanfledge calls `LaunchURL(ctx, LaunchRequest{ActivityID: ..., ReturnURL: CanvasDeepLinkReturnURL})` to generate the Deep Linking Response!
A Deep Linking Response is a POST to `ReturnURL` with a signed JWT (`JWT message signing`!) containing the selected content (`OIDC login initiation`? No, Deep Linking is not OIDC login initiation).
But the comment says "OIDC login initiation + JWT message signing".

Wait! What if "JWT message signing" means we sign the JWT for the OIDC login initiation, like I said earlier (the `request` object)?
OR what if we are generating a JWT for a **Client Assertion** to use with `tokenURL`? No, that's not `LaunchURL`.

Let's look closely at: `// TODO: Implement OIDC login initiation + JWT message signing`
Could it mean:
1. Generate the OIDC login initiation request (redirect to `a.authURL` with parameters).
2. Generate a JWT and sign it for the `request` parameter?
Wait, if it's the `client_assertion` for a token? No.
If it's LTI 1.3 Third-Party Initiated Login, the Tool redirects to `a.authURL` with:
- `iss`: The Tool's Client ID
- `login_hint`: user ID
- `target_link_uri`: the Tool's launch URL
- `client_id`: `a.clientID`
- `lti_message_hint`: activity ID
Wait, does it require a signed JWT?
"The Tool MAY send the OIDC Authorization Request parameters inside a signed JWT as the `request` parameter."
Let's just generate the signed JWT and put it in the `request` parameter, and return `a.authURL` as the `URL` with `Method: "POST"` and `FormData`.
