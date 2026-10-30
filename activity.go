package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
)

// Activity pages the notifications inbox; Items holds the page's priority, new
// and old stories, Badges the badge counts of the last page. The first page
// marks the inbox as seen, as the app does.
type Activity struct {
	*Pager[RecentItems]
	Badges ActivityBadges
}

// ActivityBadges decodes badging_info.badge_count_breakdown, a list of
// {badge_use_case_id, count}.
type ActivityBadges struct {
	Total           int
	ActivityFeedDot int
	BrandedContent  int
	Campaign        int
	CommentLikes    int
	Comments        int
	Fundraiser      int
	Likes           int
	MediaToApprove  int
	NewPosts        int
	Promotional     int
	Relationships   int
	Shopping        int
	StoryMentions   int
	UserTags        int
}

func (b *ActivityBadges) UnmarshalJSON(data []byte) error {
	var raw struct {
		Total     int `json:"total_badge_count"`
		Breakdown []struct {
			UseCase string `json:"badge_use_case_id"`
			Count   int    `json:"count"`
		} `json:"badge_count_breakdown"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*b = ActivityBadges{Total: raw.Total}
	fields := map[string]*int{
		"activity_feed_dot_badge": &b.ActivityFeedDot,
		"branded_content":         &b.BrandedContent,
		"campaign_notification":   &b.Campaign,
		"comment_likes":           &b.CommentLikes,
		"comments":                &b.Comments,
		"fundraiser":              &b.Fundraiser,
		"likes":                   &b.Likes,
		"media_to_approve":        &b.MediaToApprove,
		"new_posts":               &b.NewPosts,
		"promotional":             &b.Promotional,
		"relationships":           &b.Relationships,
		"shopping_notification":   &b.Shopping,
		"story_mentions":          &b.StoryMentions,
		"usertags":                &b.UserTags,
	}
	for _, e := range raw.Breakdown {
		if f, ok := fields[e.UseCase]; ok {
			*f = e.Count
		}
	}
	return nil
}

type activityPage struct {
	Badges          ActivityBadges `json:"badging_info"`
	PriorityStories []RecentItems  `json:"priority_stories"`
	NewStories      []RecentItems  `json:"new_stories"`
	OldStories      []RecentItems  `json:"old_stories"`
	NextID          string         `json:"next_max_id"`
	LastChecked     float64        `json:"last_checked"`
	FirstRecTs      float64        `json:"pagination_first_record_timestamp"`
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
			"should_skip_su":      "true",
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
		var items []RecentItems
		if last == nil {
			if err := insta.MarkActivitySeen(ctx); err != nil {
				return nil, "", err
			}
			// Later pages repeat the priority stories.
			items = page.PriorityStories
		}
		last, act.Badges = &page, page.Badges
		items = append(items, page.NewStories...)
		return append(items, page.OldStories...), page.NextID, nil
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
