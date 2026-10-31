package goinsta

import (
	"context"
	"encoding/json"
	"time"
)

type seriesInsightsVars struct {
	PlaylistID string `json:"playlistId"`
}

type seriesOverviewVars struct {
	PlaylistID           string `json:"playlistId"`
	IncludeViewsOverTime bool   `json:"includeViewsOverTime"`
}

// SeriesInsightsOverview loads the overview tab of a Reels series (a linked
// media playlist, the linked_media_playlist_id of its Reels).
func (insta *Instagram) SeriesInsightsOverview(ctx context.Context, seriesID string, viewsOverTime bool) (json.RawMessage, error) {
	return insta.creatorInsights(ctx, friendlySeriesInsightsOverview, docSeriesInsightsOverview, rootLinkedMediaPlaylist,
		seriesOverviewVars{PlaylistID: seriesID, IncludeViewsOverTime: viewsOverTime})
}

func (insta *Instagram) SeriesInsightsEngagement(ctx context.Context, seriesID string) (json.RawMessage, error) {
	return insta.creatorInsights(ctx, friendlySeriesInsightsEngagement, docSeriesInsightsEngagement, rootLinkedMediaPlaylist,
		seriesInsightsVars{PlaylistID: seriesID})
}

func (insta *Instagram) SeriesInsightsAudience(ctx context.Context, seriesID string) (json.RawMessage, error) {
	return insta.creatorInsights(ctx, friendlySeriesInsightsAudience, docSeriesInsightsAudience, rootLinkedMediaPlaylist,
		seriesInsightsVars{PlaylistID: seriesID})
}

type fanClubInsightsVars struct {
	FanClubID string `json:"fan_club_id"`
	Date      string `json:"date"`
	Count     int    `json:"count"`
}

// FanClubInsights loads the subscriptions (fan club) dashboard for day; a zero
// day means today, as the app asks.
func (account *Account) FanClubInsights(ctx context.Context, fanClubID string, day time.Time) (json.RawMessage, error) {
	if day.IsZero() {
		day = time.Now()
	}
	return account.insta.creatorInsights(ctx, friendlyFanClubInsights, docFanClubInsights, rootFanClubInsights,
		fanClubInsightsVars{FanClubID: fanClubID, Date: day.Format(time.DateOnly), Count: 50})
}

func (insta *Instagram) creatorInsights(ctx context.Context, friendly, docID, rootName string, vars any) (json.RawMessage, error) {
	_, root, err := insta.graphQLData(ctx, gqlQuery{url: graphQLWWWURL, friendly: friendly, docID: docID, root: rootName}, vars)
	return root, err
}

// SubscriptionInsights is the paid subscriptions (user pay) dashboard.
type SubscriptionInsights struct {
	TotalEarned string `json:"formatted_total_amount_earned_by_account"`
	// Per-media earnings; empty on every account seen so far, so the shape is unknown.
	MediaInsights []json.RawMessage `json:"media_insights"`
}

func (account *Account) SubscriptionInsights(ctx context.Context) (*SubscriptionInsights, error) {
	var in SubscriptionInsights
	if err := account.insta.callJSON(ctx, &reqOptions{Endpoint: urlUserPayInsights}, &in); err != nil {
		return nil, err
	}
	return &in, nil
}
