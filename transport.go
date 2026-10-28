package goinsta

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
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

// dialAppTLS opens a TLS connection over raw that presents the app's ClientHello.
func dialAppTLS(ctx context.Context, raw net.Conn, host string, insecure bool) (net.Conn, error) {
	uc := utls.UClient(raw, &utls.Config{ServerName: host, NextProtos: []string{"h2"}, InsecureSkipVerify: insecure}, utls.HelloCustom)
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

// rawDialer opens the TCP stream the TLS handshake runs over.
type rawDialer func(ctx context.Context, network, addr string) (net.Conn, error)

// newAppTransport speaks HTTP/2 over the app-fingerprinted TLS connection,
// directly or through proxyURL (http:// CONNECT or socks5://). insecure skips
// certificate verification, e.g. behind an intercepting proxy.
func newAppTransport(proxyURL *url.URL, insecure bool) (http.RoundTripper, error) {
	d := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	dial := rawDialer(d.DialContext)
	if proxyURL != nil {
		var err error
		if dial, err = proxyDialer(proxyURL, d); err != nil {
			return nil, err
		}
	}
	return &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			raw, err := dial(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return dialAppTLS(ctx, raw, host, insecure)
		},
		ReadIdleTimeout: 30 * time.Second,
	}, nil
}

func proxyDialer(u *url.URL, d *net.Dialer) (rawDialer, error) {
	switch u.Scheme {
	case "http":
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialConnect(ctx, d, u, addr)
		}, nil
	case "socks5", "socks5h":
		pd, err := proxy.FromURL(u, d)
		if err != nil {
			return nil, err
		}
		cd, ok := pd.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("socks5 dialer has no context support")
		}
		return cd.DialContext, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q (want http or socks5)", u.Scheme)
	}
}

// dialConnect opens a tunnel to addr through an HTTP proxy with CONNECT.
func dialConnect(ctx context.Context, d *net.Dialer, proxyURL *url.URL, addr string) (net.Conn, error) {
	conn, err := d.DialContext(ctx, "tcp", proxyURL.Host)
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
		defer conn.SetDeadline(time.Time{})
	}
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: http.Header{},
	}
	if pu := proxyURL.User; pu != nil {
		pass, _ := pu.Password()
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(pu.Username()+":"+pass)))
	}
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT %s: %w", addr, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT %s: %s", addr, resp.Status)
	}
	if br.Buffered() > 0 { // the TLS server speaks only after our ClientHello
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT %s: unexpected data after response", addr)
	}
	return conn, nil
}
