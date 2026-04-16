package oauthhandler

import "html/template"

// UserCodePlaceholder is the display shape shown in the device verification
// input and referenced in tests. A change here must be mirrored in the 9-char
// `GenerateUserCode` output width (4 chars + "-" + 4 chars).
const UserCodePlaceholder = "XXXX-XXXX"

// consentTmpl renders the OAuth authorization consent screen.
var consentTmpl = template.Must(template.New("consent").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorize Application</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;color:#0f172a;background:#ffffff}
h1{font-size:1.5em}
.client-name{font-weight:bold;color:#2563eb}
.scope{background:#f1f5f9;padding:4px 8px;border-radius:4px;font-family:monospace}
form{margin-top:24px}
button{padding:10px 24px;font-size:1em;border:none;border-radius:6px;cursor:pointer;margin-right:8px}
.approve{background:#2563eb;color:white}
.deny{background:#e2e8f0;color:#334155}
button:focus-visible{outline:2px solid #4f46e5;outline-offset:2px}
@media(prefers-color-scheme:dark){
body{color:#e2e8f0;background:#030712}
.client-name{color:#60a5fa}
.scope{background:#111827;color:#e2e8f0}
.deny{background:#334155;color:#e2e8f0}
button:focus-visible{outline-color:#818cf8}
}
</style>
</head>
<body>
<h1>Authorize Application</h1>
<p><span class="client-name">{{.ClientName}}</span> is requesting access to your account.</p>
<p>Scope: <span class="scope">{{.Scope}}</span></p>
<form method="POST" action="/oauth/authorize/approve">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="hidden" name="scope" value="{{.Scope}}">
<input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
<input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
<input type="hidden" name="nonce" value="{{.Nonce}}">
<button type="submit" class="approve">Authorize</button>
<button type="submit" class="deny" formaction="/oauth/authorize/deny">Deny</button>
</form>
</body>
</html>`))

// jsRedirectTmpl renders a page that performs a client-side redirect via
// JavaScript. The URL is injected as a pre-encoded JSON string to prevent XSS.
var jsRedirectTmpl = template.Must(template.New("jsRedirect").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Redirecting…</title></head>
<body>
<p>Redirecting…</p>
<script>window.location.href={{.}};</script>
<noscript><p>JavaScript is required to complete this authorization flow.</p></noscript>
</body>
</html>`))

// deviceVerificationTmpl renders the device code entry form.
var deviceVerificationTmpl = template.Must(template.New("deviceVerification").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Device Authorization</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;color:#0f172a;background:#ffffff}
h1{font-size:1.5em}
label{display:block;margin-bottom:8px}
input[type=text]{font-size:1.5em;padding:10px;width:200px;text-align:center;letter-spacing:4px;text-transform:uppercase;border:2px solid #94a3b8;border-radius:6px;background:#ffffff;color:#0f172a}
input[type=text]:focus-visible{outline:2px solid #4f46e5;outline-offset:2px;border-color:#4f46e5}
button{padding:10px 24px;font-size:1em;border:none;border-radius:6px;cursor:pointer;background:#2563eb;color:white}
button:focus-visible{outline:2px solid #4f46e5;outline-offset:2px}
@media(prefers-color-scheme:dark){
body{color:#e2e8f0;background:#030712}
input[type=text]{background:#111827;color:#e2e8f0;border-color:#6b7280}
input[type=text]:focus-visible{outline-color:#818cf8;border-color:#818cf8}
button:focus-visible{outline-color:#818cf8}
}
</style>
</head>
<body>
<h1>Device Authorization</h1>
<form method="POST" action="/oauth/device">
<label for="user_code">Enter the code shown on your device:</label>
<input id="user_code" type="text" name="user_code" value="{{.}}" maxlength="9" placeholder="` + UserCodePlaceholder + `" autocomplete="off" required>
<br><br>
<button type="submit">Continue</button>
</form>
</body>
</html>`))

// deviceConsentTmpl renders the device authorization consent screen.
var deviceConsentTmpl = template.Must(template.New("deviceConsent").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorize Device</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;color:#0f172a;background:#ffffff}
h1{font-size:1.5em}
.client-name{font-weight:bold;color:#2563eb}
.scope{background:#f1f5f9;padding:4px 8px;border-radius:4px;font-family:monospace}
button{padding:10px 24px;font-size:1em;border:none;border-radius:6px;cursor:pointer;margin-right:8px}
.approve{background:#2563eb;color:white}
.deny{background:#e2e8f0;color:#334155}
button:focus-visible{outline:2px solid #4f46e5;outline-offset:2px}
@media(prefers-color-scheme:dark){
body{color:#e2e8f0;background:#030712}
.client-name{color:#60a5fa}
.scope{background:#111827;color:#e2e8f0}
.deny{background:#334155;color:#e2e8f0}
button:focus-visible{outline-color:#818cf8}
}
</style>
</head>
<body>
<h1>Authorize Device</h1>
<p><span class="client-name">{{.ClientName}}</span> is requesting access to your account.</p>
<p>Scope: <span class="scope">{{.Scope}}</span></p>
<form method="POST" action="/oauth/device/approve">
<input type="hidden" name="user_code" value="{{.UserCode}}">
<button type="submit" name="approve" value="approve" class="approve">Authorize</button>
<button type="submit" name="approve" value="deny" class="deny">Deny</button>
</form>
</body>
</html>`))

// deviceErrorTmpl renders a simple error page for device flow errors.
var deviceErrorTmpl = template.Must(template.New("deviceError").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Error</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center;color:#0f172a;background:#ffffff}
h1{color:#dc2626}
@media(prefers-color-scheme:dark){body{color:#e2e8f0;background:#030712}h1{color:#f87171}}
</style>
</head>
<body>
<h1>Error</h1>
<p role="alert">{{.}}</p>
</body>
</html>`))

// Static HTML pages (no dynamic content — no template needed).

const denyHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorization Denied</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;color:#0f172a;background:#ffffff}
h1{font-size:1.5em}
@media(prefers-color-scheme:dark){body{color:#e2e8f0;background:#030712}}
</style>
</head>
<body>
<h1>Authorization Denied</h1>
<p>Authorization denied. You may close this window.</p>
</body>
</html>`

const deviceSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorization Successful</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center;color:#0f172a;background:#ffffff}
h1{color:#16a34a}
@media(prefers-color-scheme:dark){body{color:#e2e8f0;background:#030712}h1{color:#4ade80}}
</style>
</head>
<body>
<h1>Authorization Successful!</h1>
<p>You can close this window and return to your device.</p>
</body>
</html>`

const deviceDeniedHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorization Denied</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center;color:#0f172a;background:#ffffff}
h1{color:#dc2626}
@media(prefers-color-scheme:dark){body{color:#e2e8f0;background:#030712}h1{color:#f87171}}
</style>
</head>
<body>
<h1>Authorization Denied</h1>
<p>You can close this window.</p>
</body>
</html>`
