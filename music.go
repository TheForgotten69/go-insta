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
func (m musicItem) audio() *Audio {
	if t := m.Track; t != nil {
		return &Audio{
			AudioAssetID: t.AudioAssetID, AudioClusterID: t.AudioClusterID, Title: t.Title, Artist: t.DisplayArtist,
			DurationMs: t.DurationInMs, CoverURL: t.CoverArtworkURI, DownloadURL: t.ProgressiveDownloadURL,
			Trending: m.Metadata.IsTrendingInClips, TrendRank: m.Metadata.TrendRank,
			PreviousTrendRank: m.Metadata.PreviousTrendRank, ReelsCount: m.Metadata.FormattedClipsMediaCount,
		}
	}
	if o := m.OriginalSound; o != nil {
		return &Audio{
			AudioAssetID: o.AudioAssetID.String(), IsOriginalSound: true, Title: o.OriginalAudioTitle, Artist: o.IgArtist.Username,
			DurationMs: o.DurationInMs, CoverURL: o.IgArtist.ProfilePicURL, DownloadURL: o.ProgressiveDownloadURL,
			Trending: o.ConsumptionInfo.IsTrendingInClips, TrendRank: o.TrendRank,
			PreviousTrendRank: o.PreviousTrendRank, ReelsCount: o.FormattedClipsMediaCount,
		}
	}
	return nil
}

func audios(items []musicItem) []*Audio {
	out := make([]*Audio, 0, len(items))
	for _, it := range items {
		if a := it.audio(); a != nil {
			out = append(out, a)
		}
	}
	return out
}

type musicPage struct {
	Items    []musicItem `json:"items"`
	PageInfo struct {
		NextMaxID     *string `json:"next_max_id"`
		MoreAvailable bool    `json:"more_available"`
	} `json:"page_info"`
}

func (p *musicPage) next() string {
	if p.PageInfo.NextMaxID == nil {
		return ""
	}
	return cursorIf(p.PageInfo.MoreAvailable, *p.PageInfo.NextMaxID)
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
	var maxID string
	return newPager(func(ctx context.Context) ([]*Audio, string, error) {
		var page musicPage
		err := insta.callJSON(ctx, &reqOptions{Endpoint: urlMusicTopTrends, IsPost: true, Form: topTrendsForm{
			UUID: insta.uuid, Product: "ig_trending_audio_list", TopAudioTabType: tab, PageSize: "10", MaxID: maxID,
		}}, &page)
		if err != nil {
			return nil, "", err
		}
		maxID = page.next()
		return audios(page.Items), maxID, nil
	})
}

type musicBrowseForm struct {
	UUID                   string       `json:"_uuid"`
	Product                MusicSurface `json:"product"`
	BrowseSessionID        string       `json:"browse_session_id"`
	ShouldDisableExposures string       `json:"should_disable_exposures,omitempty"`
	Cursor                 string       `json:"cursor,omitempty"`
}

func (insta *Instagram) musicForm(surface MusicSurface, sessionID string) musicBrowseForm {
	f := musicBrowseForm{UUID: insta.uuid, Product: surface, BrowseSessionID: sessionID}
	if surface == MusicStory {
		f.ShouldDisableExposures = "true"
	}
	return f
}

// StoryTrendingMusic pages through the popular tracks the story music sticker
// lists, 30 a page. Instagram answers this list for stories, not for Reels.
func (insta *Instagram) StoryTrendingMusic() *Pager[*Audio] {
	sessionID := generateUUID()
	var cursor string
	return newPager(func(ctx context.Context) ([]*Audio, string, error) {
		form := insta.musicForm(MusicStory, sessionID)
		form.Cursor = cursor
		var page musicPage
		if err := insta.callJSON(ctx, &reqOptions{Endpoint: urlMusicTrending, IsPost: true, Form: form}, &page); err != nil {
			return nil, "", err
		}
		cursor = page.next()
		return audios(page.Items), cursor, nil
	})
}

var musicBrowsers = map[MusicSurface]string{MusicReels: urlMusicClipsBrowser, MusicStory: urlMusicStoriesBrowser}

// MusicBrowser loads the first page of a composer's music picker.
func (insta *Instagram) MusicBrowser(ctx context.Context, surface MusicSurface) ([]MusicPlaylist, error) {
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
	err := insta.callJSON(ctx, &reqOptions{Endpoint: endpoint, IsPost: true, Form: insta.musicForm(surface, generateUUID())}, &resp)
	if err != nil {
		return nil, err
	}
	out := make([]MusicPlaylist, len(resp.Items))
	for i, it := range resp.Items {
		p := it.Playlist
		out[i] = MusicPlaylist{ID: p.ID, Title: p.Title, Type: p.PlaylistType, HasMore: p.HasNextPage, Audios: audios(p.PreviewItems)}
	}
	return out, nil
}
