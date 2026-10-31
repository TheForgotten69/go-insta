package goinsta

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"maps"
	"path"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/TheForgotten69/go-insta/internal/httplog"
)

func mp4Box(typ string, payload ...[]byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(bytes.Join(payload, nil))))
	b = append(b, typ...)
	return append(b, bytes.Join(payload, nil)...)
}

// testMP4 is a minimal MP4: ftyp, moov (mvhd v0, one video trak) and mdat.
func testMP4(w, h int, durationMs uint32, rotated bool) []byte {
	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:], 1000)
	binary.BigEndian.PutUint32(mvhd[16:], durationMs)
	tkhd := make([]byte, 84)
	a, b := uint32(0x10000), uint32(0)
	if rotated {
		a, b = 0, 0x10000
	}
	binary.BigEndian.PutUint32(tkhd[40:], a)
	binary.BigEndian.PutUint32(tkhd[44:], b)
	binary.BigEndian.PutUint32(tkhd[76:], uint32(w)<<16)
	binary.BigEndian.PutUint32(tkhd[80:], uint32(h)<<16)
	hdlr := make([]byte, 24)
	copy(hdlr[8:], "vide")
	return bytes.Join([][]byte{
		mp4Box("ftyp", []byte("isom\x00\x00\x02\x00")),
		mp4Box("moov", mp4Box("mvhd", mvhd), mp4Box("trak", mp4Box("tkhd", tkhd), mp4Box("mdia", mp4Box("hdlr", hdlr)))),
		mp4Box("mdat", make([]byte, 64)),
	}, nil)
}

func TestParseMP4(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want mp4Info
		err  error
	}{
		{"portrait", testMP4(1080, 1920, 6000, false), mp4Info{1080, 1920, 6000}, nil},
		{"rotated landscape", testMP4(1920, 1080, 6013, true), mp4Info{1080, 1920, 6013}, nil},
		{"not mp4", []byte("not a video at all"), mp4Info{}, ErrInvalidVideo},
		{"no video track", bytes.Join([][]byte{mp4Box("ftyp", []byte("isom")), mp4Box("moov")}, nil), mp4Info{}, ErrInvalidVideo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMP4(tc.data)
			if !errors.Is(err, tc.err) || (err == nil && got != tc.want) {
				t.Errorf("parseMP4 = %+v, %v; want %+v, %v", got, err, tc.want, tc.err)
			}
		})
	}
}

// The video requests are the app's (2026-10-31 capture); its POST went out empty
// (the emulator could not encode), so the responses come from the live run that day.
func TestUploadReel(t *testing.T) {
	insta, r := newReplay(t, "rupload_video_offset", "rupload_video")
	r.fixtures[1].Status, r.fixtures[1].RespBody = 200, `{"media_id":1620473675799970,"status":"ok"}`
	configure := httplog.Entry{Method: "POST", Path: "/api/v1/media/configure_to_clips/"}
	notYet, done := configure, configure
	notYet.Status, notYet.RespBody = 202, `{"status":"fail","message":"Transcode not finished yet."}`
	done.Status, done.RespBody = 200, `{"media":{"id":"4019685539559466739_42865264907","pk":4019685539559466739,`+
		`"code":"DfIyrN2N2bz","media_type":2,"product_type":"clips"},"status":"ok"}`
	r.fixtures = append(r.fixtures, notYet, done)
	r.used = make([]bool, len(r.fixtures))
	defer func(w time.Duration) { transcodeWait = w }(transcodeWait)
	transcodeWait = time.Millisecond

	mp4 := testMP4(1080, 1920, 6013, false)
	it, err := insta.UploadReel(t.Context(), bytes.NewReader(mp4), ReelOptions{PostOptions: PostOptions{Caption: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.sent) != 4 {
		t.Fatalf("sent %d requests, want offset, upload, 2 configures", len(r.sent))
	}

	var got, want map[string]string
	_ = json.Unmarshal([]byte(r.sent[0].req.Header.Get("X-Instagram-Rupload-Params")), &got)
	_ = json.Unmarshal([]byte(fixture(t, "rupload_video").ReqHeaders["x-instagram-rupload-params"]), &want)
	// Not sent: C2PA/IPTC provenance, the segment id and the app's p_profile pair.
	for _, k := range []string{"provenance_metadata", "debug_segment_id", "p_profile", "p_profile_ord"} {
		delete(want, k)
	}
	if g, w := slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)); !slices.Equal(g, w) {
		t.Errorf("rupload params keys:\n got %v\nwant %v", g, w)
	}
	for k, v := range map[string]string{
		"share_type": "reels", "is_clips_video": "1", "media_type": "2", "extract_cover_frame": "1",
		"upload_media_width": "1080", "upload_media_height": "1920", "upload_media_duration_ms": "6013",
	} {
		if got[k] != v {
			t.Errorf("rupload %s = %q, want %q", k, got[k], v)
		}
	}

	if r.sent[0].req.Method != "GET" || r.sent[1].req.Method != "POST" {
		t.Errorf("methods = %s, %s; want GET then POST", r.sent[0].req.Method, r.sent[1].req.Method)
	}
	up := r.sent[1].req
	for k, v := range map[string]string{
		"X-Entity-Length": strconv.Itoa(len(mp4)), "X-Entity-Type": "video/mp4", "Offset": "0",
		"X-Entity-Name": path.Base(up.URL.Path), "Content-Type": "application/octet-stream",
	} {
		if up.Header.Get(k) != v {
			t.Errorf("upload %s = %q, want %q", k, up.Header.Get(k), v)
		}
	}

	p := r.sent[3].payload
	if p["caption"] != "hi" || p["length"] != 6.013 || p["upload_id"] != got["upload_id"] || p["audio_muted"] != false {
		t.Errorf("configure payload: caption %v length %v upload_id %v audio_muted %v", p["caption"], p["length"], p["upload_id"], p["audio_muted"])
	}
	if _, async := p["async_publish"]; async {
		t.Error("configure_to_clips must publish synchronously")
	}
	if q := r.sent[3].req.URL.RawQuery; q != "video=1" {
		t.Errorf("configure query = %q", q)
	}
	if it.Code != "DfIyrN2N2bz" || it.MediaType != 2 {
		t.Errorf("reel = %s type %d", it.Code, it.MediaType)
	}
}

// reelBody is the JSON object UploadReel sends for o, on a base whose audience is "default".
func reelBody(t *testing.T, o ReelOptions) map[string]any {
	t.Helper()
	req := clipsConfigure{configureBase: configureBase{Audience: "default"}}
	if err := o.apply(&req); err != nil {
		t.Fatal(err)
	}
	payload, err := withExtra(req, o.Extra)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestReelOptions(t *testing.T) {
	at := time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)
	o := ReelOptions{
		PostOptions:     PostOptions{ShareToStory: true},
		HideFromFeed:    true,
		DisableComments: true, AllowDownloads: true, DisableTemplate: true,
		Captions: true, TranslateCaption: true, Gifting: true,
		Audience: AudienceCloseFriends, CoverFrameMs: 1500, Trial: TrialPerformance,
		VoiceTranslation: &VoiceTranslation{InputLanguage: "fra", Languages: []string{"eng", "spa"}, LipSync: true},
		ScheduledAt:      at,
		Extra:            map[string]any{"disable_comments": "0", "music_start_time_ms": "0"},
	}
	p := reelBody(t, o)
	for k, want := range map[string]any{
		"is_auto_resharing_to_story": "1", "disable_comments": "0", "third_party_downloads_enabled": "1",
		"is_template_disabled": "1", "video_subtitles_enabled": "1", "video_subtitles_translations_enabled": "1",
		"is_gifting_enabled": "1", "audience": "besties", "cover_frame_time_ms": "1500",
		"publish_mode": "scheduled", "music_start_time_ms": "0",
		"content_scheduling_metadata": `{"scheduled_publish_time":1793642400}`,
		"clips_voice_translation_metadata": `{"translation_input_language":"fra","translation_lipsync_enabled":"1",` +
			`"translation_approval_flow_enabled":"0","translation_languages":["eng","spa"]}`,
	} {
		if p[k] != want {
			t.Errorf("%s = %v, want %v", k, p[k], want)
		}
	}
	if _, shared := p["clips_share_preview_to_feed"]; shared {
		t.Error("HideFromFeed still sent clips_share_preview_to_feed")
	}
	if tp, _ := p["trial_params"].(map[string]any); tp["graduation_strategy"] != string(TrialPerformance) {
		t.Errorf("trial_params = %v", p["trial_params"])
	}

	zero, base := reelBody(t, ReelOptions{}), reelBody(t, ReelOptions{HideFromFeed: true})
	delete(zero, "clips_share_preview_to_feed")
	if !reflect.DeepEqual(zero, base) || reelBody(t, ReelOptions{})["clips_share_preview_to_feed"] != "1" {
		t.Errorf("zero options sent %v beyond clips_share_preview_to_feed, want nothing", zero)
	}
	insta, _ := newReplay(t)
	if _, err := insta.UploadReel(t.Context(), bytes.NewReader(testMP4(1080, 1920, 6000, false)),
		ReelOptions{VoiceTranslation: &VoiceTranslation{Languages: []string{"eng"}}}); err == nil {
		t.Error("voice translation without InputLanguage accepted")
	}
}
