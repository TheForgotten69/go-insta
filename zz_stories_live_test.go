//go:build live

package goinsta

import (
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

	post, err := insta.UploadPhoto(ctx, testJPEG(t, 1080, 1080), PostOptions{Caption: "go-insta link probe"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup("post", func() error { return post.Delete(ctx) })
	t.Logf("post %s pk=%d owner=%d", post.Code, post.Pk, post.User.ID)

	pause()
	story, err := insta.UploadStory(ctx, testJPEG(t, 1080, 1920), StoryOptions{LinkPost: post})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup("story", func() error { return story.Delete(ctx) })
	t.Logf("story %s from configure: story_feed_media=%v", story.ID, story.StoryFeedMedia)

	pause()
	reel, err := insta.Account.Stories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range reel.Items {
		if it.ID == story.ID {
			found = true
			t.Logf("reel item: story_feed_media=%v", it.StoryFeedMedia)
		}
	}
	if !found {
		t.Errorf("story %s not in own reel", story.ID)
	}

	pause()
	hl, err := insta.CreateHighlight(ctx, "go-insta link", HighlightOptions{}, story)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup("highlight", func() error { return hl.Delete(ctx) })
	t.Logf("highlight %s cover=%s items=%d", hl.ReelID(), hl.CoverMedia.MediaID, len(hl.Items))
	for _, it := range hl.Items {
		t.Logf("highlight item %s: story_feed_media=%v", it.ID, it.StoryFeedMedia)
	}
}

// Run: go test -tags live -run TestLiveHighlightCoverCrop -v .
// Creates a highlight framed on the top square of its story, reframes it on the
// bottom square, reads each crop back, then deletes the highlight and the story.
func TestLiveHighlightCoverCrop(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()

	story, err := insta.UploadStory(ctx, testJPEG(t, 1080, 1920), StoryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { t.Logf("delete story: %v", story.Delete(ctx)) }()
	time.Sleep(2 * time.Second)

	top := []float64{0, 0, 1, 0.5625}
	hl, err := insta.CreateHighlight(ctx, "go-insta crop", HighlightOptions{CoverCrop: top}, story)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { t.Logf("delete highlight: %v", hl.Delete(ctx)) }()
	if formatCrop(hl.CoverMedia.CropRect) != formatCrop(top) {
		t.Errorf("created cover crop = %v, want %v", hl.CoverMedia.CropRect, top)
	}

	time.Sleep(2 * time.Second)
	bottom := []float64{0, 0.4375, 1, 1}
	if err := hl.Edit(ctx, HighlightEdit{Cover: story, CoverCrop: bottom}); err != nil {
		t.Fatal(err)
	}
	if formatCrop(hl.CoverMedia.CropRect) != formatCrop(bottom) {
		t.Errorf("edited cover crop = %v, want %v", hl.CoverMedia.CropRect, bottom)
	}
}
