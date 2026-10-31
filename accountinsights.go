package goinsta

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Account insights dashboard (the app's "Insights" screen). Doc ids from
// ig4a-instagram-schema_client_persist_ids.json; all "instagram" schema, so graphql_www.
const (
	urlAcceptInsightsTerms   = "users/accept_insights_terms/"
	urlStoriesOptInMonthBack = "insights/stories_opt_in_month_back/"
	urlResetStoriesOptIn     = "insights/reset_stories_opt_in/"

	rootAccountInsights = "ig_account_insights"

	docAccountInsightsHome              = "38444011412098991622225483328"
	docAccountInsightsOverview          = "297851856213787890962497531867"
	docAccountInsightsAudience          = "11313478377027549324232575805"
	docAccountInsightsContent           = "246019450711285746558777381992"
	docAccountInsightsMediaComponent    = "348470172667229184692559590"
	docAccountInsightsBioLinks          = "241501507416321773369365046568"
	docAccountInsightsPreviousBioLinks  = "237951042513594934414540638102"
	docAccountInsightsViewerState       = "16422176244090456288187318494"
	docActionableInsightsTipBanner      = "107424094814803234520933019235"
	friendlyAccountInsightsHome         = "AccountInsightsHomeQuery"
	friendlyAccountInsightsOverview     = "AccountInsightsOverviewQuery"
	friendlyAccountInsightsAudience     = "AccountInsightsAudienceQuery"
	friendlyAccountInsightsContent      = "AccountInsightsContentQuery"
	friendlyAccountInsightsMedia        = "AccountInsightsMediaComponentQuery"
	friendlyAccountInsightsBioLinks     = "AccountInsightsBioLinksQuery"
	friendlyAccountInsightsPrevBioLinks = "AccountInsightsPreviousBioLinksQuery"
	friendlyAccountInsightsViewerState  = "AccountInsightsViewerStateQuery"
	friendlyActionableInsightsTipBanner = "IGActionableInsightsTipBannerQuery"

	metaOnePacingCreator = "IG_COMPETITIVE_INSIGHTS_CREATOR_DASH_PERMANENT_UPSELL"
	metaOnePacingPro     = "IG_COMPETITIVE_INSIGHTS_PRO_DASH_PERMANENT_UPSELL"

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
	return InsightsPeriod{name: "CUSTOM", start: start.Unix(), end: end.Unix()}
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
	if p.name == "CUSTOM" {
		v.StartDate, v.EndDate = &p.start, &p.end
	}
	return v
}

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

type insightsBloksVars struct {
	insightsRangeVars
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

type insightsTipBannerVars struct{}

// InsightsHomeOptions are the home tab's inputs. The app sets the include flags
// from mobile config; InsightsHome turns them all on.
type InsightsHomeOptions struct {
	Period InsightsPeriod
	// Creator picks the creator dashboard's Meta One upsell key; the app decides
	// it from the account type (creator vs business).
	Creator bool
}

// AccountInsightsHome is the home tab. Raw is the whole data object: the
// next-steps cards and the Meta One subtitle come in root fields the app reads
// by hashed name only.
type AccountInsightsHome struct {
	Layout    string          `json:"layout"`
	Tiles     []InsightsTile  `json:"tiles"`
	Tools     []InsightsTool  `json:"tools"`
	BloksTree string          `json:"-"` // the dashboard's Bloks UI bundle
	Raw       json.RawMessage `json:"-"`
}

// InsightsTile is a summary metric (views, interactions, followers, …).
type InsightsTile struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Metric     InsightsValue   `json:"metric"`
	Delta      InsightsDelta   `json:"delta"`
	TimeSeries json.RawMessage `json:"time_series"` // {data_points: [...]}
}

// InsightsValue is a metric value; FormattedValue and Benchmark are set where the
// screen shows them.
type InsightsValue struct {
	Value          *float64           `json:"value"`
	FormattedValue string             `json:"formatted_value"`
	Benchmark      *InsightsBenchmark `json:"benchmark"`
}

// InsightsDelta is the change over the previous period, as display text.
type InsightsDelta struct {
	Value     string `json:"value"`
	Sentiment string `json:"sentiment"`
}

// InsightsBenchmark compares a value with similar accounts.
type InsightsBenchmark struct {
	Value         float64 `json:"value"`
	Comparison    string  `json:"comparison"`
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

// ContentInsightsOptions are the content tab's filters.
type ContentInsightsOptions struct {
	Period             InsightsPeriod
	Metric             InsightsMetric      // "" = MetricViews
	Type               InsightsContentType // "" = all
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
	IsCollab     bool   `json:"is_collab"`
}

type InsightsMetricValue struct {
	Metric string   `json:"metric"`
	Value  *float64 `json:"value"`
}

// AccountInsightsBioLinks are the link taps on the profile's bio links.
type AccountInsightsBioLinks struct {
	Taps             *InsightsValue
	PageViews        *InsightsValue
	Links            []BioLinkInsights
	HasPreviousLinks bool
	Raw              json.RawMessage
}

// BioLinkInsights is one bio link and its taps.
type BioLinkInsights struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	DisplayURL string        `json:"display_url"`
	ImageURL   string        `json:"image_url"`
	Taps       InsightsValue `json:"taps"`
}

// AccountInsightsViewerState is the dashboard's eligibility state; Raw holds the
// whole ig_account_insights object, whose other fields the app reads by hashed name.
type AccountInsightsViewerState struct {
	BrandName string
	Raw       json.RawMessage
}

type graphQLPageInfo struct {
	EndCursor   string `json:"end_cursor"`
	HasNextPage bool   `json:"has_next_page"`
}

type bioLinkConnection struct {
	Edges    []bioLinkEdge   `json:"edges"`
	PageInfo graphQLPageInfo `json:"page_info"`
}

type bioLinkEdge struct {
	Node BioLinkInsights `json:"node"`
}

func (c bioLinkConnection) links() []BioLinkInsights {
	out := make([]BioLinkInsights, 0, len(c.Edges))
	for _, e := range c.Edges {
		out = append(out, e.Node)
	}
	return out
}

type bioLinksList struct {
	Links    bioLinkConnection `json:"links"`
	HasLinks bool              `json:"has_links"`
}

type bloksBundle struct {
	Tree string `json:"bloks_bundle_tree"`
}

type bloksComponent struct {
	Bundle bloksBundle `json:"bundle"`
}

// accountInsightsData runs an account insights query and returns its data object.
// A null or missing root comes back as the server's *GraphQLError when the
// response carries one (a personal account is expected to get one), else
// ErrEmptyResponse.
func (insta *Instagram) accountInsightsData(ctx context.Context, gq gqlQuery, vars any) (map[string]json.RawMessage, error) {
	gq.url = graphQLWWWURL
	body, err := insta.graphQL(ctx, gq, vars)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []GraphQLError             `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if gq.root == "" {
		if hasData(resp.Data) {
			return resp.Data, nil
		}
	} else if raw, ok := rootField(resp.Data, gq.root); ok && string(raw) != "null" {
		return resp.Data, nil
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("%s: %w", gq.friendly, &resp.Errors[0])
	}
	return nil, fmt.Errorf("%s: %w", gq.friendly, ErrEmptyResponse)
}

func hasData(data map[string]json.RawMessage) bool {
	for _, v := range data {
		if string(v) != "null" {
			return true
		}
	}
	return false
}

// accountInsightsRoot returns the ig_account_insights object.
func (insta *Instagram) accountInsightsRoot(ctx context.Context, docID, friendly string, vars any) (json.RawMessage, error) {
	data, err := insta.accountInsightsData(ctx, gqlQuery{friendly: friendly, docID: docID, root: rootAccountInsights}, vars)
	if err != nil {
		return nil, err
	}
	raw, _ := rootField(data, rootAccountInsights)
	return raw, nil
}

// childWith returns the first child object of obj that has key. The app reads
// several insights fields only by name hash, so their names are unknown.
func childWith(obj json.RawMessage, key string) json.RawMessage {
	var fields map[string]json.RawMessage
	if json.Unmarshal(obj, &fields) != nil {
		return nil
	}
	for _, v := range fields {
		var child map[string]json.RawMessage
		if json.Unmarshal(v, &child) != nil {
			continue
		}
		if c, ok := child[key]; ok && !bytes.Equal(c, []byte("null")) {
			return v
		}
	}
	return nil
}

// bloksTreeIn returns the Bloks bundle of obj's child {component: {bundle: ...}}.
func bloksTreeIn(obj json.RawMessage) string {
	var holder struct {
		Component bloksComponent `json:"component"`
	}
	if c := childWith(obj, "component"); c != nil && json.Unmarshal(c, &holder) == nil {
		return holder.Component.Bundle.Tree
	}
	return ""
}

func (account *Account) bloksID() string { return account.insta.sess.BloksVersionID }

// InsightsHome loads the dashboard's home tab. Insights need a professional
// account; a personal one gets the server's error (a *GraphQLError when sent).
func (account *Account) InsightsHome(ctx context.Context, opts InsightsHomeOptions) (*AccountInsightsHome, error) {
	key := metaOnePacingPro
	if opts.Creator {
		key = metaOnePacingCreator
	}
	data, err := account.insta.accountInsightsData(ctx, gqlQuery{
		friendly: friendlyAccountInsightsHome, docID: docAccountInsightsHome, root: rootAccountInsights,
	}, insightsHomeVars{
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
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	root, _ := rootField(data, rootAccountInsights)
	home := AccountInsightsHome{Raw: raw, BloksTree: bloksTreeIn(root)}
	if err := json.Unmarshal(root, &home); err != nil {
		return nil, err
	}
	if summary := childWith(root, "tiles"); summary != nil {
		if err := json.Unmarshal(summary, &home); err != nil {
			return nil, err
		}
	}
	return &home, nil
}

// InsightsOverview loads the overview tab, undecoded. Unverified: no app code
// building this query was found, so the variables are the ones the other tabs share.
func (account *Account) InsightsOverview(ctx context.Context, period InsightsPeriod) (json.RawMessage, error) {
	return account.insta.accountInsightsRoot(ctx, docAccountInsightsOverview, friendlyAccountInsightsOverview,
		insightsBloksVars{insightsRangeVars: period.vars(), BloksVersioningID: account.bloksID()})
}

// InsightsAudience loads the audience (followers) tab, undecoded. Unverified like
// InsightsOverview.
func (account *Account) InsightsAudience(ctx context.Context, period InsightsPeriod) (json.RawMessage, error) {
	return account.insta.accountInsightsRoot(ctx, docAccountInsightsAudience, friendlyAccountInsightsAudience,
		insightsBloksVars{insightsRangeVars: period.vars(), BloksVersioningID: account.bloksID()})
}

// InsightsContent pages through the content tab, 24 items a page, best first.
func (account *Account) InsightsContent(opts ContentInsightsOptions) *Pager[ContentInsightsItem] {
	metric := string(cmp.Or(opts.Metric, MetricViews))
	var contentType *string
	if opts.Type != "" {
		t := string(opts.Type)
		contentType = &t
	}
	var after *string
	return newPager(func(ctx context.Context) ([]ContentInsightsItem, string, error) {
		root, err := account.insta.accountInsightsRoot(ctx, docAccountInsightsContent, friendlyAccountInsightsContent, insightsContentVars{
			insightsRangeVars:  opts.Period.vars(),
			Metric:             metric,
			SortByCreationTime: opts.SortByCreationTime,
			Count:              contentInsightsPageSize,
			After:              after,
			ContentType:        contentType,
			SortDescending:     !opts.Ascending,
			BloksVersioningID:  account.bloksID(),
		})
		if err != nil {
			return nil, "", err
		}
		var page struct {
			YourContent struct {
				Items    []ContentInsightsItem `json:"items"`
				PageInfo graphQLPageInfo       `json:"page_info"`
			} `json:"your_content"`
		}
		if err := json.Unmarshal(root, &page); err != nil {
			return nil, "", err
		}
		pi := page.YourContent.PageInfo
		next := cursorIf(pi.HasNextPage, pi.EndCursor)
		after = &next
		return page.YourContent.Items, next, nil
	})
}

// InsightsMediaComponent returns the Bloks UI bundle of one post's insights
// carousel card. mediaID is the post's pk.
func (account *Account) InsightsMediaComponent(ctx context.Context, mediaID string) (string, error) {
	root, err := account.insta.accountInsightsRoot(ctx, docAccountInsightsMediaComponent, friendlyAccountInsightsMedia,
		insightsMediaComponentVars{MediaID: mediaID, BloksVersioningID: account.bloksID()})
	if err != nil {
		return "", err
	}
	tree := bloksTreeIn(root)
	if tree == "" {
		return "", fmt.Errorf("%s: %w", friendlyAccountInsightsMedia, ErrEmptyResponse)
	}
	return tree, nil
}

// InsightsBioLinks loads the bio link taps for the period.
func (account *Account) InsightsBioLinks(ctx context.Context, period InsightsPeriod) (*AccountInsightsBioLinks, error) {
	root, err := account.insta.accountInsightsRoot(ctx, docAccountInsightsBioLinks, friendlyAccountInsightsBioLinks, period.vars())
	if err != nil {
		return nil, err
	}
	var resp struct {
		BioLinks struct {
			Taps          *InsightsValue `json:"taps"`
			PageViews     *InsightsValue `json:"page_views"`
			Links         bioLinksList   `json:"links"`
			PreviousLinks bioLinksList   `json:"previous_links"`
		} `json:"bio_links"`
	}
	if err := json.Unmarshal(root, &resp); err != nil {
		return nil, err
	}
	b := resp.BioLinks
	return &AccountInsightsBioLinks{
		Taps:             b.Taps,
		PageViews:        b.PageViews,
		Links:            b.Links.Links.links(),
		HasPreviousLinks: b.PreviousLinks.HasLinks,
		Raw:              root,
	}, nil
}

// InsightsPreviousBioLinks pages through the links removed from the bio, 20 a page.
func (account *Account) InsightsPreviousBioLinks(period InsightsPeriod) *Pager[BioLinkInsights] {
	var after *string
	return newPager(func(ctx context.Context) ([]BioLinkInsights, string, error) {
		root, err := account.insta.accountInsightsRoot(ctx, docAccountInsightsPreviousBioLinks, friendlyAccountInsightsPrevBioLinks,
			insightsPreviousBioLinksVars{insightsRangeVars: period.vars(), First: previousBioLinksPageSize, After: after})
		if err != nil {
			return nil, "", err
		}
		var resp struct {
			BioLinks struct {
				PreviousLinks bioLinksList `json:"previous_links"`
			} `json:"bio_links"`
		}
		if err := json.Unmarshal(root, &resp); err != nil {
			return nil, "", err
		}
		conn := resp.BioLinks.PreviousLinks.Links
		next := cursorIf(conn.PageInfo.HasNextPage, conn.PageInfo.EndCursor)
		after = &next
		return conn.links(), next, nil
	})
}

// InsightsViewerState loads what the dashboard shows the viewer: its premium
// (Meta Verified) insights state and copy.
func (account *Account) InsightsViewerState(ctx context.Context, period InsightsPeriod, includePremium bool) (*AccountInsightsViewerState, error) {
	root, err := account.insta.accountInsightsRoot(ctx, docAccountInsightsViewerState, friendlyAccountInsightsViewerState,
		insightsViewerStateVars{Period: period.String(), IncludePremiumInsights: includePremium})
	if err != nil {
		return nil, err
	}
	state := AccountInsightsViewerState{Raw: root}
	var premium struct {
		Strings struct {
			BrandName string `json:"brand_name"`
		} `json:"strings"`
	}
	if p := childWith(root, "strings"); p != nil && json.Unmarshal(p, &premium) == nil {
		state.BrandName = premium.Strings.BrandName
	}
	return &state, nil
}

// InsightsTipBanner loads the actionable-insights tip banner, undecoded.
// Unverified: the app's build of this query was not found, so it is sent without
// variables and its root field is unknown.
func (account *Account) InsightsTipBanner(ctx context.Context) (json.RawMessage, error) {
	data, err := account.insta.accountInsightsData(ctx, gqlQuery{
		friendly: friendlyActionableInsightsTipBanner, docID: docActionableInsightsTipBanner,
	}, insightsTipBannerVars{})
	if err != nil {
		return nil, err
	}
	return json.Marshal(data)
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
