package goinsta

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
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

	Priority string

	FriendlyName string

	Query map[string]string

	DataBytes *bytes.Buffer

	IgnoreHeaders []string
	ExtraHeaders  map[string]string

	WrapperCount int

	Ignore429 bool
}

var bloksHeaders = []string{
	"X-Bloks-Is-Layout-Rtl", "X-Bloks-Prism-Button-Version", "X-Bloks-Prism-Colors-Enabled",
	"X-Bloks-Prism-Extended-Palette-Gray", "X-Bloks-Prism-Extended-Palette-Indigo",
	"X-Bloks-Prism-Extended-Palette-Polish-Enabled", "X-Bloks-Prism-Extended-Palette-Red",
	"X-Bloks-Prism-Extended-Palette-Rest-Of-Colors", "X-Bloks-Prism-Font-Enabled",
	"X-Bloks-Prism-Indigo-Link-Version",
}

var zstdDecoder, _ = zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))

func (insta *Instagram) sendSimpleRequest(uri string, a ...interface{}) ([]byte, error) {
	body, _, err := insta.sendRequest(&reqOptions{Endpoint: fmt.Sprintf(uri, a...)})
	return body, err
}

func (o *reqOptions) url() (*url.URL, error) {
	base := instaAPIUrl
	switch {
	case strings.HasPrefix(o.Endpoint, "https://"):
		base = ""
	case o.OmitAPI:
		base = baseUrl
	case o.UseV2:
		base = instaAPIUrlv2
	}
	return url.Parse(base + o.Endpoint)
}

func (insta *Instagram) newRequest(o *reqOptions) (*http.Request, error) {
	if err := insta.refreshAttestation(time.Now()); err != nil {
		return nil, fmt.Errorf("re-sign attestation: %w", err)
	}
	u, err := o.url()
	if err != nil {
		return nil, err
	}

	vs := url.Values{}
	for k, v := range o.Query {
		vs.Set(k, v)
	}

	var body io.Reader
	var contentEncoding string
	method := http.MethodGet
	if o.IsPost {
		method = http.MethodPost
		raw := []byte(vs.Encode())
		if o.DataBytes != nil {
			raw = o.DataBytes.Bytes()
		}
		if o.Gzip {
			var buf bytes.Buffer
			zw := gzip.NewWriter(&buf)
			if _, err := zw.Write(raw); err != nil {
				return nil, err
			}
			if err := zw.Close(); err != nil {
				return nil, err
			}
			raw = buf.Bytes()
			contentEncoding = "gzip"
		}
		body = bytes.NewReader(raw)
	} else if len(vs) > 0 {
		q := u.Query()
		for k, v := range vs {
			q[k] = v
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequest(method, u.String(), body)
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
	uid := strconv.FormatInt(insta.userID(), 10)
	s := insta.sess

	headers := map[string]string{
		"Accept-Encoding":                               "zstd",
		"Accept-Language":                               strings.ReplaceAll(s.Locale, "_", "-"),
		"Priority":                                      priority,
		"User-Agent":                                    insta.userAgent,
		"Ig-Intended-User-Id":                           uid,
		"Ig-U-Ds-User-Id":                               uid,
		"X-Bloks-Is-Layout-Rtl":                         "false",
		"X-Bloks-Prism-Button-Version":                  "INDIGO_PRIMARY_BORDERED_SECONDARY",
		"X-Bloks-Prism-Colors-Enabled":                  "true",
		"X-Bloks-Prism-Extended-Palette-Gray":           "false",
		"X-Bloks-Prism-Extended-Palette-Indigo":         "true",
		"X-Bloks-Prism-Extended-Palette-Polish-Enabled": "false",
		"X-Bloks-Prism-Extended-Palette-Red":            "true",
		"X-Bloks-Prism-Extended-Palette-Rest-Of-Colors": "true",
		"X-Bloks-Prism-Font-Enabled":                    "true",
		"X-Bloks-Prism-Indigo-Link-Version":             "1",
		"X-Bloks-Version-Id":                            s.BloksVersionID,
		"X-Fb-Client-Ip":                                "True",
		"X-Fb-Connection-Type":                          connType,
		"X-Fb-Friendly-Name":                            friendly,
		"X-Fb-Http-Engine":                              "Tigon/MNS/TCP",
		"X-Fb-Request-Analytics-Tags":                   `{"network_tags":{"product":"` + s.AppID + `","surface":"undefined","request_category":"` + category + `","purpose":"fetch","retry_attempt":"0"}}`,
		"X-Fb-Rmd":                                      "fail=Server:INVALID_MAP,Default:INVALID_MAP;v=;ip=;tkn=;reqTime=0;recvTime=0",
		"X-Fb-Server-Cluster":                           "True",
		"X-Fb-Conn-Uuid-Client":                         strings.ReplaceAll(generateUUID(), "-", ""),
		"X-Ig-Android-Id":                               insta.dID,
		"X-Ig-App-Id":                                   s.AppID,
		"X-Ig-App-Locale":                               s.Locale,
		"X-Ig-Bandwidth-Speed-Kbps":                     fmt.Sprintf("%d.000", rand.IntN(8000)+2000),
		"X-Ig-Bandwidth-Totalbytes-B":                   strconv.Itoa(rand.IntN(4_000_000) + 1_000_000),
		"X-Ig-Bandwidth-Totaltime-Ms":                   strconv.Itoa(rand.IntN(600) + 200),
		"X-Ig-Bloks-Serialize-Payload":                  "true",
		"X-Ig-Capabilities":                             s.Capabilities,
		"X-Ig-Client-Endpoint":                          "MainFeedFragment:feed_timeline",
		"X-Ig-Connection-Type":                          connType,
		"X-Ig-Device-Id":                                insta.uuid,
		"X-Ig-Device-Locale":                            s.Locale,
		"X-Ig-Family-Device-Id":                         insta.fID,
		"X-Ig-Is-Adaptive-Ui":                           "true",
		"X-Ig-Is-Foldable":                              "false",
		"X-Ig-Mapped-Locale":                            s.Locale,
		"X-Ig-Timezone-Offset":                          s.TimezoneOffset,
		"X-Pigeon-Rawclienttime":                        fmt.Sprintf("%.3f", float64(time.Now().UnixMilli())/1000),
		"X-Pigeon-Session-Id":                           insta.psID,
		"X-Tigon-Is-Retry":                              "False",
		"X-Zero-A-Device-Id":                            "",
		"X-Zero-Balance":                                "INIT",
		"X-Zero-D-Device-Id":                            insta.uuid,
		"X-Zero-Eh":                                     "",
		"X-Zero-F-Device-Id":                            insta.fID,
		"Zero-Http-Network-Interface":                   "ethernet",
	}
	if uid == "0" {
		delete(headers, "Ig-U-Ds-User-Id") // the logged-out app sends only the intended id
	}
	if o.IsPost && o.DataBytes == nil {
		headers["Content-Type"] = "application/x-www-form-urlencoded; charset=UTF-8"
	}
	if contentEncoding != "" {
		headers["Content-Encoding"] = contentEncoding
	}
	if o.Stream {
		headers["X-Ig-Transfer-Encoding"] = "chunked"
	}

	ignored := func(k string) bool {
		for _, h := range o.IgnoreHeaders {
			if strings.EqualFold(h, k) {
				return true
			}
		}
		return false
	}
	for k, v := range headers {
		if !ignored(k) {
			req.Header.Set(k, v)
		}
	}
	insta.headerOptions.Range(func(k, v any) bool {
		if !ignored(k.(string)) {
			req.Header.Set(k.(string), v.(string))
		}
		return true
	})
	for k, v := range o.ExtraHeaders {
		req.Header.Set(k, v)
	}
	return req, nil
}

func (insta *Instagram) sendRequest(o *reqOptions) (body []byte, h http.Header, err error) {
	if insta == nil {
		return nil, nil, fmt.Errorf("calling %s: %w", o.Endpoint, ErrInstaNotDefined)
	}
	req, err := insta.newRequest(o)
	if err != nil {
		return nil, nil, err
	}

	resp, err := insta.c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	insta.extractHeaders(resp.Header)
	body, err = readBody(resp)
	if err != nil {
		return nil, nil, err
	}
	if o.Stream {
		body = lastNDJSONLine(body)
	}

	err = insta.isError(resp.StatusCode, body, resp.Status, o.Endpoint)

	if insta.Debug {
		insta.debugHandler(fmt.Sprintf("%d %s %s\n%s", resp.StatusCode, req.Method, o.Endpoint, body))
	}

	hCopy := resp.Header.Clone()
	if insta.reqWrapper != nil {
		o.WrapperCount++
		body, hCopy, err = insta.reqWrapper.GoInstaWrapper(&ReqWrapperArgs{
			insta:      insta,
			reqOptions: o,
			Body:       body,
			Headers:    hCopy,
			Error:      err,
		})
	}
	return body, hCopy, err
}

func readBody(resp *http.Response) ([]byte, error) {
	switch resp.Header.Get("Content-Encoding") {
	case "zstd":
		raw, err := io.ReadAll(resp.Body)
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
		return io.ReadAll(zr)
	default:
		return io.ReadAll(resp.Body)
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
	extract := func(in, out string) {
		if v := h.Get(in); v != "" {
			insta.headerOptions.Store(out, v)
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
		insta.mu.Lock()
		insta.pwPubKey = pk
		insta.pwKeyID = h.Get("Ig-Set-Password-Encryption-Key-Id")
		insta.mu.Unlock()
	}
}

func (insta *Instagram) isError(code int, body []byte, status, endpoint string) error {
	switch code {
	case 200, 202, 204:
		return nil
	case 400, 403:
		ierr := Error400{Code: code, Endpoint: endpoint}
		if err := json.Unmarshal(body, &ierr); err != nil {
			return ErrorN{Endpoint: endpoint, Status: strconv.Itoa(code), Message: string(body), ErrorType: status}
		}
		switch ierr.GetMessage() {
		case "login_required":
			if ierr.ErrorTitle == "You've Been Logged Out" {
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
		return ierr
	case 429:
		return ErrTooManyRequests
	case 503:
		return Error503{Message: "Instagram API error. Try it later."}
	default:
		ierr := ErrorN{Endpoint: endpoint, Status: strconv.Itoa(code), Message: string(body), ErrorType: status}
		_ = json.Unmarshal(body, &ierr)
		return ierr
	}
}
