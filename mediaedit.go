package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
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
	body, err := item.insta.call(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlEditMedia, item.ID), IsPost: true, Signed: req})
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
	return item.postAction(ctx, urlDisableComments, item.insta.sessionIDs())
}

func (item *Item) EnableComments(ctx context.Context) error {
	return item.postAction(ctx, urlEnableComments, item.insta.sessionIDs())
}

// postAction signs body to one of the media/{id}/<action>/ endpoints.
func (item *Item) postAction(ctx context.Context, endpoint string, body any) error {
	_, err := item.insta.call(ctx, &reqOptions{Endpoint: fmt.Sprintf(endpoint, item.ID), IsPost: true, Signed: body})
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
	_, err := item.insta.call(ctx, &reqOptions{Endpoint: urlCountsVisibility, IsPost: true, Signed: countsVisibilityRequest{
		sessionIDs:                item.insta.sessionIDs(),
		LikeAndViewCountsDisabled: fmt.Sprint(likes),
		ShareCountDisabled:        fmt.Sprint(shares),
		MediaID:                   item.ID,
	}})
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
	// clips/item settings take the numeric media id in a plain form and answer 400 to "<pk>_<owner>".
	_, err := item.insta.call(ctx, &reqOptions{Endpoint: urlClipDownloadsEnabled, IsPost: true, Form: clipDownloadsRequest{
		sessionIDs:                 item.insta.sessionIDs(),
		MediaID:                    item.pk(),
		ThirdPartyDownloadsEnabled: fmt.Sprint(allowed),
		ContainerModule:            "clips_viewer_clips_tab",
	}})
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
	_, err := item.insta.call(ctx, &reqOptions{Endpoint: urlClipRemixAllowed, IsPost: true, Form: clipRemixRequest{
		sessionIDs:      item.insta.sessionIDs(),
		ClipsMediaID:    item.pk(),
		MediaID:         item.pk(),
		MashupsAllowed:  fmt.Sprint(allowed),
		ContainerModule: "clips_viewer_clips_tab",
	}})
	return err
}

// Unarchive shows an archived post on the profile again (undoes Archive).
func (item *Item) Unarchive(ctx context.Context) error {
	return item.postAction(ctx, urlUndoOnlyMe, mediaIDRequest{item.insta.sessionIDs(), item.ID})
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
	_, err := c.item.insta.call(ctx, &reqOptions{Endpoint: fmt.Sprintf(endpoint, c.item.ID, c.getid()), IsPost: true, Signed: c.item.insta.sessionIDs()})
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
	return item.insta.usersFrom(resp.Users, entryContext{}), nil
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
