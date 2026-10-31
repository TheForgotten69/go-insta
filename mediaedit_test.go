package goinsta

import (
	"context"
	"testing"
)

func TestMediaManagementRequests(t *testing.T) {
	insta, _ := newReplay(t)
	calls := capture(t, insta, answer(`{"status":"ok","media":{"id":"1_2","caption":{"text":"new"}},"users":[{"pk":7}],"items":[{"id":"1_2","code":"X"}]}`))
	last := func() sent { return (*calls)[len(*calls)-1] }
	item := testItem(insta, "3989118560685351393_62759345752")
	edited := testItem(insta, "3989118560685351393_62759345752")
	comment := &Comment{item: item, ID: "17900000000000001"}
	ctx := t.Context()

	for _, tc := range []struct {
		name, path string
		call       func(context.Context) error
		want       map[string]any // signed body fields, or form fields when form is true
		form       bool
	}{
		{"Edit", "/api/v1/media/3989118560685351393_62759345752/edit_media/",
			func(ctx context.Context) error {
				return edited.Edit(ctx, MediaEdit{Caption: "new", ClearLocation: true, ChildOrder: []*Item{{ID: "a_1"}, {ID: "b_1"}}})
			},
			map[string]any{"caption_text": "new", "location": "{}", "container_module": "edit_media_info",
				"allow_edit_boosted_media": "true", "carousel_children_media_ids_in_order": `["a_1","b_1"]`}, false},
		{"DisableComments", "/api/v1/media/3989118560685351393_62759345752/disable_comments/", item.DisableComments,
			map[string]any{"_uuid": insta.uuid}, false},
		{"EnableComments", "/api/v1/media/3989118560685351393_62759345752/enable_comments/", item.EnableComments,
			map[string]any{"_uuid": insta.uuid}, false},
		{"SetCountsHidden", "/api/v1/media/update_like_and_view_counts_visibility/",
			func(ctx context.Context) error { return item.SetCountsHidden(ctx, true, false) },
			map[string]any{"like_and_view_counts_disabled": "true", "share_count_disabled": "false", "media_id": item.ID}, false},
		{"Unarchive", "/api/v1/media/3989118560685351393_62759345752/undo_only_me/", item.Unarchive,
			map[string]any{"media_id": item.ID}, false},
		{"Pin", "/api/v1/media/3989118560685351393_62759345752/pin_comment/17900000000000001/", comment.Pin,
			map[string]any{"_uuid": insta.uuid}, false},
		{"Unpin", "/api/v1/media/3989118560685351393_62759345752/unpin_comment/17900000000000001/", comment.Unpin,
			map[string]any{"_uuid": insta.uuid}, false},
		{"SetRemixAllowed", "/api/v1/clips/item/set_mashups_allowed/",
			func(ctx context.Context) error { return item.SetRemixAllowed(ctx, false) },
			map[string]any{"media_id": "3989118560685351393", "clips_media_id": "3989118560685351393", "mashups_allowed": "false"}, true},
		{"SetDownloads", "/api/v1/clips/item/set_third_party_downloads_enabled/",
			func(ctx context.Context) error { return item.SetDownloads(ctx, true) },
			map[string]any{"media_id": "3989118560685351393", "third_party_downloads_enabled": "true"}, true},
	} {
		if err := tc.call(ctx); err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		got := last()
		if got.req.Method != "POST" || got.req.URL.Path != tc.path {
			t.Errorf("%s: %s %s, want POST %s", tc.name, got.req.Method, got.req.URL.Path, tc.path)
		}
		if got.signed == tc.form {
			t.Errorf("%s: signed body = %v, want %v", tc.name, got.signed, !tc.form)
		}
		for k, want := range tc.want {
			if got.payload[k] != want {
				t.Errorf("%s: %s = %v, want %v", tc.name, k, got.payload[k], want)
			}
		}
	}

	if edited.ID != "1_2" || edited.Caption.Text != "new" {
		t.Errorf("Edit did not refresh the item: %s %q", edited.ID, edited.Caption.Text)
	}
	if likers, err := item.FetchLikers(ctx); err != nil || last().req.Method != "GET" || len(likers) != 1 || likers[0].insta == nil {
		t.Errorf("FetchLikers: %v %s %d", err, last().req.Method, len(likers))
	}
	if it, err := insta.MediaInfo(ctx, "1_2"); err != nil || last().req.URL.Path != "/api/v1/media/1_2/info/" || it.Code != "X" {
		t.Errorf("MediaInfo: %v %s %+v", err, last().req.URL.Path, it)
	}
}
