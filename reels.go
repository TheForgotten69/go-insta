package goinsta

import (
	"encoding/json"
	"strconv"
)

type Reels struct {
	insta     *Instagram
	sessionID string

	Items         []*Item
	MoreAvailable bool
	NextMaxID     string
}

func (insta *Instagram) Reels() *Reels {
	return &Reels{insta: insta, sessionID: generateUUID()}
}

func (r *Reels) Next() error {
	q := map[string]string{
		"_uuid":                         r.insta.uuid,
		"container_module":              "clips_viewer_clips_tab",
		"reason":                        "navigate",
		"seen_reels":                    "[]",
		"should_refetch_chaining_media": "false",
		"viewer_session_id":             r.sessionID,
		"client_view_state_media_list":  "[]",
		"session_info":                  `{"session_id":"` + r.sessionID + `","media_info":{}}`,
	}
	friendly := "IgApi: clips/discover/stream/"
	if r.NextMaxID != "" {
		q["max_id"] = r.NextMaxID
		// Tail paging was captured only for reels opened from the feed (clips_viewer_newsfeed_you).
		q["reason"] = "tailload"
		friendly += "_tail"
	}
	tail := strconv.FormatBool(r.NextMaxID != "")
	body, _, err := r.insta.sendRequest(&reqOptions{
		Endpoint:     urlClipsDiscover,
		FriendlyName: friendly,
		IsPost:       true,
		Stream:       true,
		Query:        q,
		ExtraHeaders: map[string]string{
			"X-Ig-Tail-Load-Request": tail,
			"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + r.insta.sess.AppID +
				`","surface":"clips_viewer_clips_tab","request_category":"api","purpose":"fetch","retry_attempt":"0"}}`,
		},
	})
	if err != nil {
		return err
	}
	var resp struct {
		Items []struct {
			Media *Item `json:"media"`
		} `json:"items_with_ads"`
		Paging struct {
			MoreAvailable bool   `json:"more_available"`
			MaxID         string `json:"max_id"`
		} `json:"paging_info"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	r.MoreAvailable = resp.Paging.MoreAvailable
	r.NextMaxID = resp.Paging.MaxID
	for _, it := range resp.Items {
		if it.Media != nil {
			setToItem(it.Media, r.insta, "clips_viewer_clips_tab")
			it.Media.Index = len(r.Items)
			r.Items = append(r.Items, it.Media)
		}
	}
	return nil
}

// MarkSeen records reels as watched (clips/write_seen_state).
func (r *Reels) MarkSeen(items ...*Item) error {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Pk.String()
	}
	impressions, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	q, err := signedBody(map[string]string{
		"_uid":              strconv.FormatInt(r.insta.userID(), 10),
		"_uuid":             r.insta.uuid,
		"impressions":       string(impressions),
		"blend_impressions": "[]",
	})
	if err != nil {
		return err
	}
	_, _, err = r.insta.sendRequest(&reqOptions{Endpoint: urlClipsWriteSeen, IsPost: true, Query: q})
	return err
}
