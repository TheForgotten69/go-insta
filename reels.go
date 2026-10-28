package goinsta

import (
	"context"
	"encoding/json"
	"strconv"
)

func (insta *Instagram) Reels() *Pager[*Item] {
	sessionID := generateUUID()
	var maxID string
	var pos int
	return newPager(func(ctx context.Context) ([]*Item, string, error) {
		q := map[string]string{
			"_uuid":                         insta.uuid,
			"container_module":              "clips_viewer_clips_tab",
			"reason":                        "navigate",
			"seen_reels":                    "[]",
			"should_refetch_chaining_media": "false",
			"viewer_session_id":             sessionID,
			"client_view_state_media_list":  "[]",
			"session_info":                  `{"session_id":"` + sessionID + `","media_info":{}}`,
		}
		friendly := "IgApi: clips/discover/stream/"
		if maxID != "" {
			q["max_id"] = maxID
			// Tail paging was captured only for reels opened from the feed (clips_viewer_newsfeed_you).
			q["reason"] = "tailload"
			friendly += "_tail"
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
		err := insta.callJSON(ctx, &reqOptions{
			Endpoint:     urlClipsDiscover,
			FriendlyName: friendly,
			IsPost:       true,
			Stream:       true,
			Query:        q,
			ExtraHeaders: map[string]string{
				"X-Ig-Tail-Load-Request": strconv.FormatBool(maxID != ""),
				"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + insta.sess.AppID +
					`","surface":"clips_viewer_clips_tab","request_category":"api","purpose":"fetch","retry_attempt":"0"}}`,
			},
		}, &resp)
		if err != nil {
			return nil, "", err
		}
		maxID = resp.Paging.MaxID
		items := make([]*Item, 0, len(resp.Items))
		for _, it := range resp.Items {
			if it.Media != nil {
				setToItem(it.Media, insta, "clips_viewer_clips_tab")
				it.Media.Index = pos
				pos++
				items = append(items, it.Media)
			}
		}
		return items, cursorIf(resp.Paging.MoreAvailable, maxID), nil
	})
}

func (insta *Instagram) MarkReelsSeen(ctx context.Context, items ...*Item) error {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Pk.String()
	}
	impressions, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	q, err := signedBody(map[string]string{
		"_uid":              strconv.FormatInt(insta.userID(), 10),
		"_uuid":             insta.uuid,
		"impressions":       string(impressions),
		"blend_impressions": "[]",
	})
	if err != nil {
		return err
	}
	_, err = insta.call(ctx, &reqOptions{Endpoint: urlClipsWriteSeen, IsPost: true, Query: q})
	return err
}
