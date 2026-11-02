// Package storebridge owns a single probe bridge, never shared browser bridges.
// The process/core-specific starter is a native capability, not renderer input.
package storebridge

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
	"ant-chrome/backend/internal/storeproxy"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var ErrRequired = errors.New("store_proxy_check_connector_required")
var ErrUnavailable = errors.New("store_proxy_check_connector_unavailable")

// Input contains ephemeral credentials. It is passed only to a native starter;
// formatting it is redacted. Secrets must travel through a private input stream,
// never command arguments, environment, files, process output or shared managers.
type Input struct {
	engine string
	proxy  storeproxy.Endpoint
}

func (i Input) Engine() string   { return i.engine }
func (i Input) String() string   { return "store-proxy-bridge[" + i.engine + "]" }
func (i Input) GoString() string { return i.String() }

// WritePrivateInput supplies a bounded native-adapter envelope. It is not an
// Xray/Mihomo config format; a core-specific stdin/config adapter is still needed.
func (i Input) WritePrivateInput(w io.Writer) error {
	if i.proxy.URL == nil || i.proxy.URL.User == nil || w == nil {
		return ErrUnavailable
	}
	password, _ := i.proxy.URL.User.Password()
	port, _ := strconv.Atoi(i.proxy.URL.Port())
	if port == 0 {
		if i.proxy.URL.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	data, err := json.Marshal(struct {
		Engine   string `json:"engine"`
		Protocol string `json:"protocol"`
		Server   string `json:"server"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	}{i.engine, i.proxy.URL.Scheme, i.proxy.URL.Hostname(), port, i.proxy.URL.User.Username(), password})
	if err != nil {
		return ErrUnavailable
	}
	data = append(data, '\n')
	defer clear(data)
	if n, err := w.Write(data); err != nil || n != len(data) {
		return ErrUnavailable
	}
	return nil
}

// Start must return every owned handle even when startup fails or context has
// expired. It must not detach work. Ready and Wait include their own I/O cleanup;
// Stop targets only that exact handle and unblocks Ready. Wait joins actual exit.
// Start's context bounds startup only: do not attach a returned process to that
// context after handoff. The lease watches the original operation lifetime.
type Process interface {
	Ready(context.Context) (string, error)
	Stop()
	Wait()
	// Settled is an immutable, non-nil signal owned by the process worker. It
	// closes only after actual exit and every input/output/cleanup worker joins,
	// independently of Stop/Wait helper success. Helpers must be idempotent and
	// non-panicking; the lease still contains their faults and waits for proof.
	Settled() <-chan struct{}
}
type Starter interface {
	Start(context.Context, Input) (Process, error)
}

type Connector struct {
	stack   string
	starter Starter
}

func New(stack string, starter Starter) *Connector { return &Connector{stack: stack, starter: starter} }

func makeInput(raw, stack string) (Input, error) {
	endpoint, reason := storeproxy.ParseEndpoint(raw)
	if reason != storeproxy.None || endpoint.URL.User == nil {
		return Input{}, ErrRequired
	}
	// Derive the exact engine from the existing configured stack authority.
	if stack != "" && stack != config.BrowserConnectorXray && stack != config.BrowserConnectorMihomo {
		return Input{}, ErrRequired
	}
	resolved, err := proxy.ResolveProxyKernelForConnector(raw, nil, "", stack)
	if err != nil || resolved.Kernel != proxy.ProxyKernelXray && resolved.Kernel != proxy.ProxyKernelMihomo {
		return Input{}, ErrRequired
	}
	return Input{resolved.Kernel, endpoint}, nil
}

func (c *Connector) Connect(ctx context.Context, endpoint storeproxy.Endpoint) (storeproxy.Endpoint, func() error, storeproxy.Reason) {
	if endpoint.URL == nil {
		return storeproxy.Endpoint{}, nil, storeproxy.ConnectorRequired
	}
	lease, err := c.Acquire(ctx, endpoint.URL.String())
	if err != nil {
		reason := storeproxy.Unavailable
		if errors.Is(err, ErrRequired) {
			reason = storeproxy.ConnectorRequired
		}
		if errors.Is(err, context.Canceled) {
			reason = storeproxy.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			reason = storeproxy.Timeout
		}
		return storeproxy.Endpoint{}, nil, reason
	}
	return lease.endpoint, lease.CloseAndWait, storeproxy.None
}

func (c *Connector) Acquire(ctx context.Context, raw string) (result *Lease, callErr error) {
	defer func() {
		if recover() != nil {
			result = nil
			callErr = ErrUnavailable
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input, err := makeInput(raw, c.stack)
	if err != nil || c.starter == nil {
		return nil, ErrRequired
	}
	startup, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	process, err := c.starter.Start(startup, input)
	if process == nil {
		return nil, fixedError(startup, err)
	}
	lease := &Lease{process: process}
	lifetimeWatch := lease.watch(ctx)
	startupWatch := lease.watch(startup)
	published := false
	defer func() {
		if !published {
			if lease.CloseAndWait() != nil {
				result = nil
				callErr = ErrUnavailable
			}
			lifetimeWatch()
			startupWatch()
		}
	}()
	lease.settled = process.Settled()
	if err != nil || startup.Err() != nil {
		return nil, fixedError(startup, err)
	}
	address, err := process.Ready(startup)
	startupWatch() // join a racing timeout/stop before publishing a ready lease
	parsed, reason := storeproxy.ParseEndpoint(address)
	if err != nil || startup.Err() != nil || reason != storeproxy.None || parsed.URL.Scheme != "socks5" || parsed.URL.User != nil || parsed.URL.Hostname() != "127.0.0.1" || parsed.URL.Path != "" || parsed.URL.RawQuery != "" || parsed.URL.ForceQuery || parsed.URL.Fragment != "" {
		return nil, fixedError(startup, err)
	}
	lease.endpoint = parsed
	lease.unwatch = lifetimeWatch
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	published = true
	return lease, nil
}

func fixedError(ctx context.Context, _ error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrUnavailable
}

type Lease struct {
	process             Process
	endpoint            storeproxy.Endpoint
	stopOnce, closeOnce sync.Once
	unwatch             func()
	settled             <-chan struct{}
	fault               atomic.Bool
}

func (l *Lease) contain(work func()) {
	defer func() {
		if recover() != nil {
			l.fault.Store(true)
		}
	}()
	work()
}
func (l *Lease) stop() { l.stopOnce.Do(func() { l.contain(l.process.Stop) }) }
func (l *Lease) watch(ctx context.Context) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(done); l.stop() })
	var joined sync.Once
	return func() {
		joined.Do(func() {
			if !stop() {
				<-done
			}
		})
	}
}
func (l *Lease) CloseAndWait() error {
	l.closeOnce.Do(func() {
		l.stop()
		l.contain(l.process.Wait)
		// Helper faults never stand in for actual settlement. An invalid provider
		// without a join signal cannot acknowledge cleanup or release its gate.
		<-l.settled
		if l.unwatch != nil {
			l.unwatch()
		}
	})
	if l.fault.Load() {
		return ErrUnavailable
	}
	return nil
}

// Endpoint exposes only the validated private bridge endpoint, never upstream
// credentials. The caller still owns its transport and must close it first.
func (l *Lease) Endpoint() string { return l.endpoint.URL.String() }

var _ storeproxy.Connector = (*Connector)(nil)
