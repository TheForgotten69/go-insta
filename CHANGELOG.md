# Changelog

All notable changes to this project are documented here. The project uses
[Semantic Versioning](https://semver.org/) (pre-1.0: a minor bump may break the API).

## [Unreleased]

### Breaking

- `CreateHighlight` takes a `HighlightOptions` argument; pass `HighlightOptions{}` for the old behavior.

### Features

- `HighlightOptions.CoverCrop` and `HighlightEdit.CoverCrop` choose the part of the cover the
  tray shows (`[left, top, right, bottom]` fractions of the frame) instead of its centred
  square, for a cover whose title is not in the middle. A crop that is not a rectangle inside
  the frame returns `ErrCoverCrop` before any request.
- `Item.StoryInsights` reads a story's insights screen: navigation (total, forward, back,
  next story, exited) as counts, and every row of the overview, views, interactions and
  profile activity sections as displayed. Verified live on a story with 5 viewers (the app
  showed no data for stories with 1-3 views). `Item.FBID` holds the media's Facebook id.
- Music (verified live): `TrendingAudio` pages through the app's trending audio list with
  each audio's rank, previous rank, Reels count and trending arrow (the Reels picker's
  Trending tab ranks the same audio); `StoryTrendingMusic` pages through the story music
  sticker's popular tracks; `MusicBrowser` loads the story or Reels music picker's
  playlists (For you, Featured, New releases, Original audio), and `MusicPlaylist.AllAudio`
  pages through one of them; `SearchMusic` searches the picker; `Audio.Reels` pages
  through the Reels that use an audio (its audio page).

## [0.7.2] - 2026-11-01

### Features

- `StoryOptions.LinkPost` also links Reels (verified live).

### Changes

- `StoryOptions.LinkPost` accepts an item that has only its `ID` (`<pk>_<owner>`), and
  returns an error when the owner is unknown instead of sending your own account as owner.

### Documentation

- `StoryOptions.LinkPost` draws nothing: the story shows the uploaded image as is,
  and only the centered tap area opens the post.

## [0.7.1] - 2026-10-31

### Breaking

- `UploadStory` takes a `StoryOptions` argument; pass `StoryOptions{}` for the old behavior.

### Features

- `StoryOptions.LinkPost` puts a tappable post sticker over the story, so the story
  (and a highlight made from it) opens the post. Verified live; Instagram stores the
  sticker at most half the frame wide and tall, centered, and keeps one post sticker per story.

## [0.7.0] - 2026-10-31

### Features

- Post management, from the app's request builders and verified live:
  `Item.Edit(ctx, MediaEdit{Caption, ClearLocation, RemoveChildren, ChildOrder})`,
  `Item.DisableComments` / `EnableComments`, `Item.SetCountsHidden`, `Item.Unarchive`,
  `Comment.Pin` / `Unpin`, `Item.FetchLikers`, `Instagram.MediaInfo`, and for Reels
  `Item.SetRemixAllowed` and `Item.SetDownloads`.
- Relationships: `User.Block` / `Unblock`, `Account.BlockedUsers`,
  `Account.UpdateCloseFriends`, `User.Mute` / `Unmute` (posts and/or stories),
  `User.RemoveFollower`, `Account.PendingFollowRequests`,
  `User.ApproveFollowRequest` / `IgnoreFollowRequest`.
- Tags and places: `User.TaggedPosts`, `Item.RemoveTag`, `Instagram.SearchLocations`,
  `Instagram.LocationInfo`, `Instagram.LocationFeed`.
- Translations and insights: `Account.TranslationSettings` / `SetTranslationSettings`
  (AI voice translation, lip sync, approval), `Item.VoiceTranslations`, `Item.Insights`.
- Direct messages: `Instagram.Inbox` / `PendingInbox` (pagers), `Instagram.Thread`,
  `Thread.Messages`, `Thread.SendText`, `Instagram.SendDirectText`, `Thread.MarkSeen`.
  Sending and mark-seen use the app's legacy REST endpoints (the app itself uses
  MQTT) and are not verified live.
- Analytics, for professional accounts:
  - Account dashboard: `Account.InsightsHome`, `InsightsOverview` (views, reach,
    interactions, by follower and content type), `InsightsAudience` (followers,
    active times, age, top cities and countries), `InsightsContent` (pager, sortable
    by metric and type), `InsightsMediaComponent`, `InsightsBioLinks`,
    `InsightsPreviousBioLinks`, `InsightsViewerState`, with `InsightsPeriod` presets
    and `CustomPeriod`.
  - Content: `Item.ReelInsights` (views, watch time, skip and share rate, follows),
    `Account.MediaInsightsGrid`, `Item.StoryViewers`, and the comment summary opt-in.
  - Creator: `Account.SubscriptionInsights`; `SeriesInsightsOverview` /
    `Engagement` / `Audience` and `Account.FanClubInsights` return raw JSON and are
    not verified live (the test account has no series or fan club).
  - `Account.AcceptInsightsTerms` and the story insights opt-in and reset calls,
    not run live: accepting the terms cannot be undone.

### Enhancements

- Every request parameter is a named struct instead of a map; only HTTP headers,
  `ReelOptions.Extra` and maps keyed by dynamic ids remain maps. Requests carry them
  as `reqOptions.Form` / `Signed` and are encoded in one place. The wire format is
  unchanged.
- GraphQL: per-query gzip and the per-endpoint null-validation header the app sends;
  one root lookup that also accepts aliased roots (`1$field(...)`); missing roots
  wrap `ErrEmptyResponse`; a null root with errors returns the `*GraphQLError`, also
  when the response carries data, and so does a response with only errors.

### Bug fixes

- `Item.Insights` reads the live (non-aliased) root key.
- `User.Info` and the account decode on professional accounts, which send a bool
  where personal accounts send an object. The empty fields that broke
  (`User.LinkedFbInfo`, `User.HasEligibleWhatsappLinkingCategory`, the activity
  `Counts`) are removed.
- `ReelOptions.Trial` is documented to fail with HTTP 500 on personal accounts, where
  the Reel can still be published as a normal Reel.

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

[0.7.2]: https://github.com/TheForgotten69/go-insta/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/TheForgotten69/go-insta/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/TheForgotten69/go-insta/compare/v0.6.0...v0.7.0
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
