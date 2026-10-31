package goinsta

import (
	"context"
	"fmt"
	"strings"
)

type Collection struct {
	insta *Instagram

	ID         string `json:"collection_id"`
	MediaCount int    `json:"collection_media_count"`
	Name       string `json:"collection_name"`
	Type       string `json:"collection_type"`
	Cover      struct {
		ID             string    `json:"id"`
		Images         Images    `json:"image_versions2"`
		OriginalWidth  int       `json:"original_width"`
		OriginalHeight int       `json:"original_height"`
		MediaType      MediaType `json:"media_type"`
	} `json:"cover_media"`
}

// IsAuto reports whether this is one of Instagram's automatic collections (such
// as the all-media collection), which are read from a different endpoint than a
// user-created one.
func (c *Collection) IsAuto() bool { return strings.HasSuffix(c.Type, "AUTO_COLLECTION") }

// Posts pages the saved media in the collection.
func (c *Collection) Posts() *Pager[*Item] {
	endpoint := fmt.Sprintf(urlCollectionFeed, c.ID)
	if c.IsAuto() {
		endpoint = urlSavedFeed
	}
	var nextID string
	var pos int
	return newPager(func(ctx context.Context) ([]*Item, string, error) {
		q, err := formOf(maxIDQuery{MaxID: nextID})
		if err != nil {
			return nil, "", err
		}
		var resp struct {
			Items []struct {
				Media *Item `json:"media"`
			} `json:"items"`
			MoreAvailable bool   `json:"more_available"`
			NextID        string `json:"next_max_id"`
		}
		if err := c.insta.callJSON(ctx, &reqOptions{Endpoint: endpoint, Query: q}, &resp); err != nil {
			return nil, "", err
		}
		nextID = resp.NextID
		items := make([]*Item, 0, len(resp.Items))
		for _, it := range resp.Items {
			if it.Media != nil {
				setToItem(it.Media, c.insta, "collection_feed")
				it.Media.Index = pos
				pos++
				items = append(items, it.Media)
			}
		}
		return items, cursorIf(resp.MoreAvailable, nextID), nil
	})
}

type maxIDQuery struct {
	MaxID string `json:"max_id,omitempty"`
}

const collectionTypes = `["ALL_MEDIA_AUTO_COLLECTION","PRODUCT_AUTO_COLLECTION","EMPTY_PRODUCT_AUTO_COLLECTION","MEDIA","AUDIO_AUTO_COLLECTION","SERIES_AUTO_COLLECTION","LONGFORM_AUTO_COLLECTION","LOCATIONS_AUTO_COLLECTION","NEW_COLLECTION"]`

type collectionsQuery struct {
	CollectionTypes string `json:"collection_types"`
	MaxID           string `json:"max_id,omitempty"`
}

func (insta *Instagram) Collections() *Pager[*Collection] {
	var nextID string
	return newPager(func(ctx context.Context) ([]*Collection, string, error) {
		query, err := formOf(collectionsQuery{CollectionTypes: collectionTypes, MaxID: nextID})
		if err != nil {
			return nil, "", err
		}
		var resp struct {
			Items         []*Collection `json:"items"`
			MoreAvailable bool          `json:"more_available"`
			NextID        string        `json:"next_max_id"`
		}
		if err := insta.callJSON(ctx, &reqOptions{Endpoint: urlCollectionsList, Query: query}, &resp); err != nil {
			return nil, "", err
		}
		nextID = resp.NextID
		for _, c := range resp.Items {
			c.insta = insta
		}
		return resp.Items, cursorIf(resp.MoreAvailable, nextID), nil
	})
}
