package goinsta

import (
	"encoding/json"
	"testing"
)

func TestItemDecodesUsertagsAndTopLikers(t *testing.T) {
	var it Item
	body := `{"usertags":{"in":[{"position":[0.5,0.4],"user":{"pk":"7842327716","username":"a"}}]},"top_likers":["k.mbappe"]}`
	if err := json.Unmarshal([]byte(body), &it); err != nil {
		t.Fatal(err)
	}
	if len(it.Tags.In) != 1 || it.Tags.In[0].User.ID != 7842327716 {
		t.Errorf("usertags = %+v", it.Tags)
	}
	if got := it.TopLikers(); len(got) != 1 || got[0] != "k.mbappe" {
		t.Errorf("top likers = %v", got)
	}
}

func TestUserFallsBackToID(t *testing.T) {
	var u User
	if err := json.Unmarshal([]byte(`{"id":7842327716,"username":"a","fbid_v2":"17841442747228158","can_boost_post":true}`), &u); err != nil {
		t.Fatal(err)
	}
	if u.ID != 7842327716 || u.Username != "a" || u.FbIdV2 != 17841442747228158 || !u.CanBoostPosts {
		t.Errorf("user = %d %q %d %v", u.ID, u.Username, u.FbIdV2, u.CanBoostPosts)
	}
	if err := json.Unmarshal([]byte(`{"pk":1,"id":"1_2"}`), &u); err == nil && u.ID != 1 {
		t.Errorf("pk must win over id: %d", u.ID)
	}
}

func TestActivityPageDecodesBadgesAndPriority(t *testing.T) {
	var p activityPage
	body := `{"badging_info":{"total_badge_count":3,"badge_count_breakdown":[{"badge_use_case_id":"activity_feed_dot_badge","count":1},{"badge_use_case_id":"likes","count":2},{"badge_use_case_id":"unknown_case","count":9}]},
		"priority_stories":[{"story_type":459,"args":{"inline_follow":{"user_info":{"id":7842327716}}}}]}`
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatal(err)
	}
	if p.Badges != (ActivityBadges{Total: 3, ActivityFeedDot: 1, Likes: 2}) {
		t.Errorf("badges = %+v", p.Badges)
	}
	if len(p.PriorityStories) != 1 || p.PriorityStories[0].Args.InlineFollow.UserInfo.ID != 7842327716 {
		t.Errorf("priority stories = %+v", p.PriorityStories)
	}
}
