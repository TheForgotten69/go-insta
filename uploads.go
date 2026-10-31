package goinsta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type photo struct {
	uploadID string
	data     []byte
	mime     string
	w, h     int
}

func readPhoto(r io.Reader) (*photo, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	mime := http.DetectContentType(b)
	if mime != "image/jpeg" && mime != "image/webp" {
		return nil, ErrInvalidImage
	}
	w, h, err := getImageSize(b)
	if err != nil {
		return nil, err
	}
	return &photo{data: b, mime: mime, w: w, h: h}, nil
}

var lastUploadID atomic.Int64

// newUploadID is unix ms, bumped so an album and its children never share an id.
func newUploadID() string {
	for {
		last, now := lastUploadID.Load(), time.Now().UnixMilli()
		if now <= last {
			now = last + 1
		}
		if lastUploadID.CompareAndSwap(last, now) {
			return strconv.FormatInt(now, 10)
		}
	}
}

// ruploadIgnored are the app headers the rupload endpoints never carry.
var ruploadIgnored = append([]string{"X-Bloks-Version-Id", "X-Ig-Android-Id", "X-Ig-App-Locale",
	"X-Ig-Bloks-Serialize-Payload", "X-Ig-Device-Id", "X-Ig-Device-Locale", "X-Ig-Family-Device-Id",
	"X-Ig-Is-Adaptive-Ui", "X-Ig-Is-Foldable", "X-Ig-Mapped-Locale", "X-Ig-Timezone-Offset", "X-Ig-Device-Timezone",
	"X-Ig-Www-Claim", "X-Tigon-Is-Retry", "X-Ig-Bandwidth-Speed-Kbps", "X-Ig-Bandwidth-Totalbytes-B",
	"X-Ig-Bandwidth-Totaltime-Ms", "X-Ig-Client-Endpoint", "X-Pigeon-Rawclienttime", "X-Pigeon-Session-Id"},
	bloksHeaders...)

type imageCompression struct {
	LibName        string `json:"lib_name"`
	LibVersion     string `json:"lib_version"`
	Quality        string `json:"quality"`
	OriginalWidth  int    `json:"original_width"`
	OriginalHeight int    `json:"original_height"`
}

// photoRuploadParams is the X-Instagram-Rupload-Params of a photo upload.
type photoRuploadParams struct {
	UploadID               string `json:"upload_id"`
	SessionID              string `json:"session_id"`
	MediaType              string `json:"media_type"`
	UploadEngineConfigEnum string `json:"upload_engine_config_enum"`
	ShareType              string `json:"share_type"`
	IsOptimisticUpload     string `json:"is_optimistic_upload"`
	ImageCompression       string `json:"image_compression"`
	XsharingUserIDs        string `json:"xsharing_user_ids"`
	RetryContext           string `json:"retry_context"`
	StellaData             string `json:"stella_data"`
	IsSidecar              string `json:"is_sidecar,omitempty"`
}

// shareType is "feed" or "stories"; album children pass the album's client_sidecar_id as sessionID.
func (insta *Instagram) uploadPhoto(ctx context.Context, p *photo, shareType, sessionID string) error {
	p.uploadID = newUploadID()
	sidecar := sessionID != ""
	if !sidecar {
		sessionID = p.uploadID
	}
	quality := "70"
	if shareType == "stories" {
		quality = "76"
	}
	compression, err := json.Marshal(imageCompression{
		LibName: "libwebp", LibVersion: "33", Quality: quality, OriginalWidth: p.w, OriginalHeight: p.h,
	})
	if err != nil {
		return err
	}
	params := photoRuploadParams{
		UploadID:               p.uploadID,
		SessionID:              sessionID,
		MediaType:              "1",
		UploadEngineConfigEnum: "0",
		ShareType:              shareType,
		IsOptimisticUpload:     strconv.FormatBool(sidecar),
		ImageCompression:       string(compression),
		XsharingUserIDs:        "[]",
		RetryContext:           `{"num_reupload":0,"cover_photo_re_upload_count":0,"num_step_manual_retry":0,"num_step_auto_retry":0}`,
		StellaData:             "{}",
	}
	if sidecar {
		params.IsSidecar = "1"
	}
	rp, err := json.Marshal(params)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%s_0_%d", p.uploadID, rand.Int32()-rand.Int32())
	var resp struct {
		UploadID string `json:"upload_id"`
		Status   string `json:"status"`
	}
	err = insta.callJSON(ctx, &reqOptions{
		Endpoint:      fmt.Sprintf(urlUploadPhoto, name),
		OmitAPI:       true,
		IsPost:        true,
		DataBytes:     p.data,
		Priority:      "u=1, i",
		FriendlyName:  "undefined:media-upload",
		IgnoreHeaders: ruploadIgnored,
		ExtraHeaders: map[string]string{
			"Content-Type":               "application/octet-stream",
			"Offset":                     "0",
			"X-Entity-Length":            strconv.Itoa(len(p.data)),
			"X-Entity-Name":              name,
			"X-Entity-Type":              p.mime,
			"X-Instagram-Rupload-Params": string(rp),
			"X_fb_photo_waterfall_id":    generateUUID(),
			"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + insta.sess.AppID +
				`","surface":"undefined","request_category":"media_upload","purpose":"none","retry_attempt":"0"}}`,
		},
	}, &resp)
	if err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("upload %s: status %q", p.uploadID, resp.Status)
	}
	return nil
}

type configureDevice struct {
	Manufacturer   string `json:"manufacturer"`
	Model          string `json:"model"`
	AndroidVersion int    `json:"android_version"`
	AndroidRelease string `json:"android_release"`
}

func (insta *Instagram) deviceInfo() configureDevice {
	d, ok := parseUserAgent(insta.sess.UserAgent)
	if !ok {
		d = Waydroid
	}
	manufacturer, _, _ := strings.Cut(d.Manufacturer, "/")
	return configureDevice{
		Manufacturer:   manufacturer,
		Model:          d.Model,
		AndroidVersion: d.AndroidVersion,
		AndroidRelease: strconv.Itoa(d.AndroidRelease),
	}
}

// configureBase is in every configure body.
type configureBase struct {
	sessionIDs
	DeviceID            string          `json:"device_id"`
	Device              configureDevice `json:"device"`
	UploadID            string          `json:"upload_id"`
	TimezoneOffset      string          `json:"timezone_offset"`
	SourceType          string          `json:"source_type"`
	Audience            string          `json:"audience"`
	AsyncPublish        string          `json:"async_publish,omitempty"`
	PublishID           string          `json:"publish_id,omitempty"`
	HideFromProfileGrid string          `json:"hide_from_profile_grid"`
	CameraSessionID     string          `json:"camera_session_id,omitempty"`
}

func (insta *Instagram) configureBase(uploadID string) configureBase {
	return configureBase{
		sessionIDs:          insta.sessionIDs(),
		DeviceID:            insta.dID,
		Device:              insta.deviceInfo(),
		UploadID:            uploadID,
		TimezoneOffset:      insta.sess.TimezoneOffset,
		SourceType:          "4",
		Audience:            "default",
		AsyncPublish:        "1",
		PublishID:           "1",
		HideFromProfileGrid: "false",
		CameraSessionID:     generateUUID(),
	}
}

type timelineStep struct {
	Action         string `json:"action"`
	OwnerID        string `json:"owner_id,omitempty"`
	SurfaceElement string `json:"surface_element"`
	Target         string `json:"target"`
}

func stackedTimeline(uploadID string) []timelineStep {
	step := func(action string) timelineStep {
		return timelineStep{Action: action, OwnerID: uploadID, SurfaceElement: "STACKED_TIMELINE", Target: "PHOTO"}
	}
	return []timelineStep{step("ADD"), step("CROP")}
}

// mediaEdits is the configure body's edits; stories send no crop_center or crop_zoom.
type mediaEdits struct {
	CropOriginalSize []int     `json:"crop_original_size"`
	CropCenter       []float64 `json:"crop_center,omitempty"`
	CropZoom         float64   `json:"crop_zoom,omitempty"`
	FilterType       int       `json:"filter_type"`
	FilterStrength   int       `json:"filter_strength"`
}

func photoEdits(p *photo) mediaEdits {
	return mediaEdits{CropOriginalSize: []int{p.w, p.h}, CropCenter: []float64{0, 0}, CropZoom: 1, FilterStrength: 1}
}

type configureMedia struct {
	MediaDict *Item `json:"media_dict"`
}

type configureResp struct {
	Medias  []configureMedia `json:"medias"`
	Media   *Item            `json:"media"`
	Status  string           `json:"status"`
	Message string           `json:"message"`
}

func (insta *Instagram) configure(ctx context.Context, endpoint string, payload any) (*Item, error) {
	var resp configureResp
	err := insta.callJSON(ctx, &reqOptions{
		Endpoint: endpoint,
		IsPost:   true,
		Signed:   payload,
		Priority: "u=1, i",
		ExtraHeaders: map[string]string{
			"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + insta.sess.AppID +
				`","surface":"undefined","request_category":"media_upload","purpose":"fetch","retry_attempt":"0"}}`,
			"X-Ig-Eu-Configure-Disabled": "true",
		},
	}, &resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Medias) == 0 && resp.Media != nil {
		resp.Medias = []configureMedia{{MediaDict: resp.Media}}
	}
	if len(resp.Medias) == 0 || resp.Medias[0].MediaDict == nil {
		return nil, fmt.Errorf("%s: no media in response (status %q: %s)", endpoint, resp.Status, resp.Message)
	}
	item := resp.Medias[0].MediaDict
	if endpoint == urlConfigureSidecar {
		// medias lists each child, then the album itself.
		i := slices.IndexFunc(resp.Medias, func(m configureMedia) bool {
			return m.MediaDict != nil && m.MediaDict.MediaType == MediaAlbum
		})
		if i < 0 {
			return nil, fmt.Errorf("%s: no album in response (status %q)", endpoint, resp.Status)
		}
		item = resp.Medias[i].MediaDict
	}
	setToItem(item, insta, "self_profile")
	return item, nil
}

// PostOptions are the optional fields of a feed post; the zero value posts
// without a caption.
type PostOptions struct {
	Caption string
	// ShareToThreads cross-posts to the Threads profile linked to the account.
	ShareToThreads bool
	// HideLikeCounts hides the like and view counts.
	HideLikeCounts  bool
	HideShareCounts bool
	// ShareToStory also shares the post to the story as a tappable post card
	// drawn from its first image. If only the story share fails, the post is
	// returned with the error.
	ShareToStory bool
}

// postExtras are PostOptions resolved before anything is uploaded, so a failed
// render or lookup never leaves a half-published post.
type postExtras struct {
	opts  PostOptions
	fbid  int64
	story *storyCard
}

func (insta *Instagram) preparePost(ctx context.Context, opts PostOptions, first *photo) (*postExtras, error) {
	x := &postExtras{opts: opts}
	if opts.ShareToStory {
		card, err := renderStoryCard(first.data)
		if err != nil {
			return nil, err
		}
		x.story = card
	}
	if opts.ShareToThreads {
		id, err := insta.threadsDestinationID(ctx)
		if err != nil {
			return nil, err
		}
		x.fbid = id
	}
	return x, nil
}

// postFields are the PostOptions part of a configure body.
type postFields struct {
	Caption                        string   `json:"caption,omitempty"`
	ShareToThreads                 string   `json:"share_to_threads,omitempty"`
	ShareToThreadsDestinationID    string   `json:"share_to_threads_destination_id,omitempty"`
	ShareToThreadsValidationBypass []string `json:"share_to_threads_validation_bypass,omitempty"`
	LikeAndViewCountsDisabled      string   `json:"like_and_view_counts_disabled,omitempty"`
	ShareCountDisabled             string   `json:"share_count_disabled,omitempty"`
	IsAutoResharingToStory         string   `json:"is_auto_resharing_to_story,omitempty"`
}

func (x *postExtras) fields() postFields {
	f := postFields{Caption: x.opts.Caption}
	if x.opts.ShareToThreads {
		f.ShareToThreads = "1"
		f.ShareToThreadsDestinationID = strconv.FormatInt(x.fbid, 10)
		f.ShareToThreadsValidationBypass = []string{"AUTO_CROSSPOST_SETTING"}
	}
	if x.opts.HideLikeCounts {
		f.LikeAndViewCountsDisabled = "1"
	}
	if x.opts.HideShareCounts {
		f.ShareCountDisabled = "1"
	}
	if x.story != nil {
		f.IsAutoResharingToStory = "1"
	}
	return f
}

// publish configures the post, then reshares it to the story when asked.
func (insta *Instagram) publish(ctx context.Context, endpoint string, payload any, x *postExtras) (*Item, error) {
	item, err := insta.configure(ctx, endpoint, payload)
	if err != nil || x.story == nil {
		return item, err
	}
	if err := insta.reshareToStory(ctx, item, x.story); err != nil {
		return item, fmt.Errorf("post %s published, story share failed: %w", item.Code, err)
	}
	return item, nil
}

// threadsDestinationID is the account's fbid_v2, looked up once per session.
func (insta *Instagram) threadsDestinationID(ctx context.Context) (int64, error) {
	if id := insta.fbid.Load(); id != 0 {
		return id, nil
	}
	u := insta.NewUser(insta.userID())
	if err := u.Info(ctx); err != nil {
		return 0, fmt.Errorf("threads destination: %w", err)
	}
	if u.FbIdV2 == 0 {
		return 0, errors.New("threads destination: account has no fbid_v2")
	}
	insta.fbid.Store(int64(u.FbIdV2))
	return int64(u.FbIdV2), nil
}

// UploadPhoto posts a single JPEG or WebP image to the feed. Instagram stores
// the pixels as uploaded, so crop to the wanted aspect ratio first.
func (insta *Instagram) UploadPhoto(ctx context.Context, r io.Reader, opts PostOptions) (*Item, error) {
	p, err := readPhoto(r)
	if err != nil {
		return nil, err
	}
	x, err := insta.preparePost(ctx, opts, p)
	if err != nil {
		return nil, err
	}
	if err := insta.uploadPhoto(ctx, p, "feed", ""); err != nil {
		return nil, err
	}
	return insta.publish(ctx, urlConfigure, photoConfigure{
		configureBase:           insta.configureBase(p.uploadID),
		postFields:              x.fields(),
		Edits:                   photoEdits(p),
		Extra:                   sourceSize{Width: p.w, Height: p.h},
		OriginalWidth:           strconv.Itoa(p.w),
		OriginalHeight:          strconv.Itoa(p.h),
		CreationToolInfo:        "[]",
		StackedTimelineMetadata: stackedTimeline(p.uploadID),
	}, x)
}

type photoConfigure struct {
	configureBase
	postFields
	SceneCaptureType               string         `json:"scene_capture_type"`
	Edits                          mediaEdits     `json:"edits"`
	Extra                          sourceSize     `json:"extra"`
	OriginalWidth                  string         `json:"original_width"`
	OriginalHeight                 string         `json:"original_height"`
	CreationToolInfo               string         `json:"creation_tool_info"`
	AppAttributionAndroidNamespace string         `json:"app_attribution_android_namespace"`
	StackedTimelineMetadata        []timelineStep `json:"stacked_timeline_metadata"`
}

const maxAlbumPhotos = 20

// UploadAlbum posts 2 to 20 JPEG or WebP images as one carousel. All images are
// checked before any upload starts; the uploads then run in parallel. Instagram
// stores each image as uploaded and takes the album's shape from the first one
// (1080x1350 gives a 4:5 album), so crop them all to one ratio first.
func (insta *Instagram) UploadAlbum(ctx context.Context, photos []io.Reader, opts PostOptions) (*Item, error) {
	if len(photos) < 2 || len(photos) > maxAlbumPhotos {
		return nil, ErrCarouselMediaLimit
	}
	ps := make([]*photo, len(photos))
	for i, r := range photos {
		p, err := readPhoto(r)
		if err != nil {
			return nil, fmt.Errorf("photo %d: %w", i+1, err)
		}
		ps[i] = p
	}
	x, err := insta.preparePost(ctx, opts, ps[0])
	if err != nil {
		return nil, err
	}
	sidecarID := newUploadID()
	errs := make([]error, len(ps))
	var wg sync.WaitGroup
	for i, p := range ps {
		wg.Go(func() { errs[i] = insta.uploadPhoto(ctx, p, "feed", sidecarID) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	device := jsonText[configureDevice]{insta.deviceInfo()}
	children := make([]albumChild, len(ps))
	for i, p := range ps {
		children[i] = albumChild{
			UploadID:                 p.uploadID,
			Device:                   device,
			Edits:                    jsonText[mediaEdits]{photoEdits(p)},
			Extra:                    jsonText[sourceSize]{sourceSize{Width: p.w, Height: p.h}},
			OriginalWidth:            strconv.Itoa(p.w),
			OriginalHeight:           strconv.Itoa(p.h),
			SourceType:               "4",
			Audience:                 "default",
			TimezoneOffset:           insta.sess.TimezoneOffset,
			CreationToolInfo:         "[]",
			HideFromProfileGrid:      "false",
			BottomCameraDialSelected: "11",
			StackedTimelineMetadata:  jsonText[[]timelineStep]{stackedTimeline(p.uploadID)},
		}
	}
	return insta.publish(ctx, urlConfigureSidecar, albumConfigure{
		configureBase:                insta.configureBase(sidecarID),
		postFields:                   x.fields(),
		ClientSidecarID:              sidecarID,
		ChildrenMetadata:             children,
		CarouselMixedAspectRatioType: "0",
		StackedTimelineMetadata:      []timelineStep{{Action: "ADD", SurfaceElement: "STACKED_TIMELINE", Target: "MULTI_PHOTO"}},
		OriginalWidth:                "0",
		OriginalHeight:               "0",
	}, x)
}

// albumConfigure has no scene_capture_type at the top level, unlike its children.
type albumConfigure struct {
	configureBase
	postFields
	ClientSidecarID              string         `json:"client_sidecar_id"`
	ChildrenMetadata             []albumChild   `json:"children_metadata"`
	CarouselMixedAspectRatioType string         `json:"carousel_mixed_aspect_ratio_type"`
	StackedTimelineMetadata      []timelineStep `json:"stacked_timeline_metadata"`
	OriginalWidth                string         `json:"original_width"`
	OriginalHeight               string         `json:"original_height"`
}

type albumChild struct {
	UploadID                       string                    `json:"upload_id"`
	Device                         jsonText[configureDevice] `json:"device"`
	Edits                          jsonText[mediaEdits]      `json:"edits"`
	Extra                          jsonText[sourceSize]      `json:"extra"`
	OriginalWidth                  string                    `json:"original_width"`
	OriginalHeight                 string                    `json:"original_height"`
	SourceType                     string                    `json:"source_type"`
	Audience                       string                    `json:"audience"`
	TimezoneOffset                 string                    `json:"timezone_offset"`
	CreationToolInfo               string                    `json:"creation_tool_info"`
	AppAttributionAndroidNamespace string                    `json:"app_attribution_android_namespace"`
	HideFromProfileGrid            string                    `json:"hide_from_profile_grid"`
	SceneCaptureType               string                    `json:"scene_capture_type"`
	BottomCameraDialSelected       string                    `json:"bottom_camera_dial_selected"`
	StackedTimelineMetadata        jsonText[[]timelineStep]  `json:"stacked_timeline_metadata"`
}

// jsonText marshals V as a JSON string holding V's JSON, the form album
// children use for nested objects.
type jsonText[T any] struct{ V T }

func (j jsonText[T]) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(j.V)
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(b))
}

// UploadStory posts a JPEG or WebP image to the story.
func (insta *Instagram) UploadStory(ctx context.Context, r io.Reader) (*Item, error) {
	p, err := readPhoto(r)
	if err != nil {
		return nil, err
	}
	if err := insta.uploadPhoto(ctx, p, "stories", ""); err != nil {
		return nil, err
	}
	req := insta.storyBase(p)
	req.CaptureType = "normal"
	req.CompositionID = generateUUID()
	req.MediaTransformationInfo = fmt.Sprintf(
		`{"width":"%d","height":"%d","x_transform":"0","y_transform":"0","zoom":"1.0","rotation":"0.0","background_coverage":100}`, p.w, p.h)
	return insta.configure(ctx, urlConfigureStory, req)
}

// storyConfigure is a configure_to_story body: a plain story upload, or a feed
// post reshared as a sticker.
type storyConfigure struct {
	configureBase
	SceneCaptureType         string     `json:"scene_capture_type"`
	Edits                    mediaEdits `json:"edits"`
	Extra                    sourceSize `json:"extra"`
	OriginalWidth            string     `json:"original_width"`
	OriginalHeight           string     `json:"original_height"`
	OriginalMediaType        string     `json:"original_media_type"`
	ConfigureMode            string     `json:"configure_mode"`
	CreationSurface          string     `json:"creation_surface"`
	CameraPosition           string     `json:"camera_position"`
	ClientSharedAt           string     `json:"client_shared_at"`
	ClientTimestamp          string     `json:"client_timestamp"`
	SupportedCapabilitiesNew string     `json:"supported_capabilities_new"`
	CaptureType              string     `json:"capture_type"`

	CompositionID           string `json:"composition_id,omitempty"`
	MediaTransformationInfo string `json:"media_transformation_info,omitempty"`

	ResharedMediaID        string `json:"reshared_media_id,omitempty"`
	IsAutoResharingToStory string `json:"is_auto_resharing_to_story,omitempty"`
	ImportedTakenAt        string `json:"imported_taken_at,omitempty"`
	HasCameraMetadata      string `json:"has_camera_metadata,omitempty"`
	HasOriginalSound       string `json:"has_original_sound,omitempty"`
	TapModels              string `json:"tap_models,omitempty"`
}

func (insta *Instagram) storyBase(p *photo) storyConfigure {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	return storyConfigure{
		configureBase:            insta.configureBase(p.uploadID),
		Edits:                    mediaEdits{CropOriginalSize: []int{p.w, p.h}, FilterStrength: 1},
		Extra:                    sourceSize{Width: p.w, Height: p.h},
		OriginalWidth:            strconv.Itoa(p.w),
		OriginalHeight:           strconv.Itoa(p.h),
		OriginalMediaType:        "1",
		ConfigureMode:            "1",
		CreationSurface:          "camera",
		CameraPosition:           "unknown",
		ClientSharedAt:           now,
		ClientTimestamp:          now,
		SupportedCapabilitiesNew: supportedCapabilities,
	}
}

// postSticker is the tappable post card on a reshared story.
type postSticker struct {
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Z              int     `json:"z"`
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
	Rotation       float64 `json:"rotation"`
	Type           string  `json:"type"`
	MediaID        string  `json:"media_id"`
	MediaOwnerID   string  `json:"media_owner_id"`
	CarouselChild  string  `json:"carousel_share_child_media_id"`
	ProductType    string  `json:"product_type"`
	IsSticker      bool    `json:"is_sticker"`
	TapState       int     `json:"tap_state"`
	TapStateString string  `json:"tap_state_str_id"`
}

func (insta *Instagram) reshareToStory(ctx context.Context, post *Item, card *storyCard) error {
	p := card.photo
	if err := insta.uploadPhoto(ctx, p, "stories", ""); err != nil {
		return err
	}
	pk := strconv.FormatInt(int64(post.Pk), 10)
	// The app sent product_type carousel_container for a single photo too.
	tap, err := json.Marshal([]postSticker{{
		X: 0.5, Y: 0.5, Width: card.w, Height: card.h,
		Type: "feed_media", MediaID: pk, MediaOwnerID: strconv.FormatInt(insta.userID(), 10),
		ProductType: "carousel_container", IsSticker: true, TapStateString: "feed_post_sticker_square",
	}})
	if err != nil {
		return err
	}
	req := insta.storyBase(p)
	req.CaptureType = "unknown"
	req.CameraSessionID = ""
	req.SourceType = "feed_reshare"
	req.ResharedMediaID = pk
	req.IsAutoResharingToStory = "1"
	req.ImportedTakenAt = req.ClientSharedAt
	req.HasCameraMetadata = "0"
	req.HasOriginalSound = "1"
	req.TapModels = string(tap)
	_, err = insta.configure(ctx, urlConfigureStory, req)
	return err
}
