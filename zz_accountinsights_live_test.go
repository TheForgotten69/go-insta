//go:build live

package goinsta

import (
	"os"
	"testing"
)

// go test -tags live -run TestLiveAccountInsights -v . [INSIGHTS_WRITE=1]
//
// Read-only unless INSIGHTS_WRITE=1, which also accepts the insights terms (no
// known way back) and opts story insights in, then resets the opt-in.
func TestLiveAccountInsights(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()
	acc := insta.Account
	show := func(name string, v any, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		t.Logf("%s: %+v", name, v)
	}
	home, err := acc.InsightsHome(ctx, InsightsHomeOptions{})
	show("InsightsHome", home, err)
	over, err := acc.InsightsOverview(ctx, PeriodThisMonth)
	show("InsightsOverview", over, err)
	aud, err := acc.InsightsAudience(ctx, PeriodThisMonth)
	show("InsightsAudience", aud, err)
	content := acc.InsightsContent(ContentInsightsOptions{})
	show("InsightsContent", content.Items, content.Next(ctx))
	if len(content.Items) > 0 {
		tree, err := acc.InsightsMediaComponent(ctx, content.Items[0].Thumbnail.MediaID)
		show("InsightsMediaComponent", len(tree), err)
	}
	links, err := acc.InsightsBioLinks(ctx, PeriodThisMonth)
	show("InsightsBioLinks", links, err)
	prev := acc.InsightsPreviousBioLinks(PeriodThisMonth)
	show("InsightsPreviousBioLinks", prev.Items, prev.Next(ctx))
	state, err := acc.InsightsViewerState(ctx, PeriodThisMonth, true)
	show("InsightsViewerState", state, err)

	if os.Getenv("INSIGHTS_WRITE") != "1" {
		return
	}
	t.Logf("show_insights_terms before: %v", acc.ShowInsightsTerms)
	t.Logf("AcceptInsightsTerms: %v", acc.AcceptInsightsTerms(ctx))
	if err := acc.Sync(ctx); err == nil {
		t.Logf("show_insights_terms after: %v", acc.ShowInsightsTerms)
	}
	t.Logf("OptInStoriesInsightsMonthBack: %v", acc.OptInStoriesInsightsMonthBack(ctx))
	t.Logf("ResetStoriesInsightsOptIn: %v", acc.ResetStoriesInsightsOptIn(ctx))
}
