package goinsta

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

type reqOptions struct {
	Endpoint string

	OmitAPI bool

	UseV2 bool

	IsPost bool

	// The app gzips bodies for feed/timeline and v2 media/seen.
	Gzip bool

	// NDJSON response where each line supersedes the previous; the last is kept.
	Stream bool

	// NDJSON response whose lines are separate chunks; all are kept.
	Chunked bool

	// LoggedOut sends the logged-out profile: user id 0 and no account headers.
	LoggedOut bool

	Priority string

	FriendlyName string

	Query map[string]string

	DataBytes []byte

	IgnoreHeaders []string
	ExtraHeaders  map[string]string

	Ignore429 bool
}

var bloksHeaders = []string{
	"X-Bloks-Is-Layout-Rtl", "X-Bloks-Prism-Button-Version", "X-Bloks-Prism-Colors-Enabled",
	"X-Bloks-Prism-Extended-Palette-Gray", "X-Bloks-Prism-Extended-Palette-Indigo",
	"X-Bloks-Prism-Extended-Palette-Polish-Enabled", "X-Bloks-Prism-Extended-Palette-Red",
	"X-Bloks-Prism-Extended-Palette-Rest-Of-Colors", "X-Bloks-Prism-Font-Enabled",
	"X-Bloks-Prism-Indigo-Link-Version", "X-Ig-Device-Languages",
}

var loggedInHeaders = []string{
	"Authorization", "Ig-U-Ds-User-Id", "Ig-U-Rur", "Ig-U-Shbid", "Ig-U-Shbts", "Ig-U-Ig-Direct-Region-Hint",
	"X-Ig-Device-Languages", "X-Ig-Device-Timezone",
}

const maxBodySize = 64 << 20

var zstdDecoder, _ = zstd.NewReader(nil, zstd.WithDecoderConcurrency(0), zstd.WithDecoderMaxMemory(maxBodySize))

func (o *reqOptions) url() (*url.URL, error) {
	base := apiURL
	switch {
	case strings.HasPrefix(o.Endpoint, "https://"):
		base = ""
	case o.OmitAPI:
		base = baseURL
	case o.UseV2:
		base = apiV2URL
	}
	return url.Parse(base + o.Endpoint)
}

func (insta *Instagram) staticHeaders() http.Header {
	s := insta.sess
	lang := strings.ReplaceAll(s.Locale, "_", "-")
	h := http.Header{}
	for k, v := range map[string]string{
		"Accept-Encoding":                               "zstd",
		"Accept-Language":                               lang,
		"User-Agent":                                    s.UserAgent,
		"X-Bloks-Is-Layout-Rtl":                         "false",
		"X-Bloks-Prism-Button-Version":                  "INDIGO_PRIMARY_BORDERED_SECONDARY",
		"X-Bloks-Prism-Colors-Enabled":                  "true",
		"X-Bloks-Prism-Extended-Palette-Gray":           "true",
		"X-Bloks-Prism-Extended-Palette-Indigo":         "true",
		"X-Bloks-Prism-Extended-Palette-Polish-Enabled": "true",
		"X-Bloks-Prism-Extended-Palette-Red":            "true",
		"X-Bloks-Prism-Extended-Palette-Rest-Of-Colors": "true",
		"X-Bloks-Prism-Font-Enabled":                    "true",
		"X-Bloks-Prism-Indigo-Link-Version":             "1",
		"X-Bloks-Version-Id":                            s.BloksVersionID,
		"X-Fb-Client-Ip":                                "True",
		"X-Fb-Connection-Type":                          connType,
		"X-Fb-Http-Engine":                              "Tigon/MNS/TCP",
		"X-Fb-Rmd":                                      "state=URL_ELIGIBLE",
		"X-Fb-Server-Cluster":                           "True",
		"X-Ig-Android-Id":                               insta.dID,
		"X-Ig-App-Id":                                   s.AppID,
		"X-Ig-App-Locale":                               s.Locale,
		"X-Ig-Bloks-Serialize-Payload":                  "true",
		"X-Ig-Capabilities":                             s.Capabilities,
		"X-Ig-Client-Endpoint":                          "MainFeedFragment:feed_timeline",
		"X-Ig-Connection-Type":                          connType,
		"X-Ig-Device-Id":                                insta.uuid,
		"X-Ig-Device-Languages":                         `{"system_languages":"` + lang + `","keyboard_language":"` + lang + `"}`,
		"X-Ig-Device-Timezone":                          s.TimezoneName,
		"X-Ig-Device-Locale":                            s.Locale,
		"X-Ig-Family-Device-Id":                         insta.fID,
		"X-Ig-Is-Adaptive-Ui":                           "true",
		"X-Ig-Is-Foldable":                              "false",
		"X-Ig-Mapped-Locale":                            s.Locale,
		"X-Ig-Timezone-Offset":                          s.TimezoneOffset,
		"X-Pigeon-Session-Id":                           insta.psID,
		"X-Tigon-Is-Retry":                              "False",
		// Network-stack headers of the current build (absent from the older capture).
		"X-Fb-Appnetsession-Nid":         insta.net.nid,
		"X-Fb-Appnetsession-Sid":         insta.net.sid,
		"X-Fb-Network-Properties":        "Validated;",
		"X-Fb-Session-Id":                insta.net.sessionID,
		"X-Fb-Session-Private":           insta.net.sessionPrivate,
		"X-Fb-Tasos-Td-V2-Config":        "prod_android",
		"X-Meta-Tasos-Congestion-Config": "v6_1",
		"X-Meta-Tasos-Dcc-Constraint":    "active=0;dcc=0;mit=0;alw=0;level=none;mode=0",
	} {
		h.Set(k, v)
	}
	return h
}

func (insta *Instagram) newRequest(ctx context.Context, o *reqOptions) (*http.Request, error) {
	now := time.Now()
	if err := insta.refreshAttestation(now); err != nil {
		return nil, fmt.Errorf("re-sign attestation: %w", err)
	}
	u, err := o.url()
	if err != nil {
		return nil, err
	}

	var body io.Reader
	method := http.MethodGet
	if o.IsPost {
		method = http.MethodPost
		raw := o.DataBytes
		if raw == nil {
			raw = []byte(encodeQuery(o.Query))
		}
		if o.Gzip {
			if raw, err = gzipBytes(raw); err != nil {
				return nil, err
			}
		}
		body = bytes.NewReader(raw)
	} else if len(o.Query) > 0 {
		q := u.Query()
		for k, v := range o.Query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}

	priority := o.Priority
	if priority == "" {
		priority = "u=3"
	}
	friendly := o.FriendlyName
	if friendly == "" {
		friendly = "IgApi: " + o.Endpoint
	}
	category := "api"
	if priority == "u=0" {
		category = "critical_api"
	}
	uid := "0"
	if !o.LoggedOut {
		uid = strconv.FormatInt(insta.userID(), 10)
	}
	speed, totalBytes, totalMs := insta.bw.sample(now)

	h := insta.baseHeader.Clone()
	h.Set("Priority", priority)
	h.Set("X-Fb-Friendly-Name", friendly)
	h.Set("X-Fb-Request-Analytics-Tags", `{"network_tags":{"product":"`+insta.sess.AppID+`","surface":"undefined","request_category":"`+category+`","purpose":"fetch","retry_attempt":"0"}}`)
	h.Set("X-Fb-Conn-Uuid-Client", strings.ReplaceAll(generateUUID(), "-", ""))
	h.Set("X-Ig-Bandwidth-Speed-Kbps", strconv.Itoa(speed)+".000")
	h.Set("X-Ig-Bandwidth-Totalbytes-B", strconv.Itoa(totalBytes))
	h.Set("X-Ig-Bandwidth-Totaltime-Ms", strconv.Itoa(totalMs))
	h.Set("X-Pigeon-Rawclienttime", strconv.FormatFloat(float64(now.UnixMilli())/1000, 'f', 3, 64))
	h.Set("Ig-Intended-User-Id", uid)
	if uid != "0" {
		h.Set("Ig-U-Ds-User-Id", uid) // the logged-out app sends only the intended id
	}
	if o.IsPost && o.DataBytes == nil {
		h.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	}
	if o.Gzip {
		h.Set("Content-Encoding", "gzip")
	}
	if o.Stream || o.Chunked {
		h.Set("X-Ig-Transfer-Encoding", "chunked")
	}
	insta.headerOptions.Range(func(k, v any) bool {
		h.Set(k.(string), v.(string))
		return true
	})
	for _, k := range o.IgnoreHeaders {
		h.Del(k)
	}
	if o.LoggedOut {
		for _, k := range loggedInHeaders {
			h.Del(k)
		}
	}
	for k, v := range o.ExtraHeaders {
		h.Set(k, v)
	}
	req.Header = h
	return req, nil
}

func encodeQuery(q map[string]string) string {
	vs := make(url.Values, len(q))
	for k, v := range q {
		vs[k] = []string{v}
	}
	return vs.Encode()
}

func gzipBytes(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (insta *Instagram) call(ctx context.Context, o *reqOptions) ([]byte, error) {
	body, _, err := insta.sendRequest(ctx, o)
	return body, err
}

func (insta *Instagram) callJSON(ctx context.Context, o *reqOptions, v any) error {
	body, err := insta.call(ctx, o)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func (insta *Instagram) callLines(ctx context.Context, o *reqOptions, each func(line []byte) error) error {
	o.Chunked = true
	body, err := insta.call(ctx, o)
	if err != nil {
		return err
	}
	for line := range bytes.Lines(body) {
		if line = bytes.TrimSpace(line); len(line) == 0 {
			continue
		}
		if err := each(line); err != nil {
			return err
		}
	}
	return nil
}

func (insta *Instagram) sendRequest(ctx context.Context, o *reqOptions) (body []byte, h http.Header, err error) {
	for attempt := 0; ; attempt++ {
		body, h, err = insta.sendOnce(ctx, o)
		if !errors.Is(err, ErrTooManyRequests) || o.Ignore429 || insta.RateLimitWait <= 0 || attempt == 3 {
			return body, h, err
		}
		insta.Logger.WarnContext(ctx, "rate limited, waiting", "endpoint", o.Endpoint, "wait", insta.RateLimitWait)
		t := time.NewTimer(insta.RateLimitWait)
		select {
		case <-ctx.Done():
			t.Stop()
			return body, h, err
		case <-t.C:
		}
	}
}

func (insta *Instagram) sendOnce(ctx context.Context, o *reqOptions) ([]byte, http.Header, error) {
	req, err := insta.newRequest(ctx, o)
	if err != nil {
		return nil, nil, err
	}
	resp, err := insta.client().Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	insta.extractHeaders(resp.Header)
	body, err := readBody(resp)
	if err != nil {
		return nil, nil, err
	}
	if o.Stream {
		body = lastNDJSONLine(body)
	}
	err = insta.isError(resp.StatusCode, body, resp.Status, o.Endpoint)
	insta.Logger.DebugContext(ctx, "response", "status", resp.StatusCode, "method", req.Method, "endpoint", o.Endpoint, "body", body)
	return body, resp.Header, err
}

var errBodyTooLarge = fmt.Errorf("response body over %d bytes", maxBodySize)

func readAllLimited(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxBodySize+1))
	if err == nil && len(b) > maxBodySize {
		return nil, errBodyTooLarge
	}
	return b, err
}

func readBody(resp *http.Response) ([]byte, error) {
	switch resp.Header.Get("Content-Encoding") {
	case "zstd":
		raw, err := readAllLimited(resp.Body)
		if err != nil {
			return nil, err
		}
		return zstdDecoder.DecodeAll(raw, nil)
	case "gzip":
		zr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return readAllLimited(zr)
	default:
		return readAllLimited(resp.Body)
	}
}

func lastNDJSONLine(b []byte) []byte {
	b = bytes.TrimSpace(b)
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		return b[i+1:]
	}
	return b
}

func (insta *Instagram) extractHeaders(h http.Header) {
	changed := false
	extract := func(in, out string) {
		v := h.Get(in)
		if v == "" {
			return
		}
		if old, ok := insta.headerOptions.Load(out); !ok || old.(string) != v {
			insta.headerOptions.Store(out, v)
			changed = true
		}
	}
	extract("Ig-Set-Authorization", "Authorization")
	extract("Ig-Set-X-Mid", "X-Mid")
	extract("X-Ig-Set-Www-Claim", "X-Ig-Www-Claim")
	extract("Ig-Set-Ig-U-Ig-Direct-Region-Hint", "Ig-U-Ig-Direct-Region-Hint")
	extract("Ig-Set-Ig-U-Shbid", "Ig-U-Shbid")
	extract("Ig-Set-Ig-U-Shbts", "Ig-U-Shbts")
	extract("Ig-Set-Ig-U-Rur", "Ig-U-Rur")

	if pk := h.Get("Ig-Set-Password-Encryption-Pub-Key"); pk != "" {
		kid := h.Get("Ig-Set-Password-Encryption-Key-Id")
		insta.mu.Lock()
		if insta.pwPubKey != pk || insta.pwKeyID != kid {
			insta.pwPubKey, insta.pwKeyID = pk, kid
			changed = true
		}
		insta.mu.Unlock()
	}

	if changed {
		insta.notifySessionChange()
	}
}

func (insta *Instagram) notifySessionChange() {
	if insta.OnSessionChange != nil {
		insta.OnSessionChange(insta.Session())
	}
}

func (insta *Instagram) isError(code int, body []byte, status, endpoint string) error {
	switch code {
	case 200, 202, 204:
		return nil
	case 429:
		return ErrTooManyRequests
	}
	e := &APIError{StatusCode: code, Endpoint: endpoint}
	if json.Unmarshal(body, e) != nil {
		e.Status, e.Message = status, string(snippet(body))
		return e
	}
	switch e.kind() {
	case "login_required":
		if e.ErrorTitle == "You've Been Logged Out" {
			return ErrLoggedOut
		}
		return ErrLoginRequired
	case "Sorry, this media has been deleted":
		return ErrMediaDeleted
	case "checkpoint_required":
		return ErrCheckpointRequired
	case "checkpoint_challenge_required", "challenge_required":
		return ErrChallengeRequired
	}
	return e
}

// emptyArray, emptyObject and jsonNull marshal to the [], {} and null the app
// sends for fields it leaves unset.
type (
	emptyArray  struct{}
	emptyObject struct{}
	jsonNull    struct{}
)

func (emptyArray) MarshalJSON() ([]byte, error)  { return []byte("[]"), nil }
func (emptyObject) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }
func (jsonNull) MarshalJSON() ([]byte, error)    { return []byte("null"), nil }
