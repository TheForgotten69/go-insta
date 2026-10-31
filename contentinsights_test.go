package goinsta

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestReelAndLongformInsights(t *testing.T) {
	insta, _ := newReplay(t)
	got := capture(t, insta, func(s sent) (int, string) {
		if s.payload["fb_api_req_friendly_name"] == friendlyReelInsights {
			return 200, `{"data":{"__typename":"Query","ig_reel_insights":{"likes":3,"total_views":40,"ig_plays":41}}}`
		}
		return 200, `{"data":{"__typename":"Query","longform_insights_info":{"plays":5}}}`
	})
	item := testItem(insta, "4019685539559466739_42865264907")

	in, err := item.ReelInsights(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	r := (*got)[0]
	if r.req.URL.Path != "/graphql_www" || r.payload["client_doc_id"] != docReelInsights {
		t.Errorf("reel insights request: %s %v", r.req.URL.Path, r.payload["client_doc_id"])
	}
	if v := r.vars(); v["mediaIgid"] != "4019685539559466739" || v["bloksVersioningId"] != insta.sess.BloksVersionID {
		t.Errorf("reel insights vars = %v", v)
	}
	if in.Likes != 3 || in.TotalViews != 40 || !strings.Contains(string(in.Raw), "ig_plays") {
		t.Errorf("reel insights = %+v", in)
	}

	raw, err := item.LongformInsights(t.Context())
	if err != nil || string(raw) != `{"plays":5}` {
		t.Errorf("longform = %s, %v", raw, err)
	}
	if v := (*got)[1].vars(); !reflect.DeepEqual(v, map[string]any{"mediaIgid": "4019685539559466739"}) {
		t.Errorf("longform vars = %v", v)
	}
}

func TestMediaInsightsGrid(t *testing.T) {
	insta, _ := newReplay(t)
	got := capture(t, insta, answer(`{"data":{"xig_media_grid":{"edges":[]}}}`))
	if _, err := insta.Account.MediaInsightsGrid(t.Context(), MediaProductStory, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := insta.Account.MediaInsightsGrid(t.Context(), MediaProductReels, "C1"); err != nil {
		t.Fatal(err)
	}
	story, reels := (*got)[0].vars(), (*got)[1].vars()
	qp, _ := story["query_params"].(map[string]any)
	if qp["id"] != "42865264907" || qp["access_token"] != "" || story["timeframe"] != "ONE_DAY" ||
		story["media_product_type"] != "STORY" || story["count"] != 15.0 || story["should_include_insights_data"] != true {
		t.Errorf("story grid vars = %v", story)
	}
	if _, ok := story["cursor"]; ok {
		t.Error("first page sent a cursor")
	}
	if reels["timeframe"] != "LIFETIME" || reels["media_product_type"] != "CLIPS" || reels["cursor"] != "C1" {
		t.Errorf("reels grid vars = %v", reels)
	}

	insta2, _ := newReplay(t)
	capture(t, insta2, answer(`{"data":{"__typename":"Query"}}`))
	if _, err := insta2.Account.MediaInsightsGrid(t.Context(), MediaProductFeed, ""); !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("empty grid err = %v", err)
	}
}

func TestCommentInsightsOptIn(t *testing.T) {
	insta, _ := newReplay(t)
	got := capture(t, insta, func(s sent) (int, string) {
		if s.payload["fb_api_req_friendly_name"] == friendlyCommentInsightsOptIn {
			return 200, `{"data":{"xdt_get_comment_summary_opt_in_status_by_user_id":{"user":{"comment_summary_opt_in_status":true}}}}`
		}
		return 200, `{"data":{"xdt_update_comment_summary_opt_in_by_user_id":{"success":true}}}`
	})
	on, err := insta.Account.CommentInsightsOptIn(t.Context())
	if err != nil || !on {
		t.Errorf("opt-in = %v, %v", on, err)
	}
	if err := insta.Account.SetCommentInsightsOptIn(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	get, set := (*got)[0], (*got)[1]
	if get.req.URL.Path != "/graphql/query" || get.payload["client_doc_id"] != docCommentInsightsOptIn {
		t.Errorf("opt-in read: %s %v", get.req.URL.Path, get.payload["client_doc_id"])
	}
	if rd, _ := get.vars()["request_data"].(map[string]any); rd["user_id"] != "42865264907" {
		t.Errorf("opt-in read vars = %v", get.vars())
	}
	if rd, _ := set.vars()["request_data"].(map[string]any); set.req.URL.Path != "/graphql/query" || rd["opted_in"] != false {
		t.Errorf("opt-in write: %s %v", set.req.URL.Path, set.vars())
	}
}

func TestStoryViewers(t *testing.T) {
	insta, _ := newReplay(t)
	pages := []string{
		`{"users":[{"pk":1,"username":"a"}],"next_max_id":"M2","user_count":2,"status":"ok"}`,
		`{"users":[{"pk":2,"username":"b"}],"next_max_id":null,"status":"ok"}`,
	}
	var n int
	got := capture(t, insta, func(sent) (int, string) {
		n++
		return 200, pages[n-1]
	})
	viewers, err := testItem(insta, "4019000000000000001_42865264907").StoryViewers().Collect(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(viewers) != 2 || viewers[1].Username != "b" || viewers[0].insta == nil {
		t.Errorf("viewers = %+v", viewers)
	}
	first, second := (*got)[0], (*got)[1]
	if first.req.Method != "GET" || first.req.URL.Path != "/api/v1/media/4019000000000000001/list_reel_media_viewer/" {
		t.Errorf("viewers request: %s %s", first.req.Method, first.req.URL.Path)
	}
	q := first.req.URL.Query()
	if q.Get("reel_id") != "42865264907" || q.Get("reel_viewer_source") != "unknown" ||
		q.Get("story_has_interactive_stickers") != "false" || q.Has("max_id") {
		t.Errorf("viewers page 1 query = %v", q)
	}
	if second.req.URL.Query().Get("max_id") != "M2" {
		t.Errorf("viewers page 2 query = %v", second.req.URL.Query())
	}
}
