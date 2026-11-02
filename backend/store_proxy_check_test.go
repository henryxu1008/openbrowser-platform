package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
	"ant-chrome/backend/internal/storeproxy"
	"ant-chrome/internal/shuomiapi"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

type storeProbeFunc func(context.Context, string, storeproxy.Target, string, string, storeproxy.Authorize) storeproxy.Result
type storeProbeTCPConn struct{ net.Conn }

func (*storeProbeTCPConn) Close() error { return nil }
func TestStoreProxyRealTransportJoinsCanceledDialBeforeSettlement(t *testing.T) {
	a, _, s := storeAppFixture(t)
	binding := installStoreFixture(t, a, s)
	a.storeProbeCatalog = probeCatalog(t)
	entered, aborted, cleanup := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(cleanup) })
	var calls atomic.Int32
	a.storeProbeRunner = storeproxy.NetworkProber{Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		switch calls.Add(1) {
		case 1:
			return &storeProbeTCPConn{}, nil
		case 2:
			close(entered)
			<-ctx.Done()
			close(aborted)
			<-cleanup
			return nil, ctx.Err()
		default:
			return nil, errors.New("unexpected probe reentry")
		}
	}}
	id := uuid.NewString()
	var request storeProxyRequest
	_ = json.Unmarshal([]byte(probeRaw(id, a.storeProbeCatalog.Version())), &request)
	request.ProxyConfig = "http://proxy.example.invalid:8080"
	raw, _ := json.Marshal(request)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		result, err := a.StoreProxyCheck(s.StoreID, binding.BindingVersion, string(raw), 1)
		if r := parseProbeResult(t, result, err); r.Reason != storeproxy.Canceled {
			t.Errorf("canceled worker returned %s", r.Reason)
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("real transport did not dial")
	}
	canceled := make(chan struct{})
	go func() {
		defer close(canceled)
		result, err := a.StoreProxyCheckCancel(s.StoreID, id, 1)
		if err != nil || !strings.Contains(result, `"settled":true`) {
			t.Error("cancel did not acknowledge settled cleanup")
		}
	}()
	select {
	case <-aborted:
	case <-time.After(time.Second):
		t.Fatal("dial did not receive cancellation")
	}
	select {
	case <-finished:
		t.Error("check settled before actual dial cleanup")
	case <-time.After(30 * time.Millisecond):
	}
	select {
	case <-canceled:
		t.Error("cancel acknowledged before actual dial cleanup")
	default:
	}
	_ = json.Unmarshal([]byte(probeRaw(uuid.NewString(), a.storeProbeCatalog.Version())), &request)
	request.ProxyConfig = "http://proxy.example.invalid:8080"
	raw, _ = json.Marshal(request)
	result, e := a.StoreProxyCheck(s.StoreID, binding.BindingVersion, string(raw), 1)
	if r := parseProbeResult(t, result, e); r.Reason != storeproxy.Busy || calls.Load() != 2 {
		t.Errorf("another worker entered before cleanup: reason=%s dials=%d", r.Reason, calls.Load())
	}
	release.Do(func() { close(cleanup) })
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("worker did not settle after cleanup")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("cancel did not acknowledge after cleanup")
	}
}

func TestStoreProxyNativeSubsetMatchesExistingConnectorMatrix(t *testing.T) {
	for _, connector := range []string{config.BrowserConnectorXray, config.BrowserConnectorMihomo} {
		for _, scheme := range []string{"http", "https", "socks5"} {
			for _, credentials := range []string{"", "fixture-user:fixture-secret@"} {
				raw := scheme + "://" + credentials + "proxy.example.invalid:1080"
				resolution, e := proxy.ResolveProxyKernelForConnector(raw, nil, "", connector)
				if e != nil {
					t.Fatal("existing matrix rejected fixture", e)
				}
				if credentials == "" && resolution.Kernel != proxy.ProxyKernelNative || credentials != "" && resolution.Kernel == proxy.ProxyKernelNative {
					t.Fatal("native subset drifted from configured stack")
				}
			}
		}
	}
}

func (f storeProbeFunc) Run(c context.Context, p string, t storeproxy.Target, id, target string, a storeproxy.Authorize) storeproxy.Result {
	return f(c, p, t, id, target, a)
}

type probeAPIFixture struct {
	*storeAPIFixture
	allowed  string
	checks   atomic.Int32
	deny     atomic.Bool
	onCanUse func(context.Context) error
}

func (f *probeAPIFixture) CanUse(ctx context.Context, _ uint64, _ string) error {
	f.checks.Add(1)
	if f.onCanUse != nil {
		return f.onCanUse(ctx)
	}
	if f.deny.Load() {
		return errors.New("PERMISSION_DENIED")
	}
	return nil
}
func TestStoreProxyCancelOwnsAdmissionBeforeAnyProbe(t *testing.T) {
	a, base, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	entered := make(chan struct{})
	api := &probeAPIFixture{storeAPIFixture: base, allowed: s.StoreID, onCanUse: func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		return errors.New("unavailable fixture-secret")
	}}
	a.store.client = api
	a.storeProbeCatalog = probeCatalog(t)
	a.storeProbeRunner = storeProbeFunc(func(context.Context, string, storeproxy.Target, string, string, storeproxy.Authorize) storeproxy.Result {
		t.Error("canceled admission probed")
		return storeproxy.Result{}
	})
	id := uuid.NewString()
	finished := make(chan storeproxy.Result, 1)
	go func() {
		raw, e := a.StoreProxyCheck(s.StoreID, b.BindingVersion, probeRaw(id, a.storeProbeCatalog.Version()), 1)
		if e != nil {
			finished <- storeproxy.Result{}
			return
		}
		var r storeproxy.Result
		_ = json.Unmarshal([]byte(raw), &r)
		finished <- r
	}()
	<-entered
	raw, e := a.StoreProxyCheckCancel(s.StoreID, id, 1)
	if e != nil || !strings.Contains(raw, `"settled":true`) {
		t.Fatal("cleanup not acknowledged", raw, e)
	}
	select {
	case r := <-finished:
		if r.Reason != storeproxy.Canceled {
			t.Fatal(r)
		}
	case <-time.After(time.Second):
		t.Fatal("admission did not settle")
	}
}
func (f *probeAPIFixture) Store(_ context.Context, _ uint64, _, id string) (shuomiapi.Store, error) {
	if id != f.allowed {
		return shuomiapi.Store{}, errors.New("NOT_FOUND")
	}
	return shuomiapi.Store{ID: id}, nil
}
func probeCatalog(t *testing.T) storeproxy.Catalog {
	t.Helper()
	c, e := storeproxy.NewCatalog([]storeproxy.Target{{ID: "fixture", Label: "Offline target", URL: "https://target.example.invalid/check", ExpectedStatus: []int{204}}})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func probeRaw(id, version string) string {
	b, _ := json.Marshal(storeProxyRequest{id, "http://fixture-user:fixture-secret@proxy.example.invalid:8080", "fixture", version, time.Now().Add(12 * time.Second).UnixMilli()})
	return string(b)
}
func parseProbeResult(t *testing.T, raw string, err error) storeproxy.Result {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	var r storeproxy.Result
	if json.Unmarshal([]byte(raw), &r) != nil {
		t.Fatal("invalid DTO")
	}
	if strings.Contains(raw, "fixture-secret") || strings.Contains(raw, "recentLogs") {
		t.Fatal("credentials/diagnostics escaped")
	}
	return r
}
func TestStoreProxyStaleDispatchCannotReviveCanceledIntent(t *testing.T) {
	a, _, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	a.storeProbeCatalog = probeCatalog(t)
	a.storeProbeRunner = storeProbeFunc(func(context.Context, string, storeproxy.Target, string, string, storeproxy.Authorize) storeproxy.Result {
		t.Error("stale dispatch probed")
		return storeproxy.Result{}
	})
	var request storeProxyRequest
	_ = json.Unmarshal([]byte(probeRaw(uuid.NewString(), a.storeProbeCatalog.Version())), &request)
	request.ExpiresAtUnixMs = time.Now().Add(-time.Second).UnixMilli()
	raw, _ := json.Marshal(request)
	result, e := a.StoreProxyCheck(s.StoreID, b.BindingVersion, string(raw), 1)
	if r := parseProbeResult(t, result, e); r.Reason != storeproxy.Timeout {
		t.Fatal(r)
	}
	request.ExpiresAtUnixMs = time.Now().Add(time.Minute).UnixMilli()
	raw, _ = json.Marshal(request)
	if _, e = a.StoreProxyCheck(s.StoreID, b.BindingVersion, string(raw), 1); e != errStoreProxyRequest {
		t.Fatal("unbounded deadline accepted")
	}
}

func TestStoreProxyCheckProductionCatalogIsFailClosed(t *testing.T) {
	a, api, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	a.config.ProxyCheck.Targets = []config.ProxyCheckTarget{{ID: "user-target", URL: "https://user.example.invalid/"}}
	a.storeProbeRunner = storeProbeFunc(func(context.Context, string, storeproxy.Target, string, string, storeproxy.Authorize) storeproxy.Result {
		t.Fatal("unapproved catalog reached probe")
		return storeproxy.Result{}
	})
	raw, e := a.StoreEnvironmentOptions(s.StoreID, 1)
	if e != nil {
		t.Fatal(e)
	}
	var options struct {
		ProxyCheck storeProxyOptions `json:"proxyCheck"`
	}
	if json.Unmarshal([]byte(raw), &options) != nil || options.ProxyCheck.Available || len(options.ProxyCheck.Targets) != 0 || options.ProxyCheck.Reason != storeproxy.TargetsRequired {
		t.Fatal("borrowed generic targets", raw)
	}
	result, e := a.StoreProxyCheck(s.StoreID, b.BindingVersion, probeRaw(uuid.NewString(), ""), 1)
	r := parseProbeResult(t, result, e)
	if r.Reason != storeproxy.TargetsRequired || r.Syntax != "not_checked" || api.admitted != 0 || api.reports != 0 {
		t.Fatal(r)
	}
}
func TestStoreProxyRequestAndPanicNeverExposeCredentials(t *testing.T) {
	for _, raw := range []string{`{"requestId":"not-uuid","proxyConfig":"fixture-secret"}`, `{"requestId":"10000000-0000-0000-0000-000000000001","proxyConfig":"fixture-secret","extra":"credential"}`, strings.Repeat("x", 16385)} {
		if _, e := decodeStoreProxyRequest(raw); e != errStoreProxyRequest {
			t.Fatal("unbounded or unclassified input accepted")
		}
	}
	a, _, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	a.storeProbeCatalog = probeCatalog(t)
	a.storeProbeRunner = storeProbeFunc(func(context.Context, string, storeproxy.Target, string, string, storeproxy.Authorize) storeproxy.Result {
		panic("http://fixture-user:fixture-secret@proxy.example.invalid:8080")
	})
	if raw, e := a.StoreProxyCheck(s.StoreID, b.BindingVersion, probeRaw(uuid.NewString(), a.storeProbeCatalog.Version()), 1); e == nil || e.Error() != "store_proxy_check_unavailable" || raw != "" {
		t.Fatal("panic not mapped to fixed error")
	}
}
func TestStoreProxyCheckRequiresCurrentIdentityStorePermissionAndBinding(t *testing.T) {
	a, base, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	api := &probeAPIFixture{storeAPIFixture: base, allowed: s.StoreID}
	a.store.client = api
	a.storeProbeCatalog = probeCatalog(t)
	calls := 0
	a.storeProbeRunner = storeProbeFunc(func(ctx context.Context, _ string, _ storeproxy.Target, id, target string, authorize storeproxy.Authorize) storeproxy.Result {
		calls++
		for range 3 {
			if reason := authorize(ctx); reason != storeproxy.None {
				return storeproxy.Unknown(id, target, reason)
			}
		}
		r := storeproxy.Unknown(id, target, storeproxy.None)
		r.Syntax = "passed"
		r.TCP = "passed"
		r.Target = "passed"
		return r
	})
	bound := NewAuthBoundApp(a)
	if _, e := bound.StoreProxyCheck(s.StoreID, b.BindingVersion, probeRaw(uuid.NewString(), a.storeProbeCatalog.Version()), 1, 8); e == nil {
		t.Fatal("stale identity admitted")
	}
	for _, tc := range []struct {
		store, version string
		revision       uint64
		denied         bool
		reason         storeproxy.Reason
	}{{s.StoreID, b.BindingVersion, 2, false, storeproxy.ContextChanged}, {s.StoreID, "0", 1, false, storeproxy.ContextChanged}, {uuid.NewString(), "0", 1, false, storeproxy.PermissionDenied}, {s.StoreID, b.BindingVersion, 1, true, storeproxy.PermissionDenied}} {
		api.deny.Store(tc.denied)
		raw, e := bound.StoreProxyCheck(tc.store, tc.version, probeRaw(uuid.NewString(), a.storeProbeCatalog.Version()), tc.revision, 7)
		if r := parseProbeResult(t, raw, e); r.Reason != tc.reason {
			t.Fatal(tc, r)
		}
	}
	if calls != 0 {
		t.Fatal("denied request probed")
	}
	api.deny.Store(false)
	raw, e := bound.StoreProxyCheck(s.StoreID, b.BindingVersion, probeRaw(uuid.NewString(), a.storeProbeCatalog.Version()), 1, 7)
	r := parseProbeResult(t, raw, e)
	if r.Target != "passed" || calls != 1 || api.checks.Load() < 4 || base.admitted != 0 || base.reports != 0 || len(a.browserMgr.Profiles) != 1 {
		t.Fatal("admission/side effect drift", r)
	}
	if _, e = bound.TestProxyConnectivity("", "http://proxy.example.invalid:8080", 7); !errors.Is(e, errStoreBoundary) {
		t.Fatal("generic Store guard weakened", e)
	}
}
func TestStoreProxyCheckDropsPermissionAndBindingChangesBeforePublication(t *testing.T) {
	for _, kind := range []string{"permission", "binding", "organization", "identity"} {
		t.Run(kind, func(t *testing.T) {
			a, base, s := storeAppFixture(t)
			b := installStoreFixture(t, a, s)
			api := &probeAPIFixture{storeAPIFixture: base, allowed: s.StoreID}
			a.store.client = api
			a.storeProbeCatalog = probeCatalog(t)
			a.storeProbeRunner = storeProbeFunc(func(ctx context.Context, _ string, _ storeproxy.Target, id, target string, authorize storeproxy.Authorize) storeproxy.Result {
				if reason := authorize(ctx); reason != storeproxy.None {
					t.Fatal(reason)
				}
				switch kind {
				case "permission":
					api.deny.Store(true)
				case "binding":
					if _, e := a.db.GetConn().Exec("UPDATE browser_store_bindings SET binding_version=binding_version+1 WHERE store_id=?", s.StoreID); e != nil {
						t.Fatal(e)
					}
				case "organization":
					a.store.mu.Lock()
					a.store.revision++
					a.store.organization = "org-b"
					a.store.mu.Unlock()
				case "identity":
					base.auth.mu.Lock()
					base.auth.status.Generation++
					base.auth.mu.Unlock()
				}
				r := storeproxy.Unknown(id, target, storeproxy.None)
				r.Syntax = "passed"
				r.TCP = "passed"
				r.Target = "passed"
				return r
			})
			raw, e := a.StoreProxyCheck(s.StoreID, b.BindingVersion, probeRaw(uuid.NewString(), a.storeProbeCatalog.Version()), 1)
			r := parseProbeResult(t, raw, e)
			if r.Target != "not_checked" || r.Reason != storeproxy.ContextChanged && r.Reason != storeproxy.PermissionDenied {
				t.Fatal("stale evidence published", r)
			}
		})
	}
}
func TestStoreProxyCancelHoldsGateAcrossRevisionAndRemountUntilCleanup(t *testing.T) {
	a, base, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	a.storeProbeCatalog = probeCatalog(t)
	entered, aborted, cleanup := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	a.storeProbeRunner = storeProbeFunc(func(ctx context.Context, _ string, _ storeproxy.Target, id, target string, _ storeproxy.Authorize) storeproxy.Result {
		calls.Add(1)
		close(entered)
		<-ctx.Done()
		close(aborted)
		<-cleanup
		return storeproxy.Unknown(id, target, storeproxy.Canceled)
	})
	id := uuid.NewString()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = a.StoreProxyCheck(s.StoreID, b.BindingVersion, probeRaw(id, a.storeProbeCatalog.Version()), 1)
	}()
	<-entered
	a.store.mu.Lock()
	a.store.revision = 2
	a.store.organization = "org-b"
	a.store.mu.Unlock()
	canceled := make(chan struct{})
	go func() { defer close(canceled); _, _ = a.StoreProxyCheckCancel(s.StoreID, id, 1) }()
	<-aborted
	select {
	case <-canceled:
		t.Fatal("cancel acknowledged before worker cleanup")
	default:
	}
	raw, e := a.StoreProxyCheck(s.StoreID, "0", probeRaw(uuid.NewString(), a.storeProbeCatalog.Version()), 2)
	if r := parseProbeResult(t, raw, e); r.Reason != storeproxy.Busy || calls.Load() != 1 {
		t.Fatal("remount overlapped old worker", r)
	}
	close(cleanup)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("worker did not settle")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("cancel did not acknowledge cleanup")
	}
	if base.admitted != 0 || base.reports != 0 {
		t.Fatal("cancel performed business action")
	}
}
