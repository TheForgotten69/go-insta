package goinsta

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestRelationshipRequests(t *testing.T) {
	insta, _ := newReplay(t)
	calls := capture(t, insta, answer(`{"status":"ok","friendship_status":{"blocking":true,"following":false}}`))
	last := func() sent { return (*calls)[len(*calls)-1] }
	ctx := t.Context()
	u := insta.NewUser(77200259753)

	for _, tc := range []struct {
		name, path string
		call       func(context.Context) error
		want       map[string]any
	}{
		{"Block", "/api/v1/friendships/block/77200259753/", u.Block,
			map[string]any{"user_id": "77200259753", "surface": "profile", "container_module": "profile",
				"is_auto_block_enabled": "false", "_uuid": insta.uuid}},
		{"Unblock", "/api/v1/friendships/unblock/77200259753/", u.Unblock,
			map[string]any{"user_id": "77200259753", "container_module": "profile"}},
		{"Mute posts", "/api/v1/friendships/mute_posts_or_story_from_follow/",
			func(ctx context.Context) error { return u.Mute(ctx, true, false) },
			map[string]any{"target_posts_author_id": "77200259753", "container_module": "profile"}},
		{"Unmute both", "/api/v1/friendships/unmute_posts_or_story_from_follow/",
			func(ctx context.Context) error { return u.Unmute(ctx, true, true) },
			map[string]any{"target_posts_author_id": "77200259753", "target_reel_author_id": "77200259753"}},
		{"RemoveFollower", "/api/v1/friendships/remove_follower/77200259753/", u.RemoveFollower,
			map[string]any{"user_id": "77200259753", "container_module": "self_followers"}},
	} {
		if err := tc.call(ctx); err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		got := last()
		if got.req.Method != "POST" || got.req.URL.Path != tc.path {
			t.Errorf("%s: %s %s, want POST %s", tc.name, got.req.Method, got.req.URL.Path, tc.path)
		}
		for k, want := range tc.want {
			if got.payload[k] != want {
				t.Errorf("%s: %s = %v, want %v", tc.name, k, got.payload[k], want)
			}
		}
	}
	if !u.Friendship.Blocking {
		t.Error("friendship_status not stored")
	}
	if err := u.Mute(ctx, false, false); err == nil {
		t.Error("Mute with nothing to mute accepted")
	}

	if err := insta.Account.UpdateCloseFriends(ctx, []*User{u}, []*User{insta.NewUser(1)}); err != nil {
		t.Fatal(err)
	}
	got := last()
	add, _ := got.payload["add"].([]any)
	remove, _ := got.payload["remove"].([]any)
	if got.req.URL.Path != "/api/v1/friendships/set_besties/" || len(add) != 1 || add[0] != "77200259753" ||
		len(remove) != 1 || remove[0] != "1" || got.payload["block_on_empty_thread_creation"] != "false" {
		t.Errorf("set_besties = %s %v", got.req.URL.Path, got.payload)
	}
	if err := insta.Account.UpdateCloseFriends(ctx, nil, nil); err == nil {
		t.Error("empty close-friends update accepted")
	}
}

func TestBlockedUsersPages(t *testing.T) {
	insta, _ := newReplay(t)
	var cursors []string
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		max := req.URL.Query().Get("max_id")
		cursors = append(cursors, max)
		if req.Method != "GET" || req.URL.Path != "/api/v1/users/blocked_list/" {
			t.Errorf("%s %s", req.Method, req.URL.Path)
		}
		if max == "" {
			return 200, `{"blocked_list":[{"user_id":1,"username":"a"}],"next_max_id":"p2","status":"ok"}`
		}
		return 200, `{"blocked_list":[{"user_id":"2","username":"b","block_at":1793400000}],"status":"ok"}`
	})})
	got, err := insta.Account.BlockedUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].UserID != 2 || got[1].BlockedAt != 1793400000 || strings.Join(cursors, ",") != ",p2" {
		t.Errorf("blocked = %+v, cursors %q", got, cursors)
	}
}

func TestFollowRequestGraphQL(t *testing.T) {
	insta, _ := newReplay(t)
	calls := capture(t, insta, answer(`{"data":{"xdt_friendships_pending_users":{"users":{"edges":[{"node":{"pk":"5","username":"x"}}]}}}}`))
	last := func() sent { return (*calls)[len(*calls)-1] }
	ctx := t.Context()

	users, err := insta.Account.PendingFollowRequests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := last()
	if v := got.vars(); got.req.URL.Path != "/graphql/query" || got.payload["client_doc_id"] != docPendingFollows ||
		got.payload["fb_api_req_friendly_name"] != "PendingFollows" || v["page_size"] != 20.0 ||
		v["exclude_su_friendship_status"] != false {
		t.Errorf("pending: %s %v", got.req.URL.Path, got.payload)
	}
	if len(users) != 1 || users[0].Username != "x" || users[0].insta == nil {
		t.Errorf("pending users = %+v", users)
	}

	u := insta.NewUser(5)
	if err := u.ApproveFollowRequest(ctx); err != nil {
		t.Fatal(err)
	}
	got = last()
	if v := got.vars(); got.req.URL.Path != "/graphql_www" || got.payload["client_doc_id"] != docApproveFollowRequest ||
		v["user_id"] != "5" || v["radio_type"] != "ethernet-none" {
		t.Errorf("approve: %s %v", got.req.URL.Path, got.payload)
	}
	if err := u.IgnoreFollowRequest(ctx); err != nil {
		t.Fatal(err)
	}
	got = last()
	if v := got.vars(); got.req.URL.Path != "/graphql/query" || got.payload["client_doc_id"] != docIgnoreFollowRequest || v["user_id"] != "5" {
		t.Errorf("ignore: %s %v", got.req.URL.Path, got.payload)
	}
}
