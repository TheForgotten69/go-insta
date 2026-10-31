package goinsta

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Live responses on the Creator test account (2026-10-31), lists cut to two
// entries; the bio links bodies are made up, as that account has no links.
func accountInsightsBodies(t *testing.T) map[string]string {
	raw, err := os.ReadFile("testdata/accountinsights.json")
	if err != nil {
		t.Fatal(err)
	}
	var live map[string]json.RawMessage
	if err := json.Unmarshal(raw, &live); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{
		friendlyAccountInsightsBioLinks: `{"data":{"ig_account_insights":{"bio_links":{
			"taps":{"value":7,"benchmark":{"value":3.5,"comparison":"HIGHER","sentiment":"POSITIVE","display_string":"2x"}},
			"page_views":{"value":70},
			"links":{"links":{"edges":[{"node":{"id":"l1","title":"Site","display_url":"x.com","taps":{"value":7}}}],"page_info":{"has_next_page":false}}},
			"previous_links":{"has_links":true}}}}}`,
		friendlyAccountInsightsPrevBioLinks: `{"data":{"ig_account_insights":{"bio_links":{"previous_links":{"links":{
			"edges":[{"node":{"id":"old","title":"Old","taps":{"value":1}}}],"page_info":{"end_cursor":"p2","has_next_page":false}}}}}}}`,
	}
	for name, body := range live {
		bodies[name] = string(body)
	}
	return bodies
}

func TestAccountInsightsRequests(t *testing.T) {
	insta, _ := newReplay(t)
	bodies := accountInsightsBodies(t)
	calls := capture(t, insta, func(s sent) (int, string) {
		// One line: a multi-line body reads as an incremental (NDJSON) response.
		var b bytes.Buffer
		if err := json.Compact(&b, []byte(bodies[s.payload["fb_api_req_friendly_name"].(string)])); err != nil {
			t.Fatal(err)
		}
		return 200, b.String()
	})
	last := func() sent { return (*calls)[len(*calls)-1] }
	ctx := t.Context()
	acc := insta.Account
	bloks := insta.sess.BloksVersionID

	check := func(name, docID, root string, want map[string]any) {
		t.Helper()
		got := last()
		if got.req.URL.Path != "/graphql_www" || got.payload["client_doc_id"] != docID ||
			got.payload["fb_api_req_friendly_name"] != name || got.req.Header.Get("X-Root-Field-Name") != root {
			t.Errorf("%s: %s doc=%v friendly=%v root=%q", name, got.req.URL.Path, got.payload["client_doc_id"],
				got.payload["fb_api_req_friendly_name"], got.req.Header.Get("X-Root-Field-Name"))
		}
		v := got.vars()
		if len(v) != len(want) {
			t.Errorf("%s: vars %v, want keys of %v", name, v, want)
		}
		for k, w := range want {
			if gv, ok := v[k]; !ok || gv != w {
				t.Errorf("%s: %s = %#v (present %v), want %#v", name, k, gv, ok, w)
			}
		}
	}

	home, err := acc.InsightsHome(ctx, InsightsHomeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	check(friendlyAccountInsightsHome, docAccountInsightsHome, rootAccountInsights, map[string]any{
		"period": "THIS_MONTH", "startDate": nil, "endDate": nil, "includeTools": true, "includeMetaOneSubtitle": true,
		"includeNextStepsTips": true, "includeAIGuidance": true, "meta_one_pacing_key": metaOnePacingPro, "bloksVersioningId": bloks,
	})
	if home.Layout != "HERO_PLUS_TWO" || len(home.Tiles) != 2 || *home.Tiles[0].Metric.Value != 33 ||
		home.Tiles[0].Delta.Sentiment != "POSITIVE" || len(home.Tiles[0].TimeSeries.DataPoints) != 2 || home.Tiles[1].ID != "followers" ||
		len(home.Tools) != 2 || home.Tools[0].ID != "ACCOUNT_STATUS" || home.NextStepsBloks != `{"tree":1}` ||
		home.MetaOneEligibility != "INELIGIBLE" {
		t.Errorf("home = %+v", home)
	}
	if _, err := acc.InsightsHome(ctx, InsightsHomeOptions{Period: PeriodLast90Days, Creator: true}); err != nil {
		t.Fatal(err)
	}
	if v := last().vars(); v["period"] != "LAST_90_DAYS" || v["meta_one_pacing_key"] != metaOnePacingCreator {
		t.Errorf("creator home vars = %v", v)
	}

	start, end := time.Unix(1793000000, 0), time.Unix(1793400000, 0)
	content := acc.InsightsContent(ContentInsightsOptions{Period: CustomPeriod(start, end), Type: ContentReels, Metric: MetricShares})
	if err := content.Next(ctx); err != nil {
		t.Fatal(err)
	}
	contentVars := map[string]any{
		"period": "CUSTOM", "startDate": 1793000000.0, "endDate": 1793400000.0, "metric": "shares",
		"includeCarouselMediaInsightsRendering": false, "sortByCreationTime": false, "count": 24.0, "after": nil,
		"contentType": "reels", "sortDescending": true, "bloksVersioningId": bloks,
	}
	check(friendlyAccountInsightsContent, docAccountInsightsContent, rootAccountInsights, contentVars)
	it := content.Items
	if len(it) != 2 || it[0].Thumbnail.MediaID != "4017451613201790201" || it[0].Thumbnail.CreationTime != 1793137574 ||
		it[0].Thumbnail.MediaType != "post" || len(it[0].Metrics) != 6 || *it[0].Metrics[4].Value != 8 || it[0].Metrics[5].Value != nil ||
		content.More() {
		t.Errorf("content = %+v", it)
	}
	if err := acc.InsightsContent(ContentInsightsOptions{Ascending: true}).Next(ctx); err != nil {
		t.Fatal(err)
	}
	if v := last().vars(); v["metric"] != "views" || v["contentType"] != nil || v["sortDescending"] != false || v["startDate"] != nil {
		t.Errorf("default content vars = %v", v)
	}

	links, err := acc.InsightsBioLinks(ctx, PeriodThisWeek)
	if err != nil {
		t.Fatal(err)
	}
	check(friendlyAccountInsightsBioLinks, docAccountInsightsBioLinks, rootAccountInsights,
		map[string]any{"period": "THIS_WEEK", "startDate": nil, "endDate": nil})
	if *links.Taps.Value != 7 || links.Taps.Benchmark.DisplayString != "2x" || *links.PageViews.Value != 70 ||
		len(links.Links) != 1 || links.Links[0].DisplayURL != "x.com" || *links.Links[0].Taps.Value != 7 || !links.HasPreviousLinks {
		t.Errorf("bio links = %+v", links)
	}

	prev := acc.InsightsPreviousBioLinks(PeriodLast2Years)
	if err := prev.Next(ctx); err != nil {
		t.Fatal(err)
	}
	check(friendlyAccountInsightsPrevBioLinks, docAccountInsightsPreviousBioLinks, rootAccountInsights,
		map[string]any{"period": "LAST_2_YEARS", "startDate": nil, "endDate": nil, "first": 20.0, "after": nil})
	if len(prev.Items) != 1 || prev.Items[0].ID != "old" || prev.More() {
		t.Errorf("previous links = %+v more=%v", prev.Items, prev.More())
	}

	state, err := acc.InsightsViewerState(ctx, PeriodThisMonth, true)
	if err != nil {
		t.Fatal(err)
	}
	check(friendlyAccountInsightsViewerState, docAccountInsightsViewerState, rootAccountInsights,
		map[string]any{"period": "THIS_MONTH", "includePremiumInsights": true})
	if state.IsCXUser || state.CXInsightsEnabled || string(state.PremiumInsights) != "null" {
		t.Errorf("viewer state = %+v", state)
	}

	tree, err := acc.InsightsMediaComponent(ctx, "31")
	if err != nil {
		t.Fatal(err)
	}
	check(friendlyAccountInsightsMedia, docAccountInsightsMediaComponent, rootAccountInsights,
		map[string]any{"mediaId": "31", "bloksVersioningId": bloks})
	if tree != `{"tree":1}` {
		t.Errorf("media component = %q", tree)
	}

	over, err := acc.InsightsOverview(ctx, PeriodLast14Days)
	if err != nil {
		t.Fatal(err)
	}
	check(friendlyAccountInsightsOverview, docAccountInsightsOverview, rootAccountInsights, map[string]any{
		"period": "LAST_14_DAYS", "startDate": nil, "endDate": nil, "redesignEnabled": true, "bloksVersioningId": bloks})
	if *over.Views.Total.Value != 33 || over.Views.Total.Benchmark.Comparison != "HIGHER" || *over.Views.Share.Follower.Value != 0.152 ||
		over.ViewsByContentType.Total != 33 || over.ViewsByContentType.Entries[0].NonFollowerCount != 22 ||
		over.ViewsByContentType.Entries[0].FacebookCount != nil || *over.Viewers.Value != 10 || *over.Interactions.Total.Value != 21 ||
		*over.Interactions.NonFollower.Value != 1 || over.InteractionsByContentType.Total.Total != 21 ||
		over.InteractionsByContentType.ByType[0].Type != "likes" || over.InteractionsByContentType.ByType[0].Entries[0].Value != 4 ||
		*over.ProfileVisits.Value != 8 || over.LinkPageViews.Value != nil || len(over.ViewsOverTime.DataPoints) != 2 {
		t.Errorf("overview = %+v", over)
	}

	aud, err := acc.InsightsAudience(ctx, CustomPeriod(start, end))
	if err != nil {
		t.Fatal(err)
	}
	check(friendlyAccountInsightsAudience, docAccountInsightsAudience, rootAccountInsights, map[string]any{
		"period": "CUSTOM", "startDate": 1793000000.0, "endDate": 1793400000.0, "redesignEnabled": true,
		"useFollowerSnapshot": true, "topLanguagesEnabled": true, "bloksVersioningId": bloks})
	if *aud.Followers.Value != 3 || aud.FollowersDelta.Value != "--" || len(aud.ActiveTimes.Slots) != 2 || aud.ActiveTimes.Slots[1].Hour != 3 ||
		len(aud.Age.Entries) != 2 || aud.Age.Entries[0].AgeRange != "13-17" || len(aud.TopCities) != 2 || aud.TopCountries[0].Name != "--" ||
		len(aud.ActiveFollowersOverTime.DataPoints) != 2 {
		t.Errorf("audience = %+v", aud)
	}
	if _, err := acc.InsightsAudience(ctx, PeriodThisWeek); err != nil {
		t.Fatal(err)
	}
	if v := last().vars(); v["useFollowerSnapshot"] != false {
		t.Errorf("audience vars = %v", v)
	}

	for _, c := range *calls {
		if c.req.Header.Get("Content-Encoding") == "gzip" {
			t.Errorf("%v sent gzipped", c.payload["fb_api_req_friendly_name"])
		}
	}
}

func TestAccountInsightsErrors(t *testing.T) {
	insta, _ := newReplay(t)
	body := `{"data":{"ig_account_insights":null},"errors":[{"code":1675030,"summary":"Query error","message":"not a professional account"}]}`
	capture(t, insta, func(sent) (int, string) { return 200, body })
	_, err := insta.Account.InsightsHome(t.Context(), InsightsHomeOptions{})
	var gqlErr *GraphQLError
	if !errors.As(err, &gqlErr) || gqlErr.Code != 1675030 || !strings.Contains(err.Error(), friendlyAccountInsightsHome) {
		t.Errorf("home error = %v", err)
	}

	body = `{"data":{"ig_account_insights":null}}`
	if _, err := insta.Account.InsightsBioLinks(t.Context(), PeriodThisMonth); !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("bio links error = %v", err)
	}
	body = `{"data":{"ig_account_insights":{}}}`
	if _, err := insta.Account.InsightsMediaComponent(t.Context(), "1"); !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("media component without bundle = %v", err)
	}
}

func TestAccountInsightsREST(t *testing.T) {
	insta, _ := newReplay(t)
	calls := capture(t, insta, answer(`{"status":"ok"}`))
	ctx := t.Context()
	acc := insta.Account
	for _, tc := range []struct {
		method, path string
		signed       bool
		call         func() error
	}{
		{"POST", "/api/v1/users/accept_insights_terms/", true, func() error { return acc.AcceptInsightsTerms(ctx) }},
		{"GET", "/api/v1/insights/stories_opt_in_month_back/", false, func() error { return acc.OptInStoriesInsightsMonthBack(ctx) }},
		{"GET", "/api/v1/insights/reset_stories_opt_in/", false, func() error { return acc.ResetStoriesInsightsOptIn(ctx) }},
	} {
		if err := tc.call(); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		got := (*calls)[len(*calls)-1]
		if got.req.Method != tc.method || got.req.URL.Path != tc.path || got.signed != tc.signed {
			t.Errorf("%s %s signed=%v, want %s %s signed=%v", got.req.Method, got.req.URL.Path, got.signed, tc.method, tc.path, tc.signed)
		}
		if tc.signed && got.payload["_uuid"] != insta.uuid {
			t.Errorf("%s: payload %v", tc.path, got.payload)
		}
	}
}
