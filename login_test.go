package goinsta

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// buildBloksHeaders reproduces the triple-backslash escaping the real send_login_request
// response uses to embed the session headers, so the parser is tested against that shape.
func buildBloksHeaders(pairs map[string]string) string {
	var b strings.Builder
	b.WriteString(`{"layout":{"bloks_payload":{"action":"(bk.action.map.Make ... headers=`)
	// Each pair rendered as \\\\\\\"key\\\\\\\": \\\\\\\"value\\\\\\\"
	q := strings.Repeat(`\`, 7) + `"`
	first := true
	for k, v := range pairs {
		if !first {
			b.WriteString(",")
		}
		first = false
		b.WriteString(q + k + q + ": " + q + v + q)
	}
	b.WriteString(` ...)"}}}`)
	return b.String()
}

func TestApplyLoginResponse(t *testing.T) {
	insta, _ := newReplay(t)
	body := buildBloksHeaders(map[string]string{
		"IG-Set-Authorization":               "Bearer IGT:2:eyJkc191c2VyX2lkIjoiNDI4NjUifQ==",
		"ig-set-ig-u-rur":                    "CLN,42865,1800000000:01abcdef",
		"x-ig-set-www-claim":                 "hmac.AR1234567890abcdef",
		"ig-set-ig-u-ds-user-id":             "42865264907",
		"ig-set-x-mid":                       "abcdefMIDvalue",
		"IG-Set-Password-Encryption-Key-Id":  "182",
		"IG-Set-Password-Encryption-Pub-Key": "LS0tLS1CRUdJTiBQVUJMSUMgS0VZ+/=",
	})
	if err := insta.applyLoginResponse([]byte(body)); err != nil {
		t.Fatal(err)
	}
	s := insta.Session()
	if s.Authorization != "Bearer IGT:2:eyJkc191c2VyX2lkIjoiNDI4NjUifQ==" {
		t.Errorf("authorization = %q", s.Authorization)
	}
	if s.Rur != "CLN,42865,1800000000:01abcdef" {
		t.Errorf("rur = %q", s.Rur)
	}
	if !strings.HasPrefix(s.WWWClaim, "hmac.AR") {
		t.Errorf("www-claim = %q", s.WWWClaim)
	}
	if s.Mid != "abcdefMIDvalue" {
		t.Errorf("mid = %q", s.Mid)
	}
	if insta.Account.ID != 42865264907 {
		t.Errorf("ds-user-id = %d", insta.Account.ID)
	}
	if s.PasswordKeyID != "182" || s.PasswordPubKey != "LS0tLS1CRUdJTiBQVUJMSUMgS0VZ+/=" {
		t.Errorf("rotated password key not stored: %q %q", s.PasswordKeyID, s.PasswordPubKey)
	}
}

func TestApplyLoginResponseFailure(t *testing.T) {
	insta, _ := newReplay(t)
	// A checkpoint/2FA/bad-credential response carries no bearer token.
	err := insta.applyLoginResponse([]byte(`{"layout":{"bloks_payload":{"action":"(some.error two_factor_required)"}}}`))
	if err == nil {
		t.Fatal("expected login failure when no token present")
	}
}

// loginServer answers the three logged-out login calls and records each request.
type loginServer struct {
	t         *testing.T
	reqs      map[string]*http.Request
	form      map[string]url.Values
	twoFactor bool // send_login answers with the 2FA redirect
}

const testAAC = "Q7jiDQEFnnWYRW7pTESTaacTESTaacTESTaac_-0123456789"

func (s *loginServer) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, _ := io.ReadAll(req.Body)
	form, _ := url.ParseQuery(string(raw))
	var name, body string
	switch {
	case strings.HasSuffix(req.URL.Path, "/attestation/create_android_keystore/"):
		name, body = "keystore", `{"challenge_nonce":"NONCE____abc","key_nonce":"k","status":"ok"}`
	case strings.Contains(req.URL.Path, "process_client_data_and_redirect"):
		// Shape copied from a capture: aac is a Bloks global-state initial value.
		name, body = "pcd", `{"layout":{"bloks_payload":{"data":[{"id":"s1","type":"gs","data":{"key":"CAA_ACCOUNT_ACCESS_CONTEXT:aac","mode":"p","initial_lispy":"\t(fhy \"`+testAAC+`\")"}}]}},"status":"ok"}`
	case strings.Contains(req.URL.Path, "send_login_request") && s.twoFactor:
		name, body = "login", twoFactorLoginBody
	case strings.Contains(req.URL.Path, "send_login_request"):
		name, body = "login", buildBloksHeaders(map[string]string{
			"IG-Set-Authorization": "Bearer IGT:2:abc", "ig-set-ig-u-ds-user-id": "42865264907",
		})
	case strings.Contains(req.URL.Path, "method_picker.navigation.async"):
		name, body = "method_picker", `{"status":"ok"}`
	case strings.Contains(req.URL.Path, "verify_code.async"):
		name, body = "verify_code", buildBloksHeaders(map[string]string{
			"IG-Set-Authorization": "Bearer IGT:2:tok2fa", "ig-set-ig-u-ds-user-id": "42865264907",
		})
	default:
		s.t.Fatalf("unexpected request %s", req.URL)
	}
	s.reqs[name], s.form[name] = req, form
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func newLoggedOut(t *testing.T) (*Instagram, *loginServer) {
	t.Helper()
	insta, err := New(Session{
		AndroidID: "android-0000000000000000", DeviceID: "00000000-0000-0000-0000-00000000dev0",
		FamilyDeviceID: "00000000-0000-0000-0000-00000000fam0", Mid: "mid",
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := &loginServer{t: t, reqs: map[string]*http.Request{}, form: map[string]url.Values{}}
	insta.SetHTTPClient(&http.Client{Transport: srv})
	return insta, srv
}

func zcaOf(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(req.Header.Get("X-Meta-Zca"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Android struct{ Aka map[string]any } `json:"android"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Android.Aka
}

func setTestPasswordKey(t *testing.T, insta *Instagram) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	spki, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	insta.pwPubKey = base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: spki}))
	insta.pwKeyID = "181"
}

func TestLoginFlow(t *testing.T) {
	insta, srv := newLoggedOut(t)
	att, _, _ := newTestAttestation(t)
	if err := insta.SetAttestation(att); err != nil {
		t.Fatal(err)
	}
	setTestPasswordKey(t, insta)

	if err := insta.Login(t.Context(), "bobby707880", "pw"); err != nil {
		t.Fatal(err)
	}

	// The keystore call announces the fresh, never-uploaded zca key it signs with.
	ks := srv.reqs["keystore"]
	aka := zcaOf(t, ks)
	if aka["keyHash"] != srv.form["keystore"].Get("key_hash") || aka["keyHash"] == att.keyHash {
		t.Errorf("keystore key_hash %q, zca keyHash %v, registered %q", srv.form["keystore"].Get("key_hash"), aka["keyHash"], att.keyHash)
	}
	if aka["lastUploadedKeyTimeMs"] != float64(0) {
		t.Errorf("logged-out lastUploadedKeyTimeMs = %v, want 0", aka["lastUploadedKeyTimeMs"])
	}
	if got := srv.form["keystore"].Get("app_scoped_device_id"); got != insta.uuid {
		t.Errorf("app_scoped_device_id = %q", got)
	}

	login := srv.reqs["login"]
	var params struct {
		Client map[string]any `json:"client_input_params"`
		Server map[string]any `json:"server_params"`
	}
	if err := json.Unmarshal([]byte(srv.form["login"].Get("params")), &params); err != nil {
		t.Fatal(err)
	}
	if params.Client["aac"] != testAAC {
		t.Errorf("aac = %v", params.Client["aac"])
	}
	var pcd map[string]any
	_ = json.Unmarshal([]byte(srv.form["pcd"].Get("params")), &pcd)
	if params.Server["waterfall_id"] != pcd["waterfall_id"] {
		t.Errorf("waterfall differs: %v vs %v", params.Server["waterfall_id"], pcd["waterfall_id"])
	}
	if !strings.Contains(login.Header.Get("X-Ig-Attest-Params"), `"challenge_nonce":"NONCE____abc"`) {
		t.Errorf("attest params = %s", login.Header.Get("X-Ig-Attest-Params"))
	}
	if zcaOf(t, login)["keyHash"] != aka["keyHash"] {
		t.Error("login zca must use the same logged-out key as the keystore call")
	}
	for _, r := range srv.reqs {
		if r.Header.Get("Ig-U-Ds-User-Id") != "" || r.Header.Get("Ig-Intended-User-Id") != "0" || r.Header.Get("Authorization") != "" {
			t.Errorf("%s: logged-out user headers wrong: %v", r.URL.Path, r.Header)
		}
		if r.Header.Get("X-Fb-Rmd") != "" || r.Header.Get("X-Fb-Session-Id") == "" || r.Header.Get("X-Meta-Tasos-Congestion-Config") == "" {
			t.Errorf("%s: network-stack headers wrong: %v", r.URL.Path, r.Header)
		}
		if r.Header.Get("X-Ig-Www-Claim") != "0" || r.Header.Get("X-Zero-Balance") != "" {
			t.Errorf("%s: not the logged-out profile", r.URL.Path)
		}
	}

	if insta.Session().Authorization != "Bearer IGT:2:abc" || insta.Account.ID != 42865264907 {
		t.Errorf("session not applied: %+v", insta.Session())
	}
	// Afterwards the registered attestation is back in place.
	if v, _ := insta.headerOptions.Load("X-Meta-Zca"); !strings.Contains(decodeB64(t, v.(string)), att.keyHash) {
		t.Error("registered zca not restored after login")
	}
}

func TestLoginNeedsPasswordKey(t *testing.T) {
	insta, _ := newLoggedOut(t)
	if err := insta.Login(t.Context(), "bobby707880", "pw"); err != ErrPasswordKeyUnknown {
		t.Errorf("err = %v, want ErrPasswordKeyUnknown", err)
	}
}

func decodeB64(t *testing.T, s string) string {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPasswordKeyCapturedFromHeaders(t *testing.T) {
	insta, _ := newReplay(t)
	h := http.Header{}
	h.Set("Ig-Set-Password-Encryption-Pub-Key", "cGVt")
	h.Set("Ig-Set-Password-Encryption-Key-Id", "181")
	insta.extractHeaders(h)
	if s := insta.Session(); s.PasswordPubKey != "cGVt" || s.PasswordKeyID != "181" {
		t.Errorf("password key not captured: %q %q", s.PasswordPubKey, s.PasswordKeyID)
	}
}

func TestReloginUsesLoggedOutProfile(t *testing.T) {
	insta, err := New(Session{
		UserID: 42865264907, Authorization: "Bearer IGT:2:old", Rur: "rur", WWWClaim: "claim",
		AndroidID: "android-0000000000000000", DeviceID: "00000000-0000-0000-0000-00000000dev0",
		FamilyDeviceID: "00000000-0000-0000-0000-00000000fam0", Mid: "mid", ZCA: "zca", USDID: "usdid",
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := &loginServer{t: t, reqs: map[string]*http.Request{}, form: map[string]url.Values{}}
	insta.SetHTTPClient(&http.Client{Transport: srv})
	setTestPasswordKey(t, insta)

	if err := insta.Login(t.Context(), "bobby707880", "pw"); err != nil {
		t.Fatal(err)
	}
	for name, r := range srv.reqs {
		for _, h := range []string{"Authorization", "Ig-U-Ds-User-Id", "Ig-U-Rur", "X-Meta-Usdid"} {
			if v := r.Header.Get(h); v != "" {
				t.Errorf("%s sent %s: %q", name, h, v)
			}
		}
		if r.Header.Get("Ig-Intended-User-Id") != "0" || r.Header.Get("X-Mid") != "mid" {
			t.Errorf("%s: intended id %q, mid %q", name, r.Header.Get("Ig-Intended-User-Id"), r.Header.Get("X-Mid"))
		}
	}
	if s := insta.Session(); s.ZCA != "zca" || s.USDID != "usdid" || s.Authorization != "Bearer IGT:2:abc" {
		t.Errorf("after login: zca %q usdid %q auth %q", s.ZCA, s.USDID, s.Authorization)
	}
	insta.mu.Lock()
	cur := insta.curAtt
	insta.mu.Unlock()
	if cur != nil {
		t.Error("throwaway login key still signing")
	}
}

// twoFactorLoginBody has the shape of a captured send_login 2FA response: the
// next screen's params as a Bloks key list then value list.
const twoFactorLoginBody = `{"layout":{"bloks_payload":{"action":"(fgu (f6m 6 \"error_info\" \"redirect_two_fac\")) (f6m 6 \"params\" (fma (f4i (dkc \"server_params\" \"client_input_params\") (dkc (f4i (dkc \"flow_source\" \"two_step_verification_context\" \"device_id\" \"should_fallback_to_sms\" \"INTERNAL_INFRA_screen_id\") (dkc \"two_factor_login\" \"AWR9_ctx-abcdefghijklmnop\" \"android-0000000000000000\" false \"ap666c:3\")) (f4i (dkc \"machine_id\" \"device_id\") (dkc (drc) (dvy))))))"}},"status":"ok"}`

func TestParseTwoFactorContext(t *testing.T) {
	ctx, ok := parseTwoFactorContext([]byte(twoFactorLoginBody))
	if !ok || ctx != "AWR9_ctx-abcdefghijklmnop" {
		t.Fatalf("ctx=%q ok=%v", ctx, ok)
	}
	if _, ok := parseTwoFactorContext([]byte(`{"no":"context here"}`)); ok {
		t.Error("matched a body with no context")
	}
}

func TestLoginTwoFactor(t *testing.T) {
	insta, err := New(Session{
		UserID: 42865264907, Authorization: "Bearer IGT:2:old", AndroidID: "android-0000000000000000",
		DeviceID: "00000000-0000-0000-0000-00000000dev0", FamilyDeviceID: "00000000-0000-0000-0000-00000000fam0",
		Mid: "mid", ZCA: "zca", USDID: "usdid",
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := &loginServer{t: t, reqs: map[string]*http.Request{}, form: map[string]url.Values{}, twoFactor: true}
	insta.SetHTTPClient(&http.Client{Transport: srv})
	setTestPasswordKey(t, insta)
	var saved []Session
	insta.OnSessionChange = func(s Session) { saved = append(saved, s) }

	if err := insta.Login(t.Context(), "bobby707880", "pw"); !errors.Is(err, ErrTwoFactorRequired) {
		t.Fatalf("Login = %v, want ErrTwoFactorRequired", err)
	}
	if insta.header("X-Meta-Zca") == "zca" {
		t.Error("registered attestation restored before the code step")
	}
	// A second Login while 2FA is pending starts over from the registered key.
	if err := insta.Login(t.Context(), "bobby707880", "pw"); !errors.Is(err, ErrTwoFactorRequired) {
		t.Fatalf("second Login = %v", err)
	}
	if err := insta.LoginTwoFactor(t.Context(), "  170664 "); err != nil {
		t.Fatal(err)
	}

	params := func(name string) (client, server map[string]any) {
		var p map[string]map[string]any
		if err := json.Unmarshal([]byte(srv.form[name].Get("params")), &p); err != nil {
			t.Fatalf("%s params: %v", name, err)
		}
		return p["client_input_params"], p["server_params"]
	}
	mc, ms := params("method_picker")
	if mc["selected_method"] != "totp" || ms["two_step_verification_context"] != "AWR9_ctx-abcdefghijklmnop" {
		t.Errorf("method_picker: %v %v", mc, ms)
	}
	vc, vs := params("verify_code")
	if vc["code"] != "170664" || vc["machine_id"] != "mid" || vc["device_id"] != "android-0000000000000000" {
		t.Errorf("verify_code client params: %v", vc)
	}
	if vs["challenge"] != "totp" || vs["flow_source"] != "two_factor_login" || vs["two_step_verification_context"] != "AWR9_ctx-abcdefghijklmnop" {
		t.Errorf("verify_code server params: %v", vs)
	}
	if got := srv.reqs["verify_code"].Header.Get("X-Ig-Client-Endpoint"); !strings.HasSuffix(got, "two_factor_login.enter_totp_code") {
		t.Errorf("verify_code endpoint %q", got)
	}

	s := insta.Session()
	if s.Authorization != "Bearer IGT:2:tok2fa" || s.ZCA != "zca" || s.USDID != "usdid" {
		t.Errorf("after 2FA: auth %q zca %q usdid %q", s.Authorization, s.ZCA, s.USDID)
	}
	if insta.tfa != nil {
		t.Error("pending 2FA state not cleared")
	}
	if len(saved) == 0 || saved[len(saved)-1].Authorization != "Bearer IGT:2:tok2fa" || saved[len(saved)-1].ZCA != "zca" {
		t.Errorf("OnSessionChange did not see the final session: %d calls", len(saved))
	}
}

func TestLoginTwoFactorRejectedCode(t *testing.T) {
	insta, _ := newReplay(t)
	insta.tfa = &twoFactorState{context: "CTX"}
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(*http.Request) (int, string) {
		return 200, `{"layout":{"bloks_payload":{"action":"(dq8 \"error\")"}},"status":"ok"}`
	})})
	if err := insta.LoginTwoFactor(t.Context(), "000000"); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Errorf("err = %v", err)
	}
	if insta.header("Authorization") != "Bearer IGT:2:test" {
		t.Error("bearer changed on a rejected code")
	}
}

func TestLoginTwoFactorNoPending(t *testing.T) {
	insta, _ := newReplay(t)
	if err := insta.LoginTwoFactor(t.Context(), "123456"); err == nil {
		t.Error("expected error with no pending 2FA login")
	}
}
