package goinsta

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type StoryReelMention struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Z           int     `json:"z"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Rotation    float64 `json:"rotation"`
	IsPinned    int     `json:"is_pinned"`
	IsHidden    int     `json:"is_hidden"`
	IsSticker   int     `json:"is_sticker"`
	IsFBSticker int     `json:"is_fb_sticker"`
	User        User
	DisplayType string `json:"display_type"`
}

type StoryCTA struct {
	Links []struct {
		LinkType                                int         `json:"linkType"`
		WebURI                                  string      `json:"webUri"`
		AndroidClass                            string      `json:"androidClass"`
		Package                                 string      `json:"package"`
		DeeplinkURI                             string      `json:"deeplinkUri"`
		CallToActionTitle                       string      `json:"callToActionTitle"`
		RedirectURI                             interface{} `json:"redirectUri"`
		LeadGenFormID                           string      `json:"leadGenFormId"`
		IgUserID                                string      `json:"igUserId"`
		AppInstallObjectiveInvalidationBehavior interface{} `json:"appInstallObjectiveInvalidationBehavior"`
	} `json:"links"`
}

type StoryMedia struct {
	Reel   Reel   `json:"reel"`
	Status string `json:"status"`
}

type Reel struct {
	insta *Instagram

	ID                     json.RawMessage `json:"id"`
	Items                  []*Item         `json:"items"`
	MediaCount             int             `json:"media_count"`
	MediaIDs               []Int64         `json:"media_ids"`
	Muted                  bool            `json:"muted"`
	LatestReelMedia        Int64           `json:"latest_reel_media"`
	LatestBestiesReelMedia float64         `json:"latest_besties_reel_media"`
	ExpiringAt             float64         `json:"expiring_at"`
	Seen                   float64         `json:"seen"`
	SeenRankedPosition     int             `json:"seen_ranked_position"`
	CanReply               bool            `json:"can_reply"`
	CanGifQuickReply       bool            `json:"can_gif_quick_reply"`
	ClientPrefetchScore    float64         `json:"client_prefetch_score"`
	Title                  string          `json:"title"`
	CanReshare             bool            `json:"can_reshare"`
	ReelType               string          `json:"reel_type"`
	ReelMentions           []string        `json:"reel_mentions"`
	PrefetchCount          int             `json:"prefetch_count"`
	HasBestiesMedia        interface{}     `json:"has_besties_media"`
	HasPrideMedia          bool            `json:"has_pride_media"`
	HasVideo               bool            `json:"has_video"`
	IsCacheable            bool            `json:"is_cacheable"`
	IsSensitiveVerticalAd  bool            `json:"is_sensitive_vertical_ad"`
	RankedPosition         int             `json:"ranked_position"`
	RankerScores           struct {
		Fp   float64 `json:"fp"`
		Ptap float64 `json:"ptap"`
		Vm   float64 `json:"vm"`
	} `json:"ranker_scores"`
	StoryRankingToken    string `json:"story_ranking_token"`
	FaceFilterNuxVersion int    `json:"face_filter_nux_version"`
	HasNewNuxStory       bool   `json:"has_new_nux_story"`
	User                 User   `json:"user"`
}

func (user *User) Stories() (*StoryMedia, error) {
	return user.insta.fetchStories(int64(user.ID))
}

// Highlights returns the highlight covers; call Reel.Sync to load their items.
func (user *User) Highlights() ([]*Reel, error) {
	body, _, err := user.insta.sendRequest(&reqOptions{
		Endpoint:     fmt.Sprintf(urlUserHighlights, user.ID),
		Stream:       true,
		ExtraHeaders: map[string]string{"X-Ig-Accept-Hint": "feed"},
		Query: map[string]string{
			"check_highlights_subtab_gating":        "false",
			"fetch_high_res_cover_image":            "true",
			"max_highlights_to_fetch_on_pagination": "10",
			"supported_capabilities_new":            supportedCapabilities,
		},
	})
	if err != nil {
		return nil, err
	}
	tray := &Tray{}
	if err := json.Unmarshal(body, tray); err != nil {
		return nil, err
	}
	tray.set(user.insta)
	return tray.Stories, nil
}

func (media *StoryMedia) setValues(insta *Instagram) {
	media.Reel.setValues(insta)
}

func (media *Reel) setValues(insta *Instagram) {
	media.insta = insta
	media.User.insta = insta
	for i, it := range media.Items {
		setToItem(it, insta, "reel_feed_timeline")
		it.Index = i
	}
}

// ReelID is the tray identifier: a user id, or "highlight:<id>" for highlights.
func (media *Reel) ReelID() string {
	var s string
	if json.Unmarshal(media.ID, &s) == nil {
		return s
	}
	return string(media.ID)
}

func (insta *Instagram) fetchStories(id int64) (*StoryMedia, error) {
	body, _, err := insta.sendRequest(&reqOptions{
		Endpoint: fmt.Sprintf(urlUserStories, id),
		Query:    map[string]string{"supported_capabilities_new": supportedCapabilities},
	})
	if err != nil {
		return nil, err
	}
	m := &StoryMedia{}
	if err := json.Unmarshal(body, m); err != nil {
		return nil, err
	}
	m.setValues(insta)
	return m, nil
}

// FetchReels loads the items of the given reels (user ids or "highlight:<id>").
func (insta *Instagram) FetchReels(reelIDs ...string) (map[string]*Reel, error) {
	q, err := signedBody(map[string]any{
		"_uid":                       strconv.FormatInt(insta.userID(), 10),
		"_uuid":                      insta.uuid,
		"batch_size":                 "0",
		"exclude_media_ids":          "[]",
		"reason":                     "on_tap",
		"reel_ids":                   reelIDs,
		"source":                     "profile",
		"supported_capabilities_new": supportedCapabilities,
	})
	if err != nil {
		return nil, err
	}
	body, _, err := insta.sendRequest(&reqOptions{
		Endpoint:     urlReelsMediaStream,
		IsPost:       true,
		Stream:       true,
		Query:        q,
		ExtraHeaders: map[string]string{"X-Ig-Accept-Hint": "feed"},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Reels map[string]*Reel `json:"reels"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	for _, r := range resp.Reels {
		r.setValues(insta)
	}
	return resp.Reels, nil
}

// Sync loads the reel's items (needed for highlights from User.Highlights).
func (media *Reel) Sync() error {
	reels, err := media.insta.FetchReels(media.ReelID())
	if err != nil {
		return err
	}
	r, ok := reels[media.ReelID()]
	if !ok {
		return ErrNoMedia
	}
	insta := media.insta
	*media = *r
	media.setValues(insta)
	return nil
}

func (media *Reel) MarkSeen(items ...*Item) error {
	insta := media.insta
	reelID := media.ReelID()
	now := time.Now().Unix()
	reels := map[string][]string{}
	for _, it := range items {
		reels[it.ID+"_"+reelID] = []string{fmt.Sprintf("%d_%d", it.TakenAt, now)}
	}
	module := "reel_feed_timeline"
	if strings.HasPrefix(reelID, "highlight:") {
		module = "highlight_profile"
	}
	q, err := signedBody(map[string]any{
		"_uid":                 strconv.FormatInt(insta.userID(), 10),
		"_uuid":                insta.uuid,
		"container_module":     module,
		"force_seen_story_ids": []string{},
		"nuxes":                map[string]any{},
		"nuxes_skipped":        map[string]any{},
		"reel_media_skipped":   map[string]any{},
		"reels":                reels,
	})
	if err != nil {
		return err
	}
	_, _, err = insta.sendRequest(&reqOptions{
		Endpoint:     urlMediaSeenV2 + "?reel=1&live_vod=0",
		FriendlyName: "IgApi: media/seen/?reel=1&live_vod=0",
		UseV2:        true,
		IsPost:       true,
		Gzip:         true,
		Query:        q,
	})
	return err
}

func (item *Item) LikeStory() error {
	module := "reel_feed_timeline"
	if item.module != "" && item.module != "reel_feed_timeline" {
		module = item.module
	}
	q, err := signedBody(map[string]string{
		"_uid":              strconv.FormatInt(item.insta.userID(), 10),
		"_uuid":             item.insta.uuid,
		"container_module":  module,
		"delivery_class":    item.deliveryClass(),
		"media_id":          item.ID,
		"tray_position":     strconv.Itoa(item.Index),
		"tray_session_id":   generateUUID(),
		"viewer_session_id": generateUUID(),
	})
	if err != nil {
		return err
	}
	_, _, err = item.insta.sendRequest(&reqOptions{Endpoint: urlStoryLike, IsPost: true, Query: q})
	return err
}
