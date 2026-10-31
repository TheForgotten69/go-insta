package goinsta

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// ReelOptions are the optional fields of a Reel. The zero value posts with no
// caption and leaves every toggle at the account's default. Each field maps to a
// configure_to_clips parameter as the app sends it; docs/reel-fields.md lists them all.
type ReelOptions struct {
	// Caption, ShareToThreads and the hidden counts work as for photos.
	// ShareToStory asks Instagram to reshare the Reel to the story itself.
	PostOptions
	// HideFromFeed keeps the Reel out of the feed (no clips_share_preview_to_feed).
	HideFromFeed     bool
	DisableComments  bool
	AllowDownloads   bool // third_party_downloads_enabled
	DisableTemplate  bool // is_template_disabled: others cannot use the Reel as a template
	Captions         bool // auto-generated captions (video_subtitles_enabled)
	TranslateCaption bool // translated captions (video_subtitles_translations_enabled)
	Gifting          bool // is_gifting_enabled
	Audience         Audience
	// CoverFrameMs picks the cover frame at that offset into the video.
	CoverFrameMs int
	// Trial posts a trial Reel, shown to non-followers first. A personal
	// account gets HTTP 500; Instagram offers trials to professional accounts.
	Trial TrialStrategy
	// VoiceTranslation asks for AI voice translation of the audio. The app also
	// has an upload-time form (autodub_creation_params) that needs its segmented
	// upload; this sends the configure_to_clips form, which Instagram accepts.
	VoiceTranslation *VoiceTranslation
	// ScheduledAt publishes the Reel later instead of now.
	ScheduledAt time.Time
	// Extra sets any other configure_to_clips field, last, so it also overrides the above.
	Extra map[string]any
}

// Audience is who sees the Reel.
type Audience string

const (
	AudienceDefault         Audience = "default"
	AudienceCloseFriends    Audience = "besties"
	AudienceFollowersOnly   Audience = "followers_only"
	AudienceMutualFollowers Audience = "mutual_followers"
	AudienceOnlyMe          Audience = "only_me"
)

// TrialStrategy is when a trial Reel graduates to followers.
type TrialStrategy string

const (
	TrialManual      TrialStrategy = "manual"         // the creator decides
	TrialPerformance TrialStrategy = "ss_performance" // automatically, if it performs well
	TrialTimeLimit   TrialStrategy = "time_limit"     // automatically, after a delay
)

// VoiceTranslation languages are ISO 639-3 codes ("eng", "spa", "fra"); the app
// offers eng and spa by default.
type VoiceTranslation struct {
	InputLanguage string
	Languages     []string
	LipSync       bool
	// Approval holds each translation for the creator to review before it goes live.
	Approval bool
}

// UploadReel posts an MP4 (H.264 video, AAC audio) as a Reel; Instagram picks
// the cover frame unless CoverFrameMs is set.
func (insta *Instagram) UploadReel(ctx context.Context, r io.Reader, opts ReelOptions) (*Item, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	info, err := parseMP4(b)
	if err != nil {
		return nil, err
	}
	if t := opts.VoiceTranslation; t != nil && (t.InputLanguage == "" || len(t.Languages) == 0) {
		return nil, errors.New("voice translation needs InputLanguage and Languages (ISO 639-3)")
	}
	v := &video{uploadID: newUploadID(), data: b, info: info}
	post := opts.PostOptions
	post.ShareToStory = false // the server reshares Reels; no card to render
	x, err := insta.preparePost(ctx, post, nil)
	if err != nil {
		return nil, err
	}
	if err := insta.uploadVideo(ctx, v); err != nil {
		return nil, err
	}
	secs := float64(info.durationMs) / 1000
	req := clipsConfigure{
		configureBase: insta.configureBase(v.uploadID),
		postFields:    x.fields(),
		Length:        secs,
		Clips:         []clipSegment{{Length: secs, SourceType: "4"}},
		Extra:         sourceSize{Width: info.w, Height: info.h},
		FilterType:    "0",
	}
	// Published synchronously: async_publish answers PENDING with an empty media.
	req.AsyncPublish, req.PublishID = "", ""
	if err := opts.apply(&req); err != nil {
		return nil, err
	}
	payload, err := withExtra(req, opts.Extra)
	if err != nil {
		return nil, err
	}
	return insta.configureReel(ctx, v.uploadID, payload)
}

// clipsConfigure is a configure_to_clips body.
type clipsConfigure struct {
	configureBase
	postFields
	reelFields
	SceneCaptureType string        `json:"scene_capture_type"`
	Length           float64       `json:"length"`
	Clips            []clipSegment `json:"clips"`
	Extra            sourceSize    `json:"extra"`
	AudioMuted       bool          `json:"audio_muted"`
	PosterFrameIndex int           `json:"poster_frame_index"`
	FilterType       string        `json:"filter_type"`
}

// reelFields are the ReelOptions part of a configure_to_clips body.
// is_auto_resharing_to_story comes from postFields.
type reelFields struct {
	ClipsSharePreviewToFeed           string       `json:"clips_share_preview_to_feed,omitempty"`
	DisableComments                   string       `json:"disable_comments,omitempty"`
	ThirdPartyDownloadsEnabled        string       `json:"third_party_downloads_enabled,omitempty"`
	IsTemplateDisabled                string       `json:"is_template_disabled,omitempty"`
	VideoSubtitlesEnabled             string       `json:"video_subtitles_enabled,omitempty"`
	VideoSubtitlesTranslationsEnabled string       `json:"video_subtitles_translations_enabled,omitempty"`
	IsGiftingEnabled                  string       `json:"is_gifting_enabled,omitempty"`
	CoverFrameTimeMs                  string       `json:"cover_frame_time_ms,omitempty"`
	TrialParams                       *trialParams `json:"trial_params,omitempty"`
	ClipsVoiceTranslationMetadata     string       `json:"clips_voice_translation_metadata,omitempty"`
	PublishMode                       string       `json:"publish_mode,omitempty"`
	ContentSchedulingMetadata         string       `json:"content_scheduling_metadata,omitempty"`
}

// withExtra overlays extra on the JSON object of req. ReelOptions.Extra is the
// one place a request leaves the typed structs, so it can set any field.
func withExtra(req any, extra map[string]any) (any, error) {
	if len(extra) == 0 {
		return req, nil
	}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}
	for k, v := range extra {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("extra %s: %w", k, err)
		}
		obj[k] = raw
	}
	return obj, nil
}

func (o *ReelOptions) apply(req *clipsConfigure) error {
	flag := func(on bool) string {
		if on {
			return "1"
		}
		return ""
	}
	f := &req.reelFields
	f.ClipsSharePreviewToFeed = flag(!o.HideFromFeed)
	f.DisableComments = flag(o.DisableComments)
	f.ThirdPartyDownloadsEnabled = flag(o.AllowDownloads)
	f.IsTemplateDisabled = flag(o.DisableTemplate)
	f.VideoSubtitlesEnabled = flag(o.Captions)
	f.VideoSubtitlesTranslationsEnabled = flag(o.TranslateCaption)
	f.IsGiftingEnabled = flag(o.Gifting)
	if o.ShareToStory {
		req.IsAutoResharingToStory = "1"
	}
	if o.Audience != "" {
		req.Audience = string(o.Audience)
	}
	if o.CoverFrameMs > 0 {
		f.CoverFrameTimeMs = strconv.Itoa(o.CoverFrameMs)
	}
	if o.Trial != "" {
		f.TrialParams = &trialParams{GraduationStrategy: o.Trial}
	}
	if t := o.VoiceTranslation; t != nil {
		meta, err := json.Marshal(voiceTranslationMeta{
			InputLanguage: t.InputLanguage, LipSync: boolFlag(t.LipSync),
			Approval: boolFlag(t.Approval), Languages: t.Languages,
		})
		if err != nil {
			return err
		}
		f.ClipsVoiceTranslationMetadata = string(meta)
	}
	if !o.ScheduledAt.IsZero() {
		meta, err := json.Marshal(schedulingMeta{ScheduledPublishTime: o.ScheduledAt.Unix()})
		if err != nil {
			return err
		}
		f.PublishMode = "scheduled"
		f.ContentSchedulingMetadata = string(meta)
	}
	return nil
}

type clipSegment struct {
	Length     float64 `json:"length"`
	SourceType string  `json:"source_type"`
}

type sourceSize struct {
	Width  int `json:"source_width"`
	Height int `json:"source_height"`
}

type trialParams struct {
	GraduationStrategy TrialStrategy `json:"graduation_strategy"`
}

type schedulingMeta struct {
	ScheduledPublishTime int64 `json:"scheduled_publish_time"`
}

// voiceTranslationMeta is configure_to_clips' clips_voice_translation_metadata.
type voiceTranslationMeta struct {
	InputLanguage string   `json:"translation_input_language"`
	LipSync       string   `json:"translation_lipsync_enabled"`
	Approval      string   `json:"translation_approval_flow_enabled"`
	Languages     []string `json:"translation_languages"`
}

func boolFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// Instagram transcodes the upload before configure_to_clips accepts it; until
// then it answers 202 "Transcode not finished yet." (6 s video: one retry).
var transcodeWait = 3 * time.Second

const transcodeTries = 100 // 5 minutes; a 45 s Reel finished within 2

func (insta *Instagram) configureReel(ctx context.Context, uploadID string, payload any) (*Item, error) {
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
	return nil, fmt.Errorf("reel %s: transcode not finished after %v", uploadID, transcodeWait*transcodeTries)
}

// videoRuploadParams is the X-Instagram-Rupload-Params of a Reel upload; every
// value is a string, as the app sends it.
type videoRuploadParams struct {
	UploadID               string `json:"upload_id"`
	SessionID              string `json:"session_id"`
	MediaType              string `json:"media_type"`
	ShareType              string `json:"share_type"`
	IsClipsVideo           string `json:"is_clips_video"`
	Width                  string `json:"upload_media_width"`
	Height                 string `json:"upload_media_height"`
	DurationMs             string `json:"upload_media_duration_ms"`
	ExtractCoverFrame      string `json:"extract_cover_frame"`
	ContentTags            string `json:"content_tags"`
	UploadEngineConfigEnum string `json:"upload_engine_config_enum"`
	IsOptimisticUpload     string `json:"is_optimistic_upload"`
	XsharingUserIDs        string `json:"xsharing_user_ids"`
	StellaData             string `json:"stella_data"`
	RetryContext           string `json:"retry_context"`
}

// uploadVideo asks for the resume offset, then sends the rest of the file in one request.
func (insta *Instagram) uploadVideo(ctx context.Context, v *video) error {
	params, err := json.Marshal(videoRuploadParams{
		UploadID:               v.uploadID,
		SessionID:              v.uploadID,
		MediaType:              "2",
		ShareType:              "reels",
		IsClipsVideo:           "1",
		Width:                  strconv.Itoa(v.info.w),
		Height:                 strconv.Itoa(v.info.h),
		DurationMs:             strconv.FormatInt(v.info.durationMs, 10),
		ExtractCoverFrame:      "1",
		ContentTags:            "use_default_cover",
		UploadEngineConfigEnum: "0",
		IsOptimisticUpload:     "false",
		XsharingUserIDs:        "[]",
		StellaData:             "{}",
		RetryContext:           `{"num_reupload":0,"cover_photo_re_upload_count":0,"num_step_manual_retry":0,"num_step_auto_retry":0}`,
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
