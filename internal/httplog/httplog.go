// Package httplog parses the plain-text HTTP capture format produced by the
// device proxy (log_http.txt) into structured entries.
//
// Format, per exchange:
//
//	===== 2026-10-27 19:00:25.096 POST https://host/path?q =====
//	> request-header: value
//	--- request body ---          (optional, raw bytes)
//	--- response 200 OK ---
//	< response-header: value
//	--- response body ---         (optional, decompressed)
package httplog

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Entry struct {
	Time        string            `json:"time"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Host        string            `json:"host"`
	Path        string            `json:"path"`
	Query       map[string]string `json:"query,omitempty"`
	ReqHeaders  map[string]string `json:"req_headers"`
	ReqBody     string            `json:"req_body,omitempty"`
	ReqForm     map[string]string `json:"req_form,omitempty"`
	ReqSigned   map[string]any    `json:"req_signed,omitempty"`
	Status      int               `json:"status"`
	RespHeaders map[string]string `json:"resp_headers,omitempty"`
	RespBody    string            `json:"resp_body,omitempty"`
}

var startRe = regexp.MustCompile(`^===== (\S+ \S+) (\S+) (\S+) =====$`)

const (
	markReqBody  = "--- request body ---"
	markResp     = "--- response "
	markRespBody = "--- response body ---"
)

func Parse(r io.Reader) ([]Entry, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, block := range splitBlocks(data) {
		e, err := parseBlock(block)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func splitBlocks(data []byte) [][]byte {
	sep := []byte("\n===== ")
	var blocks [][]byte
	if !bytes.HasPrefix(data, []byte("===== ")) {
		i := bytes.Index(data, sep)
		if i < 0 {
			return nil
		}
		data = data[i+1:]
	}
	for len(data) > 0 {
		i := bytes.Index(data[1:], sep)
		if i < 0 {
			blocks = append(blocks, data)
			break
		}
		blocks = append(blocks, data[:i+1])
		data = data[i+2:]
	}
	return blocks
}

func parseBlock(b []byte) (Entry, error) {
	var e Entry
	nl := bytes.IndexByte(b, '\n')
	if nl < 0 {
		nl = len(b)
	}
	m := startRe.FindStringSubmatch(string(b[:nl]))
	if m == nil {
		return e, fmt.Errorf("httplog: bad start line %q", b[:nl])
	}
	e.Time, e.Method, e.URL = m[1], m[2], m[3]
	if u, err := url.Parse(e.URL); err == nil {
		e.Host, e.Path = u.Host, u.Path
		e.Query = flatten(u.Query())
	}
	rest := b[min(nl+1, len(b)):]

	reqPart, respPart := rest, []byte(nil)
	if i := indexLine(rest, markResp); i >= 0 {
		reqPart, respPart = rest[:i], rest[i:]
	}

	hdr, body, hasBody := cutLine(reqPart, markReqBody)
	e.ReqHeaders = headers(hdr, "> ")
	if hasBody {
		e.ReqBody = string(bytes.TrimSuffix(body, []byte("\n")))
		e.decodeForm()
	}

	if respPart != nil {
		nl := bytes.IndexByte(respPart, '\n')
		if nl < 0 {
			nl = len(respPart)
		}
		status := strings.Fields(strings.TrimPrefix(string(respPart[:nl]), markResp))
		if len(status) > 0 {
			e.Status, _ = strconv.Atoi(status[0])
		}
		hdr, body, hasBody := cutLine(respPart[min(nl+1, len(respPart)):], markRespBody)
		e.RespHeaders = headers(hdr, "< ")
		if hasBody {
			e.RespBody = strings.TrimRight(string(body), "\n")
		}
	}
	return e, nil
}

func (e *Entry) decodeForm() {
	ct := e.ReqHeaders["content-type"]
	if !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		return
	}
	vals, err := url.ParseQuery(e.ReqBody)
	if err != nil {
		return
	}
	e.ReqForm = flatten(vals)
	if sb, ok := e.ReqForm["signed_body"]; ok {
		if _, payload, ok := strings.Cut(sb, "."); ok {
			var m map[string]any
			if json.Unmarshal([]byte(payload), &m) == nil {
				e.ReqSigned = m
				delete(e.ReqForm, "signed_body")
			}
		}
	}
}

// LastJSON: streamed NDJSON responses repeat the object per line, each superseding the previous.
func (e *Entry) LastJSON() []byte {
	body := strings.TrimSpace(e.RespBody)
	if i := strings.LastIndexByte(body, '\n'); i >= 0 {
		return []byte(body[i+1:])
	}
	return []byte(body)
}

func indexLine(b []byte, prefix string) int {
	if bytes.HasPrefix(b, []byte(prefix)) {
		return 0
	}
	i := bytes.Index(b, []byte("\n"+prefix))
	if i < 0 {
		return -1
	}
	return i + 1
}

func cutLine(b []byte, marker string) (before, after []byte, found bool) {
	i := indexLine(b, marker)
	if i < 0 {
		return b, nil, false
	}
	after = b[i+len(marker):]
	after = bytes.TrimPrefix(after, []byte("\n"))
	return b[:i], after, true
}

func headers(b []byte, prefix string) map[string]string {
	h := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		line, ok := strings.CutPrefix(sc.Text(), prefix)
		if !ok {
			continue
		}
		k, v, _ := strings.Cut(line, ":")
		h[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return h
}

func flatten(v url.Values) map[string]string {
	if len(v) == 0 {
		return nil
	}
	m := make(map[string]string, len(v))
	for k, vs := range v {
		m[k] = strings.Join(vs, ",")
	}
	return m
}

type Session struct {
	UserID         int64  `json:"user_id"`
	Authorization  string `json:"authorization"`
	UserAgent      string `json:"user_agent"`
	AndroidID      string `json:"android_id"`
	DeviceID       string `json:"device_id"`
	FamilyDeviceID string `json:"family_device_id"`
	Mid            string `json:"mid"`
	WWWClaim       string `json:"www_claim"`
	Rur            string `json:"rur"`
	ZCA            string `json:"zca,omitempty"`
	USDID          string `json:"usdid,omitempty"`
	PigeonSession  string `json:"pigeon_session_id,omitempty"`
	GoogleAdID     string `json:"google_ad_id,omitempty"`
	BloksVersionID string `json:"bloks_version_id"`
	AppID          string `json:"app_id"`
	Capabilities   string `json:"capabilities"`
	TimezoneOffset string `json:"timezone_offset"`
	TimezoneName   string `json:"timezone_name,omitempty"`
	Locale         string `json:"locale"`

	PasswordPubKey string `json:"password_pub_key,omitempty"`
	PasswordKeyID  string `json:"password_key_id,omitempty"`
}

func ExtractSession(entries []Entry) (Session, error) {
	var s Session
	set := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Host, "instagram.com") {
			continue
		}
		h := e.ReqHeaders
		set(&s.Authorization, h["authorization"])
		set(&s.UserAgent, h["user-agent"])
		set(&s.AndroidID, h["x-ig-android-id"])
		set(&s.DeviceID, h["x-ig-device-id"])
		set(&s.FamilyDeviceID, h["x-ig-family-device-id"])
		set(&s.Mid, h["x-mid"])
		if c := h["x-ig-www-claim"]; c != "" && c != "0" {
			s.WWWClaim = c
		}
		set(&s.Rur, h["ig-u-rur"])
		set(&s.Rur, e.RespHeaders["ig-set-ig-u-rur"])
		set(&s.ZCA, h["x-meta-zca"])
		set(&s.USDID, h["x-meta-usdid"])
		set(&s.PigeonSession, h["x-pigeon-session-id"])
		set(&s.GoogleAdID, h["x-google-ad-id"])
		set(&s.TimezoneName, e.ReqForm["device_timezone_name"])
		set(&s.BloksVersionID, h["x-bloks-version-id"])
		set(&s.AppID, h["x-ig-app-id"])
		set(&s.Capabilities, h["x-ig-capabilities"])
		set(&s.TimezoneOffset, h["x-ig-timezone-offset"])
		set(&s.Locale, h["x-ig-app-locale"])
		if id, err := strconv.ParseInt(h["ig-u-ds-user-id"], 10, 64); err == nil && id > 0 {
			s.UserID = id
		}
	}
	if s.Authorization == "" {
		return s, fmt.Errorf("httplog: no authorization header found")
	}
	if s.UserID == 0 {
		s.UserID = userIDFromBearer(s.Authorization)
	}
	return s, nil
}

type bearerPayload struct {
	UserID    string `json:"ds_user_id"`
	SessionID string `json:"sessionid"`
}

// Bearer format: "Bearer IGT:2:" + base64(JSON{ds_user_id, sessionid}).
func decodeBearer(auth string) (bearerPayload, bool) {
	var p bearerPayload
	parts := strings.Split(strings.TrimPrefix(auth, "Bearer "), ":")
	if len(parts) != 3 {
		return p, false
	}
	raw, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return p, false
	}
	return p, json.Unmarshal(raw, &p) == nil
}

func userIDFromBearer(auth string) int64 {
	p, _ := decodeBearer(auth)
	id, _ := strconv.ParseInt(p.UserID, 10, 64)
	return id
}

func sessionIDFromBearer(auth string) string {
	p, _ := decodeBearer(auth)
	return p.SessionID
}

var sensitiveHeaders = []string{
	"authorization", "x-meta-zca", "x-meta-usdid", "x-mid", "x-ig-www-claim",
	"ig-u-rur", "ig-u-shbid", "ig-u-shbts", "cookie", "set-cookie",
	"ig-set-ig-u-rur", "ig-set-ig-u-shbid", "ig-set-ig-u-shbts", "x-ig-set-www-claim",
	"ig-set-authorization", "x-fb-client-ip-forwarded",
}

func Scrub(e Entry, s Session) Entry {
	repl := []string{}
	add := func(v, with string) {
		if len(v) >= 6 {
			repl = append(repl, v, with)
		}
	}
	add(s.AndroidID, "android-0000000000000000")
	add(s.DeviceID, "00000000-0000-0000-0000-00000000dev0")
	add(s.FamilyDeviceID, "00000000-0000-0000-0000-00000000fam0")
	token := strings.TrimPrefix(s.Authorization, "Bearer ")
	add(token, "REDACTED")
	if sid := sessionIDFromBearer(s.Authorization); sid != "" {
		add(sid, "REDACTED")
		if dec, err := url.QueryUnescape(sid); err == nil {
			add(dec, "REDACTED")
		}
	}
	for _, v := range []string{s.Mid, s.WWWClaim, s.Rur, s.ZCA, s.USDID, s.GoogleAdID} {
		add(v, "REDACTED")
	}
	r := strings.NewReplacer(repl...)

	scrubMap := func(m map[string]string) map[string]string {
		if m == nil {
			return nil
		}
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[k] = r.Replace(v)
		}
		for _, k := range sensitiveHeaders {
			if _, ok := out[k]; ok {
				out[k] = "REDACTED"
			}
		}
		return out
	}
	e.URL = r.Replace(e.URL)
	e.Query = scrubMap(e.Query)
	e.ReqHeaders = scrubMap(e.ReqHeaders)
	e.RespHeaders = scrubMap(e.RespHeaders)
	e.ReqForm = scrubMap(e.ReqForm)
	e.ReqBody = r.Replace(e.ReqBody)
	e.RespBody = r.Replace(e.RespBody)
	if e.ReqSigned != nil {
		b, _ := json.Marshal(e.ReqSigned)
		var m map[string]any
		_ = json.Unmarshal([]byte(r.Replace(string(b))), &m)
		e.ReqSigned = m
	}
	return e
}
