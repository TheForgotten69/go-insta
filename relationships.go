package goinsta

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	var resp friendResp
	if err := user.insta.callJSON(ctx, &reqOptions{Endpoint: endpoint, IsPost: true, Signed: payload}, &resp); err != nil {
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

// BlockedUsers returns every account the account has blocked.
func (account *Account) BlockedUsers(ctx context.Context) ([]BlockedUser, error) {
	var cursor string
	users, err := newPager(func(ctx context.Context) ([]BlockedUser, string, error) {
		var page struct {
			BlockedList []BlockedUser `json:"blocked_list"`
			NextMaxID   string        `json:"next_max_id"`
		}
		if err := account.insta.callJSON(ctx, &reqOptions{Endpoint: urlBlockedList, Form: maxIDQuery{MaxID: cursor}}, &page); err != nil {
			return nil, "", err
		}
		cursor = page.NextMaxID
		return page.BlockedList, cursor, nil
	}).Collect(ctx, 0)
	if err != nil {
		return nil, err
	}
	return users, nil
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
	_, err := account.insta.call(ctx, &reqOptions{Endpoint: urlSetBesties, IsPost: true, Signed: setBestiesRequest{
		sessionIDs:                 account.insta.sessionIDs(),
		Source:                     "audience_dialog",
		Module:                     "audience_selection",
		BlockOnEmptyThreadCreation: "false",
		Add:                        userIDs(add),
		Remove:                     userIDs(remove),
	}})
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
	var pending struct {
		Users connection[*User] `json:"users"`
	}
	err := account.insta.graphQLRoot(ctx, gqlQuery{friendly: "PendingFollows", docID: docPendingFollows,
		root: "xdt_friendships_pending_users"}, pendingFollowsVars{PageSize: 20, ExcludeSuggestedUsers: true}, &pending)
	if err != nil {
		return nil, err
	}
	users := slices.DeleteFunc(pending.Users.nodes(), func(u *User) bool { return u == nil })
	return account.insta.usersFrom(users, entryContext{}), nil
}

type followRequestVars struct {
	UserID    string `json:"user_id"`
	RadioType string `json:"radio_type"`
}

// ApproveFollowRequest accepts the user's request to follow the account.
func (user *User) ApproveFollowRequest(ctx context.Context) error {
	return user.insta.graphQLAck(ctx, gqlQuery{url: graphQLWWWURL, friendly: "ApproveFollowRequest",
		docID: docApproveFollowRequest, root: "xdt_friendships_approve_user"}, user.followRequestVars())
}

// IgnoreFollowRequest declines the user's request to follow the account.
func (user *User) IgnoreFollowRequest(ctx context.Context) error {
	return user.insta.graphQLAck(ctx, gqlQuery{friendly: "IgnoreFollowRequest",
		docID: docIgnoreFollowRequest, root: "xdt_friendships_ignore_user_v2"}, user.followRequestVars())
}

func (user *User) followRequestVars() followRequestVars {
	return followRequestVars{UserID: user.ID.String(), RadioType: "ethernet-none"}
}
