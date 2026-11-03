package backend

import (
	"ant-chrome/backend/internal/storebridge"
	"ant-chrome/backend/internal/storeproxy"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type ownedProbeRoundTrip func(*http.Request) (*http.Response, error)

func (f ownedProbeRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func buildNativeCoreFixture(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "core-fixture.exe")
	command := exec.CommandContext(ctx, "go", "build", "-o", path, "./internal/storebridge/testdata/core")
	if command.Run() != nil {
		t.Fatal("local native core fixture did not build")
	}
	return path
}

func TestStoreProxyRealStarterUsesNativeSnapshotAndNoBusinessEffects(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows native candidate")
	}
	basePath := buildNativeCoreFixture(t)
	for _, stack := range []string{"xray", "mihomo"} {
		t.Run(stack, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "core-fixture.exe")
			data, _ := os.ReadFile(basePath)
			if os.WriteFile(path, data, 0700) != nil {
				t.Fatal("fixture copy failed")
			}
			a, api, scope := storeAppFixture(t)
			binding := installStoreFixture(t, a, scope)
			a.config.Browser.DefaultConnectorType = stack
			a.config.Browser.XrayBinaryPath = path
			a.config.Browser.ClashBinaryPath = path
			a.storeProbeCatalog = probeCatalog(t)
			calls, targetCalls := 0, 0
			a.storeProbeFactory = func(ctx context.Context, snapshot storebridge.RuntimeSnapshot) storeproxy.Prober {
				calls++
				if snapshot.Stack != stack || snapshot.XrayPath != path || snapshot.MihomoPath != path {
					t.Error("native snapshot drift")
				}
				p := a.storeNetworkProber(ctx, snapshot)
				p.Dial = func(context.Context, string, string) (net.Conn, error) { return &storeProbeTCPConn{}, nil }
				p.Transport = func(endpoint storeproxy.Endpoint) (http.RoundTripper, func()) {
					if endpoint.URL.Hostname() != "127.0.0.1" || endpoint.URL.User != nil {
						t.Error("native transport bypassed owned bridge")
					}
					return ownedProbeRoundTrip(func(r *http.Request) (*http.Response, error) {
						targetCalls++
						if r.URL.String() != "https://target.example.invalid/check" || r.Header.Get("Authorization") != "" {
							t.Error("target/auth drift")
						}
						return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
					}), func() {}
				}
				return p
			}
			raw, err := NewAuthBoundApp(a).StoreProxyCheck(scope.StoreID, binding.BindingVersion, probeRaw(uuid.NewString(), a.storeProbeCatalog.Version()), 1, 7)
			result := parseProbeResult(t, raw, err)
			if result.Reason != storeproxy.None || result.Target != "passed" || calls != 1 || targetCalls != 1 {
				t.Error("real native starter/fixture failed", result)
			}
			if api.admitted != 0 || api.reports != 0 || len(a.browserMgr.Profiles) != 1 || a.browserMgr.Profiles[binding.ProfileID].Running {
				t.Error("check changed business state")
			}
			if os.Remove(path) != nil {
				t.Error("native call returned before process/runtime lock cleanup")
			}
		})
	}
}

func TestStoreProxySnapshotFenceBlocksGenericSettingsWrite(t *testing.T) {
	for _, test := range []struct {
		name    string
		managed bool
	}{{"unsaved", false}, {"managed", true}} {
		t.Run(test.name, func(t *testing.T) {
			a, _, scope := storeAppFixture(t)
			a.appRoot = t.TempDir()
			version := "0"
			if test.managed {
				version = installStoreFixture(t, a, scope).BindingVersion
			}
			a.storeProbeCatalog = probeCatalog(t)
			a.config.Browser.DefaultConnectorType = "xray"
			entered, release := make(chan struct{}), make(chan struct{})
			a.storeProbeFactory = func(ctx context.Context, snapshot storebridge.RuntimeSnapshot) storeproxy.Prober {
				if snapshot.Stack != "xray" {
					t.Error("wrong native stack")
				}
				return storeProbeFunc(func(context.Context, string, storeproxy.Target, string, string, storeproxy.Authorize) storeproxy.Result {
					close(entered)
					<-release
					return storeproxy.Unknown("id", "fixture", storeproxy.Unavailable)
				})
			}
			bound := NewAuthBoundApp(a)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				_, _ = bound.StoreProxyCheck(scope.StoreID, version, probeRaw(uuid.NewString(), a.storeProbeCatalog.Version()), 1, 7)
			}()
			<-entered
			settingsDone := make(chan error, 1)
			go func() { settingsDone <- bound.SaveBrowserSettings(BrowserSettings{DefaultConnectorType: "mihomo"}, 7) }()
			select {
			case <-settingsDone:
				t.Error("generic settings crossed the native snapshot fence")
			case <-time.After(30 * time.Millisecond):
			}
			close(release)
			<-finished
			select {
			case err := <-settingsDone:
				if test.managed && !errors.Is(err, errStoreBoundary) || !test.managed && err != nil {
					t.Error("generic Store guard weakened")
				}
			case <-time.After(time.Second):
				t.Fatal("native fence did not release")
			}
			expected := "xray"
			if !test.managed {
				expected = "mihomo"
			}
			if a.config.Browser.DefaultConnectorType != expected {
				t.Error("generic settings changed checked stack")
			}
		})
	}
}

func TestStoreProxyRequestCannotSupplyRuntimeAuthority(t *testing.T) {
	a, _, scope := storeAppFixture(t)
	binding := installStoreFixture(t, a, scope)
	a.storeProbeCatalog = probeCatalog(t)
	for _, field := range []string{"stack", "xrayPath", "mihomoPath", "binary", "command"} {
		var request map[string]any
		_ = json.Unmarshal([]byte(probeRaw(uuid.NewString(), a.storeProbeCatalog.Version())), &request)
		request[field] = "fixture-injection"
		raw, _ := json.Marshal(request)
		if _, err := a.StoreProxyCheck(scope.StoreID, binding.BindingVersion, string(raw), 1); err != errStoreProxyRequest {
			t.Error("caller runtime authority accepted")
		}
	}
}
