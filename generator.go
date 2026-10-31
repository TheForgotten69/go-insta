package goinsta

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	mrand "math/rand/v2"
	"strconv"
	"time"
	"unicode/utf16"
)

// signedBody wraps a JSON payload the way the app does: no HMAC, the literal
// "SIGNATURE." prefix is all the server checks.
func signedBody(payload any) (map[string]string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return map[string]string{"signed_body": "SIGNATURE." + string(b)}, nil
}

// formOf encodes a request struct as form or query values through its json
// tags: string fields as they are, any other field as its JSON text.
func formOf(v any) (map[string]string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("form %T: %w", v, err)
	}
	out := make(map[string]string, len(raw))
	for k, r := range raw {
		var s string
		if json.Unmarshal(r, &s) == nil {
			out[k] = s
		} else {
			out[k] = string(r)
		}
	}
	return out, nil
}

// sessionIDs is the _uid/_uuid pair most signed bodies carry.
type sessionIDs struct {
	UID  string `json:"_uid"`
	UUID string `json:"_uuid"`
}

func (insta *Instagram) sessionIDs() sessionIDs {
	return sessionIDs{UID: strconv.FormatInt(insta.userID(), 10), UUID: insta.uuid}
}

func generateUUID() string {
	u := make([]byte, 16)
	_, _ = rand.Read(u)
	u[8] = u[8]&^0xc0 | 0x80
	u[6] = u[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
}

const breadcrumbKey = "iN4$aGr0m"

// userBreadcrumb is "b64(hmac)\nb64(data)\n" with data "<len> <typing ms> 0 <unix ms>";
// len counts UTF-16 units like Java's String.length(). Verified against two captured comments.
func userBreadcrumb(text string) string {
	n := len(utf16.Encode([]rune(text)))
	typing := n*(150+mrand.IntN(200)) + 300 + mrand.IntN(900)
	data := fmt.Sprintf("%d %d 0 %d", n, typing, time.Now().UnixMilli())
	return breadcrumbFor(data)
}

func breadcrumbFor(data string) string {
	mac := hmac.New(sha256.New, []byte(breadcrumbKey))
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)) + "\n" + base64.StdEncoding.EncodeToString([]byte(data)) + "\n"
}
