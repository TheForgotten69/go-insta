# go-insta: remaining work

Status (2026-10-31, v0.4.0): password login with TOTP 2FA works live (`go run ./cmd/login`).
`go run ./cmd/smoke -writes` passes 51/51 live: every public method except Login/Logout (reads,
follow/unfollow, settings toggle, photo/album/story upload with like, save, pin, note, comments,
archive, download and delete). `-crosspost photo|album` (Threads cross-post, hidden counts, story
reshare) and `-highlight` (create, add, edit, archive, unarchive, delete) also pass live. Replay tests pass.

Done since 2026-10-28, all verified live:
- Threads cross-post, hidden like/share counts, post reshare to the story (`PostOptions`).
- `CreateHighlight` and `Reel.AddItems`; `User.Highlights` follows the tray cursor past 10.
- `User.Posts` on a private, unfollowed profile returns `ErrPrivateAccount` (wraps `*GraphQLError`).
- Request profile follows the 2026-10-28/30 captures: palette flags, device language/timezone
  headers, upload priority `u=1, i` and `x-ig-eu-configure-disabled`, no `enable_canonical_*` on
  GraphQL, `is_non_feed_surface` on the timeline, no `lukewarm` tag on later timeline pages.

The Instagram app is still 453.0.0.55.79 in every capture; `threads_app_version=453.0.0.56.79`
is the Threads app.

Ownership: **[you]** = capture or key work done outside this repo; **[code]** = changes in this repo.
The `just` recipes below live in `~/Documents/projects/research/instagram/justfile`.

## Features

1. **Reel (video) upload** (issue #6): the emulator cannot post one. Waydroid's soft H.264
   encoder asserts on Surface input unless `ro.hardware.egl=mesa` (NVIDIA venus mode sets
   `angle`); with that check patched out it still encodes nothing, so the app uploads an empty
   file. `log_feed_20261031-002406.txt` has the steps before that: `clips_info_for_creation`,
   `upload_settings/<uuid>`, and `rupload_igvideo/<name>` (offset GET, then POST with
   `X-Instagram-Rupload-Params` share_type `reels`, `is_clips_video`, `media_type` 2,
   duration/size, `extract_cover_frame`). Missing: a successful video POST and the publish call
   (`configure_to_clips`). Next: build those from the documented protocol and verify live
   with an MP4, or capture on hardware that encodes. **[code]**
2. **Highlight cover from the camera roll** (a dedicated cover image): not captured; the
   captured edits only pick a story frame. **[you]** Instagram cuts titles to 16 characters
   without an error.

Done 2026-10-31 (live): highlight create, add, edit (rename, cover, add/remove), archive,
unarchive, delete; tray paging through `paginated_highlights_tray`.

## Login

3. **Checkpoints (`checkpoint.ufac.*`)**: captcha, contact-point code, selfie flows are in the
   captures but unhandled; no checkpoint response from our own logins captured yet. **[code]**
4. `oauth.token.fetch.async` (sent by the app before every login) is skipped. Login works without
   it; add only if logins start failing. **[code]**

Done: 2FA (`docs/twofactor.md`), clear login errors (`ErrBadPassword`, `ErrTooManyRequests`,
`ErrTwoFactorRequired`).

## Device identity and attestation

5. **Mixed installs**: `login.session.json` (attestation keys embedded under `attestation`) comes
   from Waydroid install 1, and its user agent still says Waydroid while the app has sent a Pixel 5
   user agent since 2026-10-28. Re-extract keys, device ids and the user agent from one current
   install, or self-register (next item). **[you]**
6. **Post-login key upload**: after login the app uploads a fresh zca key
   (`create_android_keystore` → `upload_android_keystore_key`); we keep signing with the old
   install-1 key. Flow is mapped (see memory/attestation notes), not built. **[code]**

## Request profile

7. **Fixture refresh**: 20 of 57 fixtures now come from the 2026-10-28/30 captures. Still from
   2026-10-27, because no later capture exercises them: follow/unfollow, story like, restricted
   users, reel settings, like, save, comments (all), collections, note, album configure, clips
   seen, media delete/only-me/pin, account search, followers/following, show_many. Kept old on
   purpose: `reels_tray`, `reels_media_stream`, `clips_discover`, `user_info_stream` and
   `info_stream_search`/`_following` (different entry points), plain `configure`/`configure_story`
   (the later captures only have other variants), `chaining` (later response has no users),
   `profile_timeline` (item 8). A capture that does those actions once lets the rest move
   over. **[you]**
8. **Profile timeline variables**: the later `IGProfileTimelineQuery` captures send different
   variables from ours (e.g. `include_media_notes_fields`, `skip_ig_play_count`,
   `lightweight_carousel_type`); compare per surface (own vs other profile) and update. **[code]**
9. **`x-ig-session-visitation`**: a token seen on many requests in the 2026-10-30 16:16 capture
   only; no response sets it. Find its origin if requests start failing without it. **[you]**
10. **HTTP/2 fingerprint**: header order (app: app-layer headers sorted, then network-stack
    headers sorted), pseudo-header order and SETTINGS still differ from the app. Not needed so
    far. **[you]** `just capture feed` fills `h2/*.jsonl` (h2-dump.py); **[code]** switch to an
    HTTP/2 stack that allows ordering (e.g. bogdanfinn/fhttp) if it becomes necessary.

Not planned: a live `Logout` test.
