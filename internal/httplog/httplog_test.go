package httplog

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func TestParseAndScrub(t *testing.T) {
	token := "IGT:2:" + base64.StdEncoding.EncodeToString([]byte(`{"ds_user_id":"42","sessionid":"42%3Asecret%3A1"}`))
	log := "noise\n" +
		"===== 2026-10-27 19:00:25.096 POST https://i.instagram.com/api/v1/media/1_2/like/?x=1 =====\n" +
		"> authorization: Bearer " + token + "\n" +
		"> content-type: application/x-www-form-urlencoded; charset=UTF-8\n" +
		"> x-ig-android-id: android-abcdefabcdefabcd\n" +
		"--- request body ---\n" +
		"signed_body=SIGNATURE.%7B%22media_id%22%3A%221_2%22%2C%22s%22%3A%2242%3Asecret%3A1%22%7D&d=0\n" +
		"--- response 200 OK ---\n" +
		"< content-type: application/json\n" +
		"--- response body ---\n" +
		"{\"a\":1}\n{\"a\":2,\"device\":\"android-abcdefabcdefabcd\"}\n\n" +
		"===== 2026-10-27 19:00:26.000 GET https://graph.facebook.com/x =====\n" +
		"--- response 204 No Content ---\n"

	entries, err := Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries", len(entries))
	}
	e := entries[0]
	if e.Method != "POST" || e.Path != "/api/v1/media/1_2/like/" || e.Query["x"] != "1" || e.Status != 200 {
		t.Errorf("bad entry: %+v", e)
	}
	if e.ReqForm["d"] != "0" || e.ReqSigned["media_id"] != "1_2" {
		t.Errorf("form not decoded: %v %v", e.ReqForm, e.ReqSigned)
	}
	if string(e.LastJSON()) != `{"a":2,"device":"android-abcdefabcdefabcd"}` {
		t.Errorf("LastJSON = %s", e.LastJSON())
	}
	if entries[1].Status != 204 {
		t.Errorf("second status = %d", entries[1].Status)
	}

	s, err := ExtractSession(entries)
	if err != nil {
		t.Fatal(err)
	}
	if s.UserID != 42 {
		t.Errorf("user id from bearer = %d", s.UserID)
	}
	scrubbed := Scrub(e, s)
	all := scrubbed.ReqHeaders["authorization"] + scrubbed.RespBody + scrubbed.ReqSigned["s"].(string)
	for _, secret := range []string{token, "42:secret:1", "android-abcdefabcdefabcd"} {
		if strings.Contains(all, secret) {
			t.Errorf("scrub leaked %q", secret)
		}
	}
}

func TestScrubPressSignals(t *testing.T) {
	signals := `{"client_ipv4":"203.0.113.7","client_ipv6":"2001:db8::1"}`
	e := Entry{
		ReqForm: map[string]string{"press_signals": signals, "reason": "pagination"},
		ReqBody: "reason=pagination&press_signals=" + url.QueryEscape(signals),
	}
	got := Scrub(e, Session{})
	for _, s := range []string{got.ReqForm["press_signals"], got.ReqBody} {
		if strings.Contains(s, "203.0.113.7") || strings.Contains(s, "2001:db8") {
			t.Errorf("client ip left in %q", s)
		}
	}
	if got.ReqForm["reason"] != "pagination" {
		t.Errorf("other fields changed: %v", got.ReqForm)
	}
}
