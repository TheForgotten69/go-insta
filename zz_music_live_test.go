//go:build live

package goinsta

import "testing"

// Run: go test -tags live -run TestLiveMusic -v .
// Reads two pages of trending audio, the story sticker's popular tracks and the
// story and Reels music pickers.
func TestLiveMusic(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()

	trending := insta.TrendingAudio(TrendingAudioAll)
	for range 2 {
		if err := trending.Next(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range trending.Items[:5] {
		t.Logf("#%d (was %d) %s by %s, %s, trending=%v original=%v", a.TrendRank, a.PreviousTrendRank, a.Title, a.Artist, a.ReelsCount, a.Trending, a.IsOriginalSound)
	}
	if len(trending.Items) == 0 || trending.Items[0].TrendRank != 11 {
		t.Errorf("page 2 does not start at rank 11: %d items", len(trending.Items))
	}

	story := insta.StoryTrendingMusic()
	if err := story.Next(ctx); err != nil || len(story.Items) == 0 {
		t.Fatalf("story trending: %d, %v", len(story.Items), err)
	}
	t.Logf("story sticker: %d tracks, first %s by %s", len(story.Items), story.Items[0].Title, story.Items[0].Artist)

	for _, s := range []MusicSurface{MusicStory, MusicReels} {
		lists, err := insta.MusicBrowser(ctx, s)
		if err != nil || len(lists) == 0 {
			t.Fatalf("%s browser: %d, %v", s, len(lists), err)
		}
		for _, l := range lists {
			t.Logf("%s: %s (%s) %d audios more=%v", s, l.Title, l.Type, len(l.Audios), l.HasMore)
		}
	}
}
