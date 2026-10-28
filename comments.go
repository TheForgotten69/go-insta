package goinsta

import (
	"encoding/json"
)

type Comments struct {
	item *Item

	Items                          []Comment       `json:"comments"`
	CommentCount                   Int64           `json:"comment_count"`
	Caption                        Caption         `json:"caption"`
	CaptionIsEdited                bool            `json:"caption_is_edited"`
	HasMoreComments                bool            `json:"has_more_comments"`
	HasMoreHeadloadComments        bool            `json:"has_more_headload_comments"`
	ThreadingEnabled               bool            `json:"threading_enabled"`
	MediaHeaderDisplay             string          `json:"media_header_display"`
	InitiateAtTop                  bool            `json:"initiate_at_top"`
	InsertNewCommentToTop          bool            `json:"insert_new_comment_to_top"`
	PreviewComments                []Comment       `json:"preview_comments"`
	NextID                         json.RawMessage `json:"next_max_id,omitempty"`
	NextMinID                      string          `json:"next_min_id,omitempty"`
	CommentLikesEnabled            bool            `json:"comment_likes_enabled"`
	DisplayRealtimeTypingIndicator bool            `json:"display_realtime_typing_indicator"`
	Status                         string          `json:"status"`
}

type Comment struct {
	insta *Instagram
	item  *Item

	ID                             json.Number `json:"pk"`
	Text                           string      `json:"text"`
	Type                           interface{} `json:"type"`
	User                           User        `json:"user"`
	UserID                         Int64       `json:"user_id"`
	BitFlags                       int         `json:"bit_flags"`
	ChildCommentCount              int         `json:"child_comment_count"`
	CommentIndex                   int         `json:"comment_index"`
	CommentLikeCount               int         `json:"comment_like_count"`
	ContentType                    string      `json:"content_type"`
	CreatedAt                      Int64       `json:"created_at"`
	CreatedAtUtc                   Int64       `json:"created_at_utc"`
	DidReportAsSpam                bool        `json:"did_report_as_spam"`
	HasLikedComment                bool        `json:"has_liked_comment"`
	InlineComposerDisplayCondition string      `json:"inline_composer_display_condition"`
	OtherPreviewUsers              []*User     `json:"other_preview_users"`
	PreviewChildComments           []Comment   `json:"preview_child_comments"`
	NextMaxChildCursor             string      `json:"next_max_child_cursor,omitempty"`
	HasMoreTailChildComments       bool        `json:"has_more_tail_child_comments,omitempty"`
	NextMinChildCursor             string      `json:"next_min_child_cursor,omitempty"`
	HasMoreHeadChildComments       bool        `json:"has_more_head_child_comments,omitempty"`
	NumTailChildComments           int         `json:"num_tail_child_comments,omitempty"`
	NumHeadChildComments           int         `json:"num_head_child_comments,omitempty"`
	ShareEnabled                   bool        `json:"share_enabled"`
	IsCovered                      bool        `json:"is_covered"`
	PrivateReplyStatus             Int64       `json:"private_reply_status"`
	SupporterInfo                  struct {
		SupportTier string `json:"support_tier"`
		BadgesCount int    `json:"badges_count"`
	} `json:"supporter_info"`
	Status string `json:"status"`
}

func (c *Comment) setValues(insta *Instagram) {
	c.User.insta = insta
	for i := range c.OtherPreviewUsers {
		c.OtherPreviewUsers[i].insta = insta
	}
	for i := range c.PreviewChildComments {
		c.PreviewChildComments[i].setValues(insta)
	}
}

func (c Comment) getid() string {
	return c.ID.String()
}
