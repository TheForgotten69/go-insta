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
	compression, _ := json.Marshal(map[string]any{
		"lib_name": "libwebp", "lib_version": "33", "quality": quality,
		"original_width": p.w, "original_height": p.h,
	})
	params := map[string]string{
		"upload_id":                 p.uploadID,
		"session_id":                sessionID,
		"media_type":                "1",
		"upload_engine_config_enum": "0",
		"share_type":                shareType,
		"is_optimistic_upload":      strconv.FormatBool(sidecar),
		"image_compression":         string(compression),
		"xsharing_user_ids":         "[]",
		"retry_context":             `{"num_reupload":0,"cover_photo_re_upload_count":0,"num_step_manual_retry":0,"num_step_auto_retry":0}`,
		"stella_data":               "{}",
	}
	if sidecar {
		params["is_sidecar"] = "1"
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
		Endpoint:     fmt.Sprintf(urlUploadPhoto, name),
		OmitAPI:      true,
		IsPost:       true,
		DataBytes:    p.data,
		Priority:     "u=6, i",
		FriendlyName: "undefined:media-upload",
		IgnoreHeaders: append([]string{"X-Bloks-Version-Id", "X-Ig-Android-Id", "X-Ig-App-Locale",
			"X-Ig-Bloks-Serialize-Payload", "X-Ig-Device-Id", "X-Ig-Device-Locale", "X-Ig-Family-Device-Id",
			"X-Ig-Is-Adaptive-Ui", "X-Ig-Is-Foldable", "X-Ig-Mapped-Locale", "X-Ig-Timezone-Offset",
			"X-Ig-Www-Claim", "X-Tigon-Is-Retry", "X-Ig-Bandwidth-Speed-Kbps", "X-Ig-Bandwidth-Totalbytes-B",
			"X-Ig-Bandwidth-Totaltime-Ms", "X-Ig-Client-Endpoint", "X-Pigeon-Rawclienttime", "X-Pigeon-Session-Id"},
			bloksHeaders...),
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

func (insta *Instagram) deviceInfo() map[string]any {
	d, ok := parseUserAgent(insta.sess.UserAgent)
	if !ok {
		d = Waydroid
	}
	manufacturer, _, _ := strings.Cut(d.Manufacturer, "/")
	return map[string]any{
		"manufacturer":    manufacturer,
		"model":           d.Model,
		"android_version": d.AndroidVersion,
		"android_release": strconv.Itoa(d.AndroidRelease),
	}
}

func (insta *Instagram) configureBase(uploadID string) map[string]any {
	return map[string]any{
		"_uid":                   strconv.FormatInt(insta.userID(), 10),
		"_uuid":                  insta.uuid,
		"device_id":              insta.dID,
		"device":                 insta.deviceInfo(),
		"upload_id":              uploadID,
		"timezone_offset":        insta.sess.TimezoneOffset,
		"source_type":            "4",
		"audience":               "default",
		"async_publish":          "1",
		"publish_id":             "1",
		"hide_from_profile_grid": "false",
		"scene_capture_type":     "",
		"camera_session_id":      generateUUID(),
	}
}

func stackedTimeline(uploadID string) []map[string]string {
	step := func(action string) map[string]string {
		return map[string]string{"action": action, "owner_id": uploadID, "surface_element": "STACKED_TIMELINE", "target": "PHOTO"}
	}
	return []map[string]string{step("ADD"), step("CROP")}
}

func photoEdits(p *photo) (edits, extra map[string]any) {
	edits = map[string]any{
		"crop_original_size": []int{p.w, p.h},
		"crop_center":        []float64{0, 0},
		"crop_zoom":          1.0,
		"filter_type":        0,
		"filter_strength":    1,
	}
	extra = map[string]any{"source_width": p.w, "source_height": p.h}
	return edits, extra
}

type configureMedia struct {
	MediaDict *Item `json:"media_dict"`
}

type configureResp struct {
	Medias []configureMedia `json:"medias"`
	Status string           `json:"status"`
}

func (insta *Instagram) configure(ctx context.Context, endpoint string, payload map[string]any) (*Item, error) {
	q, err := signedBody(payload)
	if err != nil {
		return nil, err
	}
	var resp configureResp
	err = insta.callJSON(ctx, &reqOptions{
		Endpoint: endpoint,
		IsPost:   true,
		Query:    q,
		Priority: "u=6, i",
		ExtraHeaders: map[string]string{"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + insta.sess.AppID +
			`","surface":"undefined","request_category":"media_upload","purpose":"fetch","retry_attempt":"0"}}`},
	}, &resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Medias) == 0 || resp.Medias[0].MediaDict == nil {
		return nil, fmt.Errorf("%s: no media in response (status %q)", endpoint, resp.Status)
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
}

func (o PostOptions) apply(payload map[string]any) {
	if o.Caption != "" {
		payload["caption"] = o.Caption
	}
}

// UploadPhoto posts a single JPEG or WebP image to the feed.
func (insta *Instagram) UploadPhoto(ctx context.Context, r io.Reader, opts PostOptions) (*Item, error) {
	p, err := readPhoto(r)
	if err != nil {
		return nil, err
	}
	if err := insta.uploadPhoto(ctx, p, "feed", ""); err != nil {
		return nil, err
	}
	payload := insta.configureBase(p.uploadID)
	payload["edits"], payload["extra"] = photoEdits(p)
	payload["original_width"] = strconv.Itoa(p.w)
	payload["original_height"] = strconv.Itoa(p.h)
	payload["creation_tool_info"] = "[]"
	payload["app_attribution_android_namespace"] = ""
	payload["stacked_timeline_metadata"] = stackedTimeline(p.uploadID)
	opts.apply(payload)
	return insta.configure(ctx, urlConfigure, payload)
}

// UploadAlbum posts 2 to 10 JPEG or WebP images as one carousel. All images are
// checked before any upload starts; the uploads then run in parallel.
func (insta *Instagram) UploadAlbum(ctx context.Context, photos []io.Reader, opts PostOptions) (*Item, error) {
	if len(photos) < 2 || len(photos) > 10 {
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

	device, err := json.Marshal(insta.deviceInfo())
	if err != nil {
		return nil, err
	}
	children := make([]map[string]any, len(ps))
	for i, p := range ps {
		edits, extra := photoEdits(p)
		editsJSON, _ := json.Marshal(edits)
		extraJSON, _ := json.Marshal(extra)
		stackedJSON, _ := json.Marshal(stackedTimeline(p.uploadID))
		// Children carry device/edits/extra as JSON strings, unlike the top level.
		children[i] = map[string]any{
			"upload_id":                         p.uploadID,
			"device":                            string(device),
			"edits":                             string(editsJSON),
			"extra":                             string(extraJSON),
			"original_width":                    strconv.Itoa(p.w),
			"original_height":                   strconv.Itoa(p.h),
			"source_type":                       "4",
			"audience":                          "default",
			"timezone_offset":                   insta.sess.TimezoneOffset,
			"creation_tool_info":                "[]",
			"app_attribution_android_namespace": "",
			"hide_from_profile_grid":            "false",
			"scene_capture_type":                "",
			"bottom_camera_dial_selected":       "11",
			"stacked_timeline_metadata":         string(stackedJSON),
		}
	}
	payload := insta.configureBase(sidecarID)
	delete(payload, "scene_capture_type")
	payload["client_sidecar_id"] = sidecarID
	payload["children_metadata"] = children
	payload["carousel_mixed_aspect_ratio_type"] = "0"
	payload["stacked_timeline_metadata"] = []map[string]string{{"action": "ADD", "surface_element": "STACKED_TIMELINE", "target": "MULTI_PHOTO"}}
	payload["original_width"] = "0"
	payload["original_height"] = "0"
	opts.apply(payload)
	return insta.configure(ctx, urlConfigureSidecar, payload)
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
	now := strconv.FormatInt(time.Now().Unix(), 10)
	payload := insta.configureBase(p.uploadID)
	payload["edits"] = map[string]any{"crop_original_size": []int{p.w, p.h}, "filter_type": 0, "filter_strength": 1}
	payload["extra"] = map[string]any{"source_width": p.w, "source_height": p.h}
	payload["original_width"] = strconv.Itoa(p.w)
	payload["original_height"] = strconv.Itoa(p.h)
	payload["original_media_type"] = "1"
	payload["configure_mode"] = "1"
	payload["capture_type"] = "normal"
	payload["creation_surface"] = "camera"
	payload["camera_position"] = "unknown"
	payload["client_shared_at"] = now
	payload["client_timestamp"] = now
	payload["composition_id"] = generateUUID()
	payload["supported_capabilities_new"] = supportedCapabilities
	payload["media_transformation_info"] = fmt.Sprintf(
		`{"width":"%d","height":"%d","x_transform":"0","y_transform":"0","zoom":"1.0","rotation":"0.0","background_coverage":100}`, p.w, p.h)
	return insta.configure(ctx, urlConfigureStory, payload)
}
