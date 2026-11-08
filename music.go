package goinsta

import (
	"context"
	"fmt"
)

// MusicSurface is the composer a music list is for.
type MusicSurface string

const (
	MusicReels MusicSurface = "story_camera_clips_v2"
	MusicStory MusicSurface = "story_camera_music_overlay_post_capture"
)

// TrendingAudioTab selects a tab of the app's trending audio list.
type TrendingAudioTab string

const (
	TrendingAudioAll       TrendingAudioTab = "trending"
	TrendingAudioFollowers TrendingAudioTab = "popular_with_your_followers"
)

// Audio is a licensed track or an original sound. Trending is the arrow the
// app draws on audio that is trending in Reels; TrendRank is 0 when unranked.
type Audio struct {
	insta *Instagram

	AudioAssetID      string
	AudioClusterID    string // licensed tracks only
	IsOriginalSound   bool
	Title             string
	Artist            string // the display artist, or the original sound's owner
	DurationMs        int
	CoverURL          string
	DownloadURL       string
	Trending          bool
	TrendRank         int
	PreviousTrendRank int
	ReelsCount        string // as displayed, "148K reels"
}

// MusicPlaylist is one section of the music picker's first page ("For you",
// "Featured", …) with its preview audio.
type MusicPlaylist struct {
	insta   *Instagram
	surface MusicSurface

	ID      string
	Title   string
	Type    string // for_you, artist_spotlight, default_playlist_type
	HasMore bool
	Audios  []*Audio
}

type trendRanks struct {
	TrendRank                int    `json:"trend_rank"`
	PreviousTrendRank        int    `json:"previous_trend_rank"`
	FormattedClipsMediaCount string `json:"formatted_clips_media_count"`
}

type musicItem struct {
	Track *struct {
		AudioAssetID           string `json:"audio_asset_id"`
		AudioClusterID         string `json:"audio_cluster_id"`
		Title                  string `json:"title"`
		DisplayArtist          string `json:"display_artist"`
		DurationInMs           int    `json:"duration_in_ms"`
		CoverArtworkURI        string `json:"cover_artwork_uri"`
		ProgressiveDownloadURL string `json:"progressive_download_url"`
	} `json:"track"`
	Metadata struct {
		trendRanks
		IsTrendingInClips bool `json:"is_trending_in_clips"`
	} `json:"metadata"`
	OriginalSound *struct {
		trendRanks
		AudioAssetID           Int64  `json:"audio_asset_id"`
		OriginalAudioTitle     string `json:"original_audio_title"`
		DurationInMs           int    `json:"duration_in_ms"`
		ProgressiveDownloadURL string `json:"progressive_download_url"`
		IgArtist               struct {
			Username      string `json:"username"`
			ProfilePicURL string `json:"profile_pic_url"`
		} `json:"ig_artist"`
		ConsumptionInfo struct {
			IsTrendingInClips bool `json:"is_trending_in_clips"`
		} `json:"consumption_info"`
	} `json:"original_sound"`
}

// audio returns nil for items that are neither (imported audio).
func (m musicItem) audio(insta *Instagram) *Audio {
	var a *Audio
	var ranks trendRanks
	var trending bool
	switch t, o := m.Track, m.OriginalSound; {
	case t != nil:
		a = &Audio{
			AudioAssetID:   t.AudioAssetID,
			AudioClusterID: t.AudioClusterID,
			Title:          t.Title,
			Artist:         t.DisplayArtist,
			DurationMs:     t.DurationInMs,
			CoverURL:       t.CoverArtworkURI,
			DownloadURL:    t.ProgressiveDownloadURL,
		}
		ranks, trending = m.Metadata.trendRanks, m.Metadata.IsTrendingInClips
	case o != nil:
		a = &Audio{
			AudioAssetID:    o.AudioAssetID.String(),
			IsOriginalSound: true,
			Title:           o.OriginalAudioTitle,
			Artist:          o.IgArtist.Username,
			DurationMs:      o.DurationInMs,
			CoverURL:        o.IgArtist.ProfilePicURL,
			DownloadURL:     o.ProgressiveDownloadURL,
		}
		ranks, trending = o.trendRanks, o.ConsumptionInfo.IsTrendingInClips
	default:
		return nil
	}
	a.insta, a.Trending = insta, trending
	a.TrendRank, a.PreviousTrendRank, a.ReelsCount = ranks.TrendRank, ranks.PreviousTrendRank, ranks.FormattedClipsMediaCount
	return a
}

func audios(insta *Instagram, items []musicItem) []*Audio {
	out := make([]*Audio, 0, len(items))
	for _, it := range items {
		if a := it.audio(insta); a != nil {
			out = append(out, a)
		}
	}
	return out
}

// musicPager pages through a music list; form builds each page's request from
// the cursor the previous page returned.
func (insta *Instagram) musicPager(endpoint string, form func(cursor string) any) *Pager[*Audio] {
	var cursor string
	return newPager(func(ctx context.Context) ([]*Audio, string, error) {
		var page struct {
			Items    []musicItem `json:"items"`
			PageInfo struct {
				NextMaxID     string `json:"next_max_id"`
				MoreAvailable bool   `json:"more_available"`
			} `json:"page_info"`
		}
		if err := insta.callJSON(ctx, &reqOptions{Endpoint: endpoint, IsPost: true, Form: form(cursor)}, &page); err != nil {
			return nil, "", err
		}
		cursor = cursorIf(page.PageInfo.MoreAvailable, page.PageInfo.NextMaxID)
		return audios(insta, page.Items), cursor, nil
	})
}

type topTrendsForm struct {
	UUID            string           `json:"_uuid"`
	Product         string           `json:"product"`
	TopAudioTabType TrendingAudioTab `json:"top_audio_tab_type"`
	PageSize        string           `json:"page_size"`
	MaxID           string           `json:"max_id,omitempty"`
}

// TrendingAudio pages through the app's trending audio list (the Reels picker's
// Trending tab ranks the same audio). The followers tab is empty for an account
// whose followers have not used any.
func (insta *Instagram) TrendingAudio(tab TrendingAudioTab) *Pager[*Audio] {
	return insta.musicPager(urlMusicTopTrends, func(maxID string) any {
		return topTrendsForm{UUID: insta.uuid, Product: "ig_trending_audio_list", TopAudioTabType: tab, PageSize: "10", MaxID: maxID}
	})
}

type musicBrowseForm struct {
	UUID                   string       `json:"_uuid"`
	Product                MusicSurface `json:"product"`
	BrowseSessionID        string       `json:"browse_session_id"`
	ShouldDisableExposures string       `json:"should_disable_exposures,omitempty"`
	Cursor                 string       `json:"cursor,omitempty"`
}

func (insta *Instagram) musicForm(surface MusicSurface, sessionID, cursor string) musicBrowseForm {
	f := musicBrowseForm{UUID: insta.uuid, Product: surface, BrowseSessionID: sessionID, Cursor: cursor}
	if surface == MusicStory {
		f.ShouldDisableExposures = "true"
	}
	return f
}

func (insta *Instagram) browsePager(endpoint string, surface MusicSurface) *Pager[*Audio] {
	sessionID := generateUUID()
	return insta.musicPager(endpoint, func(cursor string) any {
		return insta.musicForm(surface, sessionID, cursor)
	})
}

// StoryTrendingMusic pages through the popular tracks the story music sticker
// lists, 30 a page. Instagram answers this list for stories, not for Reels.
func (insta *Instagram) StoryTrendingMusic() *Pager[*Audio] {
	return insta.browsePager(urlMusicTrending, MusicStory)
}

// AllAudio pages through the whole playlist ("See more" in the app).
func (p *MusicPlaylist) AllAudio() *Pager[*Audio] {
	return p.insta.browsePager(fmt.Sprintf(urlMusicPlaylist, p.ID), p.surface)
}

type musicSearchForm struct {
	musicBrowseForm
	Q               string `json:"q"`
	SearchSessionID string `json:"search_session_id"`
	FromTypeahead   string `json:"from_typeahead"`
	FromSearch      string `json:"from_search"`
	FromTypingPage  string `json:"from_typing_page"`
}

// SearchMusic pages through the music picker's search results for q.
func (insta *Instagram) SearchMusic(surface MusicSurface, q string) *Pager[*Audio] {
	sessionID, searchID := generateUUID(), generateUUID()
	return insta.musicPager(urlMusicSearch, func(cursor string) any {
		return musicSearchForm{
			musicBrowseForm: insta.musicForm(surface, sessionID, cursor),
			Q:               q,
			SearchSessionID: searchID,
			FromTypeahead:   "false",
			FromSearch:      "true",
			FromTypingPage:  "false",
		}
	})
}

type audioPageForm struct {
	UUID                      string `json:"_uuid"`
	AudioAssetID              string `json:"audio_asset_id,omitempty"`
	AudioClusterID            string `json:"audio_cluster_id,omitempty"`
	OriginalSoundAudioAssetID string `json:"original_sound_audio_asset_id,omitempty"`
	MaxID                     string `json:"max_id,omitempty"`
}

// Reels pages through the Reels that use the audio (its audio page).
func (a *Audio) Reels() *Pager[*Item] {
	insta := a.insta
	form := audioPageForm{UUID: insta.uuid}
	if a.IsOriginalSound {
		form.OriginalSoundAudioAssetID = a.AudioAssetID
	} else {
		// The app sends the cluster id as both.
		form.AudioAssetID, form.AudioClusterID = a.AudioClusterID, a.AudioClusterID
	}
	var pos int
	return newPager(func(ctx context.Context) ([]*Item, string, error) {
		var page struct {
			Items []struct {
				Media *Item `json:"media"`
			} `json:"items"`
			PagingInfo struct {
				MaxID         string `json:"max_id"`
				MoreAvailable bool   `json:"more_available"`
			} `json:"paging_info"`
		}
		if err := insta.callJSON(ctx, &reqOptions{Endpoint: urlClipsMusic, IsPost: true, Form: form}, &page); err != nil {
			return nil, "", err
		}
		items := make([]*Item, len(page.Items))
		for i, it := range page.Items {
			items[i] = it.Media
		}
		form.MaxID = cursorIf(page.PagingInfo.MoreAvailable, page.PagingInfo.MaxID)
		return numberItems(items, insta, "audio_page", &pos), form.MaxID, nil
	})
}

var musicBrowsers = map[MusicSurface]string{MusicReels: urlMusicClipsBrowser, MusicStory: urlMusicStoriesBrowser}

// MusicBrowser loads the first page of a composer's music picker.
func (insta *Instagram) MusicBrowser(ctx context.Context, surface MusicSurface) ([]*MusicPlaylist, error) {
	endpoint, ok := musicBrowsers[surface]
	if !ok {
		return nil, fmt.Errorf("music browser: unknown surface %q", surface)
	}
	var resp struct {
		Items []struct {
			Playlist struct {
				ID           string      `json:"id"`
				Title        string      `json:"title"`
				PlaylistType string      `json:"playlist_type"`
				HasNextPage  bool        `json:"has_next_page"`
				PreviewItems []musicItem `json:"preview_items"`
			} `json:"playlist"`
		} `json:"items"`
	}
	err := insta.callJSON(ctx, &reqOptions{Endpoint: endpoint, IsPost: true, Form: insta.musicForm(surface, generateUUID(), "")}, &resp)
	if err != nil {
		return nil, err
	}
	out := make([]*MusicPlaylist, len(resp.Items))
	for i, it := range resp.Items {
		p := it.Playlist
		out[i] = &MusicPlaylist{insta: insta, surface: surface, ID: p.ID, Title: p.Title, Type: p.PlaylistType,
			HasMore: p.HasNextPage, Audios: audios(insta, p.PreviewItems)}
	}
	return out, nil
}
