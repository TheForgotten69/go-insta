package goinsta

import (
	"encoding/json"
	"os"
	"testing"
)

// Live answers on the test account (2026-11-08), cut to two or three items,
// CDN query strings, DASH manifests and "status" removed (fixture() reads every
// testdata file as httplog entries, whose status is a number).
func musicBodies(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("testdata/music.json")
	if err != nil {
		t.Fatal(err)
	}
	var bodies map[string]json.RawMessage
	if err := json.Unmarshal(raw, &bodies); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for k, v := range bodies {
		out[k] = string(v)
	}
	return out
}

func TestTrendingAudio(t *testing.T) {
	bodies := musicBodies(t)
	insta, _ := newReplay(t)
	got := capture(t, insta, func(s sent) (int, string) {
		if s.payload["max_id"] != nil {
			return 200, `{"items":[],"page_info":{"next_max_id":null,"more_available":false},"status":"ok"}`
		}
		return 200, bodies["music_top_trends"]
	})
	list, err := insta.TrendingAudio(TrendingAudioAll).Collect(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	first, second := (*got)[0], (*got)[1]
	if first.req.URL.Path != "/api/v1/music/top_trends/" || first.signed || first.payload["product"] != "ig_trending_audio_list" ||
		first.payload["top_audio_tab_type"] != "trending" || first.payload["page_size"] != "10" ||
		first.payload["_uuid"] != insta.uuid || first.payload["max_id"] != nil {
		t.Errorf("page 1 request: %s %v", first.req.URL.Path, first.payload)
	}
	if second.payload["max_id"] == nil || second.payload["max_id"] == "" {
		t.Errorf("page 2 max_id = %v", second.payload["max_id"])
	}
	if len(list) != 3 {
		t.Fatalf("%d audios", len(list))
	}
	if a := list[0]; a.Title != "November" || a.Artist != "Onoychenko" || a.AudioClusterID != "831681715950717" ||
		a.AudioAssetID != "1115857888092269" || !a.Trending || a.TrendRank != 1 || a.ReelsCount != "148K reels" || a.IsOriginalSound {
		t.Errorf("track = %+v", a)
	}
	if a := list[2]; !a.IsOriginalSound || a.AudioAssetID != "28952492164372492" || a.Artist != "khaly.off" ||
		a.TrendRank != 3 || a.PreviousTrendRank != 2 || a.ReelsCount != "4.4K reels" || !a.Trending {
		t.Errorf("original sound = %+v", a)
	}
}

func TestStoryTrendingMusic(t *testing.T) {
	bodies := musicBodies(t)
	insta, _ := newReplay(t)
	got := capture(t, insta, answer(bodies["music_trending"]))
	p := insta.StoryTrendingMusic()
	if err := p.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := p.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	first, second := (*got)[0], (*got)[1]
	if first.req.URL.Path != "/api/v1/music/trending/" || first.payload["product"] != "story_camera_music_overlay_post_capture" ||
		first.payload["should_disable_exposures"] != "true" || first.payload["browse_session_id"] == "" || first.payload["cursor"] != nil {
		t.Errorf("page 1 request: %v", first.payload)
	}
	if second.payload["cursor"] != "30" || second.payload["browse_session_id"] != first.payload["browse_session_id"] {
		t.Errorf("page 2 request: %v", second.payload)
	}
	if a := p.Items[0]; a.Title != "Wfet" || a.Artist != "A.L.A" || a.Trending || a.TrendRank != 0 {
		t.Errorf("track = %+v", a)
	}
}

func TestMusicBrowser(t *testing.T) {
	bodies := musicBodies(t)
	insta, _ := newReplay(t)
	got := capture(t, insta, answer(bodies["music_stories_browser"]))
	lists, err := insta.MusicBrowser(t.Context(), MusicStory)
	if err != nil {
		t.Fatal(err)
	}
	r := (*got)[0]
	if r.req.URL.Path != "/api/v1/music/stories_audio_browser/" || r.payload["product"] != string(MusicStory) || r.signed {
		t.Errorf("request: %s %v", r.req.URL.Path, r.payload)
	}
	if len(lists) != 3 || lists[1].ID != "2282005535164995" || lists[1].Title != "For you" || lists[1].Type != "for_you" ||
		len(lists[1].Audios) != 2 || len(lists[2].Audios) != 2 || !lists[2].Audios[0].IsOriginalSound {
		t.Errorf("playlists = %+v", lists)
	}
	if _, err := insta.MusicBrowser(t.Context(), "feed"); err == nil {
		t.Error("unknown surface: no error")
	}
}
