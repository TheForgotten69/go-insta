package goinsta

import (
	"encoding/json"
)

type Comments struct {
	*Pager[*Comment]
	item  *Item
	minID string

	Count   Int64
	Caption Caption
}

type Comment struct {
	item *Item

	ID                             json.Number `json:"pk"`
	Text                           string      `json:"text"`
	Type                           any         `json:"type"`
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
	PreviewChildComments           []*Comment  `json:"preview_child_comments"`
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

func (c *Comment) setValues(item *Item) {
	insta := item.insta
	c.item = item
	c.User.insta = insta
	for _, u := range c.OtherPreviewUsers {
		u.insta = insta
	}
	for _, r := range c.PreviewChildComments {
		r.setValues(item)
	}
}

func (c Comment) getid() string {
	return c.ID.String()
}
