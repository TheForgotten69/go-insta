package goinsta

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (item *Item) containerModule() string {
	if item.module != "" {
		return item.module
	}
	return "feed_timeline"
}

func (item *Item) deliveryClass() string {
	if item.IsCommercial {
		return "ad"
	}
	return "organic"
}

func (item *Item) basePayload() map[string]string {
	insta := item.insta
	return map[string]string{
		"_uid":                    strconv.FormatInt(insta.userID(), 10),
		"_uuid":                   insta.uuid,
		"radio_type":              "ethernet-none",
		"delivery_class":          item.deliveryClass(),
		"is_carousel_bumped_post": "false",
		"floating_context_items":  "[]",
		"mezql_token":             "",
	}
}

func (item *Item) postSigned(endpoint string, payload map[string]string, form map[string]string) ([]byte, error) {
	q, err := signedBody(payload)
	if err != nil {
		return nil, err
	}
	body, _, err := item.insta.sendRequest(&reqOptions{
		Endpoint: endpoint,
		IsPost:   true,
		Query:    MergeMapS(q, form),
	})
	return body, err
}

func (item *Item) Like() error {
	p := MergeMapS(item.basePayload(), map[string]string{
		"media_id":             item.ID,
		"container_module":     item.containerModule(),
		"feed_position":        strconv.Itoa(item.Index),
		"tap_source":           "button",
		"is_carousel_ad":       "false",
		"is_2m_enabled":        "false",
		"is_from_swipe":        "false",
		"like_bubble_user_ids": "",
	})
	if item.InventorySource != "" {
		p["inventory_source"] = item.InventorySource
	}
	_, err := item.postSigned(fmt.Sprintf(urlMediaLike, item.ID), p, map[string]string{"d": "0"})
	return err
}

func (item *Item) Save() error {
	p := MergeMapS(item.basePayload(), map[string]string{
		"module_name":          item.containerModule(),
		"client_position":      strconv.Itoa(item.Index),
		"like_bubble_user_ids": "",
	})
	delete(p, "is_carousel_bumped_post")
	_, err := item.postSigned(fmt.Sprintf(urlMediaSave, item.ID), p, nil)
	return err
}

func (item *Item) Comment(text string) (*Comment, error) {
	key := generateUUID()
	p := MergeMapS(item.basePayload(), map[string]string{
		"comment_text":                    text,
		"comment_creation_key":            key,
		"idempotence_token":               key,
		"user_breadcrumb":                 userBreadcrumb(text),
		"container_module":                "comments_v2_" + item.containerModule(),
		"carousel_index":                  "-1",
		"carousel_child_mentions":         "[]",
		"include_carousel_child_mentions": "false",
		"is_from_carousel_child_thread":   "false",
		"is_text_app_xpost_attempt":       "false",
		"feed_position":                   strconv.Itoa(item.Index),
		"recs_ix":                         "-1",
		"chaining_seed_media_id":          "null",
		"starting_clips_media_id":         "null",
		"ranking_session_id":              generateUUID(),
	})
	if item.InventorySource != "" {
		p["inventory_source"] = item.InventorySource
	}
	body, err := item.postSigned(fmt.Sprintf(urlCommentAdd, item.ID), p, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Comment Comment `json:"comment"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	resp.Comment.setValues(item.insta)
	resp.Comment.item = item
	return &resp.Comment, nil
}

// Comments fetches the first page of comments (stream_comments); page on with Comments.Next.
func (item *Item) Comments() (*Comments, error) {
	c := &Comments{item: item}
	return c, c.fetch("")
}

func (c *Comments) fetch(minID string) error {
	item := c.item
	q := map[string]string{
		"can_support_threading":              "true",
		"feed_position":                      strconv.Itoa(item.Index),
		"is_carousel_bumped_post":            "false",
		"should_fetch_creator_comment_nudge": "true",
	}
	if minID != "" {
		q["min_id"] = minID
		q["sort_order"] = "popular"
	}
	body, _, err := item.insta.sendRequest(&reqOptions{
		Endpoint: fmt.Sprintf(urlStreamComments, item.ID),
		Stream:   true,
		Query:    q,
	})
	if err != nil {
		return err
	}
	page := Comments{item: item}
	if err := json.Unmarshal(body, &page); err != nil {
		return err
	}
	for i := range page.Items {
		page.Items[i].item = item
		page.Items[i].setValues(item.insta)
	}
	*c = page
	return nil
}

// Next loads the following page in place; it returns ErrNoMore at the end.
func (c *Comments) Next() error {
	if !c.HasMoreHeadloadComments || c.NextMinID == "" {
		return ErrNoMore
	}
	return c.fetch(c.NextMinID)
}

func (c *Comment) likePayload() (map[string]string, error) {
	if c.item == nil {
		return nil, fmt.Errorf("comment %v has no parent media", c.ID)
	}
	it := c.item
	p := map[string]string{
		"_uid":                    strconv.FormatInt(it.insta.userID(), 10),
		"_uuid":                   it.insta.uuid,
		"container_module":        "comments_v2_" + it.containerModule(),
		"delivery_class":          it.deliveryClass(),
		"feed_position":           strconv.Itoa(it.Index),
		"is_carousel_bumped_post": "false",
		"is_feed_preview_comment": "false",
		"is_reply_highlight":      "false",
	}
	if it.InventorySource != "" {
		p["inventory_source"] = it.InventorySource
	}
	return p, nil
}

func (c *Comment) Like() error {
	p, err := c.likePayload()
	if err != nil {
		return err
	}
	_, err = c.item.postSigned(fmt.Sprintf(urlCommentLike, c.getid()), p, nil)
	return err
}

func (c *Comment) Unlike() error {
	p, err := c.likePayload()
	if err != nil {
		return err
	}
	p["m_ix"] = strconv.Itoa(c.item.Index)
	_, err = c.item.postSigned(fmt.Sprintf(urlCommentUnlike, c.getid()), p, nil)
	return err
}

// Replies loads the comment's thread (inline_child_comments).
func (c *Comment) Replies() ([]Comment, error) {
	if c.item == nil {
		return nil, fmt.Errorf("comment %v has no parent media", c.ID)
	}
	body, _, err := c.item.insta.sendRequest(&reqOptions{
		Endpoint: fmt.Sprintf(urlInlineChildComments, c.item.ID, c.getid()),
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		ChildComments []Comment `json:"child_comments"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	for i := range resp.ChildComments {
		resp.ChildComments[i].item = c.item
		resp.ChildComments[i].setValues(c.item.insta)
	}
	return resp.ChildComments, nil
}

func (c *Comment) Delete() error {
	if c.item == nil {
		return fmt.Errorf("comment %v has no parent media", c.ID)
	}
	return c.item.DeleteComments(c.getid())
}

// DeleteComments removes comments by id (comment/bulk_delete).
func (item *Item) DeleteComments(ids ...string) error {
	_, err := item.postSigned(fmt.Sprintf(urlCommentBulkDelete, item.ID), map[string]string{
		"_uid":                   strconv.FormatInt(item.insta.userID(), 10),
		"_uuid":                  item.insta.uuid,
		"comment_ids_to_delete":  strings.Join(ids, ","),
		"container_module":       "self_comments_v2_feed_contextual_self_profile",
		"client_position":        strconv.Itoa(item.Index),
		"chaining_seed_media_id": item.pk(),
		"mezql_token":            "",
	}, nil)
	return err
}

// pk is the numeric media id; ID is "<pk>_<owner id>".
func (item *Item) pk() string {
	if item.Pk != 0 {
		return item.Pk.String()
	}
	pk, _, _ := strings.Cut(item.ID, "_")
	return pk
}

func (item *Item) mediaTypeName() string {
	switch item.MediaType {
	case 2:
		return "VIDEO"
	case 8:
		return "CAROUSEL"
	default:
		return "PHOTO"
	}
}

// Delete removes one of the account's own posts.
func (item *Item) Delete() error {
	q, err := signedBody(map[string]string{
		"_uid":              strconv.FormatInt(item.insta.userID(), 10),
		"_uuid":             item.insta.uuid,
		"igtv_feed_preview": "false",
		"media_id":          item.ID,
	})
	if err != nil {
		return err
	}
	_, _, err = item.insta.sendRequest(&reqOptions{
		Endpoint:     fmt.Sprintf(urlMediaDelete, item.ID) + "?media_type=" + item.mediaTypeName(),
		FriendlyName: "IgApi: " + fmt.Sprintf(urlMediaDelete, item.ID) + "?media_type=" + item.mediaTypeName(),
		IsPost:       true,
		Query:        q,
	})
	return err
}

// Archive hides one of the account's own posts from the profile (only_me).
func (item *Item) Archive() error {
	_, err := item.postSigned(fmt.Sprintf(urlMediaOnlyMe, item.ID), map[string]string{
		"_uid":     strconv.FormatInt(item.insta.userID(), 10),
		"_uuid":    item.insta.uuid,
		"media_id": item.ID,
	}, nil)
	return err
}

// Pin pins one of the account's own posts to the top of the profile grid.
func (item *Item) Pin() error {
	_, _, err := item.insta.sendRequest(&reqOptions{
		Endpoint: urlPinTimelineMedia,
		IsPost:   true,
		Query:    map[string]string{"_uuid": item.insta.uuid, "post_id": item.pk()},
	})
	return err
}

type Note struct {
	ID        string `json:"id"`
	MediaID   string `json:"media_id"`
	Text      string `json:"text"`
	UserID    string `json:"user_id"`
	Audience  int    `json:"audience"`
	NoteStyle int    `json:"note_style"`
	CreatedAt Int64  `json:"created_at"`
	ExpiresAt Int64  `json:"expires_at"`
}

// AddNote leaves a note on the media (the bubble shown over reels/posts).
func (item *Item) AddNote(text string) (*Note, error) {
	body, _, err := item.insta.sendRequest(&reqOptions{
		Endpoint:     urlCreateNote,
		FriendlyName: "IgApi: api/v1/media/create_note/v2/",
		IsPost:       true,
		Query: map[string]string{
			"_uuid":                 item.insta.uuid,
			"audience":              "7",
			"carousel_index":        "-1",
			"container_module":      item.containerModule(),
			"event_source":          "ufi",
			"media_client_position": strconv.Itoa(item.Index),
			"media_id":              item.ID,
			"note_style":            "13",
			"text":                  text,
		},
	})
	if err != nil {
		return nil, err
	}
	var n Note
	return &n, json.Unmarshal(body, &n)
}
