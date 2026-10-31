//go:build live

package goinsta

import (
	"os"
	"strconv"
	"testing"
	"time"
)

// Reads the inbox and the first thread; with DM_TO=<user id> it also sends a
// text and marks it seen.
func TestLiveDirect(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()

	in := insta.Inbox()
	if err := in.Next(ctx); err != nil {
		t.Fatalf("inbox: %v", err)
	}
	t.Logf("inbox: %d threads, unseen %d, pending %d, more %v", len(in.Items), in.UnseenCount, in.PendingTotal, in.More())
	pending := insta.PendingInbox()
	t.Logf("pending inbox: %v", pending.Next(ctx))

	if len(in.Items) > 0 {
		th := in.Items[0]
		msgs := th.Messages()
		err := msgs.Next(ctx)
		t.Logf("thread %s %q: %d messages, err %v, more %v", th.ThreadID, th.Title, len(msgs.Items), err, msgs.More())
		for _, m := range msgs.Items {
			t.Logf("  %s %s %q", m.ItemID, m.ItemType, m.Text)
		}
	}

	to := os.Getenv("DM_TO")
	if to == "" {
		return
	}
	uid, err := strconv.ParseInt(to, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	sent, err := insta.SendDirectText(ctx, "go-insta live test "+time.Now().Format(time.TimeOnly), uid)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	t.Logf("sent item %s in thread %s", sent.ItemID, sent.ThreadID)
	th, err := insta.Thread(ctx, sent.ThreadID)
	if err != nil {
		t.Fatalf("thread: %v", err)
	}
	t.Logf("thread now has %d items, newest %q", len(th.Items), th.Items[0].Text)
	t.Logf("mark seen: %v", th.MarkSeen(ctx, th.Items[0]))
	reply, err := th.SendText(ctx, "go-insta live reply")
	t.Logf("send to thread: %+v %v", reply, err)
}
