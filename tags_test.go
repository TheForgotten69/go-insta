package goinsta

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestTaggedPosts(t *testing.T) {
	insta, r := newReplay(t, "usertags_feed", "usertags_feed")
	r.fixtures[0].RespBody = `{"items":[{"id":"1_2","pk":1},{"id":"3_4","pk":3}],"more_available":true,"next_max_id":"M1","status":"ok"}`
	tagged := insta.NewUser(42865264907).TaggedPosts()
	if err := tagged.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	f := fixture(t, "usertags_feed")
	checkRequest(t, f, r.sent[0])
	if len(tagged.Items) != 2 || tagged.Items[1].Index != 1 || tagged.Items[0].insta == nil || !tagged.More() {
		t.Fatalf("page 1: %d items, more %v", len(tagged.Items), tagged.More())
	}
	if err := tagged.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
	if q := r.sent[1].req.URL.Query(); q.Get("max_id") != "M1" || q.Get("count") != "12" {
		t.Errorf("page 2 query = %v", q)
	}
	if tagged.More() {
		t.Error("captured empty page should end the feed")
	}
}

func TestRemoveTag(t *testing.T) {
	insta, _ := newReplay(t)
	var path string
	var body map[string]any
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		path = req.URL.Path
		_, js, _ := strings.Cut(sentForm(t, req).Get("signed_body"), ".")
		_ = json.Unmarshal([]byte(js), &body)
		return 200, `{"status":"ok"}`
	})})
	item := testItem(insta, "3989118560685351393_62759345752")
	if err := item.RemoveTag(t.Context()); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v1/usertags/3989118560685351393_62759345752/remove/" ||
		body["media_id"] != item.ID || body["container_module"] != "feed_contextual_self_profile" || body["_uuid"] != insta.uuid {
		t.Errorf("remove tag: %s %v", path, body)
	}
}
