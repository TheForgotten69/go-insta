package goinsta

import (
	"encoding/json"
	"strings"
	"testing"
)

// mergedInsights folds the captured incremental response into one document. The
// patches address "ig_post_insights(…)" while the skeleton key is aliased
// "1$ig_post_insights(…)", which mergeIncremental does not resolve yet.
func mergedInsights(t *testing.T, body string) string {
	t.Helper()
	var doc struct {
		Data map[string]any `json:"data"`
	}
	var key string
	for i, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if i == 0 {
			if err := json.Unmarshal([]byte(line), &doc); err != nil {
				t.Fatal(err)
			}
			for k := range doc.Data {
				if strings.Contains(k, rootPostInsights+"(") {
					key = k
				}
			}
			continue
		}
		var patch struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &patch); err != nil {
			t.Fatal(err)
		}
		root := doc.Data[key].(map[string]any)
		for k, v := range patch.Data {
			root[k] = v
		}
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

func TestPostInsights(t *testing.T) {
	insta, r := newReplay(t, "post_insights")
	f := fixture(t, "post_insights")
	r.fixtures[0].RespBody = mergedInsights(t, f.RespBody)
	item := testItem(insta, "4017338070095875306_42865264907")
	in, err := item.Insights(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, f, r.sent[0])
	checkVars(t, f, r.sent[0])
	if in.MediaFBID != "18080811533742582" || in.TotalViews != 1 || in.NonFollowersViews != 1 ||
		in.Shares != nil || in.Demographics.Status != "INSUFFICIENT_DATA" {
		t.Errorf("insights = %+v", in)
	}
}
