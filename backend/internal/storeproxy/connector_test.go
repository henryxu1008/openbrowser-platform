package storeproxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"reflect"
	"testing"
)

type connectorFunc func(context.Context, Endpoint) (Endpoint, func() error, Reason)

func (f connectorFunc) Connect(ctx context.Context, e Endpoint) (Endpoint, func() error, Reason) {
	return f(ctx, e)
}

func TestProbeOwnedConnectorCleanupFaultReturnsUnknown(t *testing.T) {
	bridge, _ := ParseEndpoint("socks5://127.0.0.1:31991")
	p := NetworkProber{Dial: func(context.Context, string, string) (net.Conn, error) { return &connFixture{}, nil }, Connector: connectorFunc(func(context.Context, Endpoint) (Endpoint, func() error, Reason) {
		return bridge, func() error { return errors.New("fixture-secret") }, None
	}), Transport: func(Endpoint) (http.RoundTripper, func()) {
		return roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 204, Body: &bodyFixture{}, Request: r}, nil
		}), func() {}
	}}
	r := p.Run(context.Background(), "http://user:pass@proxy.example.invalid:8080", fixtureTarget(), "id", "fixture", func(context.Context) Reason { return None })
	if r.Reason != Unavailable || r.Target != "not_checked" || r.TCP != "not_checked" {
		t.Fatal("cleanup fault published evidence")
	}
}

func TestProbeOwnedConnectorUsesLoopbackAndJoinsTransportBeforeBridge(t *testing.T) {
	var events []string
	bridge, _ := ParseEndpoint("socks5://127.0.0.1:31991")
	p := NetworkProber{
		Dial: func(_ context.Context, _, address string) (net.Conn, error) {
			if address != "proxy.example.invalid:8080" {
				t.Error("TCP evidence did not use explicit upstream")
			}
			return &connFixture{}, nil
		},
		Connector: connectorFunc(func(ctx context.Context, e Endpoint) (Endpoint, func() error, Reason) {
			if e.URL.User == nil || e.URL.User.Username() != "fixture-user" {
				t.Error("credentialed endpoint not passed to owned adapter")
			}
			events = append(events, "bridge-ready")
			return bridge, func() error { events = append(events, "bridge-joined"); return nil }, None
		}),
		Transport: func(e Endpoint) (http.RoundTripper, func()) {
			if e.URL.String() != bridge.URL.String() || e.URL.User != nil {
				t.Error("bridge/native boundary drift")
			}
			return roundTripFunc(func(r *http.Request) (*http.Response, error) {
				events = append(events, "one-target")
				if r.URL.String() != fixtureTarget().URL || r.Header.Get("Authorization") != "" {
					t.Error("target or auth drift")
				}
				return &http.Response{StatusCode: 204, Body: &bodyFixture{}, Request: r}, nil
			}), func() { events = append(events, "transport-joined") }
		},
	}
	r := p.Run(context.Background(), "http://fixture-user:fixture-secret@proxy.example.invalid:8080", fixtureTarget(), "id", "fixture", func(context.Context) Reason { return None })
	if r.Reason != None || r.Target != "passed" || !reflect.DeepEqual(events, []string{"bridge-ready", "one-target", "transport-joined", "bridge-joined"}) {
		t.Fatal("owned connector order/evidence", r, events)
	}
}

func TestProbeRejectsNonOwnedBridgeEndpointsAndCleansPartialStartup(t *testing.T) {
	for _, address := range []string{"socks5://remote.example.invalid:1080", "http://127.0.0.1:1080", "socks5://user:pass@127.0.0.1:1080", "socks5://127.0.0.1:1080/?x=1"} {
		endpoint, _ := ParseEndpoint(address)
		cleaned := 0
		p := NetworkProber{Dial: func(context.Context, string, string) (net.Conn, error) { return &connFixture{}, nil }, Connector: connectorFunc(func(context.Context, Endpoint) (Endpoint, func() error, Reason) {
			return endpoint, func() error { cleaned++; return nil }, None
		}), Transport: func(Endpoint) (http.RoundTripper, func()) {
			t.Error("unowned endpoint reached transport")
			return nil, nil
		}}
		r := p.Run(context.Background(), "http://user:pass@proxy.example.invalid:8080", fixtureTarget(), "id", "fixture", func(context.Context) Reason { return None })
		if r.Reason != ConnectorRequired || r.Target != "not_checked" || cleaned != 1 {
			t.Error("untrusted bridge accepted or leaked")
		}
	}
	cleaned := 0
	p := NetworkProber{Dial: func(context.Context, string, string) (net.Conn, error) { return &connFixture{}, nil }, Connector: connectorFunc(func(context.Context, Endpoint) (Endpoint, func() error, Reason) {
		return Endpoint{}, func() error { cleaned++; return nil }, Timeout
	})}
	r := p.Run(context.Background(), "http://user:pass@proxy.example.invalid:8080", fixtureTarget(), "id", "fixture", func(context.Context) Reason { return None })
	if r.Reason != Timeout || r.TCP != "passed" || r.Target != "not_checked" || cleaned != 1 {
		t.Error("partial bridge startup cleanup/evidence lost")
	}
}

func TestProbeRechecksAuthorityAfterOwnedBridgeStartup(t *testing.T) {
	checks, cleaned := 0, 0
	bridge, _ := ParseEndpoint("socks5://127.0.0.1:31991")
	p := NetworkProber{Dial: func(context.Context, string, string) (net.Conn, error) { return &connFixture{}, nil }, Connector: connectorFunc(func(context.Context, Endpoint) (Endpoint, func() error, Reason) {
		return bridge, func() error { cleaned++; return nil }, None
	}), Transport: func(Endpoint) (http.RoundTripper, func()) { t.Error("revoked bridge reached target"); return nil, nil }}
	r := p.Run(context.Background(), "http://user:pass@proxy.example.invalid:8080", fixtureTarget(), "id", "fixture", func(context.Context) Reason {
		checks++
		if checks == 3 {
			return PermissionDenied
		}
		return None
	})
	if r.Reason != PermissionDenied || r.Syntax != "not_checked" || cleaned != 1 {
		t.Fatal("bridge ignored revoked authority")
	}
}
