// Command smoke runs the API methods against a logged-in session and reports
// which ones work. By default it only reads. -writes adds reversible actions on
// your own account: follow/unfollow, toggling story auto-archive, and posting a
// photo, an album and a story that are interacted with and then deleted.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"os/signal"
	"time"

	goinsta "github.com/TheForgotten69/go-insta"
)

type runner struct {
	ctx     context.Context
	pause   time.Duration
	ok, bad int
}

func (r *runner) step(name string, fn func() (string, error)) {
	if r.ctx.Err() != nil {
		return
	}
	summary, err := fn()
	if err != nil {
		r.bad++
		fmt.Printf("FAIL  %-26s %v\n", name, err)
	} else {
		r.ok++
		fmt.Printf("ok    %-26s %s\n", name, summary)
	}
	time.Sleep(r.pause)
}

// page loads one page; ErrNoMore on an empty feed is not a failure.
func page(ctx context.Context, next func(context.Context) error) error {
	if err := next(ctx); err != nil && !errors.Is(err, goinsta.ErrNoMore) {
		return err
	}
	return nil
}

func main() {
	session := flag.String("session", "login.session.json", "logged-in session file")
	target := flag.String("user", "instagram", "public account to look up")
	pause := flag.Duration("pause", 2*time.Second, "delay between calls")
	writes := flag.Bool("writes", false, "also run reversible writes on your own account")
	albumSize := flag.Int("album", 2, "photos in the -writes album")
	keep := flag.Bool("keep", false, "with -crosspost, -highlight or -reel, leave what it made up")
	reel := flag.String("reel", "", "MP4 to post as a Reel, then delete")
	crosspost := flag.String("crosspost", "", "photo or album: post it shared to Threads and the story with hidden counts, then delete it (the Threads copy stays)")
	highlight := flag.Bool("highlight", false, "post two stories; create, add to, edit, archive, unarchive and delete a highlight; delete the stories")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	insta, err := goinsta.ImportSession(*session)
	if err != nil {
		fail("session", err)
	}
	var rotations int
	insta.OnSessionChange = func(goinsta.Session) { rotations++ }
	defer func() { fmt.Printf("OnSessionChange fired %d times\n", rotations) }()
	dir, err := os.MkdirTemp("", "smoke-")
	if err != nil {
		fail("tempdir", err)
	}
	defer os.RemoveAll(dir)

	r := &runner{ctx: ctx, pause: *pause}
	readAccount(r, insta, dir)
	firstPost := readFeeds(r, insta)
	user := readUser(r, insta, *target, dir)
	if firstPost != nil {
		r.step("item.Comments (2 pages)", func() (string, error) {
			c := firstPost.Comments()
			got, err := c.Collect(ctx, 40)
			return fmt.Sprintf("%d comments, total %d", len(got), c.Count), err
		})
	}
	if *writes {
		if user != nil {
			r.step("user.Follow", func() (string, error) { return "@" + user.Username, user.Follow(ctx) })
			r.step("user.Unfollow", func() (string, error) { return "@" + user.Username, user.Unfollow(ctx) })
		}
		writeAccount(r, insta)
		writePost(r, insta)
		writeAlbum(r, insta, dir, *albumSize)
		writeStory(r, insta)
	}
	if *crosspost != "" {
		writeCrosspost(r, insta, *crosspost, *keep)
	}
	if *highlight {
		writeHighlight(r, insta, *keep)
	}
	if *reel != "" {
		writeReel(r, insta, *reel, *keep)
	}

	if err := insta.ExportSession(*session); err != nil {
		fmt.Fprintln(os.Stderr, "save session:", err)
	}
	fmt.Printf("\n%d ok, %d failed\n", r.ok, r.bad)
	if r.bad > 0 {
		os.Exit(1)
	}
}

func readAccount(r *runner, insta *goinsta.Instagram, dir string) {
	ctx, acc := r.ctx, insta.Account
	r.step("account.Sync", func() (string, error) {
		if err := acc.Sync(ctx); err != nil {
			return "", err
		}
		return fmt.Sprintf("@%s, %d followers, %d posts", acc.Username, acc.FollowerCount, acc.MediaCount), nil
	})
	r.step("account.ReelSettings", func() (string, error) {
		s, err := acc.ReelSettings(ctx)
		if err != nil {
			return "", err
		}
		return "auto_archive=" + s.ReelAutoArchive, nil
	})
	r.step("account.Stories", func() (string, error) {
		reel, err := acc.Stories(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d own story items", len(reel.Items)), nil
	})
	r.step("account.SuggestedUsers", func() (string, error) {
		u, err := acc.SuggestedUsers(ctx)
		return fmt.Sprintf("%d users", len(u)), err
	})
	r.step("account.RestrictedUsers", func() (string, error) {
		u, err := acc.RestrictedUsers(ctx)
		return fmt.Sprintf("%d users", len(u)), err
	})
	r.step("AccountFamily", func() (string, error) {
		b, err := insta.AccountFamily(ctx)
		return fmt.Sprintf("%d bytes", len(b)), err
	})
	r.step("user.DownloadProfilePicTo", func() (string, error) {
		u := acc.User()
		if err := u.Info(ctx); err != nil {
			return "", err
		}
		return u.DownloadProfilePicTo(ctx, dir+"/")
	})
	r.step("Activity + seen", func() (string, error) {
		a := insta.Activity()
		if err := page(ctx, a.Next); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d inbox items, badge %d", len(a.Items), a.Badges.Total), nil
	})
	r.step("Collections + Posts", func() (string, error) {
		c, err := insta.Collections().Collect(ctx, 0)
		if err != nil {
			return "", err
		}
		posts := 0
		if len(c) > 0 {
			p, err := c[0].Posts().Collect(ctx, 0)
			if err != nil {
				return "", err
			}
			posts = len(p)
		}
		return fmt.Sprintf("%d collections, %d posts in first", len(c), posts), nil
	})
}

func readFeeds(r *runner, insta *goinsta.Instagram) *goinsta.Item {
	ctx := r.ctx
	var firstPost *goinsta.Item
	r.step("Timeline (2 pages)", func() (string, error) {
		tl := insta.Timeline()
		if err := page(ctx, tl.Next); err != nil {
			return "", err
		}
		n := len(tl.Items)
		if n > 0 {
			firstPost = tl.Items[0]
		}
		if err := page(ctx, tl.Next); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d + %d items, last index %d", n, len(tl.Items), lastIndex(tl.Items)), nil
	})
	r.step("Timeline.Refresh", func() (string, error) {
		tl := insta.Timeline()
		if err := tl.Refresh(ctx); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d items", len(tl.Items)), nil
	})
	r.step("StoryTray + LoadReels", func() (string, error) {
		tray, err := insta.StoryTray(ctx, goinsta.TrayColdStart)
		if err != nil {
			return "", err
		}
		loaded := 0
		for _, reel := range tray.Reels {
			if reel.Loaded() {
				loaded++
			}
		}
		if err := insta.LoadReels(ctx, tray.Reels...); err != nil {
			return "", err
		}
		items := 0
		for _, reel := range tray.Reels {
			items += len(reel.Items)
		}
		return fmt.Sprintf("%d reels (%d preloaded), %d items after LoadReels", len(tray.Reels), loaded, items), nil
	})
	r.step("Reels (2 pages) + seen", func() (string, error) {
		reels := insta.Reels()
		if err := page(ctx, reels.Next); err != nil {
			return "", err
		}
		if len(reels.Items) == 0 {
			return "no reels", nil
		}
		if err := insta.MarkReelsSeen(ctx, reels.Items[0]); err != nil {
			return "", err
		}
		n := len(reels.Items)
		if err := page(ctx, reels.Next); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d + %d reels", n, len(reels.Items)), nil
	})
	return firstPost
}

func readUser(r *runner, insta *goinsta.Instagram, target, dir string) *goinsta.User {
	ctx := r.ctx
	var user *goinsta.User
	r.step("SearchUsers (2 pages)", func() (string, error) {
		users, err := insta.SearchUsers(target).Collect(ctx, 40)
		if err != nil {
			return "", err
		}
		for _, u := range users {
			if u.Username == target {
				user = u
			}
		}
		if user == nil {
			return "", fmt.Errorf("@%s not among %d results", target, len(users))
		}
		return fmt.Sprintf("%d results", len(users)), nil
	})
	if user == nil {
		return nil
	}
	r.step("user.Sync", func() (string, error) {
		if err := user.Sync(ctx); err != nil {
			return "", err
		}
		return fmt.Sprintf("@%s id=%d, %d followers", user.Username, user.ID, user.FollowerCount), nil
	})
	r.step("UserByID", func() (string, error) {
		u, err := insta.UserByID(ctx, int64(user.ID))
		if err != nil {
			return "", err
		}
		return "@" + u.Username, nil
	})
	r.step("user.LoadFriendship", func() (string, error) {
		f, err := user.LoadFriendship(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("following=%v", f.Following), nil
	})
	r.step("Friendships", func() (string, error) {
		m, err := insta.Friendships(ctx, int64(user.ID))
		return fmt.Sprintf("%d entries", len(m)), err
	})
	var post *goinsta.Item
	r.step("user.Posts (2 pages)", func() (string, error) {
		posts, err := user.Posts().Collect(ctx, 13)
		if err != nil {
			return "", err
		}
		if len(posts) > 0 {
			post = posts[0]
		}
		return fmt.Sprintf("%d posts, last index %d", len(posts), lastIndex(posts)), nil
	})
	if post != nil {
		r.step("MediaIDFromShortID", func() (string, error) {
			id, err := goinsta.MediaIDFromShortID(post.Code)
			if err != nil {
				return "", err
			}
			if id != post.Pk.String() {
				return "", fmt.Errorf("%s decoded to %s, pk %s", post.Code, id, post.Pk)
			}
			return post.Code + " = " + id, nil
		})
		r.step("item.DownloadTo", func() (string, error) {
			paths, err := post.DownloadTo(ctx, dir+"/")
			return fmt.Sprintf("%d files", len(paths)), err
		})
	}
	r.step("user.Followers (2 pages)", func() (string, error) {
		u, err := user.Followers().Collect(ctx, 30)
		return fmt.Sprintf("%d users", len(u)), err
	})
	r.step("user.Following", func() (string, error) {
		l := user.Following()
		if err := page(ctx, l.Next); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d users, more=%v", len(l.Items), l.More()), nil
	})
	r.step("user.Stories", func() (string, error) {
		reel, err := user.Stories(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d items", len(reel.Items)), nil
	})
	r.step("Highlights + LoadReels", func() (string, error) {
		hl, err := user.Highlights(ctx)
		if err != nil || len(hl) == 0 {
			return "0 highlights", err
		}
		if err := insta.LoadReels(ctx, hl...); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d highlights, first has %d items", len(hl), len(hl[0].Items)), nil
	})
	r.step("user.SuggestedUsers", func() (string, error) {
		s, err := user.SuggestedUsers(ctx)
		return fmt.Sprintf("%d users", len(s)), err
	})
	return user
}

func writeAccount(r *runner, insta *goinsta.Instagram) {
	ctx, acc := r.ctx, insta.Account
	r.step("SetStoryAutoArchive x2", func() (string, error) {
		s, err := acc.ReelSettings(ctx)
		if err != nil {
			return "", err
		}
		on := s.ReelAutoArchive == "on"
		if err := acc.SetStoryAutoArchive(ctx, !on); err != nil {
			return "", err
		}
		return fmt.Sprintf("toggled from %v and back", on), acc.SetStoryAutoArchive(ctx, on)
	})
}

func writePost(r *runner, insta *goinsta.Instagram) {
	ctx := r.ctx
	var post *goinsta.Item
	r.step("UploadPhoto", func() (string, error) {
		var err error
		post, err = insta.UploadPhoto(ctx, testJPEG(0), goinsta.PostOptions{Caption: "go-insta smoke test"})
		if err != nil {
			return "", err
		}
		return post.ID, nil
	})
	if post == nil {
		return
	}
	defer r.step("item.Delete (photo)", func() (string, error) { return post.ID, post.Delete(ctx) })

	r.step("item.Like", func() (string, error) { return "", post.Like(ctx) })
	r.step("item.Save", func() (string, error) { return "", post.Save(ctx) })
	r.step("item.Download", func() (string, error) {
		b, err := post.Download(ctx)
		return fmt.Sprintf("%d bytes", len(b)), err
	})
	r.step("item.Pin", func() (string, error) { return "", post.Pin(ctx) })
	r.step("item.AddNote", func() (string, error) {
		n, err := post.AddNote(ctx, "smoke")
		if err != nil {
			return "", err
		}
		return "note " + n.ID, nil
	})
	var comment *goinsta.Comment
	r.step("item.Comment", func() (string, error) {
		var err error
		comment, err = post.Comment(ctx, "smoke test")
		if err != nil {
			return "", err
		}
		return comment.ID.String(), nil
	})
	if comment != nil {
		r.step("comment.Like", func() (string, error) { return "", comment.Like(ctx) })
		r.step("comment.Unlike", func() (string, error) { return "", comment.Unlike(ctx) })
		r.step("comment.Replies", func() (string, error) {
			rs, err := comment.Replies(ctx)
			return fmt.Sprintf("%d replies", len(rs)), err
		})
		r.step("item.Comments (own)", func() (string, error) {
			c, err := post.Comments().Collect(ctx, 0)
			return fmt.Sprintf("%d comments", len(c)), err
		})
		r.step("comment.Delete", func() (string, error) { return "", comment.Delete(ctx) })
	}
	r.step("item.Archive", func() (string, error) { return "", post.Archive(ctx) })
}

func writeAlbum(r *runner, insta *goinsta.Instagram, dir string, size int) {
	ctx := r.ctx
	var album *goinsta.Item
	r.step("UploadAlbum", func() (string, error) {
		var err error
		photos := make([]io.Reader, size)
		for i := range photos {
			photos[i] = testJPEG(uint8(i * 160 / size))
		}
		album, err = insta.UploadAlbum(ctx, photos, goinsta.PostOptions{Caption: "go-insta album smoke"})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s, %d children", album.ID, len(album.CarouselMedia)), nil
	})
	if album == nil {
		return
	}
	r.step("item.DownloadTo (album)", func() (string, error) {
		paths, err := album.DownloadTo(ctx, dir+"/")
		return fmt.Sprintf("%d files", len(paths)), err
	})
	r.step("item.Delete (album)", func() (string, error) { return album.ID, album.Delete(ctx) })
}

func writeStory(r *runner, insta *goinsta.Instagram) {
	ctx := r.ctx
	var story *goinsta.Item
	r.step("UploadStory", func() (string, error) {
		var err error
		story, err = insta.UploadStory(ctx, testJPEG(160))
		if err != nil {
			return "", err
		}
		return story.ID, nil
	})
	if story == nil {
		return
	}
	r.step("reel.MarkSeen (own)", func() (string, error) {
		reel, err := insta.Account.Stories(ctx)
		if err != nil {
			return "", err
		}
		if len(reel.Items) == 0 {
			return "", errors.New("uploaded story not in own reel")
		}
		return fmt.Sprintf("%d items", len(reel.Items)), reel.MarkSeen(ctx, reel.Items...)
	})
	r.step("item.LikeStory (own)", func() (string, error) { return "", story.LikeStory(ctx) })
	r.step("item.Delete (story)", func() (string, error) { return story.ID, story.Delete(ctx) })
}

func writeCrosspost(r *runner, insta *goinsta.Instagram, kind string, keep bool) {
	ctx := r.ctx
	opts := goinsta.PostOptions{
		Caption:        "go-insta crosspost smoke (" + kind + ")",
		ShareToThreads: true, HideLikeCounts: true, HideShareCounts: true,
		ShareToStory: true,
	}
	upload := func() (*goinsta.Item, error) { return insta.UploadPhoto(ctx, testJPEG(80), opts) }
	switch kind {
	case "photo":
	case "album":
		upload = func() (*goinsta.Item, error) {
			return insta.UploadAlbum(ctx, []io.Reader{testJPEG(0), testJPEG(80), testJPEG(160)}, opts)
		}
	default:
		fail("crosspost", fmt.Errorf("unknown kind %q, want photo or album", kind))
	}
	var post *goinsta.Item
	r.step("crosspost "+kind+" (Threads+story)", func() (string, error) {
		var err error
		post, err = upload()
		if post == nil {
			return "", err
		}
		summary := fmt.Sprintf("%s code=%s children=%d fbid_v2=%d likes_hidden=%v shares_hidden=%v",
			post.ID, post.Code, len(post.CarouselMedia), insta.Session().FbIDV2, post.LikeViewCountDisabled, post.ShareCountDisabled)
		if err == nil && (!post.LikeViewCountDisabled || !post.ShareCountDisabled) {
			err = errors.New("counts not hidden: " + summary)
		}
		return summary, err
	})
	if post == nil {
		return
	}
	if !keep {
		defer r.step("item.Delete (crosspost)", func() (string, error) { return post.ID, post.Delete(ctx) })
	}

	r.step("own story reshare", func() (string, error) {
		reel, err := insta.Account.Stories(ctx)
		if err != nil {
			return "", err
		}
		if len(reel.Items) == 0 {
			return "", errors.New("no story in own reel")
		}
		story := reel.Items[len(reel.Items)-1]
		summary := fmt.Sprintf("%s, %d feed-post stickers", story.ID, len(story.StoryFeedMedia))
		if len(story.StoryFeedMedia) == 0 {
			err = errors.New("newest story has no post sticker: " + summary)
		}
		if keep {
			return summary + " (kept)", err
		}
		return summary, errors.Join(err, story.Delete(ctx))
	})
}

func writeReel(r *runner, insta *goinsta.Instagram, file string, keep bool) {
	ctx := r.ctx
	var it *goinsta.Item
	r.step("UploadReel", func() (string, error) {
		f, err := os.Open(file)
		if err != nil {
			return "", err
		}
		defer f.Close()
		it, err = insta.UploadReel(ctx, f, goinsta.ReelOptions{PostOptions: goinsta.PostOptions{Caption: "go-insta reel smoke"}})
		if err != nil {
			return "", err
		}
		return expect(it.Code != "" && it.ProductType == "clips" && it.VideoDuration > 0,
			"%s code=%s product=%s duration=%.2fs", it.ID, it.Code, it.ProductType, it.VideoDuration)
	})
	if it != nil && !keep {
		r.step("item.Delete (reel)", func() (string, error) { return it.ID, it.Delete(ctx) })
	}
}

func writeHighlight(r *runner, insta *goinsta.Instagram, keep bool) {
	ctx := r.ctx
	var stories []*goinsta.Item
	for _, hue := range []uint8{40, 200} {
		r.step("UploadStory (highlight)", func() (string, error) {
			s, err := insta.UploadStory(ctx, testJPEG(hue))
			if err != nil {
				return "", err
			}
			stories = append(stories, s)
			return s.ID, nil
		})
	}
	var hl *goinsta.Reel
	if len(stories) == 2 {
		r.step("CreateHighlight", func() (string, error) {
			var err error
			hl, err = insta.CreateHighlight(ctx, "go-insta smoke", stories[0])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s title=%q items=%d", hl.ReelID(), hl.Title, len(hl.Items)), nil
		})
	}
	if hl != nil {
		r.step("reel.AddItems", func() (string, error) {
			if err := hl.AddItems(ctx, stories[1]); err != nil {
				return "", err
			}
			return expect(len(hl.Items) == 2, "%s items=%d", hl.ReelID(), len(hl.Items))
		})
		r.step("reel.Edit (rename, cover, remove)", func() (string, error) {
			err := hl.Edit(ctx, goinsta.HighlightEdit{Title: "go-insta edit", Cover: stories[1], Remove: stories[:1]})
			if err != nil {
				return "", err
			}
			return expect(hl.Title == "go-insta edit" && len(hl.Items) == 1 && hl.CoverMedia.MediaID == stories[1].ID,
				"title=%q items=%d cover=%s", hl.Title, len(hl.Items), hl.CoverMedia.MediaID)
		})
		r.step("own Highlights lists it", func() (string, error) { return listed(ctx, insta, hl, true) })
		r.step("reel.Archive", func() (string, error) {
			if err := hl.Archive(ctx); err != nil {
				return "", err
			}
			return listed(ctx, insta, hl, false)
		})
		r.step("reel.Unarchive", func() (string, error) {
			if err := hl.Unarchive(ctx); err != nil {
				return "", err
			}
			return listed(ctx, insta, hl, true)
		})
	}
	if keep {
		return
	}
	if hl != nil {
		r.step("reel.Delete (highlight)", func() (string, error) {
			if err := hl.Delete(ctx); err != nil {
				return "", err
			}
			return listed(ctx, insta, hl, false)
		})
	}
	for _, s := range stories {
		r.step("item.Delete (highlight story)", func() (string, error) { return s.ID, s.Delete(ctx) })
	}
}

func expect(ok bool, format string, args ...any) (string, error) {
	summary := fmt.Sprintf(format, args...)
	if !ok {
		return summary, errors.New("unexpected: " + summary)
	}
	return summary, nil
}

func listed(ctx context.Context, insta *goinsta.Instagram, hl *goinsta.Reel, want bool) (string, error) {
	h, err := findHighlight(ctx, insta, hl.ReelID())
	if err != nil {
		return "", err
	}
	return expect((h != nil) == want, "%s listed=%v", hl.ReelID(), h != nil)
}

func findHighlight(ctx context.Context, insta *goinsta.Instagram, id string) (*goinsta.Reel, error) {
	all, err := insta.Account.User().Highlights(ctx)
	if err != nil {
		return nil, err
	}
	for _, h := range all {
		if h.ReelID() == id {
			return h, nil
		}
	}
	return nil, nil
}

func lastIndex(items []*goinsta.Item) int {
	if len(items) == 0 {
		return -1
	}
	return items[len(items)-1].Index
}

// testJPEG draws a 1080x1080 gradient, a size the feed and stories accept
// without cropping; hue shifts the colours so album children differ.
func testJPEG(hue uint8) io.Reader {
	img := image.NewRGBA(image.Rect(0, 0, 1080, 1080))
	for y := range 1080 {
		for x := range 1080 {
			img.Set(x, y, color.RGBA{uint8(x/5) + hue, uint8(y / 5), 160 - hue, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		fail("jpeg", err)
	}
	return &buf
}

func fail(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
