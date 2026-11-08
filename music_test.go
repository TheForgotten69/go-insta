package goinsta

import (
	"encoding/json"
	"testing"
)

// testdata/bodies/music.json: live answers on the test account (2026-11-08), cut
// to two or three items, CDN query strings and DASH manifests removed.

// nextCursor reads the cursor a recorded body hands to the next page.
func nextCursor(t *testing.T, body string) string {
	t.Helper()
	var r struct {
		PageInfo struct {
			NextMaxID string `json:"next_max_id"`
		} `json:"page_info"`
		PagingInfo struct {
			MaxID string `json:"max_id"`
		} `json:"paging_info"`
	}
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	return r.PageInfo.NextMaxID + r.PagingInfo.MaxID
}

func TestTrendingAudio(t *testing.T) {
	bodies := testBodies(t, "music")
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
	if second.payload["max_id"] != nextCursor(t, bodies["music_top_trends"]) {
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
	bodies := testBodies(t, "music")
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
	bodies := testBodies(t, "music")
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

func TestPlaylistAndSearchMusic(t *testing.T) {
	bodies := testBodies(t, "music")
	insta, _ := newReplay(t)
	got := capture(t, insta, func(s sent) (int, string) {
		if s.req.URL.Path == "/api/v1/music/search_v2/" {
			return 200, bodies["music_search"]
		}
		return 200, bodies["music_playlist"]
	})
	pl := (&MusicPlaylist{insta: insta, surface: MusicReels, ID: "2282005535164995"}).AllAudio()
	for range 2 {
		if err := pl.Next(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	search := insta.SearchMusic(MusicReels, "november")
	if err := search.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	p1, p2, s := (*got)[0], (*got)[1], (*got)[2]
	if p1.req.URL.Path != "/api/v1/music/playlist/2282005535164995/" || p1.payload["product"] != string(MusicReels) ||
		p1.payload["cursor"] != nil || p2.payload["cursor"] != "61" || p1.payload["should_disable_exposures"] != nil {
		t.Errorf("playlist requests: %v / %v", p1.payload, p2.payload)
	}
	if a := pl.Items[0]; !a.IsOriginalSound || a.AudioAssetID != "796523205870974" {
		t.Errorf("playlist audio = %+v", a)
	}
	if s.payload["q"] != "november" || s.payload["from_search"] != "true" || s.payload["from_typeahead"] != "false" ||
		s.payload["from_typing_page"] != "false" || s.payload["search_session_id"] == "" || s.payload["browse_session_id"] == "" {
		t.Errorf("search request: %v", s.payload)
	}
	if len(search.Items) != 2 || search.Items[0].Title != "November" {
		t.Errorf("search = %+v", search.Items)
	}
}

func TestAudioReelsPage(t *testing.T) {
	bodies := testBodies(t, "music")
	insta, _ := newReplay(t)
	got := capture(t, insta, answer(bodies["clips_music"]))
	reels := (&Audio{insta: insta, AudioAssetID: "1115857888092269", AudioClusterID: "831681715950717"}).Reels()
	for range 2 {
		if err := reels.Next(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	first, second := (*got)[0], (*got)[1]
	if first.req.URL.Path != "/api/v1/clips/music/" || first.payload["audio_asset_id"] != "831681715950717" ||
		first.payload["audio_cluster_id"] != "831681715950717" || first.payload["max_id"] != nil {
		t.Errorf("page 1 request: %v", first.payload)
	}
	if second.payload["max_id"] != nextCursor(t, bodies["clips_music"]) {
		t.Errorf("page 2 max_id = %v", second.payload["max_id"])
	}
	if len(reels.Items) != 2 || reels.Items[0].Code != "DfWat9woBuY" || reels.Items[0].Index != 2 {
		t.Errorf("reels = %+v", reels.Items)
	}

	got = capture(t, insta, answer(bodies["clips_music"]))
	_ = (&Audio{insta: insta, AudioAssetID: "28952492164372492", IsOriginalSound: true}).Reels().Next(t.Context())
	if p := (*got)[0].payload; p["original_sound_audio_asset_id"] != "28952492164372492" || p["audio_asset_id"] != nil {
		t.Errorf("original sound request: %v", p)
	}
}
