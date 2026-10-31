package goinsta

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Captured client_doc_ids (app 453.0.0.55.79); they change with app versions.
const (
	docProfileTimeline = "56030350814162075904140659961"
	docFollowersList   = "284797047911918316998205836755"
	docFollowingList   = "16104639286363954576550227636"

	docArchiveHighlight   = "355043344414417317058010037446"
	docUnarchiveHighlight = "52990869615876622121615642083"

	// client_doc_ids from assets/ig4a-instagram-schema_client_persist_ids.json; the
	// "instagram" schema goes to graphql_www, "distillery" to graphql/query.
	docPendingFollows       = "33138429313628334965326831157"
	docApproveFollowRequest = "356404437015335956218183369113"
	docIgnoreFollowRequest  = "1956506636707216159350023433"

	docPostInsights      = "187673829817521675190278983686"
	rootPostInsights     = "ig_post_insights"
	friendlyPostInsights = "PostInsightsQuery"

	docTranslationsUser             = "156105369516447605998684255875"
	docUpsertTranslationsCreation   = "35466444201443961438414953916"
	rootTranslationsUser            = "fetch__XDTUserDict"
	rootUpsertTranslationsCreation  = "xig_upsert_user_translations_creation_settings"
	friendlyTranslationsUser        = "TranslationsUserQuery"
	friendlyUpsertTranslationsInput = "UpsertUserTranslationsCreationSettings"

	docLongformInsights      = "142319532318085841412086369996"
	friendlyLongformInsights = "LongformInsightsQuery"

	docMediaInsightsGrid      = "13476930769765169094605715723"
	friendlyMediaInsightsGrid = "IGInsightsIntegratedMediaGridQuery"

	docCommentInsightsOptIn         = "1221062957381201146258962681"
	rootCommentInsightsOptIn        = "xdt_get_comment_summary_opt_in_status_by_user_id"
	friendlyCommentInsightsOptIn    = "IGCommentInsightsOptinStatusQuery"
	docSetCommentInsightsOptIn      = "34077348003540024418408340205"
	rootSetCommentInsightsOptIn     = "xdt_update_comment_summary_opt_in_by_user_id"
	friendlySetCommentInsightsOptIn = "IGCommentInsightsOptinStatusMutation"

	docSeriesInsightsOverview        = "383515635314126103399555900562"
	docSeriesInsightsEngagement      = "281083989014079368110826164376"
	docSeriesInsightsAudience        = "386340097017018189326628161392"
	docFanClubInsights               = "147178694610827473488858200058"
	friendlySeriesInsightsOverview   = "SeriesInsightsOverviewQuery"
	friendlySeriesInsightsEngagement = "SeriesInsightsEngagementQuery"
	friendlySeriesInsightsAudience   = "SeriesInsightsAudienceQuery"
	friendlyFanClubInsights          = "FanClubInsights"
	rootLinkedMediaPlaylist          = "fetch__XIGLinkedMediaPlaylist"
	rootFanClubInsights              = "node"

	rootAccountInsights                 = "ig_account_insights"
	docAccountInsightsHome              = "38444011412098991622225483328"
	docAccountInsightsOverview          = "297851856213787890962497531867"
	docAccountInsightsAudience          = "11313478377027549324232575805"
	docAccountInsightsContent           = "246019450711285746558777381992"
	docAccountInsightsMediaComponent    = "348470172667229184692559590"
	docAccountInsightsBioLinks          = "241501507416321773369365046568"
	docAccountInsightsPreviousBioLinks  = "237951042513594934414540638102"
	docAccountInsightsViewerState       = "16422176244090456288187318494"
	friendlyAccountInsightsHome         = "AccountInsightsHomeQuery"
	friendlyAccountInsightsOverview     = "AccountInsightsOverviewQuery"
	friendlyAccountInsightsAudience     = "AccountInsightsAudienceQuery"
	friendlyAccountInsightsContent      = "AccountInsightsContentQuery"
	friendlyAccountInsightsMedia        = "AccountInsightsMediaComponentQuery"
	friendlyAccountInsightsBioLinks     = "AccountInsightsBioLinksQuery"
	friendlyAccountInsightsPrevBioLinks = "AccountInsightsPreviousBioLinksQuery"
	friendlyAccountInsightsViewerState  = "AccountInsightsViewerStateQuery"
)

type gqlQuery struct {
	url      string // graphQLURL when empty
	friendly string
	docID    string
	root     string
	purpose  string // "fetch", "refresh" or "prefetch"; "" omits it, as list and search queries do
	gzip     bool   // per query in the captures, not per endpoint
}

type graphQLForm struct {
	ClientDocID      string `json:"client_doc_id"`
	FriendlyName     string `json:"fb_api_req_friendly_name"`
	Format           string `json:"format"`
	Locale           string `json:"locale"`
	Method           string `json:"method"`
	ServerTimestamps string `json:"server_timestamps"`
	Variables        string `json:"variables"`
	Purpose          string `json:"purpose,omitempty"`
}

func (insta *Instagram) graphQL(ctx context.Context, gq gqlQuery, vars any) ([]byte, error) {
	url := cmp.Or(gq.url, graphQLURL)
	v, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}
	req := graphQLForm{
		ClientDocID:      gq.docID,
		FriendlyName:     gq.friendly,
		Format:           "json",
		Locale:           "user",
		Method:           "post",
		ServerTimestamps: "true",
		Variables:        string(v),
		Purpose:          gq.purpose,
	}
	headers := map[string]string{
		"Content-Type":             "application/x-www-form-urlencoded",
		"X-Client-Doc-Id":          gq.docID,
		validateNullHeader(url):    "true",
		"X-Graphql-Client-Library": "pando",
		"X-Root-Field-Name":        gq.root,
	}
	tagPurpose := "none"
	if gq.purpose != "" {
		headers["X-Graphql-Request-Purpose"] = gq.purpose
		tagPurpose = gq.purpose
	}
	headers["X-Fb-Request-Analytics-Tags"] = `{"network_tags":{"product":"` + insta.sess.AppID +
		`","request_category":"graphql","purpose":"` + tagPurpose + `","retry_attempt":"0"}}`
	body, err := insta.call(ctx, &reqOptions{
		Endpoint: url,
		IsPost:   true,
		Gzip:     gq.gzip,
		Priority: "u=3, i",
		IgnoreHeaders: append([]string{"X-Ig-Www-Claim", "X-Ig-Family-Device-Id", "X-Fb-Connection-Type",
			"X-Ig-Connection-Type", "X-Ig-Client-Endpoint", "X-Ig-Bloks-Serialize-Payload",
			"X-Ig-Bandwidth-Speed-Kbps", "X-Ig-Bandwidth-Totalbytes-B", "X-Ig-Bandwidth-Totaltime-Ms"}, bloksHeaders...),
		FriendlyName: gq.friendly,
		Form:         req,
		ExtraHeaders: headers,
	})
	if err != nil {
		return nil, err
	}
	return mergeIncremental(body)
}

// mergeIncremental folds a @defer/@stream response (NDJSON: a skeleton, then
// {"path":[...],"data":{...}} patches) into one {"data":...} document.
func mergeIncremental(body []byte) ([]byte, error) {
	lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
	if len(lines) < 2 {
		return body, nil
	}
	var root struct {
		Data map[string]any `json:"data"`
	}
	if err := decodeNumber(lines[0], &root); err != nil {
		return nil, err
	}
	for _, line := range lines[1:] {
		var patch struct {
			Path   []any          `json:"path"`
			Data   any            `json:"data"`
			Errors []GraphQLError `json:"errors"`
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := decodeNumber(line, &patch); err != nil {
			return nil, err
		}
		if patch.Data == nil {
			if len(patch.Errors) > 0 {
				return nil, &patch.Errors[0]
			}
			continue
		}
		if err := applyPatch(root.Data, patch.Path, patch.Data); err != nil {
			return nil, err
		}
	}
	return json.Marshal(root)
}

// GraphQLError is the first error of a response part that came back without
// data. Instagram answers 1357005 for a private account the viewer does not follow.
type GraphQLError struct {
	Code    int    `json:"code"`
	Summary string `json:"summary"`
	Message string `json:"message"`
}

func (e *GraphQLError) Error() string {
	return fmt.Sprintf("graphql error %d: %s: %s", e.Code, e.Summary, e.Message)
}

func decodeNumber(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}

// validateNullHeader is the null-validation header the app sends to each endpoint.
func validateNullHeader(url string) string {
	if url == graphQLURL {
		return "X-Ig-Validate-Null-In-Legacy-Eager-Dict"
	}
	return "X-Ig-Validate-Null-In-Legacy-Dict"
}

func applyPatch(node any, path []any, data any) error {
	for _, step := range path {
		switch n := node.(type) {
		case map[string]any:
			k, _ := step.(string)
			v, ok := n[k]
			if !ok {
				// The skeleton may key a root field "1$<field>" while patches use "<field>".
				for key, val := range n {
					if isRootKey(key, k) {
						v = val
						break
					}
				}
			}
			node = v
		case []any:
			i, err := step.(json.Number).Int64()
			if err != nil || i < 0 || int(i) >= len(n) {
				return fmt.Errorf("graphql patch: bad index %v", step)
			}
			node = n[i]
		default:
			return fmt.Errorf("graphql patch: path %v does not resolve", path)
		}
	}
	dst, ok := node.(map[string]any)
	src, ok2 := data.(map[string]any)
	if !ok || !ok2 {
		return fmt.Errorf("graphql patch: path %v is not an object", path)
	}
	deepMerge(dst, src)
	return nil
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if sm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				deepMerge(dm, sm)
				continue
			}
		}
		dst[k] = v
	}
}

// isRootKey reports whether key names field name, as is or aliased
// ("1$name" or "1$name(args)").
func isRootKey(key, name string) bool {
	if key == name {
		return true
	}
	_, alias, ok := strings.Cut(key, "$")
	return ok && (alias == name || strings.HasPrefix(alias, name+"("))
}

// graphQLData runs the query and returns its data fields and its root: gq.root,
// or with none the first non-null field. A null or missing root is the
// response's first error, else ErrEmptyResponse.
func (insta *Instagram) graphQLData(ctx context.Context, gq gqlQuery, vars any) (map[string]json.RawMessage, json.RawMessage, error) {
	body, err := insta.graphQL(ctx, gq, vars)
	if err != nil {
		return nil, nil, err
	}
	var resp struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []GraphQLError             `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, err
	}
	for k, v := range resp.Data {
		if string(v) == "null" || (gq.root == "" && k == "__typename") || (gq.root != "" && !isRootKey(k, gq.root)) {
			continue
		}
		return resp.Data, v, nil
	}
	if len(resp.Errors) > 0 {
		return nil, nil, fmt.Errorf("%s: %w", gq.friendly, &resp.Errors[0])
	}
	return nil, nil, fmt.Errorf("%s: %w", gq.friendly, ErrEmptyResponse)
}

// graphQLRoot runs the query and decodes its root (see graphQLData) into out.
func (insta *Instagram) graphQLRoot(ctx context.Context, gq gqlQuery, vars, out any) error {
	_, root, err := insta.graphQLData(ctx, gq, vars)
	if err != nil {
		return err
	}
	return json.Unmarshal(root, out)
}

// graphQLInto runs the query and returns its root decoded as a T.
func graphQLInto[T any](ctx context.Context, insta *Instagram, gq gqlQuery, vars any) (*T, error) {
	var out T
	if err := insta.graphQLRoot(ctx, gq, vars, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// connection is a GraphQL edges/node list.
type connection[T any] struct {
	Edges []struct {
		Node T `json:"node"`
	} `json:"edges"`
	PageInfo graphQLPageInfo `json:"page_info"`
}

type graphQLPageInfo struct {
	EndCursor   string `json:"end_cursor"`
	HasNextPage bool   `json:"has_next_page"`
}

func (c connection[T]) nodes() []T {
	out := make([]T, len(c.Edges))
	for i, e := range c.Edges {
		out[i] = e.Node
	}
	return out
}

func (c connection[T]) next() string { return c.PageInfo.cursor() }

func (p graphQLPageInfo) cursor() string { return cursorIf(p.HasNextPage, p.EndCursor) }

// graphQLAck runs a mutation whose root only acknowledges it.
func (insta *Instagram) graphQLAck(ctx context.Context, gq gqlQuery, vars any) error {
	_, _, err := insta.graphQLData(ctx, gq, vars)
	return err
}

func (user *User) Posts() *Pager[*Item] {
	var maxID, cursor string
	var pos int
	return newPager(func(ctx context.Context) ([]*Item, string, error) {
		vars := newProfileTimelineVars(user.ID)
		if maxID != "" {
			pinned := ""
			vars.MaxID, vars.ProfileGridItemsCursor, vars.PinnedProfileGridItemsIDs = &maxID, &cursor, &pinned
			vars.RequestMediaChunk = false
		}
		var tl profileTimeline
		err := user.insta.graphQLRoot(ctx, gqlQuery{friendly: "IGProfileTimelineQuery", docID: docProfileTimeline,
			root: "xdt_api__v1__profile_timeline", purpose: "fetch", gzip: true}, vars, &tl)
		if gqlErr := (*GraphQLError)(nil); user.IsPrivate && errors.As(err, &gqlErr) {
			err = fmt.Errorf("%w: %w", ErrPrivateAccount, err)
		}
		if err != nil {
			return nil, "", err
		}
		page := tl.page(user.insta, pos)
		maxID, cursor = page.nextMaxID, page.gridCursor
		pos += len(page.items)
		return page.items, cursorIf(page.more, maxID), nil
	})
}

// profileTimelineVars: the pointer fields are set, possibly to "", on later pages only.
type profileTimelineVars struct {
	RequestMediaChunk                              bool    `json:"request_media_chunk"`
	RequestHintsChunk                              bool    `json:"request_hints_chunk"`
	DeferHintsChunk                                bool    `json:"defer_hints_chunk"`
	SkipIgPlayCount                                bool    `json:"skip_ig_play_count"`
	EnableCarouselMediaCountInDeferred             bool    `json:"enable_carousel_media_count_in_deferred"`
	SkipClipsCaptionsFields                        bool    `json:"skip_clips_captions_fields"`
	NumPreviewsForAssociatedHighlights             int     `json:"num_previews_for_associated_highlights"`
	EnableProfileFmIntegration                     bool    `json:"enable_profile_fm_integration"`
	IncludeProfileGridRenderingOption              bool    `json:"include_profile_grid_rendering_option"`
	Count                                          int     `json:"count"`
	IncludeFbMentionedUsers                        bool    `json:"include_fb_mentioned_users"`
	IncludeAssociatedHighlights                    bool    `json:"include_associated_highlights"`
	IncludeMediaNotesFields                        bool    `json:"include_media_notes_fields"`
	IncludeAccessibilityCaptionForCarousel         bool    `json:"include_accessibility_caption_for_carousel"`
	IncludeEligibleInsightsEntrypoints             bool    `json:"include_eligible_insights_entrypoints"`
	IncludeVideosForAssociatedHighlights           bool    `json:"include_videos_for_associated_highlights"`
	ExcludeUser                                    bool    `json:"exclude_user"`
	ExcludeCollaborativePosts                      bool    `json:"exclude_collaborative_posts"`
	IncludeIsPhotoCommentsComposerEnabledForAuthor bool    `json:"include_is_photo_comments_composer_enabled_for_author"`
	InitialCountCarouselMedia                      int     `json:"initial_count_carousel_media"`
	FetchProfileGridItems                          bool    `json:"fetch_profile_grid_items"`
	IncludeHorizonAttributionInfo                  bool    `json:"include_horizon_attribution_info"`
	ForceComputeUserTags                           bool    `json:"force_compute_user_tags"`
	ExcludePinnedPosts                             bool    `json:"exclude_pinned_posts"`
	IncludeRingCreatorFields                       bool    `json:"include_ring_creator_fields"`
	ExcludeHighlights                              bool    `json:"exclude_highlights"`
	IncludeIsUnseenByViewer                        bool    `json:"include_is_unseen_by_viewer"`
	IncludeTimelineOrderedEdge                     bool    `json:"include_timeline_ordered_edge"`
	ExcludeBestiesContent                          bool    `json:"exclude_besties_content"`
	UserID                                         string  `json:"user_id"`
	IncludeHiddenFeedProductTags                   bool    `json:"include_hidden_feed_product_tags"`
	EnabledFeaturedPinUserIDs                      bool    `json:"enabled_featured_pin_user_ids"`
	ExcludeComment                                 string  `json:"exclude_comment"`
	IncludeHiddenReelProductTags                   bool    `json:"include_hidden_reel_product_tags"`
	MaxID                                          *string `json:"max_id,omitempty"`
	ProfileGridItemsCursor                         *string `json:"profile_grid_items_cursor,omitempty"`
	PinnedProfileGridItemsIDs                      *string `json:"pinned_profile_grid_items_ids,omitempty"`
}

func newProfileTimelineVars(id Int64) profileTimelineVars {
	return profileTimelineVars{
		RequestMediaChunk:                  true,
		RequestHintsChunk:                  true,
		DeferHintsChunk:                    true,
		NumPreviewsForAssociatedHighlights: 3,
		Count:                              12,
		IncludeFbMentionedUsers:            true,
		IncludeEligibleInsightsEntrypoints: true,
		ExcludeUser:                        true,
		InitialCountCarouselMedia:          1,
		FetchProfileGridItems:              true,
		UserID:                             id.String(),
		ExcludeComment:                     "true",
	}
}

type profilePage struct {
	items                 []*Item
	more                  bool
	nextMaxID, gridCursor string
}

type profileTimeline struct {
	Grid []struct {
		Media *struct {
			*Item
			TakenAt Int64 `json:"1ltaken_at"`
		} `json:"media"`
	} `json:"profile_grid_items"`
	MoreAvailable bool    `json:"more_available"`
	NextMaxID     *string `json:"next_max_id"`
	GridCursor    *string `json:"profile_grid_items_cursor"`
}

func (tl *profileTimeline) page(insta *Instagram, pos int) *profilePage {
	page := &profilePage{more: tl.MoreAvailable, items: make([]*Item, 0, len(tl.Grid))}
	if tl.NextMaxID != nil {
		page.nextMaxID = *tl.NextMaxID
	}
	if tl.GridCursor != nil {
		page.gridCursor = *tl.GridCursor
	}
	for _, g := range tl.Grid {
		if g.Media == nil || g.Media.Item == nil {
			continue
		}
		it := g.Media.Item
		if it.TakenAt == 0 {
			it.TakenAt = g.Media.TakenAt
		}
		if it.Pk == 0 {
			pk, _ := strconv.ParseInt(it.pk(), 10, 64)
			it.Pk = Int64(pk)
		}
		setToItem(it, insta, "profile")
		it.Index = pos + len(page.items)
		page.items = append(page.items, it)
	}
	return page
}
