package goinsta

import "context"

type Collection struct {
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

func (insta *Instagram) Collections() *Pager[*Collection] {
	var nextID string
	return newPager(func(ctx context.Context) ([]*Collection, bool, error) {
		query := map[string]string{
			"collection_types": `["ALL_MEDIA_AUTO_COLLECTION","PRODUCT_AUTO_COLLECTION","EMPTY_PRODUCT_AUTO_COLLECTION","MEDIA","AUDIO_AUTO_COLLECTION","SERIES_AUTO_COLLECTION","LONGFORM_AUTO_COLLECTION","LOCATIONS_AUTO_COLLECTION","NEW_COLLECTION"]`,
		}
		if nextID != "" {
			query["max_id"] = nextID
		}
		var resp struct {
			Items         []*Collection `json:"items"`
			MoreAvailable bool          `json:"more_available"`
			NextID        string        `json:"next_max_id"`
		}
		if err := insta.callJSON(ctx, &reqOptions{Endpoint: urlCollectionsList, Query: query}, &resp); err != nil {
			return nil, false, err
		}
		nextID = resp.NextID
		return resp.Items, resp.MoreAvailable && nextID != "", nil
	})
}
