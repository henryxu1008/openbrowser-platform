package storebridge

import (
	"ant-chrome/backend/internal/storeproxy"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const fixtureProxy = "http://fixture-user:fixture-secret%3A%40%2F@proxy.example.invalid:8080"

type startFunc func(context.Context, Input) (Process, error)

func (f startFunc) Start(ctx context.Context, input Input) (Process, error) { return f(ctx, input) }

// The only executable launched by these tests is this test binary, with fixed
// arguments. The child reads synthetic credentials on stdin and emits one
// bounded private readiness frame. No core, port listener or network is started.
func TestBridgeChildFixture(t *testing.T) {
	if os.Getenv("STORE_BRIDGE_CHILD_FIXTURE") != "1" {
		return
	}
	reader := bufio.NewReader(io.LimitReader(os.Stdin, 16384))
	line, err := reader.ReadBytes('\n')
	var input struct {
		Engine, Protocol, Server, Username, Password string
		Port                                         int
	}
	if err != nil || json.Unmarshal(line, &input) != nil || input.Username != "fixture-user" || input.Password != "fixture-secret:@/" || input.Server != "proxy.example.invalid" || input.Protocol != "http" || input.Port != 8080 {
		os.Exit(3)
	}
	mode := os.Getenv("STORE_BRIDGE_CHILD_MODE")
	if mode != "no-ready" {
		endpoint := "socks5://127.0.0.1:31991"
		if mode == "bad-ready" {
			endpoint = "socks5://remote.example.invalid:31991"
		}
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Endpoint, Engine string
			PrivateInputOK   bool
		}{endpoint, input.Engine, true})
	}
	_, _ = io.Copy(io.Discard, reader)
	os.Exit(0)
}

type processFixture struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	ready chan struct {
		endpoint string
		err      error
	}
	exited, waitDone chan struct{}
	waitCleanup      <-chan struct{}
	stops, waits     atomic.Int32
	inputOK          atomic.Bool
	engine           string
}

func launchFixture(t *testing.T, input Input, mode string, cleanup <-chan struct{}) *processFixture {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal("test executable unavailable")
	}
	cmd := exec.Command(executable, "-test.run=^TestBridgeChildFixture$")
	hideFixture(cmd)
	cmd.Env = append(os.Environ(), "STORE_BRIDGE_CHILD_FIXTURE=1", "STORE_BRIDGE_CHILD_MODE="+mode)
	if strings.Contains(strings.Join(cmd.Args, " ")+strings.Join(cmd.Env, " "), "fixture-secret") {
		t.Fatal("credential in process metadata")
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal("stdin unavailable")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal("stdout unavailable")
	}
	cmd.Stderr = io.Discard
	if cmd.Start() != nil {
		t.Fatal("fixture process unavailable")
	}
	p := &processFixture{cmd: cmd, stdin: stdin, exited: make(chan struct{}), waitDone: make(chan struct{}), waitCleanup: cleanup, engine: input.Engine(), ready: make(chan struct {
		endpoint string
		err      error
	}, 1)}
	writerDone, readerDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(writerDone); _ = input.WritePrivateInput(stdin) }()
	go func() {
		defer close(readerDone)
		reader := bufio.NewReader(io.LimitReader(stdout, 512))
		line, err := reader.ReadBytes('\n')
		var frame struct {
			Endpoint, Engine string
			PrivateInputOK   bool
		}
		if err == nil && json.Unmarshal(line, &frame) == nil && frame.Engine == input.Engine() && frame.PrivateInputOK {
			p.inputOK.Store(true)
		} else {
			err = ErrUnavailable
		}
		p.ready <- struct {
			endpoint string
			err      error
		}{frame.Endpoint, err}
		_, _ = io.Copy(io.Discard, stdout)
	}()
	go func() {
		_ = cmd.Wait()
		<-writerDone
		<-readerDone
		close(p.exited)
		if p.waitCleanup != nil {
			<-p.waitCleanup
		}
		close(p.waitDone)
	}()
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Process.Kill(); <-p.exited })
	return p
}

func (p *processFixture) Ready(ctx context.Context) (string, error) {
	select {
	case result := <-p.ready:
		return result.endpoint, result.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
func (p *processFixture) Stop()                    { p.stops.Add(1); _ = p.stdin.Close(); _ = p.cmd.Process.Kill() }
func (p *processFixture) Wait()                    { p.waits.Add(1); <-p.waitDone }
func (p *processFixture) Settled() <-chan struct{} { return p.waitDone }

type panicCleanupProcess struct {
	*processFixture
	stopPanic, waitPanic bool
	stopCalls, waitCalls atomic.Int32
}

func (p *panicCleanupProcess) Ready(ctx context.Context) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}
func (p *panicCleanupProcess) Stop() {
	p.stopCalls.Add(1)
	if p.stopPanic {
		panic(fixtureProxy)
	}
}
func (p *panicCleanupProcess) Wait() {
	p.waitCalls.Add(1)
	if p.waitPanic {
		panic(fixtureProxy)
	}
	p.processFixture.Wait()
}

func TestOwnedBridgeStopWaitPanicsStillJoin(t *testing.T) {
	for _, mode := range []string{"wait", "stop", "both"} {
		t.Run(mode, func(t *testing.T) {
			cleanup := make(chan struct{})
			var release sync.Once
			defer release.Do(func() { close(cleanup) })
			started := make(chan *panicCleanupProcess, 1)
			connector := New("xray", startFunc(func(ctx context.Context, input Input) (Process, error) {
				p := &panicCleanupProcess{processFixture: launchFixture(t, input, "no-ready", cleanup), stopPanic: mode == "stop" || mode == "both", waitPanic: mode == "wait" || mode == "both"}
				started <- p
				return p, nil
			}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { _, err := connector.Acquire(ctx, fixtureProxy); result <- err }()
			process := <-started
			// The fixture's independent owner has killed/joined the exact child,
			// while I/O/resource cleanup deliberately remains incomplete.
			process.processFixture.Stop()
			<-process.exited
			cancel()
			select {
			case <-result:
				t.Error("cleanup panic acknowledged settlement before owned cleanup")
			case <-time.After(30 * time.Millisecond):
			}
			release.Do(func() { close(cleanup) })
			select {
			case err := <-result:
				if err != ErrUnavailable {
					t.Error("cleanup fault did not become fixed unavailable")
				}
			case <-time.After(time.Second):
				t.Fatal("fault cleanup did not settle")
			}
			if process.stopCalls.Load() != 1 || process.waitCalls.Load() != 1 {
				t.Error("fault skipped or repeated a cleanup phase")
			}
		})
	}
}

func TestOwnedBridgeUsesConfiguredStackAndPrivateInput(t *testing.T) {
	for _, stack := range []string{"xray", "mihomo"} {
		t.Run(stack, func(t *testing.T) {
			var process *processFixture
			connector := New(stack, startFunc(func(ctx context.Context, input Input) (Process, error) {
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 3*time.Second {
					t.Error("unbounded startup")
				}
				if input.Engine() != stack || strings.Contains(fmt.Sprintf("%v %+v %#v", input, input, input), "fixture-secret") {
					t.Error("stack or redaction drift")
				}
				process = launchFixture(t, input, "ready", nil)
				return process, nil
			}))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			lease, err := connector.Acquire(ctx, fixtureProxy)
			if err != nil {
				t.Fatal(err)
			}
			if lease.Endpoint() != "socks5://127.0.0.1:31991" || !process.inputOK.Load() {
				t.Error("private input or readiness failed")
			}
			var closes sync.WaitGroup
			for range 8 {
				closes.Add(1)
				go func() { defer closes.Done(); lease.CloseAndWait() }()
			}
			closes.Wait()
			if process.stops.Load() != 1 || process.waits.Load() != 1 {
				t.Fatal("process cleanup repeated")
			}
			select {
			case <-process.exited:
			default:
				t.Fatal("process not joined")
			}
		})
	}
}

type panicReady struct{ Process }

func (panicReady) Ready(context.Context) (string, error) { panic(fixtureProxy) }

func TestOwnedBridgeReadyPanicAndDeadlineJoinRealProcess(t *testing.T) {
	for _, mode := range []string{"panic", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			var process *processFixture
			connector := New("xray", startFunc(func(ctx context.Context, input Input) (Process, error) {
				process = launchFixture(t, input, "no-ready", nil)
				if mode == "panic" {
					return panicReady{process}, nil
				}
				return process, nil
			}))
			ctx := context.Background()
			if mode == "deadline" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 60*time.Millisecond)
				defer cancel()
			}
			lease, err := connector.Acquire(ctx, fixtureProxy)
			if lease != nil || mode == "panic" && err != ErrUnavailable || mode == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
				t.Error("panic/deadline result not fixed")
			}
			select {
			case <-process.exited:
			default:
				t.Error("panic/deadline process not joined")
			}
			if process.stops.Load() != 1 || process.waits.Load() != 1 {
				t.Error("panic/deadline cleanup repeated")
			}
		})
	}
}

func TestOwnedBridgeStandardProtocolMatrix(t *testing.T) {
	for _, stack := range []string{"xray", "mihomo"} {
		for _, scheme := range []string{"http", "https", "socks5"} {
			input, err := makeInput(scheme+"://fixture-user:fixture-secret%3A%40%2F@proxy.example.invalid:8080", stack)
			if err != nil || input.Engine() != stack {
				t.Error("configured standard-protocol stack drift")
			}
		}
	}
}

func TestOwnedBridgeCancelDuringStartupJoinsRealChildAndDelayedCleanup(t *testing.T) {
	cleanup := make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(cleanup) })
	started := make(chan *processFixture, 1)
	connector := New("xray", startFunc(func(ctx context.Context, input Input) (Process, error) {
		p := launchFixture(t, input, "no-ready", cleanup)
		started <- p
		return p, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := connector.Acquire(ctx, fixtureProxy); result <- err }()
	process := <-started
	cancel()
	select {
	case <-process.exited:
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not stop/join child")
	}
	select {
	case <-result:
		t.Error("operation returned before process cleanup joined")
	case <-time.After(30 * time.Millisecond):
	}
	release.Do(func() { close(cleanup) })
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Error("cancellation reason lost")
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup did not settle")
	}
	if process.stops.Load() != 1 || process.waits.Load() != 1 {
		t.Fatal("cleanup repeated")
	}
}

func TestOwnedBridgeRejectsRemoteReadinessAndMapsErrors(t *testing.T) {
	for _, mode := range []string{"bad-ready", "start-error"} {
		t.Run(mode, func(t *testing.T) {
			var process *processFixture
			connector := New("mihomo", startFunc(func(ctx context.Context, input Input) (Process, error) {
				process = launchFixture(t, input, "bad-ready", nil)
				if mode == "start-error" {
					return process, errors.New(fixtureProxy)
				}
				return process, nil
			}))
			lease, err := connector.Acquire(context.Background(), fixtureProxy)
			if lease != nil || err != ErrUnavailable {
				t.Error("untrusted readiness or raw error accepted")
			}
			select {
			case <-process.exited:
			default:
				t.Fatal("failed startup process not joined")
			}
			if process.stops.Load() != 1 || process.waits.Load() != 1 {
				t.Fatal("failed-start cleanup repeated")
			}
		})
	}
}

func TestOwnedBridgeLateStartupHandleIsCleanedBeforeReturn(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var process *processFixture
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connector := New("xray", startFunc(func(_ context.Context, input Input) (Process, error) {
		process = launchFixture(t, input, "ready", nil)
		close(started)
		<-release
		return process, nil
	}))
	result := make(chan error, 1)
	go func() { _, err := connector.Acquire(ctx, fixtureProxy); result <- err }()
	<-started
	cancel()
	select {
	case <-result:
		t.Error("canceled startup detached owned handle")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Error("cancel reason lost")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("late handle not cleaned")
	}
	select {
	case <-process.exited:
	default:
		t.Fatal("late process not joined")
	}
}

func TestOwnedBridgeAdmissionAndUnboundDefaultNeverStart(t *testing.T) {
	calls := 0
	starter := startFunc(func(context.Context, Input) (Process, error) { calls++; return nil, errors.New(fixtureProxy) })
	for _, raw := range []string{"", "direct://", "http://proxy.example.invalid:8080", "vless://fixture", "http://user:pass@bad.invalid:0"} {
		if lease, err := New("xray", starter).Acquire(context.Background(), raw); lease != nil || err != ErrRequired {
			t.Error("unsupported input admitted")
		}
	}
	if _, err := New("other", starter).Acquire(context.Background(), fixtureProxy); err != ErrRequired {
		t.Error("invalid stack admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New("xray", starter).Acquire(ctx, fixtureProxy); !errors.Is(err, context.Canceled) {
		t.Error("canceled admission")
	}
	endpoint, _ := storeproxy.ParseEndpoint(fixtureProxy)
	if _, cleanup, reason := New("xray", nil).Connect(context.Background(), endpoint); cleanup != nil || reason != storeproxy.ConnectorRequired {
		t.Error("unbound production default admitted")
	}
	if calls != 0 {
		t.Fatal("inadmissible request started a process")
	}
}
