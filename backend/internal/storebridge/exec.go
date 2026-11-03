package storebridge

import (
	"ant-chrome/backend/internal/proxy"
	releaseassets "ant-chrome/publish"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RuntimeSnapshot is copied from native configuration while its existing global
// ingress fence is held. It is never decoded from a renderer request.
type RuntimeSnapshot struct{ AppRoot, Stack, XrayPath, MihomoPath string }

type ExecStarter struct {
	operation context.Context
	snapshot  RuntimeSnapshot
}

func NewExecStarter(operation context.Context, snapshot RuntimeSnapshot) *ExecStarter {
	return &ExecStarter{operation, snapshot}
}

func (s *ExecStarter) Start(ctx context.Context, input Input) (Process, error) {
	if !listenerOwnershipSupported() {
		return nil, ErrRequired
	}
	expected := s.snapshot.Stack
	if expected == "" {
		expected = "xray"
	}
	if input.Engine() != expected || (expected != "xray" && expected != "mihomo") {
		return nil, ErrRequired
	}
	deadline, ok := s.operation.Deadline()
	if !ok || time.Until(deadline) > 12*time.Second || s.operation.Err() != nil || ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	startupDeadline, bounded := ctx.Deadline()
	if !bounded || time.Until(startupDeadline) > 3*time.Second {
		return nil, ErrUnavailable
	}
	path, pinned, err := s.runtimePath(input.Engine())
	if err != nil {
		return nil, ErrRequired
	}
	binary, err := openRuntime(path)
	if err != nil {
		return nil, ErrRequired
	}
	owned := false
	defer func() {
		if !owned {
			_ = binary.Close()
		}
	}()
	info, err := binary.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 128<<20 || info.Size() == 0 {
		return nil, ErrRequired
	}
	if pinned != "" {
		hash := sha256.New()
		buffer := make([]byte, 64*1024)
		for {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			n, e := binary.Read(buffer)
			if n > 0 {
				_, _ = hash.Write(buffer[:n])
			}
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, ErrRequired
			}
		}
		if hex.EncodeToString(hash.Sum(nil)) != pinned {
			return nil, ErrRequired
		}
	}
	reservation, err := new(net.ListenConfig).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, ErrUnavailable
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	_ = reservation.Close()
	configuration, err := proxy.BuildOwnedStandardProbeConfig(input.proxy.URL.String(), input.Engine(), port)
	if err != nil {
		return nil, ErrUnavailable
	}
	home, err := os.MkdirTemp("", "shuomi-store-probe-")
	if err != nil {
		clear(configuration)
		return nil, ErrUnavailable
	}
	defer func() {
		if !owned {
			_ = removeOwnedHome(home)
		}
	}()
	args := []string{"run", "-format=json", "-config", "stdin:"}
	if input.Engine() == proxy.ProxyKernelMihomo {
		args = []string{"-d", home, "-f", "-"}
	}
	command := exec.Command(path, args...)
	hideOwnedProcess(command)
	command.Dir = home
	command.Env = privateEnvironment()
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		clear(configuration)
		return nil, ErrUnavailable
	}
	command.Stdout = null
	command.Stderr = null
	stdin, err := command.StdinPipe()
	if err != nil {
		_ = null.Close()
		clear(configuration)
		return nil, ErrUnavailable
	}
	if ctx.Err() != nil {
		_ = stdin.Close()
		_ = null.Close()
		clear(configuration)
		return nil, ctx.Err()
	}
	if command.Start() != nil {
		_ = stdin.Close()
		_ = null.Close()
		clear(configuration)
		return nil, ErrUnavailable
	}
	p := &execProcess{command: command, stdin: stdin, binary: binary, null: null, home: home, path: path, info: info, port: port, writeDone: make(chan struct{}), exited: make(chan struct{}), settled: make(chan struct{})}
	p.startupWatch = p.watch(ctx)
	p.operationWatch = p.watch(s.operation)
	owned = true
	go func() {
		defer close(p.writeDone)
		defer clear(configuration)
		defer stdin.Close()
		if n, e := stdin.Write(configuration); e != nil || n != len(configuration) {
			p.inputFailed.Store(true)
		}
	}()
	go p.join()
	return p, nil
}

func (s *ExecStarter) runtimePath(engine string) (string, string, error) {
	configured := ""
	bundled := false
	switch engine {
	case proxy.ProxyKernelXray:
		configured = s.snapshot.XrayPath
		if configured == "" {
			configured = "bin/xray"
			if runtime.GOOS == "windows" {
				configured += ".exe"
			}
			bundled = true
		}
	case proxy.ProxyKernelMihomo:
		configured = s.snapshot.MihomoPath
	default:
		return "", "", ErrRequired
	}
	if configured == "" || strings.ContainsAny(configured, "\x00\r\n") || strings.HasPrefix(configured, `\\`) {
		return "", "", ErrRequired
	}
	if !filepath.IsAbs(configured) {
		if !filepath.IsAbs(s.snapshot.AppRoot) {
			return "", "", ErrRequired
		}
		configured = filepath.Join(s.snapshot.AppRoot, configured)
	}
	resolved := filepath.Clean(configured)
	if !safeRuntimeAncestors(resolved) {
		return "", "", ErrRequired
	}
	pinned := ""
	if bundled {
		var manifest struct {
			Schema int `json:"schemaVersion"`
			Files  []struct {
				Path, SHA256 string
				Targets      []string
			}
		}
		if json.Unmarshal(releaseassets.RuntimeManifest(), &manifest) != nil || manifest.Schema != 1 {
			return "", "", ErrRequired
		}
		for _, entry := range manifest.Files {
			for _, target := range entry.Targets {
				if target == runtime.GOOS+"-"+runtime.GOARCH && (filepath.Base(entry.Path) == "xray" || filepath.Base(entry.Path) == "xray.exe") {
					pinned = entry.SHA256
				}
			}
		}
		if len(pinned) != 64 {
			return "", "", ErrRequired
		}
	}
	return resolved, pinned, nil
}

func privateEnvironment() []string {
	out := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "LANG", "LC_ALL":
			out = append(out, entry)
		}
	}
	return out
}

type execProcess struct {
	command                      *exec.Cmd
	stdin                        io.WriteCloser
	binary, null                 *os.File
	home, path                   string
	info                         os.FileInfo
	port                         int
	writeDone, exited, settled   chan struct{}
	startupWatch, operationWatch func()
	stopOnce                     sync.Once
	inputFailed, cleanupFailed   atomic.Bool
}

func (p *execProcess) watch(ctx context.Context) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(done); _ = p.Stop() })
	var joined sync.Once
	return func() {
		joined.Do(func() {
			if !stop() {
				<-done
			}
		})
	}
}
func (p *execProcess) Stop() error {
	p.stopOnce.Do(func() {
		// Windows anonymous-pipe Close can wait for an active WriteFile. Kill
		// the owned reader first so a core that never reads stdin cannot prevent
		// cancellation from reaching the process.
		if err := p.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			select {
			case <-p.exited:
			default:
				p.cleanupFailed.Store(true)
			}
		}
		_ = p.stdin.Close()
	})
	if p.cleanupFailed.Load() {
		return ErrUnavailable
	}
	return nil
}
func (p *execProcess) join() {
	_ = p.command.Wait()
	close(p.exited)
	<-p.writeDone
	p.startupWatch()
	p.operationWatch()
	if p.binary.Close() != nil {
		p.cleanupFailed.Store(true)
	}
	if p.null.Close() != nil {
		p.cleanupFailed.Store(true)
	}
	if removeOwnedHome(p.home) != nil {
		p.cleanupFailed.Store(true)
	}
	close(p.settled)
}
func (p *execProcess) Wait() error {
	<-p.settled
	if p.cleanupFailed.Load() {
		return ErrUnavailable
	}
	return nil
}
func (p *execProcess) Settled() <-chan struct{} { return p.settled }

func (p *execProcess) Ready(ctx context.Context) (string, error) {
	select {
	case <-p.writeDone:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if p.inputFailed.Load() {
		return "", ErrUnavailable
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(p.port))
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-p.exited:
			return "", ErrUnavailable
		default:
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if ownsListener(ctx, p.command.Process.Pid, p.port) && socksReady(ctx, address) == nil && ownsListener(ctx, p.command.Process.Pid, p.port) {
			p.startupWatch()
			current, err := os.Stat(p.path)
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if err != nil || !os.SameFile(p.info, current) {
				return "", ErrUnavailable
			}
			select {
			case <-p.exited:
				return "", ErrUnavailable
			default:
				return "socks5://" + address, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-p.exited:
			return "", ErrUnavailable
		case <-ticker.C:
		}
	}
}
func socksReady(ctx context.Context, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	conn, err := new(net.Dialer).DialContext(ctx, "tcp4", address)
	if err != nil {
		return ErrUnavailable
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(done); _ = conn.Close() })
	defer func() {
		_ = conn.Close()
		if !stop() {
			<-done
		}
	}()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	if _, err = conn.Write([]byte{5, 1, 0}); err != nil {
		return ErrUnavailable
	}
	response := [2]byte{}
	if _, err = io.ReadFull(conn, response[:]); err != nil || response != [2]byte{5, 0} {
		return ErrUnavailable
	}
	return nil
}
func removeOwnedHome(path string) error {
	if filepath.Dir(filepath.Clean(path)) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(path), "shuomi-store-probe-") {
		return ErrUnavailable
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrUnavailable
	}
	return os.RemoveAll(path)
}
