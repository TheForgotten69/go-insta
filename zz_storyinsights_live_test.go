//go:build live

package goinsta

import (
	"os"
	"testing"
)

// Run: STORY=<pk>_<owner> go test -tags live -run TestLiveStoryInsights -v .
// Reads one own story's insights screen (the newest live story without STORY).
// Navigation shows only once the story has about 5 viewers.
func TestLiveStoryInsights(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()

	story := &Item{ID: os.Getenv("STORY"), insta: insta}
	if story.ID == "" {
		reel, err := insta.Account.Stories(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(reel.Items) == 0 {
			t.Skip("no live story; set STORY")
		}
		story = reel.Items[0]
	}
	in, err := story.StoryInsights(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("story %s fbid=%d", story.ID, story.FBID)
	for unit, rows := range in.Rows {
		t.Logf("  %s: %v", unit, rows)
	}
	if len(in.Rows[StoryUnitInteractions]) == 0 {
		t.Error("interactions unit has no rows")
	}
}
