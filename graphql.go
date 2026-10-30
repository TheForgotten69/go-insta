package goinsta

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Captured client_doc_ids (app 453.0.0.55.79); they change with app versions.
const (
	docProfileTimeline = "56030350814162075904140659961"
	docFollowersList   = "284797047911918316998205836755"
	docFollowingList   = "16104639286363954576550227636"

	docArchiveHighlight   = "355043344414417317058010037446"
	docUnarchiveHighlight = "52990869615876622121615642083"
)

type gqlQuery struct {
	url      string // graphQLURL when empty
	friendly string
	docID    string
	root     string
	purpose  string // "fetch", "refresh" or "prefetch"; "" omits it, as list and search queries do
}

func (insta *Instagram) graphQL(ctx context.Context, gq gqlQuery, vars map[string]any) ([]byte, error) {
	url := cmp.Or(gq.url, graphQLURL)
	v, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}
	form := map[string]string{
		"client_doc_id":            gq.docID,
		"fb_api_req_friendly_name": gq.friendly,
		"format":                   "json",
		"locale":                   "user",
		"method":                   "post",
		"server_timestamps":        "true",
		"variables":                string(v),
	}
	headers := map[string]string{
		"Content-Type":                      "application/x-www-form-urlencoded",
		"X-Client-Doc-Id":                   gq.docID,
		"X-Ig-Validate-Null-In-Legacy-Dict": "true",
		"X-Graphql-Client-Library":          "pando",
		"X-Root-Field-Name":                 gq.root,
	}
	tagPurpose := "none"
	if gq.purpose != "" {
		form["purpose"] = gq.purpose
		headers["X-Graphql-Request-Purpose"] = gq.purpose
		tagPurpose = gq.purpose
	}
	headers["X-Fb-Request-Analytics-Tags"] = `{"network_tags":{"product":"` + insta.sess.AppID +
		`","request_category":"graphql","purpose":"` + tagPurpose + `","retry_attempt":"0"}}`
	body, err := insta.call(ctx, &reqOptions{
		Endpoint: url,
		IsPost:   true,
		Gzip:     url == graphQLURL, // graphql_www bodies go uncompressed
		Priority: "u=3, i",
		IgnoreHeaders: append([]string{"X-Ig-Www-Claim", "X-Ig-Family-Device-Id", "X-Fb-Connection-Type",
			"X-Ig-Connection-Type", "X-Ig-Client-Endpoint", "X-Ig-Bloks-Serialize-Payload",
			"X-Ig-Bandwidth-Speed-Kbps", "X-Ig-Bandwidth-Totalbytes-B", "X-Ig-Bandwidth-Totaltime-Ms"}, bloksHeaders...),
		FriendlyName: gq.friendly,
		Query:        form,
		ExtraHeaders: headers,
	})
	if err != nil {
		return nil, err
	}
	return mergeIncremental(body)
}

// mergeIncremental folds a @defer/@stream response (NDJSON: a skeleton, then
// {"path":[...],"data":{...}} patches) into one {"data":...} document.
func mergeIncremental(body []byte) ([]byte, error) {
	lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
	if len(lines) < 2 {
		return body, nil
	}
	var root struct {
		Data map[string]any `json:"data"`
	}
	if err := decodeNumber(lines[0], &root); err != nil {
		return nil, err
	}
	for _, line := range lines[1:] {
		var patch struct {
			Path   []any          `json:"path"`
			Data   any            `json:"data"`
			Errors []GraphQLError `json:"errors"`
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := decodeNumber(line, &patch); err != nil {
			return nil, err
		}
		if patch.Data == nil {
			if len(patch.Errors) > 0 {
				return nil, &patch.Errors[0]
			}
			continue
		}
		if err := applyPatch(root.Data, patch.Path, patch.Data); err != nil {
			return nil, err
		}
	}
	return json.Marshal(root)
}

// GraphQLError is the first error of a response part that came back without
// data. Instagram answers 1357005 for a private account the viewer does not follow.
type GraphQLError struct {
	Code    int    `json:"code"`
	Summary string `json:"summary"`
	Message string `json:"message"`
}

func (e *GraphQLError) Error() string {
	return fmt.Sprintf("graphql error %d: %s: %s", e.Code, e.Summary, e.Message)
}

func decodeNumber(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}

func applyPatch(node any, path []any, data any) error {
	for _, step := range path {
		switch n := node.(type) {
		case map[string]any:
			k, _ := step.(string)
			node = n[k]
		case []any:
			i, err := step.(json.Number).Int64()
			if err != nil || i < 0 || int(i) >= len(n) {
				return fmt.Errorf("graphql patch: bad index %v", step)
			}
			node = n[i]
		default:
			return fmt.Errorf("graphql patch: path %v does not resolve", path)
		}
	}
	dst, ok := node.(map[string]any)
	src, ok2 := data.(map[string]any)
	if !ok || !ok2 {
		return fmt.Errorf("graphql patch: path %v is not an object", path)
	}
	deepMerge(dst, src)
	return nil
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if sm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				deepMerge(dm, sm)
				continue
			}
		}
		dst[k] = v
	}
}

// rootField returns the canonical-named field "1$<name>(...)" from a GraphQL data object.
func rootField(data map[string]json.RawMessage, name string) (json.RawMessage, bool) {
	for k, v := range data {
		if strings.HasPrefix(k, "1$"+name+"(") || k == name {
			return v, true
		}
	}
	return nil, false
}

func (user *User) Posts() *Pager[*Item] {
	var maxID, cursor string
	var pos int
	return newPager(func(ctx context.Context) ([]*Item, string, error) {
		vars := profileTimelineVars(user.ID)
		if maxID != "" {
			vars["max_id"] = maxID
			vars["profile_grid_items_cursor"] = cursor
			vars["pinned_profile_grid_items_ids"] = ""
			vars["request_media_chunk"] = false
		}
		body, err := user.insta.graphQL(ctx, gqlQuery{friendly: "IGProfileTimelineQuery", docID: docProfileTimeline,
			root: "xdt_api__v1__profile_timeline", purpose: "fetch"}, vars)
		if gqlErr := (*GraphQLError)(nil); user.IsPrivate && errors.As(err, &gqlErr) {
			err = fmt.Errorf("%w: %w", ErrPrivateAccount, err)
		}
		if err != nil {
			return nil, "", err
		}
		page, err := parseProfilePosts(user.insta, body, pos)
		if err != nil {
			return nil, "", err
		}
		maxID, cursor = page.nextMaxID, page.gridCursor
		pos += len(page.items)
		return page.items, cursorIf(page.more, maxID), nil
	})
}

func profileTimelineVars(id Int64) map[string]any {
	return map[string]any{
		"request_media_chunk": true, "request_hints_chunk": true, "defer_hints_chunk": true,
		"skip_ig_play_count": false, "enable_carousel_media_count_in_deferred": false,
		"skip_clips_captions_fields": false, "num_previews_for_associated_highlights": 3,
		"enable_profile_fm_integration": false, "include_profile_grid_rendering_option": false,
		"count": 12, "include_fb_mentioned_users": true, "include_associated_highlights": false,
		"include_media_notes_fields": false, "include_accessibility_caption_for_carousel": false,
		"include_eligible_insights_entrypoints": true, "include_videos_for_associated_highlights": false,
		"exclude_user": true, "exclude_collaborative_posts": false,
		"include_is_photo_comments_composer_enabled_for_author": false,
		"initial_count_carousel_media":                          1, "fetch_profile_grid_items": true,
		"include_horizon_attribution_info": false, "force_compute_user_tags": false,
		"exclude_pinned_posts": false, "include_ring_creator_fields": false, "exclude_highlights": false,
		"include_is_unseen_by_viewer": false, "include_timeline_ordered_edge": false,
		"exclude_besties_content": false, "user_id": id.String(),
		"include_hidden_feed_product_tags": false, "enabled_featured_pin_user_ids": false,
		"exclude_comment": "true", "include_hidden_reel_product_tags": false,
	}
}

type profilePage struct {
	items                 []*Item
	more                  bool
	nextMaxID, gridCursor string
}

func parseProfilePosts(insta *Instagram, body []byte, pos int) (*profilePage, error) {
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	raw, ok := rootField(resp.Data, "xdt_api__v1__profile_timeline")
	if !ok || string(raw) == "null" {
		return nil, ErrEmptyResponse
	}
	var tl struct {
		Grid []struct {
			Media *struct {
				*Item
				TakenAt Int64 `json:"1ltaken_at"`
			} `json:"media"`
		} `json:"profile_grid_items"`
		MoreAvailable bool    `json:"more_available"`
		NextMaxID     *string `json:"next_max_id"`
		GridCursor    *string `json:"profile_grid_items_cursor"`
	}
	if err := json.Unmarshal(raw, &tl); err != nil {
		return nil, err
	}
	page := &profilePage{more: tl.MoreAvailable, items: make([]*Item, 0, len(tl.Grid))}
	if tl.NextMaxID != nil {
		page.nextMaxID = *tl.NextMaxID
	}
	if tl.GridCursor != nil {
		page.gridCursor = *tl.GridCursor
	}
	for _, g := range tl.Grid {
		if g.Media == nil || g.Media.Item == nil {
			continue
		}
		it := g.Media.Item
		if it.TakenAt == 0 {
			it.TakenAt = g.Media.TakenAt
		}
		if it.Pk == 0 {
			pk, _ := strconv.ParseInt(it.pk(), 10, 64)
			it.Pk = Int64(pk)
		}
		setToItem(it, insta, "profile")
		it.Index = pos + len(page.items)
		page.items = append(page.items, it)
	}
	return page, nil
}
