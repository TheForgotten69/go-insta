package goinsta

import (
	"context"
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

func (insta *Instagram) usersFrom(users []*User, entry entryContext) []*User {
	for _, u := range users {
		u.insta = insta
		u.entry = entry
	}
	return users
}

type searchUsersQuery struct {
	SearchSurface     string `json:"search_surface"`
	SerpSessionID     string `json:"serp_session_id"`
	QplJoinID         string `json:"qpl_join_id"`
	SessionState      string `json:"session_state"`
	DisableAds        string `json:"disable_ads"`
	EnableLightweight string `json:"enable_lightweight_query_with_basic_mmc_and_media_grid"`
	SearchSessionID   string `json:"search_session_id"`
	TimezoneOffset    string `json:"timezone_offset"`
	EnableMetadata    string `json:"enable_metadata"`
	SerpOrigination   string `json:"serp_origination"`
	KeywordContext    string `json:"keyword_context"`
	Count             string `json:"count"`
	Query             string `json:"query"`
	PageToken         string `json:"page_token,omitempty"`
}

// SearchUsers pages the "Accounts" tab of search. The session ids stay fixed
// across pages, as the app keeps them; later pages carry the server's page_token.
func (insta *Instagram) SearchUsers(query string) *Pager[*User] {
	serpSession := generateUUID()
	searchSession := generateUUID()
	var pageToken string
	return newPager(func(ctx context.Context) ([]*User, string, error) {
		var resp struct {
			Users     []*User `json:"users"`
			HasMore   bool    `json:"has_more"`
			PageToken string  `json:"page_token"`
		}
		if err := insta.callJSON(ctx, &reqOptions{Endpoint: urlAccountSerp, Form: searchUsersQuery{
			SearchSurface:     "user_serp",
			SerpSessionID:     serpSession,
			QplJoinID:         serpSession,
			SessionState:      "typeahead",
			DisableAds:        "true",
			EnableLightweight: "true",
			SearchSessionID:   searchSession,
			TimezoneOffset:    insta.sess.TimezoneOffset,
			EnableMetadata:    "false",
			SerpOrigination:   "keyboard",
			KeywordContext:    "null",
			Count:             "30",
			Query:             query,
			PageToken:         pageToken,
		}}, &resp); err != nil {
			return nil, "", err
		}
		pageToken = resp.PageToken
		return insta.usersFrom(resp.Users, entrySearch), cursorIf(resp.HasMore, pageToken), nil
	})
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

func (user *User) Followers() *Pager[*User] {
	return user.followList("FollowersList", docFollowersList, "xdt_api__v1__friendships__followers", entryFollowers, newFollowersVars)
}

func (user *User) Following() *Pager[*User] {
	return user.followList("FollowingList", docFollowingList, "xdt_api__v1__friendships__following", entryFollowing, newFollowingVars)
}

type followersVars struct {
	UserID                        string               `json:"user_id"`
	SkipSuggestedUsers            bool                 `json:"skip_suggested_users"`
	SkipMoreGroupsAvailable       bool                 `json:"skip_more_groups_available"`
	SkipFriendshipFollowersFields bool                 `json:"skip_friendship_followers_fields"`
	RequestData                   followersRequestData `json:"request_data"`
	SkipPageSize                  bool                 `json:"skip_page_size"`
	SkipPendingAdmins             bool                 `json:"skip_pending_admins"`
	SkipHasMore                   bool                 `json:"skip_has_more"`
	SearchSurface                 string               `json:"search_surface"`
	Query                         string               `json:"query"`
	SkipBigList                   bool                 `json:"skip_big_list"`
	IncludeUnseenCount            bool                 `json:"include_unseen_count"`
	MaxID                         string               `json:"max_id,omitempty"`
}

type followersRequestData struct {
	RankToken    string `json:"rank_token"`
	EnableGroups bool   `json:"enableGroups"`
}

func newFollowersVars(id Int64, maxID string) any {
	return followersVars{
		UserID:                        id.String(),
		SkipFriendshipFollowersFields: true,
		RequestData:                   followersRequestData{EnableGroups: true},
		SearchSurface:                 "follow_list_page",
		IncludeUnseenCount:            true,
		MaxID:                         maxID,
	}
}

type followingVars struct {
	UserID                         string               `json:"user_id"`
	SkipUseClickableSeeMore        bool                 `json:"skip_use_clickable_see_more"`
	SkipPreviewHashtags            bool                 `json:"skip_preview_hashtags"`
	SkipShouldLimitListOfFollowers bool                 `json:"skip_should_limit_list_of_followers"`
	SkipPendingAdmins              bool                 `json:"skip_pending_admins"`
	SkipMoreGroupsAvailable        bool                 `json:"skip_more_groups_available"`
	SkipFriendshipFollowersFields  bool                 `json:"skip_friendship_followers_fields"`
	RequestData                    followingRequestData `json:"request_data"`
	SkipPageSize                   bool                 `json:"skip_page_size"`
	SkipFriendRequests             bool                 `json:"skip_friend_requests"`
	SkipBigList                    bool                 `json:"skip_big_list"`
	Query                          string               `json:"query"`
	IncludeProfileUpdateInfo       bool                 `json:"include_profile_update_info"`
	SkipSuggestedUsers             bool                 `json:"skip_suggested_users"`
	IncludeUnseenCount             bool                 `json:"include_unseen_count"`
	SkipHasMore                    bool                 `json:"skip_has_more"`
	EnableGroups                   bool                 `json:"enable_groups"`
	SkipHashtagCount               bool                 `json:"skip_hashtag_count"`
	MaxID                          string               `json:"max_id,omitempty"`
}

// followingRequestData: the app sends the nil-pointer fields as explicit nulls.
type followingRequestData struct {
	RankToken                string   `json:"rank_token"`
	SupportedCapabilitiesNew []string `json:"supported_capabilities_new"`
	IncludesHashtags         bool     `json:"includes_hashtags"`
	SearchSurface            string   `json:"search_surface"`
	Username                 *string  `json:"username"`
	SupportNewAPI            *string  `json:"support_new_api"`
	AbsoluteURLProcessed     *string  `json:"absolute_url_processed"`
	PhoneID                  *string  `json:"phone_id"`
	TargetID                 *string  `json:"target_id"`
	PageSize                 *string  `json:"page_size"`
	LoginAttemptCount        *string  `json:"login_attempt_count"`
	Module                   *string  `json:"module"`
	Limit                    *string  `json:"limit"`
	IgSigKeyVersion          *string  `json:"ig_sig_key_version"`
	First                    *string  `json:"first"`
	DeviceID                 *string  `json:"device_id"`
	Surface                  *string  `json:"surface"`
	SearchClientType         *string  `json:"search_client_type"`
	IgSig                    *string  `json:"ig_sig"`
	CSRFMiddlewareToken      *string  `json:"csrfmiddlewaretoken"`
	Query                    *string  `json:"query"`
	AdID                     *string  `json:"adid"`
	GUID                     *string  `json:"guid"`
	RankMutual               *string  `json:"rank_mutual"`
	Choice                   *string  `json:"choice"`
	UUID                     *string  `json:"_uuid"`
	UID                      *string  `json:"_uid"`
	CSRFToken                *string  `json:"_csrftoken"`
}

func newFollowingVars(id Int64, maxID string) any {
	return followingVars{
		UserID: id.String(),
		RequestData: followingRequestData{
			SupportedCapabilitiesNew: []string{},
			IncludesHashtags:         true,
			SearchSurface:            "follow_list_page",
		},
		IncludeProfileUpdateInfo: true,
		IncludeUnseenCount:       true,
		EnableGroups:             true,
		MaxID:                    maxID,
	}
}

// The response carries a non-fatal "execution error" in errors[] next to valid data.
func (user *User) followList(friendly, doc, root string, entry entryContext, varsFor func(id Int64, maxID string) any) *Pager[*User] {
	insta := user.insta
	var maxID string
	return newPager(func(ctx context.Context) ([]*User, string, error) {
		var list struct {
			Users     []gqlUser `json:"users"`
			HasMore   bool      `json:"has_more"`
			NextMaxID *string   `json:"next_max_id"`
		}
		err := insta.graphQLRoot(ctx, gqlQuery{friendly: friendly, docID: doc, root: root, gzip: true}, varsFor(user.ID, maxID), &list)
		if err != nil {
			return nil, "", err
		}
		maxID = ""
		if list.NextMaxID != nil {
			maxID = *list.NextMaxID
		}
		users := make([]*User, len(list.Users))
		for i, g := range list.Users {
			users[i] = &User{
				ID: g.ID, Username: g.Username, FullName: g.FullName, IsPrivate: g.IsPrivate,
				IsVerified: g.IsVerified, ProfilePicID: g.ProfilePicID, ProfilePicURL: g.ProfilePicURL,
				LatestReelMedia: g.LatestReelMedia, Friendship: g.Friendship,
			}
		}
		return insta.usersFrom(users, entry), cursorIf(list.HasMore, maxID), nil
	})
}

type showManyRequest struct {
	UUID              string `json:"_uuid"`
	IncludeFollowedBy string `json:"include_followed_by"`
	UserIDs           string `json:"user_ids"`
}

// Friendships returns the friendship status for each user id (friendships/show_many).
func (insta *Instagram) Friendships(ctx context.Context, ids ...int64) (map[string]Friendship, error) {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = Int64(id).String()
	}
	var resp struct {
		Statuses map[string]Friendship `json:"friendship_statuses"`
	}
	err := insta.callJSON(ctx, &reqOptions{Endpoint: urlFriendshipShowMany, IsPost: true, Form: showManyRequest{UUID: insta.uuid, IncludeFollowedBy: "true", UserIDs: strings.Join(s, ", ")}}, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Statuses, nil
}
