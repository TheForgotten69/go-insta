package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
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

func (item *Item) postSigned(ctx context.Context, endpoint string, payload map[string]string, form map[string]string) ([]byte, error) {
	q, err := signedBody(payload)
	if err != nil {
		return nil, err
	}
	return item.insta.call(ctx, &reqOptions{
		Endpoint: endpoint,
		IsPost:   true,
		Query:    mergeForms(q, form),
	})
}

func (item *Item) Like(ctx context.Context) error {
	p := mergeForms(item.basePayload(), map[string]string{
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
	_, err := item.postSigned(ctx, fmt.Sprintf(urlMediaLike, item.ID), p, map[string]string{"d": "0"})
	return err
}

func (item *Item) Save(ctx context.Context) error {
	p := mergeForms(item.basePayload(), map[string]string{
		"module_name":          item.containerModule(),
		"client_position":      strconv.Itoa(item.Index),
		"like_bubble_user_ids": "",
	})
	delete(p, "is_carousel_bumped_post")
	_, err := item.postSigned(ctx, fmt.Sprintf(urlMediaSave, item.ID), p, nil)
	return err
}

func (item *Item) Comment(ctx context.Context, text string) (*Comment, error) {
	key := generateUUID()
	p := mergeForms(item.basePayload(), map[string]string{
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
	body, err := item.postSigned(ctx, fmt.Sprintf(urlCommentAdd, item.ID), p, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Comment Comment `json:"comment"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	resp.Comment.setValues(item)
	return &resp.Comment, nil
}

func (item *Item) Comments() *Comments {
	c := &Comments{item: item}
	c.Pager = newPager(c.fetch)
	return c
}

func (c *Comments) fetch(ctx context.Context) ([]*Comment, string, error) {
	item := c.item
	q := map[string]string{
		"can_support_threading":              "true",
		"feed_position":                      strconv.Itoa(item.Index),
		"is_carousel_bumped_post":            "false",
		"should_fetch_creator_comment_nudge": "true",
	}
	if c.minID != "" {
		q["min_id"] = c.minID
		q["sort_order"] = "popular"
	}
	var page struct {
		Items                   []*Comment `json:"comments"`
		CommentCount            Int64      `json:"comment_count"`
		Caption                 Caption    `json:"caption"`
		HasMoreHeadloadComments bool       `json:"has_more_headload_comments"`
		NextMinID               string     `json:"next_min_id"`
	}
	err := item.insta.callJSON(ctx, &reqOptions{
		Endpoint: fmt.Sprintf(urlStreamComments, item.ID),
		Stream:   true,
		Query:    q,
	}, &page)
	if err != nil {
		return nil, "", err
	}
	for _, cm := range page.Items {
		cm.setValues(item)
	}
	c.Count, c.Caption, c.minID = page.CommentCount, page.Caption, page.NextMinID
	return page.Items, cursorIf(page.HasMoreHeadloadComments, page.NextMinID), nil
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

func (c *Comment) Like(ctx context.Context) error {
	p, err := c.likePayload()
	if err != nil {
		return err
	}
	_, err = c.item.postSigned(ctx, fmt.Sprintf(urlCommentLike, c.getid()), p, nil)
	return err
}

func (c *Comment) Unlike(ctx context.Context) error {
	p, err := c.likePayload()
	if err != nil {
		return err
	}
	p["m_ix"] = strconv.Itoa(c.item.Index)
	_, err = c.item.postSigned(ctx, fmt.Sprintf(urlCommentUnlike, c.getid()), p, nil)
	return err
}

// Replies loads the comment's thread (inline_child_comments).
func (c *Comment) Replies(ctx context.Context) ([]*Comment, error) {
	if c.item == nil {
		return nil, fmt.Errorf("comment %v has no parent media", c.ID)
	}
	var resp struct {
		ChildComments []*Comment `json:"child_comments"`
	}
	err := c.item.insta.callJSON(ctx, &reqOptions{
		Endpoint: fmt.Sprintf(urlInlineChildComments, c.item.ID, c.getid()),
	}, &resp)
	if err != nil {
		return nil, err
	}
	for _, r := range resp.ChildComments {
		r.setValues(c.item)
	}
	return resp.ChildComments, nil
}

func (c *Comment) Delete(ctx context.Context) error {
	if c.item == nil {
		return fmt.Errorf("comment %v has no parent media", c.ID)
	}
	return c.item.DeleteComments(ctx, c.getid())
}

// DeleteComments removes comments by id (comment/bulk_delete).
func (item *Item) DeleteComments(ctx context.Context, ids ...string) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlCommentBulkDelete, item.ID), map[string]string{
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
	case MediaVideo:
		return "VIDEO"
	case MediaAlbum:
		return "CAROUSEL"
	default:
		return "PHOTO"
	}
}

// Delete removes one of the account's own posts.
func (item *Item) Delete(ctx context.Context) error {
	q, err := signedBody(map[string]string{
		"_uid":              strconv.FormatInt(item.insta.userID(), 10),
		"_uuid":             item.insta.uuid,
		"igtv_feed_preview": "false",
		"media_id":          item.ID,
	})
	if err != nil {
		return err
	}
	_, err = item.insta.call(ctx, &reqOptions{
		Endpoint:     fmt.Sprintf(urlMediaDelete, item.ID) + "?media_type=" + item.mediaTypeName(),
		FriendlyName: "IgApi: " + fmt.Sprintf(urlMediaDelete, item.ID) + "?media_type=" + item.mediaTypeName(),
		IsPost:       true,
		Query:        q,
	})
	return err
}

// Archive hides one of the account's own posts from the profile (only_me).
func (item *Item) Archive(ctx context.Context) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlMediaOnlyMe, item.ID), map[string]string{
		"_uid":     strconv.FormatInt(item.insta.userID(), 10),
		"_uuid":    item.insta.uuid,
		"media_id": item.ID,
	}, nil)
	return err
}

// Pin pins one of the account's own posts to the top of the profile grid.
func (item *Item) Pin(ctx context.Context) error {
	_, err := item.insta.call(ctx, &reqOptions{
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
func (item *Item) AddNote(ctx context.Context, text string) (*Note, error) {
	var n Note
	err := item.insta.callJSON(ctx, &reqOptions{
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
	}, &n)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func mergeForms(dst, src map[string]string) map[string]string {
	maps.Copy(dst, src)
	return dst
}
