package goinsta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	urlBlock          = "friendships/block/%d/"
	urlUnblock        = "friendships/unblock/%d/"
	urlBlockedList    = "users/blocked_list/"
	urlSetBesties     = "friendships/set_besties/"
	urlMuteFollow     = "friendships/mute_posts_or_story_from_follow/"
	urlUnmuteFollow   = "friendships/unmute_posts_or_story_from_follow/"
	urlRemoveFollower = "friendships/remove_follower/%d/"

	// client_doc_ids from assets/ig4a-instagram-schema_client_persist_ids.json; the
	// "instagram" schema goes to graphql_www, "distillery" to graphql/query.
	docPendingFollows       = "33138429313628334965326831157"
	docApproveFollowRequest = "356404437015335956218183369113"
	docIgnoreFollowRequest  = "1956506636707216159350023433"
)

type blockRequest struct {
	sessionIDs
	UserID             string `json:"user_id"`
	Surface            string `json:"surface"`
	ContainerModule    string `json:"container_module"`
	IsAutoBlockEnabled string `json:"is_auto_block_enabled"`
}

type unblockRequest struct {
	sessionIDs
	UserID          string `json:"user_id"`
	ContainerModule string `json:"container_module"`
}

// Block blocks the user and updates user.Friendship.
func (user *User) Block(ctx context.Context) error {
	return user.postFriendship(ctx, fmt.Sprintf(urlBlock, user.ID), blockRequest{
		sessionIDs:         user.insta.sessionIDs(),
		UserID:             user.ID.String(),
		Surface:            "profile",
		ContainerModule:    "profile",
		IsAutoBlockEnabled: "false",
	})
}

func (user *User) Unblock(ctx context.Context) error {
	return user.postFriendship(ctx, fmt.Sprintf(urlUnblock, user.ID), unblockRequest{
		sessionIDs:      user.insta.sessionIDs(),
		UserID:          user.ID.String(),
		ContainerModule: "profile",
	})
}

// postFriendship posts a signed body and stores the returned friendship_status.
func (user *User) postFriendship(ctx context.Context, endpoint string, payload any) error {
	q, err := signedBody(payload)
	if err != nil {
		return err
	}
	var resp friendResp
	if err := user.insta.callJSON(ctx, &reqOptions{Endpoint: endpoint, IsPost: true, Query: q}, &resp); err != nil {
		return err
	}
	user.Friendship = resp.Friendship
	return nil
}

// BlockedUser is one entry of the account's blocked list.
type BlockedUser struct {
	UserID        Int64  `json:"user_id"`
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	ProfilePicURL string `json:"profile_pic_url"`
	BlockedAt     Int64  `json:"block_at"`
}

type maxIDForm struct {
	MaxID string `json:"max_id,omitempty"`
}

// maxBlockedPages bounds the blocked-list walk if the server keeps handing out cursors.
const maxBlockedPages = 100

// BlockedUsers returns every account the account has blocked.
func (account *Account) BlockedUsers(ctx context.Context) ([]BlockedUser, error) {
	var all []BlockedUser
	cursor := ""
	for range maxBlockedPages {
		q, err := formOf(maxIDForm{MaxID: cursor})
		if err != nil {
			return nil, err
		}
		var page struct {
			BlockedList []BlockedUser `json:"blocked_list"`
			NextMaxID   string        `json:"next_max_id"`
		}
		if err := account.insta.callJSON(ctx, &reqOptions{Endpoint: urlBlockedList, Query: q}, &page); err != nil {
			return nil, err
		}
		all = append(all, page.BlockedList...)
		if page.NextMaxID == "" || page.NextMaxID == cursor {
			return all, nil
		}
		cursor = page.NextMaxID
	}
	return nil, fmt.Errorf("blocked list: still paging after %d pages", maxBlockedPages)
}

type setBestiesRequest struct {
	sessionIDs
	Source                     string   `json:"source"`
	Module                     string   `json:"module"`
	BlockOnEmptyThreadCreation string   `json:"block_on_empty_thread_creation"`
	Add                        []string `json:"add"`
	Remove                     []string `json:"remove"`
}

// UpdateCloseFriends adds and removes users on the account's close-friends list.
func (account *Account) UpdateCloseFriends(ctx context.Context, add, remove []*User) error {
	if len(add) == 0 && len(remove) == 0 {
		return errors.New("close friends: nothing to add or remove")
	}
	q, err := signedBody(setBestiesRequest{
		sessionIDs:                 account.insta.sessionIDs(),
		Source:                     "audience_dialog",
		Module:                     "audience_selection",
		BlockOnEmptyThreadCreation: "false",
		Add:                        userIDs(add),
		Remove:                     userIDs(remove),
	})
	if err != nil {
		return err
	}
	_, err = account.insta.call(ctx, &reqOptions{Endpoint: urlSetBesties, IsPost: true, Query: q})
	return err
}

func userIDs(users []*User) []string {
	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.ID.String()
	}
	return ids
}

type muteFollowRequest struct {
	sessionIDs
	ContainerModule     string `json:"container_module"`
	TargetPostsAuthorID string `json:"target_posts_author_id,omitempty"`
	TargetReelAuthorID  string `json:"target_reel_author_id,omitempty"`
}

// Mute hides a followed account's posts and/or stories from the feed and tray.
func (user *User) Mute(ctx context.Context, posts, stories bool) error {
	return user.setMuted(ctx, urlMuteFollow, posts, stories)
}

func (user *User) Unmute(ctx context.Context, posts, stories bool) error {
	return user.setMuted(ctx, urlUnmuteFollow, posts, stories)
}

func (user *User) setMuted(ctx context.Context, endpoint string, posts, stories bool) error {
	if !posts && !stories {
		return errors.New("mute: choose posts, stories or both")
	}
	req := muteFollowRequest{sessionIDs: user.insta.sessionIDs(), ContainerModule: "profile"}
	if posts {
		req.TargetPostsAuthorID = user.ID.String()
	}
	if stories {
		req.TargetReelAuthorID = user.ID.String()
	}
	return user.postFriendship(ctx, endpoint, req)
}

type removeFollowerRequest struct {
	sessionIDs
	UserID          string `json:"user_id"`
	RadioType       string `json:"radio_type"`
	ContainerModule string `json:"container_module"`
}

// RemoveFollower makes the user stop following the account.
func (user *User) RemoveFollower(ctx context.Context) error {
	return user.postFriendship(ctx, fmt.Sprintf(urlRemoveFollower, user.ID), removeFollowerRequest{
		sessionIDs:      user.insta.sessionIDs(),
		UserID:          user.ID.String(),
		RadioType:       "ethernet-none",
		ContainerModule: "self_followers",
	})
}

type pendingFollowsVars struct {
	PageSize                     int  `json:"page_size"`
	DeferSuggestedUsers          bool `json:"defer_suggested_users"`
	ExcludeSuggestedUsers        bool `json:"exclude_suggested_users"`
	IncludeRecentAutoRfFollowers bool `json:"include_recent_auto_rf_followers"`
	ExcludeSuFriendshipStatus    bool `json:"exclude_su_friendship_status"`
	IncludeFacepiles             bool `json:"include_facepiles"`
}

// PendingFollowRequests returns the users waiting for the (private) account to
// accept their follow request; first page only, as the app's request screen.
func (account *Account) PendingFollowRequests(ctx context.Context) ([]*User, error) {
	body, err := account.insta.graphQL(ctx, gqlQuery{friendly: "PendingFollows", docID: docPendingFollows,
		root: "xdt_friendships_pending_users"}, pendingFollowsVars{PageSize: 20, ExcludeSuggestedUsers: true})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Pending struct {
				Users json.RawMessage `json:"users"`
			} `json:"xdt_friendships_pending_users"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	users, err := decodeUserConnection(resp.Data.Pending.Users)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		u.insta = account.insta
	}
	return users, nil
}

// decodeUserConnection reads either a plain user list or a connection with
// edges[].node; the decompiled query names a "users" connection.
func decodeUserConnection(raw json.RawMessage) ([]*User, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var users []*User
	if json.Unmarshal(raw, &users) == nil {
		return users, nil
	}
	var conn struct {
		Edges []struct {
			Node *User `json:"node"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(raw, &conn); err != nil {
		return nil, fmt.Errorf("pending users: %w", err)
	}
	for _, e := range conn.Edges {
		if e.Node != nil {
			users = append(users, e.Node)
		}
	}
	return users, nil
}

type followRequestVars struct {
	UserID    string `json:"user_id"`
	RadioType string `json:"radio_type"`
}

// ApproveFollowRequest accepts the user's request to follow the account.
func (user *User) ApproveFollowRequest(ctx context.Context) error {
	_, err := user.insta.graphQL(ctx, gqlQuery{url: graphQLWWWURL, friendly: "ApproveFollowRequest",
		docID: docApproveFollowRequest, root: "xdt_friendships_approve_user"}, user.followRequestVars())
	return err
}

// IgnoreFollowRequest declines the user's request to follow the account.
func (user *User) IgnoreFollowRequest(ctx context.Context) error {
	_, err := user.insta.graphQL(ctx, gqlQuery{friendly: "IgnoreFollowRequest",
		docID: docIgnoreFollowRequest, root: "xdt_friendships_ignore_user_v2"}, user.followRequestVars())
	return err
}

func (user *User) followRequestVars() followRequestVars {
	return followRequestVars{UserID: user.ID.String(), RadioType: "ethernet-none"}
}
