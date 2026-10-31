package goinsta

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestCreatorInsightsRequests(t *testing.T) {
	insta, _ := newReplay(t)
	sent := capture(t, insta, func(s sent) (int, string) {
		if s.req.URL.Path == "/api/v1/creators/user_pay/insights/" {
			return 200, `{"status":"ok","total":3}`
		}
		return 200, `{"data":{"fetch__XIGLinkedMediaPlaylist":{"id":"1"},"node":{"id":"2"}},"extensions":{"is_final":true}}`
	})
	ctx := t.Context()
	day := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name, doc string
		call      func(context.Context) (json.RawMessage, error)
		vars      map[string]any
	}{
		{friendlySeriesInsightsOverview, docSeriesInsightsOverview,
			func(ctx context.Context) (json.RawMessage, error) {
				return insta.SeriesInsightsOverview(ctx, "99", true)
			},
			map[string]any{"playlistId": "99", "includeViewsOverTime": true}},
		{friendlySeriesInsightsEngagement, docSeriesInsightsEngagement,
			func(ctx context.Context) (json.RawMessage, error) { return insta.SeriesInsightsEngagement(ctx, "99") },
			map[string]any{"playlistId": "99"}},
		{friendlySeriesInsightsAudience, docSeriesInsightsAudience,
			func(ctx context.Context) (json.RawMessage, error) { return insta.SeriesInsightsAudience(ctx, "99") },
			map[string]any{"playlistId": "99"}},
		{friendlyFanClubInsights, docFanClubInsights,
			func(ctx context.Context) (json.RawMessage, error) {
				return insta.Account.FanClubInsights(ctx, "7", day)
			},
			map[string]any{"fan_club_id": "7", "date": "2026-10-31", "count": 50.0}},
	} {
		in, err := tc.call(ctx)
		if err != nil || len(in) == 0 {
			t.Errorf("%s: %v %v", tc.name, in, err)
			continue
		}
		s := (*sent)[len(*sent)-1]
		if s.req.URL.Path != "/graphql_www" || s.payload["fb_api_req_friendly_name"] != tc.name || s.payload["client_doc_id"] != tc.doc {
			t.Errorf("%s: %s %v %v", tc.name, s.req.URL.Path, s.payload["fb_api_req_friendly_name"], s.payload["client_doc_id"])
		}
		if s.req.Header.Get("Content-Encoding") == "gzip" {
			t.Errorf("%s: gzipped", tc.name)
		}
		v := s.vars()
		for k, want := range tc.vars {
			if v[k] != want {
				t.Errorf("%s: %s = %v, want %v", tc.name, k, v[k], want)
			}
		}
	}
	in, err := insta.Account.SubscriptionInsights(ctx)
	s := (*sent)[len(*sent)-1]
	if err != nil || s.req.Method != "GET" || s.req.URL.Path != "/api/v1/creators/user_pay/insights/" || in == nil {
		t.Errorf("SubscriptionInsights: %v %s %s", err, s.req.Method, s.req.URL.Path)
	}
}

func TestCreatorInsightsEmptyRoot(t *testing.T) {
	insta, _ := newReplay(t)
	capture(t, insta, answer(`{"data":{"fetch__XIGLinkedMediaPlaylist":null}}`))
	if _, err := insta.SeriesInsightsAudience(t.Context(), "1"); err == nil {
		t.Fatal("null root accepted")
	}
}
