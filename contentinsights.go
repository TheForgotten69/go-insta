package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Metric is one labelled insights figure as the app shows it; Value stays nil
// until Instagram has data.
type Metric struct {
	ID             string             `json:"id"`
	Label          string             `json:"label"`
	Format         string             `json:"format"` // COUNT, PERCENTAGE, DURATION_SECONDS
	FormattedValue string             `json:"formatted_value"`
	Value          *float64           `json:"value"`
	Benchmark      *InsightsBenchmark `json:"benchmark"`
}

// ReelInsights are the metrics of a Reel or long-form video.
type ReelInsights struct {
	MediaFBID          string   `json:"media_id"`
	Views              Metric   `json:"total_nullable_views"`
	TotalViews         Metric   `json:"aggregated_total_views"`
	Viewers            Metric   `json:"viewers_or_reach"`
	AvgWatchTime       Metric   `json:"avg_watch_time"`
	Follows            Metric   `json:"aggregated_follows"`
	PerformanceFactors []Metric `json:"performance_factors"` // skip rate and others
	Likes              int      `json:"likes"`
	Comments           int      `json:"comments"`
	Saves              int      `json:"saves"`
	Reposts            int      `json:"reposts"`
	Shares             *int     `json:"shares"`
	FollowersViews     int      `json:"followers_views"`
	NonFollowersViews  int      `json:"non_followers_views"`
	// ViewsOverTime holds all/followers/non_followers series and media_created_at.
	ViewsOverTime json.RawMessage `json:"views_over_time"`
	Demographics  json.RawMessage `json:"demographics"`
	LinkedReels   json.RawMessage `json:"linked_reels"`
	ThumbnailURL  string          `json:"thumbnail_url"`
}

type mediaIGIDVars struct {
	MediaIGID string `json:"mediaIgid"`
}

// ReelInsights loads a Reel's metrics. It runs the app's LongformInsightsQuery,
// which serves Reels; its ReelInsightsQuery needs variables the decompiled app
// does not name.
func (item *Item) ReelInsights(ctx context.Context) (*ReelInsights, error) {
	var in ReelInsights
	err := item.insta.graphQLRoot(ctx, gqlQuery{
		url:      graphQLWWWURL,
		friendly: friendlyLongformInsights,
		docID:    docLongformInsights,
	}, mediaIGIDVars{MediaIGID: item.pk()}, &in)
	if err != nil {
		return nil, err
	}
	return &in, nil
}

// MediaProduct selects the kind of media in MediaInsightsGrid.
type MediaProduct string

const (
	MediaProductFeed  MediaProduct = "FEED"
	MediaProductStory MediaProduct = "STORY"
	MediaProductReels MediaProduct = "CLIPS"
)

type mediaGridQueryParams struct {
	AccessToken string `json:"access_token"`
	ID          string `json:"id"`
}

type mediaGridVars struct {
	QueryParams                  mediaGridQueryParams `json:"query_params"`
	Count                        int                  `json:"count"`
	Timeframe                    string               `json:"timeframe"`
	Metric                       string               `json:"metric"`
	ShouldIncludeInsightsData    bool                 `json:"should_include_insights_data"`
	ShowOnlyIGBoostEligibleMedia bool                 `json:"show_only_ig_boost_eligible_media"`
	MediaProductType             MediaProduct         `json:"media_product_type"`
	Cursor                       string               `json:"cursor,omitempty"`
	CallerID                     string               `json:"caller_id"`
}

// MediaInsightsGrid pages through the account's media of one kind with their
// reach (15 per page); each item is the raw media node. Stories cover the last
// day, other kinds their lifetime.
func (account *Account) MediaInsightsGrid(product MediaProduct) *Pager[json.RawMessage] {
	timeframe := "LIFETIME"
	if product == MediaProductStory {
		timeframe = "ONE_DAY"
	}
	var cursor string
	return newPager(func(ctx context.Context) ([]json.RawMessage, string, error) {
		var page struct {
			BusinessManager struct {
				Medias struct {
					Edges []struct {
						Node json.RawMessage `json:"node"`
					} `json:"edges"`
					PageInfo graphQLPageInfo `json:"page_info"`
				} `json:"medias"`
			} `json:"business_manager"`
		}
		err := account.insta.graphQLRoot(ctx, gqlQuery{
			url:      graphQLWWWURL,
			friendly: friendlyMediaInsightsGrid,
			docID:    docMediaInsightsGrid,
		}, mediaGridVars{
			QueryParams:                  mediaGridQueryParams{ID: strconv.FormatInt(account.insta.userID(), 10)},
			Count:                        15,
			Timeframe:                    timeframe,
			Metric:                       "MEDIA_ORGANIC_ACCOUNT_REACH_OR_PROMOTED_PEOPLE_BASED_REACH",
			ShouldIncludeInsightsData:    true,
			ShowOnlyIGBoostEligibleMedia: true,
			MediaProductType:             product,
			Cursor:                       cursor,
			CallerID:                     "BOOST_MEDIA_PICKER",
		}, &page)
		if err != nil {
			return nil, "", err
		}
		m := page.BusinessManager.Medias
		items := make([]json.RawMessage, len(m.Edges))
		for i, e := range m.Edges {
			items[i] = e.Node
		}
		cursor = m.PageInfo.EndCursor
		return items, cursorIf(m.PageInfo.HasNextPage, cursor), nil
	})
}

type userIDRequestData struct {
	UserID string `json:"user_id"`
}

type commentInsightsOptInVars struct {
	RequestData userIDRequestData `json:"request_data"`
}

type optedInRequestData struct {
	OptedIn bool `json:"opted_in"`
}

type setCommentInsightsOptInVars struct {
	RequestData optedInRequestData `json:"request_data"`
}

// CommentInsightsOptIn reports whether the account opted in to comment summaries.
func (account *Account) CommentInsightsOptIn(ctx context.Context) (bool, error) {
	var resp struct {
		User struct {
			OptedIn bool `json:"comment_summary_opt_in_status"`
		} `json:"user"`
	}
	err := account.insta.graphQLRoot(ctx, gqlQuery{
		friendly: friendlyCommentInsightsOptIn,
		docID:    docCommentInsightsOptIn,
		root:     rootCommentInsightsOptIn,
	}, commentInsightsOptInVars{RequestData: userIDRequestData{UserID: strconv.FormatInt(account.insta.userID(), 10)}}, &resp)
	return resp.User.OptedIn, err
}

func (account *Account) SetCommentInsightsOptIn(ctx context.Context, optedIn bool) error {
	_, err := account.insta.graphQL(ctx, gqlQuery{
		friendly: friendlySetCommentInsightsOptIn,
		docID:    docSetCommentInsightsOptIn,
		root:     rootSetCommentInsightsOptIn,
	}, setCommentInsightsOptInVars{RequestData: optedInRequestData{OptedIn: optedIn}})
	return err
}

type storyViewersQuery struct {
	StoryHasInteractiveStickers string `json:"story_has_interactive_stickers"`
	MaxID                       string `json:"max_id,omitempty"`
	ReelViewerSource            string `json:"reel_viewer_source"`
	ReelID                      string `json:"reel_id"`
}

// StoryViewers pages through the accounts that viewed one of the account's own stories.
func (item *Item) StoryViewers() *Pager[*User] {
	var maxID string
	owner := item.User.ID.String()
	if _, id, ok := strings.Cut(item.ID, "_"); ok {
		owner = id
	}
	return newPager(func(ctx context.Context) ([]*User, string, error) {
		var page struct {
			Users     []*User         `json:"users"`
			NextMaxID json.RawMessage `json:"next_max_id"`
		}
		err := item.insta.callJSON(ctx, &reqOptions{
			Endpoint: fmt.Sprintf(urlStoryViewers, item.pk()),
			Form: storyViewersQuery{
				StoryHasInteractiveStickers: "false",
				MaxID:                       maxID,
				ReelViewerSource:            "unknown",
				ReelID:                      owner,
			},
		}, &page)
		if err != nil {
			return nil, "", err
		}
		maxID = strings.Trim(string(page.NextMaxID), `"`)
		if maxID == "null" {
			maxID = ""
		}
		return item.insta.usersFrom(page.Users, entryContext{}), maxID, nil
	})
}
