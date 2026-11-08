//go:build live

package goinsta

import "testing"

// Run: go test -tags live -run TestLiveMusic -v .
// Reads two pages of trending audio, the story sticker's popular tracks, the
// story and Reels music pickers, a playlist's second page, a search and the
// Reels of a trending track and of an original sound.
func TestLiveMusic(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()

	trending := insta.TrendingAudio(TrendingAudioAll)
	if err := trending.Next(ctx); err != nil || len(trending.Items) == 0 {
		t.Fatalf("trending: %d, %v", len(trending.Items), err)
	}
	top := trending.Items[0]
	var original *Audio
	for _, a := range trending.Items {
		if a.IsOriginalSound && original == nil {
			original = a
		}
	}
	if err := trending.Next(ctx); err != nil {
		t.Fatal(err)
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
		pl := insta.PlaylistAudio(s, lists[len(lists)-1].ID)
		for range 2 {
			if err := pl.Next(ctx); err != nil {
				t.Fatalf("%s playlist %s: %v", s, lists[len(lists)-1].Title, err)
			}
		}
		t.Logf("%s: %s page 2 has %d audios", s, lists[len(lists)-1].Title, len(pl.Items))
	}

	search := insta.SearchMusic(MusicReels, "november")
	if err := search.Next(ctx); err != nil || len(search.Items) == 0 {
		t.Fatalf("search: %d, %v", len(search.Items), err)
	}
	t.Logf("search november: %d, first %s by %s", len(search.Items), search.Items[0].Title, search.Items[0].Artist)

	for _, a := range []*Audio{top, original} {
		if a == nil {
			continue
		}
		reels := insta.AudioReels(a)
		for range 2 {
			if err := reels.Next(ctx); err != nil {
				t.Fatalf("reels of %s: %v", a.Title, err)
			}
		}
		if len(reels.Items) == 0 || reels.Items[0].ProductType != "clips" {
			t.Errorf("reels of %s: %d items", a.Title, len(reels.Items))
		}
		t.Logf("reels of %s (original=%v): page 2 has %d, first %s", a.Title, a.IsOriginalSound, len(reels.Items), reels.Items[0].Code)
	}
}
