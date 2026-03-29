Wait, if it's "OIDC login initiation + JWT message signing", maybe our application is the **Tool** but we are sending a **ResourceLinkRequest**? No, only the platform sends a ResourceLinkRequest. A tool sends a `DeepLinkingResponse`.
But a tool *does* do OIDC login initiation. The Tool redirects the user to the platform's `authURL`. The tool does *not* sign a JWT for login initiation!
Wait, if our application is the **Platform**, we *do* send OIDC login initiation (to the tool's initiation endpoint), and then the tool redirects back to the platform's AuthURL. Then the platform creates the JWT message and signs it, sending it to the tool.
BUT look at the `LTI13Adapter` fields:
- `clientID`: The client ID given by the platform to the tool.
- `deploymentID`: The deployment ID.
- `platformURL`: The issuer URL of the platform.
- `keysetURL`: The platform's JWKS URL.
- `authURL`: The platform's OIDC Auth URL.
- `tokenURL`: The platform's OAuth2 Token URL.
- `privateKey`: The **tool's** private key (used to sign JWTs for client assertions when getting tokens for AGS/NRPS!).

If our application is the **Tool** (the adapter is an adapter to connect TO a specific LMS/Platform), then:
- Why do we have `LaunchURL`? "LaunchURL generates a launch URL for a specific learning activity."
If we are the Tool, we don't usually generate a "LaunchURL" for the user to go to the LMS. The user is *in* the LMS and clicks a link to launch the Tool.
UNLESS... this is a **Platform** and these fields are for configuring a Tool integration on our Platform?
If we are the Platform:
- `clientID`: ID given to the Tool.
- `deploymentID`: Deployment ID of the Tool.
- `platformURL`: Our platform URL.
- `keysetURL`: Tool's JWKS? Or our JWKS? If it's our Platform URL, maybe these are our platform endpoints we configure the adapter with.
Wait, if we are the Platform, why would we need an adapter to integrate with our own Platform? That doesn't make sense. The adapter is for *external* Learning Management System integrations.
If we are the Tool, maybe `LaunchURL` is used to generate a link to start the **Third-Party Initiated Login**?
In Third-Party Initiated Login (OIDC), the tool provides a URL to the user. The user clicks it. The tool generates an OIDC Login Initiation request and redirects the user to the platform's `authURL`. Wait, the standard says the Tool redirects to the Platform's OIDC Authorization Endpoint (`authURL`) with a signed `id_token`? NO. The OIDC Auth Request is an HTTP GET/POST with query/form parameters. NO JWT signing is needed for the authorization request itself, unless it's using OIDC `request` parameter (which LTI doesn't typically require, it uses plain parameters, though sometimes it's allowed).
Wait, "Implement OIDC login initiation + JWT message signing".
If the prompt says "JWT message signing", then what JWT are we signing?
- If we are the **Tool**, we sign a JWT for **Client Assertion** (to get an access token via `tokenURL` for AGS/NRPS).
- If we are the **Platform**, we sign the **LTI Launch message** (`id_token`).
Let's look at `ReportScore`. "TODO: Implement AGS (Assignment and Grade Services) score passback".
To do AGS, the Tool needs to get an OAuth2 access token from the `tokenURL` using a JWT client assertion signed with `privateKey`.
So our application is the **Tool**.
Why does the Tool have a `LaunchURL` method that says "Implement OIDC login initiation + JWT message signing"?
Ah! Is `LaunchURL` actually meant to do the full OIDC flow from the Tool side?
Wait, if the user starts on our app (the Tool) and wants to launch an activity in the LMS? No, LTI always launches *into* the Tool.
Maybe we are the **Platform** after all!
"LMSAdapter defines the contract for external Learning Management System integrations."
"LaunchURL generates a launch URL for a specific learning activity."
If we are the platform, we want to launch an external Tool.
"Implementations: LTI 1.3, SCORM 2004, xAPI."
SCORM and xAPI are content formats. If we launch a SCORM package, we host it and generate a URL to the player.
If we launch an LTI 1.3 activity, we act as the Platform launching an external Tool.
Let's check `scorm.go` and `xapi.go`.
