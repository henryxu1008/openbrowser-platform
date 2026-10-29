//go:build windows

package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Exercise the actual Windows creation-time, directory and listener checks.
// The child is this Go test executable serving a synthetic CDP endpoint; no
// browser, retained profile, login or shared service is launched or touched.
func TestOwnedCleanupWindowsNativeProof(t *testing.T) {
	dir := t.TempDir()
	reservation, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	_ = reservation.Close()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestOwnedCleanupWindowsHelper$", "--", "--user-data-dir="+dir, "--remote-debugging-port="+strconv.Itoa(port))
	cmd.Env = append(os.Environ(), "ANT_BROWSER_OWNED_CLEANUP_HELPER=1")
	hideWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := newBrowserProcessMonitor(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	monitor.Start()
	t.Cleanup(func() {
		if !monitor.HasExited() {
			_ = cmd.Process.Kill()
		}
		<-monitor.Done()
		_ = stdout.Close()
	})
	ready := make(chan bool, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		ready <- strings.TrimSpace(line) == "READY"
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("synthetic CDP helper did not start")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("synthetic CDP helper readiness timed out")
	}
	app := NewApp(t.TempDir())
	app.admission = func() error { return nil }
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.Profiles["native-proof"] = &browser.Profile{ProfileId: "native-proof", UserDataDir: dir, Pid: cmd.Process.Pid, DebugPort: port, Running: true}
	app.browserMgr.BrowserProcesses["native-proof"] = cmd
	app.browserProcessMonitors["native-proof"] = monitor
	app.rememberOwnedBrowser("native-proof", dir, port, cmd, monitor)
	if err = NewAuthBoundApp(app).CloseOwnedBrowser("native-proof"); err != nil {
		t.Fatalf("native ownership proof rejected the owned synthetic endpoint: %v", err)
	}
	select {
	case <-monitor.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("owned helper did not exit after the single normal close")
	}
}

func TestOwnedCleanupWindowsHelper(t *testing.T) {
	if os.Getenv("ANT_BROWSER_OWNED_CLEANUP_HELPER") != "1" {
		return
	}
	var port string
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--remote-debugging-port=") {
			port = strings.TrimPrefix(arg, "--remote-debugging-port=")
		}
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:"+port)
	if err != nil {
		os.Exit(2)
	}
	closed := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"webSocketDebuggerUrl": "ws://127.0.0.1:" + port + "/devtools/browser/synthetic"})
	})
	mux.HandleFunc("/devtools/browser/synthetic", func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var message cdpMessage
		if conn.ReadJSON(&message) != nil || message.Method != "Browser.close" {
			return
		}
		if conn.WriteJSON(map[string]any{"id": message.Id, "result": map[string]any{}}) == nil {
			close(closed)
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	go func() { _ = server.Serve(listener) }()
	fmt.Println("READY")
	select {
	case <-closed:
		_ = server.Close()
		os.Exit(0)
	case <-time.After(45 * time.Second):
		os.Exit(3)
	}
}
