package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	urlStoryViewers = "media/%s/list_reel_media_viewer/"

	docReelInsights      = "34117726174753558798011202277"
	rootReelInsights     = "ig_reel_insights"
	friendlyReelInsights = "ReelInsightsQuery"

	docLongformInsights      = "142319532318085841412086369996"
	friendlyLongformInsights = "LongformInsightsQuery"

	docMediaInsightsGrid      = "13476930769765169094605715723"
	friendlyMediaInsightsGrid = "IGInsightsIntegratedMediaGridQuery"

	docCommentInsightsOptIn         = "1221062957381201146258962681"
	rootCommentInsightsOptIn        = "xdt_get_comment_summary_opt_in_status_by_user_id"
	friendlyCommentInsightsOptIn    = "IGCommentInsightsOptinStatusQuery"
	docSetCommentInsightsOptIn      = "34077348003540024418408340205"
	rootSetCommentInsightsOptIn     = "xdt_update_comment_summary_opt_in_by_user_id"
	friendlySetCommentInsightsOptIn = "IGCommentInsightsOptinStatusMutation"
)

// ReelInsights are a Reel's metrics. The fields shared with posts are decoded;
// Raw holds the whole ig_reel_insights object.
type ReelInsights struct {
	Insights
	Raw json.RawMessage `json:"-"`
}

// ReelInsights loads the Reel's metrics; same variables as the post query.
func (item *Item) ReelInsights(ctx context.Context) (*ReelInsights, error) {
	var raw json.RawMessage
	err := item.insta.graphQLRoot(ctx, gqlQuery{
		url:      graphQLWWWURL,
		friendly: friendlyReelInsights,
		docID:    docReelInsights,
		root:     rootReelInsights,
	}, postInsightsVars{MediaIGID: item.pk(), BloksVersioningID: item.insta.sess.BloksVersionID}, &raw)
	if err != nil {
		return nil, err
	}
	in := &ReelInsights{Raw: raw}
	if err := json.Unmarshal(raw, &in.Insights); err != nil {
		return nil, err
	}
	return in, nil
}

type mediaIGIDVars struct {
	MediaIGID string `json:"mediaIgid"`
}

// LongformInsights loads the metrics of a long-form video as the raw root object.
func (item *Item) LongformInsights(ctx context.Context) (json.RawMessage, error) {
	return item.insta.graphQLOnlyRoot(ctx, gqlQuery{
		url:      graphQLWWWURL,
		friendly: friendlyLongformInsights,
		docID:    docLongformInsights,
	}, mediaIGIDVars{MediaIGID: item.pk()})
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

// MediaInsightsGrid loads one page (15 items) of the account's media of one kind
// with their reach, as the raw root object; cursor is "" for the first page.
// Stories cover the last day, other kinds their lifetime.
func (account *Account) MediaInsightsGrid(ctx context.Context, product MediaProduct, cursor string) (json.RawMessage, error) {
	timeframe := "LIFETIME"
	if product == MediaProductStory {
		timeframe = "ONE_DAY"
	}
	return account.insta.graphQLOnlyRoot(ctx, gqlQuery{
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

// graphQLOnlyRoot runs a query whose root field name is not known and returns the
// response's single data field.
func (insta *Instagram) graphQLOnlyRoot(ctx context.Context, gq gqlQuery, vars any) (json.RawMessage, error) {
	body, err := insta.graphQL(ctx, gq, vars)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	for k, v := range resp.Data {
		if k != "__typename" && string(v) != "null" {
			return v, nil
		}
	}
	return nil, fmt.Errorf("%s: %w", gq.friendly, ErrEmptyResponse)
}
