//go:build live

package goinsta

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"
	"time"
)

// Run: go test -tags live -run TestLiveStoryLinkPost -v .
// Posts a photo, a story linked to it and a highlight of that story, reads the
// sticker back from the reel and the highlight, then deletes all three
// (LINK_KEEP=1 leaves them up to try in the app).
func TestLiveStoryLinkPost(t *testing.T) {
	keep := os.Getenv("LINK_KEEP") == "1"
	cleanup := func(name string, del func() error) {
		if keep {
			t.Logf("kept %s", name)
			return
		}
		t.Logf("delete %s: %v", name, del())
	}
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()
	pause := func() { time.Sleep(2 * time.Second) }

	post, err := insta.UploadPhoto(ctx, solidJPEG(t, 1080, 1080, color.RGBA{200, 60, 40, 255}), PostOptions{Caption: "go-insta link probe"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup("post", func() error { return post.Delete(ctx) })
	t.Logf("post %s pk=%d owner=%d", post.Code, post.Pk, post.User.ID)

	pause()
	story, err := insta.UploadStory(ctx, solidJPEG(t, 1080, 1920, color.RGBA{40, 60, 200, 255}), StoryOptions{LinkPost: post})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup("story", func() error { return story.Delete(ctx) })
	t.Logf("story %s from configure: story_feed_media=%s", story.ID, raw(story.StoryFeedMedia))

	pause()
	reel, err := insta.Account.Stories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range reel.Items {
		if it.ID == story.ID {
			found = true
			t.Logf("reel item: story_feed_media=%s", raw(it.StoryFeedMedia))
		}
	}
	if !found {
		t.Errorf("story %s not in own reel", story.ID)
	}

	pause()
	hl, err := insta.CreateHighlight(ctx, "go-insta link", story)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup("highlight", func() error { return hl.Delete(ctx) })
	t.Logf("highlight %s cover=%s items=%d", hl.ReelID(), hl.CoverMedia.MediaID, len(hl.Items))
	for _, it := range hl.Items {
		t.Logf("highlight item %s: story_feed_media=%s", it.ID, raw(it.StoryFeedMedia))
	}
}

func solidJPEG(t *testing.T, w, h int, c color.RGBA) *bytes.Buffer {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func raw(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
