//go:build live

package goinsta

import (
	"os"
	"testing"
)

// INSIGHTS_WRITE=1 go test -tags live -run TestLiveAccountInsightsWrites -v .
//
// Accepts the insights terms (no known way back), opts story insights in, then
// resets the opt-in. The reads run in cmd/smoke.
func TestLiveAccountInsightsWrites(t *testing.T) {
	if os.Getenv("INSIGHTS_WRITE") != "1" {
		t.Skip("set INSIGHTS_WRITE=1")
	}
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()
	acc := insta.Account
	t.Logf("show_insights_terms before: %v", acc.ShowInsightsTerms)
	t.Logf("AcceptInsightsTerms: %v", acc.AcceptInsightsTerms(ctx))
	if err := acc.Sync(ctx); err == nil {
		t.Logf("show_insights_terms after: %v", acc.ShowInsightsTerms)
	}
	t.Logf("OptInStoriesInsightsMonthBack: %v", acc.OptInStoriesInsightsMonthBack(ctx))
	t.Logf("ResetStoriesInsightsOptIn: %v", acc.ResetStoriesInsightsOptIn(ctx))
}
