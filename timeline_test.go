package goinsta

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type staticBody string

func (b staticBody) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b))), Request: req}, nil
}

// A feed of recommended posts (account follows few people) wraps each item in
// explore_story; followed posts come as a bare media_or_ad. Shape from a live
// 2026-10-28 response. The body is one line: timeline responses are NDJSON.
func TestTimelineParsesWrappedItems(t *testing.T) {
	insta, _ := newReplay(t)
	insta.SetHTTPClient(&http.Client{Transport: staticBody(`{"feed_items":[ {"media_or_ad":{"id":"1_1","pk":1}}, {"explore_story":{"id":2,"source":"feed_recs","media_or_ad":{"id":"2_2","pk":2}}}, {"end_of_feed_demarcator":{"title":"done"}} ],"more_available":true,"next_max_id":"abc","status":"ok"}`)})

	tl := insta.Timeline()
	if err := tl.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(tl.Items) != 2 || tl.Items[0].ID != "1_1" || tl.Items[1].ID != "2_2" {
		t.Fatalf("items = %+v", tl.Items)
	}
	if tl.Items[1].Index != 1 || !tl.More() || tl.nextID != "abc" {
		t.Errorf("paging state wrong: index=%d more=%v next=%q", tl.Items[1].Index, tl.More(), tl.nextID)
	}
}

// recorder answers every request with {"status":"ok"} and keeps the last one.
type recorder struct{ last *http.Request }

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.last = req
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"status":"ok"}`)), Request: req}, nil
}

// Header profile of the current build, measured over 529 logged-in requests in the
// 2026-10-28 captures: everything goes to i.instagram.com, the zero-rating headers
// are gone and the network-stack headers are present.
func TestCurrentBuildProfile(t *testing.T) {
	insta, _ := newReplay(t)
	rec := &recorder{}
	insta.SetHTTPClient(&http.Client{Transport: rec})
	if _, err := insta.Account.ReelSettings(t.Context()); err != nil {
		t.Fatal(err)
	}
	h := rec.last.Header
	if rec.last.URL.Host != "i.instagram.com" {
		t.Errorf("host = %s, want i.instagram.com", rec.last.URL.Host)
	}
	for _, k := range []string{"X-Zero-Balance", "X-Zero-Eh", "X-Zero-D-Device-Id", "Zero-Http-Network-Interface"} {
		if h.Get(k) != "" {
			t.Errorf("%s sent; the current build dropped it", k)
		}
	}
	want := map[string]string{
		"X-Fb-Network-Properties":        "Validated;",
		"X-Fb-Tasos-Td-V2-Config":        "prod_android",
		"X-Meta-Tasos-Congestion-Config": "v6_1",
		"X-Fb-Rmd":                       "state=URL_ELIGIBLE",
	}
	for k, v := range want {
		if h.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, h.Get(k), v)
		}
	}
	if !strings.HasSuffix(h.Get("X-Fb-Appnetsession-Nid"), ",Wifi") || len(h.Get("X-Fb-Appnetsession-Sid")) != 32 ||
		!strings.HasPrefix(h.Get("X-Fb-Session-Id"), "nid=") || h.Get("X-Fb-Session-Private") == "" {
		t.Errorf("network session headers malformed: %v", h)
	}
}
