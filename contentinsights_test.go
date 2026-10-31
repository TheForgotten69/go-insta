package goinsta

import (
	"errors"
	"reflect"
	"strconv"
	"testing"
)

// The response is the live LongformInsightsQuery answer for a fresh Reel (2026-10-31).
const longformInsightsBody = `{"data":{"__typename":"Query","longform_insights_info":{"aggregated_follows":{"benchmark":null,"format":"COUNT","formatted_value":"0","id":"longform_aggregated_follows","label":"Follows","value":null},"aggregated_total_views":{"benchmark":null,"format":"COUNT","formatted_value":"0","id":"longform_aggregated_unified_total_views","label":"Total views","value":0},"avg_watch_time":{"format":"DURATION_SECONDS","formatted_value":"--","id":"avg_watch_time","label":"Average watch time","value":null},"comments":0,"demographics":null,"engagement_comments":0,"engagement_likes":0,"engagement_reposts":0,"engagement_saves":0,"engagement_shares":null,"followers_views":0,"follows":null,"likes":0,"linked_reels":[],"media_id":"17966887134201144","non_followers_views":0,"performance_factors":[{"benchmark":null,"format":"PERCENTAGE","formatted_value":"0.0%","id":"reels_skip_rate","label":"Skip rate","value":0},{"benchmark":null,"format":"PERCENTAGE","formatted_value":"0.0%","id":"reshares_rate","label":"Share rate","value":0}],"reposts":0,"saves":0,"shares":null,"thumbnail_url":"https://example/thumb.jpg","total_nullable_views":{"format":"COUNT","formatted_value":"0","id":"views","label":"Views","value":0},"viewers_or_reach":{"format":"COUNT","formatted_value":"--","id":"viewers","label":"Viewers","value":null},"views_over_time":{"all":{"benchmark":[],"current":[],"middle":[]},"followers":{"benchmark":[],"current":[],"middle":[]},"media_created_at":1793411691,"non_followers":{"benchmark":[],"current":[],"middle":[]}}}}}`

func TestReelInsights(t *testing.T) {
	insta, _ := newReplay(t)
	got := capture(t, insta, answer(longformInsightsBody))
	in, err := testItem(insta, "4019685539559466739_42865264907").ReelInsights(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	r := (*got)[0]
	if r.req.URL.Path != "/graphql_www" || r.payload["client_doc_id"] != docLongformInsights {
		t.Errorf("request: %s %v", r.req.URL.Path, r.payload["client_doc_id"])
	}
	if v := r.vars(); !reflect.DeepEqual(v, map[string]any{"mediaIgid": "4019685539559466739"}) {
		t.Errorf("vars = %v", v)
	}
	if in.AvgWatchTime.Label != "Average watch time" || in.AvgWatchTime.Format != "DURATION_SECONDS" ||
		in.Views.Value == nil || len(in.PerformanceFactors) != 2 || in.PerformanceFactors[0].ID != "reels_skip_rate" {
		t.Errorf("insights = %+v", in)
	}
}

func TestMediaInsightsGrid(t *testing.T) {
	insta, _ := newReplay(t)
	page := func(node, cursor string, more bool) string {
		return `{"data":{"__typename":"Query","xig_user":{"__typename":"XFBInstagramUserV2","business_manager":{"medias":{"edges":[{"node":` +
			node + `}],"page_info":{"has_next_page":` + strconv.FormatBool(more) + `,"end_cursor":"` + cursor + `"}}}}}}`
	}
	got := capture(t, insta, func(s sent) (int, string) {
		if _, ok := s.vars()["cursor"]; ok {
			return 200, page(`{"id":"2"}`, "", false)
		}
		return 200, page(`{"id":"1"}`, "C1", true)
	})
	items, err := insta.Account.MediaInsightsGrid(MediaProductStory).Collect(t.Context(), 0)
	if err != nil || len(items) != 2 || string(items[1]) != `{"id":"2"}` {
		t.Fatalf("items = %s, %v", items, err)
	}
	first, second := (*got)[0].vars(), (*got)[1].vars()
	qp, _ := first["query_params"].(map[string]any)
	if qp["id"] != "42865264907" || qp["access_token"] != "" || first["timeframe"] != "ONE_DAY" ||
		first["media_product_type"] != "STORY" || first["count"] != 15.0 || first["should_include_insights_data"] != true {
		t.Errorf("first page vars = %v", first)
	}
	if _, ok := first["cursor"]; ok || second["cursor"] != "C1" {
		t.Errorf("cursors: first %v, second %v", first["cursor"], second["cursor"])
	}

	reels := insta.Account.MediaInsightsGrid(MediaProductReels)
	_ = reels.Next(t.Context())
	if v := (*got)[2].vars(); v["timeframe"] != "LIFETIME" || v["media_product_type"] != "CLIPS" {
		t.Errorf("reels vars = %v", v)
	}

	insta2, _ := newReplay(t)
	capture(t, insta2, answer(`{"data":{"__typename":"Query"}}`))
	if err := insta2.Account.MediaInsightsGrid(MediaProductFeed).Next(t.Context()); !errors.Is(err, ErrEmptyResponse) {
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

func TestGraphQLTopLevelError(t *testing.T) {
	insta, _ := newReplay(t)
	capture(t, insta, answer(`{"errors":[{"message":"A server error missing_required_variable_value occured.","code":1675012,`+
		`"summary":"Your Request Couldn't be Processed"}],"extensions":{"is_final":true}}`))
	_, err := testItem(insta, "1_2").ReelInsights(t.Context())
	var ge *GraphQLError
	if !errors.As(err, &ge) || ge.Code != 1675012 {
		t.Errorf("err = %v, want GraphQLError 1675012", err)
	}
}
