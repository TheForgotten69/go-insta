//go:build live

package goinsta

import (
	"encoding/json"
	"testing"
)

// Read-only, except the comment opt-in, which is written back unchanged.
func TestLiveContentInsights(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()
	if err := insta.Account.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	posts, err := insta.Account.User().Posts().Collect(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	var post, reel *Item
	for _, p := range posts {
		if p.ProductType == "clips" && reel == nil {
			reel = p
		} else if p.ProductType != "clips" && post == nil {
			post = p
		}
	}
	if post != nil {
		in, err := post.Insights(ctx)
		t.Logf("post %s Insights: err=%v %+v", post.ID, err, in)
	}
	if reel != nil {
		in, err := reel.ReelInsights(ctx)
		t.Logf("reel %s ReelInsights: err=%v %+v", reel.ID, err, in)
	} else {
		t.Log("no own Reel: post one (cmd/smoke -reel -keep) to read Reel insights")
	}

	for _, p := range []MediaProduct{MediaProductFeed, MediaProductStory, MediaProductReels} {
		grid := insta.Account.MediaInsightsGrid(p)
		err := grid.Next(ctx)
		t.Logf("MediaInsightsGrid %s: err=%v items=%d more=%v", p, err, len(grid.Items), grid.More())
	}

	on, err := insta.Account.CommentInsightsOptIn(ctx)
	t.Logf("CommentInsightsOptIn: %v err=%v", on, err)
	if err == nil {
		t.Logf("SetCommentInsightsOptIn(%v) unchanged: %v", on, insta.Account.SetCommentInsightsOptIn(ctx, on))
	}

	stories, err := insta.Account.Stories(ctx)
	if err != nil || len(stories.Items) == 0 {
		t.Logf("no own story (err=%v): post one to read viewers", err)
		return
	}
	story := stories.Items[len(stories.Items)-1]
	viewers := story.StoryViewers()
	err = viewers.Next(ctx)
	users, _ := json.Marshal(viewers.Items)
	t.Logf("StoryViewers %s: err=%v n=%d more=%v users=%.1500s (run with httplog for user_count)", story.ID, err, len(viewers.Items), viewers.More(), users)
}
