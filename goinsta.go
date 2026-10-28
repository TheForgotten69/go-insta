package goinsta

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"sync"
	"time"

	"go-insta/internal/httplog"
)

// Session: generate with `httplog session log_http.txt > session.json`.
type Session = httplog.Session

type Instagram struct {
	dID       string // android-<16 hex>
	uuid      string
	fID       string
	psID      string // UFS-<uuid>-0
	net       netSession
	userAgent string
	sess      Session

	headerOptions sync.Map

	mu       sync.Mutex
	pwPubKey string // base64 of a PEM RSA key, from ig-set-password-encryption-pub-key
	pwKeyID  string

	att *Attestation // registered keys, restored after the logged-out login flow

	Timeline    *Timeline
	Account     *Account
	Collections *Collections
	Activity    *Activity

	c *http.Client

	Debug bool

	infoHandler  func(...interface{})
	warnHandler  func(...interface{})
	debugHandler func(...interface{})

	reqWrapper ReqWrapper
}

func defaultHandler(args ...interface{}) {
	fmt.Println(args...)
}

func New(s Session) (*Instagram, error) {
	// A logged-out session (device fields, no token) is valid for Login; a session
	// with neither a token nor a device id is not usable.
	if s.Authorization == "" && s.DeviceID == "" {
		return nil, ErrSessionInvalid
	}
	if s.Authorization != "" && s.UserID == 0 {
		return nil, ErrSessionInvalid
	}
	if s.UserAgent == "" {
		s.UserAgent = createUserAgent(Waydroid)
	}
	if s.BloksVersionID == "" {
		s.BloksVersionID = bloksVerID
	}
	if s.AppID == "" {
		s.AppID = fbAnalytics
	}
	if s.Capabilities == "" {
		s.Capabilities = igCapabilities
	}
	if s.Locale == "" {
		s.Locale = locale
	}
	if s.TimezoneOffset == "" {
		s.TimezoneOffset = timeOffset
	}
	if s.TimezoneName == "" {
		s.TimezoneName = localTimezoneName()
	}
	if s.DeviceID == "" {
		s.DeviceID = generateUUID()
	}
	if s.FamilyDeviceID == "" {
		s.FamilyDeviceID = generateUUID()
	}
	if s.PigeonSession == "" {
		s.PigeonSession = "UFS-" + generateUUID() + "-0"
	}

	insta := &Instagram{
		dID:          s.AndroidID,
		uuid:         s.DeviceID,
		fID:          s.FamilyDeviceID,
		psID:         s.PigeonSession,
		net:          newNetSession(),
		userAgent:    s.UserAgent,
		sess:         s,
		c:            &http.Client{Transport: newAppTransport()},
		infoHandler:  defaultHandler,
		warnHandler:  defaultHandler,
		debugHandler: defaultHandler,
		reqWrapper:   DefaultWrapper(),
		Debug:        os.Getenv("GOINSTA_DEBUG") != "",
	}
	for k, v := range map[string]string{
		"Authorization":  s.Authorization,
		"X-Mid":          s.Mid,
		"X-Ig-Www-Claim": s.WWWClaim,
		"Ig-U-Rur":       s.Rur,
		"X-Meta-Zca":     s.ZCA,
		"X-Meta-Usdid":   s.USDID,
	} {
		if v != "" {
			insta.headerOptions.Store(k, v)
		}
	}
	insta.pwPubKey, insta.pwKeyID = s.PasswordPubKey, s.PasswordKeyID
	insta.Account = &Account{insta: insta, ID: Int64(s.UserID)}
	insta.Timeline = newTimeline(insta)
	insta.Collections = newCollections(insta)
	insta.Activity = newActivity(insta)
	return insta, nil
}

func ImportSession(path string) (*Instagram, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse session %s: %w", path, err)
	}
	return New(s)
}

func (insta *Instagram) Session() Session {
	s := insta.sess
	get := func(k string) string {
		if v, ok := insta.headerOptions.Load(k); ok {
			return v.(string)
		}
		return ""
	}
	s.Authorization = get("Authorization")
	s.Mid = get("X-Mid")
	s.WWWClaim = get("X-Ig-Www-Claim")
	s.Rur = get("Ig-U-Rur")
	insta.mu.Lock()
	s.PasswordPubKey, s.PasswordKeyID = insta.pwPubKey, insta.pwKeyID
	insta.mu.Unlock()
	return s
}

func (insta *Instagram) ExportSession(path string) error {
	b, err := json.MarshalIndent(insta.Session(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600) // WriteFile keeps the mode of an existing file
}

func (insta *Instagram) userID() int64 { return int64(insta.Account.ID) }

// LogoutResult carries the one-tap re-login nonces the server returns on logout.
type LogoutResult struct {
	LoginNonce               string `json:"login_nonce"`
	CloudAccountIDLoginNonce string `json:"cloud_account_id_login_nonce"`
	DeviceBasedLoginEnabled  bool   `json:"device_based_login_enabled"`
	Status                   string `json:"status"`
}

// Logout ends the session server-side. The session's token is invalid afterward;
// the returned nonce allows a later one-tap re-login from the same device.
func (insta *Instagram) Logout() (*LogoutResult, error) {
	body, _, err := insta.sendRequest(&reqOptions{
		Endpoint: urlLogout,
		IsPost:   true,
		Query: map[string]string{
			"_uuid":                         insta.uuid,
			"cloud_account_id":              insta.uuid,
			"device_id":                     insta.dID,
			"guid":                          insta.uuid,
			"is_app_level_spi":              "true",
			"one_tap_app_login":             "true",
			"phone_id":                      insta.fID,
			"prevent_remove_device_from_wa": "true",
		},
	})
	if err != nil {
		return nil, err
	}
	var r LogoutResult
	return &r, json.Unmarshal(body, &r)
}

func (insta *Instagram) SetInfoHandler(f func(...interface{}))  { insta.infoHandler = f }
func (insta *Instagram) SetWarnHandler(f func(...interface{}))  { insta.warnHandler = f }
func (insta *Instagram) SetDebugHandler(f func(...interface{})) { insta.debugHandler = f }
func (insta *Instagram) SetWrapper(fn ReqWrapper)               { insta.reqWrapper = fn }

func (insta *Instagram) SetHTTPClient(client *http.Client) { insta.c = client }

func (insta *Instagram) SetHTTPTransport(transport http.RoundTripper) { insta.c.Transport = transport }

func (insta *Instagram) SetTimeout(t time.Duration) { insta.c.Timeout = t }

func (insta *Instagram) SetProxy(url string, insecure bool, forceHTTP2 bool) error {
	uri, err := neturl.Parse(url)
	if err != nil {
		return err
	}
	insta.c.Transport = &http.Transport{
		Proxy:             http.ProxyURL(uri),
		ForceAttemptHTTP2: forceHTTP2,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: insecure},
	}
	return nil
}

func (insta *Instagram) UnsetProxy() {
	insta.c.Transport = newAppTransport()
}

func (insta *Instagram) GetAccountFamily() ([]byte, error) {
	body, _, err := insta.sendRequest(&reqOptions{Endpoint: urlGetAccFamily})
	return body, err
}
