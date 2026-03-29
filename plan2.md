If it says "Implement OIDC login initiation + JWT message signing", maybe it means doing it in one step. Since we only return a LaunchResponse with FormData, it seems we are the LMS (Platform) and we want to launch the tool, bypassing the OIDC login initiation flow from the tool (which is sometimes called Third-Party Initiated Login) or rather, we are initiating it and directly creating the auth response.
Wait, no. The Tool has an OIDC Login Initiation URL.
A platform typically launches an LTI 1.3 Tool by:
1. POSTing or GETting to the Tool's OIDC login initiation URL (which can be `AuthURL` in the config).
   Parameters: `iss`, `target_link_uri`, `login_hint`, `lti_message_hint`, `client_id`, `lti_deployment_id`.
   If `LaunchResponse` Method is "GET" or "POST", we can construct this. But wait, where is the JWT signing coming in?
   The platform signs the `id_token` in the **LTI Launch request**, not the OIDC login initiation.
   If "OIDC login initiation + JWT message signing" is all done in `LaunchURL`, maybe `LaunchURL` actually constructs the LTI Launch form POST with the signed `id_token`?
   Some tools accept the OIDC launch without initiation? No, OIDC requires a nonce/state from the tool.
   Unless this adapter *is* a tool, and we are signing the JWT? No, a tool doesn't sign a JWT in launch unless it's a Deep Linking Response.
   Wait, if our application is the *Platform*, we sign the JWT (`id_token`) as the LMS to send to the tool.
   If we sign a JWT to send to the tool, `LaunchURL` could be returning the form parameters for the front-end to auto-submit to the tool's LTI launch endpoint.
   Let's check `lti.go`'s `LTI13Adapter`.
