//go:build live

package goinsta

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Read-only. SERIES_ID and FAN_CLUB_ID select a series and a fan club; without
// them the calls run with placeholder ids to show the server's error.
func TestLiveCreatorInsights(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()
	series := os.Getenv("SERIES_ID")
	if series == "" {
		series = "0"
	}
	fanClub := os.Getenv("FAN_CLUB_ID")
	if fanClub == "" {
		fanClub = "0"
	}
	show := func(name string, in json.RawMessage, err error) {
		if err != nil {
			t.Logf("%-28s err=%v", name, err)
			return
		}
		t.Logf("%-28s %.1500s", name, in)
	}
	in, err := insta.SeriesInsightsOverview(ctx, series, true)
	show("SeriesInsightsOverview", in, err)
	in, err = insta.SeriesInsightsEngagement(ctx, series)
	show("SeriesInsightsEngagement", in, err)
	in, err = insta.SeriesInsightsAudience(ctx, series)
	show("SeriesInsightsAudience", in, err)
	in, err = insta.Account.FanClubInsights(ctx, fanClub, time.Time{})
	show("FanClubInsights", in, err)
}
