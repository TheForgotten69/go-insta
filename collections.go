package goinsta

import (
	"encoding/json"
)

type Collections struct {
	insta *Instagram
	err   error

	AutoLoadMoreEnabled bool          `json:"auto_load_more_enabled"`
	Items               []*Collection `json:"items"`
	MoreAvailable       bool          `json:"more_available"`
	NextID              string        `json:"next_max_id"`
	NumResults          int
	Status              string `json:"status"`
}

type Collection struct {
	insta *Instagram
	err   error

	ID         string `json:"collection_id"`
	MediaCount int    `json:"collection_media_count"`
	Name       string `json:"collection_name"`
	Type       string `json:"collection_type"`
	Cover      struct {
		ID             string `json:"id"`
		Images         Images `json:"image_versions2"`
		OriginalWidth  int    `json:"original_width"`
		OriginalHeight int    `json:"original_height"`
		MediaType      int    `json:"media_type"`
	} `json:"cover_media"`

	Items         []Item
	NumResults    int         `json:"num_results"`
	MoreAvailable bool        `json:"more_available"`
	NextID        interface{} `json:"next_max_id"`
}

func newCollections(insta *Instagram) *Collections {
	return &Collections{
		insta: insta,
	}
}

func (c *Collections) Next() bool {
	if c.err != nil {
		return false
	}

	if len(c.Items) > 0 && !c.MoreAvailable {
		return false
	}
	insta := c.insta
	query := map[string]string{
		"collection_types": `["ALL_MEDIA_AUTO_COLLECTION","PRODUCT_AUTO_COLLECTION","EMPTY_PRODUCT_AUTO_COLLECTION","MEDIA","AUDIO_AUTO_COLLECTION","SERIES_AUTO_COLLECTION","LONGFORM_AUTO_COLLECTION","LOCATIONS_AUTO_COLLECTION","NEW_COLLECTION"]`,
	}
	if c.NextID != "" {
		query["max_id"] = c.NextID
	}
	body, _, err := insta.sendRequest(
		&reqOptions{
			Endpoint: urlCollectionsList,
			Query:    query,
		},
	)
	if err != nil {
		c.err = err
		return false
	}
	tmp := Collections{}
	err = json.Unmarshal(body, &tmp)
	if err != nil {
		c.err = err
		return false
	}
	c.MoreAvailable = tmp.MoreAvailable
	c.NextID = tmp.NextID
	c.AutoLoadMoreEnabled = tmp.AutoLoadMoreEnabled
	c.Status = tmp.Status
	c.NumResults = len(tmp.Items)

	for _, i := range tmp.Items {
		i.insta = insta
	}
	c.Items = append(c.Items, tmp.Items...)
	if !c.MoreAvailable {
		c.err = ErrNoMore
	}

	return c.MoreAvailable
}

func (c *Collections) Latest() []*Collection {
	return c.Items[len(c.Items)-c.NumResults:]
}

func (c *Collections) Error() error {
	return c.err
}
