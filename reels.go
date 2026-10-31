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
		req := clipsDiscoverRequest{
			UUID:                       insta.uuid,
			ContainerModule:            "clips_viewer_clips_tab",
			Reason:                     "navigate",
			SeenReels:                  "[]",
			ShouldRefetchChainingMedia: "false",
			ViewerSessionID:            sessionID,
			ClientViewStateMediaList:   "[]",
			SessionInfo:                `{"session_id":"` + sessionID + `","media_info":{}}`,
		}
		friendly := "IgApi: clips/discover/stream/"
		if maxID != "" {
			req.MaxID, req.Reason = maxID, "tailload"
			friendly += "_tail"
		}
		// Lines hold different reels; the last one carries the cursor.
		var resp struct {
			Items []struct {
				Media *Item `json:"media"`
			} `json:"items_with_ads"`
			Paging struct {
				MoreAvailable bool   `json:"more_available"`
				MaxID         string `json:"max_id"`
			} `json:"paging_info"`
		}
		var items []*Item
		seen := map[string]bool{}
		err := insta.callLines(ctx, &reqOptions{
			Endpoint:     urlClipsDiscover,
			FriendlyName: friendly,
			IsPost:       true,
			Form:         req,
			ExtraHeaders: map[string]string{
				"X-Ig-Tail-Load-Request": strconv.FormatBool(maxID != ""),
				"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + insta.sess.AppID +
					`","surface":"clips_viewer_clips_tab","request_category":"api","purpose":"fetch","retry_attempt":"0"}}`,
			},
		}, func(line []byte) error {
			resp.Items = nil
			if err := json.Unmarshal(line, &resp); err != nil {
				return err
			}
			for _, it := range resp.Items {
				if it.Media == nil || seen[it.Media.ID] {
					continue
				}
				seen[it.Media.ID] = true
				setToItem(it.Media, insta, "clips_viewer_clips_tab")
				it.Media.Index = pos
				pos++
				items = append(items, it.Media)
			}
			return nil
		})
		if err != nil {
			return nil, "", err
		}
		maxID = resp.Paging.MaxID
		return items, cursorIf(resp.Paging.MoreAvailable, maxID), nil
	})
}

type clipsDiscoverRequest struct {
	UUID                       string `json:"_uuid"`
	ContainerModule            string `json:"container_module"`
	Reason                     string `json:"reason"`
	SeenReels                  string `json:"seen_reels"`
	ShouldRefetchChainingMedia string `json:"should_refetch_chaining_media"`
	ViewerSessionID            string `json:"viewer_session_id"`
	ClientViewStateMediaList   string `json:"client_view_state_media_list"`
	SessionInfo                string `json:"session_info"`
	MaxID                      string `json:"max_id,omitempty"`
}

type clipsSeenRequest struct {
	sessionIDs
	Impressions      string `json:"impressions"`
	BlendImpressions string `json:"blend_impressions"`
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
	_, err = insta.call(ctx, &reqOptions{Endpoint: urlClipsWriteSeen, IsPost: true, Signed: clipsSeenRequest{
		sessionIDs:       insta.sessionIDs(),
		Impressions:      string(impressions),
		BlendImpressions: "[]",
	}})
	return err
}
