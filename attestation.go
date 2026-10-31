package goinsta

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strconv"
	"time"

	"github.com/TheForgotten69/go-insta/internal/httplog"
)

// Attestation reproduces the x-meta-zca and x-meta-usdid headers the app signs
// at launch, using the device keys already registered for this session. It does
// not register anything new: keyHash and lastUploadedKeyTimeMs stay the values
// the device uploaded, carried over from the capture.
type Attestation struct {
	cfg                   AttestationConfig
	zcaKey                *ecdsa.PrivateKey
	usdidKey              *ecdsa.PrivateKey
	keyHash               string
	lastUploadedKeyTimeMs int64
	usdidUUID             string

	// payload.plugins.bat; defaults match the capture.
	BatteryLevel int
	BatteryState string
}

type AttestationConfig = httplog.AttestationKeys

func parseECKey(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		ec, ok := k.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is %T, want EC", k)
		}
		return ec, nil
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func NewAttestation(c AttestationConfig) (*Attestation, error) {
	zk, err := parseECKey([]byte(c.ZCAKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("zca key: %w", err)
	}
	uk, err := parseECKey([]byte(c.USDIDKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("usdid key: %w", err)
	}
	keyHash := c.KeyHash
	if keyHash == "" {
		if keyHash, err = keyHashOf(zk); err != nil {
			return nil, err
		}
	}
	return &Attestation{
		cfg: c, zcaKey: zk, usdidKey: uk, keyHash: keyHash,
		lastUploadedKeyTimeMs: c.LastUploadedKeyTimeMs, usdidUUID: c.USDIDUUID,
		BatteryLevel: 85, BatteryState: "Charging",
	}, nil
}

// keyHashOf is the aka.keyHash: lowercase hex SHA-256 of the key's
// SubjectPublicKeyInfo (verified against the capture).
func keyHashOf(k *ecdsa.PrivateKey) (string, error) {
	spki, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(spki)
	return hex.EncodeToString(h[:]), nil
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// ECDSA signatures are non-deterministic, so these never equal a captured value;
// the server verifies them with the registered public key.
func signSHA256(k *ecdsa.PrivateKey, msg []byte) (string, error) {
	h := sha256.Sum256(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, k, h[:])
	if err != nil {
		return "", err
	}
	return b64url(sig), nil
}

// ZCA builds the x-meta-zca value. hash is base64url(sha256(time)); dataToSign is
// signed by the zca key (verified against two captures).
func (a *Attestation) ZCA(now time.Time) (string, error) {
	timeStr := strconv.FormatInt(now.UnixMilli(), 10)
	hsum := sha256.Sum256([]byte(timeStr))
	dataToSign, err := json.Marshal(zcaSignedData{Time: timeStr, Hash: b64url(hsum[:])})
	if err != nil {
		return "", err
	}
	signed, err := signSHA256(a.zcaKey, dataToSign)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(zcaDoc{Android: zcaAndroid{
		AKA: zcaAKA{
			DataToSign:            string(dataToSign),
			SignedData:            signed,
			KeyHash:               a.keyHash,
			LastUploadedKeyTimeMs: a.lastUploadedKeyTimeMs,
		},
		GPIA: zcaGPIA{Errors: []string{"PLAY_INTEGRITY_DISABLED_BY_CONFIG"}},
		Payload: zcaPayload{Plugins: zcaPlugins{
			Bat: zcaBattery{Sta: a.BatteryState, Lvl: a.BatteryLevel},
			ADB: zcaADB{USB: -1, ADB: 1, USBADB: -1},
		}},
	}})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// zcaSignedData is the aka.dataToSign document, keys in the app's order.
type zcaSignedData struct {
	Time string `json:"time"`
	Hash string `json:"hash"`
}

type zcaDoc struct {
	Android zcaAndroid `json:"android"`
}

type zcaAndroid struct {
	AKA     zcaAKA     `json:"aka"`
	GPIA    zcaGPIA    `json:"gpia"`
	Payload zcaPayload `json:"payload"`
}

type zcaAKA struct {
	DataToSign            string `json:"dataToSign"`
	SignedData            string `json:"signedData"`
	KeyHash               string `json:"keyHash"`
	LastUploadedKeyTimeMs int64  `json:"lastUploadedKeyTimeMs"`
}

type zcaGPIA struct {
	Token  string   `json:"token"`
	Errors []string `json:"errors"`
}

type zcaPayload struct {
	Plugins zcaPlugins `json:"plugins"`
}

type zcaPlugins struct {
	Bat zcaBattery  `json:"bat"`
	Sct emptyObject `json:"sct"`
	ADB zcaADB      `json:"adb"`
}

type zcaBattery struct {
	Sta string `json:"sta"`
	Lvl int    `json:"lvl"`
}

type zcaADB struct {
	USB    int `json:"usb"`
	ADB    int `json:"adb"`
	USBADB int `json:"usb_adb"`
}

// USDID builds the x-meta-usdid value "uuid.expiry.spki.sig", signed over the
// first three fields. The app re-signs with a fresh one-hour expiry each launch.
func (a *Attestation) USDID(now time.Time) (string, error) {
	expiry := strconv.FormatInt(now.Add(time.Hour).Unix(), 10)
	spki, err := x509.MarshalPKIXPublicKey(&a.usdidKey.PublicKey)
	if err != nil {
		return "", err
	}
	msg := a.usdidUUID + "." + expiry + "." + b64url(spki)
	sig, err := signSHA256(a.usdidKey, []byte(msg))
	if err != nil {
		return "", err
	}
	return msg + "." + sig, nil
}

// loggedOut returns a copy with a freshly minted zca key that has never been
// uploaded (lastUploadedKeyTimeMs 0). Logged out, the app signs zca with such a
// key and announces its hash to attestation/create_android_keystore.
func (a *Attestation) loggedOut() (*Attestation, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	kh, err := keyHashOf(k)
	if err != nil {
		return nil, err
	}
	c := *a
	c.zcaKey, c.keyHash, c.lastUploadedKeyTimeMs = k, kh, 0
	return &c, nil
}

// attestationMaxAge is when the headers get re-signed: x-meta-usdid expires one
// hour after signing.
const attestationMaxAge = 50 * time.Minute

// SetAttestation signs both headers and uses them for every request, replacing
// the values carried in the session. They are re-signed before they expire.
func (insta *Instagram) SetAttestation(a *Attestation) error {
	if err := insta.applyAttestation(a); err != nil {
		return err
	}
	insta.mu.Lock()
	insta.att = a
	insta.mu.Unlock()
	return nil
}

func (insta *Instagram) applyAttestation(a *Attestation) error {
	now := time.Now()
	zca, err := a.ZCA(now)
	if err != nil {
		return err
	}
	insta.headerOptions.Store("X-Meta-Zca", zca)
	if a.usdidKey == nil {
		insta.headerOptions.Delete("X-Meta-Usdid") // a carried-over value would be expired
	} else {
		usdid, err := a.USDID(now)
		if err != nil {
			return err
		}
		insta.headerOptions.Store("X-Meta-Usdid", usdid)
	}
	insta.mu.Lock()
	insta.curAtt, insta.attSignedAt = a, now
	insta.mu.Unlock()
	return nil
}

// refreshAttestation re-signs the active attestation once it nears expiry.
func (insta *Instagram) refreshAttestation(now time.Time) error {
	insta.mu.Lock()
	a, signed := insta.curAtt, insta.attSignedAt
	insta.mu.Unlock()
	if a == nil || now.Sub(signed) < attestationMaxAge {
		return nil
	}
	return insta.applyAttestation(a)
}
