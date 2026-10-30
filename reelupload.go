package goinsta

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type video struct {
	uploadID string
	data     []byte
	info     mp4Info
}

// UploadReel posts an MP4 (H.264 video, AAC audio) as a Reel; Instagram picks
// the cover frame. Caption, ShareToThreads and the hidden counts work as for
// photos; ShareToStory returns ErrReelStoryShare.
func (insta *Instagram) UploadReel(ctx context.Context, r io.Reader, opts PostOptions) (*Item, error) {
	if opts.ShareToStory {
		return nil, ErrReelStoryShare
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	info, err := parseMP4(b)
	if err != nil {
		return nil, err
	}
	x, err := insta.preparePost(ctx, opts, nil)
	if err != nil {
		return nil, err
	}
	v := &video{uploadID: newUploadID(), data: b, info: info}
	if err := insta.uploadVideo(ctx, v); err != nil {
		return nil, err
	}
	secs := float64(info.durationMs) / 1000
	payload := insta.configureBase(v.uploadID)
	// Published synchronously: async_publish answers PENDING with an empty media.
	delete(payload, "async_publish")
	delete(payload, "publish_id")
	payload["length"] = secs
	payload["clips"] = []map[string]any{{"length": secs, "source_type": "4"}}
	payload["extra"] = map[string]any{"source_width": info.w, "source_height": info.h}
	payload["audio_muted"] = false
	payload["poster_frame_index"] = 0
	payload["clips_share_preview_to_feed"] = "1"
	payload["filter_type"] = "0"
	x.apply(payload)
	return insta.configureReel(ctx, payload)
}

// Instagram transcodes the upload before configure_to_clips accepts it; until
// then it answers 202 "Transcode not finished yet." (6 s video: one retry).
var transcodeWait = 3 * time.Second

const transcodeTries = 40

func (insta *Instagram) configureReel(ctx context.Context, payload map[string]any) (*Item, error) {
	for range transcodeTries {
		item, err := insta.configure(ctx, urlConfigureClips, payload)
		if err == nil || !strings.Contains(err.Error(), "Transcode not finished") {
			return item, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(transcodeWait):
		}
	}
	return nil, fmt.Errorf("reel %v: transcode not finished after %v", payload["upload_id"], transcodeWait*transcodeTries)
}

// uploadVideo asks for the resume offset, then sends the rest of the file in one request.
func (insta *Instagram) uploadVideo(ctx context.Context, v *video) error {
	params, err := json.Marshal(map[string]string{
		"upload_id":                 v.uploadID,
		"session_id":                v.uploadID,
		"media_type":                "2",
		"share_type":                "reels",
		"is_clips_video":            "1",
		"upload_media_width":        strconv.Itoa(v.info.w),
		"upload_media_height":       strconv.Itoa(v.info.h),
		"upload_media_duration_ms":  strconv.FormatInt(v.info.durationMs, 10),
		"extract_cover_frame":       "1",
		"content_tags":              "use_default_cover",
		"upload_engine_config_enum": "0",
		"is_optimistic_upload":      "false",
		"xsharing_user_ids":         "[]",
		"stella_data":               "{}",
		"retry_context":             `{"num_reupload":0,"cover_photo_re_upload_count":0,"num_step_manual_retry":0,"num_step_auto_retry":0}`,
	})
	if err != nil {
		return err
	}
	name := randomHex(16) + "-0-0-" + v.uploadID + "-" + v.uploadID
	headers := func(extra map[string]string) map[string]string {
		h := map[string]string{
			"X-Instagram-Rupload-Params": string(params),
			"X_fb_video_waterfall_id":    generateUUID() + "_" + strings.ToUpper(randomHex(6)) + "_Video_0",
			"X-Fb-Request-Analytics-Tags": `{"network_tags":{"product":"` + insta.sess.AppID +
				`","surface":"undefined","request_category":"media_upload","purpose":"none","retry_attempt":"0"}}`,
		}
		for k, val := range extra {
			h[k] = val
		}
		return h
	}
	opts := func(post bool, body []byte, extra map[string]string) *reqOptions {
		return &reqOptions{
			Endpoint:      fmt.Sprintf(urlUploadVideo, name),
			OmitAPI:       true,
			IsPost:        post,
			DataBytes:     body,
			Priority:      "u=1, i",
			FriendlyName:  "undefined:media-upload",
			IgnoreHeaders: ruploadIgnored,
			ExtraHeaders:  headers(extra),
		}
	}
	var start struct {
		Offset int `json:"offset"`
	}
	if err := insta.callJSON(ctx, opts(false, nil, nil), &start); err != nil {
		return fmt.Errorf("video upload offset: %w", err)
	}
	if start.Offset < 0 || start.Offset > len(v.data) {
		start.Offset = 0
	}
	var resp struct {
		Status string `json:"status"`
	}
	err = insta.callJSON(ctx, opts(true, v.data[start.Offset:], map[string]string{
		"Content-Type":    "application/octet-stream",
		"Offset":          strconv.Itoa(start.Offset),
		"X-Entity-Length": strconv.Itoa(len(v.data)),
		"X-Entity-Name":   name,
		"X-Entity-Type":   "video/mp4",
	}), &resp)
	if err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("video upload %s: status %q", v.uploadID, resp.Status)
	}
	return nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
