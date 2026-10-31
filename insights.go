package goinsta

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

const (
	docPostInsights      = "187673829817521675190278983686"
	rootPostInsights     = "ig_post_insights"
	friendlyPostInsights = "PostInsightsQuery"
)

// Insights are a post's metrics, as the app's post insights screen shows them;
// captured on a personal account. Shares is nil when Instagram has no count.
type Insights struct {
	MediaFBID              string               `json:"media_id"`
	Likes                  int                  `json:"likes"`
	Comments               int                  `json:"comments"`
	Saves                  int                  `json:"saves"`
	Shares                 *int                 `json:"shares"`
	Reposts                int                  `json:"reposts"`
	TotalViews             int                  `json:"total_views"`
	FollowersViews         int                  `json:"followers_views"`
	NonFollowersViews      int                  `json:"non_followers_views"`
	Demographics           InsightsDemographics `json:"demographics"`
	IsMediaRecentlyCreated bool                 `json:"is_media_recently_created"`
	IsCollab               bool                 `json:"is_collab"`
	CollaboratorUsernames  []string             `json:"collaborator_usernames"`
	ShowBoostCTA           bool                 `json:"show_boost_cta"`
	ThumbnailURL           string               `json:"thumbnail_url"`
}

// InsightsDemographics.Status is "INSUFFICIENT_DATA" until the post has enough
// views for the age, gender and country breakdowns.
type InsightsDemographics struct {
	Status string `json:"status"`
}

type postInsightsVars struct {
	MediaIGID         string `json:"mediaIgid"`
	BloksVersioningID string `json:"bloksVersioningId"`
}

// Insights loads the post's metrics.
func (item *Item) Insights(ctx context.Context) (*Insights, error) {
	body, err := item.insta.graphQL(ctx, gqlQuery{
		url:      graphQLWWWURL,
		friendly: friendlyPostInsights,
		docID:    docPostInsights,
		root:     rootPostInsights,
	}, postInsightsVars{MediaIGID: item.pk(), BloksVersioningID: item.insta.sess.BloksVersionID})
	if err != nil {
		return nil, err
	}
	return parseInsights(body)
}

// parseInsights reads the response; the root key is "ig_post_insights" live and
// aliased ("1$ig_post_insights(bloks_versioning_id:…,media_igid:…)") in the capture.
func parseInsights(body []byte) (*Insights, error) {
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	for k, raw := range resp.Data {
		if k != rootPostInsights && !strings.Contains(k, rootPostInsights+"(") {
			continue
		}
		var in Insights
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
		return &in, nil
	}
	return nil, errors.New(rootPostInsights + ": no insights in response")
}
