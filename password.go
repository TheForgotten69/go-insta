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
	"fmt"
	"strconv"
	"time"
)

// encryptPassword builds an Instagram #PWD_INSTAGRAM:4 value. The AES-256-GCM key
// (AAD = the timestamp) is wrapped with RSA-PKCS1v1.5 to the server key. Layout:
// [0x01][keyID][iv:12][le16 len(sealed)][sealed][gcm tag:16][ciphertext].
func encryptPassword(password string, keyID int, pubKeyPEM []byte, now time.Time) (string, error) {
	pub, err := parseRSAPub(pubKeyPEM)
	if err != nil {
		return "", err
	}
	aesKey := make([]byte, 32)
	iv := make([]byte, 12)
	if _, err := rand.Read(aesKey); err != nil {
		return "", err
	}
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	ts := strconv.FormatInt(now.Unix(), 10)

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, iv, []byte(password), []byte(ts))
	ct, tag := sealed[:len(sealed)-16], sealed[len(sealed)-16:]

	wrapped, err := rsa.EncryptPKCS1v15(rand.Reader, pub, aesKey)
	if err != nil {
		return "", err
	}

	var buf []byte
	buf = append(buf, 0x01, byte(keyID))
	buf = append(buf, iv...)
	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(wrapped)))
	buf = append(buf, wrapped...)
	buf = append(buf, tag...)
	buf = append(buf, ct...)

	return fmt.Sprintf("#PWD_INSTAGRAM:4:%s:%s", ts, base64.StdEncoding.EncodeToString(buf)), nil
}

// parseRSAPub accepts a PEM SubjectPublicKeyInfo, which is how the server delivers
// the password key (base64 of this PEM in ig-set-password-encryption-pub-key).
func parseRSAPub(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("password key: no PEM block")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("password key: %T, want RSA", k)
	}
	return rk, nil
}
