package goinsta

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	metaOnePacingCreator = "IG_COMPETITIVE_INSIGHTS_CREATOR_DASH_PERMANENT_UPSELL"
	metaOnePacingPro     = "IG_COMPETITIVE_INSIGHTS_PRO_DASH_PERMANENT_UPSELL"

	periodCustom = "CUSTOM"

	contentInsightsPageSize  = 24
	previousBioLinksPageSize = 20
)

// InsightsPeriod is the dashboard's time range. The zero value is PeriodThisMonth,
// the app's default.
type InsightsPeriod struct {
	name       string
	start, end int64 // epoch seconds, CUSTOM only
}

// The periods in the app's picker order, plus LIFETIME, which only an insights
// deep-link parser accepts.
var (
	PeriodThisWeek      = InsightsPeriod{name: "THIS_WEEK"}
	PeriodLast14Days    = InsightsPeriod{name: "LAST_14_DAYS"}
	PeriodThisMonth     = InsightsPeriod{name: "THIS_MONTH"}
	PeriodLast60Days    = InsightsPeriod{name: "LAST_60_DAYS"}
	PeriodPreviousMonth = InsightsPeriod{name: "PREVIOUS_MONTH"}
	PeriodLast90Days    = InsightsPeriod{name: "LAST_90_DAYS"}
	PeriodLast180Days   = InsightsPeriod{name: "LAST_180_DAYS"}
	PeriodLast365Days   = InsightsPeriod{name: "LAST_365_DAYS"}
	PeriodLast2Years    = InsightsPeriod{name: "LAST_2_YEARS"}
	PeriodLifetime      = InsightsPeriod{name: "LIFETIME"}
)

// CustomPeriod is the date picker's range; Instagram takes it in whole seconds.
func CustomPeriod(start, end time.Time) InsightsPeriod {
	return InsightsPeriod{name: periodCustom, start: start.Unix(), end: end.Unix()}
}

func (p InsightsPeriod) String() string {
	if p.name == "" {
		return PeriodThisMonth.name
	}
	return p.name
}

// insightsRangeVars is sent as the app does: startDate and endDate are null
// unless the period is CUSTOM.
type insightsRangeVars struct {
	Period    string `json:"period"`
	StartDate *int64 `json:"startDate"`
	EndDate   *int64 `json:"endDate"`
}

func (p InsightsPeriod) vars() insightsRangeVars {
	v := insightsRangeVars{Period: p.String()}
	if p.custom() {
		v.StartDate, v.EndDate = &p.start, &p.end
	}
	return v
}

func (p InsightsPeriod) custom() bool { return p.name == periodCustom }

// InsightsMetric ranks the content tab. The empty value sends "views", as the app
// does for its "latest" sort.
type InsightsMetric string

const (
	MetricViews               InsightsMetric = "views"
	MetricViewers             InsightsMetric = "viewers"
	MetricFollows             InsightsMetric = "follows"
	MetricLikes               InsightsMetric = "likes"
	MetricComments            InsightsMetric = "comments"
	MetricReposts             InsightsMetric = "reposts"
	MetricShares              InsightsMetric = "shares"
	MetricSaves               InsightsMetric = "saves"
	MetricSkipRate            InsightsMetric = "skip_rate" // Reels only
	MetricProfileVisits       InsightsMetric = "profile_visits"
	MetricBioLinkTaps         InsightsMetric = "bio_link_taps"
	MetricBusinessAddressTaps InsightsMetric = "business_address_taps"
	MetricLinkClicks          InsightsMetric = "link_clicks"
)

// InsightsContentType filters the content tab; the empty value is all content.
type InsightsContentType string

const (
	ContentPosts      InsightsContentType = "posts"
	ContentReels      InsightsContentType = "reels"
	ContentStories    InsightsContentType = "stories"
	ContentLiveVideos InsightsContentType = "live_videos"
)

type insightsAudienceVars struct {
	insightsRangeVars
	RedesignEnabled     bool   `json:"redesignEnabled"`
	UseFollowerSnapshot bool   `json:"useFollowerSnapshot"`
	TopLanguagesEnabled bool   `json:"topLanguagesEnabled"`
	BloksVersioningID   string `json:"bloksVersioningId"`
}

type insightsOverviewVars struct {
	insightsRangeVars
	RedesignEnabled   bool   `json:"redesignEnabled"`
	BloksVersioningID string `json:"bloksVersioningId"`
}

type insightsHomeVars struct {
	insightsRangeVars
	IncludeTools           bool   `json:"includeTools"`
	IncludeMetaOneSubtitle bool   `json:"includeMetaOneSubtitle"`
	IncludeNextStepsTips   bool   `json:"includeNextStepsTips"`
	IncludeAIGuidance      bool   `json:"includeAIGuidance"`
	MetaOnePacingKey       string `json:"meta_one_pacing_key"`
	BloksVersioningID      string `json:"bloksVersioningId"`
}

type insightsContentVars struct {
	insightsRangeVars
	Metric                                string  `json:"metric"`
	IncludeCarouselMediaInsightsRendering bool    `json:"includeCarouselMediaInsightsRendering"`
	SortByCreationTime                    bool    `json:"sortByCreationTime"`
	Count                                 int     `json:"count"`
	After                                 *string `json:"after"`
	ContentType                           *string `json:"contentType"`
	SortDescending                        bool    `json:"sortDescending"`
	BloksVersioningID                     string  `json:"bloksVersioningId"`
}

type insightsPreviousBioLinksVars struct {
	insightsRangeVars
	First int     `json:"first"`
	After *string `json:"after"`
}

type insightsViewerStateVars struct {
	Period                 string `json:"period"`
	IncludePremiumInsights bool   `json:"includePremiumInsights"`
}

type insightsMediaComponentVars struct {
	MediaID           string `json:"mediaId"`
	BloksVersioningID string `json:"bloksVersioningId"`
}

// InsightsHomeOptions are the home tab's inputs. The app sets the include flags
// from mobile config; InsightsHome turns them all on.
type InsightsHomeOptions struct {
	Period InsightsPeriod
	// Creator picks the creator dashboard's Meta One upsell key; the app decides
	// it from the account type (creator vs business).
	Creator bool
}

// AccountInsightsHome is the home tab.
type AccountInsightsHome struct {
	Layout                   string // HERO_PLUS_TWO, …
	Tiles                    []InsightsTile
	Tools                    []InsightsTool
	HasAccountStatusWarnings bool
	NextStepsBloks           string // the next-steps cards' Bloks UI bundle
	MetaOneEligibility       string // Meta Verified upsell: INELIGIBLE, …
}

// InsightsTile is a summary metric (views, interactions, followers, …).
type InsightsTile struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Metric     Metric        `json:"metric"`
	Delta      InsightsDelta `json:"delta"`
	TimeSeries TimeSeries    `json:"time_series"`
}

// TimeSeries is a chart, one point a day.
type TimeSeries struct {
	DataPoints []DataPoint `json:"data_points"`
}

type DataPoint struct {
	Timestamp int64   `json:"timestamp"` // epoch seconds
	Value     float64 `json:"value"`
}

// InsightsDelta is the change over the previous period, as display text.
type InsightsDelta struct {
	Value     string `json:"value"`
	Sentiment string `json:"sentiment"`
}

// InsightsBenchmark compares a value with the previous period or similar accounts.
type InsightsBenchmark struct {
	Value         float64 `json:"value"`
	Comparison    string  `json:"comparison"` // HIGHER, TYPICAL, …
	Sentiment     string  `json:"sentiment"`
	DisplayString string  `json:"display_string"`
}

// InsightsTool is a home-tab shortcut (an id enum and an in-app destination).
type InsightsTool struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Destination        string `json:"destination"`
	DestinationType    string `json:"destination_type"`
	ImpressionDeeplink string `json:"impression_deeplink"`
}

// AccountInsightsOverview is the overview tab: views, reach and interactions,
// split by follower and content type.
type AccountInsightsOverview struct {
	Views                     ViewsCount            `json:"account_views_count"`
	ViewsByContentType        ContentTypeBreakdown  `json:"account_views_by_content_type"`
	Viewers                   Metric                `json:"viewers_or_reach"`
	FollowerViews             Metric                `json:"follower_views"`
	NonFollowerViews          Metric                `json:"non_follower_views"`
	ViewsOverTime             TimeSeries            `json:"views_over_time"`
	ViewersOverTime           TimeSeries            `json:"viewers_over_time"`
	FollowerViewsOverTime     TimeSeries            `json:"follower_views_over_time"`
	NonFollowerViewsOverTime  TimeSeries            `json:"non_follower_views_over_time"`
	Interactions              InteractionsCount     `json:"interactions_count"`
	InteractionsByContentType InteractionsBreakdown `json:"interactions_by_content_type"`
	ProfileVisits             Metric                `json:"profile_visits"`
	BioLinkTaps               Metric                `json:"bio_link_taps"`
	BusinessAddressTaps       Metric                `json:"business_address_taps"`
	LinkPageViews             Metric                `json:"link_page_views"`
	// Null on every account tried, so the shapes are unknown.
	FollowerGrowth json.RawMessage `json:"follower_growth"`
	Tip            json.RawMessage `json:"overview_tip"`
}

// FollowerShare splits a total between followers and everyone else, as
// fractions (0.152 is 15.2%).
type FollowerShare struct {
	Follower    Metric `json:"follower"`
	NonFollower Metric `json:"non_follower"`
}

type ViewsCount struct {
	Total Metric        `json:"total"`
	Share FollowerShare `json:"follow_type_breakdown"`
}

type InteractionsCount struct {
	Total Metric `json:"total"`
	FollowerShare
}

// ContentTypeBreakdown counts per content type: post, story, reel, live.
type ContentTypeBreakdown struct {
	Total   int                `json:"total"`
	Entries []ContentTypeCount `json:"entries"`
}

type ContentTypeCount struct {
	ContentType      string `json:"content_type"`
	Total            int    `json:"total"`
	FollowerCount    int    `json:"follower_count"`
	NonFollowerCount int    `json:"non_follower_count"`
	FacebookCount    *int   `json:"facebook_count"`
}

type InteractionsBreakdown struct {
	Total  ContentTypeBreakdown   `json:"total_breakdown"`
	ByType []InteractionTypeCount `json:"sub_breakdowns"`
}

// InteractionTypeCount is one interaction (likes, comments, reposts, shares,
// saves, …) per content type.
type InteractionTypeCount struct {
	Type    string             `json:"interaction_type"`
	Total   int                `json:"total"`
	Entries []ContentTypeValue `json:"entries"`
}

type ContentTypeValue struct {
	ContentType string `json:"content_type"`
	Value       int    `json:"value"`
}

// AccountInsightsAudience is the audience tab: followers, their activity and
// demographics.
type AccountInsightsAudience struct {
	Followers               Metric          `json:"follower_count"`
	FollowersDelta          InsightsDelta   `json:"follower_growth_delta"`
	ActiveFollowersOverTime TimeSeries      `json:"active_followers_over_time"`
	ActiveTimes             ActiveTimes     `json:"active_times"`
	Age                     AgeDemographics `json:"age_demographics"`
	TopCities               []AudienceShare `json:"top_cities"`
	TopCountries            []AudienceShare `json:"top_countries"`
	// Empty or null on every account tried, so the shapes are unknown.
	Genders              json.RawMessage `json:"gender_demographics"`
	TopLanguages         json.RawMessage `json:"top_languages"`
	NetFollowersOverTime json.RawMessage `json:"net_followers_over_time"` // {follows, unfollows, overall}
	FollowerGrowth       json.RawMessage `json:"follower_growth"`
	Tip                  json.RawMessage `json:"audience_tip"`
	ActiveTimesTip       json.RawMessage `json:"active_times_tip"`
}

// ActiveTimes is when followers are on Instagram, in 3-hour slots per weekday.
type ActiveTimes struct {
	Slots    []ActiveSlot    `json:"slots"`
	TopSlots json.RawMessage `json:"top_slots"`
}

type ActiveSlot struct {
	Day   int     `json:"day"`
	Hour  int     `json:"hour"`
	Value float64 `json:"value"`
}

type AgeDemographics struct {
	Entries []AgeShare `json:"entries"`
}

type AgeShare struct {
	AgeRange        string  `json:"age_range"` // 13-17 … 65+
	Percentage      float64 `json:"percentage"`
	MenPercentage   float64 `json:"men_percentage"`
	WomenPercentage float64 `json:"women_percentage"`
	OtherPercentage float64 `json:"other_percentage"`
}

// AudienceShare is a city or country; "--" names a slot without data.
type AudienceShare struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	DisplayValue string          `json:"display_value"`
	Percentage   float64         `json:"percentage"`
	Segments     json.RawMessage `json:"segments"`
}

// ContentInsightsOptions are the content tab's filters.
type ContentInsightsOptions struct {
	Period             InsightsPeriod
	Metric             InsightsMetric
	Type               InsightsContentType
	SortByCreationTime bool
	Ascending          bool
}

// ContentInsightsItem is one post, Reel or story with its metrics.
type ContentInsightsItem struct {
	Thumbnail ContentInsightsThumbnail `json:"thumbnail"`
	Metrics   []InsightsMetricValue    `json:"metrics"`
}

type ContentInsightsThumbnail struct {
	MediaID      string `json:"media_id"`
	MediaURL     string `json:"media_url"`
	MediaType    string `json:"media_type"`
	CaptionText  string `json:"caption_text"`
	CreationTime Int64  `json:"creation_time"`
	IsPromoted   bool   `json:"is_promoted"`
	IsTrialReel  bool   `json:"is_trial_reel"`
	IsCollab     bool   `json:"is_collab"`
}

type InsightsMetricValue struct {
	Metric string   `json:"metric"`
	Value  *float64 `json:"value"`
}

// AccountInsightsBioLinks are the link taps on the profile's bio links.
type AccountInsightsBioLinks struct {
	Taps             Metric
	PageViews        Metric
	Links            []BioLinkInsights
	HasPreviousLinks bool
}

// BioLinkInsights is one bio link and its taps.
type BioLinkInsights struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	DisplayURL string `json:"display_url"`
	ImageURL   string `json:"image_url"`
	Taps       Metric `json:"taps"`
}

// AccountInsightsViewerState is the dashboard's eligibility state.
type AccountInsightsViewerState struct {
	IsCXUser          bool            `json:"is_cx_user"`
	CXInsightsEnabled bool            `json:"cx_insights_enabled"`
	PremiumInsights   json.RawMessage `json:"premium_insights"` // null without Meta Verified
}

type bioLinksList struct {
	Links    connection[BioLinkInsights] `json:"links"`
	HasLinks bool                        `json:"has_links"`
}

type bloksHolder struct {
	Component struct {
		Bundle struct {
			Tree string `json:"bloks_bundle_tree"`
		} `json:"bundle"`
	} `json:"component"`
}

func (account *Account) bloksID() string { return account.insta.sess.BloksVersionID }

// accountInsightsQuery reads the ig_account_insights object. Insights need a
// professional account; a personal one gets the server's *GraphQLError.
func accountInsightsQuery(docID, friendly string) gqlQuery {
	return gqlQuery{url: graphQLWWWURL, friendly: friendly, docID: docID, root: rootAccountInsights}
}

func (account *Account) insights(ctx context.Context, docID, friendly string, vars, out any) error {
	return account.insta.graphQLRoot(ctx, accountInsightsQuery(docID, friendly), vars, out)
}

// InsightsHome loads the dashboard's home tab.
func (account *Account) InsightsHome(ctx context.Context, opts InsightsHomeOptions) (*AccountInsightsHome, error) {
	key := metaOnePacingPro
	if opts.Creator {
		key = metaOnePacingCreator
	}
	data, root, err := account.insta.graphQLData(ctx, accountInsightsQuery(docAccountInsightsHome, friendlyAccountInsightsHome), insightsHomeVars{
		insightsRangeVars:      opts.Period.vars(),
		IncludeTools:           true,
		IncludeMetaOneSubtitle: true,
		IncludeNextStepsTips:   true,
		IncludeAIGuidance:      true,
		MetaOnePacingKey:       key,
		BloksVersioningID:      account.bloksID(),
	})
	if err != nil {
		return nil, err
	}
	var in struct {
		Summary struct {
			Layout string         `json:"layout"`
			Tiles  []InsightsTile `json:"tiles"`
		} `json:"summary_unit"`
		Tools                    []InsightsTool `json:"tools"`
		HasAccountStatusWarnings bool           `json:"has_account_status_warnings"`
		NextSteps                bloksHolder    `json:"next_steps_tips"`
	}
	if err := json.Unmarshal(root, &in); err != nil {
		return nil, err
	}
	var metaOne struct {
		Status string `json:"eligibility_status"`
	}
	if raw := data["metaOneEligibility"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &metaOne); err != nil {
			return nil, err
		}
	}
	return &AccountInsightsHome{
		Layout:                   in.Summary.Layout,
		Tiles:                    in.Summary.Tiles,
		Tools:                    in.Tools,
		HasAccountStatusWarnings: in.HasAccountStatusWarnings,
		NextStepsBloks:           in.NextSteps.Component.Bundle.Tree,
		MetaOneEligibility:       metaOne.Status,
	}, nil
}

// InsightsOverview loads the overview tab.
func (account *Account) InsightsOverview(ctx context.Context, period InsightsPeriod) (*AccountInsightsOverview, error) {
	return graphQLInto[AccountInsightsOverview](ctx, account.insta, accountInsightsQuery(docAccountInsightsOverview, friendlyAccountInsightsOverview),
		insightsOverviewVars{insightsRangeVars: period.vars(), RedesignEnabled: true, BloksVersioningID: account.bloksID()})
}

// InsightsAudience loads the audience (followers) tab. A custom period reads the
// follower snapshot, as the redesigned app does.
func (account *Account) InsightsAudience(ctx context.Context, period InsightsPeriod) (*AccountInsightsAudience, error) {
	return graphQLInto[AccountInsightsAudience](ctx, account.insta, accountInsightsQuery(docAccountInsightsAudience, friendlyAccountInsightsAudience), insightsAudienceVars{
		insightsRangeVars:   period.vars(),
		RedesignEnabled:     true,
		UseFollowerSnapshot: period.custom(),
		TopLanguagesEnabled: true,
		BloksVersioningID:   account.bloksID(),
	})
}

// InsightsContent pages through the content tab.
func (account *Account) InsightsContent(opts ContentInsightsOptions) *Pager[ContentInsightsItem] {
	metric := string(cmp.Or(opts.Metric, MetricViews))
	var contentType *string
	if opts.Type != "" {
		t := string(opts.Type)
		contentType = &t
	}
	var after *string
	return newPager(func(ctx context.Context) ([]ContentInsightsItem, string, error) {
		var page struct {
			YourContent struct {
				Items    []ContentInsightsItem `json:"items"`
				PageInfo graphQLPageInfo       `json:"page_info"`
			} `json:"your_content"`
		}
		err := account.insights(ctx, docAccountInsightsContent, friendlyAccountInsightsContent, insightsContentVars{
			insightsRangeVars:  opts.Period.vars(),
			Metric:             metric,
			SortByCreationTime: opts.SortByCreationTime,
			Count:              contentInsightsPageSize,
			After:              after,
			ContentType:        contentType,
			SortDescending:     !opts.Ascending,
			BloksVersioningID:  account.bloksID(),
		}, &page)
		if err != nil {
			return nil, "", err
		}
		next := page.YourContent.PageInfo.cursor()
		after = &next
		return page.YourContent.Items, next, nil
	})
}

// InsightsMediaComponent returns the Bloks UI bundle of one post's insights
// carousel card. mediaID is the post's pk.
func (account *Account) InsightsMediaComponent(ctx context.Context, mediaID string) (string, error) {
	var in struct {
		Carousel bloksHolder `json:"bloks_carousel_media_insights_component"`
	}
	err := account.insights(ctx, docAccountInsightsMediaComponent, friendlyAccountInsightsMedia,
		insightsMediaComponentVars{MediaID: mediaID, BloksVersioningID: account.bloksID()}, &in)
	if err != nil {
		return "", err
	}
	tree := in.Carousel.Component.Bundle.Tree
	if tree == "" {
		return "", fmt.Errorf("%s: %w", friendlyAccountInsightsMedia, ErrEmptyResponse)
	}
	return tree, nil
}

// InsightsBioLinks loads the bio link taps for the period.
func (account *Account) InsightsBioLinks(ctx context.Context, period InsightsPeriod) (*AccountInsightsBioLinks, error) {
	var in struct {
		BioLinks struct {
			Taps          Metric       `json:"taps"`
			PageViews     Metric       `json:"page_views"`
			Links         bioLinksList `json:"links"`
			PreviousLinks bioLinksList `json:"previous_links"`
		} `json:"bio_links"`
	}
	if err := account.insights(ctx, docAccountInsightsBioLinks, friendlyAccountInsightsBioLinks, period.vars(), &in); err != nil {
		return nil, err
	}
	b := in.BioLinks
	return &AccountInsightsBioLinks{
		Taps:             b.Taps,
		PageViews:        b.PageViews,
		Links:            b.Links.Links.nodes(),
		HasPreviousLinks: b.PreviousLinks.HasLinks,
	}, nil
}

// InsightsPreviousBioLinks pages through the links removed from the bio.
func (account *Account) InsightsPreviousBioLinks(period InsightsPeriod) *Pager[BioLinkInsights] {
	var after *string
	return newPager(func(ctx context.Context) ([]BioLinkInsights, string, error) {
		var in struct {
			BioLinks struct {
				PreviousLinks bioLinksList `json:"previous_links"`
			} `json:"bio_links"`
		}
		err := account.insights(ctx, docAccountInsightsPreviousBioLinks, friendlyAccountInsightsPrevBioLinks,
			insightsPreviousBioLinksVars{insightsRangeVars: period.vars(), First: previousBioLinksPageSize, After: after}, &in)
		if err != nil {
			return nil, "", err
		}
		conn := in.BioLinks.PreviousLinks.Links
		next := conn.next()
		after = &next
		return conn.nodes(), next, nil
	})
}

// InsightsViewerState loads the dashboard's eligibility state, with the premium
// (Meta Verified) insights when includePremium is set.
func (account *Account) InsightsViewerState(ctx context.Context, period InsightsPeriod, includePremium bool) (*AccountInsightsViewerState, error) {
	return graphQLInto[AccountInsightsViewerState](ctx, account.insta, accountInsightsQuery(docAccountInsightsViewerState, friendlyAccountInsightsViewerState),
		insightsViewerStateVars{Period: period.String(), IncludePremiumInsights: includePremium})
}

// AcceptInsightsTerms accepts the insights terms (Account.ShowInsightsTerms).
func (account *Account) AcceptInsightsTerms(ctx context.Context) error {
	_, err := account.insta.call(ctx, &reqOptions{Endpoint: urlAcceptInsightsTerms, IsPost: true, Signed: account.insta.sessionIDs()})
	return err
}

// OptInStoriesInsightsMonthBack turns on story insights for the past month. The
// app sends it as a GET.
func (account *Account) OptInStoriesInsightsMonthBack(ctx context.Context) error {
	_, err := account.insta.call(ctx, &reqOptions{Endpoint: urlStoriesOptInMonthBack})
	return err
}

// ResetStoriesInsightsOptIn resets the story insights opt-in; a GET too.
func (account *Account) ResetStoriesInsightsOptIn(ctx context.Context) error {
	_, err := account.insta.call(ctx, &reqOptions{Endpoint: urlResetStoriesOptIn})
	return err
}
