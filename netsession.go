package goinsta

import (
	"crypto/rand"
	"encoding/hex"
	mrand "math/rand/v2"
	"sync"
	"time"
)

// netSession holds the network-stack session ids the app's Tigon layer attaches.
// They stay fixed for one app run. Formats from the 2026-10-28 captures:
// appnetsession nid "<32 hex>,Wifi", sid "<32 hex>"; session-id
// "nid=<12 alnum>;nc=1;fc=1;bc=0;"; session-private "<12 alnum>".
type netSession struct {
	nid, sid, sessionID, sessionPrivate string
}

func newNetSession() netSession {
	return netSession{
		nid:            randHex(16) + ",Wifi",
		sid:            randHex(16),
		sessionID:      "nid=" + randAlnum(12) + ";nc=1;fc=1;bc=0;",
		sessionPrivate: randAlnum(12),
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randAlnum(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// bandwidth mimics the app's x-ig-bandwidth-* sample: one estimate is reused
// for 20-60 s before the next replaces it, and a cold start reports a cached
// speed with no bytes measured yet (2026-10-27 capture).
type bandwidth struct {
	mu                       sync.Mutex
	until                    time.Time
	speed, totalBytes, total int
}

func (b *bandwidth) sample(now time.Time) (speedKbps, totalBytes, totalMs int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.until.IsZero():
		b.speed = 20_000 + mrand.IntN(70_000)
	case now.After(b.until):
		b.speed = 2_500 + mrand.IntN(9_000)
		b.totalBytes = 3_000_000 + mrand.IntN(57_000_000)
		b.total = b.totalBytes / (3_500 + mrand.IntN(7_500)) // 3.5k-11k bytes/ms, as captured
	default:
		return b.speed, b.totalBytes, b.total
	}
	b.until = now.Add(time.Duration(20+mrand.IntN(40)) * time.Second)
	return b.speed, b.totalBytes, b.total
}
