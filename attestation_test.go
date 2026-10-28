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
	"strconv"
	"strings"
	"testing"
	"time"
)

func testKeyPEM(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), k
}

func newTestAttestation(t *testing.T) (*Attestation, *ecdsa.PrivateKey, *ecdsa.PrivateKey) {
	t.Helper()
	zcaPEM, zcaKey := testKeyPEM(t)
	usdidPEM, usdidKey := testKeyPEM(t)
	a, err := NewAttestation(AttestationConfig{
		ZCAKeyPEM: zcaPEM, USDIDKeyPEM: usdidPEM,
		KeyHash:               "eb59585dd168ba8a3297bcd69150dcb65dfffc430238d4f4103240989e7b70d4",
		LastUploadedKeyTimeMs: 1793119682768,
		USDIDUUID:             "521604d9-eb45-4763-81a7-a748c024e43e",
	})
	if err != nil {
		t.Fatal(err)
	}
	return a, zcaKey, usdidKey
}

func verifyASN1(t *testing.T, pub *ecdsa.PublicKey, msg []byte, sigB64 string) bool {
	t.Helper()
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(msg)
	return ecdsa.VerifyASN1(pub, h[:], sig)
}

func TestZCAFormat(t *testing.T) {
	a, zcaKey, _ := newTestAttestation(t)
	now := time.UnixMilli(1793137242688)
	val, err := a.ZCA(now)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		t.Fatalf("zca is not standard base64: %v", err)
	}
	var doc struct {
		Android struct {
			Aka struct {
				DataToSign            string `json:"dataToSign"`
				SignedData            string `json:"signedData"`
				KeyHash               string `json:"keyHash"`
				LastUploadedKeyTimeMs int64  `json:"lastUploadedKeyTimeMs"`
			} `json:"aka"`
		} `json:"android"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	aka := doc.Android.Aka
	if aka.KeyHash != a.keyHash || aka.LastUploadedKeyTimeMs != a.lastUploadedKeyTimeMs {
		t.Errorf("registration constants not carried: %+v", aka)
	}
	if !verifyASN1(t, &zcaKey.PublicKey, []byte(aka.DataToSign), aka.SignedData) {
		t.Error("signedData does not verify against the zca key")
	}
	var data struct{ Time, Hash string }
	if err := json.Unmarshal([]byte(aka.DataToSign), &data); err != nil {
		t.Fatal(err)
	}
	if data.Time != "1793137242688" {
		t.Errorf("time = %q", data.Time)
	}
	want := base64.RawURLEncoding.EncodeToString(sha256sum(data.Time))
	if data.Hash != want {
		t.Errorf("hash = %q, want base64url(sha256(time)) = %q", data.Hash, want)
	}
}

func TestUSDIDFormat(t *testing.T) {
	a, _, usdidKey := newTestAttestation(t)
	now := time.Unix(1793137246, 0)
	val, err := a.USDID(now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(val, ".")
	if len(parts) != 4 {
		t.Fatalf("usdid has %d fields, want 4", len(parts))
	}
	if parts[0] != a.usdidUUID {
		t.Errorf("uuid = %q", parts[0])
	}
	if parts[1] != "1793140846" {
		t.Errorf("expiry = %q, want now + 1h", parts[1])
	}
	spki, err := x509.MarshalPKIXPublicKey(&usdidKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if parts[2] != base64.RawURLEncoding.EncodeToString(spki) {
		t.Error("spki field does not match the key")
	}
	msg := parts[0] + "." + parts[1] + "." + parts[2]
	if !verifyASN1(t, &usdidKey.PublicKey, []byte(msg), parts[3]) {
		t.Error("signature does not verify over uuid.expiry.spki")
	}
}

func TestSetAttestationOverridesSession(t *testing.T) {
	insta, _ := newReplay(t)
	a, _, _ := newTestAttestation(t)
	if err := insta.SetAttestation(a); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"X-Meta-Zca", "X-Meta-Usdid"} {
		v, ok := insta.headerOptions.Load(k)
		if !ok || v.(string) == "" || v.(string) == "zca" || v.(string) == "usdid" {
			t.Errorf("%s not replaced: %v", k, v)
		}
	}
}

func TestKeyHashDerivation(t *testing.T) {
	zcaPEM, zcaKey := testKeyPEM(t)
	usdidPEM, _ := testKeyPEM(t)
	a, err := NewAttestation(AttestationConfig{ZCAKeyPEM: zcaPEM, USDIDKeyPEM: usdidPEM})
	if err != nil {
		t.Fatal(err)
	}
	spki, err := x509.MarshalPKIXPublicKey(&zcaKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(spki)
	if a.keyHash != hex.EncodeToString(h[:]) {
		t.Errorf("keyHash = %q, want hex(sha256(zca spki)) = %q", a.keyHash, hex.EncodeToString(h[:]))
	}
}

func sha256sum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func TestAttestationResignedBeforeExpiry(t *testing.T) {
	insta, _ := newReplay(t)
	a, _, _ := newTestAttestation(t)
	if err := insta.SetAttestation(a); err != nil {
		t.Fatal(err)
	}
	before, _ := insta.headerOptions.Load("X-Meta-Usdid")

	if err := insta.refreshAttestation(time.Now().Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if v, _ := insta.headerOptions.Load("X-Meta-Usdid"); v != before {
		t.Fatal("re-signed while still fresh")
	}

	insta.mu.Lock()
	insta.attSignedAt = time.Now().Add(-55 * time.Minute)
	insta.mu.Unlock()
	if err := insta.refreshAttestation(time.Now()); err != nil {
		t.Fatal(err)
	}
	after, _ := insta.headerOptions.Load("X-Meta-Usdid")
	if after == before {
		t.Fatal("usdid not re-signed near expiry")
	}
	exp, _ := strconv.ParseInt(strings.Split(after.(string), ".")[1], 10, 64)
	if time.Until(time.Unix(exp, 0)) < 55*time.Minute {
		t.Errorf("new usdid expires too soon: %v", time.Unix(exp, 0))
	}
}

func TestAttestationWithoutUSDIDKeyDropsStaleHeader(t *testing.T) {
	insta, _ := newReplay(t)
	lo, err := (&Attestation{}).loggedOut()
	if err != nil {
		t.Fatal(err)
	}
	if err := insta.applyAttestation(lo); err != nil {
		t.Fatal(err)
	}
	if v := insta.header("X-Meta-Usdid"); v != "" {
		t.Errorf("stale usdid kept: %q", v)
	}
}

func TestSessionCarriesAttestation(t *testing.T) {
	insta, _ := newReplay(t)
	a, _, _ := newTestAttestation(t)
	if err := insta.SetAttestation(a); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/s.json"
	if err := insta.ExportSession(path); err != nil {
		t.Fatal(err)
	}
	again, err := ImportSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.att == nil || again.att.keyHash != a.keyHash || again.att.usdidUUID != a.usdidUUID {
		t.Fatalf("attestation not restored: %+v", again.att)
	}
	if !strings.HasPrefix(again.header("X-Meta-Usdid"), a.usdidUUID+".") {
		t.Error("imported attestation not signing")
	}
}
