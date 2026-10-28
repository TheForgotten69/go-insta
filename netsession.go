package goinsta

import (
	"crypto/rand"
	"encoding/hex"
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
