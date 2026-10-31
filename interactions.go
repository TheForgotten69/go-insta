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

// mediaContext is what the app attaches to a like or a comment on a post.
type mediaContext struct {
	sessionIDs
	RadioType            string `json:"radio_type"`
	DeliveryClass        string `json:"delivery_class"`
	IsCarouselBumpedPost string `json:"is_carousel_bumped_post"`
	FloatingContextItems string `json:"floating_context_items"`
	MezqlToken           string `json:"mezql_token"`
	InventorySource      string `json:"inventory_source,omitempty"`
}

func (item *Item) mediaContext() mediaContext {
	return mediaContext{
		sessionIDs:           item.insta.sessionIDs(),
		RadioType:            "ethernet-none",
		DeliveryClass:        item.deliveryClass(),
		IsCarouselBumpedPost: "false",
		FloatingContextItems: "[]",
		InventorySource:      item.InventorySource,
	}
}

// postSigned posts payload as the signed body; form, when not nil, adds plain form fields.
func (item *Item) postSigned(ctx context.Context, endpoint string, payload, form any) ([]byte, error) {
	q, err := signedBody(payload)
	if err != nil {
		return nil, err
	}
	if form != nil {
		extra, err := formOf(form)
		if err != nil {
			return nil, err
		}
		maps.Copy(q, extra)
	}
	return item.insta.call(ctx, &reqOptions{Endpoint: endpoint, IsPost: true, Query: q})
}

type likeRequest struct {
	mediaContext
	MediaID           string `json:"media_id"`
	ContainerModule   string `json:"container_module"`
	FeedPosition      string `json:"feed_position"`
	TapSource         string `json:"tap_source"`
	IsCarouselAd      string `json:"is_carousel_ad"`
	Is2mEnabled       string `json:"is_2m_enabled"`
	IsFromSwipe       string `json:"is_from_swipe"`
	LikeBubbleUserIDs string `json:"like_bubble_user_ids"`
}

func (item *Item) Like(ctx context.Context) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlMediaLike, item.ID), likeRequest{
		mediaContext:    item.mediaContext(),
		MediaID:         item.ID,
		ContainerModule: item.containerModule(),
		FeedPosition:    strconv.Itoa(item.Index),
		TapSource:       "button",
		IsCarouselAd:    "false",
		Is2mEnabled:     "false",
		IsFromSwipe:     "false",
	}, likeForm{D: "0"})
	return err
}

// likeForm is the plain form field next to the like's signed body.
type likeForm struct {
	D string `json:"d"`
}

type saveRequest struct {
	sessionIDs
	RadioType     string `json:"radio_type"`
	DeliveryClass string `json:"delivery_class"`
	// Unlike like and comment, save sends a real array.
	FloatingContextItems []string `json:"floating_context_items"`
	MezqlToken           string   `json:"mezql_token"`
	ModuleName           string   `json:"module_name"`
	ClientPosition       string   `json:"client_position"`
	LikeBubbleUserIDs    string   `json:"like_bubble_user_ids"`
}

func (item *Item) Save(ctx context.Context) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlMediaSave, item.ID), saveRequest{
		sessionIDs:           item.insta.sessionIDs(),
		RadioType:            "ethernet-none",
		DeliveryClass:        item.deliveryClass(),
		FloatingContextItems: []string{},
		ModuleName:           item.containerModule(),
		ClientPosition:       strconv.Itoa(item.Index),
	}, nil)
	return err
}

type commentRequest struct {
	mediaContext
	CommentText                  string `json:"comment_text"`
	CommentCreationKey           string `json:"comment_creation_key"`
	IdempotenceToken             string `json:"idempotence_token"`
	UserBreadcrumb               string `json:"user_breadcrumb"`
	ContainerModule              string `json:"container_module"`
	CarouselIndex                string `json:"carousel_index"`
	CarouselChildMentions        string `json:"carousel_child_mentions"`
	IncludeCarouselChildMentions string `json:"include_carousel_child_mentions"`
	IsFromCarouselChildThread    string `json:"is_from_carousel_child_thread"`
	IsTextAppXpostAttempt        string `json:"is_text_app_xpost_attempt"`
	FeedPosition                 string `json:"feed_position"`
	RecsIx                       string `json:"recs_ix"`
	ChainingSeedMediaID          string `json:"chaining_seed_media_id"`
	StartingClipsMediaID         string `json:"starting_clips_media_id"`
	RankingSessionID             string `json:"ranking_session_id"`
}

func (item *Item) Comment(ctx context.Context, text string) (*Comment, error) {
	key := generateUUID()
	body, err := item.postSigned(ctx, fmt.Sprintf(urlCommentAdd, item.ID), commentRequest{
		mediaContext:                 item.mediaContext(),
		CommentText:                  text,
		CommentCreationKey:           key,
		IdempotenceToken:             key,
		UserBreadcrumb:               userBreadcrumb(text),
		ContainerModule:              "comments_v2_" + item.containerModule(),
		CarouselIndex:                "-1",
		CarouselChildMentions:        "[]",
		IncludeCarouselChildMentions: "false",
		IsFromCarouselChildThread:    "false",
		IsTextAppXpostAttempt:        "false",
		FeedPosition:                 strconv.Itoa(item.Index),
		RecsIx:                       "-1",
		ChainingSeedMediaID:          "null",
		StartingClipsMediaID:         "null",
		RankingSessionID:             generateUUID(),
	}, nil)
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
	query := streamCommentsQuery{
		CanSupportThreading:            "true",
		FeedPosition:                   strconv.Itoa(item.Index),
		IsCarouselBumpedPost:           "false",
		ShouldFetchCreatorCommentNudge: "true",
	}
	if c.minID != "" {
		query.MinID, query.SortOrder = c.minID, "popular"
	}
	q, err := formOf(query)
	if err != nil {
		return nil, "", err
	}
	// Each NDJSON line holds different comments under the same cursor.
	var page struct {
		Items                   []*Comment `json:"comments"`
		CommentCount            Int64      `json:"comment_count"`
		Caption                 Caption    `json:"caption"`
		HasMoreHeadloadComments bool       `json:"has_more_headload_comments"`
		NextMinID               string     `json:"next_min_id"`
	}
	var items []*Comment
	seen := map[json.Number]bool{}
	err = item.insta.callLines(ctx, &reqOptions{
		Endpoint: fmt.Sprintf(urlStreamComments, item.ID),
		Query:    q,
	}, func(line []byte) error {
		page.Items = nil
		if err := json.Unmarshal(line, &page); err != nil {
			return err
		}
		for _, cm := range page.Items {
			if !seen[cm.ID] {
				seen[cm.ID] = true
				cm.setValues(item)
				items = append(items, cm)
			}
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	c.Count, c.Caption, c.minID = page.CommentCount, page.Caption, page.NextMinID
	return items, cursorIf(page.HasMoreHeadloadComments, page.NextMinID), nil
}

type streamCommentsQuery struct {
	CanSupportThreading            string `json:"can_support_threading"`
	FeedPosition                   string `json:"feed_position"`
	IsCarouselBumpedPost           string `json:"is_carousel_bumped_post"`
	ShouldFetchCreatorCommentNudge string `json:"should_fetch_creator_comment_nudge"`
	MinID                          string `json:"min_id,omitempty"`
	SortOrder                      string `json:"sort_order,omitempty"`
}

type commentLikeRequest struct {
	sessionIDs
	ContainerModule      string `json:"container_module"`
	DeliveryClass        string `json:"delivery_class"`
	FeedPosition         string `json:"feed_position"`
	IsCarouselBumpedPost string `json:"is_carousel_bumped_post"`
	IsFeedPreviewComment string `json:"is_feed_preview_comment"`
	IsReplyHighlight     string `json:"is_reply_highlight"`
	InventorySource      string `json:"inventory_source,omitempty"`
	MIx                  string `json:"m_ix,omitempty"` // unlike only
}

func (c *Comment) likeRequest() (commentLikeRequest, error) {
	if c.item == nil {
		return commentLikeRequest{}, fmt.Errorf("comment %v has no parent media", c.ID)
	}
	it := c.item
	return commentLikeRequest{
		sessionIDs:           it.insta.sessionIDs(),
		ContainerModule:      "comments_v2_" + it.containerModule(),
		DeliveryClass:        it.deliveryClass(),
		FeedPosition:         strconv.Itoa(it.Index),
		IsCarouselBumpedPost: "false",
		IsFeedPreviewComment: "false",
		IsReplyHighlight:     "false",
		InventorySource:      it.InventorySource,
	}, nil
}

func (c *Comment) Like(ctx context.Context) error {
	req, err := c.likeRequest()
	if err != nil {
		return err
	}
	_, err = c.item.postSigned(ctx, fmt.Sprintf(urlCommentLike, c.getid()), req, nil)
	return err
}

func (c *Comment) Unlike(ctx context.Context) error {
	req, err := c.likeRequest()
	if err != nil {
		return err
	}
	req.MIx = strconv.Itoa(c.item.Index)
	_, err = c.item.postSigned(ctx, fmt.Sprintf(urlCommentUnlike, c.getid()), req, nil)
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

type bulkDeleteCommentsRequest struct {
	sessionIDs
	CommentIDsToDelete  string `json:"comment_ids_to_delete"`
	ContainerModule     string `json:"container_module"`
	ClientPosition      string `json:"client_position"`
	ChainingSeedMediaID string `json:"chaining_seed_media_id"`
	MezqlToken          string `json:"mezql_token"`
}

// DeleteComments removes comments by id (comment/bulk_delete).
func (item *Item) DeleteComments(ctx context.Context, ids ...string) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlCommentBulkDelete, item.ID), bulkDeleteCommentsRequest{
		sessionIDs:          item.insta.sessionIDs(),
		CommentIDsToDelete:  strings.Join(ids, ","),
		ContainerModule:     "self_comments_v2_feed_contextual_self_profile",
		ClientPosition:      strconv.Itoa(item.Index),
		ChainingSeedMediaID: item.pk(),
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

type mediaDeleteRequest struct {
	sessionIDs
	IgtvFeedPreview string `json:"igtv_feed_preview"`
	MediaID         string `json:"media_id"`
}

type mediaIDRequest struct {
	sessionIDs
	MediaID string `json:"media_id"`
}

// Delete removes one of the account's own posts.
func (item *Item) Delete(ctx context.Context) error {
	q, err := signedBody(mediaDeleteRequest{sessionIDs: item.insta.sessionIDs(), IgtvFeedPreview: "false", MediaID: item.ID})
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
	_, err := item.postSigned(ctx, fmt.Sprintf(urlMediaOnlyMe, item.ID), mediaIDRequest{item.insta.sessionIDs(), item.ID}, nil)
	return err
}

// Pin pins one of the account's own posts to the top of the profile grid.
func (item *Item) Pin(ctx context.Context) error {
	q, err := formOf(pinRequest{UUID: item.insta.uuid, PostID: item.pk()})
	if err != nil {
		return err
	}
	_, err = item.insta.call(ctx, &reqOptions{Endpoint: urlPinTimelineMedia, IsPost: true, Query: q})
	return err
}

type pinRequest struct {
	UUID   string `json:"_uuid"`
	PostID string `json:"post_id"`
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

type createNoteRequest struct {
	UUID                string `json:"_uuid"`
	Audience            string `json:"audience"`
	CarouselIndex       string `json:"carousel_index"`
	ContainerModule     string `json:"container_module"`
	EventSource         string `json:"event_source"`
	MediaClientPosition string `json:"media_client_position"`
	MediaID             string `json:"media_id"`
	NoteStyle           string `json:"note_style"`
	Text                string `json:"text"`
}

// AddNote leaves a note on the media (the bubble shown over reels/posts).
func (item *Item) AddNote(ctx context.Context, text string) (*Note, error) {
	q, err := formOf(createNoteRequest{
		UUID:                item.insta.uuid,
		Audience:            "7",
		CarouselIndex:       "-1",
		ContainerModule:     item.containerModule(),
		EventSource:         "ufi",
		MediaClientPosition: strconv.Itoa(item.Index),
		MediaID:             item.ID,
		NoteStyle:           "13",
		Text:                text,
	})
	if err != nil {
		return nil, err
	}
	var n Note
	err = item.insta.callJSON(ctx, &reqOptions{
		Endpoint:     urlCreateNote,
		FriendlyName: "IgApi: api/v1/media/create_note/v2/",
		IsPost:       true,
		Query:        q,
	}, &n)
	if err != nil {
		return nil, err
	}
	return &n, nil
}
