package goinsta

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/TheForgotten69/go-insta/internal/httplog"
)

var (
	fixturesOnce sync.Once
	fixtures     map[string]httplog.Entry
)

func fixture(t *testing.T, name string) httplog.Entry {
	t.Helper()
	fixturesOnce.Do(func() {
		b, err := os.ReadFile("testdata/fixtures.json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(b, &fixtures); err != nil {
			panic(err)
		}
	})
	e, ok := fixtures[name]
	if !ok {
		t.Fatalf("no fixture %q", name)
	}
	return e
}

// The first capture cut bodies at 64 KiB; such responses cannot be decoded.
func truncated(e httplog.Entry) bool {
	body := strings.TrimSpace(e.RespBody)
	last := body[strings.LastIndexByte(body, '\n')+1:]
	return body != "" && !json.Valid([]byte(last))
}

type sent struct {
	req     *http.Request
	payload map[string]any
}

type replay struct {
	mu       sync.Mutex
	t        *testing.T
	fixtures []httplog.Entry
	used     []bool
	sent     []sent
}

func (r *replay) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var raw []byte
	if req.Body != nil {
		raw, _ = io.ReadAll(req.Body)
		if req.Header.Get("Content-Encoding") == "gzip" {
			zr, err := gzip.NewReader(bytes.NewReader(raw))
			if err != nil {
				return nil, err
			}
			raw, _ = io.ReadAll(zr)
		}
	}
	r.sent = append(r.sent, sent{req: req, payload: payloadOf(req, raw)})

	idx := -1
	for i, f := range r.fixtures {
		if samePath(f.Path, req.URL.Path) && (!r.used[i] || idx == -1) {
			idx = i
			if !r.used[i] {
				break
			}
		}
	}
	if idx < 0 {
		r.t.Fatalf("unexpected request %s %s", req.Method, req.URL)
	}
	r.used[idx] = true
	f := r.fixtures[idx]

	body := []byte(f.RespBody)
	if truncated(f) {
		body = []byte(`{"status":"ok"}`)
	}
	enc, _ := zstd.NewWriter(nil)
	h := http.Header{}
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Content-Encoding", "zstd")
	h.Set("Ig-Set-Ig-U-Rur", "ROTATED")
	return &http.Response{
		StatusCode: f.Status,
		Header:     h,
		Body:       io.NopCloser(bytes.NewReader(enc.EncodeAll(body, nil))),
		Request:    req,
	}, nil
}

func samePath(fixture, got string) bool {
	if strings.HasPrefix(fixture, "/rupload_igphoto/") {
		return strings.HasPrefix(got, "/rupload_igphoto/")
	}
	return fixture == got
}

func payloadOf(req *http.Request, raw []byte) map[string]any {
	vals := req.URL.Query()
	if strings.HasPrefix(req.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		form, _ := url.ParseQuery(string(raw))
		for k, v := range form {
			vals[k] = v
		}
	}
	out := map[string]any{}
	for k, v := range vals {
		out[k] = strings.Join(v, ",")
	}
	if sb, ok := out["signed_body"].(string); ok {
		delete(out, "signed_body")
		_, js, _ := strings.Cut(sb, ".")
		var m map[string]any
		if err := json.Unmarshal([]byte(js), &m); err == nil {
			for k, v := range m {
				out[k] = v
			}
		}
	}
	return out
}

func newReplay(t *testing.T, names ...string) (*Instagram, *replay) {
	t.Helper()
	insta, err := New(Session{
		UserID:         42865264907,
		Authorization:  "Bearer IGT:2:test",
		AndroidID:      "android-0000000000000000",
		DeviceID:       "00000000-0000-0000-0000-00000000dev0",
		FamilyDeviceID: "00000000-0000-0000-0000-00000000fam0",
		Mid:            "mid",
		WWWClaim:       "claim",
		Rur:            "rur",
		ZCA:            "zca",
		USDID:          "usdid",
		TimezoneOffset: "3600",
		GoogleAdID:     "ad",
	})
	if err != nil {
		t.Fatal(err)
	}
	r := &replay{t: t}
	for _, n := range names {
		r.fixtures = append(r.fixtures, fixture(t, n))
	}
	r.used = make([]bool, len(r.fixtures))
	insta.SetHTTPClient(&http.Client{Transport: r})
	return insta, r
}

// Headers whose value legitimately differs per request or per UI context.
var volatileHeaders = map[string]bool{
	"ig-u-rur": true, "x-pigeon-rawclienttime": true, "x-fb-conn-uuid-client": true,
	"x-ig-bandwidth-speed-kbps": true, "x-ig-bandwidth-totalbytes-b": true, "x-ig-bandwidth-totaltime-ms": true,
	"x-ig-nav-chain": true, "x-ig-client-endpoint": true, "x-ig-salt-ids": true, "x-pigeon-session-id": true,
	"x-entity-name": true, "x-entity-length": true, "x-entity-type": true, "x_fb_photo_waterfall_id": true,
	"x-instagram-rupload-params": true, "x-cm-bandwidth-kbps": true, "x-cm-latency": true,
	"x-ig-app-start-time": true, "x-ig-app-start-request": true, "retry_context": true,
	"x-fb-rmd": true,
}

// The fixtures come from the older build (2026-10-27). The current build drops the
// zero-rating headers and adds a network-stack set; TestCurrentBuildProfile checks
// those against the 2026-10-28 captures instead.
var olderBuildOnly = map[string]bool{
	"x-zero-a-device-id": true, "x-zero-balance": true, "x-zero-d-device-id": true,
	"x-zero-eh": true, "x-zero-f-device-id": true, "zero-http-network-interface": true,
}
var currentBuildOnly = map[string]bool{
	"x-fb-appnetsession-nid": true, "x-fb-appnetsession-sid": true, "x-fb-network-properties": true,
	"x-fb-session-id": true, "x-fb-session-private": true, "x-fb-tasos-td-v2-config": true,
	"x-meta-tasos-congestion-config": true, "x-meta-tasos-dcc-constraint": true,
}

// Headers the app sends that are deliberately not replicated.
var skippedHeaders = map[string]bool{
	"x-ig-app-start-time": true, "x-ig-app-start-request": true, "x-cm-bandwidth-kbps": true,
	"x-cm-latency": true, "retry_context": true, "x-ig-nav-chain": true, "x-ig-salt-ids": true,
	"content-length": true, // set by the transport, not visible on the request headers
}

// UI-context payload fields the library cannot know (nav chains, ranking tokens, impressions).
var contextKeys = map[string]bool{
	"nav_chain": true, "ranking_info_token": true, "starting_clips_ranking_info_token": true,
	"follow_ranking_token": true, "chaining_session_id": true, "chaining_seed_media_id": true,
	"chaining_position": true, "media_pct_watched": true, "inventory_source": true,
	"media_id_attribution": true, "client_position": true, "recs_ix": true, "tracking_token": true,
	"num_visible_media_notes": true, "fully_visible_media_note_ids": true, "user_breadcrumb": true,
	"session_info": true, "client_view_state_media_list": true, "client_flashcache_size": true,
	"seen_reels": true, "music_params": true, "camera_entry_point": true, "bottom_camera_dial_selected": true,
	"creation_logger_session_id": true, "imported_taken_at": true, "media_folder": true,
	"starting_clips_media_id": true, "page_type": true, "media_id": true, "app_start_time": true,
	"client_seen_store_media_list": true, "feed_reshare_info": true, "organic_realtime_information": true,
	"request_build_time": true, "session_level_signals": true, "triggered_by_visible_spinner": true,
	"is_from_swipe": true, "container_module": true, "logging_info_token": true,
}

var uaResolution = regexp.MustCompile(`; \d+x\d+;`)

func checkRequest(t *testing.T, want httplog.Entry, got sent, extraOK ...string) {
	t.Helper()
	if got.req.Method != want.Method {
		t.Errorf("method = %s, want %s", got.req.Method, want.Method)
	}
	if !samePath(want.Path, got.req.URL.Path) {
		t.Errorf("path = %s, want %s", got.req.URL.Path, want.Path)
	}

	var missingH, diffH []string
	for k, v := range want.ReqHeaders {
		if skippedHeaders[k] || olderBuildOnly[k] {
			continue
		}
		g := got.req.Header.Get(k)
		if g == "" && v != "" {
			missingH = append(missingH, k)
			continue
		}
		if k == "user-agent" {
			// The Waydroid window was resized mid-capture; GraphQL picked up the new size.
			g, v = uaResolution.ReplaceAllString(g, ""), uaResolution.ReplaceAllString(v, "")
		}
		if v != "REDACTED" && !volatileHeaders[k] && g != v {
			diffH = append(diffH, k+": got "+g+" want "+v)
		}
	}
	var extraH []string
	for k := range got.req.Header {
		lk := strings.ToLower(k)
		if _, ok := want.ReqHeaders[lk]; !ok && !volatileHeaders[lk] && !currentBuildOnly[lk] {
			extraH = append(extraH, lk)
		}
	}
	sort.Strings(extraH)
	if len(extraH) > 0 {
		t.Errorf("headers the app did not send: %v", extraH)
	}
	sort.Strings(missingH)
	sort.Strings(diffH)
	if len(missingH) > 0 {
		t.Errorf("missing headers: %v", missingH)
	}
	if len(diffH) > 0 {
		t.Errorf("header values differ:\n  %s", strings.Join(diffH, "\n  "))
	}

	wantKeys := map[string]bool{}
	for k := range want.Query {
		wantKeys[k] = true
	}
	for k := range want.ReqForm {
		wantKeys[k] = true
	}
	for k := range want.ReqSigned {
		wantKeys[k] = true
	}
	ok := map[string]bool{}
	for _, k := range extraOK {
		ok[k] = true
	}
	var missing, extra []string
	for k := range wantKeys {
		if _, has := got.payload[k]; !has && !contextKeys[k] && !ok[k] {
			missing = append(missing, k)
		}
	}
	for k := range got.payload {
		if !wantKeys[k] && !ok[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("missing payload keys: %v", missing)
	}
	if len(extra) > 0 {
		t.Errorf("unexpected payload keys: %v", extra)
	}
}

func testItem(insta *Instagram, id string) *Item {
	it := &Item{ID: id}
	setToItem(it, insta, "feed_timeline")
	return it
}

func TestTimeline(t *testing.T) {
	insta, r := newReplay(t, "timeline")
	_ = insta.Timeline().Next(t.Context())
	checkRequest(t, fixture(t, "timeline"), r.sent[0])
	if r.sent[0].req.Header.Get("Content-Encoding") != "gzip" {
		t.Error("timeline body must be gzipped")
	}
}

func TestTimelineTail(t *testing.T) {
	insta, r := newReplay(t, "timeline_tail")
	f := fixture(t, "timeline_tail")
	tl := insta.Timeline()
	tl.started, tl.nextID = true, f.ReqForm["max_id"]
	_ = tl.Next(t.Context())
	checkRequest(t, f, r.sent[0], "client_view_state_media_list", "feed_view_info")
}

func TestReelsTray(t *testing.T) {
	insta, r := newReplay(t, "reels_tray")
	if _, err := insta.StoryTray(t.Context(), TrayPullToRefresh); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "reels_tray"), r.sent[0])
}

func TestLogout(t *testing.T) {
	insta, r := newReplay(t, "logout")
	res, err := insta.Logout(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// The logout capture used a spoofed device profile with a different header
	// set, so only the method, path and payload keys are comparable.
	f := fixture(t, "logout")
	got := r.sent[0]
	if got.req.Method != f.Method || got.req.URL.Path != f.Path {
		t.Errorf("logout = %s %s, want %s %s", got.req.Method, got.req.URL.Path, f.Method, f.Path)
	}
	for k := range f.ReqForm {
		if _, ok := got.payload[k]; !ok {
			t.Errorf("logout missing payload key %q", k)
		}
	}
	if res.Status != "ok" || res.LoginNonce == "" {
		t.Errorf("logout result not parsed: %+v", res)
	}
}

func TestAccountSync(t *testing.T) {
	insta, r := newReplay(t, "user_info")
	if err := insta.Account.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "user_info"), r.sent[0])
	if insta.Account.Username == "" || insta.Account.ID != 42865264907 {
		t.Errorf("account not parsed: %+v", insta.Account.Username)
	}
}

func TestUserInfoStream(t *testing.T) {
	insta, r := newReplay(t, "user_info_stream")
	u, err := insta.UserByID(t.Context(), 62759345752)
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "user_info_stream"), r.sent[0])
	if u.Username == "" || u.insta != insta || u.FbIdV2 == 0 {
		t.Errorf("user not parsed: %q fbid_v2 %d", u.Username, u.FbIdV2)
	}
	if got := insta.Session().Rur; got != "ROTATED" {
		t.Errorf("rur not rotated from ig-set header: %q", got)
	}
}

func TestFriendship(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   int64
		call func(*User, context.Context) error
	}{
		{"friendship_show", 62759345752, func(u *User, ctx context.Context) error { _, err := u.LoadFriendship(ctx); return err }},
		{"follow", 62759345752, (*User).Follow},
		{"unfollow", 27626942043, (*User).Unfollow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			insta, r := newReplay(t, tc.name)
			u := insta.NewUser(tc.id)
			if err := tc.call(u, t.Context()); err != nil {
				t.Fatal(err)
			}
			checkRequest(t, fixture(t, tc.name), r.sent[0])
		})
	}
}

func TestStoriesAndHighlights(t *testing.T) {
	insta, r := newReplay(t, "user_story", "highlights_tray")
	u := insta.NewUser(62759345752)
	if _, err := u.Stories(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "user_story"), r.sent[0])
	hl, err := u.Highlights(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "highlights_tray"), r.sent[1])
	if len(hl) == 0 || !strings.HasPrefix(hl[0].ReelID(), "highlight:") {
		t.Errorf("highlights not parsed: %d", len(hl))
	}
}

func TestReelsMediaStream(t *testing.T) {
	insta, r := newReplay(t, "reels_media_stream")
	_, _ = insta.FetchReels(t.Context(), "highlight:18094052522158448")
	checkRequest(t, fixture(t, "reels_media_stream"), r.sent[0])
}

func TestStorySeenAndLike(t *testing.T) {
	insta, r := newReplay(t, "media_seen", "story_like")
	reel := &Reel{insta: insta, ID: json.RawMessage(`"highlight:18094052522158448"`)}
	it := testItem(insta, "3989118560685351393_62759345752")
	it.TakenAt = 1789760010
	if err := reel.MarkSeen(t.Context(), it); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "media_seen"), r.sent[0])
	if q := r.sent[0].req.URL.RawQuery; q != "reel=1&live_vod=0" {
		t.Errorf("seen query = %q", q)
	}
	if err := it.LikeStory(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "story_like"), r.sent[1])
}

func TestSuggestions(t *testing.T) {
	insta, r := newReplay(t, "chaining", "ayml", "restricted_users")
	f := fixture(t, "chaining")
	u := insta.NewUser(62759345752)
	u.Username = f.Query["target_username"]
	if id := f.Query["target_id"]; id != "62759345752" {
		u.ID = Int64(mustInt(t, id))
	}
	users, err := u.SuggestedUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, f, r.sent[0])
	if len(users) == 0 {
		t.Error("no chaining users")
	}
	if _, err := insta.Account.SuggestedUsers(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "ayml"), r.sent[1])
	if _, err := insta.Account.RestrictedUsers(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "restricted_users"), r.sent[2])
}

func TestReelSettings(t *testing.T) {
	insta, r := newReplay(t, "reel_settings", "set_reel_settings")
	s, err := insta.Account.ReelSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "reel_settings"), r.sent[0])
	if s.ReelAutoArchive == "" {
		t.Error("reel settings not parsed")
	}
	if err := insta.Account.SetStoryAutoArchive(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "set_reel_settings"), r.sent[1])
}

func TestMediaInteractions(t *testing.T) {
	insta, r := newReplay(t, "like", "save", "stream_comments", "create_note")
	it := testItem(insta, "4017333050152331980_8663171404")

	if err := it.Like(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "like"), r.sent[0], "is_from_swipe")

	if err := it.Save(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "save"), r.sent[1])
	if _, ok := r.sent[1].payload["floating_context_items"].([]any); !ok {
		t.Errorf("save floating_context_items = %#v, want a JSON array", r.sent[1].payload["floating_context_items"])
	}

	sc := fixture(t, "stream_comments")
	scItem := testItem(insta, strings.Split(sc.Path, "/")[4])
	cs := scItem.Comments()
	if err := cs.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, sc, r.sent[2])
	if len(cs.Items) != 15 {
		t.Errorf("comments = %d, want all 15 across the 3 NDJSON lines", len(cs.Items))
	}

	if _, err := it.AddNote(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "create_note"), r.sent[3])
}

func TestComment(t *testing.T) {
	f := fixture(t, "comment")
	insta, r := newReplay(t, "comment")
	it := testItem(insta, strings.Split(f.Path, "/")[4])
	text := f.ReqSigned["comment_text"].(string)
	c, err := it.Comment(t.Context(), text)
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, f, r.sent[0])
	if c.Text != text {
		t.Errorf("comment text = %q", c.Text)
	}
	bc := r.sent[0].payload["user_breadcrumb"].(string)
	parts := strings.Split(bc, "\n")
	data, _ := base64.StdEncoding.DecodeString(parts[1])
	if len(parts) != 3 || parts[2] != "" || breadcrumbFor(string(data)) != bc {
		t.Errorf("breadcrumb malformed: %q", bc)
	}
	if !strings.HasPrefix(string(data), "4 ") {
		t.Errorf("breadcrumb must count UTF-16 units of %q: %q", text, data)
	}
}

func TestBreadcrumbMatchesCapture(t *testing.T) {
	want := fixture(t, "comment").ReqSigned["user_breadcrumb"].(string)
	data, err := base64.StdEncoding.DecodeString(strings.Split(want, "\n")[1])
	if err != nil {
		t.Fatal(err)
	}
	if got := breadcrumbFor(string(data)); got != want {
		t.Errorf("breadcrumb = %q, want %q", got, want)
	}
}

func TestCommentThreads(t *testing.T) {
	insta, r := newReplay(t, "stream_comments_page", "inline_child_comments", "comment_like", "comment_unlike", "comment_bulk_delete")
	page := fixture(t, "stream_comments_page")
	it := testItem(insta, strings.Split(page.Path, "/")[4])
	cs := it.Comments()
	cs.minID = page.Query["min_id"]
	if err := cs.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, page, r.sent[0])
	if len(cs.Items) != 14 || cs.Items[0].item != it {
		t.Errorf("page = %d comments, want all 14 across the 3 NDJSON lines", len(cs.Items))
	}

	child := fixture(t, "inline_child_comments")
	parent := &Comment{ID: json.Number(strings.Split(child.Path, "/")[6]), item: it}
	replies, err := parent.Replies(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, child, r.sent[1])
	if len(replies) == 0 || replies[0].Text == "" {
		t.Error("replies not parsed")
	}

	like := fixture(t, "comment_like")
	cm := &Comment{ID: json.Number(strings.Split(like.Path, "/")[4]), item: it}
	if err := cm.Like(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, like, r.sent[2])
	if err := cm.Unlike(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "comment_unlike"), r.sent[3])

	del := fixture(t, "comment_bulk_delete")
	own := testItem(insta, strings.Split(del.Path, "/")[4])
	if err := own.DeleteComments(t.Context(), del.ReqSigned["comment_ids_to_delete"].(string)); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, del, r.sent[4])
	if got := r.sent[4].payload["chaining_seed_media_id"]; got != del.ReqSigned["chaining_seed_media_id"] {
		t.Errorf("chaining_seed_media_id = %v", got)
	}
}

func TestOwnMediaActions(t *testing.T) {
	insta, r := newReplay(t, "media_delete", "media_only_me", "pin_timeline_media")
	del := fixture(t, "media_delete")
	if err := testItem(insta, strings.Split(del.Path, "/")[4]).Delete(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, del, r.sent[0])
	if q := r.sent[0].req.URL.RawQuery; q != "media_type=PHOTO" {
		t.Errorf("delete query = %q", q)
	}
	om := fixture(t, "media_only_me")
	if err := testItem(insta, strings.Split(om.Path, "/")[4]).Archive(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, om, r.sent[1])
	pin := fixture(t, "pin_timeline_media")
	pk := pin.ReqForm["post_id"]
	if err := testItem(insta, pk+"_42865264907").Pin(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, pin, r.sent[2])
	if got := r.sent[2].payload["post_id"]; got != pk {
		t.Errorf("post_id = %v", got)
	}
}

func TestSearchAndFollowLists(t *testing.T) {
	insta, r := newReplay(t, "account_serp", "info_stream_search", "followers_list", "following_list",
		"following_list_page", "info_stream_following", "show_many")
	serp := fixture(t, "account_serp")
	search := insta.SearchUsers(serp.Query["query"])
	if err := search.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, serp, r.sent[0])
	var target *User
	for _, u := range search.Items {
		if u.ID == 4213518589 {
			target = u
		}
	}
	if target == nil {
		t.Fatalf("searched user missing among %d", len(search.Items))
	}
	if err := target.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "info_stream_search"), r.sent[1])
	if ep := r.sent[1].payload["entry_point"]; ep != "search_navigate_to_user" {
		t.Errorf("entry_point = %v", ep)
	}

	followers := target.Followers()
	if err := followers.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "followers_list"), r.sent[2])
	if len(followers.Items) == 0 || followers.Items[0].ID == 0 || followers.Items[0].Username == "" {
		t.Error("followers not parsed")
	}

	following := target.Following()
	if err := following.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "following_list"), r.sent[3])
	if !following.More() {
		t.Error("following has no next page")
	}
	firstFollowed := following.Items[0]
	if err := following.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "following_list_page"), r.sent[4])
	var gotVars, wantVars map[string]any
	_ = json.Unmarshal([]byte(r.sent[4].payload["variables"].(string)), &gotVars)
	_ = json.Unmarshal([]byte(fixture(t, "following_list_page").ReqForm["variables"]), &wantVars)
	if !reflect.DeepEqual(gotVars, wantVars) {
		t.Errorf("following variables differ:\n got %v\nwant %v", gotVars, wantVars)
	}

	fs := fixture(t, "info_stream_following")
	followed := insta.NewUser(mustInt(t, strings.Split(fs.Path, "/")[4]))
	followed.entry = firstFollowed.entry
	if err := followed.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fs, r.sent[5])
	if ep := r.sent[5].payload["entry_point"]; ep != "following_list_user_row" {
		t.Errorf("entry_point = %v", ep)
	}

	sm := fixture(t, "show_many")
	var ids []int64
	for _, s := range strings.Split(sm.ReqForm["user_ids"], ", ") {
		ids = append(ids, mustInt(t, s))
	}
	st, err := insta.Friendships(t.Context(), ids...)
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, sm, r.sent[6])
	if r.sent[6].payload["user_ids"] != sm.ReqForm["user_ids"] || len(st) != len(ids) {
		t.Errorf("show_many: sent %v, parsed %d", r.sent[6].payload["user_ids"], len(st))
	}
}

func TestSelfInfoStream(t *testing.T) {
	insta, r := newReplay(t, "info_stream_self")
	if err := insta.NewUser(42865264907).Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "info_stream_self"), r.sent[0])
}

func TestActivity(t *testing.T) {
	insta, r := newReplay(t, "news_inbox", "news_inbox_seen")
	if err := insta.Activity().Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "news_inbox"), r.sent[0])
	checkRequest(t, fixture(t, "news_inbox_seen"), r.sent[1])
}

func TestCollections(t *testing.T) {
	insta, r := newReplay(t, "collections_list")
	if err := insta.Collections().Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "collections_list"), r.sent[0], "count", "media_id")
}

func testJPEG(t *testing.T, w, h int) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestUploadPhoto(t *testing.T) {
	insta, r := newReplay(t, "rupload_feed", "configure")
	it, err := insta.UploadPhoto(t.Context(), testJPEG(t, 1080, 1350), PostOptions{Caption: "Love it!"})
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "rupload_feed"), r.sent[0])
	checkRequest(t, fixture(t, "configure"), r.sent[1], "publish_id", "camera_session_id", "caption_add_on")
	if c := r.sent[1].payload["caption"]; c != "Love it!" {
		t.Errorf("caption = %v", c)
	}
	if it.Code == "" || it.insta != insta {
		t.Errorf("configured item not parsed: %q", it.Code)
	}
	var rp map[string]string
	_ = json.Unmarshal([]byte(r.sent[0].req.Header.Get("X-Instagram-Rupload-Params")), &rp)
	if rp["share_type"] != "feed" || rp["upload_id"] == "" {
		t.Errorf("rupload params = %v", rp)
	}
}

func TestUploadAlbum(t *testing.T) {
	insta, r := newReplay(t, "rupload_sidecar", "configure_sidecar")
	album, err := insta.UploadAlbum(t.Context(), []io.Reader{testJPEG(t, 100, 100), testJPEG(t, 100, 120)}, PostOptions{Caption: "Mono caption"})
	if err != nil {
		t.Fatal(err)
	}
	if album.MediaType != MediaAlbum || album.ID != "4017452512634029760_42865264907" || len(album.CarouselMedia) == 0 {
		t.Errorf("album = %s %v with %d children", album.ID, album.MediaType, len(album.CarouselMedia))
	}
	if len(r.sent) != 3 {
		t.Fatalf("sent %d requests, want 3", len(r.sent))
	}
	checkRequest(t, fixture(t, "rupload_sidecar"), r.sent[0])
	checkRequest(t, fixture(t, "configure_sidecar"), r.sent[2], "camera_entry_point", "bottom_camera_dial_selected")
	children := r.sent[2].payload["children_metadata"].([]any)
	if len(children) != 2 {
		t.Fatalf("children = %d", len(children))
	}
	wantChild := fixture(t, "configure_sidecar").ReqSigned["children_metadata"].([]any)[0].(map[string]any)
	gotChild := children[0].(map[string]any)
	for k := range wantChild {
		if _, ok := gotChild[k]; !ok && k != "nav_chain" {
			t.Errorf("child missing %s", k)
		}
	}
	if _, isString := gotChild["edits"].(string); !isString {
		t.Error("child edits must be a JSON string")
	}
	sidecarID := r.sent[2].payload["client_sidecar_id"]
	for i := range 2 {
		var rp map[string]string
		_ = json.Unmarshal([]byte(r.sent[i].req.Header.Get("X-Instagram-Rupload-Params")), &rp)
		if rp["session_id"] != sidecarID || rp["upload_id"] == sidecarID {
			t.Errorf("child %d rupload session_id=%s upload_id=%s, sidecar %v", i, rp["session_id"], rp["upload_id"], sidecarID)
		}
	}
}

func TestUploadStory(t *testing.T) {
	insta, r := newReplay(t, "rupload_story", "configure_story")
	if _, err := insta.UploadStory(t.Context(), testJPEG(t, 1080, 1920)); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "rupload_story"), r.sent[0])
	// The story was captured in the earlier session, before configure moved to the media_upload priority.
	f := fixture(t, "configure_story")
	for _, k := range []string{"priority", "x-fb-request-analytics-tags"} {
		f.ReqHeaders[k] = fixture(t, "configure").ReqHeaders[k]
	}
	checkRequest(t, f, r.sent[1])
}

func TestReels(t *testing.T) {
	insta, r := newReplay(t, "clips_discover", "clips_seen")
	f := fixture(t, "clips_discover")
	reels := insta.Reels()
	if err := reels.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, f, r.sent[0])
	if len(reels.Items) == 0 || reels.Items[0].ID == "" || !reels.More() {
		t.Errorf("reels not parsed: %d items, more %v", len(reels.Items), reels.More())
	}
	if err := insta.MarkReelsSeen(t.Context(), &Item{Pk: 4017333050152331980}); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, fixture(t, "clips_seen"), r.sent[1])
	if got := r.sent[1].payload["impressions"]; got != `["4017333050152331980"]` {
		t.Errorf("impressions = %v", got)
	}
}

func TestProfilePosts(t *testing.T) {
	insta, r := newReplay(t, "profile_timeline")
	f := fixture(t, "profile_timeline")
	posts := insta.NewUser(4213518589).Posts()
	if err := posts.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkRequest(t, f, r.sent[0])
	var gotVars, wantVars map[string]any
	_ = json.Unmarshal([]byte(r.sent[0].payload["variables"].(string)), &gotVars)
	_ = json.Unmarshal([]byte(f.ReqForm["variables"]), &wantVars)
	if !reflect.DeepEqual(gotVars, wantVars) {
		t.Errorf("variables differ:\n got %v\nwant %v", gotVars, wantVars)
	}
	if len(posts.Items) != 3 {
		t.Fatalf("posts = %d, want the 3 grid items kept in the fixture", len(posts.Items))
	}
	for _, it := range posts.Items {
		if it.ID == "" || it.TakenAt == 0 {
			t.Errorf("grid item missing skeleton fields: %+v", it.ID)
		}
	}
}

func TestSessionRoundTrip(t *testing.T) {
	insta, _ := newReplay(t)
	path := t.TempDir() + "/s.json"
	if err := insta.ExportSession(path); err != nil {
		t.Fatal(err)
	}
	again, err := ImportSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Session() != insta.Session() {
		t.Errorf("session changed after round trip")
	}
	if _, err := New(Session{}); err != ErrSessionInvalid {
		t.Errorf("empty session err = %v", err)
	}
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	n := json.Number(s)
	v, err := n.Int64()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func gzipReader(raw []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(zr)
}
