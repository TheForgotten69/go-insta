package goinsta

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

// TestEncryptPassword builds an enc_password with a throwaway RSA key, then unwraps
// it the way the server would, proving the byte layout and AES-GCM/AAD are correct.
func TestEncryptPassword(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: spki})

	now := time.Unix(1793149031, 0)
	const password = "hunter2 pass"
	enc, err := encryptPassword(password, 181, pubPEM, now)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(enc, ":")
	if len(parts) != 4 || parts[0] != "#PWD_INSTAGRAM" || parts[1] != "4" || parts[2] != "1793149031" {
		t.Fatalf("prefix wrong: %q", enc[:40])
	}
	buf, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		t.Fatal(err)
	}
	if buf[0] != 0x01 || buf[1] != 181 {
		t.Errorf("version/keyid = %d/%d", buf[0], buf[1])
	}
	iv := buf[2:14]
	ln := int(binary.LittleEndian.Uint16(buf[14:16]))
	if ln != 256 {
		t.Errorf("sealed len = %d, want 256 (RSA-2048)", ln)
	}
	wrapped := buf[16 : 16+ln]
	tag := buf[16+ln : 16+ln+16]
	ct := buf[16+ln+16:]

	// Server side: RSA-unwrap the AES key, then GCM-open with AAD = timestamp.
	aesKey, err := rsa.DecryptPKCS1v15(rand.Reader, priv, wrapped)
	if err != nil {
		t.Fatalf("RSA unwrap: %v", err)
	}
	block, _ := aes.NewCipher(aesKey)
	gcm, _ := cipher.NewGCMWithNonceSize(block, len(iv))
	plain, err := gcm.Open(nil, iv, append(append([]byte{}, ct...), tag...), []byte(parts[2]))
	if err != nil {
		t.Fatalf("GCM open: %v", err)
	}
	if string(plain) != password {
		t.Errorf("decrypted = %q, want %q", plain, password)
	}
}

func TestEncryptPasswordRejectsNonRSA(t *testing.T) {
	if _, err := encryptPassword("x", 1, []byte("not a pem"), time.Now()); err == nil {
		t.Error("expected error on bad key")
	}
}
