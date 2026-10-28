package goinsta

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// connectProxy is a minimal HTTP CONNECT proxy that counts tunnels it opens.
func connectProxy(t *testing.T) (*url.URL, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var tunnels atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				req, err := http.ReadRequest(bufio.NewReader(c))
				if err != nil || req.Method != http.MethodConnect {
					return
				}
				up, err := net.Dial("tcp", req.Host)
				if err != nil {
					io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
					return
				}
				defer up.Close()
				tunnels.Add(1)
				io.WriteString(c, "HTTP/1.1 200 Connection established\r\n\r\n")
				go io.Copy(up, c)
				io.Copy(c, up)
			}(c)
		}
	}()
	return &url.URL{Scheme: "http", Host: ln.Addr().String()}, &tunnels
}

// The app ClientHello (TLS 1.3, X25519MLKEM768-only key share, h2) must complete
// a real handshake, both directly and through a CONNECT proxy.
func TestAppTransportHandshake(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.Proto)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	proxyURL, tunnels := connectProxy(t)
	for _, tc := range []struct {
		name  string
		proxy *url.URL
	}{{"direct", nil}, {"connect-proxy", proxyURL}} {
		t.Run(tc.name, func(t *testing.T) {
			rt, err := newAppTransport(tc.proxy, true)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
			resp, err := (&http.Client{Transport: rt}).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if string(body) != "HTTP/2.0" {
				t.Errorf("server saw %q, want HTTP/2.0", body)
			}
		})
	}
	if tunnels.Load() != 1 {
		t.Errorf("proxy tunnels = %d, want 1", tunnels.Load())
	}
}

func TestSetProxyRejectsUnknownScheme(t *testing.T) {
	insta, _ := newReplay(t)
	if err := insta.SetProxy("ftp://127.0.0.1:21", false); err == nil {
		t.Fatal("expected an error for ftp:// proxy")
	}
}
