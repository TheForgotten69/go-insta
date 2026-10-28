package goinsta

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// appClientHello is the ClientHello the app's Tigon/Fizz stack sends to
// i.instagram.com (453.0.0.55.79, captured 2026-10-28): TLS 1.3 only, one
// cipher, an X25519MLKEM768-only key share, no GREASE/EMS/padding, an empty
// legacy session id, and this exact extension order. Meta answers logins from
// other TLS fingerprints with 429.
func appClientHello() *utls.ClientHelloSpec {
	return &utls.ClientHelloSpec{
		TLSVersMin:         utls.VersionTLS13,
		TLSVersMax:         utls.VersionTLS13,
		CipherSuites:       []uint16{utls.TLS_AES_128_GCM_SHA256},
		CompressionMethods: []byte{0},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.SupportedVersionsExtension{Versions: []uint16{utls.VersionTLS13}},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519MLKEM768, utls.X25519, utls.CurveP256}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519MLKEM768}}},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256, utls.ECDSAWithP384AndSHA384, utls.PSSWithSHA256,
			}},
			&utls.ALPNExtension{AlpnProtocols: []string{"h2"}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModePlain, utls.PskModeDHE}},
		},
	}
}

// dialAppTLS opens a TLS connection that presents the app's ClientHello.
func dialAppTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	d := net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	raw, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	uc := utls.UClient(raw, &utls.Config{ServerName: host, NextProtos: []string{"h2"}}, utls.HelloCustom)
	if err := uc.ApplyPreset(appClientHello()); err != nil {
		raw.Close()
		return nil, err
	}
	if err := uc.BuildHandshakeState(); err != nil {
		raw.Close()
		return nil, err
	}
	uc.HandshakeState.Hello.SessionId = nil
	// uTLS 1.8.2 aborts with "internal error" when the plain ECDHE key is unset,
	// which it is for an X25519MLKEM768-only key share. Its X25519 half is the
	// same group, so point the plain slot at it; the wire bytes do not change.
	if ks := uc.HandshakeState.State13.KeyShareKeys; ks != nil && ks.Ecdhe == nil {
		ks.Ecdhe = ks.MlkemEcdhe
	}
	if err := uc.MarshalClientHello(); err != nil {
		raw.Close()
		return nil, err
	}
	if err := uc.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, fmt.Errorf("tls handshake with %s: %w", host, err)
	}
	if p := uc.ConnectionState().NegotiatedProtocol; p != "h2" {
		uc.Close()
		return nil, fmt.Errorf("%s negotiated %q, want h2", host, p)
	}
	return uc, nil
}

// newAppTransport speaks HTTP/2 over the app-fingerprinted TLS connection.
// It dials directly; SetProxy/SetHTTPTransport replace it (and the fingerprint).
func newAppTransport() http.RoundTripper {
	return &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return dialAppTLS(ctx, network, addr)
		},
		ReadIdleTimeout: 30 * time.Second,
	}
}
