# Changelog

All notable changes to this project are documented here. The project uses
[Semantic Versioning](https://semver.org/) (pre-1.0: a minor bump may break the API).

## [Unreleased]

### Features

- Post management, from the app's request builders and verified live:
  `Item.Edit(ctx, MediaEdit{Caption, ClearLocation, RemoveChildren, ChildOrder})`,
  `Item.DisableComments` / `EnableComments`, `Item.SetCountsHidden`, `Item.Unarchive`,
  `Comment.Pin` / `Unpin`, `Item.FetchLikers`, `Instagram.MediaInfo`, and for Reels
  `Item.SetRemixAllowed` and `Item.SetDownloads`.

### Enhancements

- Every request parameter is a named struct instead of a map; only HTTP headers,
  `ReelOptions.Extra` and maps keyed by dynamic ids remain maps. The wire format is
  unchanged.

## [0.6.0] - 2026-10-31

### Features

- **Breaking:** `UploadReel` takes `ReelOptions`, which embeds `PostOptions` and adds
  every Reel setting decoded from the app: `HideFromFeed`, `DisableComments`,
  `AllowDownloads`, `DisableTemplate`, `Captions`, `TranslateCaption`, `Gifting`,
  `Audience` (close friends, followers only, …), `CoverFrameMs`, `Trial`
  (trial Reels with a graduation strategy), `VoiceTranslation` (AI voice translation),
  `ScheduledAt`, and `Extra` for any other field.
- `PostOptions.ShareToStory` on a Reel asks Instagram to reshare it to the story
  (`ErrReelStoryShare` is gone).
- `docs/reel-fields.md` lists all 200 fields the app can send when it publishes a
  Reel, how each is encoded, and which `ReelOptions` field sets it.

### Enhancements

- `UploadReel` waits up to 5 minutes for Instagram's transcode (was 2).

## [0.5.0] - 2026-10-31

### Features

- `Instagram.UploadReel(ctx, mp4, PostOptions)` posts an MP4 (H.264 video, AAC audio)
  as a Reel (#6). The size and duration come from the MP4 itself; Instagram picks
  the cover frame. It waits while Instagram reports "Transcode not finished yet."
  `PostOptions.ShareToStory` returns `ErrReelStoryShare`.
- `ErrInvalidVideo` for input that is not an MP4 with a video track.
- `cmd/smoke -reel file.mp4` posts and deletes a Reel.

## [0.4.0] - 2026-10-31

### Features

- `Reel.Edit(ctx, HighlightEdit{Title, Cover, Add, Remove})`: the profile's "Edit
  highlight" save (rename, change cover, add and remove stories in one call).
- `Reel.Delete`, `Reel.Archive` and `Reel.Unarchive` for highlights.
- `Reel.CoverMedia` carries a highlight's cover media id and crop.
- `cmd/smoke -highlight` also edits, archives, unarchives and deletes the highlight.

### Enhancements

- `User.Highlights` pages through `paginated_highlights_tray` like the app, and keeps
  going past empty pages that still carry a cursor.

## [0.3.4] - 2026-10-30

### Enhancements

- Requests follow the 2026-10-28/30 captures: uploads and `configure` use priority
  `u=1, i` and send `x-ig-eu-configure-disabled`; GraphQL drops the
  `enable_canonical_*` fields; the timeline sends `is_non_feed_surface` and no longer
  tags requests `app_start_tag: lukewarm`; the activity inbox sends `should_skip_su=true`.
- 20 of the 57 replay fixtures now come from the 2026-10-28/30 captures; the rest
  wait for a capture that exercises them (see `PLAN.md`).

### Bug fixes

- `cmd/httplog` scrubs `press_signals`, which carries the client's IP addresses.

## [0.3.3] - 2026-10-30

### Enhancements

- Request headers match what the app has sent since 2026-10-28 (same app version,
  453.0.0.55.79): `x-bloks-prism-extended-palette-gray` and `-polish-enabled` are
  `true`; logged-in REST requests add `x-ig-device-languages` and
  `x-ig-device-timezone`, GraphQL adds only the timezone, uploads and logged-out
  requests add neither.

## [0.3.2] - 2026-10-30

### Bug fixes

- `User.Posts` on a private account you don't follow returns `ErrPrivateAccount`
  instead of `graphql patch: path ... is not an object` (#3). Any GraphQL response
  part that fails without data now returns a `*GraphQLError` with Instagram's code
  and message.
- Security: `github.com/klauspost/compress` 1.20.3 and `golang.org/x/image` 0.47.0
  (Dependabot alerts on 1.18.0 and 0.24.0).

## [0.3.1] - 2026-10-30

### Bug fixes

- `User.Highlights` follows the tray cursor, so accounts with more than 10
  highlights get all of them instead of the first 10.

## [0.3.0] - 2026-10-30

### Features

- `Instagram.CreateHighlight(ctx, title, items...)` creates a story highlight; the
  first item is the cover, cropped to its centred square.
- `Reel.AddItems(ctx, items...)` adds stories to an existing highlight and refreshes
  the reel from the server's response.
- `ErrNoHighlightItems` and `ErrNotHighlight`.
- `cmd/smoke -highlight`: posts two stories, makes a highlight of one, adds the
  other, checks it is listed, then deletes the stories (Instagram then drops the
  highlight).

### Enhancements

- `cmd/smoke -crosspost` takes `photo` or `album`; `-keep` leaves what `-crosspost`
  or `-highlight` made.
- **Breaking:** `PostOptions.StoryImage io.Reader` is replaced by
  `PostOptions.ShareToStory bool`. The story card is now rendered from the post's
  first image instead of being supplied by the caller.

## [0.2.0] - 2026-10-30

### Features

- `PostOptions.ShareToThreads` cross-posts a photo or album to Threads.
- `PostOptions.HideLikeCounts` and `PostOptions.HideShareCounts`.
- `PostOptions.StoryImage` reshares the new post to the story as a tappable post card.
- `cmd/smoke -crosspost`.

### Bug fixes

- Upload docs: Instagram keeps the uploaded pixels, and an album's shape follows
  its first image.

## [0.1.1] - 2026-10-28

### Enhancements

- Albums take up to 20 photos.

### Bug fixes

- Comment and reel streams keep every NDJSON line.
- Decoding of usertags, top likers, activity and user id fields.

## [0.1.0] - 2026-10-28

First release: an Instagram for Android 453 private API client.

### Features

- CAA password login with TOTP two-factor (`Login`, `LoginTwoFactor`, `cmd/login`),
  and session import/export with an `OnSessionChange` hook.
- Attestation header re-signing and uTLS proxy support.
- Context-aware pagers for timeline, reels, posts, followers, following, comments,
  collections, activity and user search; a client safe to share across goroutines.
- Stories, highlights, likes, comments, saves, follows, photo, album and story uploads.
- `cmd/httplog` for device captures and `cmd/smoke` for live checks.

[0.6.0]: https://github.com/TheForgotten69/go-insta/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/TheForgotten69/go-insta/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/TheForgotten69/go-insta/compare/v0.3.4...v0.4.0
[0.3.4]: https://github.com/TheForgotten69/go-insta/compare/v0.3.3...v0.3.4
[0.3.3]: https://github.com/TheForgotten69/go-insta/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/TheForgotten69/go-insta/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/TheForgotten69/go-insta/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/TheForgotten69/go-insta/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/TheForgotten69/go-insta/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/TheForgotten69/go-insta/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/TheForgotten69/go-insta/releases/tag/v0.1.0
