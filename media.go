package goinsta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
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
		FullNeme           string  `json:"full_name"`
		IsPrivate          bool    `json:"is_private"`
		IsVerified         bool    `json:"is_verified"`
		Pk                 float64 `json:"pk"`
		ProfilePicID       string  `json:"profile_pic_id"`
		ProfilePicURL      string  `json:"profile_pic_url"`
		Username           string  `json:"username"`
	} `json:"facepile_top_likers"`
	MediaType             int     `json:"media_type"`
	Code                  string  `json:"code"`
	ClientCacheKey        string  `json:"client_cache_key"`
	FilterType            int     `json:"filter_type"`
	User                  User    `json:"user"`
	CanReply              bool    `json:"can_reply"`
	CanReshare            bool    `json:"can_reshare"` // Used for stories
	CanViewerReshare      bool    `json:"can_viewer_reshare"`
	Caption               Caption `json:"caption"`
	CaptionIsEdited       bool    `json:"caption_is_edited"`
	LikeViewCountDisabled bool    `json:"like_and_view_counts_disabled"`
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
	Toplikers  interface{} `json:"top_likers"`
	Likers     []*User     `json:"likers"`
	PhotoOfYou bool        `json:"photo_of_you"`

	CommentLikesEnabled          bool `json:"comment_likes_enabled"`
	CommentThreadingEnabled      bool `json:"comment_threading_enabled"`
	HasMoreComments              bool `json:"has_more_comments"`
	MaxNumVisiblePreviewComments int  `json:"max_num_visible_preview_comments"`

	CommentCount int `json:"comment_count"`

	// Previewcomments can be `string` or `[]string` or `[]Comment`.
	// Use PreviewComments function instead of getting it directly.
	Previewcomments interface{} `json:"preview_comments,omitempty"`

	Tags struct {
		In []Tag `json:"in"`
	} `json:"usertags,omitempty"`
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
		ElegibleForInsertingAds bool `json:"is_video_elegible_for_inserting_ads"`
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
		Type  int         `json:"type"`
		Value interface{} `json:"value"`
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
	StoryEvents              []interface{}      `json:"story_events"`
	StoryHashtags            []interface{}      `json:"story_hashtags"`
	StoryPolls               []interface{}      `json:"story_polls"`
	StoryFeedMedia           []interface{}      `json:"story_feed_media"`
	StorySoundOn             []interface{}      `json:"story_sound_on"`
	CreativeConfig           interface{}        `json:"creative_config"`
	StoryLocations           []interface{}      `json:"story_locations"`
	StorySliders             []interface{}      `json:"story_sliders"`
	StoryQuestions           []interface{}      `json:"story_questions"`
	StoryProductItems        []interface{}      `json:"story_product_items"`
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

func (item *Item) MediaToString() string {
	return MediaToString(item.MediaType)
}

func MediaToString(t int) string {
	switch t {
	case 1:
		return "photo"
	case 2:
		return "video"
	case 6:
		return "ad_map"
	case 7:
		return "live"
	case 8:
		return "carousel"
	case 9:
		return "live_replay"
	case 10:
		return "collection"
	case 11:
		return "audio"
	case 12:
		return "showreel_native"
	case 13:
		return "guide_facade"
	}
	return ""
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

func getname(name string) string {
	nname := name
	i := 1
	for {
		ext := path.Ext(name)

		_, err := os.Stat(name)
		if err != nil {
			break
		}
		if ext != "" {
			nname = strings.Replace(nname, ext, "", -1)
		}
		name = fmt.Sprintf("%s.%d%s", nname, i, ext)
		i++
	}
	return name
}

type bestMedia struct {
	w, h int
	url  string
}

// GetBest accepts []Video or []Candidate.
func GetBest(obj interface{}) string {
	m := bestMedia{}

	switch t := obj.(type) {
	case []Video:
		for _, video := range t {
			if m.w < video.Width && video.Height > m.h && video.URL != "" {
				m.w = video.Width
				m.h = video.Height
				m.url = video.URL
			}
		}
	case []Candidate:
		for _, image := range t {
			if m.w < image.Width && image.Height > m.h && image.URL != "" {
				m.w = image.Width
				m.h = image.Height
				m.url = image.URL
			}
		}
	}
	return m.url
}

var rxpTags = regexp.MustCompile(`#\w+`)

func (item *Item) DownloadTo(dst string) error {
	insta := item.insta
	folder, file := filepath.Split(dst)

	if err := os.MkdirAll(folder, 0o777); err != nil {
		return err
	}

	switch item.MediaType {
	case 1:
		return insta.downloadTo(folder, file, item.Images.Versions)
	case 2:
		return insta.downloadTo(folder, file, item.Videos)
	case 8:
		return item.downloadCarousel(folder, file)
	}

	insta.warnHandler(
		fmt.Sprintf(
			"Unable to download %s media (media type %d), this has not been implemented",
			item.MediaToString(),
			item.MediaType,
		),
	)
	return ErrNoMedia

}

func (item *Item) Download() ([]byte, error) {
	insta := item.insta

	switch item.MediaType {
	case 1:
		url := GetBest(item.Images.Versions)
		return insta.download(url)
	case 2:
		url := GetBest(item.Videos)
		return insta.download(url)
	case 8:
		return nil, fmt.Errorf("Unable to download a carousel with this method, use DownloadTo instead to save it to a file. If this is a feature you wish to use please let me know.")
	}
	return nil, ErrNoMedia
}

func (item *Item) downloadCarousel(folder, fn string) error {
	if fn == "" {
		fn = item.ID
	}
	for i, media := range item.CarouselMedia {
		n := fmt.Sprintf("%s_%d", fn, i+1)
		if err := media.DownloadTo(path.Join(folder, n)); err != nil {
			return err
		}
	}
	return nil
}

func (insta *Instagram) downloadTo(folder, fn string, media interface{}) error {
	url := GetBest(media)
	fn, err := getDownloadName(url, fn)
	if err != nil {
		return err
	}
	b, err := insta.download(url)
	if err != nil {
		return err
	}
	err = saveToFolder(folder, fn, b)
	return err
}

func (insta *Instagram) download(url string) ([]byte, error) {
	resp, err := insta.c.Get(url)
	if err != nil {
		return nil, err
	}
	media, err := io.ReadAll(resp.Body)
	return media, err
}

func saveToFolder(folder, fn string, media []byte) error {
	dst := path.Join(folder, fn)

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	b := bytes.NewBuffer(media)
	_, err = io.Copy(file, b)
	return err

}

func getDownloadName(url, name string) (string, error) {
	u, err := neturl.Parse(url)
	if err != nil {
		return "", err
	}
	ext := path.Ext(u.Path)
	if name == "" {
		name = path.Base(u.Path)
	} else if !strings.HasSuffix(name, ext) {
		name += ext
	}
	name = getname(name)
	return name, nil
}

func (item *Item) TopLikers() []string {
	switch s := item.Toplikers.(type) {
	case string:
		return []string{s}
	case []string:
		return s
	}
	return nil
}

func (item *Item) PreviewComments() []Comment {
	switch s := item.Previewcomments.(type) {
	case []interface{}:
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
		case interface{}:
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
