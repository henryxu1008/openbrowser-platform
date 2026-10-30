package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/internal/desktopauth"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type nativeIngressFixture struct {
	app      *App
	bound    *AuthBoundApp
	identity *mockSession
	server   *launchcode.LaunchServer
	client   *http.Client
	base     string
	key      string
}

// Only identity is substituted. App, facade, SQLite DAO, profile manager,
// production Start/middleware/router and HTTP operations are the real code.
// The listener is test-owned loopback:0; no host startup, EXE or kernel runs.
func newNativeIngressFixture(t *testing.T) *nativeIngressFixture {
	t.Helper()
	t.Setenv("ABFREE_OIDC_ISSUER", "")
	t.Setenv("ABFREE_OIDC_CLIENT_ID", "")
	a, b, identity := authFixture(t)
	// This fixture tests identity ingress over profile-only operations, without
	// a platform catalog. Published-kernel selection is covered separately.
	a.kernels = nil
	a.config = config.DefaultConfig()
	a.config.Browser.UserDataRoot = filepath.Join(a.appRoot, "profiles")
	a.db = newProfilePackageDatabase(t, a.appRoot)
	a.browserMgr = browser.NewManager(a.config, a.appRoot)
	a.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(a.db.GetConn())
	a.browserMgr.InitData()
	s := launchcode.NewLaunchServer(nil, a, a.browserMgr, 0)
	s.SetIdentityAuthority(identity, a.scopedHTTP)
	s.SetAPIAuthConfig(launchcode.APIAuthConfig{Enabled: true, APIKey: t.Name()})
	a.launchServer = s
	if err := b.AuthGrantAPI(7, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: nil, ExpectContinueTimeout: 10 * time.Second}
	f := &nativeIngressFixture{app: a, bound: b, identity: identity, server: s, client: &http.Client{Transport: transport, Timeout: 5 * time.Second}, base: fmt.Sprintf("http://127.0.0.1:%d", s.Port()), key: t.Name()}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		if err := s.Stop(); err != nil {
			t.Errorf("test-owned listener cleanup: %v", err)
		}
	})
	return f
}

func (f *nativeIngressFixture) request(t *testing.T, method, path, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, f.base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(launchcode.DefaultAPIKeyHeader, f.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, data
}

func (f *nativeIngressFixture) assertProfiles(t *testing.T, want int) {
	t.Helper()
	var rows int
	if err := f.app.db.GetConn().QueryRow("SELECT COUNT(*) FROM browser_profiles").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if got := len(f.app.browserMgr.List()); rows != want || got != want {
		t.Fatalf("real catalog/SQLite changed: memory=%d rows=%d want=%d", got, rows, want)
	}
	if len(f.app.browserMgr.BrowserProcesses) != 0 || f.app.forceQuit {
		t.Fatal("profile-only operation entered process lifecycle")
	}
}

func (f *nativeIngressFixture) replaceIdentity(t *testing.T) uint64 {
	t.Helper()
	f.identity.replace()
	f.identity.mu.Lock()
	f.identity.status.ExpiresAt = time.Now().Add(time.Hour)
	f.identity.mu.Unlock()
	return f.identity.Status().Generation
}

func TestAuthBoundRealStorageRejectsRevokedIngress(t *testing.T) {
	for _, mode := range []string{"logout", "expired-deadline"} {
		t.Run(mode, func(t *testing.T) {
			f := newNativeIngressFixture(t)
			created, err := f.bound.BrowserProfileCreate(BrowserProfileInput{ProfileName: "native-before"}, 7)
			if err != nil {
				t.Fatal(err)
			}
			status, body := f.request(t, http.MethodPost, "/api/profiles", `{"profile":{"profileName":"http-before"},"autoLaunch":false}`)
			if status != http.StatusCreated {
				t.Fatalf("normal real HTTP delegation=%d", status)
			}
			var receipt struct {
				Created  bool `json:"created"`
				Launched bool `json:"launched"`
			}
			if json.Unmarshal(body, &receipt) != nil || !receipt.Created || receipt.Launched {
				t.Fatal("HTTP did not return a profile-only creation receipt")
			}
			f.assertProfiles(t, 2)
			if mode == "logout" {
				if _, err := f.bound.AuthLogout(7); err != nil {
					t.Fatal(err)
				}
			} else {
				f.identity.mu.Lock()
				f.identity.status.ExpiresAt = time.Unix(1, 0)
				f.identity.mu.Unlock()
				if f.identity.Status().State != "signed_in" {
					t.Fatal("expiry fixture must exercise deadline, not an expired state label")
				}
			}
			if _, err := f.bound.BrowserProfileCreate(BrowserProfileInput{ProfileName: "denied"}, 7); !errors.Is(err, desktopauth.ErrUnauthenticated) {
				t.Fatalf("revoked Wails create=%v", err)
			}
			if err := f.bound.BrowserProfileDelete(created.ProfileId, 7); !errors.Is(err, desktopauth.ErrUnauthenticated) {
				t.Fatalf("revoked Wails delete=%v", err)
			}
			if profiles, err := f.bound.BrowserProfileList(7); !errors.Is(err, desktopauth.ErrUnauthenticated) || len(profiles) != 0 {
				t.Fatal("revoked Wails read returned profile data")
			}
			for _, req := range []struct{ method, path, body string }{
				{http.MethodGet, "/api/profiles", ""},
				{http.MethodPost, "/api/profiles", `{"profile":{"profileName":"denied"}}`},
				{http.MethodDelete, "/api/profiles/" + created.ProfileId, ""},
				{http.MethodGet, "/json/version", ""},
				{http.MethodGet, "/devtools/browser/denied", ""},
			} {
				if status, _ := f.request(t, req.method, req.path, req.body); status != http.StatusUnauthorized {
					t.Fatalf("revoked %s %s=%d", req.method, req.path, status)
				}
			}
			if status, _ := f.request(t, http.MethodGet, "/api/health", ""); status != http.StatusOK {
				t.Fatal("revocation removed liveness")
			}
			f.assertProfiles(t, 2)
			generation := f.replaceIdentity(t)
			if _, err := f.bound.BrowserProfileCreate(BrowserProfileInput{ProfileName: "stale"}, 7); !errors.Is(err, desktopauth.ErrUnauthenticated) {
				t.Fatal("old Wails generation adopted replacement")
			}
			if status, _ := f.request(t, http.MethodPost, "/api/profiles", `{"profile":{"profileName":"ungranted"}}`); status != http.StatusUnauthorized {
				t.Fatal("replacement inherited API grant")
			}
			f.assertProfiles(t, 2)
			if err := f.bound.AuthGrantAPI(generation, true); err != nil {
				t.Fatal(err)
			}
			if status, _ := f.request(t, http.MethodPost, "/api/profiles", `{"profile":{"profileName":"http-after"}}`); status != http.StatusCreated {
				t.Fatalf("fresh HTTP delegation=%d", status)
			}
			if _, err := f.bound.BrowserProfileCreate(BrowserProfileInput{ProfileName: "native-after"}, generation); err != nil {
				t.Fatal(err)
			}
			f.assertProfiles(t, 4)
		})
	}
}

func TestAuthBoundHTTPBodyCannotAdoptReplacement(t *testing.T) {
	f := newNativeIngressFixture(t)
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	payload := `{"profile":{"profileName":"late"},"autoLaunch":false}`
	req, err := http.NewRequest(http.MethodPost, f.base+"/api/profiles", reader)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = int64(len(payload))
	req.Header.Set(launchcode.DefaultAPIKeyHeader, f.key)
	req.Header.Set("Expect", "100-continue")
	entered := make(chan struct{})
	var once sync.Once
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{Got100Continue: func() { once.Do(func() { close(entered) }) }}))
	type result struct {
		status int
		err    error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := f.client.Do(req)
		if err != nil {
			done <- result{err: err}
			return
		}
		defer resp.Body.Close()
		_, err = io.Copy(io.Discard, resp.Body)
		done <- result{status: resp.StatusCode, err: err}
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("production handler did not begin reading admitted body")
	}
	if _, err := f.bound.AuthLogout(7); err != nil {
		t.Fatal(err)
	}
	generation := f.replaceIdentity(t)
	if err := f.bound.AuthGrantAPI(generation, true); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(writer, payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var got result
	select {
	case got = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("late body request did not settle")
	}
	f.assertProfiles(t, 0)
	if got.err != nil || got.status != http.StatusUnauthorized {
		t.Fatalf("late body should retain revoked request without writing profiles: status=%d err=%v", got.status, got.err)
	}
	if status, _ := f.request(t, http.MethodPost, "/api/profiles", `{"profile":{"profileName":"fresh"}}`); status != http.StatusCreated {
		t.Fatalf("fresh request blocked=%d", status)
	}
	f.assertProfiles(t, 1)
}

func TestAuthBoundCDPExpiryLogoutAndReplacementCloseOnlyProxy(t *testing.T) {
	for _, mode := range []string{"logout", "expired-deadline", "replacement-granted"} {
		t.Run(mode, func(t *testing.T) {
			f := newNativeIngressFixture(t)
			closed := make(chan struct{})
			upgraded := make(chan int32, 2)
			var connections, messages atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/alive" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				connection := connections.Add(1)
				upgraded <- connection
				if connection != 1 && mode != "replacement-granted" {
					t.Error("revoked upgrade reached protocol fixture")
					return
				}
				if connection == 1 {
					defer close(closed)
				}
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
					messages.Add(1)
				}
			}))
			defer target.Close()
			awaitUpgrade := func(want int32) {
				t.Helper()
				select {
				case got := <-upgraded:
					if got != want {
						t.Fatalf("protocol fixture upgrade=%d want=%d", got, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("protocol fixture did not finish its upgrade")
				}
			}
			u, _ := url.Parse(target.URL)
			port, _ := strconv.Atoi(u.Port())
			f.server.SetActiveProfile(&browser.Profile{ProfileId: "protocol-fixture", Running: true, DebugReady: true, DebugPort: port})
			headers := http.Header{}
			headers.Set(launchcode.DefaultAPIKeyHeader, f.key)
			dialer := &websocket.Dialer{HandshakeTimeout: 2 * time.Second}
			endpoint := "ws" + strings.TrimPrefix(f.base, "http") + "/devtools/browser/fixture"
			conn, resp, err := dialer.Dial(endpoint, headers)
			if err != nil {
				if resp != nil {
					t.Fatalf("live production upgrade=%d", resp.StatusCode)
				}
				t.Fatal(err)
			}
			defer conn.Close()
			awaitUpgrade(1)
			if mode == "logout" || mode == "replacement-granted" {
				if _, err := f.bound.AuthLogout(7); err != nil {
					t.Fatal(err)
				}
				if mode == "replacement-granted" {
					generation := f.replaceIdentity(t)
					if err := f.bound.AuthGrantAPI(generation, true); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				f.identity.mu.Lock()
				f.identity.status.ExpiresAt = time.Unix(1, 0)
				f.identity.mu.Unlock()
			}
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			if _, _, err := conn.ReadMessage(); err == nil {
				t.Fatal("revoked proxy remained open")
			}
			select {
			case <-closed:
			case <-time.After(2 * time.Second):
				t.Fatal("upstream proxy connection retained")
			}
			if connections.Load() != 1 || messages.Load() != 0 || f.app.forceQuit {
				t.Fatal("revocation sent a browser command or forced shutdown")
			}
			newConn, rejected, dialErr := dialer.Dial(endpoint, headers)
			if newConn != nil {
				defer newConn.Close()
			}
			if rejected != nil {
				defer rejected.Body.Close()
			}
			if mode == "replacement-granted" {
				if dialErr != nil || newConn == nil {
					t.Fatal("fresh explicitly granted identity could not use a new proxy")
				}
				awaitUpgrade(2)
			} else {
				if status, _ := f.request(t, http.MethodGet, "/json/version", ""); status != http.StatusUnauthorized {
					t.Fatalf("revoked new CDP request=%d", status)
				}
				if dialErr == nil || rejected == nil || rejected.StatusCode != http.StatusUnauthorized || connections.Load() != 1 {
					t.Fatal("revoked websocket upgrade was not rejected before upstream")
				}
			}
			alive, err := f.client.Get(target.URL + "/alive")
			if err != nil {
				t.Fatal(err)
			}
			alive.Body.Close()
			if alive.StatusCode != http.StatusNoContent {
				t.Fatal("protocol fixture was stopped")
			}
		})
	}
}
