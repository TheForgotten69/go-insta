# go-insta

Private Instagram Android API client (app 453.0.0.55.79), rebuilt from device
captures on top of [Davincible/goinsta](https://github.com/Davincible/goinsta).
Requests replay the app's TLS fingerprint, headers and attestation.

```sh
go get github.com/TheForgotten69/go-insta
```

## Session

A session file holds the device ids, token, password key and the attestation
keys (`attestation.zca_key_pem`, `usdid_key_pem`, `last_uploaded_key_time_ms`,
`usdid_uuid`). It is written mode 0600; treat it as a credential.

```sh
go run ./cmd/httplog session log_http.txt > login.session.json   # from a capture
IG_PASSWORD=... go run ./cmd/login -user NAME                     # password login (prompts for 2FA)
IG_PASSWORD=... IG_TOTP_SECRET=... go run ./cmd/login -user NAME  # non-interactive 2FA
IG_PASSWORD=... go run ./cmd/login -user NAME -log login.log       # also log every response (holds the token)
go run ./cmd/smoke [-writes] [-highlight] [-reel file.mp4]       # live check
```

## Use

Every call takes a `context.Context`. Paged endpoints return a `*Pager[T]`:
`Next(ctx)` loads one page into `Items` (`ErrNoMore` at the end), `All(ctx)`
ranges over every item, `Collect(ctx, n)` gathers up to n.

```go
insta, err := goinsta.ImportSession("login.session.json")
defer insta.ExportSession("login.session.json") // keeps rotated headers

for post, err := range user.Posts().All(ctx) { ... }
followers, err := user.Followers().Collect(ctx, 200)

tray, err := insta.StoryTray(ctx, goinsta.TrayColdStart)
err = insta.LoadReels(ctx, tray.Reels...) // fills unloaded reels, batched

item, err := insta.UploadAlbum(ctx, photos, goinsta.PostOptions{Caption: "hi"})
// Cross-post to Threads, hide counts, and share to the story as a post card.
item, err = insta.UploadPhoto(ctx, img, goinsta.PostOptions{
	ShareToThreads: true, HideLikeCounts: true, HideShareCounts: true, ShareToStory: true,
})
paths, err := item.DownloadTo(ctx, "out/")
err = item.Edit(ctx, goinsta.MediaEdit{Caption: "fixed typo"}) // also DisableComments, SetCountsHidden, Unarchive…

reel, err := insta.UploadReel(ctx, mp4, goinsta.ReelOptions{ // H.264/AAC MP4
	PostOptions: goinsta.PostOptions{Caption: "hi", ShareToStory: true},
	Captions:    true,
	VoiceTranslation: &goinsta.VoiceTranslation{InputLanguage: "fra", Languages: []string{"eng", "spa"}},
}) // every field: docs/reel-fields.md

story, err := insta.UploadStory(ctx, img)
hl, err := insta.CreateHighlight(ctx, "Trips", story) // first item is the cover
err = hl.AddItems(ctx, otherStory)
err = hl.Edit(ctx, goinsta.HighlightEdit{Title: "Trips 2026", Cover: otherStory, Remove: []*goinsta.Item{story}})
err = hl.Archive(ctx) // hidden from the profile; Unarchive shows it again
err = hl.Delete(ctx)
```

Pagers: `Timeline()`, `Reels()`, `Collections()`, `Activity()`, `SearchUsers()`,
`User.Posts/Followers/Following()`, `Item.Comments()`, `Collection.Posts()`.

One `*Instagram` can be shared across goroutines; each pager belongs to one.
`Login` must not overlap other requests. Rate-limit waits and, at debug level,
every response body go to `insta.Logger` (an `*slog.Logger`, default `slog.Default()`).
Set `insta.OnSessionChange` to persist the session when the server rotates a header.

`PLAN.md` tracks the remaining work.
