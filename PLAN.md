# go-insta: remaining work

Status (2026-10-28): password login works live (`go run ./cmd/login`). `go run ./cmd/smoke -writes`
passes 51/51 live: every public method except Login/Logout (reads, follow/unfollow, settings toggle,
photo/album/story upload with like, save, pin, note, comments, archive, download and delete).
Parallel album uploads and batched `LoadReels` (10 reels per call) work live. Replay tests pass.

Done since the login fix: attestation re-signed before its 1 h expiry, `SetProxy` keeps the app TLS
fingerprint (http CONNECT and socks5), rotated password key stored from the login response,
recommended-post (`explore_story`) feed items parsed, logged-in request profile moved to the
current build.

Ownership: **[you]** = capture or key work done outside this repo; **[code]** = changes in this repo.
The `just` recipes below live in `~/Documents/projects/research/instagram/justfile`.

## Login

1. **2FA** — implemented, pending live validation. **[code DONE 2026-10-28]** `TOTP(secret,t)`,
   `ErrTwoFactorRequired`, `Login` detects 2FA and stashes the context, `LoginTwoFactor(code)` replays
   `verify_code.async` -> `has_been_allowed.async`, and `cmd/login` prompts for the code (or reads
   `IG_TOTP_SECRET`). Request side verified against `log_tigon_login_20261028-165404.txt`; see
   `docs/twofactor.md`. **[you] VALIDATE LIVE**: `IG_PASSWORD=... IG_TOTP_SECRET=... go run ./cmd/login`.
   The responses were not in the capture (request-only), so context extraction and bearer parsing are
   best-effort with the raw response surfaced on failure — a live run confirms or reveals the exact shape.


2. **Clear login errors** — tell apart wrong password, 429, 2FA and checkpoint from the Bloks
   response instead of "no authorization token". **[you]** capture one failed login (wrong
   password) with `just login`; no failure response exists yet. **[code]** classify from it.
3. **Checkpoints (`checkpoint.ufac.*`)** — captcha, contact-point code, selfie flows are in the
   captures but unhandled. **[code]**
4. `oauth.token.fetch.async` (sent by the app before every login) is skipped. Login works without
   it; add only if logins start failing. **[code]**

## Device identity and attestation

5. **Mixed installs** — `login.session.json` (attestation keys now embedded under `attestation`)
   comes from Waydroid install 1; the app has been reinstalled 3 times since. Re-extract keys and
   device ids from one current install, or self-register (next item). **[you]**
6. **Post-login key upload** — after login the app uploads a fresh zca key
   (`create_android_keystore` → `upload_android_keystore_key`); we keep signing with the old
   install-1 key. Flow is mapped (see memory/attestation notes), not built. **[code]**

## Request profile

7. **Fixture refresh** — the request profile now follows the 2026-10-28 captures (host
   `i.instagram.com`, no `x-zero-*`, network-stack headers; `TestCurrentBuildProfile`). The
   replay fixtures still come from the older build, so request bodies are only checked against
   it. 14 endpoints exist in the new captures and can be re-fixtured from them; the write
   endpoints (like, follow, comment, upload…) need a new logged-in capture that uses them. **[code]**/**[you]**
8. **HTTP/2 fingerprint** — header order (app: app-layer headers sorted, then network-stack
    headers sorted), pseudo-header order and SETTINGS still differ from the app. Not needed for
    login so far. **[you]** `just capture feed` fills `h2/*.jsonl` (h2-dump.py); **[code]** switch
    to an HTTP/2 stack that allows ordering (e.g. bogdanfinn/fhttp) if it becomes necessary.

## Coverage

9. **Untested writes** — like, save, story like/seen, album and story upload, pin, archive, note,
    settings changes. Only add pairs that undo themselves to `cmd/smoke -writes`. **[code]**
10. **`Logout` untested live.** **[code]**
11. ~~**Session persistence**~~ — DONE: `insta.OnSessionChange` fires when a rotated header is
    stored, so callers can persist without polling (verified live, 2026-10-28).

Added 2026-10-28 (verified live): `SearchUsers` and `Collection.Posts` are now pagers;
the all-media auto collection reads from `feed/saved/posts/`, a numbered one from
`feed/collection/<id>/`.
