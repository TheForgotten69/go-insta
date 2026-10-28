# Two-factor login (CAA / app 453)

Programmatic TOTP two-factor login, built from the capture
`research/instagram/log_tigon_login_20261028-165404.txt` (requests and responses).

## The flow the app runs

| # | Call | Role |
|---|------|------|
| 26 | `bloks.caa.login.async.send_login_request` | Password accepted, no bearer; response redirects to 2FA (`redirect_two_fac`) and carries the `two_step_verification_context` |
| 28 | `bloks/apps/two_step_verification.entrypoint` | Screen render |
| 34 | `bloks/apps/two_step_verification.method_picker` | Screen render |
| 40 | `two_step_verification.method_picker.navigation.async` | Selects `totp` |
| 41 | `bloks/apps/two_factor_login.enter_totp_code` | Screen render |
| 43 | `two_step_verification.verify_code.async` | Submits the code; **response carries the bearer** |
| 44 | `two_step_verification.has_been_allowed.async` | "Approve from another device" poll, fired in parallel; answers `is_loading false`, no token |

We replay 26 → 40 → 43. The screen renders are UI only, and 44 is the push-approval
poll, which a TOTP login does not need.

## Context

The context (about 2.3 KB, base64url) is not a JSON pair. The send_login response
holds the next screen's params as a Bloks key list followed by a value list:

```
(f4i (dkc "two_step_verification_context" "flow_source" "device_id" …)
     (dkc "AWR9…"                         "two_factor_login" "android-…" …))
```

`parseTwoFactorContext` finds the key list, takes the key's index and reads the
same slot from the value list. The same context is echoed unchanged by every
later call.

## Request specs

All async calls use the logged-out profile (`X-Ig-Www-Claim: 0`, no attest params)
and every 2FA call shares these params:

```
client_input_params: { cloud_trust_token: null, network_bssid: null, … }
server_params: { two_step_verification_context, flow_source: "two_factor_login", device_id,
  spectra_reg_login_data: null, INTERNAL__latency_qpl_marker_id: 36707139,
  INTERNAL__latency_qpl_instance_id: <int> }
```

- `method_picker.navigation.async`: endpoint `…two_step_verification.method_picker`;
  client `selected_method: "totp"`; server `should_fallback_to_sms: 0`.
- `verify_code.async`: endpoint `…two_factor_login.enter_totp_code`; client `code`,
  `machine_id` (the `X-Mid`), `device_id` (android id), `family_device_id`,
  `should_trust_device: 0`, `auth_secure_device_id: ""`, `block_store_machine_id: ""`;
  server `challenge: "totp"`.

The verify_code response embeds the session headers (`IG-Set-Authorization`, rur,
www-claim, ds-user-id, password key) in the same escaped Bloks shape as a normal
login, so `applyLoginResponse` parses it.

## Live status

Verified live 2026-10-28: skipping the screen renders and the approval poll is
accepted, and the verify_code token works for logged-in requests.

Not yet observed: a rejected code's response. `LoginTwoFactor` reports any
verify_code response without a bearer as a rejected code and includes the start
of the body.

## Code

- `Login` returns `ErrTwoFactorRequired` when the password is accepted but 2FA is
  on; it stashes the context and keeps the logged-out attestation applied.
- `LoginTwoFactor(ctx, code)` replays method_picker → verify_code, then restores the
  pre-login attestation and fires `OnSessionChange`. The pending login is consumed
  either way, so a rejected code needs a fresh `Login`.
- `TOTP(secret, t)` generates the code; `cmd/login` prompts for it, or reads
  `IG_TOTP_SECRET` to run non-interactively.
