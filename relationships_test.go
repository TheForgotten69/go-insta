package goinsta

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type relCall struct {
	method, host, path string
	query              url.Values
	form               url.Values
	signed             map[string]any
	vars               map[string]any
}

func relRecorder(t *testing.T, insta *Instagram, body string) *relCall {
	t.Helper()
	got := &relCall{}
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		*got = relCall{method: req.Method, host: req.URL.Host, path: req.URL.Path, query: req.URL.Query()}
		if req.Body != nil {
			raw, _ := io.ReadAll(req.Body)
			if req.Header.Get("Content-Encoding") == "gzip" {
				zr, err := gzip.NewReader(bytes.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				raw, _ = io.ReadAll(zr)
			}
			got.form, _ = url.ParseQuery(string(raw))
			if _, js, ok := strings.Cut(got.form.Get("signed_body"), "."); ok {
				_ = json.Unmarshal([]byte(js), &got.signed)
			}
			if v := got.form.Get("variables"); v != "" {
				_ = json.Unmarshal([]byte(v), &got.vars)
			}
		}
		return 200, body
	})})
	return got
}

func TestRelationshipRequests(t *testing.T) {
	insta, _ := newReplay(t)
	got := relRecorder(t, insta, `{"status":"ok","friendship_status":{"blocking":true,"following":false}}`)
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
		if got.method != "POST" || got.path != tc.path {
			t.Errorf("%s: %s %s, want POST %s", tc.name, got.method, got.path, tc.path)
		}
		for k, want := range tc.want {
			if got.signed[k] != want {
				t.Errorf("%s: %s = %v, want %v", tc.name, k, got.signed[k], want)
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
	add, _ := got.signed["add"].([]any)
	remove, _ := got.signed["remove"].([]any)
	if got.path != "/api/v1/friendships/set_besties/" || len(add) != 1 || add[0] != "77200259753" ||
		len(remove) != 1 || remove[0] != "1" || got.signed["block_on_empty_thread_creation"] != "false" {
		t.Errorf("set_besties = %s %v", got.path, got.signed)
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
	got := relRecorder(t, insta, `{"data":{"xdt_friendships_pending_users":{"users":{"edges":[{"node":{"pk":"5","username":"x"}}]}}}}`)
	ctx := t.Context()

	users, err := insta.Account.PendingFollowRequests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/graphql/query" || got.form.Get("client_doc_id") != docPendingFollows ||
		got.form.Get("fb_api_req_friendly_name") != "PendingFollows" || got.vars["page_size"] != 20.0 ||
		got.vars["exclude_su_friendship_status"] != false {
		t.Errorf("pending: %s %v %v", got.path, got.form, got.vars)
	}
	if len(users) != 1 || users[0].Username != "x" || users[0].insta == nil {
		t.Errorf("pending users = %+v", users)
	}
	if list, err := decodeUserConnection(json.RawMessage(`[{"pk":"6","username":"y"}]`)); err != nil || len(list) != 1 {
		t.Errorf("plain list: %v %v", list, err)
	}

	u := insta.NewUser(5)
	if err := u.ApproveFollowRequest(ctx); err != nil {
		t.Fatal(err)
	}
	if got.path != "/graphql_www" || got.form.Get("client_doc_id") != docApproveFollowRequest ||
		got.vars["user_id"] != "5" || got.vars["radio_type"] != "ethernet-none" {
		t.Errorf("approve: %s %v %v", got.path, got.form, got.vars)
	}
	if err := u.IgnoreFollowRequest(ctx); err != nil {
		t.Fatal(err)
	}
	if got.path != "/graphql/query" || got.form.Get("client_doc_id") != docIgnoreFollowRequest || got.vars["user_id"] != "5" {
		t.Errorf("ignore: %s %v %v", got.path, got.form, got.vars)
	}
}
