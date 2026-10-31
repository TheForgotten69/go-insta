package goinsta

import (
	"net/http"
	"net/url"
	"testing"
)

func TestLocations(t *testing.T) {
	insta, _ := newReplay(t)
	var sent []*http.Request
	var forms []url.Values
	responses := []string{
		`{"venues":[{"external_id":106078429431815,"external_id_source":"facebook_places","name":"Paris, France","address":"","lat":48.8566,"lng":2.35222}],"rank_token":"r","status":"ok"}`,
		`{"location":{"pk":106078429431815,"name":"Paris, France","lat":48.8566,"lng":2.35222},"status":"ok"}`,
		`{"sections":[{"layout_content":{"medias":[{"media":{"id":"1_2","pk":1}},{"media":{"id":"3_4","pk":3}}],"fill_items":[{"media":{"id":"5_6","pk":5}}]}}],` +
			`"more_available":true,"next_max_id":"MX","next_page":2,"next_media_ids":[11,12],"status":"ok"}`,
		`{"sections":[],"more_available":false,"status":"ok"}`,
	}
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		sent = append(sent, req)
		f := url.Values{}
		if req.Body != nil {
			f = sentForm(t, req)
		}
		forms = append(forms, f)
		return 200, responses[len(sent)-1]
	})})
	ctx := t.Context()

	locs, err := insta.SearchLocations(ctx, "Paris", 0, 0)
	if err != nil || len(locs) != 1 || locs[0].ID != 106078429431815 || locs[0].Name != "Paris, France" {
		t.Fatalf("search = %+v, %v", locs, err)
	}
	if q := sent[0].URL.Query(); sent[0].Method != "GET" || sent[0].URL.Path != "/api/v1/location_search/" ||
		q.Get("search_query") != "Paris" || q.Get("latitude") != "0.000000" || q.Get("longitude") != "0.000000" {
		t.Errorf("search request: %s %s %v", sent[0].Method, sent[0].URL.Path, q)
	}

	loc, err := insta.LocationInfo(ctx, int64(locs[0].ID))
	if err != nil || loc.Name != "Paris, France" || sent[1].Method != "GET" || sent[1].URL.Path != "/api/v1/locations/106078429431815/info/" {
		t.Errorf("info = %+v, %v (%s %s)", loc, err, sent[1].Method, sent[1].URL.Path)
	}

	feed := insta.LocationFeed(int64(locs[0].ID), LocationTabRecent)
	if err := feed.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 3 || feed.Items[2].ID != "5_6" || feed.Items[2].Index != 2 || !feed.More() {
		t.Fatalf("feed page 1: %d items, more %v", len(feed.Items), feed.More())
	}
	if f := forms[2]; sent[2].Method != "POST" || sent[2].URL.Path != "/api/v1/locations/106078429431815/sections/" ||
		f.Get("tab") != "recent" || f.Get("exclude_bloks_widgets") != "true" || f.Get("_uuid") != insta.uuid ||
		f.Get("session_id") == "" || f.Has("max_id") || f.Has("page") {
		t.Errorf("feed page 1 request: %s %s %v", sent[2].Method, sent[2].URL.Path, f)
	}
	if err := feed.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if f := forms[3]; f.Get("max_id") != "MX" || f.Get("page") != "2" || f.Get("next_media_ids") != "[11, 12]" ||
		f.Get("session_id") != forms[2].Get("session_id") {
		t.Errorf("feed page 2 request: %v", f)
	}
	if feed.More() {
		t.Error("feed should end")
	}

	if got := coordinate(48.8566, 2.35222, 48.8566); got != "48.8566" {
		t.Errorf("coordinate = %q", got)
	}
	if got := coordinate(45, 3, 45); got != "45.0" {
		t.Errorf("coordinate = %q", got)
	}
}
