package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuthBoundOwnedCleanupUsesProofAndOneNormalCommand(t *testing.T) {
	for _, mode := range []string{"verified", "creation-mismatch", "recovered", "unknown-ack", "foreign-endpoint", "unproven-endpoint"} {
		t.Run(mode, func(t *testing.T) {
			var commands atomic.Int32
			var target *httptest.Server
			target = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/json/version" {
					endpoint := "ws" + strings.TrimPrefix(target.URL, "http") + "/devtools/browser/synthetic"
					if mode == "foreign-endpoint" {
						endpoint = "ws://example.invalid/devtools/browser/foreign"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"webSocketDebuggerUrl": endpoint})
					return
				}
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				var request cdpMessage
				if conn.ReadJSON(&request) != nil {
					return
				}
				if request.Method != "Browser.close" {
					t.Error("cleanup sent a non-close command")
				}
				commands.Add(1)
				if mode == "unknown-ack" {
					return
				}
				_ = conn.WriteJSON(map[string]any{"id": request.Id, "result": map[string]any{}})
			}))
			defer target.Close()
			u, _ := url.Parse(target.URL)
			port, _ := strconv.Atoi(u.Port())
			a, b, m := authFixture(t)
			a.config = config.DefaultConfig()
			a.browserMgr = browser.NewManager(a.config, a.appRoot)
			directory := filepath.Join(t.TempDir(), "synthetic-profile")
			cmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
			monitor := &browserProcessMonitor{cmd: cmd, waitDone: make(chan struct{})}
			a.browserMgr.Profiles["owned"] = &browser.Profile{ProfileId: "owned", UserDataDir: directory, Pid: 12345, DebugPort: port, Running: true}
			a.browserMgr.BrowserProcesses["owned"] = cmd
			a.browserProcessMonitors["owned"] = monitor
			oldCreation, oldEndpoint := readOwnedProcessCreation, verifyOwnedProcessEndpoint
			t.Cleanup(func() { readOwnedProcessCreation = oldCreation; verifyOwnedProcessEndpoint = oldEndpoint })
			readOwnedProcessCreation = func(pid int) (string, error) {
				if pid != 12345 {
					t.Fatal("arbitrary PID proof requested")
				}
				return "synthetic-creation", nil
			}
			verifyOwnedProcessEndpoint = func(pid int, path string, p int) error {
				if pid != 12345 || path != directory || p != port {
					t.Fatal("ownership tuple changed")
				}
				if mode == "unproven-endpoint" {
					return errors.New("private native query failure containing identity data")
				}
				return nil
			}
			scope, err := a.scoped(7)
			if err != nil {
				t.Fatal(err)
			}
			scope.rememberOwnedBrowser("owned", directory, port, cmd, monitor)
			_, _ = m.SignOut(7)
			if mode == "creation-mismatch" {
				readOwnedProcessCreation = func(int) (string, error) { return "reused-pid", nil }
			}
			if mode == "recovered" {
				a.browserMgr.BrowserProcesses["owned"] = nil
			}
			err = b.CloseOwnedBrowser("owned")
			if mode == "verified" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, errOwnedCleanupUnknown) {
				t.Fatalf("unknown/unproven close accepted: %v", err)
			}
			if mode != "verified" {
				stage := map[string]string{
					"creation-mismatch": "process_identity_changed",
					"recovered":         "start_identity_changed",
					"unknown-ack":       "cdp_exchange_unconfirmed",
					"foreign-endpoint":  "endpoint_identity_unproven",
					"unproven-endpoint": "process_endpoint_unproven",
				}[mode]
				if !strings.Contains(err.Error(), stage) {
					t.Errorf("cleanup lost its bounded failure stage %s: %v", stage, err)
				}
				if strings.Contains(err.Error(), "identity data") {
					t.Fatal("native query details leaked through cleanup error")
				}
			}
			expected := int32(0)
			if mode == "verified" || mode == "unknown-ack" {
				expected = 1
			}
			if commands.Load() != expected {
				t.Fatalf("normal commands %d, expected %d", commands.Load(), expected)
			}
			if err := b.CloseOwnedBrowser("owned"); !errors.Is(err, errOwnedCleanupUnknown) {
				t.Fatal("cleanup replayed")
			}
			if commands.Load() != expected {
				t.Fatal("unknown cleanup automatically retried")
			}
			if a.forceQuit || !a.browserMgr.Profiles["owned"].Running {
				t.Fatal("cleanup invoked force/metadata fallback")
			}
		})
	}
}

func TestAuthBoundTrayExitWithoutSessionKeepsInstances(t *testing.T) {
	a, _, m := authFixture(t)
	_, _ = m.SignOut(7)
	QuitFromTray(a)
	if a.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("tray bypassed login to full runtime shutdown")
	}
}

func TestStoreCloseRetainsExitUnknownWithoutReplayingCommand(t *testing.T) {
	var commands atomic.Int32
	var target *httptest.Server
	target = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			_ = json.NewEncoder(w).Encode(map[string]string{"webSocketDebuggerUrl": "ws" + strings.TrimPrefix(target.URL, "http") + "/devtools/browser/synthetic"})
			return
		}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var command cdpMessage
		if conn.ReadJSON(&command) != nil || command.Method != "Browser.close" {
			return
		}
		commands.Add(1)
		_ = conn.WriteJSON(map[string]any{"id": command.Id, "result": map[string]any{}})
	}))
	defer target.Close()
	u, _ := url.Parse(target.URL)
	port, _ := strconv.Atoi(u.Port())
	a, api, scope := storeAppFixture(t)
	b := installStoreFixture(t, a, scope)
	key := uuid.NewString()
	dao := a.store.dao
	if err := dao.Prepare(a.ctx, scope, b, key, "open_environment"); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"admitted", "attempted"} {
		if err := dao.Phase(a.ctx, scope, key, phase); err != nil {
			t.Fatal(err)
		}
	}
	if err := dao.Finish(a.ctx, scope, key, "succeeded", "process_started"); err != nil {
		t.Fatal(err)
	}
	cmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	monitor := &browserProcessMonitor{cmd: cmd, waitDone: make(chan struct{})}
	p := a.browserMgr.Profiles[b.ProfileID]
	p.Pid, p.DebugPort, p.Running = 12345, port, true
	a.browserMgr.BrowserProcesses[b.ProfileID] = cmd
	a.browserProcessMonitors = map[string]*browserProcessMonitor{b.ProfileID: monitor}
	oldCreation, oldEndpoint := readOwnedProcessCreation, verifyOwnedProcessEndpoint
	t.Cleanup(func() { readOwnedProcessCreation = oldCreation; verifyOwnedProcessEndpoint = oldEndpoint })
	readOwnedProcessCreation = func(int) (string, error) { return "synthetic-creation", nil }
	verifyOwnedProcessEndpoint = func(int, string, int) error { return nil }
	scoped, err := a.storeActionReceiver(1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	scoped.rememberOwnedBrowser(b.ProfileID, a.browserMgr.ResolveUserDataDir(p), port, cmd, monitor)
	for attempt := range 2 {
		_, err := a.StoreEnvironmentClose(scope.StoreID, 1)
		stage := "process_exit_unconfirmed"
		if attempt == 1 {
			stage = "close_already_attempted"
		}
		if !errors.Is(err, errOwnedCleanupUnknown) || !strings.Contains(fmt.Sprint(err), stage) {
			t.Errorf("close uncertainty lost bounded stage %s: %v", stage, err)
		}
	}
	reports, err := dao.Reports(a.ctx, scope, key)
	if err != nil || len(reports) != 1 || reports[0].Kind != "close_result" || reports[0].Outcome != "unknown" || reports[0].ReasonCode != "result_unknown" {
		t.Fatalf("original immutable unknown report lost: %+v %v", reports, err)
	}
	if commands.Load() != 1 || api.reports != 1 || !p.Running || a.forceQuit {
		t.Fatal("unknown close replayed a command/report or pretended to stop/force the browser")
	}
}
