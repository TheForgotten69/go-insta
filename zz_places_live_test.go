//go:build live

package goinsta

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// go test -tags live -run TestLivePlaces -v . (TAG_REMOVE_MEDIA=<media id> also removes the own tag from that post)
func TestLivePlaces(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	var log bytes.Buffer
	insta.Logger = slog.New(slog.NewJSONHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	defer dumpBodies(t, &log, "location_search", "/info/", "/sections/", "usertags/")
	ctx := t.Context()
	if err := insta.Account.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	tagged := insta.Account.User().TaggedPosts()
	err = tagged.Next(ctx)
	t.Logf("TaggedPosts: %d items, more=%v err=%v", len(tagged.Items), tagged.More(), err)

	locs, err := insta.SearchLocations(ctx, "Paris", 0, 0)
	t.Logf("SearchLocations: %d results err=%v", len(locs), err)
	if len(locs) == 0 {
		return
	}
	t.Logf("first: %+v", locs[0])
	loc, err := insta.LocationInfo(ctx, int64(locs[0].ID))
	t.Logf("LocationInfo: %+v err=%v", loc, err)
	feed := insta.LocationFeed(int64(locs[0].ID), LocationTabRecent)
	err = feed.Next(ctx)
	t.Logf("LocationFeed: %d items, more=%v err=%v", len(feed.Items), feed.More(), err)
	if len(feed.Items) > 0 {
		t.Logf("first feed item %s code=%s", feed.Items[0].ID, feed.Items[0].Code)
	}

	if id := os.Getenv("TAG_REMOVE_MEDIA"); id != "" {
		t.Logf("RemoveTag %s: %v", id, testItem(insta, id).RemoveTag(ctx))
	}
}

func dumpBodies(t *testing.T, log *bytes.Buffer, endpoints ...string) {
	for _, l := range strings.Split(log.String(), "\n") {
		var d struct {
			Endpoint string `json:"endpoint"`
			Status   int    `json:"status"`
			Body     string `json:"body"`
		}
		if json.Unmarshal([]byte(l), &d) != nil {
			continue
		}
		for _, e := range endpoints {
			if strings.Contains(d.Endpoint, e) {
				b, _ := base64.StdEncoding.DecodeString(d.Body)
				t.Logf("%d %s: %.1500s", d.Status, d.Endpoint, b)
			}
		}
	}
}
