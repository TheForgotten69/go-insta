package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	urlEditMedia            = "media/%s/edit_media/"
	urlDisableComments      = "media/%s/disable_comments/"
	urlEnableComments       = "media/%s/enable_comments/"
	urlCountsVisibility     = "media/update_like_and_view_counts_visibility/"
	urlClipDownloadsEnabled = "clips/item/set_third_party_downloads_enabled/"
	urlClipRemixAllowed     = "clips/item/set_mashups_allowed/"
	urlUndoOnlyMe           = "media/%s/undo_only_me/"
	urlPinComment           = "media/%s/pin_comment/%s/"
	urlUnpinComment         = "media/%s/unpin_comment/%s/"
	urlMediaLikers          = "media/%s/likers/"
	urlMediaInfo            = "media/%s/info/"
)

// MediaEdit is one save of a post's "Edit" screen. Caption replaces the whole
// caption, so pass the current one to keep it.
type MediaEdit struct {
	Caption       string
	ClearLocation bool
	// RemoveChildren deletes album photos; ChildOrder reorders the remaining ones.
	RemoveChildren []*Item
	ChildOrder     []*Item
}

type editMediaRequest struct {
	sessionIDs
	CaptionText           string `json:"caption_text"`
	DeviceID              string `json:"device_id"`
	ContainerModule       string `json:"container_module"`
	AllowEditBoostedMedia string `json:"allow_edit_boosted_media"`
	Location              string `json:"location,omitempty"`
	ChildrenToDelete      string `json:"carousel_children_media_ids_to_delete,omitempty"`
	ChildrenInOrder       string `json:"carousel_children_media_ids_in_order,omitempty"`
	InventorySource       string `json:"inventory_source,omitempty"`
}

// Edit saves e on one of the account's own posts and refreshes the item from
// the server's copy.
func (item *Item) Edit(ctx context.Context, e MediaEdit) error {
	req := editMediaRequest{
		sessionIDs:            item.insta.sessionIDs(),
		CaptionText:           e.Caption,
		DeviceID:              item.insta.dID,
		ContainerModule:       "edit_media_info",
		AllowEditBoostedMedia: "true",
		InventorySource:       item.InventorySource,
	}
	if e.ClearLocation {
		req.Location = "{}"
	}
	if len(e.RemoveChildren) > 0 {
		req.ChildrenToDelete = mediaIDList(e.RemoveChildren)
	}
	if len(e.ChildOrder) > 0 {
		req.ChildrenInOrder = mediaIDList(e.ChildOrder)
	}
	body, err := item.postSigned(ctx, fmt.Sprintf(urlEditMedia, item.ID), req, nil)
	if err != nil {
		return err
	}
	var resp struct {
		Media *Item `json:"media"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Media != nil {
		setToItem(resp.Media, item.insta, item.module)
		*item = *resp.Media
	}
	return nil
}

func (item *Item) DisableComments(ctx context.Context) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlDisableComments, item.ID), item.insta.sessionIDs(), nil)
	return err
}

func (item *Item) EnableComments(ctx context.Context) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlEnableComments, item.ID), item.insta.sessionIDs(), nil)
	return err
}

type countsVisibilityRequest struct {
	sessionIDs
	LikeAndViewCountsDisabled string `json:"like_and_view_counts_disabled"`
	ShareCountDisabled        string `json:"share_count_disabled"`
	MediaID                   string `json:"media_id"`
}

// SetCountsHidden hides or shows the like/view and share counts of an own post.
func (item *Item) SetCountsHidden(ctx context.Context, likes, shares bool) error {
	_, err := item.postSigned(ctx, urlCountsVisibility, countsVisibilityRequest{
		sessionIDs:                item.insta.sessionIDs(),
		LikeAndViewCountsDisabled: fmt.Sprint(likes),
		ShareCountDisabled:        fmt.Sprint(shares),
		MediaID:                   item.ID,
	}, nil)
	return err
}

type clipDownloadsRequest struct {
	sessionIDs
	MediaID                    string `json:"media_id"`
	ThirdPartyDownloadsEnabled string `json:"third_party_downloads_enabled"`
	ContainerModule            string `json:"container_module"`
}

// SetDownloads allows or blocks downloading one of the account's own Reels.
func (item *Item) SetDownloads(ctx context.Context, allowed bool) error {
	return item.postForm(ctx, urlClipDownloadsEnabled, clipDownloadsRequest{
		sessionIDs:                 item.insta.sessionIDs(),
		MediaID:                    item.pk(),
		ThirdPartyDownloadsEnabled: fmt.Sprint(allowed),
		ContainerModule:            "clips_viewer_clips_tab",
	})
}

// postForm posts req as a plain form; the clips/item settings take the numeric
// media id this way and answer 400 to "<pk>_<owner>".
func (item *Item) postForm(ctx context.Context, endpoint string, req any) error {
	q, err := formOf(req)
	if err != nil {
		return err
	}
	_, err = item.insta.call(ctx, &reqOptions{Endpoint: endpoint, IsPost: true, Query: q})
	return err
}

type clipRemixRequest struct {
	sessionIDs
	ClipsMediaID    string `json:"clips_media_id"`
	MediaID         string `json:"media_id"`
	MashupsAllowed  string `json:"mashups_allowed"`
	ContainerModule string `json:"container_module"`
}

// SetRemixAllowed allows or blocks remixes of one of the account's own Reels.
func (item *Item) SetRemixAllowed(ctx context.Context, allowed bool) error {
	return item.postForm(ctx, urlClipRemixAllowed, clipRemixRequest{
		sessionIDs:      item.insta.sessionIDs(),
		ClipsMediaID:    item.pk(),
		MediaID:         item.pk(),
		MashupsAllowed:  fmt.Sprint(allowed),
		ContainerModule: "clips_viewer_clips_tab",
	})
}

// Unarchive shows an archived post on the profile again (undoes Archive).
func (item *Item) Unarchive(ctx context.Context) error {
	_, err := item.postSigned(ctx, fmt.Sprintf(urlUndoOnlyMe, item.ID), mediaIDRequest{item.insta.sessionIDs(), item.ID}, nil)
	return err
}

// Pin pins the comment to the top of its post; Unpin undoes it.
func (c *Comment) Pin(ctx context.Context) error {
	return c.post(ctx, urlPinComment)
}

func (c *Comment) Unpin(ctx context.Context) error {
	return c.post(ctx, urlUnpinComment)
}

func (c *Comment) post(ctx context.Context, endpoint string) error {
	if c.item == nil {
		return fmt.Errorf("comment %v has no parent media", c.ID)
	}
	_, err := c.item.postSigned(ctx, fmt.Sprintf(endpoint, c.item.ID, c.getid()), c.item.insta.sessionIDs(), nil)
	return err
}

// FetchLikers returns the accounts that liked the post (the first page, as the app shows it).
func (item *Item) FetchLikers(ctx context.Context) ([]*User, error) {
	var resp struct {
		Users []*User `json:"users"`
	}
	if err := item.insta.callJSON(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlMediaLikers, item.ID)}, &resp); err != nil {
		return nil, err
	}
	for _, u := range resp.Users {
		u.insta = item.insta
	}
	return resp.Users, nil
}

// MediaInfo loads one post, Reel or story item by id ("<pk>_<owner id>" or pk).
func (insta *Instagram) MediaInfo(ctx context.Context, id string) (*Item, error) {
	var resp struct {
		Items []*Item `json:"items"`
	}
	if err := insta.callJSON(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlMediaInfo, id)}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, ErrNoMedia
	}
	setToItem(resp.Items[0], insta, "")
	return resp.Items[0], nil
}
