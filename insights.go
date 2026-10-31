package goinsta

import (
	"context"
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
	var in Insights
	err := item.insta.graphQLRoot(ctx, gqlQuery{
		url:      graphQLWWWURL,
		friendly: friendlyPostInsights,
		docID:    docPostInsights,
		root:     rootPostInsights,
	}, postInsightsVars{MediaIGID: item.pk(), BloksVersioningID: item.insta.sess.BloksVersionID}, &in)
	if err != nil {
		return nil, err
	}
	return &in, nil
}
