package goinsta

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDirectInbox(t *testing.T) {
	insta, r := newReplay(t, "direct_inbox")
	in := insta.Inbox()
	if err := in.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	f := fixture(t, "direct_inbox")
	checkRequest(t, f, r.sent[0])
	q := r.sent[0].req.URL.Query()
	for _, k := range []string{"fetch_reason", "include_old_mrs", "limit", "thread_message_limit"} {
		if q.Get(k) != f.Query[k] {
			t.Errorf("%s = %q, want %q", k, q.Get(k), f.Query[k])
		}
	}
	if len(in.Items) != 0 || in.More() {
		t.Errorf("empty inbox: %d threads, more=%v", len(in.Items), in.More())
	}
}

func TestDirectRequests(t *testing.T) {
	insta, _ := newReplay(t)
	var reqs []*http.Request
	var forms []url.Values
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		reqs = append(reqs, req)
		form := url.Values{}
		if req.Body != nil {
			form = sentForm(t, req)
		}
		forms = append(forms, form)
		switch {
		case strings.HasSuffix(req.URL.Path, "/direct_v2/inbox/") && req.URL.Query().Get("fetch_reason") == "initial_snapshot":
			return 200, `{"inbox":{"threads":[{"thread_id":"340282366841710300949128","thread_title":"bob",` +
				`"users":[{"pk":7,"username":"bob"}],"items":[{"item_id":"1","item_type":"text","text":"hi","user_id":7,"timestamp":1793000000000000}]}],` +
				`"unseen_count":1,"has_older":true,"prev_cursor_v2":{"thread_fbid":111,"relevancy_score":0,"timestamp_ms":222}},"status":"ok"}`
		case strings.HasSuffix(req.URL.Path, "/direct_v2/inbox/"):
			return 200, `{"inbox":{"threads":[],"has_older":false,"prev_cursor_v2":{"thread_fbid":0}},"status":"ok"}`
		case strings.Contains(req.URL.Path, "/direct_v2/threads/340282366841710300949128/") && req.Method == "GET":
			if req.URL.Query().Get("cursor") == "" {
				return 200, `{"thread":{"thread_id":"340282366841710300949128","items":[` +
					`{"item_id":"3","item_type":"media_share","media_share":{"id":"9_7","code":"C"}},` +
					`{"item_id":"2","item_type":"xma_share","xma_share":[{"target_url":"x"}]}],` +
					`"oldest_cursor":"OLD","has_older":true},"status":"ok"}`
			}
			return 200, `{"thread":{"thread_id":"340282366841710300949128","items":[{"item_id":"1","item_type":"text","text":"hi"}],"has_older":false},"status":"ok"}`
		default:
			return 200, `{"action":"item_ack","payload":{"item_id":"4","thread_id":"340282366841710300949128","client_context":"x","timestamp":"1"},"status":"ok"}`
		}
	})})
	ctx := t.Context()

	in := insta.Inbox()
	threads, err := in.Collect(ctx, 0)
	if err != nil || len(threads) != 1 || in.UnseenCount != 1 {
		t.Fatalf("inbox: %v, %d threads, unseen %d", err, len(threads), in.UnseenCount)
	}
	th := threads[0]
	if th.Title != "bob" || th.Users[0].insta == nil || th.Items[0].Text != "hi" || th.Items[0].Timestamp != 1793000000000000 {
		t.Errorf("thread parsed: %+v %+v", th, th.Items[0])
	}
	if q := reqs[1].URL.Query(); q.Get("fetch_reason") != "page_scroll" || q.Get("cursor_thread_v2_id") != "111" ||
		q.Get("cursor_timestamp_ms") != "222" || q.Get("cursor_relevancy_score") != "0" {
		t.Errorf("inbox page 2 query = %v", q)
	}

	msgs, err := th.Messages().Collect(ctx, 0)
	if err != nil || len(msgs) != 3 {
		t.Fatalf("messages: %v, %d", err, len(msgs))
	}
	if msgs[0].MediaShare == nil || msgs[0].MediaShare.insta == nil || !strings.Contains(string(msgs[1].Raw), "xma_share") {
		t.Errorf("message kinds: media_share %v, raw %s", msgs[0].MediaShare, msgs[1].Raw)
	}
	if q := reqs[3].URL.Query(); q.Get("cursor") != "OLD" || q.Get("direction") != "older" || q.Get("visual_message_return_type") != "unseen" {
		t.Errorf("thread page 2 query = %v", q)
	}

	sent, err := th.SendText(ctx, "hello")
	if err != nil || sent.ItemID != "4" {
		t.Fatalf("SendText: %v %+v", err, sent)
	}
	f := forms[len(forms)-1]
	if p := reqs[len(reqs)-1].URL.Path; p != "/api/v1/direct_v2/threads/broadcast/text/" {
		t.Errorf("send path = %s", p)
	}
	otid := f.Get("client_context")
	for k, want := range map[string]string{
		"action": "send_item", "text": "hello", "thread_ids": "[340282366841710300949128]",
		"offline_threading_id": otid, "mutation_token": otid, "_uuid": insta.uuid, "device_id": insta.dID,
	} {
		if f.Get(k) != want {
			t.Errorf("send %s = %q, want %q", k, f.Get(k), want)
		}
	}
	if f.Has("recipient_users") || f.Has("signed_body") {
		t.Errorf("send to a thread sent %v", f)
	}

	if _, err := insta.SendDirectText(ctx, "yo", 7, 8); err != nil {
		t.Fatal(err)
	}
	if f := forms[len(forms)-1]; f.Get("recipient_users") != "[[7,8]]" || f.Has("thread_ids") {
		t.Errorf("send to users: %v", f)
	}

	if err := th.MarkSeen(ctx, msgs[0]); err != nil {
		t.Fatal(err)
	}
	if p := reqs[len(reqs)-1].URL.Path; p != "/api/v1/direct_v2/threads/340282366841710300949128/items/3/seen/" {
		t.Errorf("seen path = %s", p)
	}
	if f := forms[len(forms)-1]; f.Get("action") != "mark_seen" || f.Get("thread_id") != th.ThreadID {
		t.Errorf("seen form = %v", f)
	}
}

func TestOfflineThreadingID(t *testing.T) {
	before := time.Now().UnixMilli()
	n, err := strconv.ParseInt(offlineThreadingID(), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if ms := n >> 22; ms < before || ms > time.Now().UnixMilli() {
		t.Errorf("otid time bits = %d, want about %d", ms, before)
	}
}
