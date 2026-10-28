package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"go-insta/internal/httplog"
)

// Session: generate with `httplog session log_http.txt > session.json`.
type Session = httplog.Session

// Instagram is safe for concurrent use once configured: call the Set* methods
// (except SetProxy/UnsetProxy/SetHTTPClient, which may run at any time) before
// sharing it. Login swaps the client into the logged-out profile, so do not run
// other requests while it is in flight. Pagers (Timeline, Posts, ...) and the
// Account struct belong to one goroutine each.
type Instagram struct {
	dID  string // android-<16 hex>
	uuid string
	fID  string
	psID string // UFS-<uuid>-0
	sess Session

	net           netSession
	bw            bandwidth
	baseHeader    http.Header // static per client; cloned per request
	feedSessionID string

	uid           atomic.Int64
	c             atomic.Pointer[http.Client]
	headerOptions sync.Map

	mu          sync.Mutex
	pwPubKey    string // base64 of a PEM RSA key, from ig-set-password-encryption-pub-key
	pwKeyID     string
	att         *Attestation // registered keys, restored after the logged-out login flow
	curAtt      *Attestation // the one currently signing headers
	attSignedAt time.Time

	Account *Account

	// Logger receives rate-limit warnings and, at debug level, every response
	// body. It defaults to slog.Default().
	Logger *slog.Logger
	// RateLimitWait is how long a 429 is waited out before retrying (up to 3
	// times); 0 returns ErrTooManyRequests at once.
	RateLimitWait time.Duration
}

const DefaultRateLimitWait = 8 * time.Minute

func New(s Session) (*Instagram, error) {
	// A logged-out session (device fields, no token) is valid for Login; a session
	// with neither a token nor a device id is not usable.
	if s.Authorization == "" && s.DeviceID == "" {
		return nil, ErrSessionInvalid
	}
	if s.Authorization != "" && s.UserID == 0 {
		return nil, ErrSessionInvalid
	}
	if s.Locale == "" {
		s.Locale = locale
	}
	if s.UserAgent == "" {
		s.UserAgent = createUserAgent(Waydroid, s.Locale)
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
		dID:           s.AndroidID,
		uuid:          s.DeviceID,
		fID:           s.FamilyDeviceID,
		psID:          s.PigeonSession,
		sess:          s,
		net:           newNetSession(),
		feedSessionID: generateUUID(),
		pwPubKey:      s.PasswordPubKey,
		pwKeyID:       s.PasswordKeyID,
		RateLimitWait: DefaultRateLimitWait,
		Logger:        slog.Default(),
	}
	insta.baseHeader = insta.staticHeaders()
	insta.uid.Store(s.UserID)
	insta.c.Store(&http.Client{Transport: directTransport()})
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
	insta.Account = &Account{insta: insta, ID: Int64(s.UserID)}
	if s.Attestation != nil {
		a, err := NewAttestation(*s.Attestation)
		if err != nil {
			return nil, fmt.Errorf("session attestation: %w", err)
		}
		if err := insta.SetAttestation(a); err != nil {
			return nil, err
		}
	}
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

func (insta *Instagram) header(k string) string {
	if v, ok := insta.headerOptions.Load(k); ok {
		return v.(string)
	}
	return ""
}

func (insta *Instagram) Session() Session {
	s := insta.sess
	s.UserID = insta.userID()
	s.Authorization = insta.header("Authorization")
	s.Mid = insta.header("X-Mid")
	s.WWWClaim = insta.header("X-Ig-Www-Claim")
	s.Rur = insta.header("Ig-U-Rur")
	s.ZCA = insta.header("X-Meta-Zca")
	s.USDID = insta.header("X-Meta-Usdid")
	insta.mu.Lock()
	s.PasswordPubKey, s.PasswordKeyID = insta.pwPubKey, insta.pwKeyID
	if insta.att != nil {
		cfg := insta.att.cfg
		s.Attestation = &cfg
	}
	insta.mu.Unlock()
	return s
}

// ExportSession writes the session (with the attestation private keys) mode 0600.
func (insta *Instagram) ExportSession(path string) error {
	b, err := json.MarshalIndent(insta.Session(), "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil { // WriteFile keeps the mode of an existing file
		return err
	}
	return os.Rename(tmp, path)
}

func (insta *Instagram) userID() int64 { return insta.uid.Load() }

// LogoutResult carries the one-tap re-login nonces the server returns on logout.
type LogoutResult struct {
	LoginNonce               string `json:"login_nonce"`
	CloudAccountIDLoginNonce string `json:"cloud_account_id_login_nonce"`
	DeviceBasedLoginEnabled  bool   `json:"device_based_login_enabled"`
	Status                   string `json:"status"`
}

// Logout ends the session server-side. The session's token is invalid afterward;
// the returned nonce allows a later one-tap re-login from the same device.
func (insta *Instagram) Logout(ctx context.Context) (*LogoutResult, error) {
	var r LogoutResult
	err := insta.callJSON(ctx, &reqOptions{
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
	}, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (insta *Instagram) client() *http.Client { return insta.c.Load() }

func (insta *Instagram) swapClient(c *http.Client) {
	old := insta.c.Swap(c)
	if old == nil || sameTransport(old.Transport, c.Transport) {
		return
	}
	if ci, ok := old.Transport.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}

func sameTransport(a, b http.RoundTripper) bool {
	ta := reflect.TypeOf(a)
	if ta != reflect.TypeOf(b) || ta == nil || !ta.Comparable() {
		return ta == nil && b == nil
	}
	return a == b
}

// SetHTTPClient replaces the client wholesale. Its transport does not present
// the app's TLS fingerprint unless it came from SetProxy; logins from other
// fingerprints get 429. Meant for tests and custom round trippers.
func (insta *Instagram) SetHTTPClient(client *http.Client) { insta.swapClient(client) }

func (insta *Instagram) SetTimeout(t time.Duration) {
	c := *insta.client()
	c.Timeout = t
	insta.swapClient(&c)
}

// SetProxy routes requests through an http:// (CONNECT) or socks5:// proxy while
// keeping the app's TLS fingerprint. insecure skips certificate verification,
// which an intercepting proxy such as mitmproxy needs.
func (insta *Instagram) SetProxy(rawURL string, insecure bool) error {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return err
	}
	t, err := newAppTransport(u, insecure)
	if err != nil {
		return err
	}
	c := *insta.client()
	c.Transport = t
	insta.swapClient(&c)
	return nil
}

func (insta *Instagram) UnsetProxy() {
	c := *insta.client()
	c.Transport = directTransport()
	insta.swapClient(&c)
}

// directTransport is the app-fingerprinted transport without a proxy.
func directTransport() http.RoundTripper {
	t, _ := newAppTransport(nil, false) // only proxy setup can fail
	return t
}

func (insta *Instagram) AccountFamily(ctx context.Context) ([]byte, error) {
	return insta.call(ctx, &reqOptions{Endpoint: urlGetAccFamily})
}
