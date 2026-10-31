package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// Thread is a direct-message conversation.
type Thread struct {
	insta *Instagram
	// loaded is set when Items is a full page from the thread endpoint, not an inbox preview.
	loaded bool

	ThreadID       string        `json:"thread_id"`
	ThreadV2ID     string        `json:"thread_v2_id"`
	Title          string        `json:"thread_title"`
	Users          []*User       `json:"users"`
	LeftUsers      []*User       `json:"left_users"`
	AdminUserIDs   []Int64       `json:"admin_user_ids"`
	Items          []*DirectItem `json:"items"`
	LastActivityAt Int64         `json:"last_activity_at"`
	ViewerID       Int64         `json:"viewer_id"`
	IsGroup        bool          `json:"is_group"`
	Muted          bool          `json:"muted"`
	IsPin          bool          `json:"is_pin"`
	Named          bool          `json:"named"`
	Canonical      bool          `json:"canonical"`
	Pending        bool          `json:"pending"`
	OldestCursor   string        `json:"oldest_cursor"`
	NewestCursor   string        `json:"newest_cursor"`
	HasOlder       bool          `json:"has_older"`
	HasNewer       bool          `json:"has_newer"`
	// LastSeenAt is keyed by user id.
	LastSeenAt map[string]ThreadSeen `json:"last_seen_at"`
}

type ThreadSeen struct {
	Timestamp string `json:"timestamp"`
	ItemID    string `json:"item_id"`
}

// DirectItem is one message. Raw holds the whole item, for kinds the typed
// fields do not cover.
type DirectItem struct {
	ItemID        string `json:"item_id"`
	UserID        Int64  `json:"user_id"`
	Timestamp     Int64  `json:"timestamp"` // microseconds
	ItemType      string `json:"item_type"` // text, media_share, clip, link, like, reel_share, story_share, …
	Text          string `json:"text"`
	ClientContext string `json:"client_context"`
	MediaShare    *Item  `json:"media_share"`
	Clip          *struct {
		Clip *Item `json:"clip"`
	} `json:"clip"`
	Link *struct {
		Text        string `json:"text"`
		LinkContext struct {
			URL   string `json:"link_url"`
			Title string `json:"link_title"`
		} `json:"link_context"`
	} `json:"link"`
	Raw json.RawMessage `json:"-"`
}

func (d *DirectItem) UnmarshalJSON(b []byte) error {
	type plain DirectItem
	if err := json.Unmarshal(b, (*plain)(d)); err != nil {
		return err
	}
	d.Raw = append(json.RawMessage(nil), b...)
	return nil
}

func (t *Thread) setValues(insta *Instagram) {
	t.insta = insta
	insta.usersFrom(t.Users, entryContext{})
	for _, it := range t.Items {
		if it.MediaShare != nil {
			setToItem(it.MediaShare, insta, "direct_thread")
		}
		if it.Clip != nil && it.Clip.Clip != nil {
			setToItem(it.Clip.Clip, insta, "direct_thread")
		}
	}
}

// Inbox is the direct inbox; UnseenCount and PendingTotal come from the first page.
type Inbox struct {
	*Pager[*Thread]
	UnseenCount  int
	PendingTotal int
}

type inboxQuery struct {
	FetchReason              string `json:"fetch_reason"`
	RequestLogTrackingID     string `json:"igd_request_log_tracking_id"`
	IncludeOldMessageRequest string `json:"include_old_mrs"`
	Limit                    string `json:"limit"`
	ThreadMessageLimit       string `json:"thread_message_limit"`
	CursorThreadV2ID         string `json:"cursor_thread_v2_id,omitempty"`
	CursorTimestampMs        string `json:"cursor_timestamp_ms,omitempty"`
	CursorRelevancyScore     string `json:"cursor_relevancy_score,omitempty"`
}

// inboxCursor is prev_cursor_v2, the page-after cursor the app sends back as
// cursor_thread_v2_id / cursor_timestamp_ms / cursor_relevancy_score.
type inboxCursor struct {
	ThreadFbid     Int64 `json:"thread_fbid"`
	RelevancyScore Int64 `json:"relevancy_score"`
	TimestampMs    Int64 `json:"timestamp_ms"`
}

// Inbox pages through the direct inbox, newest threads first.
func (insta *Instagram) Inbox() *Inbox {
	return insta.inbox(urlDirectInbox)
}

// PendingInbox pages through message requests.
func (insta *Instagram) PendingInbox() *Inbox {
	return insta.inbox(urlDirectPendingInbox)
}

func (insta *Instagram) inbox(endpoint string) *Inbox {
	in := &Inbox{}
	var next *inboxCursor
	in.Pager = newPager(func(ctx context.Context) ([]*Thread, string, error) {
		query := inboxQuery{
			FetchReason:              "initial_snapshot",
			RequestLogTrackingID:     generateUUID(),
			IncludeOldMessageRequest: "false",
			Limit:                    "15",
			ThreadMessageLimit:       "5",
		}
		if next != nil {
			query.FetchReason = "page_scroll"
			query.CursorThreadV2ID = next.ThreadFbid.String()
			query.CursorTimestampMs = next.TimestampMs.String()
			query.CursorRelevancyScore = next.RelevancyScore.String()
		}
		var resp struct {
			Inbox struct {
				Threads      []*Thread   `json:"threads"`
				UnseenCount  int         `json:"unseen_count"`
				HasOlder     bool        `json:"has_older"`
				PrevCursorV2 inboxCursor `json:"prev_cursor_v2"`
			} `json:"inbox"`
			PendingRequestsTotal int `json:"pending_requests_total"`
		}
		if err := insta.callJSON(ctx, &reqOptions{Endpoint: endpoint, Form: query}, &resp); err != nil {
			return nil, "", err
		}
		for _, t := range resp.Inbox.Threads {
			t.setValues(insta)
		}
		if next == nil {
			in.UnseenCount, in.PendingTotal = resp.Inbox.UnseenCount, resp.PendingRequestsTotal
		}
		c := resp.Inbox.PrevCursorV2
		next = &c
		return resp.Inbox.Threads, cursorIf(resp.Inbox.HasOlder, c.ThreadFbid.String()+"/"+c.TimestampMs.String()), nil
	})
	return in
}

type threadQuery struct {
	Cursor                  string `json:"cursor,omitempty"`
	Direction               string `json:"direction,omitempty"`
	Limit                   string `json:"limit"`
	VisualMessageReturnType string `json:"visual_message_return_type"`
	RequestLogTrackingID    string `json:"igd_request_log_tracking_id"`
}

// Thread loads one conversation with its latest messages.
func (insta *Instagram) Thread(ctx context.Context, threadID string) (*Thread, error) {
	return insta.fetchThread(ctx, threadID, "")
}

func (insta *Instagram) fetchThread(ctx context.Context, threadID, cursor string) (*Thread, error) {
	query := threadQuery{Limit: "20", VisualMessageReturnType: "unseen", RequestLogTrackingID: generateUUID()}
	if cursor != "" {
		query.Cursor, query.Direction = cursor, "older"
	}
	var resp struct {
		Thread *Thread `json:"thread"`
	}
	if err := insta.callJSON(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlDirectThread, threadID), Form: query}, &resp); err != nil {
		return nil, err
	}
	if resp.Thread == nil {
		return nil, fmt.Errorf("thread %s: %w", threadID, ErrEmptyResponse)
	}
	resp.Thread.setValues(insta)
	resp.Thread.loaded = true
	return resp.Thread, nil
}

// Messages pages backwards through the thread's messages, newest first. A thread
// from Thread already holds the first page, so the walk starts after it.
func (t *Thread) Messages() *Pager[*DirectItem] {
	cursor, first := "", t.loaded
	return newPager(func(ctx context.Context) ([]*DirectItem, string, error) {
		if first {
			first = false
			cursor = t.OldestCursor
			return t.Items, cursorIf(t.HasOlder, cursor), nil
		}
		page, err := t.insta.fetchThread(ctx, t.ThreadID, cursor)
		if err != nil {
			return nil, "", err
		}
		cursor = page.OldestCursor
		return page.Items, cursorIf(page.HasOlder, cursor), nil
	})
}

// directMutation is the part every direct write shares; client_context and
// offline_threading_id carry the same offline threading id.
type directMutation struct {
	UUID               string `json:"_uuid"`
	Action             string `json:"action"`
	ClientContext      string `json:"client_context"`
	OfflineThreadingID string `json:"offline_threading_id"`
}

func (insta *Instagram) directMutation(action string) directMutation {
	otid := offlineThreadingID()
	return directMutation{UUID: insta.uuid, Action: action, ClientContext: otid, OfflineThreadingID: otid}
}

// sendTextForm is broadcast/text's plain form; mutation_token repeats the offline threading id.
type sendTextForm struct {
	directMutation
	Text            string `json:"text"`
	MutationToken   string `json:"mutation_token"`
	DeviceID        string `json:"device_id"`
	SendAttribution string `json:"send_attribution"`
	ThreadIDs       string `json:"thread_ids,omitempty"`
	RecipientUsers  string `json:"recipient_users,omitempty"`
}

// DirectSent is the server's acknowledgement of a sent message.
type DirectSent struct {
	ItemID        string `json:"item_id"`
	ThreadID      string `json:"thread_id"`
	ClientContext string `json:"client_context"`
	Timestamp     string `json:"timestamp"`
}

// SendText sends a text message to the thread.
func (t *Thread) SendText(ctx context.Context, text string) (*DirectSent, error) {
	return t.insta.sendText(ctx, text, "["+t.ThreadID+"]", "")
}

// SendDirectText sends a text message to these users, in their existing
// thread or a new one.
func (insta *Instagram) SendDirectText(ctx context.Context, text string, userIDs ...int64) (*DirectSent, error) {
	return insta.sendText(ctx, text, "", "[["+strings.Join(idStrings(userIDs), ",")+"]]")
}

func (insta *Instagram) sendText(ctx context.Context, text, threadIDs, recipients string) (*DirectSent, error) {
	m := insta.directMutation("send_item")
	var resp struct {
		Payload DirectSent `json:"payload"`
	}
	if err := insta.callJSON(ctx, &reqOptions{Endpoint: urlDirectSendText, IsPost: true, Form: sendTextForm{
		directMutation:  m,
		Text:            text,
		MutationToken:   m.OfflineThreadingID,
		DeviceID:        insta.dID,
		SendAttribution: "direct_thread",
		ThreadIDs:       threadIDs,
		RecipientUsers:  recipients,
	}}, &resp); err != nil {
		return nil, err
	}
	return &resp.Payload, nil
}

type itemSeenForm struct {
	directMutation
	ThreadID string `json:"thread_id"`
}

// MarkSeen marks the thread read up to item.
func (t *Thread) MarkSeen(ctx context.Context, item *DirectItem) error {
	_, err := t.insta.call(ctx, &reqOptions{
		Endpoint: fmt.Sprintf(urlDirectItemSeen, t.ThreadID, item.ItemID),
		IsPost:   true,
		Form:     itemSeenForm{directMutation: t.insta.directMutation("mark_seen"), ThreadID: t.ThreadID},
	})
	return err
}

// offlineThreadingID is the app's message id: unix ms in the high bits, 22
// random low bits.
func offlineThreadingID() string {
	return strconv.FormatInt(time.Now().UnixMilli()<<22|rand.Int64N(1<<22), 10)
}
