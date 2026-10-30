# Changelog

All notable changes to this project are documented here. The project uses
[Semantic Versioning](https://semver.org/) (pre-1.0: a minor bump may break the API).

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

[0.3.2]: https://github.com/TheForgotten69/go-insta/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/TheForgotten69/go-insta/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/TheForgotten69/go-insta/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/TheForgotten69/go-insta/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/TheForgotten69/go-insta/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/TheForgotten69/go-insta/releases/tag/v0.1.0
