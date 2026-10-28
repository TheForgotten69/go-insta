# Two-factor login (CAA / app 453)

Programmatic TOTP two-factor login, built from the capture
`research/instagram/log_tigon_login_20261028-165404.txt`.

## The flow the app runs

Logged-out CAA login, then a two-step chain when 2FA is on:

1. `send_login_request` — password is accepted, the response carries **no bearer**
   and signals 2FA, minting a `two_step_verification_context`.
2. three Bloks **screen renders** — `two_step_verification.entrypoint` →
   `method_picker` → `two_factor_login.enter_totp_code`. These are UI; they only
   thread the context forward.
3. `two_step_verification.verify_code.async` — submits the code + context.
4. `two_step_verification.has_been_allowed.async` — returns the bearer.

We skip step 2 (pure UI) and replay 1 → 3 → 4.

## Request specs (verified against the capture)

Both async calls use the logged-out profile (`X-Ig-Www-Claim: 0`, no attest
params) and client endpoint
`IgCdsScreenNavigationLoggerModule:com.bloks.www.two_factor_login.enter_totp_code`.

`verify_code.async` body (`params` = JSON):

```
client_input_params: { code, machine_id, device_id (android id), family_device_id,
  should_trust_device: 0, auth_secure_device_id: "", block_store_machine_id: "",
  cloud_trust_token: null, network_bssid: null }
server_params: { challenge: "totp", two_step_verification_context, flow_source: "two_factor_login",
  device_id, spectra_reg_login_data: null, INTERNAL__latency_qpl_marker_id: 36707139,
  INTERNAL__latency_qpl_instance_id: <int> }
```

`has_been_allowed.async` is identical minus `code` and `challenge`.

`machine_id` is the `X-Mid` the server set during login (falls back to the
session mid). `device_id` is the android id; `family_device_id` the family uuid.

## What is NOT verified, and why

The capture is request-only: the tigon recorder logs outgoing requests, and
`just login` passes the API hosts through mitmproxy untouched (otherwise login
429s). So we have **no response body** for any login call. That leaves two spots
written to best understanding, each of which surfaces the raw response on failure
so a single live run pins them down:

- **Context extraction** — `parseTwoFactorContext` scans the `send_login`
  response for `two_step_verification_context`, the same regex-over-unescaped-
  Bloks technique `applyLoginResponse` already uses for the session headers. If
  the field is encoded differently, `LoginTwoFactor` returns the raw response.
- **Bearer delivery** — we assume `has_been_allowed` returns the session headers
  in the same Bloks shape as a normal login, and parse them with
  `applyLoginResponse`.

## Code

- `Login` returns `ErrTwoFactorRequired` when the password is accepted but 2FA is
  on; it stashes the context and keeps the logged-out attestation applied.
- `LoginTwoFactor(ctx, code)` replays verify_code → has_been_allowed, then
  restores the pre-login attestation.
- `TOTP(secret, t)` generates the code; `cmd/login` prompts for it, or reads
  `IG_TOTP_SECRET` to run non-interactively.

## Open questions for review

- Are the three skipped screen renders truly stateless, or does the server
  require `method_picker.navigation.async` to select TOTP before `verify_code`?
- Does `verify_code` alone return the bearer (making `has_been_allowed`
  optional), or is the second call required? The capture shows the bearer only
  on `has_been_allowed`.
- Is `machine_id` the `X-Mid`, or a distinct 2FA machine id from the (uncaptured)
  `send_login` response?
