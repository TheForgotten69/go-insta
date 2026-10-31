package goinsta

import (
	"context"
	"fmt"
)

type userTagsQuery struct {
	Count string `json:"count"`
	maxIDQuery
}

// TaggedPosts pages the posts the user is tagged in.
func (user *User) TaggedPosts() *Pager[*Item] {
	var nextID string
	var pos int
	return newPager(func(ctx context.Context) ([]*Item, string, error) {
		var resp struct {
			Items         []*Item `json:"items"`
			MoreAvailable bool    `json:"more_available"`
			NextMaxID     string  `json:"next_max_id"`
		}
		err := user.insta.callJSON(ctx, &reqOptions{
			Endpoint:     fmt.Sprintf(urlUserTagsFeed, user.ID),
			Form:         userTagsQuery{Count: "12", maxIDQuery: maxIDQuery{MaxID: nextID}},
			ExtraHeaders: map[string]string{"X_IG_FETCH_AAT": "true"},
		}, &resp)
		if err != nil {
			return nil, "", err
		}
		nextID = resp.NextMaxID
		return numberItems(resp.Items, user.insta, "usertags_feed", &pos), cursorIf(resp.MoreAvailable, nextID), nil
	})
}

type userTagRemoveRequest struct {
	sessionIDs
	MediaID         string `json:"media_id"`
	ContainerModule string `json:"container_module"`
}

// RemoveTag removes the account's own people tag from the post.
func (item *Item) RemoveTag(ctx context.Context) error {
	_, err := item.insta.call(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlUserTagRemove, item.ID), IsPost: true, Signed: userTagRemoveRequest{
		sessionIDs:      item.insta.sessionIDs(),
		MediaID:         item.ID,
		ContainerModule: "feed_contextual_self_profile",
	}})
	return err
}
