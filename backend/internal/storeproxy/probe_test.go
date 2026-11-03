package storeproxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

type connFixture struct {
	net.Conn
	closed bool
}

func (c *connFixture) Close() error { c.closed = true; return nil }

type bodyFixture struct{ reads, closes int }

func (b *bodyFixture) Read([]byte) (int, error) { b.reads++; return 0, io.EOF }
func (b *bodyFixture) Close() error             { b.closes++; return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)
type memoryConn struct {
	net.Conn
	reader io.Reader
	closed bool
}

func (c *memoryConn) Read(b []byte) (int, error) { return c.reader.Read(b) }
func (c *memoryConn) Close() error               { c.closed = true; return nil }
func TestOwnedConnectionCapsHandshakeAndResponseBytes(t *testing.T) {
	original := &memoryConn{reader: strings.NewReader(strings.Repeat("x", 1024))}
	connection := &readBudgetConn{Conn: original, remaining: 64}
	data, e := io.ReadAll(connection)
	if len(data) != 64 || !errors.Is(e, errReadBudget) || !original.closed {
		t.Fatal("unbounded wire read", len(data), e, original.closed)
	}
}

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProbeUsesOneTargetNoRedirectOrBodyAndOwnsCleanup(t *testing.T) {
	for _, status := range []int{204, 302} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls, requests, checks, cleanup := 0, 0, 0, 0
			connection := &connFixture{}
			body := &bodyFixture{}
			p := NetworkProber{Dial: func(ctx context.Context, _, address string) (net.Conn, error) {
				calls++
				if address != "proxy.example.invalid:1080" {
					t.Fatal("dialed target directly")
				}
				d, _ := ctx.Deadline()
				if time.Until(d) > 3*time.Second {
					t.Fatal("unbounded tcp")
				}
				return connection, nil
			}, Transport: func(e Endpoint) (http.RoundTripper, func()) {
				if e.URL.User != nil {
					t.Fatal("unexpected credentials in native transport")
				}
				return roundTripFunc(func(r *http.Request) (*http.Response, error) {
					requests++
					if r.URL.String() != fixtureTarget().URL || r.Method != "GET" || r.Header.Get("Authorization") != "" {
						t.Fatal("request scope drift")
					}
					d, _ := r.Context().Deadline()
					if time.Until(d) > 6*time.Second {
						t.Fatal("unbounded target")
					}
					return &http.Response{StatusCode: status, Header: http.Header{"Location": []string{"https://other.example.invalid/"}}, Body: body, Request: r}, nil
				}), func() { cleanup++ }
			}}
			r := p.Run(context.Background(), "socks5://proxy.example.invalid:1080", fixtureTarget(), "id", "fixture", func(context.Context) Reason { checks++; return None })
			if calls != 1 || requests != 1 || !connection.closed || body.reads != 0 || body.closes != 1 || cleanup != 1 || checks != 3 {
				t.Fatalf("bad bounds/cleanup: %d %d %v %#v %d %d", calls, requests, connection.closed, body, cleanup, checks)
			}
			if status == 204 && r.Target != "passed" || status == 302 && r.Target != "failed" {
				t.Fatal(r)
			}
		})
	}
}
func TestProbeFixedErrorsAndStageAdmission(t *testing.T) {
	for _, deniedAt := range []int{1, 2, 3} {
		calls, checks := 0, 0
		p := NetworkProber{Dial: func(context.Context, string, string) (net.Conn, error) { calls++; return &connFixture{}, nil }, Transport: func(Endpoint) (http.RoundTripper, func()) {
			return roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("socks5://fixture-user:fixture-secret@proxy.example.invalid:1080")
			}), func() {}
		}}
		r := p.Run(context.Background(), "http://proxy.example.invalid:1080", fixtureTarget(), "id", "fixture", func(context.Context) Reason {
			checks++
			if checks == deniedAt {
				return PermissionDenied
			}
			return None
		})
		if r.Reason != PermissionDenied || r.Syntax != "not_checked" || calls > 1 {
			t.Fatal(r, calls)
		}
	}
	for _, raw := range []string{"", "direct://", "http://host:0", "socks5://host", "http://host/path", "http://ho\nst:80", strings.Repeat("x", 8193), "vless://fixture", "hysteria2://fixture"} {
		p := NetworkProber{Dial: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("invalid input dialed")
			return nil, nil
		}}
		r := p.Run(context.Background(), raw, fixtureTarget(), "id", "fixture", func(context.Context) Reason { return None })
		if r.Reason != Unsupported && r.Syntax != "failed" || r.Reason == Unsupported && r.Syntax != "not_checked" {
			t.Fatal(raw, r)
		}
	}
}
func TestOwnedStandardTransportNeverDialsTargetOrSystemProxy(t *testing.T) {
	for _, raw := range []string{"http://proxy.example.invalid:8080", "https://proxy.example.invalid:443", "socks5://proxy.example.invalid:1080"} {
		e, reason := ParseEndpoint(raw)
		if reason != None {
			t.Fatal(reason)
		}
		p := NetworkProber{Dial: func(_ context.Context, _, address string) (net.Conn, error) {
			if address != e.Address {
				t.Fatal("direct fallback", address)
			}
			return nil, errors.New("fixture-secret")
		}}
		transport, closeOwned := p.ownedTransport(e)
		tr := transport.(*http.Transport)
		if !tr.DisableKeepAlives || !tr.DisableCompression || tr.MaxConnsPerHost != 1 || tr.ForceAttemptHTTP2 {
			t.Fatal("transport not bounded")
		}
		if e.URL.Scheme != "socks5" {
			_, _ = tr.DialContext(context.Background(), "tcp", "target.example.invalid:443")
		} else {
			_, _ = tr.DialContext(context.Background(), "tcp", "target.example.invalid:443")
		}
		closeOwned()
	}
}
func TestPhaseDeadlineDiscardsSuccessReturnedAfterCancellation(t *testing.T) {
	for _, stage := range []string{"tcp", "target"} {
		t.Run(stage, func(t *testing.T) {
			connection := &connFixture{}
			body := &bodyFixture{}
			closed := false
			p := NetworkProber{tcpTimeout: time.Millisecond, targetTimeout: time.Millisecond, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
				if stage == "tcp" {
					<-ctx.Done()
				}
				return connection, nil
			}, Transport: func(Endpoint) (http.RoundTripper, func()) {
				return roundTripFunc(func(r *http.Request) (*http.Response, error) {
					<-r.Context().Done()
					return &http.Response{StatusCode: 204, Header: http.Header{}, Body: body, Request: r}, nil
				}), func() { closed = true }
			}}
			r := p.Run(context.Background(), "http://proxy.example.invalid:8080", fixtureTarget(), "id", "fixture", func(context.Context) Reason { return None })
			if r.Reason != Timeout || r.Target == "passed" || !connection.closed {
				t.Fatal("late success accepted", r)
			}
			if stage == "target" && (!closed || body.reads != 0 || body.closes != 1) {
				t.Fatal("owned cleanup lost")
			}
		})
	}
}
func TestCredentialedStandardProxyCannotBypassConfiguredBridge(t *testing.T) {
	for _, raw := range []string{"http://fixture-user:fixture-secret@proxy.example.invalid:8080", "https://fixture-user:fixture-secret@proxy.example.invalid:443", "socks5://fixture-user:fixture-secret@proxy.example.invalid:1080"} {
		calls := 0
		p := NetworkProber{Dial: func(context.Context, string, string) (net.Conn, error) { calls++; return &connFixture{}, nil }, Transport: func(Endpoint) (http.RoundTripper, func()) {
			t.Fatal("credentialed proxy bypassed bridge")
			return nil, nil
		}}
		r := p.Run(context.Background(), raw, fixtureTarget(), "id", "fixture", func(context.Context) Reason { return None })
		if calls != 1 || r.Syntax != "passed" || r.TCP != "passed" || r.Target != "not_checked" || r.Reason != ConnectorRequired {
			t.Fatal(r, calls)
		}
		endpoint, _ := ParseEndpoint(raw)
		transport, closeOwned := p.ownedTransport(endpoint)
		_, e := transport.RoundTrip(&http.Request{})
		closeOwned()
		if !errors.Is(e, errConnectorRequired) || calls != 1 {
			t.Fatal("direct factory bypass")
		}
	}
}
