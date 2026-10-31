package goinsta

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"strconv"
	"time"
)

// Timeline pages the home feed. Its first page is a cold start; Refresh
// reloads the head like pull-to-refresh.
type Timeline struct {
	*Pager[*Item]
	insta   *Instagram
	nextID  string
	started bool
	pos     int
}

type feedCache struct {
	Items []struct {
		MediaOrAd *Item `json:"media_or_ad"`
		// Recommended posts (accounts with few follows) arrive wrapped.
		ExploreStory *struct {
			MediaOrAd *Item `json:"media_or_ad"`
		} `json:"explore_story"`
		EndOfFeed struct {
			Pause    bool   `json:"pause"`
			Title    string `json:"title"`
			Subtitle string `json:"subtitle"`
		} `json:"end_of_feed_demarcator"`
	} `json:"feed_items"`

	MoreAvailable               bool    `json:"more_available"`
	NextID                      string  `json:"next_max_id"`
	NumResults                  float64 `json:"num_results"`
	PullToRefreshWindowMs       float64 `json:"pull_to_refresh_window_ms"`
	RequestID                   string  `json:"request_id"`
	SessionID                   string  `json:"session_id"`
	ViewStateVersion            string  `json:"view_state_version"`
	AutoLoadMore                bool    `json:"auto_load_more_enabled"`
	IsDirectV2Enabled           bool    `json:"is_direct_v2_enabled"`
	ClientFeedChangelistApplied bool    `json:"client_feed_changelist_applied"`
	PreloadDistance             float64 `json:"preload_distance"`
	Status                      string  `json:"status"`
	FeedPillText                string  `json:"feed_pill_text"`
	StartupPrefetchConfigs      struct {
		Explore struct {
			ContainerModule          string `json:"containermodule"`
			ShouldPrefetch           bool   `json:"should_prefetch"`
			ShouldPrefetchThumbnails bool   `json:"should_prefetch_thumbnails"`
		} `json:"explore"`
	} `json:"startup_prefetch_configs"`
	UseAggressiveFirstTailLoad bool    `json:"use_aggressive_first_tail_load"`
	HideLikeAndViewCounts      float64 `json:"hide_like_and_view_counts"`
}

func (insta *Instagram) Timeline() *Timeline {
	tl := &Timeline{insta: insta}
	tl.Pager = newPager(func(ctx context.Context) ([]*Item, string, error) {
		reason := "cold_start_fetch"
		if tl.started {
			reason = "pagination"
		}
		return tl.fetch(ctx, reason)
	})
	return tl
}

func (tl *Timeline) Refresh(ctx context.Context) error {
	tl.pos, tl.nextID = 0, ""
	items, next, err := tl.fetch(ctx, "auto_refresh")
	if err != nil {
		return err
	}
	tl.Items, tl.done, tl.cursor = items, next == "", next
	return nil
}

type timelineRequest struct {
	UUID                        string `json:"_uuid"`
	DeviceID                    string `json:"device_id"`
	PhoneID                     string `json:"phone_id"`
	BloksVersioningID           string `json:"bloks_versioning_id"`
	TimezoneOffset              string `json:"timezone_offset"`
	DeviceTimezoneName          string `json:"device_timezone_name"`
	ClientRecordedRequestTimeMs string `json:"client_recorded_request_time_ms"`
	RequestID                   string `json:"request_id"`
	SessionID                   string `json:"session_id"`
	Reason                      string `json:"reason"`
	IsPullToRefresh             string `json:"is_pull_to_refresh"`
	IsNonFeedSurface            string `json:"is_non_feed_surface"`
	FeedViewInfo                string `json:"feed_view_info"`
	ClientSeenStoreMediaList    string `json:"client_seen_store_media_list"`
	BatteryLevel                string `json:"battery_level"`
	IsCharging                  string `json:"is_charging"`
	IsDarkMode                  string `json:"is_dark_mode"`
	WillSoundOn                 string `json:"will_sound_on"`
	HasCameraPermission         string `json:"has_camera_permission"`
	PushDisabled                string `json:"push_disabled"`
	IncludeAttributionUIData    string `json:"include_attribution_ui_data"`
	MaxID                       string `json:"max_id,omitempty"`
	PaginationSource            string `json:"pagination_source,omitempty"`
}

func (tl *Timeline) fetch(ctx context.Context, reason string) ([]*Item, string, error) {
	insta := tl.insta
	now := time.Now()
	req := timelineRequest{
		UUID:                        insta.uuid,
		DeviceID:                    insta.uuid,
		PhoneID:                     insta.fID,
		BloksVersioningID:           insta.sess.BloksVersionID,
		TimezoneOffset:              insta.sess.TimezoneOffset,
		DeviceTimezoneName:          insta.sess.TimezoneName,
		ClientRecordedRequestTimeMs: strconv.FormatInt(now.UnixMilli(), 10),
		RequestID:                   generateUUID(),
		SessionID:                   insta.feedSessionID,
		Reason:                      reason,
		IsPullToRefresh:             "0",
		IsNonFeedSurface:            "0",
		FeedViewInfo:                "[]",
		BatteryLevel:                strconv.Itoa(60 + rand.IntN(40)),
		IsCharging:                  "1",
		IsDarkMode:                  "1",
		WillSoundOn:                 "0",
		HasCameraPermission:         "1",
		PushDisabled:                "true",
		IncludeAttributionUIData:    "true",
	}
	friendly := "IgApi: feed/timeline/"
	tail := "false"
	headers := map[string]string{
		"X-Ads-Opt-Out":  "0",
		"X-Device-Id":    insta.uuid,
		"X-Google-Ad-Id": insta.sess.GoogleAdID,
		"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + insta.sess.AppID +
			`","surface":"undefined","request_category":"critical_api","purpose":"fetch","retry_attempt":"0"}}`,
	}
	switch reason {
	case "pagination":
		req.MaxID, req.PaginationSource = tl.nextID, "feed_recs"
		friendly += "_tail"
		tail = "true"
	case "auto_refresh":
		req.IsPullToRefresh = "1"
	default:
		headers["X-Ig-Accept-Hint"] = "feed"
	}
	headers["X-Ig-Tail-Load-Request"] = tail

	body, err := insta.call(ctx, &reqOptions{
		Endpoint:     urlTimeline,
		FriendlyName: friendly,
		IsPost:       true,
		Gzip:         true,
		Stream:       true,
		Priority:     "u=0",
		Form:         req,
		ExtraHeaders: headers,
	})
	if err != nil {
		return nil, "", err
	}

	var page feedCache
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, "", err
	}
	tl.started = true
	tl.nextID = page.NextID
	items := make([]*Item, 0, len(page.Items))
	for _, it := range page.Items {
		item := it.MediaOrAd
		if item == nil && it.ExploreStory != nil {
			item = it.ExploreStory.MediaOrAd
		}
		if item != nil {
			setToItem(item, insta, "feed_timeline")
			item.Index = tl.pos
			tl.pos++
			items = append(items, item)
		}
	}
	return items, cursorIf(page.MoreAvailable, page.NextID), nil
}

type TrayReason string

const (
	TrayColdStart     TrayReason = "cold_start"
	TrayPullToRefresh TrayReason = "pull_to_refresh"
)

type trayRequest struct {
	UUID                     string `json:"_uuid"`
	PageSize                 string `json:"page_size"`
	Reason                   string `json:"reason"`
	RequestID                string `json:"request_id"`
	TimezoneOffset           string `json:"timezone_offset"`
	TraySessionID            string `json:"tray_session_id"`
	SupportedCapabilitiesNew string `json:"supported_capabilities_new,omitempty"`
}

// StoryTray loads the stories tray. Reels further down arrive without items;
// LoadReels fills them in batches.
func (insta *Instagram) StoryTray(ctx context.Context, reason TrayReason) (*Tray, error) {
	req := trayRequest{
		UUID:           insta.uuid,
		PageSize:       "50",
		Reason:         string(reason),
		RequestID:      generateUUID(),
		TimezoneOffset: insta.sess.TimezoneOffset,
		TraySessionID:  generateUUID(),
	}
	if reason != TrayColdStart {
		req.SupportedCapabilitiesNew = supportedCapabilities
	}
	tray := &Tray{}
	err := insta.callJSON(ctx, &reqOptions{
		Endpoint:     urlStories,
		IsPost:       true,
		Stream:       true,
		Priority:     "u=0",
		Form:         req,
		ExtraHeaders: map[string]string{"X-Ig-Tail-Load-Request": "false"},
	}, tray)
	if err != nil {
		return nil, err
	}
	tray.set(insta)
	return tray, nil
}

func localTimezoneName() string {
	if name := time.Local.String(); name != "Local" {
		return name
	}
	return "UTC"
}
