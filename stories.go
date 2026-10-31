package goinsta

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type StoryReelMention struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Z           int     `json:"z"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Rotation    float64 `json:"rotation"`
	IsPinned    int     `json:"is_pinned"`
	IsHidden    int     `json:"is_hidden"`
	IsSticker   int     `json:"is_sticker"`
	IsFBSticker int     `json:"is_fb_sticker"`
	User        User
	DisplayType string `json:"display_type"`
}

type StoryCTA struct {
	Links []struct {
		LinkType                                int    `json:"linkType"`
		WebURI                                  string `json:"webUri"`
		AndroidClass                            string `json:"androidClass"`
		Package                                 string `json:"package"`
		DeeplinkURI                             string `json:"deeplinkUri"`
		CallToActionTitle                       string `json:"callToActionTitle"`
		RedirectURI                             any    `json:"redirectUri"`
		LeadGenFormID                           string `json:"leadGenFormId"`
		IgUserID                                string `json:"igUserId"`
		AppInstallObjectiveInvalidationBehavior any    `json:"appInstallObjectiveInvalidationBehavior"`
	} `json:"links"`
}

type Reel struct {
	insta *Instagram

	ID                     json.RawMessage `json:"id"`
	Items                  []*Item         `json:"items"`
	MediaCount             int             `json:"media_count"`
	MediaIDs               []Int64         `json:"media_ids"`
	Muted                  bool            `json:"muted"`
	LatestReelMedia        Int64           `json:"latest_reel_media"`
	LatestBestiesReelMedia float64         `json:"latest_besties_reel_media"`
	ExpiringAt             float64         `json:"expiring_at"`
	Seen                   float64         `json:"seen"`
	SeenRankedPosition     int             `json:"seen_ranked_position"`
	CanReply               bool            `json:"can_reply"`
	CanGifQuickReply       bool            `json:"can_gif_quick_reply"`
	ClientPrefetchScore    float64         `json:"client_prefetch_score"`
	Title                  string          `json:"title"`
	CanReshare             bool            `json:"can_reshare"`
	ReelType               string          `json:"reel_type"`
	ReelMentions           []string        `json:"reel_mentions"`
	PrefetchCount          int             `json:"prefetch_count"`
	HasBestiesMedia        any             `json:"has_besties_media"`
	HasPrideMedia          bool            `json:"has_pride_media"`
	HasVideo               bool            `json:"has_video"`
	IsCacheable            bool            `json:"is_cacheable"`
	IsSensitiveVerticalAd  bool            `json:"is_sensitive_vertical_ad"`
	RankedPosition         int             `json:"ranked_position"`
	RankerScores           struct {
		Fp   float64 `json:"fp"`
		Ptap float64 `json:"ptap"`
		Vm   float64 `json:"vm"`
	} `json:"ranker_scores"`
	StoryRankingToken    string `json:"story_ranking_token"`
	FaceFilterNuxVersion int    `json:"face_filter_nux_version"`
	HasNewNuxStory       bool   `json:"has_new_nux_story"`
	User                 User   `json:"user"`
	// CoverMedia is set on highlights.
	CoverMedia struct {
		MediaID  string    `json:"media_id"`
		CropRect []float64 `json:"crop_rect"`
	} `json:"cover_media"`
}

func (user *User) Stories(ctx context.Context) (*Reel, error) {
	return user.insta.fetchStories(ctx, int64(user.ID))
}

// maxHighlightPages bounds the cursor walk: the tray can answer an empty page that
// still carries a cursor, so an empty page does not end it.
const maxHighlightPages = 100

// Highlights returns every highlight cover, following the tray cursor past the
// first 10; LoadReels fills their items.
func (user *User) Highlights(ctx context.Context) ([]*Reel, error) {
	var reels []*Reel
	endpoint := fmt.Sprintf(urlUserHighlights, user.ID)
	q, err := formOf(highlightsTrayQuery{
		CheckHighlightsSubtabGating: "false",
		FetchHighResCoverImage:      "true",
		MaxHighlightsPerPage:        "10",
		SupportedCapabilitiesNew:    supportedCapabilities,
	})
	if err != nil {
		return nil, err
	}
	cursor, hint := "", "feed"
	for range maxHighlightPages {
		var page struct {
			Reels  []*Reel `json:"tray"`
			Cursor string  `json:"cursor"`
		}
		err = user.insta.callJSON(ctx, &reqOptions{
			Endpoint:     endpoint,
			Stream:       true,
			ExtraHeaders: map[string]string{"X-Ig-Accept-Hint": hint},
			Query:        q,
		}, &page)
		if err != nil {
			return nil, err
		}
		for _, r := range page.Reels {
			r.setValues(user.insta)
		}
		reels = append(reels, page.Reels...)
		if page.Cursor == "" || page.Cursor == cursor {
			return reels, nil
		}
		cursor = page.Cursor
		endpoint, hint = fmt.Sprintf(urlUserHighlightsPage, user.ID), "image-grid"
		q, err = formOf(highlightsPageQuery{
			Cursor:                   cursor,
			MaxHighlightsPerPage:     "10",
			SupportedCapabilitiesNew: supportedCapabilities,
		})
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("highlights: still paging after %d pages", maxHighlightPages)
}

type highlightsTrayQuery struct {
	CheckHighlightsSubtabGating string `json:"check_highlights_subtab_gating"`
	FetchHighResCoverImage      string `json:"fetch_high_res_cover_image"`
	MaxHighlightsPerPage        string `json:"max_highlights_to_fetch_on_pagination"`
	SupportedCapabilitiesNew    string `json:"supported_capabilities_new"`
}

type highlightsPageQuery struct {
	Cursor                   string `json:"cursor"`
	MaxHighlightsPerPage     string `json:"max_highlights_to_fetch_on_pagination"`
	SupportedCapabilitiesNew string `json:"supported_capabilities_new"`
}

// highlightBase is in every create_reel and edit_reel body; Source is the app
// surface the change came from.
type highlightBase struct {
	sessionIDs
	Source                   string `json:"source"`
	SupportedCapabilitiesNew string `json:"supported_capabilities_new"`
}

func (insta *Instagram) highlightBase(source string) highlightBase {
	return highlightBase{sessionIDs: insta.sessionIDs(), Source: source, SupportedCapabilitiesNew: supportedCapabilities}
}

type createHighlightRequest struct {
	highlightBase
	Title               string `json:"title"`
	Cover               string `json:"cover"`
	MediaIDs            string `json:"media_ids"`
	CreationID          string `json:"creation_id"`
	ShouldAddToMainGrid string `json:"should_add_to_main_grid"`
}

// addHighlightItemsRequest is the story viewer's quick add.
type addHighlightItemsRequest struct {
	highlightBase
	AddedMediaIDs   string `json:"added_media_ids"`
	RemovedMediaIDs string `json:"removed_media_ids"`
}

// editHighlightRequest is the profile's "Edit highlight" save.
type editHighlightRequest struct {
	highlightBase
	Title           string `json:"title"`
	Cover           string `json:"cover,omitempty"`
	AddedMediaIDs   string `json:"added_media_ids"`
	RemovedMediaIDs string `json:"removed_media_ids"`
}

// CreateHighlight makes a highlight from story items; the first is the cover.
// Instagram silently cuts the title to 16 characters; the returned Title shows it.
func (insta *Instagram) CreateHighlight(ctx context.Context, title string, items ...*Item) (*Reel, error) {
	if len(items) == 0 {
		return nil, ErrNoHighlightItems
	}
	return insta.postHighlight(ctx, urlHighlightCreate, createHighlightRequest{
		highlightBase:       insta.highlightBase("STORY_VIEWER_PROFILE"),
		Title:               title,
		Cover:               coverJSON(items[0].ID, formatCrop(centredSquare(items[0]))),
		MediaIDs:            mediaIDList(items),
		CreationID:          newUploadID(),
		ShouldAddToMainGrid: "false",
	})
}

// AddItems adds story items to a highlight, as the story viewer's "Highlight"
// button does, and replaces the reel with the server's copy.
func (reel *Reel) AddItems(ctx context.Context, items ...*Item) error {
	if len(items) == 0 {
		return ErrNoHighlightItems
	}
	return reel.editHighlight(ctx, addHighlightItemsRequest{
		highlightBase:   reel.insta.highlightBase("STORY_VIEWER_PROFILE"),
		AddedMediaIDs:   mediaIDList(items),
		RemovedMediaIDs: "[]",
	})
}

// HighlightEdit is one save of the profile's "Edit highlight" screen. An empty
// Title or a nil Cover keeps the current one.
type HighlightEdit struct {
	Title       string
	Cover       *Item
	Add, Remove []*Item
}

// Edit applies e and replaces the reel with the server's copy.
func (reel *Reel) Edit(ctx context.Context, e HighlightEdit) error {
	req := editHighlightRequest{
		highlightBase:   reel.insta.highlightBase("SELF_PROFILE"),
		Title:           cmp.Or(e.Title, reel.Title),
		AddedMediaIDs:   mediaIDList(e.Add),
		RemovedMediaIDs: mediaIDList(e.Remove),
	}
	media, crop := reel.CoverMedia.MediaID, formatCrop(reel.CoverMedia.CropRect)
	if e.Cover != nil {
		media, crop = e.Cover.ID, formatCrop(centredSquare(e.Cover))
	}
	if media != "" {
		req.Cover = coverJSON(media, crop)
	}
	return reel.editHighlight(ctx, req)
}

func (reel *Reel) editHighlight(ctx context.Context, req any) error {
	if !reel.isHighlight() {
		return ErrNotHighlight
	}
	got, err := reel.insta.postHighlight(ctx, fmt.Sprintf(urlHighlightEdit, reel.ReelID()), req)
	if err != nil {
		return err
	}
	*reel = *got
	return nil
}

// Delete removes the highlight; its stories stay in the story archive.
func (reel *Reel) Delete(ctx context.Context) error {
	if !reel.isHighlight() {
		return ErrNotHighlight
	}
	q, err := signedBody(reel.insta.sessionIDs())
	if err != nil {
		return err
	}
	_, err = reel.insta.call(ctx, &reqOptions{
		Endpoint:      fmt.Sprintf(urlHighlightDelete, reel.ReelID()),
		IsPost:        true,
		Query:         q,
		IgnoreHeaders: []string{"X-Tigon-Is-Retry"}, // absent on delete_reel in the capture
	})
	return err
}

// Archive hides the highlight from the profile; Unarchive shows it again.
func (reel *Reel) Archive(ctx context.Context) error {
	return reel.setArchived(ctx, gqlQuery{url: graphQLWWWURL, friendly: "IGProfileArchiveHighlightQuery",
		docID: docArchiveHighlight, root: "xdt_archive_highlight"})
}

func (reel *Reel) Unarchive(ctx context.Context) error {
	return reel.setArchived(ctx, gqlQuery{url: graphQLWWWURL, friendly: "IGProfileUnarchiveHighlightQuery",
		docID: docUnarchiveHighlight, root: "xdt_unarchive_highlight"})
}

func (reel *Reel) setArchived(ctx context.Context, gq gqlQuery) error {
	if !reel.isHighlight() {
		return ErrNotHighlight
	}
	body, err := reel.insta.graphQL(ctx, gq, map[string]any{"highlight_id": reel.ReelID()})
	if err != nil {
		return err
	}
	var resp struct {
		Data map[string]*struct {
			Success      bool   `json:"success"`
			ErrorMessage string `json:"error_message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if r := resp.Data[gq.root]; r == nil || !r.Success {
		msg := "no result"
		if r != nil {
			msg = r.ErrorMessage
		}
		return fmt.Errorf("%s: %s", gq.root, msg)
	}
	return nil
}

func (reel *Reel) isHighlight() bool {
	return strings.HasPrefix(reel.ReelID(), "highlight:")
}

// postHighlight sends a request that embeds highlightBase and decodes the reel it returns.
func (insta *Instagram) postHighlight(ctx context.Context, endpoint string, req any) (*Reel, error) {
	q, err := signedBody(req)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Reel Reel `json:"reel"`
	}
	if err := insta.callJSON(ctx, &reqOptions{Endpoint: endpoint, IsPost: true, Query: q}, &resp); err != nil {
		return nil, err
	}
	resp.Reel.setValues(insta)
	return &resp.Reel, nil
}

// mediaIDList is a JSON array of "<pk>_<owner>" ids, sent as a string.
func mediaIDList(items []*Item) string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

// coverJSON is the cover field: JSON with crop_rect itself a JSON string, keys in the app's order.
func coverJSON(mediaID, crop string) string {
	b, _ := json.Marshal(struct {
		MediaID  string `json:"media_id"`
		CropRect string `json:"crop_rect"`
	}{mediaID, crop})
	return string(b)
}

// centredSquare is the item's centred square as [left,top,right,bottom] fractions.
func centredSquare(it *Item) []float64 {
	w, h := float64(it.OriginalWidth), float64(it.OriginalHeight)
	if w <= 0 || h <= 0 {
		return nil
	}
	fx, fy := min(1, h/w), min(1, w/h)
	return []float64{(1 - fx) / 2, (1 - fy) / 2, (1 + fx) / 2, (1 + fy) / 2}
}

// formatCrop writes a crop rect like Java floats ("0.0", "0.21830457"); nil is the full frame.
func formatCrop(r []float64) string {
	if len(r) != 4 {
		r = []float64{0, 0, 1, 1}
	}
	parts := make([]string, len(r))
	for i, v := range r {
		parts[i] = strconv.FormatFloat(v, 'f', -1, 32)
		if !strings.Contains(parts[i], ".") {
			parts[i] += ".0"
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (reel *Reel) setValues(insta *Instagram) {
	reel.insta = insta
	reel.User.insta = insta
	for i, it := range reel.Items {
		setToItem(it, insta, "reel_feed_timeline")
		it.Index = i
	}
}

// ReelID is the tray identifier: a user id, or "highlight:<id>" for highlights.
func (reel *Reel) ReelID() string {
	var s string
	if json.Unmarshal(reel.ID, &s) == nil {
		return s
	}
	return string(reel.ID)
}

// Loaded reports whether the reel carries all its items.
func (reel *Reel) Loaded() bool {
	return len(reel.Items) > 0 && (reel.MediaCount == 0 || len(reel.Items) >= reel.MediaCount)
}

func (insta *Instagram) fetchStories(ctx context.Context, id int64) (*Reel, error) {
	var resp struct {
		Reel Reel `json:"reel"`
	}
	q, err := formOf(capabilitiesQuery{SupportedCapabilitiesNew: supportedCapabilities})
	if err != nil {
		return nil, err
	}
	err = insta.callJSON(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlUserStories, id), Query: q}, &resp)
	if err != nil {
		return nil, err
	}
	resp.Reel.setValues(insta)
	return &resp.Reel, nil
}

type capabilitiesQuery struct {
	SupportedCapabilitiesNew string `json:"supported_capabilities_new"`
}

// reelsBatch is how many reels one reels_media_stream call loads. Captures show
// up to 3 per call; a 52-item highlight is 1.4 MB, so 10 stays well under
// maxBodySize.
const reelsBatch = 10

// LoadReels fills the items of every reel that is not Loaded, one request per
// batch of reels, in place.
func (insta *Instagram) LoadReels(ctx context.Context, reels ...*Reel) error {
	byID := make(map[string]*Reel, len(reels))
	var ids []string
	for _, r := range reels {
		if !r.Loaded() {
			id := r.ReelID()
			byID[id] = r
			ids = append(ids, id)
		}
	}
	for chunk := range slices.Chunk(ids, reelsBatch) {
		got, err := insta.FetchReels(ctx, chunk...)
		if err != nil {
			return err
		}
		for id, r := range got {
			if dst, ok := byID[id]; ok {
				*dst = *r
			}
		}
	}
	return nil
}

type reelsMediaStreamRequest struct {
	sessionIDs
	BatchSize                string   `json:"batch_size"`
	ExcludeMediaIDs          string   `json:"exclude_media_ids"`
	Reason                   string   `json:"reason"`
	ReelIDs                  []string `json:"reel_ids"`
	Source                   string   `json:"source"`
	SupportedCapabilitiesNew string   `json:"supported_capabilities_new"`
}

// FetchReels loads the items of the given reels (user ids or "highlight:<id>").
func (insta *Instagram) FetchReels(ctx context.Context, reelIDs ...string) (map[string]*Reel, error) {
	q, err := signedBody(reelsMediaStreamRequest{
		sessionIDs:               insta.sessionIDs(),
		BatchSize:                "0",
		ExcludeMediaIDs:          "[]",
		Reason:                   "on_tap",
		ReelIDs:                  reelIDs,
		Source:                   "profile",
		SupportedCapabilitiesNew: supportedCapabilities,
	})
	if err != nil {
		return nil, err
	}
	// Each line carries one reel, or client_hints with an empty reels map.
	reels := make(map[string]*Reel, len(reelIDs))
	err = insta.callLines(ctx, &reqOptions{
		Endpoint:     urlReelsMediaStream,
		IsPost:       true,
		Query:        q,
		ExtraHeaders: map[string]string{"X-Ig-Accept-Hint": "feed"},
	}, func(line []byte) error {
		var chunk struct {
			Reels map[string]*Reel `json:"reels"`
		}
		if err := json.Unmarshal(line, &chunk); err != nil {
			return err
		}
		for id, r := range chunk.Reels {
			r.setValues(insta)
			reels[id] = r
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reels, nil
}

func (reel *Reel) Sync(ctx context.Context) error {
	reels, err := reel.insta.FetchReels(ctx, reel.ReelID())
	if err != nil {
		return err
	}
	r, ok := reels[reel.ReelID()]
	if !ok {
		return ErrNoMedia
	}
	*reel = *r
	return nil
}

type emptyObject struct{}

type mediaSeenRequest struct {
	sessionIDs
	ContainerModule   string      `json:"container_module"`
	ForceSeenStoryIDs []string    `json:"force_seen_story_ids"`
	Nuxes             emptyObject `json:"nuxes"`
	NuxesSkipped      emptyObject `json:"nuxes_skipped"`
	ReelMediaSkipped  emptyObject `json:"reel_media_skipped"`
	// Reels maps "<media id>_<reel id>" to ["<taken_at>_<seen_at>"].
	Reels map[string][]string `json:"reels"`
}

func (reel *Reel) MarkSeen(ctx context.Context, items ...*Item) error {
	insta := reel.insta
	reelID := reel.ReelID()
	now := time.Now().Unix()
	reels := make(map[string][]string, len(items))
	for _, it := range items {
		reels[it.ID+"_"+reelID] = []string{fmt.Sprintf("%d_%d", it.TakenAt, now)}
	}
	module := "reel_feed_timeline"
	if reel.isHighlight() {
		module = "highlight_profile"
	}
	q, err := signedBody(mediaSeenRequest{
		sessionIDs:        insta.sessionIDs(),
		ContainerModule:   module,
		ForceSeenStoryIDs: []string{},
		Reels:             reels,
	})
	if err != nil {
		return err
	}
	_, err = insta.call(ctx, &reqOptions{
		Endpoint:     urlMediaSeenV2 + "?reel=1&live_vod=0",
		FriendlyName: "IgApi: media/seen/?reel=1&live_vod=0",
		UseV2:        true,
		IsPost:       true,
		Gzip:         true,
		Query:        q,
	})
	return err
}

type storyLikeRequest struct {
	sessionIDs
	ContainerModule string `json:"container_module"`
	DeliveryClass   string `json:"delivery_class"`
	MediaID         string `json:"media_id"`
	TrayPosition    string `json:"tray_position"`
	TraySessionID   string `json:"tray_session_id"`
	ViewerSessionID string `json:"viewer_session_id"`
}

func (item *Item) LikeStory(ctx context.Context) error {
	module := "reel_feed_timeline"
	if item.module != "" && item.module != "reel_feed_timeline" {
		module = item.module
	}
	q, err := signedBody(storyLikeRequest{
		sessionIDs:      item.insta.sessionIDs(),
		ContainerModule: module,
		DeliveryClass:   item.deliveryClass(),
		MediaID:         item.ID,
		TrayPosition:    strconv.Itoa(item.Index),
		TraySessionID:   generateUUID(),
		ViewerSessionID: generateUUID(),
	})
	if err != nil {
		return err
	}
	_, err = item.insta.call(ctx, &reqOptions{Endpoint: urlStoryLike, IsPost: true, Query: q})
	return err
}
