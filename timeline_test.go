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

	if !insta.Timeline.Next() {
		t.Fatal(insta.Timeline.Error())
	}
	tl := insta.Timeline
	if len(tl.Items) != 2 || tl.Items[0].ID != "1_1" || tl.Items[1].ID != "2_2" {
		t.Fatalf("items = %+v", tl.Items)
	}
	if tl.Items[1].Index != 1 || !tl.MoreAvailable || tl.NextID != "abc" {
		t.Errorf("paging state wrong: index=%d more=%v next=%q", tl.Items[1].Index, tl.MoreAvailable, tl.NextID)
	}
}
