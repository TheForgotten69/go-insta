package goinsta

import (
	"encoding/json"
	"fmt"
	"strings"
)

// entryContext is the info_stream entry_point/module pair the app sends for the
// screen a profile was opened from.
type entryContext struct{ entryPoint, module string }

var (
	entrySearch    = entryContext{"search_navigate_to_user", "serp_users"}
	entryFollowing = entryContext{"following_list_user_row", "following"}
	// Not captured; mirrors the following-list value.
	entryFollowers = entryContext{"followers_list_user_row", "followers"}
	entrySuggested = entryContext{"suggested_users", "feed_timeline"}
)

func (insta *Instagram) usersFrom(users []*User, ctx entryContext) []*User {
	for _, u := range users {
		u.insta = insta
		u.entry = ctx
	}
	return users
}

type SearchResult struct {
	Users     []*User
	HasMore   bool
	RankToken string
	PageToken string
}

// SearchUsers is the "Accounts" tab of the search results page.
func (insta *Instagram) SearchUsers(query string) (*SearchResult, error) {
	serpSession := generateUUID()
	body, _, err := insta.sendRequest(&reqOptions{
		Endpoint: urlAccountSerp,
		Query: map[string]string{
			"search_surface":  "user_serp",
			"serp_session_id": serpSession,
			"qpl_join_id":     serpSession,
			"session_state":   "typeahead",
			"disable_ads":     "true",
			"enable_lightweight_query_with_basic_mmc_and_media_grid": "true",
			"search_session_id": generateUUID(),
			"timezone_offset":   insta.sess.TimezoneOffset,
			"enable_metadata":   "false",
			"serp_origination":  "keyboard",
			"keyword_context":   "null",
			"count":             "30",
			"query":             query,
		},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Users     []*User `json:"users"`
		HasMore   bool    `json:"has_more"`
		RankToken string  `json:"rank_token"`
		PageToken string  `json:"page_token"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &SearchResult{
		Users:     insta.usersFrom(resp.Users, entrySearch),
		HasMore:   resp.HasMore,
		RankToken: resp.RankToken,
		PageToken: resp.PageToken,
	}, nil
}

type FollowList struct {
	Users     []*User
	HasMore   bool
	NextMaxID string
}

// gqlUser is the XDTUserDict shape: id instead of pk, aliased fields.
type gqlUser struct {
	ID              Int64      `json:"id"`
	Username        string     `json:"username"`
	FullName        string     `json:"full_name"`
	IsPrivate       bool       `json:"is_private"`
	IsVerified      bool       `json:"is_verified"`
	ProfilePicID    string     `json:"profile_pic_id"`
	ProfilePicURL   string     `json:"profile_pic_url"`
	LatestReelMedia Int64      `json:"1llatest_reel_media"`
	Friendship      Friendship `json:"friendship_status"`
}

// Followers returns one page; pass "" then NextMaxID.
func (user *User) Followers(maxID string) (*FollowList, error) {
	vars := map[string]any{
		"user_id":                          user.ID.String(),
		"skip_suggested_users":             false,
		"skip_more_groups_available":       false,
		"skip_friendship_followers_fields": true,
		"request_data":                     map[string]any{"rank_token": "", "enableGroups": true},
		"skip_page_size":                   false,
		"skip_pending_admins":              false,
		"skip_has_more":                    false,
		"search_surface":                   "follow_list_page",
		"query":                            "",
		"skip_big_list":                    false,
		"include_unseen_count":             true,
	}
	if maxID != "" {
		vars["max_id"] = maxID
	}
	return user.followList("FollowersList", docFollowersList, "xdt_api__v1__friendships__followers", vars, entryFollowers)
}

// Following returns one page; pass "" then NextMaxID.
func (user *User) Following(maxID string) (*FollowList, error) {
	requestData := map[string]any{"rank_token": "", "supported_capabilities_new": []any{},
		"includes_hashtags": true, "search_surface": "follow_list_page"}
	for _, k := range []string{"username", "support_new_api", "absolute_url_processed", "phone_id", "target_id",
		"page_size", "login_attempt_count", "module", "limit", "ig_sig_key_version", "first", "device_id", "surface",
		"search_client_type", "ig_sig", "csrfmiddlewaretoken", "query", "adid", "guid", "rank_mutual", "choice",
		"_uuid", "_uid", "_csrftoken"} {
		requestData[k] = nil
	}
	vars := map[string]any{
		"user_id":                             user.ID.String(),
		"skip_use_clickable_see_more":         false,
		"skip_preview_hashtags":               false,
		"skip_should_limit_list_of_followers": false,
		"skip_pending_admins":                 false,
		"skip_more_groups_available":          false,
		"skip_friendship_followers_fields":    false,
		"request_data":                        requestData,
		"skip_page_size":                      false,
		"skip_friend_requests":                false,
		"skip_big_list":                       false,
		"query":                               "",
		"include_profile_update_info":         true,
		"skip_suggested_users":                false,
		"include_unseen_count":                true,
		"skip_has_more":                       false,
		"enable_groups":                       true,
		"skip_hashtag_count":                  false,
	}
	if maxID != "" {
		vars["max_id"] = maxID
	}
	return user.followList("FollowingList", docFollowingList, "xdt_api__v1__friendships__following", vars, entryFollowing)
}

// The response carries a non-fatal "execution error" in errors[] next to valid data.
func (user *User) followList(friendly, doc, root string, vars map[string]any, ctx entryContext) (*FollowList, error) {
	insta := user.insta
	body, err := insta.graphQL(friendly, doc, root, "", vars)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	raw, ok := rootField(resp.Data, root)
	if !ok || string(raw) == "null" {
		return nil, fmt.Errorf("%s: %w", friendly, ErrEmptyResponse)
	}
	var list struct {
		Users     []gqlUser `json:"users"`
		HasMore   bool      `json:"has_more"`
		NextMaxID *string   `json:"next_max_id"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	out := &FollowList{HasMore: list.HasMore}
	if list.NextMaxID != nil {
		out.NextMaxID = *list.NextMaxID
	}
	for _, g := range list.Users {
		out.Users = append(out.Users, &User{
			ID: g.ID, Username: g.Username, FullName: g.FullName, IsPrivate: g.IsPrivate,
			IsVerified: g.IsVerified, ProfilePicID: g.ProfilePicID, ProfilePicURL: g.ProfilePicURL,
			LatestReelMedia: g.LatestReelMedia, Friendship: g.Friendship,
		})
	}
	insta.usersFrom(out.Users, ctx)
	return out, nil
}

// Friendships returns the friendship status for each user id (friendships/show_many).
func (insta *Instagram) Friendships(ids ...int64) (map[string]Friendship, error) {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = Int64(id).String()
	}
	body, _, err := insta.sendRequest(&reqOptions{
		Endpoint: urlFriendshipShowMany,
		IsPost:   true,
		Query: map[string]string{
			"_uuid":               insta.uuid,
			"include_followed_by": "false",
			"user_ids":            strings.Join(s, ", "),
		},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Statuses map[string]Friendship `json:"friendship_statuses"`
	}
	return resp.Statuses, json.Unmarshal(body, &resp)
}
