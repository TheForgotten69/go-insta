package goinsta

import (
	"encoding/json"
	"math/rand/v2"
	"strconv"
	"time"
)

type Timeline struct {
	insta     *Instagram
	err       error
	sessionID string
	started   bool

	Items []*Item
	Tray  *Tray

	MoreAvailable bool
	NextID        string
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

func newTimeline(insta *Instagram) *Timeline {
	return &Timeline{insta: insta, sessionID: generateUUID()}
}

// Next loads the next page into tl.Items; the first call is a cold start.
// It returns false at the end of the feed (Error() == ErrNoMore) or on error.
func (tl *Timeline) Next() bool {
	if tl.err != nil {
		return false
	}
	reason := "cold_start_fetch"
	if tl.started {
		if !tl.MoreAvailable || tl.NextID == "" {
			tl.err = ErrNoMore
			return false
		}
		reason = "pagination"
	}
	if err := tl.fetch(reason); err != nil {
		tl.err = err
		return false
	}
	return true
}

// Refresh replaces the items with a fresh head of the feed, like pull-to-refresh.
func (tl *Timeline) Refresh() error {
	tl.Items = nil
	tl.err = nil
	return tl.fetch("auto_refresh")
}

func (tl *Timeline) fetch(reason string) error {
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
		"session_id":                      tl.sessionID,
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
		q["max_id"] = tl.NextID
		q["pagination_source"] = "feed_recs"
		friendly += "_tail"
		tail = "true"
	default:
		headers["X-Ig-Accept-Hint"] = "feed"
	case "auto_refresh":
		q["is_pull_to_refresh"] = "1"
	}

	body, _, err := insta.sendRequest(&reqOptions{
		Endpoint:     urlTimeline,
		FriendlyName: friendly,
		IsPost:       true,
		Gzip:         true,
		Stream:       true,
		Priority:     "u=0",
		Query:        q,
		ExtraHeaders: MergeMapS(headers, map[string]string{"X-Ig-Tail-Load-Request": tail}),
	})
	if err != nil {
		return err
	}

	var page feedCache
	if err := json.Unmarshal(body, &page); err != nil {
		return err
	}
	tl.started = true
	tl.NextID = page.NextID
	tl.MoreAvailable = page.MoreAvailable
	for _, it := range page.Items {
		item := it.MediaOrAd
		if item == nil && it.ExploreStory != nil {
			item = it.ExploreStory.MediaOrAd
		}
		if item != nil {
			setToItem(item, insta, "feed_timeline")
			item.Index = len(tl.Items)
			tl.Items = append(tl.Items, item)
		}
	}
	return nil
}

// FetchTray loads the stories tray. reason is "cold_start" or "pull_to_refresh".
func (tl *Timeline) FetchTray(reason string) error {
	insta := tl.insta
	q := map[string]string{
		"_uuid":           insta.uuid,
		"page_size":       "50",
		"reason":          reason,
		"request_id":      generateUUID(),
		"timezone_offset": insta.sess.TimezoneOffset,
		"tray_session_id": generateUUID(),
	}
	if reason != "cold_start" {
		q["supported_capabilities_new"] = supportedCapabilities
	}
	body, _, err := insta.sendRequest(&reqOptions{
		Endpoint:     urlStories,
		IsPost:       true,
		Stream:       true,
		Priority:     "u=0",
		Query:        q,
		ExtraHeaders: map[string]string{"X-Ig-Tail-Load-Request": "false"},
	})
	if err != nil {
		return err
	}
	tray := &Tray{}
	if err := json.Unmarshal(body, tray); err != nil {
		return err
	}
	tray.set(insta)
	tl.Tray = tray
	return nil
}

func (tl *Timeline) Stories() []*Reel {
	if tl.Tray == nil {
		return nil
	}
	return tl.Tray.Stories
}

func (tl *Timeline) Error() error {
	return tl.err
}

func localTimezoneName() string {
	if name := time.Local.String(); name != "Local" {
		return name
	}
	return "UTC"
}
