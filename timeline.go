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

func (tl *Timeline) fetch(ctx context.Context, reason string) ([]*Item, string, error) {
	insta := tl.insta
	now := time.Now()
	q := map[string]string{
		"_uuid":                           insta.uuid,
		"device_id":                       insta.uuid,
		"phone_id":                        insta.fID,
		"bloks_versioning_id":             insta.sess.BloksVersionID,
		"timezone_offset":                 insta.sess.TimezoneOffset,
		"device_timezone_name":            insta.sess.TimezoneName,
		"client_recorded_request_time_ms": strconv.FormatInt(now.UnixMilli(), 10),
		"request_id":                      generateUUID(),
		"session_id":                      insta.feedSessionID,
		"reason":                          reason,
		"is_pull_to_refresh":              "0",
		"feed_view_info":                  "[]",
		"client_seen_store_media_list":    "",
		"battery_level":                   strconv.Itoa(60 + rand.IntN(40)),
		"is_charging":                     "1",
		"is_dark_mode":                    "1",
		"will_sound_on":                   "0",
		"has_camera_permission":           "1",
		"push_disabled":                   "true",
		"include_attribution_ui_data":     "true",
	}
	friendly := "IgApi: feed/timeline/"
	tail := "false"
	headers := map[string]string{
		"X-Ads-Opt-Out":  "0",
		"X-Device-Id":    insta.uuid,
		"X-Google-Ad-Id": insta.sess.GoogleAdID,
		"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + insta.sess.AppID +
			`","surface":"undefined","request_category":"critical_api","purpose":"fetch","retry_attempt":"0"},"application_tags":{"app_start_tag":"lukewarm"}}`,
	}
	switch reason {
	case "pagination":
		q["max_id"] = tl.nextID
		q["pagination_source"] = "feed_recs"
		friendly += "_tail"
		tail = "true"
	case "auto_refresh":
		q["is_pull_to_refresh"] = "1"
	default:
		headers["X-Ig-Accept-Hint"] = "feed"
	}

	body, err := insta.call(ctx, &reqOptions{
		Endpoint:     urlTimeline,
		FriendlyName: friendly,
		IsPost:       true,
		Gzip:         true,
		Stream:       true,
		Priority:     "u=0",
		Query:        q,
		ExtraHeaders: mergeForms(headers, map[string]string{"X-Ig-Tail-Load-Request": tail}),
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

// StoryTray loads the stories tray. Reels further down arrive without items;
// LoadReels fills them in batches.
func (insta *Instagram) StoryTray(ctx context.Context, reason TrayReason) (*Tray, error) {
	q := map[string]string{
		"_uuid":           insta.uuid,
		"page_size":       "50",
		"reason":          string(reason),
		"request_id":      generateUUID(),
		"timezone_offset": insta.sess.TimezoneOffset,
		"tray_session_id": generateUUID(),
	}
	if reason != TrayColdStart {
		q["supported_capabilities_new"] = supportedCapabilities
	}
	tray := &Tray{}
	err := insta.callJSON(ctx, &reqOptions{
		Endpoint:     urlStories,
		IsPost:       true,
		Stream:       true,
		Priority:     "u=0",
		Query:        q,
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
