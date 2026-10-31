//go:build live

package goinsta

import (
	"os"
	"strconv"
	"testing"
	"time"
)

// Run: REL_TARGET=<user id> go test -tags live -run TestLiveRelationships -v .
// Block, close friends and mute are undone in the same run. RemoveFollower and
// approve/ignore only log unless REL_DESTRUCTIVE=1.
func TestLiveRelationships(t *testing.T) {
	target, err := strconv.ParseInt(os.Getenv("REL_TARGET"), 10, 64)
	if err != nil {
		t.Skip("REL_TARGET not set")
	}
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()
	if err := insta.Account.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	u := insta.NewUser(target)
	pause := func() { time.Sleep(2 * time.Second) }
	show := func(name string, err error) {
		pause()
		f, ferr := u.LoadFriendship(ctx)
		if ferr != nil {
			t.Logf("%-22s err=%v (friendship: %v)", name, err, ferr)
			return
		}
		t.Logf("%-22s err=%v blocking=%v following=%v muting=%v is_muting_reel=%v", name, err, f.Blocking, f.Following, f.Muting, f.IsMutingReel)
	}

	show("Block", u.Block(ctx))
	blocked, err := insta.Account.BlockedUsers(ctx)
	listed := false
	for _, b := range blocked {
		listed = listed || int64(b.UserID) == target
	}
	t.Logf("BlockedUsers err=%v n=%d target listed=%v", err, len(blocked), listed)
	show("Unblock", u.Unblock(ctx))

	pause()
	t.Logf("UpdateCloseFriends add err=%v", insta.Account.UpdateCloseFriends(ctx, []*User{u}, nil))
	pause()
	t.Logf("UpdateCloseFriends remove err=%v", insta.Account.UpdateCloseFriends(ctx, nil, []*User{u}))

	show("Mute posts+stories", u.Mute(ctx, true, true))
	show("Unmute posts+stories", u.Unmute(ctx, true, true))

	pause()
	pending, err := insta.Account.PendingFollowRequests(ctx)
	t.Logf("PendingFollowRequests err=%v n=%d", err, len(pending))
	for _, p := range pending {
		t.Logf("  pending @%s (%d)", p.Username, p.ID)
	}

	if os.Getenv("REL_DESTRUCTIVE") != "1" {
		t.Logf("skipping RemoveFollower(%d) and approve/ignore of pending requests (REL_DESTRUCTIVE!=1)", target)
		return
	}
	show("RemoveFollower", u.RemoveFollower(ctx))
	for _, p := range pending {
		pause()
		t.Logf("ApproveFollowRequest @%s err=%v", p.Username, p.ApproveFollowRequest(ctx))
	}
}
