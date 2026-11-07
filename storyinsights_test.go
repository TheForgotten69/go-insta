package goinsta

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The unit bodies are the live answers for a 12-day-old story with 5 viewers
// on the Creator test account (2026-11-08).
func TestStoryInsights(t *testing.T) {
	raw, err := os.ReadFile("testdata/storyinsights.json")
	if err != nil {
		t.Fatal(err)
	}
	var bodies map[string]json.RawMessage
	if err := json.Unmarshal(raw, &bodies); err != nil {
		t.Fatal(err)
	}
	insta, _ := newReplay(t)
	got := capture(t, insta, func(s sent) (int, string) {
		if strings.HasSuffix(s.req.URL.Path, "/info/") {
			return 200, `{"items":[{"pk":"4017339574726819587","id":"4017339574726819587_42865264907","fbid":18386150089230004}],"status":"ok"}`
		}
		app := strings.TrimPrefix(s.payload["fb_api_req_friendly_name"].(string), friendlyBloksAppRoot+storyInsightsApp)
		return 200, string(bodies["story_insights_"+app])
	})
	in, err := testItem(insta, "4017339574726819587_42865264907").StoryInsights(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	reqs := *got
	if len(reqs) != 1+len(storyInsightsUnits) || reqs[0].req.URL.Path != "/api/v1/media/4017339574726819587_42865264907/info/" {
		t.Fatalf("%d requests, first %s", len(reqs), reqs[0].req.URL.Path)
	}
	for i, unit := range storyInsightsUnits {
		r := reqs[i+1]
		v := r.vars()
		params, _ := v["params"].(map[string]any)
		bk, _ := v["bk_context"].(map[string]any)
		if r.req.URL.Path != "/graphql_www" || r.payload["client_doc_id"] != docBloksComponentQuery ||
			r.req.Header.Get("X-Root-Field-Name") != rootBloksComponentQuery || params["app_id"] != storyInsightsApp+unit ||
			params["bloks_versioning_id"] != bloksVerID || bk["android_os_api_level"] != 33.0 {
			t.Errorf("%s request: %s %v %v", unit, r.req.URL.Path, r.payload["client_doc_id"], v)
		}
		if p := params["params"]; p != `{"params":{"media_fbidv2":"18386150089230004","is_elevated_surface":false,"origin":"unknown"}}` {
			t.Errorf("%s params = %v", unit, p)
		}
	}

	if in.Navigation == nil || *in.Navigation != 5 || in.Exited == nil || *in.Exited != 5 ||
		in.Forward != nil || in.Back != nil || in.NextStory != nil {
		t.Errorf("navigation = %v forward=%v back=%v next=%v exited=%v", in.Navigation, in.Forward, in.Back, in.NextStory, in.Exited)
	}
	want := map[string]map[string]string{
		StoryUnitOverview:        {"Views": "6", "Interactions": "0", "Profile activity": "1"},
		StoryUnitViews:           {"Viewers": "5"},
		StoryUnitInteractions:    {"Likes": "0", "Replies": "--", "Shares": "--", "Navigation": "5", "Exited": "5"},
		StoryUnitProfileActivity: {"Profile visits": "1", "Follows": "0"},
	}
	if !reflect.DeepEqual(in.Rows, want) {
		t.Errorf("rows = %v", in.Rows)
	}
}

func TestRowCount(t *testing.T) {
	for v, want := range map[string]*int{"5": new(5), "1,234": new(1234), "--": nil, "": nil} {
		if got := rowCount(v); (got == nil) != (want == nil) || (got != nil && *got != *want) {
			t.Errorf("rowCount(%q) = %v", v, got)
		}
	}
}
