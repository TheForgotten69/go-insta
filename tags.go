package goinsta

import (
	"context"
	"fmt"
)

const (
	urlUserTagsFeed  = "usertags/%d/feed/"
	urlUserTagRemove = "usertags/%s/remove/"
)

type userTagsQuery struct {
	Count string `json:"count"`
	MaxID string `json:"max_id,omitempty"`
}

// TaggedPosts pages the posts the user is tagged in.
func (user *User) TaggedPosts() *Pager[*Item] {
	var nextID string
	var pos int
	return newPager(func(ctx context.Context) ([]*Item, string, error) {
		q, err := formOf(userTagsQuery{Count: "12", MaxID: nextID})
		if err != nil {
			return nil, "", err
		}
		var resp struct {
			Items         []*Item `json:"items"`
			MoreAvailable bool    `json:"more_available"`
			NextMaxID     string  `json:"next_max_id"`
		}
		err = user.insta.callJSON(ctx, &reqOptions{
			Endpoint:     fmt.Sprintf(urlUserTagsFeed, user.ID),
			Query:        q,
			ExtraHeaders: map[string]string{"X_IG_FETCH_AAT": "true"},
		}, &resp)
		if err != nil {
			return nil, "", err
		}
		for _, it := range resp.Items {
			setToItem(it, user.insta, "usertags_feed")
			it.Index = pos
			pos++
		}
		nextID = resp.NextMaxID
		return resp.Items, cursorIf(resp.MoreAvailable, nextID), nil
	})
}

type userTagRemoveRequest struct {
	sessionIDs
	MediaID         string `json:"media_id"`
	ContainerModule string `json:"container_module"`
}

// RemoveTag removes the account's own people tag from the post.
func (item *Item) RemoveTag(ctx context.Context) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlUserTagRemove, item.ID), userTagRemoveRequest{
		sessionIDs:      item.insta.sessionIDs(),
		MediaID:         item.ID,
		ContainerModule: "feed_contextual_self_profile",
	}, nil)
	return err
}
