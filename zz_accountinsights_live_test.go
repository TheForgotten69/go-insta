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
	bloks := acc.bloksID()
	period := PeriodThisMonth.vars()

	var mediaID string
	posts := acc.User().Posts()
	if err := posts.Next(ctx); err == nil && len(posts.Items) > 0 {
		mediaID = posts.Items[0].pk()
	}

	for _, q := range []struct {
		friendly, docID, root string
		vars                  any
	}{
		{friendlyAccountInsightsHome, docAccountInsightsHome, rootAccountInsights, insightsHomeVars{
			insightsRangeVars: period, IncludeTools: true, IncludeMetaOneSubtitle: true, IncludeNextStepsTips: true,
			IncludeAIGuidance: true, MetaOnePacingKey: metaOnePacingPro, BloksVersioningID: bloks}},
		{friendlyAccountInsightsOverview, docAccountInsightsOverview, rootAccountInsights, insightsBloksVars{period, bloks}},
		{friendlyAccountInsightsAudience, docAccountInsightsAudience, rootAccountInsights, insightsBloksVars{period, bloks}},
		{friendlyAccountInsightsContent, docAccountInsightsContent, rootAccountInsights, insightsContentVars{
			insightsRangeVars: period, Metric: string(MetricViews), Count: contentInsightsPageSize, SortDescending: true,
			BloksVersioningID: bloks}},
		{friendlyAccountInsightsMedia, docAccountInsightsMediaComponent, rootAccountInsights, insightsMediaComponentVars{mediaID, bloks}},
		{friendlyAccountInsightsBioLinks, docAccountInsightsBioLinks, rootAccountInsights, period},
		{friendlyAccountInsightsPrevBioLinks, docAccountInsightsPreviousBioLinks, rootAccountInsights,
			insightsPreviousBioLinksVars{insightsRangeVars: period, First: previousBioLinksPageSize}},
		{friendlyAccountInsightsViewerState, docAccountInsightsViewerState, rootAccountInsights,
			insightsViewerStateVars{Period: period.Period, IncludePremiumInsights: true}},
		{friendlyActionableInsightsTipBanner, docActionableInsightsTipBanner, "", insightsTipBannerVars{}},
	} {
		body, err := insta.graphQL(ctx, gqlQuery{url: graphQLWWWURL, friendly: q.friendly, docID: q.docID, root: q.root}, q.vars)
		t.Logf("%s: err=%v\n%s", q.friendly, err, body)
	}

	home, err := acc.InsightsHome(ctx, InsightsHomeOptions{})
	t.Logf("InsightsHome: %+v err=%v", home, err)
	content := acc.InsightsContent(ContentInsightsOptions{})
	err = content.Next(ctx)
	t.Logf("InsightsContent: %+v err=%v", content.Items, err)
	links, err := acc.InsightsBioLinks(ctx, PeriodThisMonth)
	t.Logf("InsightsBioLinks: %+v err=%v", links, err)
	state, err := acc.InsightsViewerState(ctx, PeriodThisMonth, true)
	t.Logf("InsightsViewerState: %+v err=%v", state, err)

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
