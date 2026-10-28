package goinsta

import (
	"context"
	"fmt"
)

// Activity pages the notifications inbox; Items holds the page's new and old
// stories, Counts the badge counts of the last page. The first page marks the
// inbox as seen, as the app does.
type Activity struct {
	*Pager[RecentItems]
	Counts ActivityCounts
}

type ActivityCounts struct {
	Campaign      int `json:"campaign_notification"`
	CommentLikes  int `json:"comment_likes"`
	Comments      int `json:"comments"`
	Fundraiser    int `json:"fundraiser"`
	Likes         int `json:"likes"`
	NewPosts      int `json:"new_posts"`
	PhotosOfYou   int `json:"photos_of_you"`
	Relationships int `json:"relationships"`
	Requests      int `json:"requests"`
	Shopping      int `json:"shopping_notification"`
	UserTags      int `json:"usertags"`
}

type activityPage struct {
	Counts      ActivityCounts `json:"counts"`
	NewStories  []RecentItems  `json:"new_stories"`
	OldStories  []RecentItems  `json:"old_stories"`
	NextID      string         `json:"next_max_id"`
	LastChecked float64        `json:"last_checked"`
	FirstRecTs  float64        `json:"pagination_first_record_timestamp"`
}

type RecentItems struct {
	Type      int `json:"type"`
	StoryType int `json:"story_type"`
	Args      struct {
		Text     string `json:"text"`
		RichText string `json:"rich_text"`
		IconURL  string `json:"icon_url"`
		Links    []struct {
			Start int    `json:"start"`
			End   int    `json:"end"`
			Type  string `json:"type"`
			ID    any    `json:"id"`
		} `json:"links"`
		InlineFollow struct {
			UserInfo        User `json:"user_info"`
			Following       bool `json:"following"`
			OutgoingRequest bool `json:"outgoing_request"`
		} `json:"inline_follow"`
		Actions         []string `json:"actions"`
		AfCandidateID   int      `json:"af_candidate_id"`
		ProfileID       Int64    `json:"profile_id"`
		ProfileImage    string   `json:"profile_image"`
		Timestamp       float64  `json:"timestamp"`
		Tuuid           string   `json:"tuuid"`
		Clicked         bool     `json:"clicked"`
		ProfileName     string   `json:"profile_name"`
		LatestReelMedia Int64    `json:"latest_reel_media"`
		Destination     string   `json:"destination"`
		Extra           any      `json:"extra"`
	} `json:"args"`
	Counts struct{} `json:"counts"`
	Pk     string   `json:"pk"`
}

func (insta *Instagram) Activity() *Activity {
	act := &Activity{}
	var last *activityPage
	act.Pager = newPager(func(ctx context.Context) ([]RecentItems, string, error) {
		query := map[string]string{
			"could_truncate_feed": "true",
			"mark_as_seen":        "false",
			"push_disabled":       "true",
			"should_skip_su":      "false",
			"timezone_name":       insta.sess.TimezoneName,
			"timezone_offset":     insta.sess.TimezoneOffset,
		}
		if last != nil {
			query["max_id"] = last.NextID
			query["last_checked"] = fmt.Sprintf("%f", last.LastChecked)
			query["pagination_first_record_timestamp"] = fmt.Sprintf("%f", last.FirstRecTs)
		}
		var page activityPage
		if err := insta.callJSON(ctx, &reqOptions{Endpoint: urlActivityRecent, Query: query}, &page); err != nil {
			return nil, "", err
		}
		if last == nil {
			if err := insta.MarkActivitySeen(ctx); err != nil {
				return nil, "", err
			}
		}
		last, act.Counts = &page, page.Counts
		return append(page.NewStories, page.OldStories...), page.NextID, nil
	})
	return act
}

func (insta *Instagram) MarkActivitySeen(ctx context.Context) error {
	_, err := insta.call(ctx, &reqOptions{
		Endpoint: urlActivitySeen,
		IsPost:   true,
		Query:    map[string]string{"_uuid": insta.uuid},
	})
	return err
}
