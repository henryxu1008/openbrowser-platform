package storeproxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	xproxy "golang.org/x/net/proxy"
)

type Reason string

const (
	None              Reason = "NONE"
	InvalidProxy      Reason = "INVALID_PROXY"
	Unsupported       Reason = "UNSUPPORTED_PROTOCOL"
	TCPUnreachable    Reason = "TCP_UNREACHABLE"
	TargetUnreachable Reason = "TARGET_UNREACHABLE"
	AuthRejected      Reason = "AUTH_REJECTED"
	Timeout           Reason = "TIMEOUT"
	Canceled          Reason = "CANCELED"
	Unavailable       Reason = "UNAVAILABLE"
	PermissionDenied  Reason = "PERMISSION_DENIED"
	ContextChanged    Reason = "CONTEXT_CHANGED"
	TargetsRequired   Reason = "TARGET_CONFIGURATION_REQUIRED"
	Busy              Reason = "CHECK_BUSY"
	ConnectorRequired Reason = "CONNECTOR_REQUIRED"
)

type Result struct {
	Schema    string `json:"schema"`
	RequestID string `json:"requestId"`
	TargetID  string `json:"targetId"`
	Syntax    string `json:"syntax"`
	TCP       string `json:"tcp"`
	Target    string `json:"target"`
	Reason    Reason `json:"reason"`
}

func Unknown(id, target string, reason Reason) Result {
	return Result{"store-proxy-check-v1", id, target, "not_checked", "not_checked", "not_checked", reason}
}

type Endpoint struct {
	URL     *url.URL
	Address string
}

func ParseEndpoint(raw string) (Endpoint, Reason) {
	value := strings.TrimSpace(raw)
	if value == "" || len(raw) > 8192 || strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) || strings.EqualFold(value, "direct://") {
		return Endpoint{}, InvalidProxy
	}
	u, err := url.Parse(value)
	if err != nil {
		return Endpoint{}, InvalidProxy
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return Endpoint{}, Unsupported
	}
	if u.Opaque != "" || u.Hostname() == "" || strings.HasSuffix(u.Host, ":") || u.RawQuery != "" || u.ForceQuery || strings.Contains(value, "#") || (u.Path != "" && u.Path != "/") {
		return Endpoint{}, InvalidProxy
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return Endpoint{}, InvalidProxy
		}
	}
	n, e := strconv.Atoi(port)
	if e != nil || n < 1 || n > 65535 {
		return Endpoint{}, InvalidProxy
	}
	if u.Scheme == "socks5" && u.User != nil {
		password, _ := u.User.Password()
		if len(u.User.Username()) == 0 || len(u.User.Username()) > 255 || len(password) == 0 || len(password) > 255 {
			return Endpoint{}, InvalidProxy
		}
	}
	u.Scheme = strings.ToLower(u.Scheme)
	return Endpoint{u, net.JoinHostPort(u.Hostname(), port)}, None
}

type Authorize func(context.Context) Reason
type Prober interface {
	Run(context.Context, string, Target, string, string, Authorize) Result
}

// Connector owns only this operation's bridge. Cleanup must join its process and
// I/O; the returned endpoint must be a credential-free loopback SOCKS5 endpoint.
type Connector interface {
	Connect(context.Context, Endpoint) (Endpoint, func() error, Reason)
}
type NetworkProber struct {
	Dial                      func(context.Context, string, string) (net.Conn, error)
	Transport                 func(Endpoint) (http.RoundTripper, func())
	Connector                 Connector
	tcpTimeout, targetTimeout time.Duration // private fixture overrides, clamped below
}

var errReadBudget = errors.New("store_proxy_check_read_limit")

type readBudgetConn struct {
	net.Conn
	mu        sync.Mutex
	remaining int64
}

func (c *readBudgetConn) Read(buffer []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.remaining <= 0 {
		_ = c.Conn.Close()
		return 0, errReadBudget
	}
	if int64(len(buffer)) > c.remaining {
		buffer = buffer[:c.remaining]
	}
	n, err := c.Conn.Read(buffer)
	c.remaining -= int64(n)
	return n, err
}

func boundedPhase(value, maximum time.Duration) time.Duration {
	if value > 0 && value < maximum {
		return value
	}
	return maximum
}

func phaseReason(ctx context.Context, err error, otherwise Reason) Reason {
	if errors.Is(ctx.Err(), context.Canceled) {
		return Canceled
	}
	var timeout net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
		return Timeout
	}
	return otherwise
}
func (p NetworkProber) dial(ctx context.Context, network, address string) (net.Conn, error) {
	var connection net.Conn
	var err error
	if p.Dial != nil {
		connection, err = p.Dial(ctx, network, address)
	} else {
		d := net.Dialer{Timeout: 3 * time.Second, KeepAlive: -1, FallbackDelay: -1}
		connection, err = d.DialContext(ctx, network, address)
	}
	if connection != nil {
		return &readBudgetConn{Conn: connection, remaining: 256 * 1024}, err
	}
	return nil, err
}

var errProxyAuth = errors.New("store_proxy_check_auth_rejected")
var errConnectorRequired = errors.New("store_proxy_check_connector_required")

type unavailableTransport struct{}

func (unavailableTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errConnectorRequired
}

func (p NetworkProber) ownedTransport(endpoint Endpoint) (http.RoundTripper, func()) {
	// Browser kernel resolution sends credentialed standard proxies through its
	// configured bridge. Do not create an alternative authenticated native path.
	if endpoint.URL.User != nil {
		return unavailableTransport{}, func() {}
	}
	owned := newOwnedIO()
	proxyDial := func(ctx context.Context, _ string, _ string) (net.Conn, error) {
		return owned.dial(ctx, "tcp", endpoint.Address, p.dial)
	}
	tr := &http.Transport{DisableKeepAlives: true, DisableCompression: true, MaxConnsPerHost: 1, MaxIdleConns: 1, MaxResponseHeaderBytes: 32 * 1024, TLSHandshakeTimeout: 2 * time.Second, ResponseHeaderTimeout: 3 * time.Second, ForceAttemptHTTP2: false, DialContext: proxyDial, OnProxyConnectResponse: func(_ context.Context, _ *url.URL, _ *http.Request, response *http.Response) error {
		if response.StatusCode == 407 {
			return errProxyAuth
		}
		return nil
	}}
	if endpoint.URL.Scheme == "socks5" {
		// x/net's standard SOCKS5 dialer, with an operation-owned direct forwarder.
		tr.DialContext = func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("store_proxy_check_transport_unavailable")
		}
		forward := &ownedForwarder{dial: proxyDial}
		d, e := xproxy.SOCKS5("tcp", endpoint.Address, nil, forward)
		if e == nil {
			if cd, ok := d.(xproxy.ContextDialer); ok {
				tr.DialContext = cd.DialContext
			} else {
				tr.DialContext = func(context.Context, string, string) (net.Conn, error) {
					return nil, errors.New("store_proxy_check_transport_unavailable")
				}
			}
		}
	} else {
		tr.Proxy = http.ProxyURL(endpoint.URL)
	}
	return tr, func() { owned.closeAndWait(tr) }
}

type ownedForwarder struct {
	dial func(context.Context, string, string) (net.Conn, error)
}

func (d *ownedForwarder) Dial(network, address string) (net.Conn, error) {
	return nil, errors.New("store_proxy_check_context_required")
}
func (d *ownedForwarder) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dial(ctx, network, address)
}
func (p NetworkProber) Run(ctx context.Context, raw string, target Target, id, targetID string, authorize Authorize) (r Result) {
	check := func() Reason {
		if ctx.Err() != nil {
			return phaseReason(ctx, ctx.Err(), Unavailable)
		}
		return authorize(ctx)
	}
	r = Unknown(id, targetID, Unavailable)
	endpoint, reason := ParseEndpoint(raw)
	if reason != None {
		if reason != Unsupported {
			r.Syntax = "failed"
		}
		r.Reason = reason
		return r
	}
	r.Syntax = "passed"
	if reason = check(); reason != None {
		return Unknown(id, targetID, reason)
	}
	tcpCtx, stopTCP := context.WithTimeout(ctx, boundedPhase(p.tcpTimeout, 3*time.Second))
	connection, err := p.dial(tcpCtx, "tcp", endpoint.Address)
	if connection != nil {
		_ = connection.Close()
	}
	if tcpCtx.Err() != nil {
		err = tcpCtx.Err()
	}
	if err == nil && connection == nil {
		err = errors.New("store_proxy_check_transport_unavailable")
	}
	if err != nil {
		r.Reason = phaseReason(tcpCtx, err, TCPUnreachable)
		if r.Reason == TCPUnreachable {
			r.TCP = "failed"
		}
		stopTCP()
		return r
	}
	stopTCP()
	r.TCP = "passed"
	if reason = check(); reason != None {
		return Unknown(id, targetID, reason)
	}
	// TCP reachability is valid independent evidence, but neither authentication
	// nor target access may bypass the browser's required connector stack.
	if endpoint.URL.User != nil {
		if p.Connector == nil {
			r.Reason = ConnectorRequired
			return r
		}
		bridgeEndpoint, cleanup, bridgeReason := p.Connector.Connect(ctx, endpoint)
		if cleanup != nil {
			defer func() {
				if cleanup() != nil {
					r = Unknown(id, targetID, Unavailable)
				}
			}()
		}
		if bridgeReason != None {
			r.Reason = phaseReason(ctx, ctx.Err(), bridgeReason)
			return r
		}
		// A trusted adapter cannot turn a bridge error into a remote/direct route.
		if cleanup == nil || !validOwnedBridgeEndpoint(bridgeEndpoint) {
			r.Reason = ConnectorRequired
			return r
		}
		endpoint = bridgeEndpoint
		if reason = check(); reason != None {
			return Unknown(id, targetID, reason)
		}
	}
	targetCtx, stopTarget := context.WithTimeout(ctx, boundedPhase(p.targetTimeout, 6*time.Second))
	defer stopTarget()
	var transport http.RoundTripper
	var closeOwned func()
	if p.Transport != nil {
		transport, closeOwned = p.Transport(endpoint)
	} else {
		transport, closeOwned = p.ownedTransport(endpoint)
	}
	if transport == nil || closeOwned == nil {
		return r
	}
	defer closeOwned()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(targetCtx, http.MethodGet, target.URL, nil)
	if err != nil {
		return r
	}
	response, err := client.Do(request)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	} // Never read or log body/headers.
	if targetCtx.Err() != nil {
		err = targetCtx.Err()
	}
	if err != nil {
		r.Reason = phaseReason(targetCtx, err, TargetUnreachable)
		if errors.Is(err, errProxyAuth) {
			r.Reason = AuthRejected
		}
		if r.Reason == TargetUnreachable || r.Reason == AuthRejected {
			r.Target = "failed"
		}
	} else {
		r.Target = "failed"
		r.Reason = TargetUnreachable
		for _, status := range target.ExpectedStatus {
			if response.StatusCode == status {
				r.Target = "passed"
				r.Reason = None
				break
			}
		}
	}
	if reason = check(); reason != None {
		return Unknown(id, targetID, reason)
	}
	return r
}

func validOwnedBridgeEndpoint(endpoint Endpoint) bool {
	if endpoint.URL == nil || endpoint.URL.Scheme != "socks5" || endpoint.URL.User != nil || endpoint.URL.Hostname() != "127.0.0.1" || endpoint.URL.Path != "" || endpoint.URL.RawQuery != "" || endpoint.URL.ForceQuery || endpoint.URL.Fragment != "" {
		return false
	}
	parsed, reason := ParseEndpoint(endpoint.URL.String())
	return reason == None && parsed.Address == endpoint.Address
}
