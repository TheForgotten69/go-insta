package goinsta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type Item struct {
	insta  *Instagram
	module string

	TakenAt           Int64  `json:"taken_at"`
	Pk                Int64  `json:"pk"`
	ID                string `json:"id"`
	Index             int    // position in feed
	CommentsDisabled  bool   `json:"comments_disabled"`
	DeviceTimestamp   Int64  `json:"device_timestamp"`
	FacepileTopLikers []struct {
		FollowFrictionType float64 `json:"follow_friction_type"`
		FullName           string  `json:"full_name"`
		IsPrivate          bool    `json:"is_private"`
		IsVerified         bool    `json:"is_verified"`
		Pk                 float64 `json:"pk"`
		ProfilePicID       string  `json:"profile_pic_id"`
		ProfilePicURL      string  `json:"profile_pic_url"`
		Username           string  `json:"username"`
	} `json:"facepile_top_likers"`
	MediaType             MediaType `json:"media_type"`
	Code                  string    `json:"code"`
	ClientCacheKey        string    `json:"client_cache_key"`
	FilterType            int       `json:"filter_type"`
	User                  User      `json:"user"`
	CanReply              bool      `json:"can_reply"`
	CanReshare            bool      `json:"can_reshare"` // Used for stories
	CanViewerReshare      bool      `json:"can_viewer_reshare"`
	Caption               Caption   `json:"caption"`
	CaptionIsEdited       bool      `json:"caption_is_edited"`
	LikeViewCountDisabled bool      `json:"like_and_view_counts_disabled"`
	FundraiserTag         struct {
		HasStandaloneFundraiser bool `json:"has_standalone_fundraiser"`
	} `json:"fundraiser_tag"`
	IsSeen                       bool   `json:"is_seen"`
	InventorySource              string `json:"inventory_source"`
	ProductType                  string `json:"product_type"`
	Likes                        int    `json:"like_count"`
	HasLiked                     bool   `json:"has_liked"`
	NearlyCompleteCopyRightMatch bool   `json:"nearly_complete_copyright_match"`
	// Toplikers can be `string` or `[]string`.
	// Use TopLikers function instead of getting it directly.
	Toplikers  any     `json:"top_likers"`
	Likers     []*User `json:"likers"`
	PhotoOfYou bool    `json:"photo_of_you"`

	CommentLikesEnabled          bool `json:"comment_likes_enabled"`
	CommentThreadingEnabled      bool `json:"comment_threading_enabled"`
	HasMoreComments              bool `json:"has_more_comments"`
	MaxNumVisiblePreviewComments int  `json:"max_num_visible_preview_comments"`

	CommentCount int `json:"comment_count"`

	// Previewcomments can be `string` or `[]string` or `[]Comment`.
	// Use PreviewComments function instead of getting it directly.
	Previewcomments any `json:"preview_comments,omitempty"`

	Tags                 Tag      `json:"usertags,omitempty"`
	FbUserTags           Tag      `json:"fb_user_tags"`
	CanViewerSave        bool     `json:"can_viewer_save"`
	OrganicTrackingToken string   `json:"organic_tracking_token"`
	Images               Images   `json:"image_versions2,omitempty"`
	OriginalWidth        int      `json:"original_width,omitempty"`
	OriginalHeight       int      `json:"original_height,omitempty"`
	ImportedTakenAt      Int64    `json:"imported_taken_at,omitempty"`
	Location             Location `json:"location,omitempty"`
	Lat                  float64  `json:"lat,omitempty"`
	Lng                  float64  `json:"lng,omitempty"`

	CarouselParentID string `json:"carousel_parent_id"`
	CarouselMedia    []Item `json:"carousel_media,omitempty"`

	IsPostLive bool `json:"is_post_live"`

	Videos            []Video `json:"video_versions,omitempty"`
	VideoCodec        string  `json:"video_codec"`
	HasAudio          bool    `json:"has_audio,omitempty"`
	VideoDuration     float64 `json:"video_duration,omitempty"`
	ViewCount         float64 `json:"view_count,omitempty"`
	PlayCount         float64 `json:"play_count,omitempty"`
	IsDashEligible    int     `json:"is_dash_eligible,omitempty"`
	IsUnifiedVideo    bool    `json:"is_unified_video"`
	VideoDashManifest string  `json:"video_dash_manifest,omitempty"`
	NumberOfQualities int     `json:"number_of_qualities,omitempty"`

	Title                    string `json:"title"`
	IGTVExistsInViewerSeries bool   `json:"igtv_exists_in_viewer_series"`
	IGTVSeriesInfo           struct {
		HasCoverPhoto bool `json:"has_cover_photo"`
		ID            int64
		NumEpisodes   int    `json:"num_episodes"`
		Title         string `json:"title"`
	} `json:"igtv_series_info"`
	IGTVAdsInfo struct {
		AdsToggledOn            bool `json:"ads_toggled_on"`
		EligibleForInsertingAds bool `json:"is_video_elegible_for_inserting_ads"`
	} `json:"igtv_ads_info"`

	IsCommercial        bool   `json:"is_commercial"`
	IsPaidPartnership   bool   `json:"is_paid_partnership"`
	CommercialityStatus string `json:"commerciality_status"`
	AdLink              string `json:"link"`
	AdLinkText          string `json:"link_text"`
	AdLinkHint          string `json:"link_hint_text"`
	AdTitle             string `json:"overlay_title"`
	AdSubtitle          string `json:"overlay_subtitle"`
	AdText              string `json:"overlay_text"`
	AdAction            string `json:"ad_action"`
	AdHeaderStyle       int    `json:"ad_header_style"`
	AdLinkType          int    `json:"ad_link_type"`
	AdMetadata          []struct {
		Type  int `json:"type"`
		Value any `json:"value"`
	} `json:"ad_metadata"`
	AndroidLinks []struct {
		AndroidClass      string `json:"androidClass"`
		CallToActionTitle string `json:"callToActionTitle"`
		DeeplinkUri       string `json:"deeplinkUri"`
		LinkType          int    `json:"linkType"`
		Package           string `json:"package"`
		WebUri            string `json:"webUri"`
	} `json:"android_links"`

	// Only for stories
	StoryEvents              []any              `json:"story_events"`
	StoryHashtags            []any              `json:"story_hashtags"`
	StoryPolls               []any              `json:"story_polls"`
	StoryFeedMedia           []any              `json:"story_feed_media"`
	StorySoundOn             []any              `json:"story_sound_on"`
	CreativeConfig           any                `json:"creative_config"`
	StoryLocations           []any              `json:"story_locations"`
	StorySliders             []any              `json:"story_sliders"`
	StoryQuestions           []any              `json:"story_questions"`
	StoryProductItems        []any              `json:"story_product_items"`
	StoryCTA                 []StoryCTA         `json:"story_cta"`
	IntegrityReviewDecision  string             `json:"integrity_review_decision"`
	IsReelMedia              bool               `json:"is_reel_media"`
	ProfileGridControl       bool               `json:"profile_grid_control_enabled"`
	ReelMentions             []StoryReelMention `json:"reel_mentions"`
	ExpiringAt               Int64              `json:"expiring_at"`
	CanSendCustomEmojis      bool               `json:"can_send_custom_emojis"`
	SupportsReelReactions    bool               `json:"supports_reel_reactions"`
	ShowOneTapFbShareTooltip bool               `json:"show_one_tap_fb_share_tooltip"`
	HasSharedToFb            Int64              `json:"has_shared_to_fb"`
	Mentions                 []Mentions
	Audience                 string `json:"audience,omitempty"`
	StoryMusicStickers       []struct {
		X              float64 `json:"x"`
		Y              float64 `json:"y"`
		Z              int     `json:"z"`
		Width          float64 `json:"width"`
		Height         float64 `json:"height"`
		Rotation       float64 `json:"rotation"`
		IsPinned       int     `json:"is_pinned"`
		IsHidden       int     `json:"is_hidden"`
		IsSticker      int     `json:"is_sticker"`
		MusicAssetInfo struct {
			ID                       string `json:"id"`
			Title                    string `json:"title"`
			Subtitle                 string `json:"subtitle"`
			DisplayArtist            string `json:"display_artist"`
			CoverArtworkURI          string `json:"cover_artwork_uri"`
			CoverArtworkThumbnailURI string `json:"cover_artwork_thumbnail_uri"`
			ProgressiveDownloadURL   string `json:"progressive_download_url"`
			HighlightStartTimesInMs  []int  `json:"highlight_start_times_in_ms"`
			IsExplicit               bool   `json:"is_explicit"`
			DashManifest             string `json:"dash_manifest"`
			HasLyrics                bool   `json:"has_lyrics"`
			AudioAssetID             string `json:"audio_asset_id"`
			IgArtist                 struct {
				Pk            Int64  `json:"pk"`
				Username      string `json:"username"`
				FullName      string `json:"full_name"`
				IsPrivate     bool   `json:"is_private"`
				ProfilePicURL string `json:"profile_pic_url"`
				ProfilePicID  string `json:"profile_pic_id"`
				IsVerified    bool   `json:"is_verified"`
			} `json:"ig_artist"`
			PlaceholderProfilePicURL string `json:"placeholder_profile_pic_url"`
			ShouldMuteAudio          bool   `json:"should_mute_audio"`
			ShouldMuteAudioReason    string `json:"should_mute_audio_reason"`
			OverlapDurationInMs      int    `json:"overlap_duration_in_ms"`
			AudioAssetStartTimeInMs  int    `json:"audio_asset_start_time_in_ms"`
			StoryLinkStickers        []struct {
				X           float64 `json:"x"`
				Y           float64 `json:"y"`
				Z           int     `json:"z"`
				Width       float64 `json:"width"`
				Height      float64 `json:"height"`
				Rotation    int     `json:"rotation"`
				IsPinned    int     `json:"is_pinned"`
				IsHidden    int     `json:"is_hidden"`
				IsSticker   int     `json:"is_sticker"`
				IsFbSticker int     `json:"is_fb_sticker"`
				StoryLink   struct {
					LinkType   string `json:"link_type"`
					URL        string `json:"url"`
					LinkTitle  string `json:"link_title"`
					DisplayURL string `json:"display_url"`
				} `json:"story_link"`
			} `json:"story_link_stickers"`
		} `json:"music_asset_info"`
	} `json:"story_music_stickers,omitempty"`
}

type MediaType int

const (
	MediaPhoto MediaType = 1
	MediaVideo MediaType = 2
	MediaAlbum MediaType = 8
)

var mediaTypeNames = map[MediaType]string{
	1: "photo", 2: "video", 6: "ad_map", 7: "live", 8: "carousel", 9: "live_replay",
	10: "collection", 11: "audio", 12: "showreel_native", 13: "guide_facade",
}

func (t MediaType) String() string {
	if n, ok := mediaTypeNames[t]; ok {
		return n
	}
	return "media_type_" + strconv.Itoa(int(t))
}

func setToItem(item *Item, insta *Instagram, module string) {
	item.insta = insta
	item.module = module
	item.User.insta = insta
	for i := range item.CarouselMedia {
		item.CarouselMedia[i].User = item.User
		setToItem(&item.CarouselMedia[i], insta, module)
	}
}

type sized interface{ size() (w, h int, url string) }

func (v Video) size() (int, int, string)     { return v.Width, v.Height, v.URL }
func (c Candidate) size() (int, int, string) { return c.Width, c.Height, c.URL }

func bestURL[T sized](xs []T) string {
	var area int
	var url string
	for _, x := range xs {
		if w, h, u := x.size(); u != "" && w*h > area {
			area, url = w*h, u
		}
	}
	return url
}

func (item *Item) mediaURL() (string, error) {
	var url string
	switch item.MediaType {
	case MediaPhoto:
		url = item.Images.Best()
	case MediaVideo:
		url = bestURL(item.Videos)
	}
	if url == "" {
		return "", fmt.Errorf("%s: %w", item.MediaType, ErrNoMedia)
	}
	return url, nil
}

// Download returns the photo or video; a carousel has one per child, so use
// DownloadTo or the children in CarouselMedia.
func (item *Item) Download(ctx context.Context) ([]byte, error) {
	url, err := item.mediaURL()
	if err != nil {
		return nil, err
	}
	return item.insta.download(ctx, url)
}

// DownloadTo saves the media and returns the paths written. dst is a file path,
// or a directory when it ends in a separator or names an existing directory.
// The extension comes from the URL and existing files are kept (a ".N" suffix
// is added). Carousel children download in parallel as <name>_<n>.
func (item *Item) DownloadTo(ctx context.Context, dst string) ([]string, error) {
	if item.MediaType != MediaAlbum {
		url, err := item.mediaURL()
		if err != nil {
			return nil, err
		}
		p, err := item.insta.downloadFile(ctx, url, dst)
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	}
	dir, name := splitDst(dst)
	if name == "" {
		name = item.ID
	}
	paths := make([]string, len(item.CarouselMedia))
	errs := make([]error, len(item.CarouselMedia))
	var wg sync.WaitGroup
	for i := range item.CarouselMedia {
		wg.Go(func() {
			child := &item.CarouselMedia[i]
			url, err := child.mediaURL()
			if err == nil {
				paths[i], err = item.insta.downloadFile(ctx, url, filepath.Join(dir, fmt.Sprintf("%s_%d", name, i+1)))
			}
			errs[i] = err
		})
	}
	wg.Wait()
	return slices.DeleteFunc(paths, func(p string) bool { return p == "" }), errors.Join(errs...)
}

func (insta *Instagram) openMedia(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := insta.client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return resp, nil
}

func (insta *Instagram) download(ctx context.Context, url string) ([]byte, error) {
	resp, err := insta.openMedia(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func splitDst(dst string) (dir, name string) {
	if fi, err := os.Stat(dst); err == nil && fi.IsDir() {
		return dst, ""
	}
	return filepath.Split(dst)
}

func (insta *Instagram) downloadFile(ctx context.Context, url, dst string) (string, error) {
	u, err := neturl.Parse(url)
	if err != nil {
		return "", err
	}
	dir, name := splitDst(dst)
	if name == "" {
		name = path.Base(u.Path)
	}
	if ext := path.Ext(u.Path); !strings.HasSuffix(name, ext) {
		name += ext
	}
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	resp, err := insta.openMedia(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	f, err := createUnique(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}

func createUnique(p string) (*os.File, error) {
	ext := filepath.Ext(p)
	stem := strings.TrimSuffix(p, ext)
	for i := 0; ; i++ {
		name := p
		if i > 0 {
			name = fmt.Sprintf("%s.%d%s", stem, i, ext)
		}
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if !errors.Is(err, fs.ErrExist) {
			return f, err
		}
	}
}

func (item *Item) TopLikers() []string {
	switch s := item.Toplikers.(type) {
	case string:
		return []string{s}
	case []any:
		out := make([]string, 0, len(s))
		for _, v := range s {
			if name, ok := v.(string); ok {
				out = append(out, name)
			}
		}
		return out
	}
	return nil
}

func (item *Item) PreviewComments() []Comment {
	switch s := item.Previewcomments.(type) {
	case []any:
		if len(s) == 0 {
			return nil
		}

		switch s[0].(type) {
		case string:
			comments := make([]Comment, 0)
			for i := range s {
				comments = append(comments, Comment{
					Text: s[i].(string),
				})
			}
			return comments
		case any:
			comments := make([]Comment, 0)
			for i := range s {
				if buf, err := json.Marshal(s[i]); err != nil {
					return nil
				} else {
					comment := &Comment{}

					if err = json.Unmarshal(buf, comment); err != nil {
						return nil
					} else {
						comments = append(comments, *comment)
					}
				}
			}
			return comments
		}
	case string:
		comments := []Comment{
			{
				Text: s,
			},
		}
		return comments
	}
	return nil
}

func (item *Item) StoryIsCloseFriends() bool {
	return item.Audience == "besties"
}
