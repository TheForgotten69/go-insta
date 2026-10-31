package goinsta

import (
	"context"
	"encoding/json"
	"time"
)

const (
	docSeriesInsightsOverview   = "383515635314126103399555900562"
	docSeriesInsightsEngagement = "281083989014079368110826164376"
	docSeriesInsightsAudience   = "386340097017018189326628161392"
	docFanClubInsights          = "147178694610827473488858200058"
	docEventInsightsUpsampling  = "196534572516133624712880039744"

	rootLinkedMediaPlaylist = "fetch__XIGLinkedMediaPlaylist"
	rootFanClubInsights     = "node"
	rootEventInsights       = "xfb_oxsights_event_insights_query_events_for_upsampling"

	urlUserPayInsights = "creators/user_pay/insights/"
)

// CreatorInsights is the response of a creator analytics query. The app maps
// these responses by hashed field ids, so the field names are not known yet;
// Raw holds the whole root object.
type CreatorInsights struct {
	Raw json.RawMessage
}

type seriesInsightsVars struct {
	PlaylistID string `json:"playlistId"`
}

type seriesOverviewVars struct {
	PlaylistID           string `json:"playlistId"`
	IncludeViewsOverTime bool   `json:"includeViewsOverTime"`
}

// SeriesInsightsOverview loads the overview tab of a Reels series (a linked
// media playlist, the linked_media_playlist_id of its Reels).
func (insta *Instagram) SeriesInsightsOverview(ctx context.Context, seriesID string, viewsOverTime bool) (*CreatorInsights, error) {
	return insta.creatorInsights(ctx, "SeriesInsightsOverviewQuery", docSeriesInsightsOverview, rootLinkedMediaPlaylist,
		seriesOverviewVars{PlaylistID: seriesID, IncludeViewsOverTime: viewsOverTime})
}

func (insta *Instagram) SeriesInsightsEngagement(ctx context.Context, seriesID string) (*CreatorInsights, error) {
	return insta.creatorInsights(ctx, "SeriesInsightsEngagementQuery", docSeriesInsightsEngagement, rootLinkedMediaPlaylist,
		seriesInsightsVars{PlaylistID: seriesID})
}

func (insta *Instagram) SeriesInsightsAudience(ctx context.Context, seriesID string) (*CreatorInsights, error) {
	return insta.creatorInsights(ctx, "SeriesInsightsAudienceQuery", docSeriesInsightsAudience, rootLinkedMediaPlaylist,
		seriesInsightsVars{PlaylistID: seriesID})
}

type fanClubInsightsVars struct {
	FanClubID string `json:"fan_club_id"`
	Date      string `json:"date"`
	Count     int    `json:"count"`
}

// FanClubInsights loads the subscriptions (fan club) dashboard for day; a zero
// day means today, as the app asks.
func (account *Account) FanClubInsights(ctx context.Context, fanClubID string, day time.Time) (*CreatorInsights, error) {
	if day.IsZero() {
		day = time.Now()
	}
	return account.insta.creatorInsights(ctx, "FanClubInsights", docFanClubInsights, rootFanClubInsights,
		fanClubInsightsVars{FanClubID: fanClubID, Date: day.Format(time.DateOnly), Count: 50})
}

type eventInsightsVars struct {
	StartTime int64   `json:"start_time"`
	EndTime   int64   `json:"end_time"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// EventInsightsNearby loads the events around a position that the app collects
// event insights for, from now to now+window.
func (insta *Instagram) EventInsightsNearby(ctx context.Context, lat, lng float64, window time.Duration) (*CreatorInsights, error) {
	now := time.Now().Unix()
	return insta.creatorInsights(ctx, "EventInsightsUpsamplingQuery", docEventInsightsUpsampling, rootEventInsights,
		eventInsightsVars{StartTime: now, EndTime: now + int64(window/time.Second), Latitude: lat, Longitude: lng})
}

func (insta *Instagram) creatorInsights(ctx context.Context, friendly, docID, root string, vars any) (*CreatorInsights, error) {
	var raw json.RawMessage
	err := insta.graphQLRoot(ctx, gqlQuery{url: graphQLWWWURL, friendly: friendly, docID: docID, root: root}, vars, &raw)
	if err != nil {
		return nil, err
	}
	return &CreatorInsights{Raw: raw}, nil
}

// SubscriptionInsights loads the paid subscriptions (user pay) insights.
func (account *Account) SubscriptionInsights(ctx context.Context) (*CreatorInsights, error) {
	var raw json.RawMessage
	if err := account.insta.callJSON(ctx, &reqOptions{Endpoint: urlUserPayInsights}, &raw); err != nil {
		return nil, err
	}
	return &CreatorInsights{Raw: raw}, nil
}
