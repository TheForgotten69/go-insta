package goinsta

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type handlerFunc func(req *http.Request) (int, string)

func (f handlerFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	code, body := f(req)
	return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func formOf(t *testing.T, req *http.Request) url.Values {
	t.Helper()
	raw, _ := io.ReadAll(req.Body)
	v, err := url.ParseQuery(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMediaIDFromShortID(t *testing.T) {
	for code, want := range map[string]string{"B": "1", "BA": "64", "DjVzxj0hXsX": "4095407142251887383"} {
		if got, err := MediaIDFromShortID(code); err != nil || got != want {
			t.Errorf("%s = %s, %v; want %s", code, got, err, want)
		}
	}
	for _, bad := range []string{"", "abc$", "AAAAAAAAAAAA"} {
		if _, err := MediaIDFromShortID(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestParseUserAgentRoundTrip(t *testing.T) {
	d, ok := parseUserAgent(createUserAgent(Waydroid, "en_US"))
	if !ok || d != Waydroid {
		t.Errorf("parsed %+v, %v", d, ok)
	}
	insta, _ := newReplay(t)
	if m := insta.deviceInfo()["manufacturer"]; m != "Waydroid" {
		t.Errorf("manufacturer = %v", m)
	}
}

func TestBandwidthSampleIsSticky(t *testing.T) {
	var b bandwidth
	now := time.Now()
	s1, bytes1, _ := b.sample(now)
	if bytes1 != 0 {
		t.Errorf("cold start bytes = %d", bytes1)
	}
	if s2, _, _ := b.sample(now.Add(10 * time.Second)); s2 != s1 {
		t.Error("sample changed within 20 s")
	}
	if _, bytes3, ms3 := b.sample(now.Add(2 * time.Minute)); bytes3 == 0 || ms3 == 0 {
		t.Error("no measured sample after the cold start window")
	}
}

func TestDownloadTo(t *testing.T) {
	insta, _ := newReplay(t)
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		if strings.Contains(req.URL.Path, "missing") {
			return 404, "nope"
		}
		return 200, "jpegdata:" + req.URL.Path
	})})
	photo := func(u string) Item {
		return Item{insta: insta, MediaType: 1, Images: Images{Versions: []Candidate{{Width: 1, Height: 1, URL: u}}}}
	}
	dir := t.TempDir()
	t.Chdir(dir)

	it := photo("https://cdn/x/pic.jpg")
	p1, err := it.DownloadTo(t.Context(), "out")
	if err != nil || len(p1) != 1 || p1[0] != "out.jpg" {
		t.Fatalf("bare name: %v %v", p1, err)
	}
	p2, err := it.DownloadTo(t.Context(), "out")
	if err != nil || p2[0] != "out.1.jpg" {
		t.Fatalf("second download overwrote or failed: %v %v", p2, err)
	}

	album := Item{insta: insta, MediaType: 8, ID: "9_9", CarouselMedia: []Item{photo("https://cdn/a.jpg"), photo("https://cdn/b.jpg")}}
	paths, err := album.DownloadTo(t.Context(), dir+string(os.PathSeparator))
	if err != nil || len(paths) != 2 {
		t.Fatalf("carousel: %v %v", paths, err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "9_9_2.jpg"))
	if string(b) != "jpegdata:/b.jpg" {
		t.Errorf("child 2 = %q", b)
	}

	bad := photo("https://cdn/missing.jpg")
	if _, err := bad.DownloadTo(t.Context(), "gone"); err == nil {
		t.Error("404 saved as media")
	}
	if _, err := os.Stat("gone.jpg"); !errors.Is(err, os.ErrNotExist) {
		t.Error("file created for a failed download")
	}
}

func TestConcurrentRequests(t *testing.T) {
	insta, _ := newReplay(t)
	ok := &http.Client{Transport: handlerFunc(func(*http.Request) (int, string) { return 200, `{"status":"ok"}` })}
	insta.SetHTTPClient(ok)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if _, err := insta.Account.ReelSettings(t.Context()); err != nil {
				t.Error(err)
			}
		})
		wg.Go(func() { insta.SetHTTPClient(ok) })
		wg.Go(func() { _ = insta.Session() })
	}
	wg.Wait()
}

func TestUploadAlbumBounds(t *testing.T) {
	insta, _ := newReplay(t)
	if _, err := insta.UploadAlbum(t.Context(), []io.Reader{testJPEG(t, 10, 10)}, PostOptions{}); !errors.Is(err, ErrCarouselMediaLimit) {
		t.Errorf("one photo: %v", err)
	}
	png := bytes.NewReader([]byte("\x89PNG\r\n\x1a\n"))
	if _, err := insta.UploadAlbum(t.Context(), []io.Reader{testJPEG(t, 10, 10), png}, PostOptions{}); !errors.Is(err, ErrInvalidImage) {
		t.Errorf("png child: %v", err)
	}
}

func TestProxyNeedsPort(t *testing.T) {
	insta, _ := newReplay(t)
	if err := insta.SetProxy("http://proxy.local", false); err == nil || !strings.Contains(err.Error(), "missing port") {
		t.Errorf("err = %v", err)
	}
}

func TestLoadReelsBatches(t *testing.T) {
	insta, _ := newReplay(t)
	var calls int
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		calls++
		var p struct {
			ReelIDs []string `json:"reel_ids"`
		}
		_, js, _ := strings.Cut(formOf(t, req).Get("signed_body"), ".")
		_ = json.Unmarshal([]byte(js), &p)
		// Live shape: client_hints lines with an empty reels map, then one reel per line.
		var lines []string
		for _, id := range p.ReelIDs {
			hints, _ := json.Marshal(map[string]any{"reels": map[string]any{}, "client_hints": map[string]any{}})
			reel, _ := json.Marshal(map[string]any{"reels": map[string]any{
				id: map[string]any{"id": id, "items": []any{map[string]any{"id": id + "_item"}}},
			}})
			lines = append(lines, string(hints), string(reel))
		}
		return 200, strings.Join(lines, "\n")
	})})
	var reels []*Reel
	for i := range reelsBatch + 5 {
		id, _ := json.Marshal(Int64(i + 1).String())
		reels = append(reels, &Reel{insta: insta, ID: id})
	}
	loaded := &Reel{insta: insta, ID: json.RawMessage(`"99"`), Items: []*Item{{ID: "x"}}}
	if err := insta.LoadReels(t.Context(), append(reels, loaded)...); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("requests = %d, want 2", calls)
	}
	for _, r := range reels {
		if len(r.Items) != 1 || r.Items[0].ID != r.ReelID()+"_item" || r.Items[0].insta != insta {
			t.Fatalf("reel %s not filled: %+v", r.ReelID(), r.Items)
		}
	}
}

func TestPostsPagingSendsCursor(t *testing.T) {
	insta, _ := newReplay(t)
	var vars []map[string]any
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		raw, _ := io.ReadAll(req.Body)
		zr, _ := gzipReader(raw)
		form, _ := url.ParseQuery(string(zr))
		var v map[string]any
		_ = json.Unmarshal([]byte(form.Get("variables")), &v)
		vars = append(vars, v)
		more := len(vars) == 1
		b, _ := json.Marshal(map[string]any{"data": map[string]any{"xdt_api__v1__profile_timeline": map[string]any{
			"profile_grid_items":        []any{map[string]any{"media": map[string]any{"id": "1_1", "pk": 1, "taken_at": 5}}},
			"more_available":            more,
			"next_max_id":               "M",
			"profile_grid_items_cursor": "C",
		}}})
		return 200, string(b)
	})})
	posts, err := insta.NewUser(1).Posts().Collect(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 || posts[1].Index != 1 {
		t.Fatalf("posts = %d, second index %d", len(posts), posts[len(posts)-1].Index)
	}
	if vars[1]["max_id"] != "M" || vars[1]["profile_grid_items_cursor"] != "C" || vars[1]["request_media_chunk"] != false {
		t.Errorf("page 2 variables: %v", vars[1])
	}
	if _, has := vars[0]["max_id"]; has {
		t.Error("first page sent max_id")
	}
}

func TestOnSessionChange(t *testing.T) {
	insta, _ := newReplay(t)
	var got []Session
	insta.OnSessionChange = func(s Session) { got = append(got, s) }

	h := http.Header{}
	h.Set("Ig-Set-Authorization", "Bearer IGT:2:new")
	insta.extractHeaders(h)
	insta.extractHeaders(h) // same value, must not fire again
	h.Set("Ig-Set-Ig-U-Rur", "rur2")
	insta.extractHeaders(h)
	h.Set("Ig-Set-Password-Encryption-Pub-Key", "key")
	h.Set("Ig-Set-Password-Encryption-Key-Id", "181")
	insta.extractHeaders(h)
	insta.extractHeaders(h) // same key, must not fire again

	if len(got) != 3 {
		t.Fatalf("fired %d times, want 3", len(got))
	}
	if got[0].Authorization != "Bearer IGT:2:new" || got[1].Rur != "rur2" {
		t.Errorf("payloads: %q then rur %q", got[0].Authorization, got[1].Rur)
	}
}

func TestSearchUsersPaging(t *testing.T) {
	insta, _ := newReplay(t)
	var tokens []string
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		tokens = append(tokens, req.URL.Query().Get("page_token"))
		more := len(tokens) == 1
		id := len(tokens)
		return 200, fmt.Sprintf(`{"users":[{"pk":%d,"username":"u%d"}],"has_more":%v,"page_token":"T%d"}`, id, id, more, id)
	})})
	users, err := insta.SearchUsers("x").Collect(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[1].Username != "u2" || users[0].entry != entrySearch {
		t.Fatalf("users %+v", users)
	}
	if tokens[0] != "" || tokens[1] != "T1" {
		t.Errorf("page tokens sent: %q", tokens)
	}
}

func TestCollectionPosts(t *testing.T) {
	insta, _ := newReplay(t)
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		if !strings.Contains(req.URL.Path, "/feed/collection/42/") {
			return 404, "{}"
		}
		max := req.URL.Query().Get("max_id")
		if max == "" {
			return 200, `{"items":[{"media":{"id":"1_1","pk":1}}],"more_available":true,"next_max_id":"N"}`
		}
		return 200, `{"items":[{"media":{"id":"2_2","pk":2}}],"more_available":false}`
	})})
	c := &Collection{insta: insta, ID: "42"}
	items, err := c.Posts().Collect(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "1_1" || items[1].Index != 1 || items[0].insta != insta {
		t.Fatalf("items %+v", items)
	}
}

func TestCollectionPostsAutoRoute(t *testing.T) {
	insta, _ := newReplay(t)
	var path string
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		path = req.URL.Path
		return 200, `{"items":[],"more_available":false}`
	})})
	auto := &Collection{insta: insta, ID: "ALL_MEDIA_AUTO_COLLECTION", Type: "ALL_MEDIA_AUTO_COLLECTION"}
	if !auto.IsAuto() {
		t.Fatal("auto collection not detected")
	}
	if _, err := auto.Posts().Collect(t.Context(), 0); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v1/feed/saved/posts/" {
		t.Errorf("auto collection hit %q", path)
	}
}

func TestTOTP(t *testing.T) {
	// RFC 6238 SHA1 vector: secret "12345678901234567890" at 59s -> 94287082.
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	got, err := TOTP(secret, time.Unix(59, 0))
	if err != nil || got != "287082" {
		t.Fatalf("TOTP = %q, %v; want 287082", got, err)
	}
	// Spaces and lowercase from the app's setup screen are accepted.
	if lc, err := TOTP("gezd gnbv gy3t qojq gezd gnbv gy3t qojq", time.Unix(59, 0)); err != nil || lc != got {
		t.Errorf("formatted secret = %q, %v", lc, err)
	}
	if _, err := TOTP("not valid base32 !!!", time.Now()); err == nil {
		t.Error("bad secret accepted")
	}
}
